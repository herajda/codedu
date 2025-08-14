<script lang="ts">
  import { onMount } from 'svelte'
  import { page } from '$app/stores'
  import { apiFetch, apiJSON } from '$lib/api'
  import { auth } from '$lib/auth'
  import CodeMirror from '$lib/components/ui/CodeMirror.svelte'

  $: id = $page.params.id
  $: role = $auth?.role ?? ''

  let assignment: any = null
  let tests: any[] = []
  let err = ''

  // local inputs for creating/uploading tests
  let tStdin = ''
  let tStdout = ''
  let tLimit = ''
  let tWeight = '1'
  let unittestFile: File | null = null

  // ──────────────────────────────────────────────────────
  // Unittest Builder state
  // ──────────────────────────────────────────────────────
  type UTAssertKind = 'equals' | 'notEquals' | 'contains' | 'notContains' | 'regex' | 'raises' | 'custom'
  type UTAssertion =
    | { kind: 'equals' | 'notEquals' | 'contains' | 'notContains'; args: string[]; expected: string }
    | { kind: 'regex'; args: string[]; pattern: string }
    | { kind: 'raises'; args: string[]; exception: string }
    | { kind: 'custom'; code: string }

  type UTTest = {
    name: string
    description?: string
    weight: string
    timeLimit: string
    assertions: UTAssertion[]
  }

  let utClassName = 'TestAssignment'
  let utSetup = ''
  let utTeardown = ''
  let utTests: UTTest[] = []
  let utShowPreview = false
  let utPreviewCode = ''

  function getInputs(a: UTAssertion): string {
    return (a as any).args ? ((a as any).args as string[]).join('\n') : ''
  }
  function setInputs(a: UTAssertion, v: string) {
    if ((a as any).args) (a as any).args = v.split('\n')
  }
  function getExpected(a: UTAssertion): string {
    return (a as any).expected ?? ''
  }
  function setExpected(a: UTAssertion, v: string) {
    (a as any).expected = v
  }
  function getPattern(a: UTAssertion): string {
    return (a as any).pattern ?? ''
  }
  function setPattern(a: UTAssertion, v: string) {
    (a as any).pattern = v
  }
  function getException(a: UTAssertion): string {
    return (a as any).exception ?? 'Exception'
  }
  function setException(a: UTAssertion, v: string) {
    (a as any).exception = v
  }
  function getCustom(a: UTAssertion): string {
    return (a as any).code ?? ''
  }
  function setCustom(a: UTAssertion, v: string) {
    (a as any).code = v
  }

  async function load() {
    err = ''
    try {
      const data = await apiJSON(`/api/assignments/${id}`)
      assignment = data.assignment
      tests = data.tests ?? []
    } catch (e: any) {
      err = e.message
    }
  }

  onMount(async () => {
    await load()
  })

  async function addTest() {
    try {
      await apiFetch(`/api/assignments/${id}/tests`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          stdin: tStdin,
          expected_stdout: tStdout,
          weight: parseFloat(tWeight) || 1,
          time_limit_sec: parseFloat(tLimit) || undefined
        })
      })
      tStdin = tStdout = tLimit = ''
      tWeight = '1'
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  async function uploadUnitTests() {
    if (!unittestFile) return
    const fd = new FormData()
    fd.append('file', unittestFile)
    try {
      await apiFetch(`/api/assignments/${id}/tests/upload`, { method: 'POST', body: fd })
      unittestFile = null
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  // ──────────────────────────────────────────────────────
  // Enhanced: Upload generated unittest code + adjust weights/limits
  // ──────────────────────────────────────────────────────
  function addUTTest() {
    utTests = [
      ...utTests,
      { name: `test_${utTests.length + 1}`, description: '', weight: '1', timeLimit: '1', assertions: [] }
    ]
  }

  function removeUTTest(idx: number) {
    utTests = utTests.filter((_, i) => i !== idx)
  }

  function addUTAssertion(idx: number, kind: UTAssertKind) {
    const t = utTests[idx]
    let a: UTAssertion
    switch (kind) {
      case 'equals':
      case 'notEquals':
      case 'contains':
      case 'notContains':
        a = { kind, args: [''], expected: '' }
        break
      case 'regex':
        a = { kind, args: [''], pattern: '' }
        break
      case 'raises':
        a = { kind, args: [''], exception: 'Exception' }
        break
      default:
        a = { kind: 'custom', code: '' }
    }
    t.assertions = [...t.assertions, a]
    utTests = [...utTests]
  }

  function removeUTAssertion(ti: number, ai: number) {
    utTests[ti].assertions = utTests[ti].assertions.filter((_, i) => i !== ai)
    utTests = [...utTests]
  }

  function generateUnittestCode(): string {
    const lines: string[] = []
    lines.push('import unittest')
    lines.push('')
    lines.push(`class ${utClassName}(unittest.TestCase):`)
    if (utSetup && utSetup.trim() !== '') {
      lines.push('    def setUp(self):')
      utSetup.split('\n').forEach((l) => lines.push('        ' + l))
      if (utSetup[utSetup.length - 1] !== '\n') lines.push('')
    }
    if (utTeardown && utTeardown.trim() !== '') {
      lines.push('    def tearDown(self):')
      utTeardown.split('\n').forEach((l) => lines.push('        ' + l))
      if (utTeardown[utTeardown.length - 1] !== '\n') lines.push('')
    }
    for (const t of utTests) {
      const methodName = t.name.startsWith('test_') ? t.name : `test_${t.name}`
      lines.push(`    def ${methodName}(self):`)
      if (t.description && t.description.trim() !== '') {
        lines.push(`        """${t.description}"""`)
      }
      if (t.assertions.length === 0) {
        lines.push('        self.assertTrue(True)')
        continue
      }
      for (const a of t.assertions) {
        if (a.kind === 'custom') {
          const code = (a as any).code || ''
          const cs = code.split('\n').map((l: string) => '        ' + l)
          lines.push(...cs)
          continue
        }
        const fmtArgs = (a as any).args?.map((s: string) => JSON.stringify(s)) ?? []
        const call = `student_code(${fmtArgs.join(', ')})`
        switch (a.kind) {
          case 'equals':
            lines.push(`        self.assertEqual(${call}, ${JSON.stringify((a as any).expected ?? '')})`)
            break
          case 'notEquals':
            lines.push(`        self.assertNotEqual(${call}, ${JSON.stringify((a as any).expected ?? '')})`)
            break
          case 'contains':
            lines.push(`        self.assertIn(${JSON.stringify((a as any).expected ?? '')}, ${call})`)
            break
          case 'notContains':
            lines.push(`        self.assertNotIn(${JSON.stringify((a as any).expected ?? '')}, ${call})`)
            break
          case 'regex':
            lines.push(`        self.assertRegex(${call}, r${JSON.stringify((a as any).pattern ?? '').replace(/^"|"$/g,'"')})`)
            break
          case 'raises':
            lines.push(`        with self.assertRaises(${(a as any).exception || 'Exception'}):`)
            lines.push(`            ${call}`)
            break
        }
      }
    }
    lines.push('')
    return lines.join('\n')
  }

  function refreshPreview() {
    utPreviewCode = generateUnittestCode()
  }

  async function uploadGeneratedUnitTests() {
    try {
      const code = generateUnittestCode()
      const blob = new Blob([code], { type: 'text/x-python' })
      const file = new File([blob], 'generated_tests.py', { type: 'text/x-python' })
      const fd = new FormData()
      fd.append('file', file)
      await apiFetch(`/api/assignments/${id}/tests/upload`, { method: 'POST', body: fd })
      await load()
      // After upload, adjust weights/time limits per created unittest method
      const nameToCfg = new Map<string, { weight: number; time: number }>()
      for (const t of utTests) {
        const methodName = t.name.startsWith('test_') ? t.name : `test_${t.name}`
        const fullName = `${utClassName}.${methodName}`
        const w = parseFloat(t.weight)
        const s = parseFloat(t.timeLimit)
        nameToCfg.set(fullName, { weight: isNaN(w) ? 1 : w, time: isNaN(s) ? 1 : s })
      }
      for (const t of tests) {
        if (t.unittest_name && nameToCfg.has(t.unittest_name)) {
          const cfg = nameToCfg.get(t.unittest_name)!
          await apiFetch(`/api/tests/${t.id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              stdin: t.stdin ?? '',
              expected_stdout: t.expected_stdout ?? '',
              weight: cfg.weight,
              time_limit_sec: cfg.time
            })
          })
        }
      }
      await load()
      utShowPreview = false
      err = ''
    } catch (e: any) {
      err = e.message
    }
  }

  async function delTest(tid: number) {
    if (!confirm('Delete this test?')) return
    try {
      await apiFetch(`/api/tests/${tid}`, { method: 'DELETE' })
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  async function updateTest(t: any) {
    try {
      await apiFetch(`/api/tests/${t.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          stdin: t.stdin,
          expected_stdout: t.expected_stdout,
          weight: parseFloat(t.weight) || 1,
          time_limit_sec: parseFloat(t.time_limit_sec) || undefined
        })
      })
      await load()
    } catch (e: any) {
      err = e.message
    }
  }
</script>

{#if role !== 'teacher' && role !== 'admin'}
  <div class="alert alert-error">
    <span>You do not have permission to manage tests.</span>
  </div>
{:else}
  {#if !assignment}
    <div class="flex items-center gap-3">
      <span class="loading loading-spinner loading-md"></span>
      <p>Loading…</p>
    </div>
  {:else}
    <div class="mb-4 flex items-center justify-between">
      <h1 class="text-2xl font-semibold">Manage tests — {assignment.title}</h1>
      <a class="btn" href={`/assignments/${id}`}>Back to assignment</a>
    </div>
    <div class="card-elevated p-6 space-y-6">
      <p class="text-sm opacity-70">Create IO tests or build Python unittest-based tests visually. You can also upload a `.py` file.</p>

      <!-- Tabs -->
      <div role="tablist" class="tabs tabs-lifted">
        <input type="radio" name="tests-tab" role="tab" class="tab" aria-label="Existing tests" checked>
        <div role="tabpanel" class="tab-content bg-base-100 border-base-300 rounded-box p-4">
          <div class="grid gap-3 max-h-[32rem] overflow-y-auto">
            {#each tests as t, i}
              <div class="rounded-xl border border-base-300/60 p-3 space-y-2">
                <div class="flex items-center justify-between">
                  <div class="font-semibold">Test {i + 1}
                    {#if t.unittest_name}
                      <span class="badge badge-outline ml-2">{t.unittest_name}</span>
                    {/if}
                  </div>
                  <div class="flex gap-2">
                    <button class="btn btn-xs" on:click={() => updateTest(t)}>Save</button>
                    <button class="btn btn-xs btn-error" on:click={() => delTest(t.id)}>Delete</button>
                  </div>
                </div>
                {#if !t.unittest_name}
                  <div class="grid sm:grid-cols-2 gap-2">
                    <label class="form-control w-full space-y-1">
                      <span class="label-text">Input</span>
                      <input class="input input-bordered w-full" placeholder="stdin" bind:value={t.stdin}>
                    </label>
                    <label class="form-control w-full space-y-1">
                      <span class="label-text">Expected output</span>
                      <input class="input input-bordered w-full" placeholder="expected stdout" bind:value={t.expected_stdout}>
                    </label>
                  </div>
                {:else}
                  {#if t.unittest_code}
                    <details class="mt-1">
                      <summary class="cursor-pointer text-sm opacity-70">View unittest code</summary>
                      <pre class="mt-2 whitespace-pre-wrap text-xs opacity-80">{t.unittest_code}</pre>
                    </details>
                  {/if}
                {/if}
                <div class="grid sm:grid-cols-2 gap-2">
                  <label class="form-control w-full space-y-1">
                    <span class="label-text">Time limit (s)</span>
                    <input class="input input-bordered w-full" placeholder="seconds" bind:value={t.time_limit_sec}>
                  </label>
                  <label class="form-control w-full space-y-1">
                    <span class="label-text">Weight</span>
                    <input class="input input-bordered w-full" placeholder="points" bind:value={t.weight}>
                  </label>
                </div>
              </div>
            {/each}
            {#if !(tests && tests.length)}<p><i>No tests</i></p>{/if}
          </div>
        </div>

        <input type="radio" name="tests-tab" role="tab" class="tab" aria-label="Add IO test">
        <div role="tabpanel" class="tab-content bg-base-100 border-base-300 rounded-box p-4">
          <div class="border-base-300/60 space-y-2">
            <h4 class="font-semibold">Add IO test</h4>
            <div class="grid sm:grid-cols-2 gap-2">
              <label class="form-control w-full space-y-1">
                <span class="label-text">Input</span>
                <input class="input input-bordered w-full" placeholder="stdin" bind:value={tStdin}>
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text">Expected output</span>
                <input class="input input-bordered w-full" placeholder="expected stdout" bind:value={tStdout}>
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text">Time limit (s)</span>
                <input class="input input-bordered w-full" placeholder="seconds" bind:value={tLimit}>
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text">Weight</span>
                <input class="input input-bordered w-full" placeholder="points" bind:value={tWeight}>
              </label>
            </div>
            <div>
              <button class="btn btn-primary" on:click={addTest} disabled={!tStdin || !tStdout}>Add</button>
            </div>
          </div>
        </div>

        <input type="radio" name="tests-tab" role="tab" class="tab" aria-label="Unittest builder">
        <div role="tabpanel" class="tab-content bg-base-100 border-base-300 rounded-box p-4 space-y-4">
          <div class="grid sm:grid-cols-2 gap-3">
            <label class="form-control w-full space-y-1">
              <span class="label-text">Class name</span>
              <input class="input input-bordered w-full" bind:value={utClassName} placeholder="TestAssignment">
            </label>
            <div class="flex items-end gap-2">
              <button class="btn btn-outline" on:click={addUTTest}>Add test method</button>
              <button class="btn" on:click={() => { utShowPreview = !utShowPreview; refreshPreview() }}>{utShowPreview ? 'Hide' : 'Preview'} code</button>
              <button class="btn btn-primary" disabled={utTests.length === 0} on:click={uploadGeneratedUnitTests}>Create tests</button>
            </div>
          </div>

          <div class="grid sm:grid-cols-2 gap-3">
            <label class="form-control w-full space-y-1">
              <span class="label-text">setUp() (optional)</span>
              <textarea class="textarea textarea-bordered h-28" bind:value={utSetup} placeholder="# Python code to run before each test"></textarea>
            </label>
            <label class="form-control w-full space-y-1">
              <span class="label-text">tearDown() (optional)</span>
              <textarea class="textarea textarea-bordered h-28" bind:value={utTeardown} placeholder="# Python code to run after each test"></textarea>
            </label>
          </div>

          <div class="space-y-3">
            {#each utTests as ut, ti}
              <div class="rounded-xl border border-base-300/60 p-3 space-y-2">
                <div class="flex items-center justify-between gap-3">
                  <div class="grid sm:grid-cols-2 gap-2 flex-1">
                    <label class="form-control w-full space-y-1">
                      <span class="label-text">Method name</span>
                      <input class="input input-bordered w-full" bind:value={ut.name} placeholder="test_something">
                    </label>
                    <label class="form-control w-full space-y-1">
                      <span class="label-text">Description</span>
                      <input class="input input-bordered w-full" bind:value={ut.description} placeholder="What this test checks">
                    </label>
                  </div>
                  <div class="flex items-end gap-2">
                    <button class="btn btn-ghost btn-xs" on:click={() => removeUTTest(ti)}>Remove</button>
                  </div>
                </div>
                <div class="grid sm:grid-cols-2 gap-2">
                  <label class="form-control w-full space-y-1">
                    <span class="label-text">Time limit (s)</span>
                    <input class="input input-bordered w-full" bind:value={ut.timeLimit}>
                  </label>
                  <label class="form-control w-full space-y-1">
                    <span class="label-text">Weight</span>
                    <input class="input input-bordered w-full" bind:value={ut.weight}>
                  </label>
                </div>
                <div class="space-y-2">
                  <div class="flex items-center justify-between">
                    <div class="font-medium">Assertions</div>
                    <div class="join">
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'equals')}>Equals</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'notEquals')}>Not equals</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'contains')}>Contains</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'notContains')}>Not contains</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'regex')}>Regex</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'raises')}>Raises</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'custom')}>Custom</button>
                    </div>
                  </div>
                  <div class="space-y-2">
                    {#each ut.assertions as a, ai}
                      <div class="rounded-lg border border-base-300/60 p-2 space-y-2">
                        <div class="flex items-center justify-between">
                          <span class="badge badge-outline">{a.kind}</span>
                          <button class="btn btn-ghost btn-xs" on:click={() => removeUTAssertion(ti, ai)}>Remove</button>
                        </div>
                        {#if a.kind === 'custom'}
                          <label class="form-control w-full space-y-1">
                            <span class="label-text">Custom Python (inside test method)</span>
                            <textarea class="textarea textarea-bordered h-24" value={getCustom(a)} on:input={(e) => setCustom(a, (e.target as HTMLTextAreaElement).value)} placeholder="self.assertTrue(...)"></textarea>
                          </label>
                        {:else if a.kind === 'regex'}
                          <div class="grid sm:grid-cols-2 gap-2">
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Inputs (one per line)</span>
                              <textarea class="textarea textarea-bordered h-24" value={getInputs(a)} on:input={(e) => setInputs(a, (e.target as HTMLTextAreaElement).value)} placeholder="2\n3"></textarea>
                            </label>
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Regex pattern</span>
                              <input class="input input-bordered w-full" value={getPattern(a)} on:input={(e) => setPattern(a, (e.target as HTMLInputElement).value)} placeholder="^5$">
                            </label>
                          </div>
                        {:else if a.kind === 'raises'}
                          <div class="grid sm:grid-cols-2 gap-2">
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Inputs (one per line)</span>
                              <textarea class="textarea textarea-bordered h-24" value={getInputs(a)} on:input={(e) => setInputs(a, (e.target as HTMLTextAreaElement).value)} placeholder="bad\ninput"></textarea>
                            </label>
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Exception type</span>
                              <input class="input input-bordered w-full" value={getException(a)} on:input={(e) => setException(a, (e.target as HTMLInputElement).value)} placeholder="ValueError">
                            </label>
                          </div>
                        {:else}
                          <div class="grid sm:grid-cols-2 gap-2">
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Inputs (one per line)</span>
                              <textarea class="textarea textarea-bordered h-24" value={getInputs(a)} on:input={(e) => setInputs(a, (e.target as HTMLTextAreaElement).value)} placeholder="2\n3"></textarea>
                            </label>
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Expected</span>
                              <input class="input input-bordered w-full" value={getExpected(a)} on:input={(e) => setExpected(a, (e.target as HTMLInputElement).value)} placeholder="5">
                            </label>
                          </div>
                        {/if}
                      </div>
                    {/each}
                    {#if ut.assertions.length === 0}
                      <p class="text-sm opacity-70">Add assertions to this test.</p>
                    {/if}
                  </div>
                </div>
              </div>
            {/each}
          </div>

          {#if utShowPreview}
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <h4 class="font-semibold">Generated Python</h4>
                <button class="btn btn-sm" on:click={refreshPreview}>Refresh</button>
              </div>
              <CodeMirror bind:value={utPreviewCode} lang={null} readOnly={true} />
            </div>
          {/if}
        </div>

        <input type="radio" name="tests-tab" role="tab" class="tab" aria-label="Upload .py">
        <div role="tabpanel" class="tab-content bg-base-100 border-base-300 rounded-box p-4">
          <h4 class="font-semibold mb-2">Upload unittest file</h4>
          <input type="file" accept=".py" class="file-input file-input-bordered w-full" on:change={(e) => (unittestFile = (e.target as HTMLInputElement).files?.[0] || null)}>
          <div class="mt-2">
            <button class="btn" on:click={uploadUnitTests} disabled={!unittestFile}>Upload</button>
          </div>
          <p class="text-xs opacity-70 mt-2">Each method named <code>test_*</code> in classes derived from <code>unittest.TestCase</code> will become a separate test. Use <code>student_code(...)</code> to run the student's program with inputs.</p>
        </div>
      </div>
    </div>
  {/if}
{/if}

{#if err}
  <div class="alert alert-error mt-4"><span>{err}</span></div>
{/if}

<style>
  :global(.card-elevated){
    border-radius: 1rem;
    border: 1px solid color-mix(in oklab, currentColor 20%, transparent);
    background: var(--fallback-b1, oklch(var(--b1)));
    box-shadow: 0 8px 24px rgba(0,0,0,.06);
  }
</style>



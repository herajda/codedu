<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { apiJSON, apiFetch } from '$lib/api'
  import { createEventSource } from '$lib/sse'
  import { page } from '$app/stores'
  import JSZip from 'jszip'
  import { FileTree, RunConsole } from '$lib'
  import { formatDateTime } from '$lib/date'
  import { goto } from '$app/navigation'
  import { auth } from '$lib/auth'
  import { extractMethodFromUnittest, stripUnittestMainBlock } from '$lib/unittests'

$: id = $page.params.id

  let submission:any=null
  let results:any[]=[]
  let err = ''
  let files: { name: string; content: string }[] = []
  let tree: FileNode[] = []
  let selected: { name: string; content: string } | null = null
  let highlighted = ''
  let manualConsoleVisible = false
  let esCtrl: { close: () => void } | null = null
  let assignmentTitle: string = ''
  let assignmentManual: boolean = false
  let assignmentTestsCount: number = 0
  let assignmentLLMInteractive: boolean = false
  let assignmentLLMFeedback: boolean = false
  let assignmentShowTestDetails = false
  let assignmentShowTraceback = false
  let sid: number = 0
  let role = ''
  $: role = $auth?.role ?? ''

  import hljs from 'highlight.js'
  import 'highlight.js/styles/github.css'
  let fileDialog: HTMLDialogElement

  let llm: any = null
  // Derived visibility flags

  // Inline teacher points override component
  // This is a tiny Svelte component defined in-file using a function that returns markup via a slot approach
  // Svelte does not support runtime component definitions; instead use a block here:
  let overrideValue: string | number | null = ''
  let savingOverride = false
  async function saveOverride(){
    try{
      savingOverride = true
      const raw: any = overrideValue
      const str = raw == null ? '' : (typeof raw === 'string' ? raw : String(raw))
      const v = str.trim() === '' ? null : parseInt(str, 10)
      await apiFetch(`/api/submissions/${submission.id}/points`,{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body: JSON.stringify({ points: v })
      })
      await load()
    }catch(e:any){ err = e.message }
    finally{ savingOverride = false }
  }

  interface FileNode {
    name: string
    content?: string
    children?: FileNode[]
  }

  async function parseFiles(b64: string) {
    let bytes: Uint8Array
    try {
      const bin = atob(b64)
      bytes = new Uint8Array(bin.length)
      for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
    } catch {
      return [{ name: 'code', content: b64 }]
    }

    try {
      const zip = await JSZip.loadAsync(bytes)
      const list: { name: string; content: string }[] = []
      for (const file of Object.values(zip.files)) {
        if (file.dir) continue
        const content = await file.async('string')
        list.push({ name: file.name, content })
      }
      return list
    } catch {
      const text = new TextDecoder().decode(bytes)
      return [{ name: 'code', content: text }]
    }
  }

  async function load() {
    err = ''
    try {
      const data = await apiJSON(`/api/submissions/${id}`)
      submission = data.submission
      results = data.results
      llm = data.llm ?? null

      // Prefill override input with the currently assigned points (teacher sees what's set)
      try {
        const cur = (submission?.override_points ?? submission?.points)
        overrideValue = (cur ?? '') as any
      } catch {}

      files = await parseFiles(submission.code_content)
      tree = buildTree(files)
      selected = files[0]

      if (submission?.assignment_id) {
        try {
          const ad = await apiJSON(`/api/assignments/${submission.assignment_id}`)
          assignmentTitle = ad.assignment?.title ?? ''
          assignmentManual = !!ad.assignment?.manual_review
          assignmentLLMInteractive = !!ad.assignment?.llm_interactive
          assignmentLLMFeedback = !!ad.assignment?.llm_feedback
          assignmentShowTestDetails = !!ad.assignment?.show_test_details
          assignmentShowTraceback = !!ad.assignment?.show_traceback
          // Prefer aggregate tests_count when present (student view), fallback to tests array (teacher/admin)
          try {
            assignmentTestsCount = typeof ad.tests_count === 'number'
              ? ad.tests_count
              : (Array.isArray(ad.tests) ? ad.tests.length : 0)
          } catch {
            assignmentTestsCount = 0
          }
        } catch {}
      }
    } catch (e: any) {
      err = e.message
    }
  }

  function buildTree(list: { name: string; content: string }[]): FileNode[] {
    const root: FileNode = { name: '', children: [] }
    for (const f of list) {
      const parts = f.name.split('/')
      let node = root
      for (let i = 0; i < parts.length; i++) {
        const part = parts[i]
        if (!node.children) node.children = []
        let child = node.children.find((c) => c.name === part)
        if (!child) {
          child = { name: part }
          node.children.push(child)
        }
        node = child
        if (i === parts.length - 1) {
          node.content = f.content
          node.children = undefined
        }
      }
    }
    return root.children ?? []
  }

  function statusColor(s:string){
    if(s==='completed') return 'badge-success'
    if(s==='running') return 'badge-info'
    if(s==='failed') return 'badge-error'
    if(s==='passed') return 'badge-success'
    if(s==='wrong_output') return 'badge-error'
    if(s==='runtime_error') return 'badge-error'
    if(s==='time_limit_exceeded' || s==='memory_limit_exceeded') return 'badge-warning'
    return ''
  }

  function resultColor(s: string){
    if(s === 'passed') return 'badge-success'
    if(s === 'wrong_output') return 'badge-error'
    if(s === 'runtime_error') return 'badge-error'
    if(s === 'time_limit_exceeded' || s==='memory_limit_exceeded') return 'badge-warning'
    return ''
  }

  // Show LLM block when assignment uses LLM-interactive
  $: showLLM = assignmentLLMInteractive
  // Allow detailed LLM artifacts for students only if teacher enabled feedback
  $: allowLLMDetails = (role !== 'student') || assignmentLLMFeedback
  $: allowTestDetails = role !== 'student' || assignmentShowTestDetails
  $: allowTraceback = role !== 'student' || assignmentShowTraceback
  // Show Auto-tests only when NOT LLM mode and there are tests configured
  $: showAutoUI = (!assignmentLLMInteractive) && (assignmentTestsCount > 0)
  // Keep legacy meaning of hideAutoUI: specifically, when no auto tests exist
  $: hideAutoUI = assignmentTestsCount === 0
  $: forceManualConsole = assignmentManual || hideAutoUI
  $: if (forceManualConsole) manualConsoleVisible = true

  function bgFromBadge(badgeClass: string){
    return badgeClass.replace('badge','bg')
  }

  $: totalTests = results?.length ?? 0
  $: passedCount = results.filter((r)=> r.status==='passed').length
  $: failedCount = results.filter((r)=> ['wrong_output','runtime_error','failed'].includes(r.status)).length
  $: warnedCount = results.filter((r)=> ['time_limit_exceeded','memory_limit_exceeded'].includes(r.status)).length

  // ----- LLM UI helpers -----
  function safeParseJSON(raw: any): any {
    try {
      if (!raw || typeof raw !== 'string') return null
      return JSON.parse(raw)
    } catch {
      return null
    }
  }

  function viewableUnitTestSnippet(code: string | null | undefined, name: string | null | undefined): string {
    if (code == null) return ''
    const sanitized = stripUnittestMainBlock(String(code))
    if (!name) return sanitized
    const snippet = extractMethodFromUnittest(String(code), String(name))
    return snippet.trim().length ? snippet : sanitized
  }

  // Parsed review JSON (typed in backend as Review)
  $: review = safeParseJSON(llm?.review_json)

  // Transcript lines styled as chat bubbles
  type TranscriptMsg = { role: 'AI' | 'Program' | 'Other'; text: string }
  $: transcriptMsgs = (() => {
    const t = llm?.transcript
    if (!t || typeof t !== 'string') return [] as TranscriptMsg[]
    return t.split('\n')
      .map((s: string) => s.trim())
      .filter((s: string) => s.length > 0)
      .map((line: string): TranscriptMsg => {
        if (line.startsWith('AI> ')) return { role: 'AI', text: line.slice(4) }
        if (line.startsWith('PROGRAM> ')) return { role: 'Program', text: line.slice(9) }
        return { role: 'Other', text: line }
      })
  })()

  function openFiles() {
    if (files.length) {
      selected = files[0]
      fileDialog.showModal()
    }
  }

  async function downloadFiles() {
    try {
      if (Array.isArray(files) && files.length) {
        const zip = new JSZip()
        for (const f of files) {
          zip.file(f.name, f.content ?? '')
        }
        const blob = await zip.generateAsync({ type: 'blob' })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        const safeTitle = (assignmentTitle || 'submission').replace(/[^a-z0-9_\-]+/gi, '_').slice(0, 60)
        a.href = url
        a.download = `${safeTitle}_${submission?.id ?? id}.zip`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(url)
      } else {
        const textContent = submission?.code_content ?? ''
        const blob = new Blob([textContent], { type: 'text/plain;charset=utf-8' })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `submission_${submission?.id ?? id}.txt`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(url)
      }
    } catch (e: any) {
      err = e?.message ?? String(e)
    }
  }

  function goBack(){
    try {
      if (typeof window !== 'undefined' && window.history.length > 1) {
        window.history.back()
        return
      }
    } catch {}
    const fromTab = $page?.url?.searchParams?.get('fromTab')
    if (submission?.assignment_id) {
      const tabPart = fromTab ? `?tab=${fromTab}` : ''
      goto(`/assignments/${submission.assignment_id}${tabPart}`)
    } else {
      goto('/submissions')
    }
  }

  function chooseFile(n: FileNode) {
    if (n.content) {
      selected = { name: n.name, content: n.content }
    }
  }

  $: if (selected) {
    highlighted = hljs.highlightAuto(selected.content).value
  }

  $: if (!selected && submission) {
    highlighted = hljs.highlightAuto(submission.code_content).value
  }

  onMount(() => {
    load()
    esCtrl = createEventSource(
      '/api/events',
      (src) => {
    src.addEventListener('status', (ev) => {
      const d = JSON.parse((ev as MessageEvent).data)
      if (submission && d.submission_id === submission.id) {
        submission.status = d.status
        if (d.status !== 'running') load()
      }
    })
    src.addEventListener('result', (ev) => {
      const d = JSON.parse((ev as MessageEvent).data)
      if (submission && d.submission_id === submission.id) {
        results = [...results, d]
      }
    })
      },
      {
        onError: (m) => { err = m },
        onOpen: () => { err = '' }
      }
    )
  })
  onDestroy(() => { esCtrl?.close() })
  $: sid = submission?.id ?? id
</script>

{#if !submission}
  <div class="flex justify-center py-10"><span class="loading loading-dots loading-lg"></span></div>
{:else}
  <div class="space-y-6">
    <div class="card bg-base-100 shadow">
      <div class="card-body space-y-4">
        <div class="flex items-start justify-between gap-6">
          <div class="space-y-2">
            <h1 class="card-title text-2xl">{assignmentTitle || 'Assignment'}</h1>
            <div class="flex flex-wrap gap-2 text-xs sm:text-sm opacity-80">
              <span class="inline-flex items-center gap-2 px-3 py-1 rounded bg-base-200">Attempt #{submission.attempt_number ?? submission.id}</span>
              <span class="inline-flex items-center gap-2 px-3 py-1 rounded bg-base-200">Submitted {formatDateTime(submission.created_at)}</span>
              {#if assignmentManual}
                <span class="inline-flex items-center gap-2 px-3 py-1 rounded bg-info/20 text-info">Manual review</span>
              {/if}
            </div>
          </div>
          <div class="flex items-center gap-2">
            <span class={`badge badge-lg ${statusColor(submission.status)}`}>{submission.status}</span>
            {#if submission.manually_accepted}
              <span class="badge badge-outline badge-success" title="Accepted by teacher">Manually accepted</span>
            {/if}
            <button class="btn btn-ghost" on:click={goBack}>Back</button>
            <button class="btn btn-primary" on:click={openFiles}>View files</button>
          </div>
        </div>

        <!-- Tabs removed: show only the relevant block based on assignment settings -->

        {#if (role==='teacher' || role==='admin')}
          <div class="rounded-box bg-base-200 p-4 mt-2">
            <div class="font-medium mb-2">Teacher actions</div>
            
            {#if submission?.manually_accepted}
              <!-- Show undo button for manually accepted submissions -->
              <div class="flex items-center gap-3">
                <button class="btn btn-warning btn-sm" on:click={async()=>{ 
                  try{ 
                    await apiFetch(`/api/submissions/${submission.id}/undo-accept`, { 
                      method:'PUT', 
                      headers:{'Content-Type':'application/json'}, 
                      body: JSON.stringify({}) 
                    }); 
                    await load(); 
                  }catch(e:any){ 
                    err=e.message 
                  } 
                }}>Undo manual acceptance</button>
              </div>
              <div class="text-xs opacity-70 mt-2">This submission was manually accepted. Click undo to allow re-grading.</div>
            {:else}
              <!-- Show accept/give points options for non-manually accepted submissions -->
              {#if submission?.status === 'failed' || submission?.status === 'pending'}
                <!-- Show expandable section for failed/pending submissions -->
                <div class="collapse collapse-arrow bg-base-100">
                  <input type="checkbox" />
                  <div class="collapse-title font-medium">
                    Accept submission and give points
                  </div>
                  <div class="collapse-content">
                    <div class="flex items-center gap-3 mt-3">
                      <input type="number" step="1" min="0" inputmode="numeric" pattern="[0-9]*" on:keydown={(e) => { if (['e','E','+','-','.',','].includes(e.key)) e.preventDefault() }} class="input input-bordered input-sm w-28 sm:w-32" bind:value={overrideValue} placeholder="points (optional)" aria-label="Override points">
                      <button class={`btn btn-primary btn-sm ${savingOverride ? 'loading' : ''}`} on:click={saveOverride} disabled={savingOverride}>Save points</button>
                      <button class="btn btn-success btn-sm" on:click={async()=>{ try{ const raw:any=overrideValue; const str = raw==null? '' : (typeof raw==='string'? raw : String(raw)); const v = str.trim()===''? null : parseInt(str,10); await apiFetch(`/api/submissions/${submission.id}/accept`, { method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({ points: v })}); await load(); }catch(e:any){ err=e.message } }}>Accept submission</button>
                    </div>
                    <div class="text-xs opacity-70 mt-2">Leave points empty to accept without overriding. Acceptance marks the submission completed and visible to the student as manually accepted.</div>
                  </div>
                </div>
              {:else}
                <!-- Show regular accept/give points for other statuses -->
                <div class="flex items-center gap-3">
                  <input type="number" step="1" min="0" inputmode="numeric" pattern="[0-9]*" on:keydown={(e) => { if (['e','E','+','-','.',','].includes(e.key)) e.preventDefault() }} class="input input-bordered input-sm w-28 sm:w-32" bind:value={overrideValue} placeholder="points (optional)" aria-label="Override points">
                  <button class={`btn btn-primary btn-sm ${savingOverride ? 'loading' : ''}`} on:click={saveOverride} disabled={savingOverride}>Save points</button>
                  <button class="btn btn-success btn-sm" on:click={async()=>{ try{ const raw:any=overrideValue; const str = raw==null? '' : (typeof raw==='string'? raw : String(raw)); const v = str.trim()===''? null : parseInt(str,10); await apiFetch(`/api/submissions/${submission.id}/accept`, { method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({ points: v })}); await load(); }catch(e:any){ err=e.message } }}>Accept submission</button>
                </div>
                <div class="text-xs opacity-70 mt-2">Leave points empty to accept without overriding. Acceptance marks the submission completed and visible to the student as manually accepted.</div>
              {/if}
            {/if}
          </div>
        {/if}

        {#if showAutoUI}
        <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
          <div class="stat bg-base-200 rounded-box">
            <div class="stat-title">Tests</div>
            <div class="stat-value text-base">{totalTests}</div>
          </div>
          <div class="stat bg-base-200 rounded-box">
            <div class="stat-title">Passed</div>
            <div class="stat-value text-success">{passedCount}</div>
          </div>
          <div class="stat bg-base-200 rounded-box">
            <div class="stat-title">Limited</div>
            <div class="stat-value text-warning">{warnedCount}</div>
          </div>
          <div class="stat bg-base-200 rounded-box">
            <div class="stat-title">Failed</div>
            <div class="stat-value text-error">{failedCount}</div>
          </div>
        </div>
        <div class="h-2 w-full rounded bg-base-200 overflow-hidden flex">
          {#each results as r}
            <div class={`h-full flex-1 ${bgFromBadge(resultColor(r.status))}`}></div>
          {/each}
          {#if !results.length}
            <div class="h-full w-1/3 bg-info animate-pulse"></div>
          {/if}
        </div>
        {/if}
      </div>
    </div>

    {#if showAutoUI}
      <div class="card bg-base-100 shadow">
        <div class="card-body">
          <h3 class="card-title">Results</h3>
          {#if Array.isArray(results) && results.length}
            <div class="space-y-2">
              {#each results as r, i}
                {@const mode = r.execution_mode ?? (r.unittest_name ? 'unittest' : r.function_name ? 'function' : 'stdin_stdout')}
                {#if allowTestDetails || allowTraceback}
                  <details class="collapse collapse-arrow rounded-box bg-base-200">
                    <summary class="collapse-title">
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                        <div class="flex flex-wrap items-center gap-2 text-sm sm:text-base font-medium">
                          <span class="rounded-full bg-base-100 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-base-content/70">
                            Test {r.test_number ?? i + 1}
                          </span>
                          {#if allowTestDetails && mode === 'function'}
                            <span class="badge badge-outline badge-info text-xs font-semibold tracking-wide uppercase">Function</span>
                            {#if r.function_name}
                              <span class="badge badge-outline text-xs font-semibold tracking-wide">{r.function_name}</span>
                            {/if}
                          {:else if allowTestDetails && r.unittest_name}
                            <span class="badge badge-outline badge-primary text-xs font-semibold tracking-wide uppercase">{r.unittest_name}</span>
                          {:else if allowTestDetails && (typeof r.stdin !== 'undefined' || typeof r.expected_stdout !== 'undefined')}
                            <span class="badge badge-outline text-xs font-semibold tracking-wide uppercase">I/O test</span>
                          {/if}
                        </div>
                        <div class="flex items-center flex-wrap gap-3 text-xs sm:text-sm">
                          <span class={`badge ${statusColor(r.status)}`}>{r.status}</span>
                          <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-base-300">
                            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                              <circle cx="12" cy="12" r="9"/>
                              <path d="M12 7v5l3 2"/>
                            </svg>
                            {r.runtime_ms} ms
                          </span>
                          <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-base-300">
                            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                              <rect x="3" y="5" width="18" height="14" rx="2" ry="2"/>
                              <path d="M7 9l3 3-3 3"/>
                              <path d="M13 15h4"/>
                            </svg>
                            exit {r.exit_code}
                          </span>
                        </div>
                      </div>
                    </summary>
                    <div class="collapse-content space-y-4">
                      {#if allowTestDetails}
                        <section class="rounded-2xl border border-base-300/70 bg-base-100 p-4 shadow-sm">
                          <header class="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-base-content/70">
                            <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                              <rect x="3" y="4" width="18" height="16" rx="2" ry="2"/>
                              <path d="M7 8h10"/>
                            </svg>
                            Test definition
                          </header>
                          {#if mode === 'function'}
                            <div class="grid gap-3 md:grid-cols-2">
                              <div class="rounded-xl border border-base-300/60 bg-base-200/70 p-3">
                                <div class="text-xs font-semibold uppercase tracking-wide text-base-content/70">Function</div>
                                <div class="mt-2 font-mono text-sm">{r.function_name}</div>
                              </div>
                              <div class="rounded-xl border border-base-300/60 bg-base-200/70 p-3">
                                <div class="text-xs font-semibold uppercase tracking-wide text-base-content/70">Expected return</div>
                                <pre class="mt-2 whitespace-pre-wrap font-mono text-sm leading-relaxed">{r.expected_return ?? '∅'}</pre>
                              </div>
                              <div class="rounded-xl border border-base-300/60 bg-base-200/70 p-3">
                                <div class="text-xs font-semibold uppercase tracking-wide text-base-content/70">Arguments</div>
                                <pre class="mt-2 whitespace-pre-wrap font-mono text-sm leading-relaxed">{r.function_args ?? '[]'}</pre>
                              </div>
                              <div class="rounded-xl border border-base-300/60 bg-base-200/70 p-3">
                                <div class="text-xs font-semibold uppercase tracking-wide text-base-content/70">Keyword args</div>
                                <pre class="mt-2 whitespace-pre-wrap font-mono text-sm leading-relaxed">{r.function_kwargs ?? '{}'}</pre>
                              </div>
                              {#if r.actual_return}
                                <div class="rounded-xl border border-base-300/60 bg-base-200/70 p-3 md:col-span-2">
                                  <div class="text-xs font-semibold uppercase tracking-wide text-base-content/70">Actual return</div>
                                  <pre class="mt-2 whitespace-pre-wrap font-mono text-sm leading-relaxed">{r.actual_return}</pre>
                                </div>
                              {/if}
                            </div>
                          {:else if r.unittest_code}
                            {#if r.unittest_name}
                              <div class="badge badge-outline badge-primary mb-3">{r.unittest_name}</div>
                            {/if}
                            <pre class="max-h-80 overflow-auto rounded-xl bg-base-200/80 p-4 text-sm leading-relaxed"><code class="font-mono whitespace-pre-wrap">{viewableUnitTestSnippet(r.unittest_code, r.unittest_name)}</code></pre>
                          {:else if typeof r.stdin !== 'undefined' || typeof r.expected_stdout !== 'undefined'}
                            <div class="grid gap-3 md:grid-cols-2">
                              <div class="rounded-xl border border-base-300/60 bg-base-200/70 p-3">
                                <div class="text-xs font-semibold uppercase tracking-wide text-base-content/70">Input</div>
                                {#if typeof r.stdin !== 'undefined'}
                                  {#if r.stdin?.length}
                                    <pre class="mt-2 whitespace-pre-wrap font-mono text-sm leading-relaxed">{r.stdin}</pre>
                                  {:else}
                                    <div class="mt-2 text-sm italic opacity-60">Empty input</div>
                                  {/if}
                                {:else}
                                  <div class="mt-2 text-sm italic opacity-60">Not provided</div>
                                {/if}
                              </div>
                              <div class="rounded-xl border border-base-300/60 bg-base-200/70 p-3">
                                <div class="text-xs font-semibold uppercase tracking-wide text-base-content/70">Expected output</div>
                                {#if typeof r.expected_stdout !== 'undefined'}
                                  {#if r.expected_stdout?.length}
                                    <pre class="mt-2 whitespace-pre-wrap font-mono text-sm leading-relaxed">{r.expected_stdout}</pre>
                                  {:else}
                                    <div class="mt-2 text-sm italic opacity-60">Empty output</div>
                                  {/if}
                                {:else}
                                  <div class="mt-2 text-sm italic opacity-60">Not provided</div>
                                {/if}
                              </div>
                            </div>
                          {:else}
                            <div class="text-sm opacity-70">No metadata available for this test.</div>
                          {/if}
                        </section>
                      {/if}
                      {#if allowTraceback}
                        <section class="rounded-2xl border border-base-300/70 bg-base-100 p-4 shadow-sm">
                          <header class="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-base-content/70">
                            <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                              <path d="M4 17l6-6 4 4 6-6"/>
                              <path d="M2 19h20"/>
                            </svg>
                            Execution log
                          </header>
                          {#if r.stderr}
                            <pre class="max-h-80 overflow-auto whitespace-pre-wrap rounded-xl bg-base-200/80 p-4 text-sm leading-relaxed">{r.stderr}</pre>
                          {:else}
                            <div class="text-sm italic opacity-60">No stderr output</div>
                          {/if}
                        </section>
                      {/if}
                    </div>
                  </details>
                {:else}
                  <div class="collapse rounded-box bg-base-200">
                    <div class="collapse-title">
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                        <div class="flex flex-wrap items-center gap-2 text-sm sm:text-base font-medium">
                          <span class="rounded-full bg-base-100 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-base-content/70">
                            Test {r.test_number ?? i + 1}
                          </span>
                        </div>
                        <div class="flex items-center flex-wrap gap-3 text-xs sm:text-sm">
                          <span class={`badge ${statusColor(r.status)}`}>{r.status}</span>
                          <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-base-300">
                            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                              <circle cx="12" cy="12" r="9"/>
                              <path d="M12 7v5l3 2"/>
                            </svg>
                            {r.runtime_ms} ms
                          </span>
                          <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-base-300">
                            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                              <rect x="3" y="5" width="18" height="14" rx="2" ry="2"/>
                              <path d="M7 9l3 3-3 3"/>
                              <path d="M13 15h4"/>
                            </svg>
                            exit {r.exit_code}
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>
                {/if}
              {/each}
            </div>
          {:else}
            <div class="text-sm opacity-70 italic">No results yet. This submission may still be running.</div>
          {/if}
        </div>
      </div>
    {/if}

    {#if showLLM}
      <div class="card bg-base-100 shadow">
        <div class="card-body space-y-4">
          <h3 class="card-title">LLM-Interactive</h3>
          {#if llm}
            <div class="grid md:grid-cols-3 gap-3">
              <div class="rounded-box bg-base-200 p-3">
                <div class="font-medium">Smoke test</div>
                <div class="text-sm">{llm.smoke_ok ? 'OK' : 'Failed'}</div>
              </div>
              <div class="rounded-box bg-base-200 p-3">
                <div class="font-medium">Verdict</div>
                <div class="text-sm">{llm.verdict ?? '-'}</div>
              </div>
              <div class="rounded-box bg-base-200 p-3">
                <div class="font-medium">Reason</div>
                <div class="text-sm break-words">{llm.reason ?? '-'}</div>
              </div>
            </div>

            {#if review && allowLLMDetails}
              <div class="rounded-box bg-base-200 p-4 space-y-3">
                <div class="font-semibold flex items-center gap-2">LLM Review</div>
                {#if review.summary}
                  <p class="text-sm leading-relaxed">{review.summary}</p>
                {/if}

                {#if Array.isArray(review.issues) && review.issues.length}
                  <div class="space-y-2">
                    <div class="font-medium">Issues</div>
                    <div class="space-y-2">
                      {#each review.issues as issue}
                        <div class="rounded bg-base-300 p-3 space-y-1">
                          <div class="flex items-center justify-between">
                            <div class="font-medium">{issue.title}</div>
                            <span class={`badge ${issue.severity==='critical' ? 'badge-error' : issue.severity==='high' ? 'badge-warning' : 'badge-info'}`}>{issue.severity}</span>
                          </div>
                          {#if issue.rationale}
                            <div class="text-sm opacity-80">{issue.rationale}</div>
                          {/if}
                          {#if issue.reproduction}
                            <div class="text-sm">
                              <div class="opacity-70">Reproduction</div>
                              {#if Array.isArray(issue.reproduction.inputs) && issue.reproduction.inputs.length}
                                <ul class="list-disc list-inside">
                                  {#each issue.reproduction.inputs as inp}
                                    <li class="font-mono">{inp}</li>
                                  {/each}
                                </ul>
                              {/if}
                              {#if issue.reproduction.expect_regex}
                                <div class="mt-1">Expect: <span class="font-mono">/{issue.reproduction.expect_regex}/</span></div>
                              {/if}
                              {#if issue.reproduction.notes}
                                <div class="mt-1 opacity-80">{issue.reproduction.notes}</div>
                              {/if}
                            </div>
                          {/if}
                        </div>
                      {/each}
                    </div>
                  </div>
                {/if}

                {#if Array.isArray(review.suggestions) && review.suggestions.length}
                  <div class="space-y-1">
                    <div class="font-medium">Suggestions</div>
                    <ul class="list-disc list-inside">
                      {#each review.suggestions as s}
                        <li>{s}</li>
                      {/each}
                    </ul>
                  </div>
                {/if}

                <!-- Risk-based tests plan removed per requirements -->

                {#if review.acceptance}
                  <div class="pt-1">
                    <div class="font-medium">Acceptance</div>
                    <div class="flex items-center gap-2 text-sm">
                      <span class={`badge ${review.acceptance.ok ? 'badge-success' : 'badge-error'}`}>{review.acceptance.ok ? 'Accepted' : 'Rejected'}</span>
                      {#if review.acceptance.reason}
                        <span class="opacity-80">{review.acceptance.reason}</span>
                      {/if}
                    </div>
                  </div>
                {/if}
              </div>
            {/if}

            {#if transcriptMsgs.length && allowLLMDetails}
              <div class="rounded-box bg-base-200 p-4 space-y-2">
                <div class="font-semibold">Interactive transcript</div>
                <div class="space-y-2">
                  {#each transcriptMsgs as m}
                    <div class={`chat ${m.role==='AI' ? 'chat-end' : 'chat-start'}`}>
                      <div class="chat-header opacity-70">{m.role}</div>
                      <div class={`chat-bubble ${m.role==='AI' ? 'chat-bubble-primary' : 'chat-bubble-neutral'}`}>{m.text}</div>
                    </div>
                  {/each}
                </div>
              </div>
            {/if}
          {:else}
            <div class="text-sm opacity-70">No LLM data yet.</div>
          {/if}
        </div>
      </div>
    {/if}

    {#if role==='teacher' || role==='admin'}
      <div class="card bg-base-100 shadow">
        <div class="card-body space-y-3">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="card-title">Manual testing</h3>
              <p class="text-sm opacity-70">Run the student's script in an isolated container with live I/O.</p>
            </div>
            {#if !forceManualConsole}
              <button class="btn btn-sm btn-outline" on:click={() => (manualConsoleVisible = !manualConsoleVisible)}>
                {manualConsoleVisible ? 'Hide console' : 'Show console'}
              </button>
            {/if}
          </div>

          {#if forceManualConsole || manualConsoleVisible}
            <RunConsole submissionId={sid} />
          {:else}
            <div class="text-sm opacity-70 italic">Use the button above to open the manual console.</div>
          {/if}
        </div>
      </div>
    {/if}
  </div>
{/if}

<dialog bind:this={fileDialog} class="modal">
  <div class="modal-box w-11/12 max-w-5xl">
    <div class="flex items-center justify-between mb-2">
      <div class="font-medium">Files</div>
      <button class="btn btn-sm btn-primary" on:click={downloadFiles}>Download</button>
    </div>
    {#if files.length}
      <div class="flex flex-col md:flex-row gap-4">
        <div class="md:w-60">
          <FileTree nodes={tree} select={chooseFile} />
        </div>
        <div class="flex-1">
          <div class="font-mono text-sm mb-2">{selected?.name}</div>
          <pre class="whitespace-pre bg-base-200 p-2 rounded"><code class="hljs">{@html highlighted}</code></pre>
        </div>
      </div>
    {:else}
      <pre class="whitespace-pre bg-base-200 p-2 rounded"><code class="hljs">{@html highlighted}</code></pre>
    {/if}
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

{#if err}<p style="color:red">{err}</p>{/if}

<style>
pre {
  background: #eee;
  padding: .5rem;
  overflow: auto;
}
.hljs {
  background: transparent;
}
</style>

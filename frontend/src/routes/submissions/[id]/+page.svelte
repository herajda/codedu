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

$: id = $page.params.id

  let submission:any=null
  let results:any[]=[]
  let err = ''
  let files: { name: string; content: string }[] = []
  let tree: FileNode[] = []
  let selected: { name: string; content: string } | null = null
  let highlighted = ''
  let esCtrl: { close: () => void } | null = null
  let assignmentTitle: string = ''
  let assignmentManual: boolean = false
  let assignmentTestsCount: number = 0
  let assignmentLLMInteractive: boolean = false
  let assignmentLLMFeedback: boolean = false
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
  // Show Auto-tests only when NOT LLM mode and there are tests configured
  $: showAutoUI = (!assignmentLLMInteractive) && (assignmentTestsCount > 0)
  // Keep legacy meaning of hideAutoUI: specifically, when no auto tests exist
  $: hideAutoUI = assignmentTestsCount === 0

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
  $: sid = submission?.id ?? Number(id)
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
            <div class="flex items-center gap-3">
              <input type="number" step="1" min="0" inputmode="numeric" pattern="[0-9]*" on:keydown={(e) => { if (['e','E','+','-','.',','].includes(e.key)) e.preventDefault() }} class="input input-bordered input-sm w-28 sm:w-32" bind:value={overrideValue} placeholder="points (optional)" aria-label="Override points">
              <button class={`btn btn-primary btn-sm ${savingOverride ? 'loading' : ''}`} on:click={saveOverride} disabled={savingOverride}>Save points</button>
              <button class="btn btn-success btn-sm" on:click={async()=>{ try{ const raw:any=overrideValue; const str = raw==null? '' : (typeof raw==='string'? raw : String(raw)); const v = str.trim()===''? null : parseInt(str,10); await apiFetch(`/api/submissions/${submission.id}/accept`, { method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({ points: v })}); await load(); }catch(e:any){ err=e.message } }}>Accept submission</button>
            </div>
            <div class="text-xs opacity-70 mt-2">Leave points empty to accept without overriding. Acceptance marks the submission completed and visible to the student as manually accepted.</div>
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
                <details class="collapse collapse-arrow bg-base-200">
                  <summary class="collapse-title">
                    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
                      <div class="font-medium">Test {i+1}</div>
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
                  <div class="collapse-content space-y-2">
                    {#if r.stderr}
                      <pre class="whitespace-pre-wrap bg-base-300 rounded p-3 overflow-x-auto">{r.stderr}</pre>
                    {:else}
                      <div class="text-sm opacity-70">No stderr output</div>
                    {/if}
                  </div>
                </details>
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

    {#if (role==='teacher' || role==='admin') && (assignmentManual || hideAutoUI)}
      <div class="card bg-base-100 shadow">
        <div class="card-body space-y-3">
          <h3 class="card-title">Manual testing</h3>
          <p class="text-sm opacity-70">Run the student's script in an isolated container with live I/O.</p>
          <RunConsole submissionId={sid} />
        </div>
      </div>
    {/if}
  </div>
{/if}

<dialog bind:this={fileDialog} class="modal">
  <div class="modal-box w-11/12 max-w-5xl">
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

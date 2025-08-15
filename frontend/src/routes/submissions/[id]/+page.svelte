<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { apiJSON, apiFetch } from '$lib/api'
  import { createEventSource } from '$lib/sse'
  import { page } from '$app/stores'
  import JSZip from 'jszip'
  import { FileTree } from '$lib'
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
  let role = ''
  $: role = $auth?.role ?? ''

  import hljs from 'highlight.js'
  import 'highlight.js/styles/github.css'
  let fileDialog: HTMLDialogElement

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
      const v = str.trim() === '' ? null : Number(str)
      await apiFetch(`/api/submissions/${submission.id}/points`,{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body: JSON.stringify({ points: v })
      })
      await load()
      overrideValue = ''
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

      files = await parseFiles(submission.code_content)
      tree = buildTree(files)
      selected = files[0]

      if (submission?.assignment_id) {
        try {
          const ad = await apiJSON(`/api/assignments/${submission.assignment_id}`)
          assignmentTitle = ad.assignment?.title ?? ''
          assignmentManual = !!ad.assignment?.manual_review
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

  function bgFromBadge(badgeClass: string){
    return badgeClass.replace('badge','bg')
  }

  $: totalTests = results?.length ?? 0
  $: passedCount = results.filter((r)=> r.status==='passed').length
  $: failedCount = results.filter((r)=> ['wrong_output','runtime_error','failed'].includes(r.status)).length
  $: warnedCount = results.filter((r)=> ['time_limit_exceeded','memory_limit_exceeded'].includes(r.status)).length

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
              <span class="inline-flex items-center gap-2 px-3 py-1 rounded bg-base-200">Submission #{submission.id}</span>
              <span class="inline-flex items-center gap-2 px-3 py-1 rounded bg-base-200">Submitted {formatDateTime(submission.created_at)}</span>
              {#if assignmentManual}
                <span class="inline-flex items-center gap-2 px-3 py-1 rounded bg-info/20 text-info">Manual review</span>
              {/if}
            </div>
          </div>
          <div class="flex items-center gap-2">
            <span class={`badge badge-lg ${statusColor(submission.status)}`}>{submission.status}</span>
            <button class="btn btn-ghost" on:click={goBack}>Back</button>
            <button class="btn btn-primary" on:click={openFiles}>View files</button>
          </div>
        </div>

        {#if assignmentManual && (role==='teacher' || role==='admin')}
          <div class="rounded-box bg-base-200 p-4 mt-2 flex items-end gap-3">
            <div class="flex-1">
              <div class="font-medium mb-1">Teacher override points</div>
              <input type="number" step="0.01" min="0" class="input input-bordered w-full" bind:value={overrideValue} placeholder="e.g. 10" aria-label="Override points">
              <div class="text-xs opacity-70 mt-1">Leave empty to clear override. Saving will also mark submission completed.</div>
            </div>
            <button class={`btn btn-primary ${savingOverride ? 'loading' : ''}`} on:click={saveOverride} disabled={savingOverride}>Save</button>
          </div>
        {/if}

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
      </div>
    </div>

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

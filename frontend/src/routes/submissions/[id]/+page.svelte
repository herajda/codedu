<script lang="ts">
  import { onMount } from 'svelte'
  import { apiJSON } from '$lib/api'
  import { page } from '$app/stores'
  import JSZip from 'jszip'
  import { FileTree } from '$lib'

$: id = $page.params.id

  let submission:any=null
  let results:any[]=[]
  let err = ''
  let files: { name: string; content: string }[] = []
  let tree: FileNode[] = []
  let selected: { name: string; content: string } | null = null
  let fileDialog: HTMLDialogElement

  interface FileNode {
    name: string
    content?: string
    children?: FileNode[]
  }

  async function load() {
    err = ''
    try {
      const data = await apiJSON(`/api/submissions/${id}`)
      submission = data.submission
      results = data.results

      files = []
      tree = []
      try {
        const zip = await JSZip.loadAsync(submission.code_content, { base64: true })
        for (const file of Object.values(zip.files)) {
          if (file.dir) continue
          const content = await file.async('string')
          files.push({ name: file.name, content })
        }
      } catch {
        try {
          const text = atob(submission.code_content)
          files = [{ name: 'code', content: text }]
        } catch {
          files = [{ name: 'code', content: submission.code_content }]
        }
      }

      if (files.length) {
        selected = files[0]
        tree = buildTree(files)
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
    if(s==='time_limit_exceeded' || s==='memory_limit_exceeded') return 'badge-warning'
    return ''
  }

  function openFiles() {
    if (files.length) {
      selected = files[0]
      fileDialog.showModal()
    }
  }

  function chooseFile(n: FileNode) {
    if (n.content) {
      selected = { name: n.name, content: n.content }
    }
  }

  onMount(load)
</script>

{#if !submission}
  <span class="loading loading-dots"></span>
{:else}
  <div class="space-y-4">
    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <h1 class="card-title">Submission {submission.id}</h1>
        <p><strong>Status:</strong> <span class={`badge ${statusColor(submission.status)}`}>{submission.status}</span></p>
      </div>
    </div>
    <div class="card bg-base-100 shadow">
      <div class="card-body space-y-2">
        <h3 class="card-title">Files</h3>
        <button class="btn btn-primary" on:click={openFiles}>Show files</button>
      </div>
    </div>
    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <h3 class="card-title">Results</h3>
        <div class="overflow-x-auto">
          <table class="table table-zebra">
            <thead>
              <tr><th>Test</th><th>Status</th><th>Runtime (ms)</th></tr>
            </thead>
            <tbody>
              {#each results as r, i}
                <tr>
                  <td>{i + 1}</td>
                  <td><span class={`badge ${statusColor(r.status)}`}>{r.status}</span></td>
                  <td>{r.runtime_ms}</td>
                </tr>
              {/each}
              {#if Array.isArray(results) && !results.length}
                <tr><td colspan="3"><i>No results yet</i></td></tr>
              {/if}
            </tbody>
          </table>
        </div>
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
        <pre class="flex-1 whitespace-pre-wrap bg-base-200 p-2 rounded">
          {selected?.content}
        </pre>
      </div>
    {:else}
      <pre class="whitespace-pre-wrap bg-base-200 p-2 rounded">
        {submission?.code_content}
      </pre>
    {/if}
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

{#if err}<p style="color:red">{err}</p>{/if}

<style>
pre{
  background:#eee;
  padding:.5rem;
  overflow:auto;
}
</style>

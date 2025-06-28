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
  let highlighted = ''

  function addLineNumbers(html: string) {
    return html
      .split('\n')
      .map((line) => `<span class="line">${line}</span>`) 
      .join('\n')
  }

  import hljs from 'highlight.js'
  import 'highlight.js/styles/github.css'
  let fileDialog: HTMLDialogElement

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

  $: if (selected) {
    highlighted = addLineNumbers(hljs.highlightAuto(selected.content).value)
  }

  $: if (!selected && submission) {
    highlighted = addLineNumbers(hljs.highlightAuto(submission.code_content).value)
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
        <div class="flex-1">
          <div class="font-mono text-sm mb-2">{selected?.name}</div>
          <pre class="whitespace-pre bg-base-200 p-2 rounded"><code class="hljs line-numbers">{@html highlighted}</code></pre>
        </div>
      </div>
    {:else}
      <pre class="whitespace-pre bg-base-200 p-2 rounded"><code class="hljs line-numbers">{@html highlighted}</code></pre>
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

code.line-numbers {
  counter-reset: line;
}

.line {
  display: block;
  padding-left: 3em;
  position: relative;
}

.line::before {
  counter-increment: line;
  content: counter(line);
  position: absolute;
  left: 0;
  width: 2.5em;
  text-align: right;
  padding-right: 0.5em;
  color: #888;
}
</style>

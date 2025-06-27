<script lang="ts">
  import { onMount } from 'svelte'
  import { apiJSON } from '$lib/api'
  import { page } from '$app/stores'

$: id = $page.params.id

  let submission:any=null
  let results:any[]=[]
  let err=''

  async function load(){
    err=''
    try{
      const data = await apiJSON(`/api/submissions/${id}`)
      submission = data.submission
      results = data.results
    }catch(e:any){ err=e.message }
  }

  function statusColor(s:string){
    if(s==='completed') return 'badge-success'
    if(s==='running') return 'badge-info'
    if(s==='failed') return 'badge-error'
    return ''
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
      <div class="card-body">
        <h3 class="card-title">File</h3>
        <pre class="whitespace-pre-wrap">{submission.code_content}</pre>
      </div>
    </div>
    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <h3 class="card-title">Results</h3>
        <ul class="list-disc ml-6">
          {#each results as r, i}
            <li>Test {i + 1}: <span class={`badge ${statusColor(r.status)}`}>{r.status}</span> ({r.runtime_ms} ms)</li>
          {/each}
          {#if Array.isArray(results) && !results.length}<i>No results yet</i>{/if}
        </ul>
      </div>
    </div>
  </div>
{/if}

{#if err}<p style="color:red">{err}</p>{/if}

<style>
pre{
  background:#eee;
  padding:.5rem;
  overflow:auto;
}
</style>

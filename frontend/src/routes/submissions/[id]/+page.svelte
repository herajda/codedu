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

  onMount(load)
</script>

{#if !submission}
  <p>Loading…</p>
{:else}
  <h1>Submission {submission.id}</h1>
  <p><strong>Status:</strong> {submission.status}</p>
  <h3>File</h3>
  <pre>{submission.code_content}</pre>
  <h3>Results</h3>
  <ul>
    {#each results as r, i}
      <li>Test {i + 1}: {r.status} ({r.runtime_ms} ms)</li>
    {/each}
    {#if Array.isArray(results) && !results.length}<i>No results yet</i>{/if}
  </ul>
{/if}

{#if err}<p style="color:red">{err}</p>{/if}

<style>
pre{
  background:#eee;
  padding:.5rem;
  overflow:auto;
}
</style>

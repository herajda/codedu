<script lang="ts">
  import { onMount } from 'svelte'
  import { get } from 'svelte/store'
  import { auth } from '../lib/auth'
  import { apiFetch, apiJSON } from '../lib/api'

  export let params:{id:string}

  const role = get(auth)?.role!

  let assignment:any=null
  let tests:any[]=[] // teacher/admin only
  let submissions:any[]=[]
  let err=''
  let tStdin='', tStdout=''
  let file:File|null=null

  async function load(){
    err=''
    try{
      const data = await apiJSON(`/api/assignments/${params.id}`)
      assignment = data.assignment
      if(role==='student') submissions = data.submissions ?? []
      else tests = data.tests ?? []
    }catch(e:any){ err=e.message }
  }

  onMount(load)

  async function addTest(){
    try{
      await apiFetch(`/api/assignments/${params.id}/tests`,{
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({stdin:tStdin, expected_stdout:tStdout})
      })
      tStdin=tStdout=''
      await load()
    }catch(e:any){ err=e.message }
  }

  async function submit(){
    if(!file) return
    const fd = new FormData()
    fd.append('file', file)
    try{
      await apiFetch(`/api/assignments/${params.id}/submissions`,{method:'POST', body:fd})
      file=null
      alert('Uploaded!')
    }catch(e:any){ err=e.message }
  }
</script>

{#if !assignment}
  <p>Loading…</p>
{:else}
  <h1>{assignment.title}</h1>
  <p>{assignment.description}</p>
  <p><strong>Deadline:</strong> {new Date(assignment.deadline).toLocaleString()}</p>
  <p><strong>Max points:</strong> {assignment.max_points}</p>
  <p><strong>Policy:</strong> {assignment.grading_policy}</p>

  {#if role !== 'student'}
    <h2>Tests</h2>
    <ul>
      {#each tests ?? [] as t, i}
        <li>Test {i + 1}: <pre>{t.stdin}</pre>→<pre>{t.expected_stdout}</pre></li>
      {/each}
      {#if !(tests && tests.length)}<i>No tests</i>{/if}
    </ul>
  {/if}

  {#if role==='student'}
    <h3>Your submissions</h3>
    <ul>
      {#each submissions as s}
        <li>
          <a href={`#/submissions/${s.id}`}>{new Date(s.created_at).toLocaleString()}</a>
          &nbsp;– {s.status}
        </li>
      {/each}
      {#if !submissions.length}<i>No submissions yet</i>{/if}
    </ul>
  {/if}

  {#if role==='teacher' || role==='admin'}
    <h3>Add test</h3>
    <input placeholder="stdin" bind:value={tStdin}>
    <br>
    <input placeholder="expected stdout" bind:value={tStdout}>
    <br>
    <button on:click={addTest}>Add</button>
  {/if}

  {#if role==='student'}
    <h3>Submit solution</h3>
    <input type="file" accept=".py" on:change={e=>file=(e.target as HTMLInputElement).files?.[0] || null}>
    <button disabled={!file} on:click={submit}>Upload</button>
  {/if}

  {#if err}<p style="color:red">{err}</p>{/if}
{/if}

<style>
pre{display:inline;margin:0 0.5rem;padding:0.2rem;background:#eee}
</style>

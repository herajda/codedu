<script lang="ts">
  import { onMount }   from 'svelte'
  import { get }       from 'svelte/store'
  import { auth }      from '../lib/auth'
  import { apiFetch, apiJSON } from '../lib/api'

  /* ───────────────────────── route param comes in as a PROP */
  export let params: {id: string}

  const role = get(auth)?.role!

  /* ───────────────────────── component state */
  let cls:any           = null
  let students:any[]    = []
  let assignments:any[] = []
  let allStudents:any[] = []
  let selectedIDs:number[] = []
  let aTitle=''
  let err = ''

  /* keep the id handy for the add/remove helpers */

  /* ───────────────────────── data fetcher */
  async function load() {
    err = ''
    try {
      const data = await apiJSON(`/api/classes/${params.id}`)
      cls        = data
      students   = data.students
      assignments = [...(data.assignments ?? [])].sort(
        (a,b) => new Date(a.deadline).getTime() - new Date(b.deadline).getTime()
      )
      if (role === 'teacher' || role === 'admin')
        allStudents = await apiJSON('/api/students')
    } catch (e:any) { err = e.message }
  }

  /* ───────────────────────── run exactly once */
  onMount(load)

  /* ───────────────────────── teacher actions (unchanged, just use currentId) */
  async function addStudents() {
    try {
      await apiFetch(`/api/classes/${params.id}/students`, {
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({ student_ids:selectedIDs })
      })
      selectedIDs = []
      await load()
    } catch (e:any) { err = e.message }
  }

  async function removeStudent(sid:number) {
    if (!confirm('Remove this student from class?')) return
    try {
      await apiFetch(`/api/classes/${params.id}/students/${sid}`,{ method:'DELETE' })
      await load()
    } catch(e:any){ err=e.message }
  }

  async function createAssignment() {
    try {
      await apiFetch(`/api/classes/${params.id}/assignments`,{
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({ title:aTitle })
      })
      aTitle=''
      await load()
    } catch(e:any){ err=e.message }
  }

  async function deleteAssignment(aid:number) {
    if (!confirm('Delete this assignment?')) return
    try {
      await apiFetch(`/api/assignments/${aid}`,{ method:'DELETE' })
      await load()
    } catch(e:any){ err=e.message }
  }
</script>

{#if !cls}
  <p>Loading…</p>
{:else}
  <h1>{cls.name}</h1>
  {#if role === 'student'}
    <p><strong>Teacher:</strong> {cls.teacher.email}</p>
  {/if}

  <!-- ‣ Students -->
  <h2>Students</h2>
  <ul>
    {#each students as s}
      <li>
        {s.email}
        {#if role === 'teacher' || role === 'admin'}
          &nbsp;<button on:click={()=>removeStudent(s.id)}>✕</button>
        {/if}
      </li>
    {/each}
    {#if !students || !students.length}<i>No students yet</i>{/if}
  </ul>

  {#if role === 'teacher' || role === 'admin'}
    <details>
      <summary><strong>Add students</strong></summary>
      <select multiple size="6" bind:value={selectedIDs}>
        {#each allStudents as s}
          <option value={s.id}>{s.email}</option>
        {/each}
      </select>
      <br>
      <button disabled={!selectedIDs.length} on:click={addStudents}>Add selected</button>
    </details>
  {/if}

  <!-- ‣ Assignments -->
  <h2>Assignments</h2>
  <ul>
    {#each assignments as a}
      <li>
        <strong><a href={`#/assignments/${a.id}`}>{a.title}</a></strong>
        {#if !a.published}
          <em style="color:gray"> (draft)</em>
        {/if}
        &nbsp;·&nbsp;
        <span style="color:{new Date(a.deadline)<new Date() ? 'red' : 'inherit'}">
          due {new Date(a.deadline).toLocaleString()}
        </span>
        {#if role === 'teacher' || role === 'admin'}
          &nbsp;<button on:click={()=>deleteAssignment(a.id)}>✕</button>
        {/if}
        <p>{a.description} (max {a.max_points} pts, {a.grading_policy})</p>
      </li>
    {/each}
    {#if !assignments.length}<i>No assignments yet</i>{/if}
  </ul>

  {#if role === 'teacher' || role === 'admin'}
    <h3>Create assignment</h3>
    <form on:submit|preventDefault={createAssignment}>
      <input placeholder="Title" bind:value={aTitle} required>
      <br>
      <button>Create</button>
    </form>
  {/if}

  {#if err}<p style="color:red">{err}</p>{/if}
{/if}

<style>
  button { cursor:pointer }
  li     { margin:.25rem 0 }
  details { margin:.5rem 0 1rem }
</style>

<script lang="ts">
  import { onMount }   from 'svelte'
  import { get }       from 'svelte/store'
  import { auth }      from '$lib/auth'
  import { apiFetch, apiJSON } from '$lib/api'
  import { page } from '$app/stores'

  $: id  = $page.params.id

  const role: string = get(auth)?.role ?? ''

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
      const data = await apiJSON(`/api/classes/${id}`)
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
      await apiFetch(`/api/classes/${id}/students`, {
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
      await apiFetch(`/api/classes/${id}/students/${sid}`,{ method:'DELETE' })
      await load()
    } catch(e:any){ err=e.message }
  }

  async function createAssignment() {
    try {
      await apiFetch(`/api/classes/${id}/assignments`,{
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

  /* ───────────────────────── Bakalari import helpers */
  let bkUser = ''
  let bkPass = ''
  let bkAtoms: { Id:string; Name:string }[] = []
  let loadingAtoms = false

  async function fetchAtoms() {
    err = ''
    loadingAtoms = true
    try {
      bkAtoms = await apiJSON('/api/bakalari/atoms', {
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body: JSON.stringify({ username: bkUser, password: bkPass })
      })
    } catch(e:any) { err = e.message }
    loadingAtoms = false
  }

  async function importAtom(aid:string) {
    err = ''
    try {
      const res = await apiJSON<{added:number}>(`/api/classes/${id}/import-bakalari`, {
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body: JSON.stringify({ username: bkUser, password: bkPass, atom_id: aid })
      })
      await load()
      alert(`Imported ${res.added} students`)
    } catch(e:any){ err = e.message }
  }
</script>

{#if !cls}
  <p>Loading…</p>
{:else}
  <h1>{cls.name}</h1>
  {#if role === 'student'}
    <p><strong>Teacher:</strong> {cls.teacher.name ?? cls.teacher.email}</p>
  {/if}

  <!-- ‣ Students -->
  <h2>Students</h2>
  <ul>
    {#each students as s}
      <li>
        {s.name ?? s.email}
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
          <option value={s.id}>{s.name ?? s.email}</option>
        {/each}
      </select>
      <br>
      <button disabled={!selectedIDs.length} on:click={addStudents}>Add selected</button>
    </details>

    <details>
      <summary><strong>Import from Bakaláři</strong></summary>
      <input placeholder="Username" bind:value={bkUser}>
      <input type="password" placeholder="Password" bind:value={bkPass}>
      <button on:click={fetchAtoms} disabled={loadingAtoms}>Load classes</button>
      {#if bkAtoms.length}
        <ul>
          {#each bkAtoms as a}
            <li>
              {a.Name}
              <button on:click={()=>importAtom(a.Id)}>Import</button>
            </li>
          {/each}
        </ul>
      {:else if loadingAtoms}
        <p>Loading…</p>
      {/if}
    </details>
  {/if}

  <!-- ‣ Assignments -->
  <h2>Assignments</h2>
  <ul>
    {#each assignments as a}
      <li>
        <strong><a href={`/assignments/${a.id}`}>{a.title}</a></strong>
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

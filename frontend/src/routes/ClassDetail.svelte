<script lang="ts">
    import { onMount }  from 'svelte'
    import { params }   from 'svelte-spa-router'
    import { get }      from 'svelte/store'
    import { auth }     from '../lib/auth'
    import { apiJSON }  from '../lib/api'
  
    const { id } = params  // class id from URL
  
    const role   = get(auth)?.role
    let cls:any  = null       // class meta
    let students:any[] = []
    let allStudents:any[] = []  // for pick-list
    let assignments:any[] = []
    let err=''
  
    // new student selection & assignment fields
    let selectedIDs:number[]=[]
    let aTitle='', aDesc='', aDeadline=''
  
    async function load() {
      cls         = await apiJSON(`/api/classes/${id}`)
      students    = cls.students        // backend may include array
      assignments = cls.assignments
      if (role==='teacher') {
        allStudents = await apiJSON('/api/students')
      }
    }
  
    async function addStudents() {
      err=''
      try{
        await apiJSON(`/api/classes/${id}/students`,{
          method:'POST',
          headers:{'Content-Type':'application/json'},
          body: JSON.stringify({ student_ids:selectedIDs })
        })
        load()
        selectedIDs=[]
      }catch(e:any){err=e.message}
    }
  
    async function createAssignment() {
      err=''
      try {
        await apiJSON(`/api/classes/${id}/assignments`,{
          method:'POST',
          headers:{'Content-Type':'application/json'},
          body: JSON.stringify({
            title:aTitle, description:aDesc, deadline:new Date(aDeadline).toISOString()
          })
        })
        aTitle=aDesc=aDeadline=''
        load()
      }catch(e:any){err=e.message}
    }
  
    onMount(load)
  </script>
  
  {#if !cls}<p>Loading…</p>{:else}
  <h1>Class: {cls.name}</h1>
  
  <h2>Students</h2>
  <ul>
    {#each students as s}<li>{s.email} (id {s.id})</li>{/each}
  </ul>
  
  {#if role==='teacher'}
    <details>
      <summary><strong>Add students</strong></summary>
      <select multiple size="6" bind:value={selectedIDs}>
        {#each allStudents as s}<option value={s.id}>{s.email}</option>{/each}
      </select>
      <br/>
      <button on:click={addStudents}>Add selected</button>
    </details>
  
    <h2>Create assignment</h2>
    <form on:submit|preventDefault={createAssignment}>
      <input  placeholder="Title"       bind:value={aTitle}      required />
      <br/>
      <textarea placeholder="Description" bind:value={aDesc}   required />
      <br/>
      <input type="datetime-local" bind:value={aDeadline} required />
      <br/>
      <button>Create</button>
    </form>
  {/if}
  
  <h2>Assignments</h2>
  <ul>
    {#each assignments as a}
      <li>
        <strong>{a.title}</strong> &nbsp;
        <small>due {new Date(a.deadline).toLocaleString()}</small>
        <p>{a.description}</p>
      </li>
    {/each}
    {#if !assignments.length}<p>No assignments yet.</p>{/if}
  </ul>
  
  {#if err}<p style="color:red">{err}</p>{/if}
  {/if}
  
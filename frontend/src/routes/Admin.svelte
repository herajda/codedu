<script lang="ts">
  import { onMount } from 'svelte'
  import { apiFetch, apiJSON } from '../lib/api'

  // ───────────────────────────────────────── tabs
  let tab:'teachers'|'users'|'classes'|'bakalari' = 'teachers'

  // ──────────────────────────── 1) add-teacher form
  let email='', password=''
  let ok='', err=''

  async function addTeacher() {
    err = ok = ''
    const r = await apiFetch('/api/teachers', {
      method:'POST',
      headers:{'Content-Type':'application/json'},
      body:JSON.stringify({email,password})
    })
    if (r.status === 201) { ok='Teacher created ✔'; email=password='' }
    else                  { err=(await r.json()).error }
  }

  // ──────────────────────────── 2) users table
  type User = { id:number; email:string; role:string; created_at:string }
  let users:User[]=[]
  const roles = ['student','teacher','admin']

  async function loadUsers(){ users = await apiJSON('/api/users') }

  async function changeRole(id:number, role:string){
    try{
      await apiFetch(`/api/users/${id}/role`,{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({role})
      })
      loadUsers()
    }catch(e){ alert(e) }
  }

  // ──────────────────────────── 3) classes overview
  type Class = { id:number; name:string; teacher_id:number; created_at:string }
  let classes:Class[]=[]
  async function loadClasses(){ classes = await apiJSON('/api/classes/all') }

  // ──────────────────────────── 4) Bakalari integration
  let bakalariURL = ''
  let bakOk='', bakErr=''
  let bakUser='', bakPass=''
  let importMsg='', importErr=''
  let importTeachMsg='', importTeachErr=''

  async function loadBakalariURL(){
    try {
      const r = await apiFetch('/api/bakalari/url')
      if (r.ok) {
        bakalariURL = (await r.json()).url
      }
    } catch {}
  }

  async function saveBakalariURL(){
    bakOk=bakErr=''
    try{
      await apiFetch('/api/bakalari/url',{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({url:bakalariURL})
      })
      bakOk='Saved ✔'
    }catch(e:any){ bakErr=e.message }
  }

  async function importStudentsBak(){
    importMsg=importErr=''
    try{
      const r = await apiJSON<{imported:number}>('/api/bakalari/import',{
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({username:bakUser,password:bakPass})
      })
      importMsg=`Imported ${r.imported} students`
    }catch(e:any){ importErr=e.message }
  }

  async function importTeachersBak(){
    importTeachMsg=importTeachErr=''
    try{
      const r = await apiJSON<{imported:number}>('/api/bakalari/import-teachers',{
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({username:bakUser,password:bakPass})
      })
      importTeachMsg=`Imported ${r.imported} teachers`
    }catch(e:any){ importTeachErr=e.message }
  }

  // load everything once
  onMount(()=>{
    loadUsers()
    loadClasses()
    loadBakalariURL()
  })
</script>

<h1>Admin panel</h1>

<nav style="margin-bottom:1.2rem">
  <button on:click={()=>tab='teachers'} class:active={tab==='teachers'}>Teachers</button>
  <button on:click={()=>tab='users'}    class:active={tab==='users'}   >Users</button>
  <button on:click={()=>tab='classes'}  class:active={tab==='classes'} >Classes</button>
  <button on:click={()=>tab='bakalari'} class:active={tab==='bakalari'}>Bakalari</button>
</nav>

{#if tab==='teachers'}
  <h2>Add teacher</h2>
  <form on:submit|preventDefault={addTeacher}>
    <input  type="email"    bind:value={email}    placeholder="Email"    required />
    <input  type="password" bind:value={password} placeholder="Password" required />
    <button>Add</button>
  </form>
  {#if ok}<p style="color:green">{ok}</p>{/if}
  {#if err}<p style="color:red">{err}</p>{/if}
{/if}

{#if tab==='users'}
  <h2>All users</h2>
  <table border="1" cellpadding="4">
    <thead>
      <tr><th>ID</th><th>Email</th><th>Role</th><th>Created</th></tr>
    </thead>
    <tbody>
      {#each users as u}
        <tr>
          <td>{u.id}</td>
          <td>{u.email}</td>
          <td>
            <select bind:value={u.role} on:change={(e)=>changeRole(u.id,(e.target as HTMLSelectElement).value)}>
              {#each roles as r}<option>{r}</option>{/each}
            </select>
          </td>
          <td>{new Date(u.created_at).toLocaleDateString()}</td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

{#if tab==='classes'}
  <h2>All classes</h2>
  <table border="1" cellpadding="4">
    <thead>
      <tr><th>ID</th><th>Name</th><th>Teacher ID</th><th>Created</th></tr>
    </thead>
    <tbody>
      {#each classes as c}
        <tr>
          <td>{c.id}</td>
          <td><a href={`#/classes/${c.id}`}>{c.name}</a></td>
          <td>{c.teacher_id}</td>
          <td>{new Date(c.created_at).toLocaleDateString()}</td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

{#if tab==='bakalari'}
  <h2>Bakalari settings</h2>
  <form on:submit|preventDefault={saveBakalariURL}>
    <input placeholder="Bakalari URL" bind:value={bakalariURL} required />
    <button>Save</button>
  </form>
  {#if bakOk}<p style="color:green">{bakOk}</p>{/if}
  {#if bakErr}<p style="color:red">{bakErr}</p>{/if}

  <h3>Import users</h3>
  <input placeholder="Username" bind:value={bakUser} />
  <input type="password" placeholder="Password" bind:value={bakPass} />
  <div>
    <button on:click={importStudentsBak}>Import students</button>
    <button on:click={importTeachersBak}>Import teachers</button>
  </div>
  {#if importMsg}<p style="color:green">{importMsg}</p>{/if}
  {#if importErr}<p style="color:red">{importErr}</p>{/if}
  {#if importTeachMsg}<p style="color:green">{importTeachMsg}</p>{/if}
  {#if importTeachErr}<p style="color:red">{importTeachErr}</p>{/if}
{/if}

<style>
  nav button { margin-right:.5rem }
  .active   { border-color:#646cff }
</style>

<script lang="ts">
  import { onMount } from 'svelte'
  import { apiFetch, apiJSON } from '$lib/api'

  // ───────────────────────────────────────── tabs
  let tab:'teachers'|'users'|'classes' = 'teachers'

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

  // load everything once
  onMount(()=>{
    loadUsers()
    loadClasses()
  })
</script>

<h1>Admin panel</h1>

<nav style="margin-bottom:1.2rem">
  <button on:click={()=>tab='teachers'} class:active={tab==='teachers'}>Teachers</button>
  <button on:click={()=>tab='users'}    class:active={tab==='users'}   >Users</button>
  <button on:click={()=>tab='classes'}  class:active={tab==='classes'} >Classes</button>
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
          <td>{u.name ?? u.email}</td>
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

<style>
  nav button { margin-right:.5rem }
  .active   { border-color:#646cff }
</style>

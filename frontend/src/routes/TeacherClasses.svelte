<script lang="ts">
    import { onMount } from 'svelte'
    import { push }     from 'svelte-spa-router'
    import { apiJSON }  from '../lib/api'
  
    type Class = { id:number; name:string; created_at:string }
  
    let classes:Class[]=[]
    let name=''        // new-class name
    let err=''
  
    async function load() {
      classes = await apiJSON<Class[]>('/api/classes')
    }
  
    async function create() {
      err=''
      try {
        await apiJSON('/api/classes',{
          method:'POST',
          headers:{'Content-Type':'application/json'},
          body:JSON.stringify({name})
        })
        name=''
        await load() // Reload the class list after creation
      } catch(e:any){ err=e.message }
    }
  
    onMount(load)
  </script>
  
  <h1>Your classes</h1>
  
  {#if !classes.length}
    <p>No classes yet.</p>
  {/if}
  <ul>
    {#each classes as c}
      <li>
        <button
           on:click={()=>push(`/classes/${c.id}`)}
           style="background:none;border:none;padding:0;cursor:pointer;text-decoration:underline;color:inherit;font:inherit">
          {c.name} &nbsp; <small>({new Date(c.created_at).toLocaleDateString()})</small>
        </button>
      </li>
    {/each}
  </ul>
  
  <h2>Create new class</h2>
  <form on:submit|preventDefault={create}>
    <input bind:value={name} placeholder="Class name" required />
    <button>Create</button>
  </form>
  {#if err}<p style="color:red">{err}</p>{/if}

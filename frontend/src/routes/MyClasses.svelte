<script lang="ts">
    import { onMount } from 'svelte'
    import { push }    from 'svelte-spa-router'
    import { apiJSON } from '../lib/api'
  
    type Class = { id:number; name:string; teacher_email:string }


  
    let list:Class[]=[]
    let err=''
  
    onMount(async()=>{
      try { list = await apiJSON('/api/classes') }  // same endpoint; backend returns student-specific data
      catch(e:any){ err=e.message }
    })
  </script>
  
  <h1>My classes</h1>
  
  <ul>
    {#each list as c}
      <li>
        <button
           on:click={()=>push(`/classes/${c.id}`)}
           style="background:none;border:none;padding:0;cursor:pointer;text-decoration:underline;color:inherit;font:inherit">
          {c.name} – <small>{c.teacher_email}</small>
        </button>
      </li>
    {/each}
    {#if !list.length && !err}<p>No classes yet.</p>{/if}
  </ul>
  
  {#if err}<p style="color:red">{err}</p>{/if}
  
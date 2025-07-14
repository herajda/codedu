<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch, apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import { get } from 'svelte/store';
  import { auth } from '$lib/auth';

  let notes:any[] = [];
  let loading = true;
  let err = '';
  let id = $page.params.id;
  $: if ($page.params.id !== id) { id = $page.params.id; load(); }
  const role = get(auth)?.role ?? '';

  async function load(){
    loading = true;
    err='';
    try{
      notes = await apiJSON(`/api/classes/${id}/notes`);
    }catch(e:any){err=e.message}
    loading=false;
  }

  let path = '';
  let content = '';
  async function create(){
    try{
      await apiFetch(`/api/classes/${id}/notes`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({path,content})});
      path=content='';
      await load();
    }catch(e:any){err=e.message}
  }

  async function save(note:any){
    try{
      await apiFetch(`/api/notes/${note.id}`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({path:note.path,content:note.content,published:note.published})});
      await load();
    }catch(e:any){err=e.message}
  }

  onMount(load);
</script>

{#if loading}
  <p>Loading…</p>
{:else}
  <h1 class="text-2xl font-bold mb-4">Notes</h1>
  {#if role==='teacher' || role==='admin'}
    <div class="card bg-base-100 shadow mb-4">
      <div class="card-body space-y-2">
        <h2 class="card-title">Create note</h2>
        <input class="input input-bordered w-full" placeholder="Path" bind:value={path} />
        <textarea class="textarea textarea-bordered w-full" rows="6" bind:value={content}></textarea>
        <button class="btn" on:click={create} disabled={!path||!content}>Create</button>
      </div>
    </div>
  {/if}
  <ul class="space-y-4">
    {#each notes as n}
      <li class="card bg-base-100 shadow">
        <div class="card-body space-y-2">
          <input class="input input-bordered w-full" bind:value={n.path} readonly={!(role==='teacher'||role==='admin')} />
          <textarea class="textarea textarea-bordered w-full" rows="6" bind:value={n.content} readonly={!(role==='teacher'||role==='admin')}></textarea>
          <label class="flex gap-2 items-center" >
            <input type="checkbox" class="checkbox" bind:checked={n.published} disabled={!(role==='teacher'||role==='admin')}>
            <span>published</span>
          </label>
          {#if role==='teacher'||role==='admin'}
            <button class="btn" on:click={()=>save(n)}>Save</button>
          {/if}
        </div>
      </li>
    {/each}
    {#if !notes.length}<li><i>No notes</i></li>{/if}
  </ul>
  {#if err}<p class="text-error">{err}</p>{/if}
{/if}

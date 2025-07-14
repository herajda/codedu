<script lang="ts">
import { onMount } from 'svelte';
import { apiFetch, apiJSON } from '$lib/api';
import { page } from '$app/stores';
import { auth } from '$lib/auth';
import { get } from 'svelte/store';

let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(); }
const role: string = get(auth)?.role ?? '';

let notes:any[] = [];
let loading = true;
let err='';
let path='';
let content='';
let createDialog: HTMLDialogElement;

async function load(){
  loading=true; err='';
  try{ notes = await apiJSON(`/api/classes/${id}/notes`); }catch(e:any){err=e.message}
  loading=false;
}

async function createNote(){
  try{
    await apiFetch(`/api/classes/${id}/notes`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({path,content})});
    path=''; content='';
    createDialog.close();
    await load();
  }catch(e:any){ err=e.message }
}

onMount(load);
</script>

<h1 class="text-2xl font-bold mb-4">Notes</h1>
{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <ul class="space-y-2">
    {#each notes as n}
      <li><a class="link" href={`/notes/${n.id}`}>{n.path}{!n.published ? ' (unpublished)' : ''}</a></li>
    {/each}
    {#if !notes.length}<li><i>No notes</i></li>{/if}
  </ul>
{/if}

{#if role==='teacher' || role==='admin'}
  <button class="btn mt-4" on:click={()=>createDialog.showModal()}>New note</button>
  <dialog bind:this={createDialog} class="modal">
    <div class="modal-box">
      <h3 class="font-bold text-lg mb-4">Create note</h3>
      <input class="input input-bordered w-full mb-2" placeholder="Path" bind:value={path}>
      <textarea class="textarea textarea-bordered w-full h-40 mb-2" bind:value={content}></textarea>
      <div class="modal-action">
        <button class="btn" on:click={createNote}>Save</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>
{/if}

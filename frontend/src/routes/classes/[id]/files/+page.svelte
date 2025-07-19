<script lang="ts">
import { onMount } from 'svelte';
import { page } from '$app/stores';
import { get } from 'svelte/store';
import { auth } from '$lib/auth';
import { apiJSON, apiFetch } from '$lib/api';

let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(currentParent); }
const role: string = get(auth)?.role ?? '';

let items:any[] = [];
let breadcrumbs:{id:number|null,name:string}[] = [{id:null,name:'root'}];
let currentParent:number|null = null;
let loading = false;
let err = '';
let uploadInput: HTMLInputElement;
let newDir = '';

async function load(parent:number|null){
  loading = true; err='';
  try{
    const q = parent===null ? '' : `?parent=${parent}`;
    items = await apiJSON(`/api/classes/${id}/files${q}`);
    currentParent = parent;
  }catch(e:any){ err = e.message }
  loading = false;
}

async function openDir(item:any){
  breadcrumbs.push({id:item.id,name:item.name});
  await load(item.id);
}

function crumbTo(i:number){
  const b = breadcrumbs[i];
  breadcrumbs = breadcrumbs.slice(0,i+1);
  load(b.id);
}

async function upload(){
  if(!uploadInput.files?.length) return;
  const fd = new FormData();
  if(currentParent!==null) fd.append('parent_id', String(currentParent));
  fd.append('file', uploadInput.files[0]);
  await apiFetch(`/api/classes/${id}/files`,{method:'POST',body:fd});
  uploadInput.value='';
  await load(currentParent);
}

async function createDir(){
  await apiFetch(`/api/classes/${id}/files`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:newDir,parent_id:currentParent,is_dir:true})});
  newDir='';
  await load(currentParent);
}

async function del(item:any){
  if(!confirm('Delete?')) return;
  await apiFetch(`/api/files/${item.id}`,{method:'DELETE'});
  await load(currentParent);
}

async function rename(item:any){
  const nm = prompt('New name', item.name);
  if(!nm) return;
  await apiFetch(`/api/files/${item.id}`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:nm})});
  await load(currentParent);
}

onMount(()=>load(null));
</script>

<h1 class="text-2xl font-bold mb-4">Files</h1>
<nav class="mb-4">
  <ul class="breadcrumbs">
    {#each breadcrumbs as b,i}
      <li><a href="#" on:click|preventDefault={() => crumbTo(i)}>{b.name}</a></li>
    {/each}
  </ul>
</nav>

{#if loading}
<p>Loading…</p>
{:else if err}
<p class="text-error">{err}</p>
{:else}
<ul class="space-y-1 mb-4">
  {#each items as it}
  <li class="flex gap-2 items-center">
    {#if it.is_dir}
      <button class="link flex-1 text-left" on:click={()=>openDir(it)}>{it.name}</button>
    {:else}
      <a class="link flex-1" href={`/api/files/${it.id}`}>{it.name}</a>
      <span class="text-sm">{it.size} B</span>
    {/if}
    {#if role==='teacher' || role==='admin'}
      <button class="btn btn-xs" on:click={()=>rename(it)}>Rename</button>
      <button class="btn btn-xs btn-error" on:click={()=>del(it)}>Delete</button>
    {/if}
  </li>
  {/each}
  {#if !items.length}<li><i>Empty</i></li>{/if}
</ul>
{#if role==='teacher' || role==='admin'}
<div class="space-y-2">
  <input type="file" bind:this={uploadInput} class="file-input" />
  <button class="btn" on:click={upload}>Upload</button>
  <div>
    <input class="input input-bordered" placeholder="Folder" bind:value={newDir} />
    <button class="btn ml-2" on:click={createDir} disabled={!newDir}>Create</button>
  </div>
</div>
{/if}
{/if}

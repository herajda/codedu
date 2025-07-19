<script lang="ts">
import { onMount } from 'svelte';
import { page } from '$app/stores';
import { get } from 'svelte/store';
import { auth } from '$lib/auth';
import { apiJSON, apiFetch } from '$lib/api';
import '@fortawesome/fontawesome-free/css/all.min.css';

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

function iconClass(name: string) {
  const ext = name.split('.').pop()?.toLowerCase();
  switch (ext) {
    case 'pdf':
      return 'fa-file-pdf text-error';
    case 'png':
    case 'jpg':
    case 'jpeg':
    case 'gif':
      return 'fa-file-image text-success';
    case 'zip':
    case 'tar':
    case 'gz':
      return 'fa-file-zipper';
    case 'js':
    case 'ts':
    case 'svelte':
    case 'py':
    case 'go':
    case 'java':
    case 'cpp':
      return 'fa-file-code text-primary';
    default:
      return 'fa-file';
  }
}

function open(item: any) {
  if (item.is_dir) openDir(item);
  else window.open(`/api/files/${item.id}`, '_blank');
}

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
<div class="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 mb-4">
  {#each items as it}
    <div class="relative border rounded p-3 flex flex-col items-center group hover:shadow cursor-pointer" on:click={() => open(it)}>
      <div class="text-5xl mb-2">
        {#if it.is_dir}
          <i class="fa-solid fa-folder text-warning"></i>
        {:else}
          <i class="fa-solid {iconClass(it.name)}"></i>
        {/if}
      </div>
      <span class="text-sm text-center break-all">{it.name}</span>
      {#if role==='teacher' || role==='admin'}
        <div class="absolute top-1 right-1 hidden group-hover:flex gap-1">
          <button class="btn btn-xs btn-circle" title="Rename" on:click|stopPropagation={() => rename(it)}>
            <i class="fa-solid fa-pen"></i>
          </button>
          <button class="btn btn-xs btn-circle btn-error" title="Delete" on:click|stopPropagation={() => del(it)}>
            <i class="fa-solid fa-trash"></i>
          </button>
        </div>
      {/if}
    </div>
  {/each}
  {#if !items.length}
    <p class="col-span-full"><i>Empty</i></p>
  {/if}
</div>
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

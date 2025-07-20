<script lang="ts">
import { onMount } from 'svelte';
import { page } from '$app/stores';
import { goto } from '$app/navigation';
import { get } from 'svelte/store';
import { auth } from '$lib/auth';
import { apiJSON, apiFetch } from '$lib/api';
import '@fortawesome/fontawesome-free/css/all.min.css';

let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(currentParent); }
const role: string = get(auth)?.role ?? '';

let items:any[] = [];
let search = '';
let searchOpen = false;
let searchResults:any[] = [];
$: if (searchOpen && search.trim() !== '') {
  fetchSearch(search.trim());
} else {
  searchResults = [];
}
$: displayed = searchOpen && search.trim() !== '' ? searchResults : items;
let breadcrumbs:{id:number|null,name:string}[] = [{id:null,name:'🏠'}];
let currentParent:number|null = null;
let loading = false;
let err = '';
let uploadInput: HTMLInputElement;

function toggleSearch() {
  searchOpen = !searchOpen;
  if (!searchOpen) search = '';
}

function isImage(name: string) {
  const ext = name.split('.').pop()?.toLowerCase();
  return ['png','jpg','jpeg','gif','webp','svg'].includes(ext ?? '');
}

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
    case 'ipynb':
      return 'fa-book text-secondary';
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
  else if (item.name.toLowerCase().endsWith('.ipynb') || isImage(item.name))
    goto(`/files/${item.id}`);
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

async function fetchSearch(q:string){
  loading = true; err='';
  try{
    searchResults = await apiJSON(`/api/classes/${id}/files?search=${encodeURIComponent(q)}`);
  }catch(e:any){ err = e.message }
  loading = false;
}

async function openDir(item:any){
  breadcrumbs = [...breadcrumbs, {id:item.id,name:item.name}];
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

async function createDir(name:string){
  await apiFetch(`/api/classes/${id}/files`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, parent_id: currentParent, is_dir: true })
  });
  await load(currentParent);
}

function promptDir(){
  const nm = prompt('Folder name');
  if(nm) createDir(nm);
}

async function createNotebook(name: string) {
  const cf = await apiJSON(`/api/classes/${id}/files`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, parent_id: currentParent })
  });
  const nb = {
    cells: [],
    metadata: {},
    nbformat: 4,
    nbformat_minor: 5
  };
  await apiFetch(`/api/files/${cf.id}/content`, {
    method: 'PUT',
    body: JSON.stringify(nb)
  });
  await load(currentParent);
}

function promptNotebook() {
  let nm = prompt('Notebook name', 'Untitled.ipynb');
  if (!nm) return;
  if (!nm.toLowerCase().endsWith('.ipynb')) nm += '.ipynb';
  createNotebook(nm);
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

<nav class="mb-4 sticky top-16 z-30 bg-base-200 rounded-box shadow px-4 py-2 flex items-center flex-wrap gap-2">
  <ul class="flex flex-wrap gap-1 text-sm items-center flex-grow">
    {#each breadcrumbs as b,i}
      <li class="after:mx-1 after:content-['/'] last:after:hidden">
        <a
          href="#"
          class="link px-2 py-1 rounded hover:bg-base-300"
          on:click|preventDefault={() => crumbTo(i)}
          >{b.name}</a
        >
      </li>
    {/each}
  </ul>
  <div class="flex items-center gap-2 ml-auto">
    <div class="relative overflow-hidden flex items-center">
      <button class="btn btn-sm btn-circle" on:click={toggleSearch} aria-label="Search">
        <i class="fa-solid fa-search"></i>
      </button>
      <input
        class="input input-sm input-bordered ml-2 transition-all duration-300"
        style:width={searchOpen ? '12rem' : '0'}
        style:padding-left={searchOpen ? '0.5rem' : '0'}
        style:padding-right={searchOpen ? '0.5rem' : '0'}
        style:opacity={searchOpen ? '1' : '0'}
        placeholder="Search"
        bind:value={search}
      />
    </div>
    {#if role==='teacher' || role==='admin'}
    <div class="flex items-center gap-2">
      <input type="file" bind:this={uploadInput} class="hidden" on:change={upload} />
      <button class="btn btn-sm btn-circle" on:click={() => uploadInput.click()} title="Upload file">
        <i class="fa-solid fa-upload"></i>
      </button>
      <button class="btn btn-sm btn-circle" on:click={promptDir} title="New folder">
        <i class="fa-solid fa-folder-plus"></i>
      </button>
      <button class="btn btn-sm btn-circle" on:click={promptNotebook} title="New notebook">
        <i class="fa-solid fa-book-medical"></i>
      </button>
    </div>
    {/if}
  </div>
</nav>

{#if loading}
<p>Loading…</p>
{:else if err}
<p class="text-error">{err}</p>
{:else}
<div class="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 mb-4">
  {#each displayed as it}
    <div class="relative border rounded p-3 flex flex-col items-center group hover:shadow cursor-pointer" on:click={() => open(it)}>
      <div class="text-5xl mb-2">
        {#if it.is_dir}
          <i class="fa-solid fa-folder text-warning"></i>
        {:else if isImage(it.name)}
          <img src={`/api/files/${it.id}`} alt={it.name} class="w-16 h-16 object-cover rounded" />
        {:else}
          <i class="fa-solid {iconClass(it.name)}"></i>
        {/if}
      </div>
      <span class="text-sm text-center break-all">{it.name}</span>
      {#if searchOpen && search.trim() !== ''}
        <span class="text-xs text-center text-gray-500 break-all">{it.path}</span>
      {/if}
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
  {#if !(searchOpen && search.trim()!=='' ? searchResults.length : items.length)}
    <p class="col-span-full"><i>No files</i></p>
  {/if}
</div>
{/if}

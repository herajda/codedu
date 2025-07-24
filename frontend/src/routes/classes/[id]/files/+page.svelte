<script lang="ts">
import { onMount } from 'svelte';
import { page } from '$app/stores';
import { goto } from '$app/navigation';
import { get } from 'svelte/store';
import { auth } from '$lib/auth';
import { apiJSON, apiFetch } from '$lib/api';
import { compressImage } from '$lib/compressImage';
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
let uploadDialog: HTMLDialogElement;
let uploadFile: File | null = null;
let uploadErr = '';
let uploading = false;
const maxFileSize = 20 * 1024 * 1024;
let viewMode: 'grid' | 'list' =
  typeof localStorage !== 'undefined' &&
  localStorage.getItem('fileViewMode') === 'list'
    ? 'list'
    : 'grid';

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

function openUploadDialog() {
  uploadErr = '';
  uploadFile = null;
  uploading = false;
  uploadDialog.showModal();
}

async function doUpload() {
  if (!uploadFile) return;
  let file = uploadFile;
  if (isImage(file.name)) {
    try {
      file = await compressImage(file);
    } catch (e) {
      console.error('Image compression failed', e);
    }
  }
  if (file.size > maxFileSize) {
    uploadErr = 'File exceeds 20 MB limit';
    return;
  }
  const fd = new FormData();
  if (currentParent !== null) fd.append('parent_id', String(currentParent));
  fd.append('file', file);
  uploading = true;
  const res = await apiFetch(`/api/classes/${id}/files`, { method: 'POST', body: fd });
  if (!res.ok) {
    uploadErr = (await res.json()).error || res.statusText;
    uploading = false;
    return;
  }
  uploadDialog.close();
  uploading = false;
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

function toggleView() {
  viewMode = viewMode === 'grid' ? 'list' : 'grid';
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem('fileViewMode', viewMode);
  }
}
function fmtSize(bytes: number | null | undefined, decimals = 1) {
  if (bytes == null) return '';
  if (bytes < 1024) return `${bytes} B`;

  const units = ['KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
  let i = -1;
  do {
    bytes /= 1024;
    i++;
  } while (bytes >= 1024 && i < units.length - 1);

  return `${bytes.toFixed(decimals)} ${units[i]}`;
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
  <div class="flex items-center gap-2">
    <button class="btn btn-sm btn-circle" on:click={toggleView} title="Toggle view">
      {#if viewMode === 'grid'}
        <i class="fa-solid fa-list"></i>
      {:else}
        <i class="fa-solid fa-th"></i>
      {/if}
    </button>
  </div>
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
    </div>
    {#if role==='teacher' || role==='admin'}
    <div class="flex items-center gap-2">
    </div>
      <button class="btn btn-sm btn-circle" on:click={openUploadDialog} title="Upload file">
        <i class="fa-solid fa-upload"></i>
      </button>
      <button class="btn btn-sm btn-circle" on:click={promptDir} title="New folder">
        <i class="fa-solid fa-folder-plus"></i>
      </button>
      <button class="btn btn-sm btn-circle" on:click={promptNotebook} title="New notebook">
        <i class="fa-solid fa-book-medical"></i>
      </button>
    {/if}
</nav>

<dialog bind:this={uploadDialog} class="modal">
  <div class="modal-box w-11/12 max-w-md space-y-2">
    <h3 class="font-bold text-lg">Upload file</h3>
    {#if uploadErr}<p class="text-error">{uploadErr}</p>{/if}
    <input type="file" class="file-input file-input-bordered w-full" on:change={e => uploadFile=(e.target as HTMLInputElement).files?.[0] || null}>
    <div class="modal-action">
      <button class="btn" on:click={doUpload} disabled={!uploadFile || uploading}>
        {#if uploading}<span class="loading loading-dots"></span>{:else}Upload{/if}
      </button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

{#if loading}
<p>Loading…</p>
{:else if err}
<p class="text-error">{err}</p>
{:else}
{/if}

{#if viewMode === 'grid'}
  <!-- ── GRID VIEW ── -->
  <div class="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 mb-4">
    {#each displayed as it (it.id)}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="relative border rounded p-3 flex flex-col items-center group hover:shadow cursor-pointer"
           on:click={() => open(it)}>
        <div class="text-5xl mb-2">
          {#if it.is_dir}
            <i class="fa-solid fa-folder text-warning"></i>
          {:else if isImage(it.name)}
            <img src={`/api/files/${it.id}`} alt={it.name} class="w-16 h-16 object-cover rounded" />
          {:else}
            <i class="fa-solid {iconClass(it.name)}"></i>
          {/if}
        </div>

        <!-- filename -->
        <span class="text-sm text-center break-all">{it.name}</span>

        <!-- optional path shown only when searching -->
        {#if searchOpen && search.trim() !== ''}
          <span class="text-xs text-center text-gray-500 break-all">{it.path}</span>
        {/if}

        {#if role === 'teacher' || role === 'admin'}
          <div class="absolute top-1 right-1 hidden group-hover:flex gap-1">
            <button class="btn btn-xs btn-circle" title="Rename"
                    on:click|stopPropagation={() => rename(it)}>
              <i class="fa-solid fa-pen"></i>
            </button>
            <button class="btn btn-xs btn-circle btn-error" title="Delete"
                    on:click|stopPropagation={() => del(it)}>
              <i class="fa-solid fa-trash"></i>
            </button>
          </div>
        {/if}
      </div>
    {/each}

    {#if !displayed.length}
      <p class="col-span-full"><i>No files</i></p>
    {/if}
  </div>

{:else}
  <!-- ── LIST VIEW ── -->
  <div class="overflow-x-auto mb-4">
    <table class="table table-zebra w-full">
      <thead>
        <tr>
          <th class="text-left">Name</th>
          <th class="text-right">Size</th>
          <th class="text-right">Modified</th>
          {#if role === 'teacher' || role === 'admin'}<th class="w-16"></th>{/if}
        </tr>
      </thead>
      <tbody>
        {#each displayed as it (it.id)}
          <tr class="hover:bg-base-200 cursor-pointer group" on:click={() => open(it)}>
            <td class="whitespace-nowrap">
              {#if it.is_dir}
                <i class="fa-solid fa-folder text-warning mr-2"></i>{it.name}
              {:else if isImage(it.name)}
                <i class="fa-solid fa-file-image text-success mr-2"></i>{it.name}
              {:else}
                <i class="fa-solid {iconClass(it.name)} mr-2"></i>{it.name}
              {/if}
              <!-- show path in list view too (optional) -->
              {#if searchOpen && search.trim() !== ''}
                <div class="text-xs text-gray-500">{it.path}</div>
              {/if}
            </td>
            <td class="text-right">{it.is_dir ? '' : fmtSize(it.size)}</td>
            <td class="text-right">{new Date(it.updated_at).toLocaleString()}</td>

            {#if role === 'teacher' || role === 'admin'}
              <td class="text-right whitespace-nowrap w-16">
                <button class="btn btn-xs btn-circle invisible group-hover:visible" title="Rename"
                        on:click|stopPropagation={() => rename(it)}>
                  <i class="fa-solid fa-pen"></i>
                </button>
                <button class="btn btn-xs btn-circle btn-error invisible group-hover:visible" title="Delete"
                        on:click|stopPropagation={() => del(it)}>
                  <i class="fa-solid fa-trash"></i>
                </button>
              </td>
            {/if}
          </tr>
        {/each}

        {#if !displayed.length}
          <tr>
            <td colspan={role === 'teacher' || role === 'admin' ? 4 : 3}><i>No files</i></td>
          </tr>
        {/if}
      </tbody>
    </table>
  </div>
{/if}
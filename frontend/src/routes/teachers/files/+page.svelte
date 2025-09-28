<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/auth';
import { apiJSON, apiFetch } from '$lib/api';
import '@fortawesome/fontawesome-free/css/all.min.css';
import { compressImage } from '$lib/utils/compressImage';
import { formatDateTime } from "$lib/date";
import { TEACHER_GROUP_ID } from '$lib/teacherGroup';
import ConfirmModal from '$lib/components/ConfirmModal.svelte';
import PromptModal from '$lib/components/PromptModal.svelte';

// Fixed Teachers' group ID
let id = TEACHER_GROUP_ID;
let role = '';
$: role = $auth?.role ?? '';

let items:any[] = [];
let search = '';
let searchOpen = false;
let searchResults:any[] = [];
$: if (searchOpen && search.trim() !== '') { fetchSearch(search.trim()); } else { searchResults = []; }
$: displayed = searchOpen && search.trim() !== '' ? searchResults : items;

type FileFilter = 'all' | 'folders' | 'images' | 'notebooks' | 'documents' | 'code';
let filter: FileFilter = 'all';
let sortKey: 'name' | 'date' | 'size' = 'name';
let sortDir: 'asc' | 'desc' = 'asc';
$: visible = sortItems((displayed ?? []).filter(matchesFilter), sortKey, sortDir);

let breadcrumbs:{id:number|null,name:string}[] = [{id:null,name:'🏠'}];
let currentParent:number|null = null;
let loading = false;
let err = '';
let uploadDialog: HTMLDialogElement;
let uploadFile: File | null = null;
let uploadErr = '';
let uploading = false;
const maxFileSize = 20 * 1024 * 1024;
let viewMode: 'grid' | 'list' = typeof localStorage !== 'undefined' && localStorage.getItem('fileViewMode') === 'list' ? 'list' : 'grid';
let confirmModal: InstanceType<typeof ConfirmModal>;
let promptModal: InstanceType<typeof PromptModal>;

function toggleSearch() { searchOpen = !searchOpen; if (!searchOpen) search = ''; }
function isImage(name: string) { const ext = name.split('.').pop()?.toLowerCase(); return ['png','jpg','jpeg','gif','webp','svg'].includes(ext ?? ''); }
function iconClass(name: string) {
  const ext = name.split('.').pop()?.toLowerCase();
  switch (ext) {
    case 'pdf': return 'fa-file-pdf text-error';
    case 'png': case 'jpg': case 'jpeg': case 'gif': return 'fa-file-image text-success';
    case 'zip': case 'tar': case 'gz': return 'fa-file-zipper';
    case 'ipynb': return 'fa-book text-secondary';
    case 'js': case 'ts': case 'svelte': case 'py': case 'go': case 'java': case 'cpp': return 'fa-file-code text-primary';
    default: return 'fa-file';
  }
}

function displayName(name: string | null | undefined) {
  if (!name) return '';
  const lastDot = name.lastIndexOf('.');
  if (lastDot <= 0) return name;
  return name.slice(0, lastDot);
}

function open(item: any) {
  if (item.is_dir) openDir(item);
  else if (item.name.toLowerCase().endsWith('.ipynb') || isImage(item.name)) goto(`/files/${item.id}`);
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
  try{ searchResults = await apiJSON(`/api/classes/${id}/files?search=${encodeURIComponent(q)}`); }
  catch(e:any){ err = e.message }
  loading = false;
}

async function openDir(item:any){
  breadcrumbs = [...breadcrumbs, {id:item.id,name:item.name}];
  if (typeof sessionStorage !== 'undefined') {
    sessionStorage.setItem(`files_breadcrumbs_${id}`, JSON.stringify(breadcrumbs));
    sessionStorage.setItem(`files_parent_${id}`, String(item.id));
  }
  await load(item.id);
}

function crumbTo(i:number){
  const b = breadcrumbs[i];
  breadcrumbs = breadcrumbs.slice(0,i+1);
  if (typeof sessionStorage !== 'undefined') {
    sessionStorage.setItem(`files_breadcrumbs_${id}`, JSON.stringify(breadcrumbs));
    sessionStorage.setItem(`files_parent_${id}`, b.id === null ? '' : String(b.id));
  }
  load(b.id);
}

function openUploadDialog() { uploadErr = ''; uploadFile = null; uploading = false; uploadDialog.showModal(); }

async function doUpload() {
  if (!uploadFile) return;
  let fileToUpload = uploadFile;
  if (uploadFile.type.startsWith('image/')) { try { fileToUpload = await compressImage(uploadFile); } catch {} }
  if (fileToUpload.size > maxFileSize) { uploadErr = 'File exceeds 20 MB limit'; return; }
  const fd = new FormData();
  if (currentParent !== null) fd.append('parent_id', String(currentParent));
  fd.append('file', fileToUpload);
  uploading = true;
  const res = await apiFetch(`/api/classes/${id}/files`, { method: 'POST', body: fd });
  if (!res.ok) { uploadErr = (await res.json()).error || res.statusText; uploading = false; return; }
  uploadDialog.close(); uploading = false; await load(currentParent);
}

async function createDir(name:string){
  await apiFetch(`/api/classes/${id}/files`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, parent_id: currentParent, is_dir: true }) });
  await load(currentParent);
}

async function promptDir() {
  const name = await promptModal?.open({
    title: 'New folder',
    label: 'Folder name',
    placeholder: 'e.g. Resources',
    confirmLabel: 'Create',
    icon: 'fa-solid fa-folder-plus text-primary',
    validate: (value) => value.trim() ? null : 'Folder name is required',
    transform: (value) => value.trim()
  });
  if (!name) return;
  await createDir(name);
}

async function createNotebook(name: string) {
  const cf = await apiJSON(`/api/classes/${id}/files`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, parent_id: currentParent }) });
  const nb = { cells: [], metadata: {}, nbformat: 4, nbformat_minor: 5 };
  await apiFetch(`/api/files/${cf.id}/content`, { method: 'PUT', body: JSON.stringify(nb) });
  await load(currentParent);
}

async function promptNotebook() {
  const notebookName = await promptModal?.open({
    title: 'New notebook',
    label: 'Notebook name',
    initialValue: 'Untitled.ipynb',
    helpText: 'Saved as a Jupyter notebook (.ipynb).',
    confirmLabel: 'Create',
    icon: 'fa-solid fa-book text-secondary',
    validate: (value) => value.trim() ? null : 'Notebook name is required',
    transform: (value) => {
      const trimmed = value.trim();
      if (!trimmed.toLowerCase().endsWith('.ipynb')) return `${trimmed}.ipynb`;
      return trimmed;
    }
  });
  if (!notebookName) return;
  await createNotebook(notebookName);
}

async function del(item:any){
  const confirmed = await confirmModal.open({
    title: item.is_dir ? 'Delete folder' : 'Delete file',
    body: item.is_dir ? 'Everything inside this folder will be removed.' : 'This file will be permanently deleted.',
    confirmLabel: 'Delete',
    confirmClass: 'btn btn-error',
    cancelClass: 'btn'
  });
  if(!confirmed) return;
  await apiFetch(`/api/files/${item.id}`,{method:'DELETE'});
  await load(currentParent);
}

async function rename(item:any){
  const name = await promptModal?.open({
    title: 'Rename',
    label: 'New name',
    initialValue: item.name,
    confirmLabel: 'Save',
    icon: item.is_dir ? 'fa-solid fa-folder text-warning' : 'fa-solid fa-pen text-primary',
    validate: (value) => value.trim() ? null : 'Name is required',
    transform: (value) => value.trim(),
    selectOnOpen: true
  });
  if (!name || name === item.name) return;
  await apiFetch(`/api/files/${item.id}`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({name})});
  await load(currentParent);
}

function toggleView() { viewMode = viewMode === 'grid' ? 'list' : 'grid'; if (typeof localStorage !== 'undefined') { localStorage.setItem('fileViewMode', viewMode); } }

function matchesFilter(it: any): boolean {
  if (filter === 'all') return true;
  if (filter === 'folders') return !!it.is_dir;
  if (filter === 'images') return !it.is_dir && isImage(it.name);
  if (filter === 'notebooks') return !it.is_dir && (it.name?.toLowerCase?.().endsWith('.ipynb'));
  if (filter === 'documents') return !it.is_dir && (it.name?.toLowerCase?.().endsWith('.pdf'));
  if (filter === 'code') return !it.is_dir && ['js','ts','svelte','py','go','java','cpp'].includes(it.name?.split('.').pop()?.toLowerCase?.() ?? '');
  return true;
}

function sortItems(arr: any[], key: 'name' | 'date' | 'size', dir: 'asc' | 'desc') {
  const sorted = [...arr].sort((a, b) => {
    let va: any; let vb: any;
    switch (key) {
      case 'size': va = a.size ?? 0; vb = b.size ?? 0; break;
      case 'date': va = new Date(a.updated_at ?? a.created_at ?? 0).getTime(); vb = new Date(b.updated_at ?? b.created_at ?? 0).getTime(); break;
      case 'name': default: va = (a.name ?? '').toLowerCase(); vb = (b.name ?? '').toLowerCase();
    }
    if (va < vb) return dir === 'asc' ? -1 : 1;
    if (va > vb) return dir === 'asc' ? 1 : -1;
    return 0;
  });
  if (key === 'name') { sorted.sort((a, b) => (b.is_dir ? 1 : 0) - (a.is_dir ? 1 : 0)); }
  return sorted;
}

// Drag & drop upload support
let isDragging = false; let dragDepth = 0; let dropping = false; let dropErr = '';
function onDragEnter() { dragDepth += 1; isDragging = true; }
function onDragLeave() { dragDepth -= 1; if (dragDepth <= 0) { isDragging = false; dragDepth = 0; } }
function onDragOver() {}
async function onDrop(e: DragEvent) {
  dragDepth = 0; isDragging = false; dropErr = '';
  const files = Array.from(e.dataTransfer?.files ?? []);
  if (!files.length) return;
  try { dropping = true; await uploadFiles(files); await load(currentParent); }
  catch (er:any) { dropErr = er?.message ?? 'Failed to upload'; }
  finally { dropping = false; }
}

async function uploadFiles(files: File[]) {
  for (const f of files) {
    let fileToUpload: File = f;
    if (f.type.startsWith('image/')) { try { fileToUpload = await compressImage(f); } catch {} }
    if (fileToUpload.size > maxFileSize) { throw new Error(`${f.name} exceeds 20 MB limit`); }
    const fd = new FormData();
    if (currentParent !== null) fd.append('parent_id', String(currentParent));
    fd.append('file', fileToUpload);
    const res = await apiFetch(`/api/classes/${id}/files`, { method: 'POST', body: fd });
    if (!res.ok) { const js = await res.json().catch(() => ({})); throw new Error(js.error || res.statusText); }
  }
}

function fmtSize(bytes: number | null | undefined, decimals = 1) {
  if (bytes == null) return '';
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
  let i = -1; do { bytes /= 1024; i++; } while (bytes >= 1024 && i < units.length - 1);
  return `${bytes.toFixed(decimals)} ${units[i]}`;
}

onMount(() => {
  let storedParent: string | null = null;
  if (typeof sessionStorage !== 'undefined') {
    const sp = sessionStorage.getItem(`files_parent_${id}`);
    if (sp) { storedParent = sp; }
    const bc = sessionStorage.getItem(`files_breadcrumbs_${id}`);
    if (bc) { try { breadcrumbs = JSON.parse(bc); } catch {} }
  }
  load(storedParent);
});
</script>

<div class="relative" on:dragenter|preventDefault={onDragEnter} on:dragover|preventDefault={onDragOver} on:dragleave|preventDefault={onDragLeave} on:drop|preventDefault={onDrop}>
  <nav class="mb-4 sticky top-16 z-30 bg-base-200 rounded-box shadow px-4 py-2 flex items-center flex-wrap gap-2">
    <ul class="flex flex-wrap gap-1 text-sm items-center flex-grow">
      {#each breadcrumbs as b,i}
        <li class="after:mx-1 after:content-['/'] last:after:hidden">
          <button type="button" class="link px-2 py-1 rounded hover:bg-base-300" on:click={() => crumbTo(i)} aria-label={`Open ${b.name}`}>{b.name}</button>
        </li>
      {/each}
    </ul>
    <div class="flex items-center gap-2 ml-auto">
      <button class="btn btn-sm btn-circle" on:click={toggleView} title="Toggle view">
        {#if viewMode === 'grid'}
          <i class="fa-solid fa-list"></i>
        {:else}
          <i class="fa-solid fa-th"></i>
        {/if}
      </button>
      <div class="relative">
        {#if searchOpen}
          <input class="input input-bordered input-sm w-48 sm:w-72" placeholder="Search files..." bind:value={search} />
        {:else}
          <button class="btn btn-sm" on:click={toggleSearch}><i class="fa-solid fa-magnifying-glass mr-2"></i>Search</button>
        {/if}
      </div>
      {#if role === 'teacher' || role === 'admin'}
        <button class="btn btn-sm" on:click={() => openUploadDialog()}><i class="fa-solid fa-upload mr-2"></i>Upload</button>
        <button class="btn btn-sm" on:click={promptDir}><i class="fa-solid fa-folder-plus mr-2"></i>Folder</button>
        <button class="btn btn-sm" on:click={promptNotebook}><i class="fa-solid fa-book mr-2"></i>Notebook</button>
      {/if}
      <div class="flex items-center gap-2">
        <select class="select select-bordered select-sm" bind:value={filter}>
          <option value="all">All</option>
          <option value="folders">Folders</option>
          <option value="images">Images</option>
          <option value="notebooks">Notebooks</option>
          <option value="documents">Documents</option>
          <option value="code">Code</option>
        </select>
        <select class="select select-bordered select-sm" bind:value={sortKey}>
          <option value="name">Name</option>
          <option value="date">Modified</option>
          <option value="size">Size</option>
        </select>
        <select class="select select-bordered select-sm" bind:value={sortDir}>
          <option value="asc">Asc</option>
          <option value="desc">Desc</option>
        </select>
      </div>
    </div>
  </nav>

  {#if err}
    <p class="text-error">{err}</p>
  {/if}
  {#if loading}
    <p>Loading…</p>
  {/if}

  {#if viewMode === 'grid'}
    <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3">
      {#each visible as it (it.id)}
        <div class="relative card bg-base-100 shadow hover:shadow-lg transition-shadow cursor-pointer group p-3" on:click={() => open(it)} title={it.path}>
          <div class="text-5xl mb-2">
            {#if it.is_dir}
              <i class="fa-solid fa-folder text-warning"></i>
            {:else if isImage(it.name)}
              <img src={`/api/files/${it.id}`} alt={it.name} class="w-16 h-16 object-cover rounded" />
            {:else}
              <i class="fa-solid {iconClass(it.name)}"></i>
            {/if}
          </div>
          <span class="text-sm text-center break-all">{it.is_dir ? it.name : displayName(it.name)}</span>
          <div class="mt-1 text-xs text-gray-500">
            {#if !it.is_dir}
              <span>{fmtSize(it.size)}</span>
              <span class="mx-1">·</span>
            {/if}
            <span>{formatDateTime(it.updated_at)}</span>
          </div>
          {#if searchOpen && search.trim() !== ''}
            <span class="text-xs text-center text-gray-500 break-all">{it.path}</span>
          {/if}
          {#if role === 'teacher' || role === 'admin'}
            <div class="absolute top-1 right-1 hidden group-hover:flex gap-1">
              <button class="btn btn-xs btn-circle" title="Rename" aria-label="Rename" on:click|stopPropagation={() => rename(it)}><i class="fa-solid fa-pen"></i></button>
              <button class="btn btn-xs btn-circle btn-error" title="Delete" aria-label="Delete" on:click|stopPropagation={() => del(it)}><i class="fa-solid fa-trash"></i></button>
            </div>
          {/if}
        </div>
      {/each}
      {#if !visible.length}
        <p class="col-span-full"><i>No files</i></p>
      {/if}
    </div>
  {:else}
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
          {#each visible as it (it.id)}
            <tr class="hover:bg-base-200 cursor-pointer group" on:click={() => open(it)}>
              <td class="whitespace-nowrap">
                {#if it.is_dir}
                  <i class="fa-solid fa-folder text-warning mr-2"></i>{it.name}
                {:else if isImage(it.name)}
                  <i class="fa-solid fa-file-image text-success mr-2"></i>{it.name}
                {:else}
                  <i class="fa-solid {iconClass(it.name)} mr-2"></i>{it.name}
                {/if}
                {#if searchOpen && search.trim() !== ''}
                  <div class="text-xs text-gray-500">{it.path}</div>
                {/if}
              </td>
              <td class="text-right">{it.is_dir ? '' : fmtSize(it.size)}</td>
              <td class="text-right">{formatDateTime(it.updated_at)}</td>
              {#if role === 'teacher' || role === 'admin'}
                <td class="text-right whitespace-nowrap w-16">
                  <button class="btn btn-xs btn-circle invisible group-hover:visible" title="Rename" aria-label="Rename" on:click|stopPropagation={() => rename(it)}><i class="fa-solid fa-pen"></i></button>
                  <button class="btn btn-xs btn-circle btn-error invisible group-hover:visible" title="Delete" aria-label="Delete" on:click|stopPropagation={() => del(it)}><i class="fa-solid fa-trash"></i></button>
                </td>
              {/if}
            </tr>
          {/each}
          {#if !visible.length}
            <tr>
              <td colspan={role === 'teacher' || role === 'admin' ? 4 : 3}><i>No files</i></td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>
  {/if}

  {#if isDragging}
    <div class="absolute inset-0 z-20 border-4 border-dashed border-primary/60 bg-base-100/70 backdrop-blur-sm rounded-box flex items-center justify-center">
      <div class="text-center">
        <i class="fa-solid fa-cloud-arrow-up text-4xl mb-2"></i>
        <p class="font-medium">Drop files to upload</p>
      </div>
    </div>
  {/if}

  {#if dropErr}
    <div class="alert alert-error mt-2"><i class="fa-solid fa-triangle-exclamation"></i><span>{dropErr}</span></div>
  {/if}

  {#if dropping}
    <div class="fixed bottom-4 right-4 z-30"><div class="btn btn-primary btn-sm no-animation"><span class="loading loading-dots loading-xs mr-2"></span>Uploading…</div></div>
  {/if}

  <dialog bind:this={uploadDialog} class="modal">
    <div class="modal-box">
      <h3 class="font-bold mb-2">Upload file</h3>
      <input type="file" class="file-input file-input-bordered w-full" on:change={(e)=>uploadFile=(e.target as HTMLInputElement).files?.[0]||null} />
      {#if uploadErr}<p class="text-error mt-2">{uploadErr}</p>{/if}
      <div class="modal-action">
        <form method="dialog"><button class="btn">Cancel</button></form>
        <button class="btn btn-primary" on:click|preventDefault={doUpload} disabled={uploading}>Upload</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>
  <ConfirmModal bind:this={confirmModal} />
  <PromptModal bind:this={promptModal} />
</div>

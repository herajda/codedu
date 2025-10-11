<script lang="ts">
import { onMount } from 'svelte';
import { goto } from '$app/navigation';
import { auth } from '$lib/auth';
import { classesStore } from '$lib/stores/classes';
import { apiJSON, apiFetch } from '$lib/api';
import '@fortawesome/fontawesome-free/css/all.min.css';
import { compressImage } from '$lib/utils/compressImage';
import { formatDateTime } from "$lib/date";
import { TEACHER_GROUP_ID } from '$lib/teacherGroup';
import ConfirmModal from '$lib/components/ConfirmModal.svelte';
import PromptModal from '$lib/components/PromptModal.svelte';
import { t, translator } from '$lib/i18n';

let translate;
$: translate = $translator;

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
let copyDialog: HTMLDialogElement;
let copyItem: any = null;
let copyErr = '';
let copyLoading = false;
let copyFolders: any[] = [];
let copyBreadcrumbs: { id: string | null; name: string }[] = [{ id: null, name: '🏠' }];
let copyParent: string | null = null;
let copyName = '';
let copying = false;
let selectedClassId = '';

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
  if (fileToUpload.size > maxFileSize) { uploadErr = t('frontend/src/routes/teachers/files/+page.svelte::upload_error_size_limit'); return; }
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
    title: t('frontend/src/routes/teachers/files/+page.svelte::new_folder_title'),
    label: t('frontend/src/routes/teachers/files/+page.svelte::folder_name_label'),
    placeholder: t('frontend/src/routes/teachers/files/+page.svelte::folder_name_placeholder'),
    confirmLabel: t('frontend/src/routes/teachers/files/+page.svelte::create_button_label'),
    icon: 'fa-solid fa-folder-plus text-primary',
    validate: (value) => value.trim() ? null : t('frontend/src/routes/teachers/files/+page.svelte::folder_name_required'),
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
    title: t('frontend/src/routes/teachers/files/+page.svelte::new_notebook_title'),
    label: t('frontend/src/routes/teachers/files/+page.svelte::notebook_name_label'),
    initialValue: 'Untitled.ipynb',
    helpText: t('frontend/src/routes/teachers/files/+page.svelte::notebook_help_text'),
    confirmLabel: t('frontend/src/routes/teachers/files/+page.svelte::create_button_label'),
    icon: 'fa-solid fa-book text-secondary',
    validate: (value) => value.trim() ? null : t('frontend/src/routes/teachers/files/+page.svelte::notebook_name_required'),
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
    title: item.is_dir ? t('frontend/src/routes/teachers/files/+page.svelte::delete_folder_title') : t('frontend/src/routes/teachers/files/+page.svelte::delete_file_title'),
    body: item.is_dir ? t('frontend/src/routes/teachers/files/+page.svelte::delete_folder_body') : t('frontend/src/routes/teachers/files/+page.svelte::delete_file_body'),
    confirmLabel: t('frontend/src/routes/teachers/files/+page.svelte::delete_button_label'),
    confirmClass: 'btn btn-error',
    cancelClass: 'btn'
  });
  if(!confirmed) return;
  await apiFetch(`/api/files/${item.id}`,{method:'DELETE'});
  await load(currentParent);
}

async function rename(item:any){
  const name = await promptModal?.open({
    title: t('frontend/src/routes/teachers/files/+page.svelte::rename_title'),
    label: t('frontend/src/routes/teachers/files/+page.svelte::new_name_label'),
    initialValue: item.name,
    confirmLabel: t('frontend/src/routes/teachers/files/+page.svelte::save_button_label'),
    icon: item.is_dir ? 'fa-solid fa-folder text-warning' : 'fa-solid fa-pen text-primary',
    validate: (value) => value.trim() ? null : t('frontend/src/routes/teachers/files/+page.svelte::name_required'),
    transform: (value) => value.trim(),
    selectOnOpen: true
  });
  if (!name || name === item.name) return;
  await apiFetch(`/api/files/${item.id}`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({name})});
  await load(currentParent);
}

async function ensureClassesLoaded() {
  if (!$classesStore.classes.length && !$classesStore.loading) {
    await classesStore.load();
  }
}

async function loadDestinationFolders(classId: string | null, parent: string | null) {
  if (!classId) return;
  copyLoading = true;
  copyErr = '';
  copyFolders = [];
  try {
    const q = parent === null ? '' : `?parent=${parent}`;
    const files = await apiJSON(`/api/classes/${classId}/files${q}`);
    copyFolders = files.filter((f: any) => f.is_dir);
    copyParent = parent;
  } catch (e: any) {
    copyErr = e?.message ?? 'Failed to load destination folders';
  }
  copyLoading = false;
}

async function openCopyToClass(item: any) {
  if (!item || item.is_dir) return;
  copyItem = item;
  copyName = item.name ?? '';
  copyErr = '';
  copyFolders = [];
  copyBreadcrumbs = [{ id: null, name: '🏠' }];
  copyParent = null;
  try {
    await ensureClassesLoaded();
  } catch (e: any) {
    copyErr = e?.message ?? 'Failed to load classes';
  }
  const classes = $classesStore.classes;
  if (!classes.length) {
    copyErr = copyErr || 'You do not have any classes yet.';
  } else {
    if (!selectedClassId || !classes.some((c) => String(c.id) === String(selectedClassId))) {
      selectedClassId = String(classes[0].id);
    }
    await loadDestinationFolders(selectedClassId, null);
  }
  copyDialog?.showModal();
}

function copyCrumbTo(index: number) {
  if (!selectedClassId) return;
  const crumb = copyBreadcrumbs[index];
  copyBreadcrumbs = copyBreadcrumbs.slice(0, index + 1);
  loadDestinationFolders(selectedClassId, crumb.id);
}

async function openDestinationFolder(folder: any) {
  if (!folder?.is_dir || !selectedClassId) return;
  copyBreadcrumbs = [...copyBreadcrumbs, { id: folder.id, name: folder.name }];
  await loadDestinationFolders(selectedClassId, folder.id);
}

function classDestinationPath() {
  return copyBreadcrumbs.map((b) => b.name).join(' / ');
}

async function handleClassChange(value: string) {
  selectedClassId = value || '';
  copyBreadcrumbs = [{ id: null, name: '🏠' }];
  copyParent = null;
  await loadDestinationFolders(selectedClassId, null);
}

async function doCopyToClass() {
  if (!copyItem) return;
  const destinationClass = selectedClassId;
  if (!destinationClass) {
    copyErr = 'Please select a destination class';
    return;
  }
  const trimmedName = copyName.trim();
  if (!trimmedName) {
    copyErr = 'File name is required';
    return;
  }
  copyErr = '';
  copying = true;
  const payload: any = { target_class_id: destinationClass };
  if (copyParent) payload.target_parent_id = copyParent;
  if (trimmedName !== (copyItem.name ?? '')) payload.new_name = trimmedName;
  try {
    const res = await apiFetch(`/api/files/${copyItem.id}/copy`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (!res.ok) {
      const js = await res.json().catch(() => ({}));
      copyErr = js?.error ?? res.statusText;
      copying = false;
      return;
    }
    await res.json().catch(() => null);
    copyDialog?.close();
    resetCopyState();
  } catch (e: any) {
    copyErr = e?.message ?? 'Failed to copy file';
  }
  copying = false;
}

function resetCopyState() {
  copyItem = null;
  copyErr = '';
  copyFolders = [];
  copyBreadcrumbs = [{ id: null, name: '🏠' }];
  copyParent = null;
  copyName = '';
  copyLoading = false;
  copying = false;
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
  catch (er:any) { dropErr = er?.message ?? t('frontend/src/routes/teachers/files/+page.svelte::drop_error_failed_to_upload'); }
  finally { dropping = false; }
}

async function uploadFiles(files: File[]) {
  for (const f of files) {
    let fileToUpload: File = f;
    if (f.type.startsWith('image/')) { try { fileToUpload = await compressImage(f); } catch {} }
    if (fileToUpload.size > maxFileSize) { throw new Error(t('frontend/src/routes/teachers/files/+page.svelte::file_size_limit_error', { filename: f.name })); }
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
  load(storedParent ? parseInt(storedParent, 10) : null);
});
</script>

<div class="relative" on:dragenter|preventDefault={onDragEnter} on:dragover|preventDefault={onDragOver} on:dragleave|preventDefault={onDragLeave} on:drop|preventDefault={onDrop}>
  <nav class="mb-4 sticky top-16 z-30 bg-base-200 rounded-box shadow px-4 py-2 flex items-center flex-wrap gap-2">
    <ul class="flex flex-wrap gap-1 text-sm items-center flex-grow">
      {#each breadcrumbs as b,i}
        <li class="after:mx-1 after:content-['/'] last:after:hidden">
          <button type="button" class="link px-2 py-1 rounded hover:bg-base-300" on:click={() => crumbTo(i)} aria-label={translate('frontend/src/routes/teachers/files/+page.svelte::open_breadcrumb_aria_label', { name: b.name })}>{b.name}</button>
        </li>
      {/each}
    </ul>
    <div class="flex items-center gap-2 ml-auto">
      <button class="btn btn-sm btn-circle" on:click={toggleView} title={translate('frontend/src/routes/teachers/files/+page.svelte::toggle_view_title')}>
        {#if viewMode === 'grid'}
          <i class="fa-solid fa-list"></i>
        {:else}
          <i class="fa-solid fa-th"></i>
        {/if}
      </button>
      <div class="relative">
        {#if searchOpen}
          <input class="input input-bordered input-sm w-48 sm:w-72" placeholder={translate('frontend/src/routes/teachers/files/+page.svelte::search_files_placeholder')} bind:value={search} />
        {:else}
          <button class="btn btn-sm" on:click={toggleSearch}><i class="fa-solid fa-magnifying-glass mr-2"></i>{translate('frontend/src/routes/teachers/files/+page.svelte::search_button_label')}</button>
        {/if}
      </div>
      {#if role === 'teacher' || role === 'admin'}
        <button class="btn btn-sm" on:click={() => openUploadDialog()}><i class="fa-solid fa-upload mr-2"></i>{translate('frontend/src/routes/teachers/files/+page.svelte::upload_button_label')}</button>
        <button class="btn btn-sm" on:click={promptDir}><i class="fa-solid fa-folder-plus mr-2"></i>{translate('frontend/src/routes/teachers/files/+page.svelte::folder_button_label')}</button>
        <button class="btn btn-sm" on:click={promptNotebook}><i class="fa-solid fa-book mr-2"></i>{translate('frontend/src/routes/teachers/files/+page.svelte::notebook_button_label')}</button>
      {/if}
      <div class="flex items-center gap-2">
        <select class="select select-bordered select-sm" bind:value={filter}>
          <option value="all">{translate('frontend/src/routes/teachers/files/+page.svelte::filter_all')}</option>
          <option value="folders">{translate('frontend/src/routes/teachers/files/+page.svelte::filter_folders')}</option>
          <option value="images">{translate('frontend/src/routes/teachers/files/+page.svelte::filter_images')}</option>
          <option value="notebooks">{translate('frontend/src/routes/teachers/files/+page.svelte::filter_notebooks')}</option>
          <option value="documents">{translate('frontend/src/routes/teachers/files/+page.svelte::filter_documents')}</option>
          <option value="code">{translate('frontend/src/routes/teachers/files/+page.svelte::filter_code')}</option>
        </select>
        <select class="select select-bordered select-sm" bind:value={sortKey}>
          <option value="name">{translate('frontend/src/routes/teachers/files/+page.svelte::sort_name')}</option>
          <option value="date">{translate('frontend/src/routes/teachers/files/+page.svelte::sort_modified')}</option>
          <option value="size">{translate('frontend/src/routes/teachers/files/+page.svelte::sort_size')}</option>
        </select>
        <select class="select select-bordered select-sm" bind:value={sortDir}>
          <option value="asc">{translate('frontend/src/routes/teachers/files/+page.svelte::sort_asc')}</option>
          <option value="desc">{translate('frontend/src/routes/teachers/files/+page.svelte::sort_desc')}</option>
        </select>
      </div>
    </div>
  </nav>

  {#if err}
    <p class="text-error">{err}</p>
  {/if}
  {#if loading}
    <p>{translate('frontend/src/routes/teachers/files/+page.svelte::loading_message')}</p>
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
              {#if !it.is_dir}
                <button class="btn btn-xs btn-circle btn-outline" title={translate('frontend/src/routes/teachers/files/+page.svelte::copy_to_class')} aria-label={translate('frontend/src/routes/teachers/files/+page.svelte::copy_to_class_label')} on:click|stopPropagation={() => openCopyToClass(it)}>
                  <i class="fa-solid fa-copy"></i>
                </button>
              {/if}
              <button class="btn btn-xs btn-circle" title={translate('frontend/src/routes/teachers/files/+page.svelte::rename_button_title')} aria-label={translate('frontend/src/routes/teachers/files/+page.svelte::rename_button_aria_label')} on:click|stopPropagation={() => rename(it)}><i class="fa-solid fa-pen"></i></button>
              <button class="btn btn-xs btn-circle btn-error" title={translate('frontend/src/routes/teachers/files/+page.svelte::delete_button_title')} aria-label={translate('frontend/src/routes/teachers/files/+page.svelte::delete_button_aria_label')} on:click|stopPropagation={() => del(it)}><i class="fa-solid fa-trash"></i></button>
            </div>
          {/if}
        </div>
      {/each}
      {#if !visible.length}
        <p class="col-span-full"><i>{translate('frontend/src/routes/teachers/files/+page.svelte::no_files_message')}</i></p>
      {/if}
    </div>
  {:else}
    <div class="overflow-x-auto mb-4">
      <table class="table table-zebra w-full">
        <thead>
          <tr>
            <th class="text-left">{translate('frontend/src/routes/teachers/files/+page.svelte::table_header_name')}</th>
            <th class="text-right">{translate('frontend/src/routes/teachers/files/+page.svelte::table_header_size')}</th>
            <th class="text-right">{translate('frontend/src/routes/teachers/files/+page.svelte::table_header_modified')}</th>
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
                  {#if !it.is_dir}
                    <button class="btn btn-xs btn-circle btn-outline invisible group-hover:visible" title={translate('frontend/src/routes/teachers/files/+page.svelte::copy_to_class')} aria-label={translate('frontend/src/routes/teachers/files/+page.svelte::copy_to_class_label')} on:click|stopPropagation={() => openCopyToClass(it)}>
                      <i class="fa-solid fa-copy"></i>
                    </button>
                  {/if}
                  <button class="btn btn-xs btn-circle invisible group-hover:visible" title={translate('frontend/src/routes/teachers/files/+page.svelte::rename_button_title')} aria-label={translate('frontend/src/routes/teachers/files/+page.svelte::rename_button_aria_label')} on:click|stopPropagation={() => rename(it)}><i class="fa-solid fa-pen"></i></button>
                  <button class="btn btn-xs btn-circle btn-error invisible group-hover:visible" title={translate('frontend/src/routes/teachers/files/+page.svelte::delete_button_title')} aria-label={translate('frontend/src/routes/teachers/files/+page.svelte::delete_button_aria_label')} on:click|stopPropagation={() => del(it)}><i class="fa-solid fa-trash"></i></button>
                </td>
              {/if}
            </tr>
          {/each}
          {#if !visible.length}
            <tr>
              <td colspan={role === 'teacher' || role === 'admin' ? 4 : 3}><i>{translate('frontend/src/routes/teachers/files/+page.svelte::no_files_message')}</i></td>
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
        <p class="font-medium">{translate('frontend/src/routes/teachers/files/+page.svelte::drop_files_to_upload')}</p>
      </div>
    </div>
  {/if}

  {#if dropErr}
    <div class="alert alert-error mt-2"><i class="fa-solid fa-triangle-exclamation"></i><span>{dropErr}</span></div>
  {/if}

  {#if dropping}
    <div class="fixed bottom-4 right-4 z-30"><div class="btn btn-primary btn-sm no-animation"><span class="loading loading-dots loading-xs mr-2"></span>{translate('frontend/src/routes/teachers/files/+page.svelte::uploading_status')}</div></div>
  {/if}

  <dialog bind:this={uploadDialog} class="modal">
    <div class="modal-box">
      <h3 class="font-bold mb-2">{translate('frontend/src/routes/teachers/files/+page.svelte::upload_file_title')}</h3>
      <input type="file" class="file-input file-input-bordered w-full" on:change={(e)=>uploadFile=(e.target as HTMLInputElement).files?.[0]||null} />
      {#if uploadErr}<p class="text-error mt-2">{uploadErr}</p>{/if}
      <div class="modal-action">
        <form method="dialog"><button class="btn">{translate('frontend/src/routes/teachers/files/+page.svelte::cancel_button_label')}</button></form>
        <button class="btn btn-primary" on:click|preventDefault={doUpload} disabled={uploading}>{translate('frontend/src/routes/teachers/files/+page.svelte::upload_button_label_dialog')}</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>
  <dialog bind:this={copyDialog} class="modal" on:close={resetCopyState}>
    <div class="modal-box max-w-2xl space-y-4">
      <h3 class="font-bold text-lg">{translate('frontend/src/routes/teachers/files/+page.svelte::copy_to_class')}</h3>
      {#if copyItem}
        <p class="text-sm text-base-content/70 break-all">Source file: {copyItem.name}</p>
      {/if}
      <div class="form-control">
        <div class="label">
          <span class="label-text">Destination class</span>
        </div>
        <select
          class="select select-bordered"
          bind:value={selectedClassId}
          on:change={(e) => handleClassChange((e.target as HTMLSelectElement).value)}
          disabled={$classesStore.loading}
        >
          {#if $classesStore.loading && !$classesStore.classes.length}
            <option value="" disabled selected>Loading classes…</option>
          {:else if !$classesStore.classes.length}
            <option value="" disabled selected>No classes available</option>
          {:else}
            {#each $classesStore.classes as cls}
              <option value={String(cls.id)}>{cls.name}</option>
            {/each}
          {/if}
        </select>
        {#if $classesStore.error}
          <div class="label">
            <span class="label-text-alt text-error">{$classesStore.error}</span>
          </div>
        {/if}
      </div>
      {#if selectedClassId}
        <div>
          <div class="flex items-center justify-between">
            <span class="text-sm font-medium">Destination folder</span>
            <button type="button" class="btn btn-ghost btn-xs" on:click={() => loadDestinationFolders(selectedClassId, copyParent)} disabled={copyLoading}>
              <i class="fa-solid fa-rotate-right mr-1"></i>Refresh
            </button>
          </div>
          <nav class="text-xs mt-1">
            <ul class="flex flex-wrap gap-1 items-center">
              {#each copyBreadcrumbs as b, i}
                <li class="after:mx-1 after:content-['/'] last:after:hidden">
                  <button type="button" class="link px-2 py-1 rounded hover:bg-base-300" on:click={() => copyCrumbTo(i)}>
                    {b.name}
                  </button>
                </li>
              {/each}
            </ul>
          </nav>
          <p class="text-xs text-base-content/70 mt-1">Current folder: {classDestinationPath()}</p>
        </div>
      {/if}
      {#if copyErr}
        <div class="alert alert-error text-sm">
          <i class="fa-solid fa-triangle-exclamation"></i>
          <span>{copyErr}</span>
        </div>
      {/if}
      <label class="form-control w-full">
        <div class="label">
          <span class="label-text">File name</span>
        </div>
        <input class="input input-bordered w-full" bind:value={copyName} />
      </label>
      {#if selectedClassId}
        <div class="border border-base-300 rounded-box max-h-64 overflow-y-auto">
          {#if copyLoading}
            <div class="p-4 text-sm">Loading folders…</div>
          {:else if !copyFolders.length}
            <div class="p-4 text-sm opacity-70">No subfolders. File will be placed in {classDestinationPath()}.</div>
          {:else}
            <ul class="menu menu-sm bg-base-200/40">
              {#each copyFolders as folder}
                <li>
                  <button type="button" on:click={() => openDestinationFolder(folder)}>
                    <i class="fa-solid fa-folder text-warning mr-2"></i>{folder.name}
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {:else}
        <div class="border border-base-300 rounded-box p-4 text-sm opacity-70">Select a class to browse its folders.</div>
      {/if}
      <div class="modal-action">
        <form method="dialog"><button class="btn">Cancel</button></form>
        <button class="btn btn-primary" on:click|preventDefault={doCopyToClass} disabled={copying || !selectedClassId || !$classesStore.classes.length}>
          {#if copying}<span class="loading loading-dots loading-sm mr-2"></span>{/if}
          {translate('frontend/src/routes/teachers/files/+page.svelte::copy_here')}
        </button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>
  <ConfirmModal bind:this={confirmModal} />
  <PromptModal bind:this={promptModal} />
</div>

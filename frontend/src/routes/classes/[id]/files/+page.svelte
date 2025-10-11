<script lang="ts">
import { onMount } from 'svelte';
import { page } from '$app/stores';
import { goto } from '$app/navigation';
import { auth } from '$lib/auth';
import { apiJSON, apiFetch } from '$lib/api';
import '@fortawesome/fontawesome-free/css/all.min.css';
import { compressImage } from '$lib/utils/compressImage';
import ConfirmModal from '$lib/components/ConfirmModal.svelte';
import PromptModal from '$lib/components/PromptModal.svelte';
import { t, translator } from '$lib/i18n'; 
let translate; $: translate = $translator; 
import { TEACHER_GROUP_ID } from '$lib/teacherGroup';

import { formatDateTime } from "$lib/date";
let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(currentParent); }
let role = '';
$: role = $auth?.role ?? '';

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
  // Enhanced UI state
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
let viewMode: 'grid' | 'list' =
  typeof localStorage !== 'undefined' &&
  localStorage.getItem('fileViewMode') === 'list'
    ? 'list'
    : 'grid';
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

function displayName(name: string | null | undefined) {
  if (!name) return '';
  const lastDot = name.lastIndexOf('.');
  if (lastDot <= 0) return name;
  return name.slice(0, lastDot);
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

function openUploadDialog() {
  uploadErr = '';
  uploadFile = null;
  uploading = false;
  uploadDialog.showModal();
}

async function doUpload() {
  if (!uploadFile) return;
  let fileToUpload = uploadFile;
  if (uploadFile.type.startsWith('image/')) {
    try {
      fileToUpload = await compressImage(uploadFile);
    } catch {}
  }
  if (fileToUpload.size > maxFileSize) {
    uploadErr = t('frontend/src/routes/classes/[id]/files/+page.svelte::file_exceeds_limit');
    return;
  }
  const fd = new FormData();
  if (currentParent !== null) fd.append('parent_id', String(currentParent));
  fd.append('file', fileToUpload);
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

async function promptDir(){
  const name = await promptModal?.open({
    title: t('frontend/src/routes/classes/[id]/files/+page.svelte::new_folder_title'),
    label: t('frontend/src/routes/classes/[id]/files/+page.svelte::folder_name_label'),
    placeholder: t('frontend/src/routes/classes/[id]/files/+page.svelte::folder_name_placeholder'),
    confirmLabel: t('frontend/src/routes/classes/[id]/files/+page.svelte::create_button_label'),
    icon: 'fa-solid fa-folder-plus text-primary',
    validate: (value) => value.trim() ? null : t('frontend/src/routes/classes/[id]/files/+page.svelte::folder_name_required_error'),
    transform: (value) => value.trim()
  });
  if(!name) return;
  await createDir(name);
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

async function promptNotebook() {
  const notebookName = await promptModal?.open({
    title: t('frontend/src/routes/classes/[id]/files/+page.svelte::new_notebook_title'),
    label: t('frontend/src/routes/classes/[id]/files/+page.svelte::notebook_name_label'),
    initialValue: t('frontend/src/routes/classes/[id]/files/+page.svelte::notebook_name_initial_value'),
    helpText: t('frontend/src/routes/classes/[id]/files/+page.svelte::notebook_help_text'),
    confirmLabel: t('frontend/src/routes/classes/[id]/files/+page.svelte::create_button_label'),
    icon: 'fa-solid fa-book text-secondary',
    validate: (value) => value.trim() ? null : t('frontend/src/routes/classes/[id]/files/+page.svelte::notebook_name_required_error'),
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
    title: item.is_dir ? t('frontend/src/routes/classes/[id]/files/+page.svelte::delete_folder_title') : t('frontend/src/routes/classes/[id]/files/+page.svelte::delete_file_title'),
    body: item.is_dir ? t('frontend/src/routes/classes/[id]/files/+page.svelte::delete_folder_body') : t('frontend/src/routes/classes/[id]/files/+page.svelte::delete_file_body'),
    confirmLabel: t('frontend/src/routes/classes/[id]/files/+page.svelte::delete_button_label'),
    confirmClass: 'btn btn-error',
    cancelClass: 'btn'
  });
  if(!confirmed) return;
  await apiFetch(`/api/files/${item.id}`,{method:'DELETE'});
  await load(currentParent);
}

async function rename(item:any){
  const name = await promptModal?.open({
    title: t('frontend/src/routes/classes/[id]/files/+page.svelte::rename_title'),
    label: t('frontend/src/routes/classes/[id]/files/+page.svelte::new_name_label'),
    initialValue: item.name,
    confirmLabel: t('frontend/src/routes/classes/[id]/files/+page.svelte::save_button_label'),
    icon: item.is_dir ? 'fa-solid fa-folder text-warning' : 'fa-solid fa-pen text-primary',
    validate: (value) => value.trim() ? null : t('frontend/src/routes/classes/[id]/files/+page.svelte::name_required_error'),
    transform: (value) => value.trim(),
    selectOnOpen: true
  });
  if(!name || name === item.name) return;
  await apiFetch(`/api/files/${item.id}`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({name})});
  await load(currentParent);
}

async function loadTeacherFolders(parent: string | null) {
  copyLoading = true;
  copyErr = '';
  copyFolders = [];
  try {
    const q = parent === null ? '' : `?parent=${parent}`;
    const files = await apiJSON(`/api/classes/${TEACHER_GROUP_ID}/files${q}`);
    copyFolders = files.filter((f: any) => f.is_dir);
    copyParent = parent;
  } catch (e: any) {
    copyErr = e?.message ?? 'Failed to load destination folders';
  }
  copyLoading = false;
}

async function openCopyToTeachers(item: any) {
  if (!item || item.is_dir) return;
  copyItem = item;
  copyName = item.name ?? '';
  copyErr = '';
  copyFolders = [];
  copyBreadcrumbs = [{ id: null, name: '🏠' }];
  copyParent = null;
  await loadTeacherFolders(null);
  copyDialog?.showModal();
}

function copyCrumbTo(index: number) {
  const crumb = copyBreadcrumbs[index];
  copyBreadcrumbs = copyBreadcrumbs.slice(0, index + 1);
  loadTeacherFolders(crumb.id);
}

async function openTeacherFolder(item: any) {
  if (!item?.is_dir) return;
  copyBreadcrumbs = [...copyBreadcrumbs, { id: item.id, name: item.name }];
  await loadTeacherFolders(item.id);
}

function teacherDestinationPath() {
  return copyBreadcrumbs.map((b) => b.name).join(' / ');
}

async function doCopyToTeachers() {
  if (!copyItem) return;
  const trimmedName = copyName.trim();
  if (!trimmedName) {
    copyErr = 'File name is required';
    return;
  }
  copyErr = '';
  copying = true;
  const payload: any = { target_class_id: TEACHER_GROUP_ID };
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

function toggleView() {
  viewMode = viewMode === 'grid' ? 'list' : 'grid';
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem('fileViewMode', viewMode);
  }
}
  
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
      let va: any;
      let vb: any;
      switch (key) {
        case 'size':
          va = a.size ?? 0;
          vb = b.size ?? 0;
          break;
        case 'date':
          va = new Date(a.updated_at ?? a.created_at ?? 0).getTime();
          vb = new Date(b.updated_at ?? b.created_at ?? 0).getTime();
          break;
        case 'name':
        default:
          va = (a.name ?? '').toLowerCase();
          vb = (b.name ?? '').toLowerCase();
      }
      if (va < vb) return dir === 'asc' ? -1 : 1;
      if (va > vb) return dir === 'asc' ? 1 : -1;
      return 0;
    });
    if (key === 'name') {
      sorted.sort((a, b) => (b.is_dir ? 1 : 0) - (a.is_dir ? 1 : 0));
    }
    return sorted;
  }

  // Drag & drop upload support
  let isDragging = false;
  let dragDepth = 0;
  let dropping = false;
  let dropErr = '';

  function onDragEnter() {
    dragDepth += 1;
    isDragging = true;
  }
  function onDragLeave() {
    dragDepth -= 1;
    if (dragDepth <= 0) {
      isDragging = false;
      dragDepth = 0;
    }
  }
  function onDragOver() {
    // allow drop
  }
  async function onDrop(e: DragEvent) {
    dragDepth = 0;
    isDragging = false;
    dropErr = '';
    const files = Array.from(e.dataTransfer?.files ?? []);
    if (!files.length) return;
    try {
      dropping = true;
      await uploadFiles(files);
      await load(currentParent);
    } catch (er:any) {
      dropErr = er?.message ?? t('frontend/src/routes/classes/[id]/files/+page.svelte::failed_to_upload_error');
    } finally {
      dropping = false;
    }
  }

  async function uploadFiles(files: File[]) {
    for (const f of files) {
      let fileToUpload: File = f;
      if (f.type.startsWith('image/')) {
        try { fileToUpload = await compressImage(f); } catch {}
      }
      if (fileToUpload.size > maxFileSize) {
        throw new Error(t('frontend/src/routes/classes/[id]/files/+page.svelte::file_exceeds_limit_named', { name: f.name }));
      }
      const fd = new FormData();
      if (currentParent !== null) fd.append('parent_id', String(currentParent));
      fd.append('file', fileToUpload);
      const res = await apiFetch(`/api/classes/${id}/files`, { method: 'POST', body: fd });
      if (!res.ok) {
        const js = await res.json().catch(() => ({}));
        throw new Error(js.error || res.statusText);
      }
    }
  }
function fmtSize(bytes: number | null | undefined, decimals = 1) {
  if (bytes == null) return '';
  if (bytes < 1024) return `${bytes} B`;

  const units = [t('frontend/src/routes/classes/[id]/files/+page.svelte::unit_kb'), t('frontend/src/routes/classes/[id]/files/+page.svelte::unit_mb'), t('frontend/src/routes/classes/[id]/files/+page.svelte::unit_gb'), t('frontend/src/routes/classes/[id]/files/+page.svelte::unit_tb'), t('frontend/src/routes/classes/[id]/files/+page.svelte::unit_pb'), t('frontend/src/routes/classes/[id]/files/+page.svelte::unit_eb'), t('frontend/src/routes/classes/[id]/files/+page.svelte::unit_zb'), t('frontend/src/routes/classes/[id]/files/+page.svelte::unit_yb')];
  let i = -1;
  do {
    bytes /= 1024;
    i++;
  } while (bytes >= 1024 && i < units.length - 1);

  return `${bytes.toFixed(decimals)} ${units[i]}`;
}

onMount(() => {
  let storedParent: string | null = null;
  if (typeof sessionStorage !== 'undefined') {
    const sp = sessionStorage.getItem(`files_parent_${id}`);
    if (sp) {
      storedParent = sp;
    }
    const bc = sessionStorage.getItem(`files_breadcrumbs_${id}`);
    if (bc) {
      try {
        breadcrumbs = JSON.parse(bc);
      } catch {}
    }
  }
  load(storedParent);
});
</script>

<nav class="mb-4 sticky top-16 z-30 bg-base-200 rounded-box shadow px-4 py-2 flex items-center flex-wrap gap-2">
  <ul class="flex flex-wrap gap-1 text-sm items-center flex-grow">
    {#each breadcrumbs as b,i}
      <li class="after:mx-1 after:content-['/'] last:after:hidden">
        <button type="button" class="link px-2 py-1 rounded hover:bg-base-300" on:click={() => crumbTo(i)} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::open_breadcrumb_aria_label', { name: b.name })}>
          {b.name}
        </button>
      </li>
    {/each}
  </ul>
  <div class="flex items-center gap-2 ml-auto">
    <button class="btn btn-sm btn-circle" on:click={toggleView} title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::toggle_view_tooltip')}>
      {#if viewMode === 'grid'}
        <i class="fa-solid fa-list"></i>
      {:else}
        <i class="fa-solid fa-th"></i>
      {/if}
    </button>
  </div>
  <div class="flex items-center gap-2 ml-auto w-full sm:w-auto justify-end">
    <div class="relative overflow-hidden flex items-center">
      <button class="btn btn-sm btn-circle" on:click={toggleSearch} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::search_button_aria_label')}>
        <i class="fa-solid fa-search"></i>
      </button>
      <input
        class="input input-sm input-bordered ml-2 transition-all duration-300 w-44 sm:w-48"
        style:width={searchOpen ? '12rem' : '0'}
        style:padding-left={searchOpen ? '0.5rem' : '0'}
        style:padding-right={searchOpen ? '0.5rem' : '0'}
        style:opacity={searchOpen ? '1' : '0'}
        placeholder={translate('frontend/src/routes/classes/[id]/files/+page.svelte::search_placeholder')}
        bind:value={search}
      />
    </div>
      <!-- Sort controls -->
      <div class="dropdown dropdown-end">
        <button type="button" class="btn btn-sm" tabindex="0" aria-haspopup="listbox" aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_options_aria_label')}>
          <i class="fa-solid fa-arrow-up-wide-short mr-2"></i>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_button_label')}
        </button>
        <ul class="dropdown-content menu bg-base-200 rounded-box z-[1] w-44 p-2 shadow" tabindex="0" role="listbox">
          <li><button type="button" class={sortKey==='name' ? 'active' : ''} on:click={() => sortKey='name'} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_by_name_aria_label')}>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_by_name_label')}</button></li>
          <li><button type="button" class={sortKey==='date' ? 'active' : ''} on:click={() => sortKey='date'} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_by_modified_aria_label')}>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_by_modified_label')}</button></li>
          <li><button type="button" class={sortKey==='size' ? 'active' : ''} on:click={() => sortKey='size'} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_by_size_aria_label')}>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_by_size_label')}</button></li>
          <li class="mt-1"><button type="button" on:click={() => sortDir = sortDir==='asc' ? 'desc' : 'asc'} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::toggle_sort_direction_aria_label')}>
            {translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_direction_label')}: {sortDir === 'asc' ? translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_direction_asc') : translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_direction_desc')}</button></li>
        </ul>
      </div>
  </div>
  {#if role==='teacher' || role==='admin'}
    <div class="flex items-center gap-2 ml-2 w-full sm:w-auto justify-end">
      <div class="dropdown">
        <button type="button" class="btn btn-sm btn-primary" aria-haspopup="listbox" aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::create_menu_aria_label')}>
          <i class="fa-solid fa-plus mr-2"></i>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::create_menu_button_label')}
        </button>
        <ul class="dropdown-content menu bg-base-200 rounded-box z-[1] w-48 p-2 shadow" role="listbox">
          <li><button type="button" on:click={openUploadDialog}><i class="fa-solid fa-upload mr-2"></i>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::upload_file_menu_item')}</button></li>
          <li><button type="button" on:click={promptDir}><i class="fa-solid fa-folder-plus mr-2"></i>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::new_folder_menu_item')}</button></li>
          <li><button type="button" on:click={promptNotebook}><i class="fa-solid fa-book-medical mr-2"></i>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::new_notebook_menu_item')}</button></li>
        </ul>
      </div>
    </div>
  {/if}
</nav>

<dialog bind:this={uploadDialog} class="modal">
  <div class="modal-box w-11/12 max-w-md space-y-2">
    <h3 class="font-bold text-lg">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::upload_file_dialog_title')}</h3>
    {#if uploadErr}<p class="text-error">{uploadErr}</p>{/if}
    <input type="file" class="file-input file-input-bordered w-full" on:change={e => uploadFile=(e.target as HTMLInputElement).files?.[0] || null}>
    <div class="modal-action">
      <button class="btn" on:click={doUpload} disabled={!uploadFile || uploading}>
        {#if uploading}<span class="loading loading-dots"></span>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::uploading_button_label')}{:else}{translate('frontend/src/routes/classes/[id]/files/+page.svelte::upload_button_label')}{/if}
      </button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::close_button_label')}</button></form>
</dialog>

<div class="relative" role="region" aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::file_list_dropzone_aria_label')}
     on:dragenter|preventDefault={onDragEnter}
     on:dragleave|preventDefault={onDragLeave}
     on:dragover|preventDefault={onDragOver}
     on:drop|preventDefault={onDrop}>

{#if loading}
  <p>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::loading_message')}</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <!-- content below controls actual view rendering -->
{/if}

{#if viewMode === 'grid'}
  <!-- ── GRID VIEW ── -->
  <div class="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 mb-4">
    {#each visible as it (it.id)}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="relative border rounded-box p-3 flex flex-col items-center group hover:shadow-lg hover:-translate-y-0.5 transition cursor-pointer"
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
        <span class="text-sm text-center break-all">{it.is_dir ? it.name : displayName(it.name)}</span>

        <!-- meta -->
        <div class="mt-1 text-xs text-gray-500">
          {#if !it.is_dir}
            <span>{fmtSize(it.size)}</span>
            <span class="mx-1">·</span>
          {/if}
          <span>{formatDateTime(it.updated_at)}</span>
        </div>

        <!-- optional path shown only when searching -->
        {#if searchOpen && search.trim() !== ''}
          <span class="text-xs text-center text-gray-500 break-all">{it.path}</span>
        {/if}

        {#if role === 'teacher' || role === 'admin'}
          <div class="absolute top-1 right-1 hidden group-hover:flex gap-1">
            {#if !it.is_dir}
              <button class="btn btn-xs btn-circle btn-outline" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::copy_to_teachers_group')} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::copy_to_teachers_group')}
                      on:click|stopPropagation={() => openCopyToTeachers(it)}>
                <i class="fa-solid fa-copy"></i>
              </button>
            {/if}
            <button class="btn btn-xs btn-circle" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::rename_tooltip')} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::rename_aria_label')}
                    on:click|stopPropagation={() => rename(it)}>
              <i class="fa-solid fa-pen"></i>
            </button>
            <button class="btn btn-xs btn-circle btn-error" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::delete_tooltip')} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::delete_aria_label')}
                    on:click|stopPropagation={() => del(it)}>
              <i class="fa-solid fa-trash"></i>
            </button>
          </div>
        {/if}
      </div>
    {/each}

    {#if !visible.length}
      <p class="col-span-full"><i>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::no_files_message')}</i></p>
    {/if}
  </div>

{:else}
  <!-- ── LIST VIEW ── -->
  <div class="overflow-x-auto mb-4">
    <table class="table table-zebra w-full">
      <thead>
        <tr>
          <th class="text-left">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::table_header_name')}</th>
          <th class="text-right">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::table_header_size')}</th>
          <th class="text-right">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::table_header_modified')}</th>
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
              <!-- show path in list view too (optional) -->
              {#if searchOpen && search.trim() !== ''}
                <div class="text-xs text-gray-500">{it.path}</div>
              {/if}
            </td>
            <td class="text-right">{it.is_dir ? '' : fmtSize(it.size)}</td>
            <td class="text-right">{formatDateTime(it.updated_at)}</td>

            {#if role === 'teacher' || role === 'admin'}
              <td class="text-right whitespace-nowrap w-16">
                {#if !it.is_dir}
                  <button class="btn btn-xs btn-circle btn-outline invisible group-hover:visible" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::copy_to_teachers_group')} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::copy_to_teachers_group')}
                          on:click|stopPropagation={() => openCopyToTeachers(it)}>
                    <i class="fa-solid fa-copy"></i>
                  </button>
                {/if}
                <button class="btn btn-xs btn-circle invisible group-hover:visible" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::rename_tooltip')} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::rename_aria_label')}
                        on:click|stopPropagation={() => rename(it)}>
                  <i class="fa-solid fa-pen"></i>
                </button>
                <button class="btn btn-xs btn-circle btn-error invisible group-hover:visible" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::delete_tooltip')} aria-label={translate('frontend/src/routes/classes/[id]/files/+page.svelte::delete_aria_label')}
                        on:click|stopPropagation={() => del(it)}>
                  <i class="fa-solid fa-trash"></i>
                </button>
              </td>
            {/if}
          </tr>
        {/each}

        {#if !visible.length}
          <tr>
            <td colspan={role === 'teacher' || role === 'admin' ? 4 : 3}><i>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::no_files_message')}</i></td>
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
      <p class="font-medium">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::drop_files_to_upload_message')}</p>
    </div>
  </div>
{/if}

{#if dropErr}
  <div class="alert alert-error mt-2">
    <i class="fa-solid fa-triangle-exclamation"></i>
    <span>{dropErr}</span>
  </div>
{/if}

{#if dropping}
  <div class="fixed bottom-4 right-4 z-30">
    <div class="btn btn-primary btn-sm no-animation">
      <span class="loading loading-dots loading-xs mr-2"></span>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::uploading_message')}
    </div>
  </div>
{/if}

<dialog bind:this={copyDialog} class="modal" on:close={resetCopyState}>
  <div class="modal-box max-w-2xl space-y-4">
    <h3 class="font-bold text-lg">Copy to Teachers' group</h3>
    {#if copyItem}
      <p class="text-sm text-base-content/70 break-all">Source file: {copyItem.name}</p>
    {/if}
    <div>
      <div class="flex items-center justify-between">
        <span class="text-sm font-medium">Destination</span>
        <button type="button" class="btn btn-ghost btn-xs" on:click={() => loadTeacherFolders(copyParent)} disabled={copyLoading}>
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
      <p class="text-xs text-base-content/70 mt-1">Current folder: {teacherDestinationPath()}</p>
    </div>
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
    <div class="border border-base-300 rounded-box max-h-64 overflow-y-auto">
      {#if copyLoading}
        <div class="p-4 text-sm">Loading folders…</div>
      {:else if !copyFolders.length}
        <div class="p-4 text-sm opacity-70">No subfolders. File will be placed in {teacherDestinationPath()}.</div>
      {:else}
        <ul class="menu menu-sm bg-base-200/40">
          {#each copyFolders as folder}
            <li>
              <button type="button" on:click={() => openTeacherFolder(folder)}>
                <i class="fa-solid fa-folder text-warning mr-2"></i>{folder.name}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
    <div class="modal-action">
      <form method="dialog"><button class="btn">Cancel</button></form>
      <button class="btn btn-primary" on:click|preventDefault={doCopyToTeachers} disabled={copying}>
        {#if copying}<span class="loading loading-dots loading-sm mr-2"></span>{/if}
        Copy here
      </button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

<ConfirmModal bind:this={confirmModal} />
<PromptModal bind:this={promptModal} />

</div>

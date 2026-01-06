<script lang="ts">
import { onMount } from 'svelte';
import { page } from '$app/stores';
import { goto } from '$app/navigation';
import { auth } from '$lib/auth';
import { apiJSON, apiFetch } from '$lib/api';
import { compressImage } from '$lib/utils/compressImage';
import ConfirmModal from '$lib/components/ConfirmModal.svelte';
import PromptModal from '$lib/components/PromptModal.svelte';
import { t, translator } from '$lib/i18n'; 
let translate; $: translate = $translator; 
import { TEACHER_GROUP_ID } from '$lib/teacherGroup';
import { 
  Folder, File, Upload, Plus, Search, LayoutGrid, List, 
  ArrowUpDown, ChevronRight, Copy, Pencil, Trash2, 
  MoreVertical, Download, Image, FileCode, BookOpen, 
  ExternalLink, RefreshCw, AlertCircle, FileType, 
  Filter, ArrowRight, FileQuestion
} from 'lucide-svelte';

import { formatDateTime, formatShortDateTime } from "$lib/date";
let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(currentParent); }
let role = '';
$: role = $auth?.role ?? '';

let cls: any = null;
let items:any[] = [];
let search = '';
let searchResults:any[] = [];
let searchTimeout: any;

$: {
  if (search.trim() !== '') {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      fetchSearch(search.trim());
    }, 300);
  } else {
    searchResults = [];
    clearTimeout(searchTimeout);
  }
}

$: displayed = search.trim() !== '' ? searchResults : items;
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



function isImage(name: string) {
  const ext = name.split('.').pop()?.toLowerCase();
  return ['png','jpg','jpeg','gif','webp','svg'].includes(ext ?? '');
}


function displayName(name: string | null | undefined) {
  if (!name) return '';
  const lastDot = name.lastIndexOf('.');
  if (lastDot <= 0) return name;
  return name.slice(0, lastDot);
}

function getIcon(name: string, isDir: boolean) {
  if (isDir) return Folder;
  const ext = name.split('.').pop()?.toLowerCase();
  if (isImage(name)) return Image;
  if (ext === 'ipynb') return BookOpen;
  if (['js','ts','svelte','py','go','java','cpp'].includes(ext ?? '')) return FileCode;
  if (ext === 'pdf') return FileType;
  return File;
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
    const [filesData, classData] = await Promise.all([
      apiJSON(`/api/classes/${id}/files${q}`),
      apiJSON(`/api/classes/${id}`)
    ]);
    items = filesData;
    const detail = classData ?? null;
    cls = detail?.class ?? detail ?? null;
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
  search = '';
  await load(item.id);
}

function crumbTo(i:number){
  const b = breadcrumbs[i];
  breadcrumbs = breadcrumbs.slice(0,i+1);
  if (typeof sessionStorage !== 'undefined') {
    sessionStorage.setItem(`files_breadcrumbs_${id}`, JSON.stringify(breadcrumbs));
    sessionStorage.setItem(`files_parent_${id}`, b.id === null ? '' : String(b.id));
  }
  search = '';
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
    initialValue: (!item.is_dir && item.name.toLowerCase().endsWith('.ipynb')) ? displayName(item.name) : item.name,
    confirmLabel: t('frontend/src/routes/classes/[id]/files/+page.svelte::save_button_label'),
    icon: item.is_dir ? 'fa-solid fa-folder text-warning' : 'fa-solid fa-pen text-primary',
    validate: (value) => value.trim() ? null : t('frontend/src/routes/classes/[id]/files/+page.svelte::name_required_error'),
    transform: (value) => {
      const trimmed = value.trim();
      if (!item.is_dir && item.name.toLowerCase().endsWith('.ipynb') && !trimmed.toLowerCase().endsWith('.ipynb')) {
        return `${trimmed}.ipynb`;
      }
      return trimmed;
    },
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
    if (draggedItem) return;
    dragDepth += 1;
    isDragging = true;
  }
  function onDragLeave() {
    if (draggedItem) return;
    dragDepth -= 1;
    if (dragDepth <= 0) {
      isDragging = false;
      dragDepth = 0;
    }
  }
  function onDragOver(e: DragEvent) {
    if (draggedItem) return;
    // Prevent default to allow drop, but only for external files
    e.preventDefault();
  }

  // Drag & drop file/folder move handlers
  let draggedItem: any = null;
  let dragOverFolder: any = null;
  let movingFile = false;

  function onItemDragStart(e: DragEvent, item: any) {
    if (!e.dataTransfer) return;
    draggedItem = item;
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', item.id);
    // Add a slight delay to allow the drag image to be created
    setTimeout(() => {
      if (e.target instanceof HTMLElement) {
        e.target.style.opacity = '0.5';
      }
    }, 0);
  }

  function onItemDragEnd(e: DragEvent) {
    if (e.target instanceof HTMLElement) {
      e.target.style.opacity = '1';
    }
    draggedItem = null;
    dragOverFolder = null;
  }

  function onFolderDragOver(e: DragEvent, folder: any) {
    if (!draggedItem || draggedItem.id === folder.id) return;
    e.preventDefault();
    e.stopPropagation();
    dragOverFolder = folder;
  }

  function onFolderDragLeave(e: DragEvent, folder: any) {
    if (dragOverFolder?.id === folder.id || dragOverFolder === folder) {
      dragOverFolder = null;
    }
  }

  async function onFolderDrop(e: DragEvent, targetFolder: any) {
    e.preventDefault();
    e.stopPropagation();
    dragOverFolder = null;
    
    if (!draggedItem || draggedItem.id === targetFolder.id) {
      draggedItem = null;
      return;
    }

    // Don't allow moving a folder into itself
    if (draggedItem.is_dir && targetFolder.id === draggedItem.id) {
      draggedItem = null;
      return;
    }

    try {
      movingFile = true;
      dropErr = '';
      
      const res = await apiFetch(`/api/files/${draggedItem.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ parent_id: targetFolder.id })
      });

      if (!res.ok) {
        const errorData = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(errorData.error || t('frontend/src/routes/files/+page.svelte::failed_to_move_error'));
      }
      
      await load(currentParent);
    } catch (e: any) {
      let msg = e?.message ?? t('frontend/src/routes/files/+page.svelte::failed_to_move_error');
      if (msg.includes('idx_class_files_class_path_unique')) {
        msg = t('frontend/src/routes/files/+page.svelte::file_already_exists_error');
      }
      dropErr = msg;
    } finally {
      movingFile = false;
      draggedItem = null;
    }
  }
  async function onDrop(e: DragEvent) {
    if (draggedItem) return;
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

<svelte:head>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@100..900&display=swap" rel="stylesheet">
  <title>{cls?.name ? `${cls.name} | CodEdu` : 'Files | CodEdu'}</title>
</svelte:head>

{#if loading && !cls}
  <div class="flex justify-center mt-12">
    <span class="loading loading-dots loading-lg text-primary"></span>
  </div>
{:else if err}
  <div class="p-8 text-center bg-base-100 rounded-[2rem] border border-base-200 shadow-sm">
    <div class="w-16 h-16 bg-error/10 text-error rounded-full flex items-center justify-center mx-auto mb-4">
      <AlertCircle size={32} />
    </div>
    <p class="text-error font-black uppercase tracking-widest text-xs mb-2">Error</p>
    <p class="text-base-content/60">{err}</p>
    <button class="btn btn-ghost btn-sm mt-4 gap-2" on:click={() => load(currentParent)}>
      <RefreshCw size={14} /> Retry
    </button>
  </div>
{:else}
  <!-- Premium Header -->
  <section class="relative bg-base-100 rounded-3xl border border-base-200 shadow-xl shadow-base-300/30 mb-8 p-6 sm:p-10">
    <div class="absolute inset-0 overflow-hidden rounded-3xl pointer-events-none">
      <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent"></div>
      <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl"></div>
    </div>
    <div class="relative flex flex-col md:flex-row items-center justify-between gap-6">
      <div class="flex-1 text-center md:text-left">
        <h1 class="text-3xl sm:text-4xl font-black tracking-tight mb-2">
          {cls?.name || ''} <span class="text-primary/40">/</span> {translate('frontend/src/routes/classes/[id]/files/+page.svelte::files_heading')}
        </h1>
        <p class="text-base-content/60 font-medium max-w-xl mx-auto md:mx-0">
          {translate('frontend/src/routes/classes/[id]/files/+page.svelte::files_description')}
        </p>
      </div>
      
      <div class="flex flex-wrap items-center gap-3">
        {#if role === 'teacher' || role === 'admin'}
          <div class="dropdown dropdown-end">
            <button class="btn btn-primary btn-sm rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] h-10 px-4 shadow-lg shadow-primary/20 select-none" type="button" tabindex="0">
              <Plus size={16} />
              {translate('frontend/src/routes/classes/[id]/files/+page.svelte::create_menu_button_label')}
            </button>
            <ul class="dropdown-content menu bg-base-100 rounded-2xl z-[50] w-56 p-2 shadow-2xl border border-base-200 mt-2" tabindex="0">
              <li class="menu-title px-4 py-2 text-[10px] font-black uppercase tracking-widest opacity-40">Actions</li>
              <li><button type="button" on:click={openUploadDialog} class="rounded-xl py-3 select-none"><Upload size={16} class="mr-2 text-primary" />{translate('frontend/src/routes/classes/[id]/files/+page.svelte::upload_file_menu_item')}</button></li>
              <li><button type="button" on:click={promptDir} class="rounded-xl py-3 select-none"><Folder size={16} class="mr-2 text-warning" />{translate('frontend/src/routes/classes/[id]/files/+page.svelte::new_folder_menu_item')}</button></li>
              <li><button type="button" on:click={promptNotebook} class="rounded-xl py-3 select-none"><BookOpen size={16} class="mr-2 text-secondary" />{translate('frontend/src/routes/classes/[id]/files/+page.svelte::new_notebook_menu_item')}</button></li>
            </ul>
          </div>
        {/if}
      </div>
    </div>
  </section>

  <!-- Controls Bar -->
  <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 mb-6 px-2">
    <!-- Breadcrumbs -->
    <nav class="flex items-center gap-1 overflow-x-auto pb-2 lg:pb-0 no-scrollbar max-w-full">
      {#each breadcrumbs as b, i}
        <div class="flex items-center gap-1 shrink-0">
          <button 
            type="button" 
            class={`btn btn-sm btn-ghost rounded-xl px-3 font-bold text-xs h-9 select-none transition-all duration-200 ${i === breadcrumbs.length - 1 ? 'bg-base-200/50' : 'opacity-60 hover:opacity-100'} ${dragOverFolder === 'crumb-' + i ? 'ring-2 ring-primary bg-primary/10 opacity-100' : ''}`}
            on:click={() => crumbTo(i)}
            on:dragover={(e) => { if (draggedItem && i < breadcrumbs.length - 1) { e.preventDefault(); e.stopPropagation(); dragOverFolder = 'crumb-' + i; } }}
            on:dragleave={() => { if (dragOverFolder === 'crumb-' + i) dragOverFolder = null; }}
            on:drop={(e) => { if (draggedItem && i < breadcrumbs.length - 1) onFolderDrop(e, { id: b.id }); }}
          >
            <span class="pointer-events-none">{b.name}</span>
          </button>
          {#if i < breadcrumbs.length - 1}
            <ChevronRight size={14} class="opacity-20" />
          {/if}
        </div>
      {/each}
    </nav>

    <!-- Search and View Toggle -->
    <div class="flex flex-wrap items-center gap-3 justify-end">
      <div class="relative flex items-center">
        <Search size={14} class="absolute left-3 opacity-40" />
        <input 
          type="text" 
          class="input input-sm bg-base-100 border-base-200 focus:border-primary/30 w-full sm:w-48 pl-9 rounded-xl font-medium text-xs h-9" 
          placeholder={translate('frontend/src/routes/classes/[id]/files/+page.svelte::search_placeholder')} 
          bind:value={search} 
        />
      </div>

      <div class="flex items-center bg-base-200/50 p-1 rounded-xl h-9">
        <button 
          title="Grid view"
          class={`btn btn-xs border-none rounded-lg w-8 h-7 px-0 ${viewMode === 'grid' ? 'bg-base-100 shadow-sm text-primary' : 'bg-transparent opacity-60'}`} 
          on:click={() => { viewMode = 'grid'; localStorage.setItem('fileViewMode', 'grid'); }}
        >
          <LayoutGrid size={14} />
        </button>
        <button 
          title="List view"
          class={`btn btn-xs border-none rounded-lg w-8 h-7 px-0 ${viewMode === 'list' ? 'bg-base-100 shadow-sm text-primary' : 'bg-transparent opacity-60'}`} 
          on:click={() => { viewMode = 'list'; localStorage.setItem('fileViewMode', 'list'); }}
        >
          <List size={14} />
        </button>
      </div>

      <div class="dropdown dropdown-end">
          <button type="button" class="btn btn-sm bg-base-100 border-base-200 hover:bg-base-200 rounded-xl h-9 px-4 gap-2 border shadow-sm" tabindex="0">
            <ArrowUpDown size={14} class="opacity-60" />
            <span class="text-[10px] font-black uppercase tracking-widest leading-none">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_button_label')}</span>
          </button>
          <ul class="dropdown-content menu bg-base-100 rounded-2xl z-[50] w-48 p-2 shadow-2xl border border-base-200 mt-2" tabindex="0">
            <li><button type="button" class={sortKey==='name' ? 'active' : ''} on:click={() => sortKey='name'} class:bg-primary={sortKey==='name'} class:text-primary-content={sortKey==='name'}>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_by_name_label')}</button></li>
            <li><button type="button" class={sortKey==='date' ? 'active' : ''} on:click={() => sortKey='date'} class:bg-primary={sortKey==='date'} class:text-primary-content={sortKey==='date'}>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_by_modified_label')}</button></li>
            <li><button type="button" class={sortKey==='size' ? 'active' : ''} on:click={() => sortKey='size'} class:bg-primary={sortKey==='size'} class:text-primary-content={sortKey==='size'}>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_by_size_label')}</button></li>
            <div class="divider my-1 opacity-10"></div>
            <li>
              <button type="button" on:click={() => sortDir = sortDir==='asc' ? 'desc' : 'asc'} class="justify-between">
                <span>{translate('frontend/src/routes/classes/[id]/files/+page.svelte::sort_direction_label')}</span>
                <span class="font-black">{sortDir === 'asc' ? '↑' : '↓'}</span>
              </button>
            </li>
          </ul>
      </div>
    </div>
  </div>

  <!-- File List Dropzone -->
  <div class="relative min-h-[400px] select-none" 
       on:dragenter|preventDefault={onDragEnter}
       on:dragleave|preventDefault={onDragLeave}
       on:dragover|preventDefault={onDragOver}
       on:drop|preventDefault={onDrop}
       on:click={() => { if (role === 'teacher' || role === 'admin') openUploadDialog(); }}>

    {#if loading}
      <div class="absolute inset-0 z-10 bg-base-100/10 backdrop-blur-[1px] flex items-center justify-center pointer-events-none">
        <span class="loading loading-spinner text-primary"></span>
      </div>
    {/if}

    {#if draggedItem && currentParent !== null}
      <!-- Move to parent folder target -->
      <div 
        class="mb-4 p-8 border-2 border-dashed border-primary/30 rounded-[2.5rem] flex flex-col items-center justify-center gap-3 bg-primary/5 transition-all duration-300 animate-in fade-in slide-in-from-top-4 {dragOverFolder === 'parent' ? 'ring-4 ring-primary/20 bg-primary/10 scale-[1.01] border-primary/50' : ''}"
        on:dragover|preventDefault={(e) => { e.preventDefault(); e.stopPropagation(); dragOverFolder = 'parent'; }}
        on:dragleave={() => { if (dragOverFolder === 'parent') dragOverFolder = null; }}
        on:drop={(e) => { if (breadcrumbs.length > 1) onFolderDrop(e, { id: breadcrumbs[breadcrumbs.length - 2].id }); }}
      >
        <div class="w-12 h-12 rounded-2xl bg-primary/10 text-primary flex items-center justify-center shadow-sm">
          <ArrowRight class="-rotate-90" size={24} />
        </div>
        <div class="text-center">
          <p class="font-black text-sm tracking-tight text-primary uppercase tracking-widest">Move to {breadcrumbs[breadcrumbs.length - 2].name}</p>
          <p class="text-[10px] font-bold opacity-40 uppercase tracking-widest mt-1">Drop here to move item up one level</p>
        </div>
      </div>
    {/if}

    {#if viewMode === 'grid'}
      <!-- ── GRID VIEW ── -->
      <div class="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 mb-8">
        {#each visible as it (it.id)}
          <div 
            class="group relative bg-base-200/50 dark:bg-base-200 hover:bg-gradient-to-br hover:from-base-200/50 hover:to-base-100 dark:hover:from-base-200 dark:hover:to-base-300 border border-base-200/60 dark:border-base-300 shadow-sm rounded-[2.5rem] p-5 flex flex-col items-center gap-4 hover:shadow-xl hover:shadow-primary/5 hover:border-primary/20 hover:-translate-y-1 transition-all duration-300 cursor-pointer overflow-hidden backdrop-blur-sm {dragOverFolder?.id === it.id || dragOverFolder === it ? 'ring-4 ring-primary/40 scale-105 bg-primary/5' : ''} {draggedItem?.id === it.id ? 'opacity-50' : ''}"
            draggable={role === 'teacher' || role === 'admin'}
            on:dragstart={(e) => onItemDragStart(e, it)}
            on:dragend={onItemDragEnd}
            on:dragover={(e) => it.is_dir && onFolderDragOver(e, it)}
            on:dragleave={(e) => it.is_dir && onFolderDragLeave(e, it)}
            on:drop={(e) => it.is_dir && onFolderDrop(e, it)}
            on:click={(e) => { e.stopPropagation(); open(it); }}
          >
            <!-- Decorative background blob -->
            <div class={`absolute top-0 right-0 w-24 h-24 rounded-bl-full transition-all duration-500 opacity-0 group-hover:opacity-100 ${it.is_dir ? 'bg-amber-400/10' : isImage(it.name) ? 'bg-purple-500/10' : 'bg-blue-500/10'}`}></div>
            
            <div class="w-20 h-20 flex items-center justify-center relative z-10">
              {#if it.is_dir}
                <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-amber-100 to-orange-100 dark:from-amber-900/30 dark:to-orange-900/30 text-amber-600 dark:text-amber-500 flex items-center justify-center shadow-sm group-hover:scale-110 group-hover:shadow-amber-500/20 transition-all duration-300">
                  <Folder size={32} fill="currentColor" fill-opacity="0.2" />
                </div>
              {:else if isImage(it.name)}
                <div class="w-16 h-16 rounded-2xl overflow-hidden shadow-sm group-hover:scale-110 transition-transform duration-300 ring-2 ring-base-200 group-hover:ring-purple-500/30 relative">
                  <img src={`/api/files/${it.id}`} alt={it.name} class="w-full h-full object-cover" />
                  <div class="absolute inset-0 ring-1 ring-inset ring-black/5 rounded-2xl"></div>
                </div>
              {:else}
                <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 text-blue-600 dark:text-blue-400 flex items-center justify-center shadow-sm group-hover:scale-110 group-hover:shadow-blue-500/20 transition-all duration-300">
                  <svelte:component this={getIcon(it.name, it.is_dir)} size={32} strokeWidth={1.5} />
                </div>
              {/if}
            </div>

            <div class="text-center w-full min-w-0 z-10">
              <h3 class="font-bold text-sm tracking-tight truncate px-2 text-base-content/90 group-hover:text-primary transition-colors" title={it.name}>
                {it.is_dir ? it.name : displayName(it.name)}
              </h3>
              <div class="flex items-center justify-center gap-2 mt-1.5 opacity-50 group-hover:opacity-70 transition-opacity">
                 <span class="text-[10px] font-bold uppercase tracking-wider">
                  {#if !it.is_dir}
                    {fmtSize(it.size)}
                  {:else}
                    Folder
                  {/if}
                 </span>
                 <span class="w-0.5 h-0.5 rounded-full bg-base-content"></span>
                 <span class="text-[10px] font-bold uppercase tracking-wider">{formatShortDateTime(it.updated_at)}</span>
              </div>
            </div>

            {#if role === 'teacher' || role === 'admin'}
              <div class="absolute top-3 right-3 flex flex-col gap-1.5 opacity-0 group-hover:opacity-100 transition-all duration-300 translate-x-2 group-hover:translate-x-0">
                {#if !it.is_dir}
                  <button class="btn btn-xs btn-circle bg-white/80 dark:bg-black/50 backdrop-blur border-none hover:bg-primary hover:text-primary-content shadow-sm transition-colors" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::copy_to_teachers_group')} on:click|stopPropagation={() => openCopyToTeachers(it)}>
                    <Copy size={12} />
                  </button>
                {/if}
                <button class="btn btn-xs btn-circle bg-white/80 dark:bg-black/50 backdrop-blur border-none hover:bg-primary hover:text-primary-content shadow-sm transition-colors" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::rename_tooltip')} on:click|stopPropagation={() => rename(it)}>
                  <Pencil size={12} />
                  </button>
                <button class="btn btn-xs btn-circle bg-white/80 dark:bg-black/50 backdrop-blur border-none hover:bg-error hover:text-error-content shadow-sm transition-colors" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::delete_tooltip')} on:click|stopPropagation={() => del(it)}>
                  <Trash2 size={12} />
                </button>
              </div>
            {/if}
            {#if search.trim() !== ''}
               <div class="absolute bottom-2 text-[9px] font-medium opacity-40 truncate max-w-[90%] bg-base-100/80 px-2 py-0.5 rounded-full">{it.path}</div>
            {/if}
          </div>
        {:else}
          <div 
            class="col-span-full py-20 text-center bg-base-100/50 rounded-[3rem] border-2 border-dashed border-base-200 flex flex-col items-center justify-center select-none {role === 'teacher' || role === 'admin' ? 'cursor-pointer hover:border-primary/30 transition-colors' : ''}"
            on:click={() => { if (role === 'teacher' || role === 'admin') openUploadDialog(); }}
          >
             <div class="w-16 h-16 rounded-full bg-base-200 flex items-center justify-center mb-4 opacity-30 pointer-events-none">
                <Folder size={32} />
             </div>
             <p class="text-sm font-bold opacity-30 uppercase tracking-[0.2em] pointer-events-none">
               {translate('frontend/src/routes/classes/[id]/files/+page.svelte::no_files_message')}
             </p>
             {#if role === 'teacher' || role === 'admin'}
               <p class="text-[10px] font-black uppercase tracking-widest opacity-20 mt-2 pointer-events-none">Click to upload your first file</p>
             {/if}
          </div>
        {/each}
      </div>

    {:else}
      <!-- ── LIST VIEW ── -->
      <div class="bg-base-100 rounded-[2rem] border border-base-200 shadow-sm overflow-hidden mb-8">
        <table class="table w-full">
          <thead>
            <tr class="border-b border-base-200 hover:bg-transparent">
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40 py-5 pl-8">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::table_header_name')}</th>
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40 text-right py-5">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::table_header_size')}</th>
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40 text-right py-5 pr-8">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::table_header_modified')}</th>
              {#if role === 'teacher' || role === 'admin'}<th class="bg-base-100 w-24 pr-8"></th>{/if}
            </tr>
          </thead>
          <tbody class="divide-y divide-base-100">
            {#each visible as it (it.id)}
              <tr 
                class="hover:bg-base-200/50 cursor-pointer group transition-colors select-none {dragOverFolder?.id === it.id ? 'bg-primary/10 ring-2 ring-inset ring-primary/40' : ''} {draggedItem?.id === it.id ? 'opacity-50' : ''}" 
                draggable={role === 'teacher' || role === 'admin'}
                on:dragstart={(e) => onItemDragStart(e, it)}
                on:dragend={onItemDragEnd}
                on:dragover={(e) => it.is_dir && onFolderDragOver(e, it)}
                on:dragleave={(e) => it.is_dir && onFolderDragLeave(e, it)}
                on:drop={(e) => it.is_dir && onFolderDrop(e, it)}
                on:click={(e) => { e.stopPropagation(); open(it); }}
              >
                <td class="py-4 pl-8">
                  <div class="flex items-center gap-4">
                    <div class={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${it.is_dir ? 'bg-warning/10 text-warning' : 'bg-primary/10 text-primary'} group-hover:scale-110 transition-transform`}>
                       <svelte:component this={getIcon(it.name, it.is_dir)} size={18} />
                    </div>
                    <div class="min-w-0">
                       <div class="font-black text-sm tracking-tight truncate group-hover:text-primary transition-colors">{it.name}</div>
                       {#if search.trim() !== ''}
                        <div class="text-[10px] opacity-30 truncate">{it.path}</div>
                       {/if}
                    </div>
                  </div>
                </td>
                <td class="text-right text-xs font-medium opacity-60 tabular-nums py-4">{it.is_dir ? '—' : fmtSize(it.size)}</td>
                <td class="text-right text-xs font-medium opacity-60 py-4 pr-8">{formatDateTime(it.updated_at)}</td>

                {#if role === 'teacher' || role === 'admin'}
                  <td class="text-right py-4 pr-8">
                    <div class="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      {#if !it.is_dir}
                        <button class="btn btn-xs btn-circle btn-ghost" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::copy_to_teachers_group')} on:click|stopPropagation={() => openCopyToTeachers(it)}>
                          <Copy size={12} />
                        </button>
                      {/if}
                      <button class="btn btn-xs btn-circle btn-ghost" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::rename_tooltip')} on:click|stopPropagation={() => rename(it)}>
                        <Pencil size={12} />
                      </button>
                      <button class="btn btn-xs btn-circle btn-ghost text-error hover:bg-error/10" title={translate('frontend/src/routes/classes/[id]/files/+page.svelte::delete_tooltip')} on:click|stopPropagation={() => del(it)}>
                        <Trash2 size={12} />
                      </button>
                    </div>
                  </td>
                {/if}
              </tr>
            {:else}
                <tr>
                  <td colspan={role === 'teacher' || role === 'admin' ? 4 : 3} class="py-20 text-center">
                    <div 
                      class="flex flex-col items-center justify-center opacity-30 select-none {role === 'teacher' || role === 'admin' ? 'cursor-pointer hover:opacity-100 transition-opacity' : ''}"
                      on:click={() => { if (role === 'teacher' || role === 'admin') openUploadDialog(); }}
                    >
                       <Folder size={32} class="mb-2 pointer-events-none" />
                       <p class="text-xs font-bold uppercase tracking-widest pointer-events-none">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::no_files_message')}</p>
                       {#if role === 'teacher' || role === 'admin'}
                          <p class="text-[10px] font-black uppercase tracking-widest mt-2 pointer-events-none">Click to upload your first file</p>
                       {/if}
                    </div>
                  </td>
                </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

    <!-- Drag overlay -->
    {#if isDragging}
      <div class="absolute inset-x-0 -inset-y-4 z-40 border-4 border-dashed border-primary/40 bg-base-100/70 backdrop-blur-sm rounded-[3rem] flex items-center justify-center transition-all animate-in fade-in zoom-in duration-300">
        <div class="text-center">
          <div class="w-20 h-20 bg-primary/10 text-primary rounded-full flex items-center justify-center mx-auto mb-4 animate-bounce">
            <Upload size={32} />
          </div>
          <p class="font-black text-xl tracking-tight text-primary">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::drop_files_to_upload_message')}</p>
          <p class="text-sm opacity-50 mt-1">Release to start uploading</p>
        </div>
      </div>
    {/if}
  </div>

  {#if dropErr}
    <div class="alert alert-error rounded-2xl mb-4 border-none shadow-lg shadow-error/10">
      <AlertCircle size={18} />
      <span class="text-sm font-bold tracking-tight">{dropErr}</span>
      <button class="btn btn-ghost btn-xs btn-circle" on:click={() => dropErr = ''}>×</button>
    </div>
  {/if}

  {#if dropping}
    <div class="fixed bottom-8 right-8 z-[100] animate-in slide-in-from-bottom-10 fade-in duration-500">
      <div class="bg-primary text-primary-content px-6 py-3 rounded-2xl shadow-2xl flex items-center gap-3">
        <span class="loading loading-spinner loading-sm"></span>
        <span class="text-sm font-black uppercase tracking-widest">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::uploading_message')}</span>
      </div>
    </div>
  {/if}
{/if}

<!-- Modals -->
<dialog bind:this={uploadDialog} class="modal">
  <div class="modal-box rounded-[2.5rem] p-8 space-y-6 shadow-2xl border border-base-200">
    <div class="flex items-center gap-4">
      <div class="w-12 h-12 bg-primary/10 text-primary rounded-2xl flex items-center justify-center">
        <Upload size={24} />
      </div>
      <div>
        <h3 class="font-black text-xl tracking-tight">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::upload_file_dialog_title')}</h3>
        <p class="text-xs font-medium opacity-50">Choose a file to upload to the current folder</p>
      </div>
    </div>

    {#if uploadErr}
      <div class="alert alert-error rounded-xl py-3 text-xs bg-error/10 text-error border-none font-bold">
        <AlertCircle size={14} />
        {uploadErr}
      </div>
    {/if}

    <label class="bg-base-200/50 border-2 border-dashed border-base-300 rounded-3xl p-8 text-center group hover:border-primary/30 transition-all relative select-none cursor-pointer block">
      <input 
        type="file" 
        class="hidden" 
        on:change={e => uploadFile=(e.target as HTMLInputElement).files?.[0] || null}
      >
      {#if uploadFile}
         <div class="flex flex-col items-center pointer-events-none">
            <div class="w-12 h-12 bg-success/10 text-success rounded-xl flex items-center justify-center mb-3">
               <File size={20} />
            </div>
            <p class="text-sm font-black tracking-tight">{uploadFile.name}</p>
            <p class="text-[10px] opacity-40 uppercase tracking-widest mt-1">{fmtSize(uploadFile.size)}</p>
         </div>
      {:else}
         <div class="flex flex-col items-center opacity-40 group-hover:opacity-60 transition-opacity pointer-events-none">
            <Upload size={32} class="mb-3" />
            <p class="text-sm font-bold uppercase tracking-widest">Click or drag to select</p>
         </div>
      {/if}
    </label>

    <div class="flex items-center gap-3 pt-2">
      <form method="dialog" class="flex-1"><button class="btn btn-ghost w-full rounded-2xl font-black uppercase tracking-widest text-[10px]">{translate('frontend/src/routes/classes/[id]/files/+page.svelte::close_button_label')}</button></form>
      <button class="btn btn-primary flex-1 rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-primary/20" on:click={doUpload} disabled={!uploadFile || uploading}>
        {#if uploading}<span class="loading loading-spinner loading-xs mr-2"></span>{:else}<Upload size={14} class="mr-2" />{/if}
        {uploading ? translate('frontend/src/routes/classes/[id]/files/+page.svelte::uploading_button_label') : translate('frontend/src/routes/classes/[id]/files/+page.svelte::upload_button_label')}
      </button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

<dialog bind:this={copyDialog} class="modal" on:close={resetCopyState}>
  <div class="modal-box max-w-2xl rounded-[2.5rem] p-8 space-y-6 shadow-2xl border border-base-200">
    <div class="flex items-center gap-4">
      <div class="w-12 h-12 bg-primary/10 text-primary rounded-2xl flex items-center justify-center">
        <Copy size={24} />
      </div>
      <div>
        <h3 class="font-black text-xl tracking-tight">Copy to Teachers' group</h3>
        <p class="text-xs font-medium opacity-50">Select destination and rename if needed</p>
      </div>
    </div>

    {#if copyItem}
      <div class="bg-base-200/50 p-4 rounded-2xl flex items-center gap-3">
         <div class="text-primary opacity-40"><File size={16} /></div>
         <span class="text-xs font-black tracking-tight opacity-60 truncate">Source: {copyItem.name}</span>
      </div>
    {/if}

    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <span class="text-[10px] font-black uppercase tracking-widest opacity-40">Destination Path</span>
        <button type="button" class="btn btn-ghost btn-xs rounded-lg gap-2" on:click={() => loadTeacherFolders(copyParent)} disabled={copyLoading}>
          <RefreshCw size={10} class={copyLoading ? 'animate-spin' : ''} /> Refresh
        </button>
      </div>
      
      <div class="bg-base-200/30 rounded-2xl p-4 border border-base-200">
        <nav class="flex items-center gap-1 overflow-x-auto no-scrollbar">
          {#each copyBreadcrumbs as b, i}
            <div class="flex items-center gap-1 shrink-0">
              <button type="button" class={`btn btn-xs rounded-lg py-0 h-6 px-2 ${i === copyBreadcrumbs.length - 1 ? 'btn-neutral' : 'btn-ghost opacity-60'}`} on:click={() => copyCrumbTo(i)}>
                {b.name}
              </button>
              {#if i < copyBreadcrumbs.length - 1}
                <ChevronRight size={10} class="opacity-20" />
              {/if}
            </div>
          {/each}
        </nav>
      </div>
    </div>

    {#if copyErr}
      <div class="alert alert-error rounded-xl py-3 text-xs bg-error/10 text-error border-none font-bold">
        <AlertCircle size={14} />
        {copyErr}
      </div>
    {/if}

    <div class="space-y-4">
      <div class="form-control">
        <label class="label pt-0 pb-1">
          <span class="label-text text-[10px] font-black uppercase tracking-widest opacity-40">New File Name</span>
        </label>
        <input class="input input-bordered w-full rounded-xl bg-base-100 border-base-200 font-bold text-sm" bind:value={copyName} />
      </div>

      <div class="border border-base-200 rounded-2xl max-h-48 overflow-y-auto bg-base-100 shadow-inner">
        {#if copyLoading}
          <div class="p-8 text-center">
            <span class="loading loading-spinner text-primary"></span>
          </div>
        {:else if !copyFolders.length}
          <div class="p-8 text-center opacity-40 italic text-xs">No subfolders. File will be placed in root.</div>
        {:else}
          <ul class="menu menu-sm p-1">
            {#each copyFolders as folder}
              <li>
                <button type="button" class="rounded-xl py-2 flex items-center justify-between group" on:click={() => openTeacherFolder(folder)}>
                  <div class="flex items-center gap-2">
                    <Folder size={14} class="text-warning" />
                    <span class="font-bold">{folder.name}</span>
                  </div>
                  <ChevronRight size={12} class="opacity-0 group-hover:opacity-40 transition-opacity" />
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>

    <div class="flex items-center gap-3 pt-2">
      <form method="dialog" class="flex-1"><button class="btn btn-ghost w-full rounded-2xl font-black uppercase tracking-widest text-[10px]">Cancel</button></form>
      <button class="btn btn-primary flex-1 rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-primary/20" on:click|preventDefault={doCopyToTeachers} disabled={copying}>
        {#if copying}<span class="loading loading-spinner loading-xs mr-2"></span>{:else}<Copy size={14} class="mr-2" />{/if}
        Copy here
      </button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

<ConfirmModal bind:this={confirmModal} />
<PromptModal bind:this={promptModal} />

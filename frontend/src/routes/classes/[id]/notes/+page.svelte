<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, apiFetch } from '$lib/api';
  import { page } from '$app/stores';
  import { formatDateTime } from "$lib/date";
  import { goto } from '$app/navigation';
  import { auth } from '$lib/auth';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import PromptModal from '$lib/components/PromptModal.svelte';
  import { t, translator } from '$lib/i18n';
  let translate;
  $: translate = $translator;
  import { 
    BookOpen, Upload, Plus, Search, LayoutGrid, List, 
    ArrowUpDown, ChevronRight, Copy, Pencil, Trash2, 
    RefreshCw, AlertCircle, Download, CloudDownload
  } from 'lucide-svelte';

  import { TEACHER_GROUP_ID } from '$lib/teacherGroup';

  let id = $page.params.id;
  $: if ($page.params.id !== id) { id = $page.params.id; load(); }

  let role = '';
  $: role = $auth?.role ?? '';

  let cls: any = null;
  let notes:any[] = [];
  let loading = true;
  let err = '';

  let search = '';
  let sortKey: 'name' | 'date' | 'size' = 'date';
  let sortDir: 'asc' | 'desc' = 'desc';
  let displayedNotes: any[] = [];
  let viewMode: 'grid' | 'list' =
    typeof localStorage !== 'undefined' && localStorage.getItem('notesViewMode') === 'list' ? 'list' : 'grid';
  let confirmModal: InstanceType<typeof ConfirmModal>;
  let promptModal: InstanceType<typeof PromptModal>;
  let uploadInput: HTMLInputElement;
  let copyDialog: HTMLDialogElement;
  let copyItem: any = null;
  let copyErr = '';
  let copyLoading = false;
  let copyFolders: any[] = [];
  let copyBreadcrumbs: { id: string | null; name: string }[] = [{ id: null, name: '🏠' }];
  let copyParent: string | null = null;
  let copyName = '';
  let copying = false;
  let importDialog: HTMLDialogElement;
  let importErr = '';
  let importLoading = false;
  let importFiles: any[] = [];
  let importBreadcrumbs: { id: string | null; name: string }[] = [{ id: null, name: '🏠' }];
  let importParent: string | null = null;
  let selectedTeacherNote: any = null;
  let importName = '';
  let importing = false;
  
  function toggleView() {
    viewMode = viewMode === 'grid' ? 'list' : 'grid';
    if (typeof localStorage !== 'undefined') localStorage.setItem('notesViewMode', viewMode);
  }

  function open(n: any) {
    goto(`/files/${n.id}`);
  }

  function fmtSize(bytes: number | null | undefined, decimals = 1) {
    if (bytes == null) return '';
    if (bytes < 1024) return `${bytes} B`;
    const units = ['KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    let i = -1;
    do { bytes /= 1024; i++; } while (bytes >= 1024 && i < units.length - 1);
    return `${bytes.toFixed(decimals)} ${units[i]}`;
  }

  function displayName(name: string | null | undefined) {
    if (!name) return '';
    return name.replace(/\.ipynb$/i, '');
  }

  // Compute the list of notes to display based on search and sorting
  $: {
    let arr = [...notes];
    if (search.trim() !== '') {
      const q = search.trim().toLowerCase();
      arr = arr.filter(n => n.name?.toLowerCase?.().includes(q));
    }
    arr.sort((a, b) => {
      let va: any;
      let vb: any;
      switch (sortKey) {
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
      if (va < vb) return sortDir === 'asc' ? -1 : 1;
      if (va > vb) return sortDir === 'asc' ? 1 : -1;
      return 0;
    });
    displayedNotes = arr;
  }

  async function load(){
    loading = true; err = '';
    try {
      const [notebooksData, classData] = await Promise.all([
        apiJSON(`/api/classes/${id}/notebooks`),
        apiJSON(`/api/classes/${id}`)
      ]);
      notes = notebooksData;
      const detail = classData ?? null;
      cls = detail?.class ?? detail ?? null;
    } catch(e:any){ err = e.message; }
    loading = false;
  }

  async function createNotebook(name: string) {
    const cf = await apiJSON(`/api/classes/${id}/files`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, parent_id: null })
    });
    const nb = { cells: [], metadata: {}, nbformat: 4, nbformat_minor: 5 };
    await apiFetch(`/api/files/${cf.id}/content`, { method: 'PUT', body: JSON.stringify(nb) });
    await load();
  }

  async function promptNotebook() {
    const notebookName = await promptModal?.open({
      title: t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_new_notebook_title'),
      label: t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_notebook_name_label'),
      initialValue: 'Untitled.ipynb',
      helpText: t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_notebook_help_text'),
      confirmLabel: t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_create_button'),
      icon: 'fa-solid fa-book text-secondary',
      validate: (value) => value.trim() ? null : t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_notebook_name_required'),
      transform: (value) => {
        const trimmed = value.trim();
        if (!trimmed.toLowerCase().endsWith('.ipynb')) return `${trimmed}.ipynb`;
        return trimmed;
      }
    });
    if (!notebookName) return;
    await createNotebook(notebookName);
  }

  function promptUpload() {
    uploadInput?.click();
  }

  async function handleUploadChange(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    try {
      const name = file.name.toLowerCase();
      if (!name.endsWith('.ipynb')) {
        alert(t('frontend/src/routes/classes/[id]/notes/+page.svelte::alert_select_ipynb_file'));
        return;
      }
      const fd = new FormData();
      fd.append('file', file);
      await apiJSON(`/api/classes/${id}/files`, { method: 'POST', body: fd });
      await load();
    } catch (e:any) {
      alert(e?.message || t('frontend/src/routes/classes/[id]/notes/+page.svelte::alert_upload_failed'));
    } finally {
      if (input) input.value = '';
    }
  }

  async function del(n:any){
    const confirmed = await confirmModal.open({
      title: t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_delete_notebook_title'),
      body: t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_delete_notebook_body'),
      confirmLabel: t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_delete_notebook_confirm_button'),
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    });
    if(!confirmed) return;
    await apiFetch(`/api/files/${n.id}`,{method:'DELETE'});
    await load();
  }

  async function rename(n:any){
    const name = await promptModal?.open({
      title: t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_rename_notebook_title'),
      label: t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_new_name_label'),
      initialValue: n.name,
      confirmLabel: t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_save_button'),
      icon: 'fa-solid fa-pen text-primary',
      validate: (value) => value.trim() ? null : t('frontend/src/routes/classes/[id]/notes/+page.svelte::modal_name_required'),
      selectOnOpen: true
    });
    if(!name || name === n.name) return;
    await apiFetch(`/api/files/${n.id}`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({name})});
    await load();
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

  async function openCopyToTeachers(note: any) {
    if (!note) return;
    copyItem = note;
    copyName = note.name ?? '';
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

  async function openTeacherFolder(folder: any) {
    if (!folder?.is_dir) return;
    copyBreadcrumbs = [...copyBreadcrumbs, { id: folder.id, name: folder.name }];
    await loadTeacherFolders(folder.id);
  }

  function teacherDestinationPath() {
    return copyBreadcrumbs.map((b) => b.name).join(' / ');
  }

  async function doCopyToTeachers() {
    if (!copyItem) return;
    const trimmedName = copyName.trim();
    if (!trimmedName) {
      copyErr = 'Notebook name is required';
      return;
    }
    const originalName = copyItem.name ?? '';
    let finalName = trimmedName;
    if (!finalName.toLowerCase().endsWith('.ipynb')) {
      finalName = `${finalName}.ipynb`;
    }
    copyErr = '';
    copying = true;
    const payload: any = { target_class_id: TEACHER_GROUP_ID };
    if (copyParent) payload.target_parent_id = copyParent;
    if (finalName !== originalName) payload.new_name = finalName;
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
      copyErr = e?.message ?? 'Failed to copy notebook';
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

  async function loadTeacherNoteEntries(parent: string | null) {
    importLoading = true;
    importErr = '';
    importFiles = [];
    try {
      const q = parent === null ? '' : `?parent=${parent}`;
      const files = await apiJSON(`/api/classes/${TEACHER_GROUP_ID}/files${q}`);
      importFiles = files.filter((f: any) => f.is_dir || (!f.is_dir && f.name?.toLowerCase?.().endsWith('.ipynb')));
      importParent = parent;
    } catch (e: any) {
      importErr = e?.message ?? 'Failed to load notebooks';
    }
    importLoading = false;
  }

  async function openImportFromTeachers() {
    importErr = '';
    selectedTeacherNote = null;
    importName = '';
    importFiles = [];
    importBreadcrumbs = [{ id: null, name: '🏠' }];
    importParent = null;
    await loadTeacherNoteEntries(null);
    importDialog?.showModal();
  }

  function importCrumbTo(index: number) {
    const crumb = importBreadcrumbs[index];
    importBreadcrumbs = importBreadcrumbs.slice(0, index + 1);
    selectedTeacherNote = null;
    importName = '';
    loadTeacherNoteEntries(crumb.id);
  }

  async function openImportFolder(folder: any) {
    if (!folder?.is_dir) return;
    importBreadcrumbs = [...importBreadcrumbs, { id: folder.id, name: folder.name }];
    selectedTeacherNote = null;
    importName = '';
    await loadTeacherNoteEntries(folder.id);
  }

  function selectTeacherNote(item: any) {
    if (item?.is_dir) return;
    selectedTeacherNote = item;
    importName = item.name ?? '';
  }

  function importDestinationPath() {
    return importBreadcrumbs.map((b) => b.name).join(' / ');
  }

  async function doImportFromTeachers() {
    if (!selectedTeacherNote) {
      importErr = 'Please select a notebook to copy';
      return;
    }
    const trimmedName = importName.trim();
    if (!trimmedName) {
      importErr = 'Notebook name is required';
      return;
    }
    const originalName = selectedTeacherNote.name ?? '';
    let finalName = trimmedName;
    if (!finalName.toLowerCase().endsWith('.ipynb')) {
      finalName = `${finalName}.ipynb`;
    }
    importErr = '';
    importing = true;
    const payload: any = { target_class_id: id };
    if (finalName !== originalName) payload.new_name = finalName;
    try {
      const res = await apiFetch(`/api/files/${selectedTeacherNote.id}/copy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      if (!res.ok) {
        const js = await res.json().catch(() => ({}));
        importErr = js?.error ?? res.statusText;
        importing = false;
        return;
      }
      await res.json().catch(() => null);
      importDialog?.close();
      resetImportState();
      await load();
    } catch (e: any) {
      importErr = e?.message ?? 'Failed to copy notebook';
    }
    importing = false;
  }

  function resetImportState() {
    importErr = '';
    importFiles = [];
    importBreadcrumbs = [{ id: null, name: '🏠' }];
    importParent = null;
    selectedTeacherNote = null;
    importName = '';
    importLoading = false;
    importing = false;
  }

  onMount(load);
</script>

<svelte:head>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@100..900&display=swap" rel="stylesheet">
  <title>{cls?.name ? `${cls.name} | CodEdu` : 'Notes | CodEdu'}</title>
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
    <button class="btn btn-ghost btn-sm mt-4 gap-2" on:click={() => load()}>
      <RefreshCw size={14} /> Retry
    </button>
  </div>
{:else}
  <!-- Premium Header -->
  <section class="relative bg-base-100 rounded-3xl border border-base-200 shadow-xl shadow-base-300/30 mb-8 p-6 sm:p-10">
    <div class="absolute inset-0 overflow-hidden rounded-3xl pointer-events-none">
      <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-secondary/5 to-transparent"></div>
      <div class="absolute -top-24 -right-24 w-64 h-64 bg-secondary/10 rounded-full blur-3xl"></div>
    </div>
    <div class="relative flex flex-col md:flex-row items-center justify-between gap-6">
      <div class="flex-1 text-center md:text-left">
        <h1 class="text-3xl sm:text-4xl font-black tracking-tight mb-2">
          {cls?.name || ''} <span class="text-secondary/40">/</span> {translate('frontend/src/routes/classes/[id]/notes/+page.svelte::heading_notes')}
        </h1>
        <p class="text-base-content/60 font-medium max-w-xl mx-auto md:mx-0">
          Manage and organize your Jupyter notebooks for this class
        </p>
      </div>
      
      <div class="flex flex-wrap items-center gap-3">
        {#if role === 'teacher' || role === 'admin'}
          <div class="dropdown dropdown-end">
            <button class="btn btn-secondary btn-sm rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] h-10 px-4 shadow-lg shadow-secondary/20" type="button">
              <Plus size={16} />
              Create
            </button>
            <ul class="dropdown-content menu bg-base-100 rounded-2xl z-[50] w-56 p-2 shadow-2xl border border-base-200 mt-2">
              <li class="menu-title px-4 py-2 text-[10px] font-black uppercase tracking-widest opacity-40">Actions</li>
              <li><button type="button" on:click={promptNotebook} class="rounded-xl py-3"><BookOpen size={16} class="mr-2 text-secondary" />{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::new_notebook_button')}</button></li>
              <li><button type="button" on:click={promptUpload} class="rounded-xl py-3"><Upload size={16} class="mr-2 text-primary" />{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::upload_from_computer')}</button></li>
              <li><button type="button" on:click={openImportFromTeachers} class="rounded-xl py-3"><CloudDownload size={16} class="mr-2 text-info" />{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::copy_from_teachers_group')}</button></li>
            </ul>
          </div>
          <input bind:this={uploadInput} type="file" accept=".ipynb,application/x-ipynb+json,application/json" class="hidden" on:change={handleUploadChange} />
        {/if}
      </div>
    </div>
  </section>

  <!-- Controls Bar -->
  <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 mb-6 px-2">
    <!-- Search and View Toggle -->
    <div class="flex flex-wrap items-center gap-3 justify-end w-full">
      <div class="relative flex items-center">
        <Search size={14} class="absolute left-3 opacity-40" />
        <input 
          type="text" 
          class="input input-sm bg-base-100 border-base-200 focus:border-secondary/30 w-full sm:w-48 pl-9 rounded-xl font-medium text-xs h-9" 
          placeholder={translate('frontend/src/routes/classes/[id]/notes/+page.svelte::search_notes_placeholder')} 
          bind:value={search} 
        />
      </div>

      <div class="flex items-center bg-base-200/50 p-1 rounded-xl h-9">
        <button 
          title="Grid view"
          class={`btn btn-xs border-none rounded-lg w-8 h-7 px-0 ${viewMode === 'grid' ? 'bg-base-100 shadow-sm text-secondary' : 'bg-transparent opacity-60'}`} 
          on:click={() => { viewMode = 'grid'; localStorage.setItem('notesViewMode', 'grid'); }}
        >
          <LayoutGrid size={14} />
        </button>
        <button 
          title="List view"
          class={`btn btn-xs border-none rounded-lg w-8 h-7 px-0 ${viewMode === 'list' ? 'bg-base-100 shadow-sm text-secondary' : 'bg-transparent opacity-60'}`} 
          on:click={() => { viewMode = 'list'; localStorage.setItem('notesViewMode', 'list'); }}
        >
          <List size={14} />
        </button>
      </div>

      <div class="dropdown dropdown-end">
        <button type="button" class="btn btn-sm bg-base-100 border-base-200 hover:bg-base-200 rounded-xl h-9 px-4 gap-2 border shadow-sm" tabindex="0">
          <ArrowUpDown size={14} class="opacity-60" />
          <span class="text-[10px] font-black uppercase tracking-widest leading-none">{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::sort_button_label')}</span>
        </button>
        <ul class="dropdown-content menu bg-base-100 rounded-2xl z-[50] w-48 p-2 shadow-2xl border border-base-200 mt-2" tabindex="0">
          <li><button type="button" class={sortKey==='name' ? 'active' : ''} on:click={() => sortKey='name'} class:bg-secondary={sortKey==='name'} class:text-secondary-content={sortKey==='name'}>{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::sort_option_name')}</button></li>
          <li><button type="button" class={sortKey==='date' ? 'active' : ''} on:click={() => sortKey='date'} class:bg-secondary={sortKey==='date'} class:text-secondary-content={sortKey==='date'}>{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::sort_option_modified')}</button></li>
          <li><button type="button" class={sortKey==='size' ? 'active' : ''} on:click={() => sortKey='size'} class:bg-secondary={sortKey==='size'} class:text-secondary-content={sortKey==='size'}>{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::sort_option_size')}</button></li>
          <div class="divider my-1 opacity-10"></div>
          <li>
            <button type="button" on:click={() => sortDir = sortDir==='asc' ? 'desc' : 'asc'} class="justify-between">
              <span>{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::sort_direction_label')}</span>
              <span class="font-black">{sortDir === 'asc' ? '↑' : '↓'}</span>
            </button>
          </li>
        </ul>
      </div>
    </div>
  </div>

  <!-- Notes List -->
  <div class="relative min-h-[400px]">
    {#if loading}
      <div class="absolute inset-0 z-10 bg-base-100/10 backdrop-blur-[1px] flex items-center justify-center pointer-events-none">
        <span class="loading loading-spinner text-secondary"></span>
      </div>
    {/if}

    {#if viewMode === 'grid'}
      <!-- ── GRID VIEW ── -->
      <div class="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 mb-8">
        {#each displayedNotes as n (n.id)}
          <div 
            class="group relative bg-base-200/50 dark:bg-base-200 hover:bg-base-100 dark:hover:bg-base-300 border border-base-200 dark:border-base-300 shadow-sm rounded-[2rem] p-4 flex flex-col items-center gap-3 hover:shadow-xl hover:shadow-secondary/5 hover:border-secondary/20 transition-all cursor-pointer overflow-hidden backdrop-blur-sm"
            on:click={() => open(n)}
            on:keydown={(e)=> (e.key==='Enter'||e.key===' ') && open(n)}
            role="button"
            tabindex="0"
            aria-label={`Open ${displayName(n.name)}`}
          >
            <div class="absolute top-0 right-0 w-12 h-12 bg-secondary/5 rounded-bl-full opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none"></div>
            
            <div class="w-16 h-16 flex items-center justify-center relative">
              <div class="text-secondary group-hover:scale-110 transition-transform duration-300">
                <BookOpen size={40} />
              </div>
            </div>

            <div class="text-center w-full min-w-0">
              <h3 class="font-black text-xs tracking-tight truncate px-1 group-hover:text-secondary transition-colors" title={n.name}>
                {displayName(n.name)}
              </h3>
              <div class="text-[9px] font-bold uppercase tracking-widest opacity-40 mt-1 whitespace-nowrap">
                {fmtSize(n.size)} •
                {formatDateTime(n.updated_at).split(' ')[0]}
              </div>
            </div>

            {#if role === 'teacher' || role === 'admin'}
              <div class="absolute top-2 right-2 flex flex-col gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                <button class="btn btn-xs btn-circle bg-base-100 border-base-200 shadow-sm hover:text-secondary" title={translate('frontend/src/routes/classes/[id]/notes/+page.svelte::copy_to_teachers_group')} on:click|stopPropagation={() => openCopyToTeachers(n)}>
                  <Copy size={10} />
                </button>
                <button class="btn btn-xs btn-circle bg-base-100 border-base-200 shadow-sm hover:text-secondary" title={translate('frontend/src/routes/classes/[id]/notes/+page.svelte::rename_tooltip')} on:click|stopPropagation={() => rename(n)}>
                  <Pencil size={10} />
                </button>
                <button class="btn btn-xs btn-circle btn-error btn-outline border-none bg-base-100 shadow-sm" title={translate('frontend/src/routes/classes/[id]/notes/+page.svelte::delete_tooltip')} on:click|stopPropagation={() => del(n)}>
                  <Trash2 size={10} />
                </button>
              </div>
            {/if}
          </div>
        {:else}
          <div class="col-span-full py-20 text-center bg-base-100/50 rounded-[3rem] border-2 border-dashed border-base-200 flex flex-col items-center justify-center">
            <div class="w-16 h-16 rounded-full bg-base-200 flex items-center justify-center mb-4 opacity-30">
              <BookOpen size={32} />
            </div>
            <p class="text-sm font-bold opacity-30 uppercase tracking-[0.2em]">
              {translate('frontend/src/routes/classes/[id]/notes/+page.svelte::no_notes_message')}
            </p>
          </div>
        {/each}
      </div>

    {:else}
      <!-- ── LIST VIEW ── -->
      <div class="bg-base-100 rounded-[2rem] border border-base-200 shadow-sm overflow-hidden mb-8">
        <table class="table w-full">
          <thead>
            <tr class="border-b border-base-200 hover:bg-transparent">
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40 py-5 pl-8">{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::table_header_name')}</th>
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40 text-right py-5">{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::table_header_size')}</th>
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40 text-right py-5 pr-8">{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::table_header_modified')}</th>
              {#if role === 'teacher' || role === 'admin'}<th class="bg-base-100 w-24 pr-8"></th>{/if}
            </tr>
          </thead>
          <tbody class="divide-y divide-base-100">
            {#each displayedNotes as n (n.id)}
              <tr class="hover:bg-base-200/50 cursor-pointer group transition-colors" on:click={() => open(n)}>
                <td class="py-4 pl-8">
                  <div class="flex items-center gap-4">
                    <div class="w-10 h-10 rounded-xl flex items-center justify-center shrink-0 bg-secondary/10 text-secondary group-hover:scale-110 transition-transform">
                      <BookOpen size={18} />
                    </div>
                    <div class="min-w-0">
                      <div class="font-black text-sm tracking-tight truncate group-hover:text-secondary transition-colors">{displayName(n.name)}</div>
                    </div>
                  </div>
                </td>
                <td class="text-right text-xs font-medium opacity-60 tabular-nums py-4">{fmtSize(n.size)}</td>
                <td class="text-right text-xs font-medium opacity-60 py-4 pr-8">{formatDateTime(n.updated_at)}</td>

                {#if role === 'teacher' || role === 'admin'}
                  <td class="text-right py-4 pr-8">
                    <div class="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button class="btn btn-xs btn-circle btn-ghost" title={translate('frontend/src/routes/classes/[id]/notes/+page.svelte::copy_to_teachers_group')} on:click|stopPropagation={() => openCopyToTeachers(n)}>
                        <Copy size={12} />
                      </button>
                      <button class="btn btn-xs btn-circle btn-ghost" title={translate('frontend/src/routes/classes/[id]/notes/+page.svelte::rename_tooltip')} on:click|stopPropagation={() => rename(n)}>
                        <Pencil size={12} />
                      </button>
                      <button class="btn btn-xs btn-circle btn-ghost text-error hover:bg-error/10" title={translate('frontend/src/routes/classes/[id]/notes/+page.svelte::delete_tooltip')} on:click|stopPropagation={() => del(n)}>
                        <Trash2 size={12} />
                      </button>
                    </div>
                  </td>
                {/if}
              </tr>
            {:else}
              <tr>
                <td colspan={role === 'teacher' || role === 'admin' ? 4 : 3} class="py-20 text-center">
                  <div class="flex flex-col items-center justify-center opacity-30">
                    <BookOpen size={32} class="mb-2" />
                    <p class="text-xs font-bold uppercase tracking-widest">{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::no_notes_message')}</p>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
{/if}

<dialog bind:this={copyDialog} class="modal" on:close={resetCopyState}>
  <div class="modal-box max-w-2xl rounded-[2.5rem] p-8 space-y-6 shadow-2xl border border-base-200">
    <div class="flex items-center gap-4">
      <div class="w-12 h-12 bg-secondary/10 text-secondary rounded-2xl flex items-center justify-center">
        <Copy size={24} />
      </div>
      <div>
        <h3 class="font-black text-xl tracking-tight">{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::copy_to_teachers_group')}</h3>
        <p class="text-xs font-medium opacity-50">Select destination and rename if needed</p>
      </div>
    </div>

    {#if copyItem}
      <div class="bg-base-200/50 p-4 rounded-2xl flex items-center gap-3">
        <div class="text-secondary opacity-40"><BookOpen size={16} /></div>
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
          <span class="label-text text-[10px] font-black uppercase tracking-widest opacity-40">Notebook Name</span>
        </label>
        <input class="input input-bordered w-full rounded-xl bg-base-100 border-base-200 font-bold text-sm" bind:value={copyName} />
      </div>

      <div class="border border-base-200 rounded-2xl max-h-48 overflow-y-auto bg-base-100 shadow-inner">
        {#if copyLoading}
          <div class="p-8 text-center">
            <span class="loading loading-spinner text-secondary"></span>
          </div>
        {:else if !copyFolders.length}
          <div class="p-8 text-center opacity-40 italic text-xs">No subfolders. Notebook will be placed in root.</div>
        {:else}
          <ul class="menu menu-sm p-1">
            {#each copyFolders as folder}
              <li>
                <button type="button" class="rounded-xl py-2 flex items-center justify-between group" on:click={() => openTeacherFolder(folder)}>
                  <div class="flex items-center gap-2">
                    <BookOpen size={14} class="text-secondary" />
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
      <button class="btn btn-secondary flex-1 rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-secondary/20" on:click|preventDefault={doCopyToTeachers} disabled={copying}>
        {#if copying}<span class="loading loading-spinner loading-xs mr-2"></span>{:else}<Copy size={14} class="mr-2" />{/if}
        Copy here
      </button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

<dialog bind:this={importDialog} class="modal" on:close={resetImportState}>
  <div class="modal-box max-w-3xl rounded-[2.5rem] p-8 space-y-6 shadow-2xl border border-base-200">
    <div class="flex items-center gap-4">
      <div class="w-12 h-12 bg-info/10 text-info rounded-2xl flex items-center justify-center">
        <CloudDownload size={24} />
      </div>
      <div>
        <h3 class="font-black text-xl tracking-tight">{translate('frontend/src/routes/classes/[id]/notes/+page.svelte::copy_from_teachers_group')}</h3>
        <p class="text-xs font-medium opacity-50">Browse and select a notebook to copy</p>
      </div>
    </div>

    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <span class="text-[10px] font-black uppercase tracking-widest opacity-40">Browse Teachers' Notebooks</span>
        <button type="button" class="btn btn-ghost btn-xs rounded-lg gap-2" on:click={() => loadTeacherNoteEntries(importParent)} disabled={importLoading}>
          <RefreshCw size={10} class={importLoading ? 'animate-spin' : ''} /> Refresh
        </button>
      </div>
      
      <div class="bg-base-200/30 rounded-2xl p-4 border border-base-200">
        <nav class="flex items-center gap-1 overflow-x-auto no-scrollbar">
          {#each importBreadcrumbs as b, i}
            <div class="flex items-center gap-1 shrink-0">
              <button type="button" class={`btn btn-xs rounded-lg py-0 h-6 px-2 ${i === importBreadcrumbs.length - 1 ? 'btn-neutral' : 'btn-ghost opacity-60'}`} on:click={() => importCrumbTo(i)}>
                {b.name}
              </button>
              {#if i < importBreadcrumbs.length - 1}
                <ChevronRight size={10} class="opacity-20" />
              {/if}
            </div>
          {/each}
        </nav>
      </div>
    </div>

    {#if importErr}
      <div class="alert alert-error rounded-xl py-3 text-xs bg-error/10 text-error border-none font-bold">
        <AlertCircle size={14} />
        {importErr}
      </div>
    {/if}

    <div class="border border-base-200 rounded-2xl max-h-72 overflow-y-auto bg-base-100 shadow-inner">
      {#if importLoading}
        <div class="p-8 text-center">
          <span class="loading loading-spinner text-info"></span>
        </div>
      {:else if !importFiles.length}
        <div class="p-8 text-center opacity-40 italic text-xs">No notebooks here.</div>
      {:else}
        <table class="table table-sm w-full">
          <thead>
            <tr class="border-b border-base-200">
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40">Name</th>
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40">Type</th>
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40 text-right">Modified</th>
            </tr>
          </thead>
          <tbody>
            {#each importFiles as item}
              <tr
                class={`hover:bg-base-200/50 cursor-pointer transition-colors ${selectedTeacherNote?.id === item.id ? 'bg-secondary/10' : ''}`}
                on:click={() => item.is_dir ? openImportFolder(item) : selectTeacherNote(item)}
              >
                <td>
                  <div class="flex items-center gap-2">
                    {#if item.is_dir}
                      <BookOpen size={14} class="text-warning" />
                    {:else}
                      <BookOpen size={14} class="text-secondary" />
                    {/if}
                    <span class="font-bold text-xs">{item.name}</span>
                  </div>
                </td>
                <td class="text-xs opacity-60">{item.is_dir ? 'Folder' : 'Notebook'}</td>
                <td class="text-right text-xs opacity-60">{formatDateTime(item.updated_at)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>

    <div class="form-control">
      <label class="label pt-0 pb-1">
        <span class="label-text text-[10px] font-black uppercase tracking-widest opacity-40">New Notebook Name</span>
      </label>
      <input class="input input-bordered w-full rounded-xl bg-base-100 border-base-200 font-bold text-sm" bind:value={importName} placeholder="e.g. Lesson Plan.ipynb" />
    </div>

    <div class="flex items-center gap-3 pt-2">
      <form method="dialog" class="flex-1"><button class="btn btn-ghost w-full rounded-2xl font-black uppercase tracking-widest text-[10px]">Cancel</button></form>
      <button class="btn btn-info flex-1 rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-info/20" on:click|preventDefault={doImportFromTeachers} disabled={importing || !selectedTeacherNote}>
        {#if importing}<span class="loading loading-spinner loading-xs mr-2"></span>{:else}<CloudDownload size={14} class="mr-2" />{/if}
        Copy to class
      </button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

<ConfirmModal bind:this={confirmModal} />
<PromptModal bind:this={promptModal} />
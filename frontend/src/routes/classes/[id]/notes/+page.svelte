<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, apiFetch } from '$lib/api';
  import { page } from '$app/stores';
  import { formatDateTime } from "$lib/date";
  import { goto } from '$app/navigation';
  import { auth } from '$lib/auth';
  import '@fortawesome/fontawesome-free/css/all.min.css';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import PromptModal from '$lib/components/PromptModal.svelte';
  import { TEACHER_GROUP_ID } from '$lib/teacherGroup';

  let id = $page.params.id;
  $: if ($page.params.id !== id) { id = $page.params.id; load(); }

  let role = '';
  $: role = $auth?.role ?? '';

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
      notes = await apiJSON(`/api/classes/${id}/notebooks`);
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
        alert('Please select a .ipynb notebook file.');
        return;
      }
      const fd = new FormData();
      fd.append('file', file);
      await apiJSON(`/api/classes/${id}/files`, { method: 'POST', body: fd });
      await load();
    } catch (e:any) {
      alert(e?.message || 'Upload failed');
    } finally {
      if (input) input.value = '';
    }
  }

  async function del(n:any){
    const confirmed = await confirmModal.open({
      title: 'Delete notebook',
      body: 'This notebook will be permanently removed for the class.',
      confirmLabel: 'Delete notebook',
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    });
    if(!confirmed) return;
    await apiFetch(`/api/files/${n.id}`,{method:'DELETE'});
    await load();
  }

  async function rename(n:any){
    const name = await promptModal?.open({
      title: 'Rename notebook',
      label: 'New name',
      initialValue: n.name,
      confirmLabel: 'Save',
      icon: 'fa-solid fa-pen text-primary',
      validate: (value) => value.trim() ? null : 'Name is required',
      transform: (value) => value.trim(),
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

<nav class="mb-4 sticky top-16 z-30 bg-base-200 rounded-box shadow px-4 py-2 flex items-center gap-2 flex-wrap">
  <h1 class="text-xl font-semibold">Notes</h1>
  <div class="ml-auto flex items-center gap-2 w-full sm:w-auto justify-end">
    <div class="relative flex items-center">
      <i class="fa-solid fa-search absolute left-3 text-sm opacity-60"></i>
       <input class="input input-sm input-bordered pl-8 w-full sm:w-48" placeholder="Search notes" bind:value={search} aria-label="Search notes" />
    </div>
      <div class="dropdown dropdown-end">
        <button type="button" class="btn btn-sm" tabindex="0" aria-haspopup="listbox" aria-label="Sort options">
          <i class="fa-solid fa-arrow-up-wide-short mr-2"></i>Sort
        </button>
        <ul class="dropdown-content menu bg-base-200 rounded-box z-[1] w-44 p-2 shadow" tabindex="0" role="listbox">
          <li><button type="button" class={sortKey==='name' ? 'active' : ''} on:click={() => sortKey='name'}>Name</button></li>
          <li><button type="button" class={sortKey==='date' ? 'active' : ''} on:click={() => sortKey='date'}>Modified</button></li>
          <li><button type="button" class={sortKey==='size' ? 'active' : ''} on:click={() => sortKey='size'}>Size</button></li>
          <li class="mt-1"><button type="button" on:click={() => sortDir = sortDir==='asc' ? 'desc' : 'asc'}>Direction: {sortDir === 'asc' ? 'Asc' : 'Desc'}</button></li>
        </ul>
      </div>
    <button class="btn btn-sm btn-circle" on:click={toggleView} title="Toggle view" aria-label="Toggle view">
      {#if viewMode === 'grid'}
        <i class="fa-solid fa-list"></i>
      {:else}
        <i class="fa-solid fa-th"></i>
      {/if}
    </button>
    {#if role==='teacher' || role==='admin'}
      <button class="btn btn-sm btn-outline" on:click={openImportFromTeachers} title="Copy from Teachers' group"><i class="fa-solid fa-copy mr-2"></i>Copy from Teachers' group</button>
      <button class="btn btn-sm" on:click={promptUpload} title="Upload notebook"><i class="fa-solid fa-file-arrow-up mr-2"></i>Upload</button>
      <button class="btn btn-sm btn-primary" on:click={promptNotebook}><i class="fa-solid fa-book-medical mr-2"></i>New notebook</button>
      <input bind:this={uploadInput} type="file" accept=".ipynb,application/x-ipynb+json,application/json" class="hidden" on:change={handleUploadChange} />
    {/if}
  </div>
</nav>

{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  {#if viewMode === 'grid'}
    <div class="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
      {#each displayedNotes as n (n.id)}
        <div role="button" tabindex="0" class="relative border rounded-box p-3 flex flex-col items-center group hover:shadow-lg hover:-translate-y-0.5 transition cursor-pointer" on:click={() => open(n)} on:keydown={(e)=> (e.key==='Enter'||e.key===' ') && open(n)} aria-label={`Open ${displayName(n.name)}`}>
          <div class="text-4xl mb-2 text-secondary"><i class="fa-solid fa-book"></i></div>
          <div class="text-sm font-medium break-all text-center">{displayName(n.name)}</div>
          <div class="mt-1 text-xs text-gray-500 text-center">
            <span>{fmtSize(n.size)}</span>
            <span class="mx-1">·</span>
            <span>{formatDateTime(n.updated_at)}</span>
          </div>
          {#if role==='teacher' || role==='admin'}
            <div class="absolute top-1 right-1 hidden group-hover:flex gap-1">
              <button class="btn btn-xs btn-circle btn-outline" title="Copy to Teachers' group" aria-label="Copy to Teachers' group" on:click|stopPropagation={() => openCopyToTeachers(n)}>
                <i class="fa-solid fa-copy"></i>
              </button>
              <button class="btn btn-xs btn-circle" title="Rename" aria-label="Rename" on:click|stopPropagation={() => rename(n)}>
                <i class="fa-solid fa-pen"></i>
              </button>
              <button class="btn btn-xs btn-circle btn-error" title="Delete" aria-label="Delete" on:click|stopPropagation={() => del(n)}>
                <i class="fa-solid fa-trash"></i>
              </button>
            </div>
          {/if}
        </div>
      {/each}
      {#if !displayedNotes.length}
        <p class="col-span-full"><i>No notes</i></p>
      {/if}
    </div>
  {:else}
    <div class="overflow-x-auto">
      <table class="table table-zebra w-full">
        <thead>
          <tr>
            <th class="text-left">Name</th>
            <th class="text-right">Size</th>
            <th class="text-right">Modified</th>
            {#if role==='teacher' || role==='admin'}<th class="w-16"></th>{/if}
          </tr>
        </thead>
        <tbody>
          {#each displayedNotes as n (n.id)}
            <tr class="hover:bg-base-200 cursor-pointer group" on:click={() => open(n)}>
              <td class="whitespace-nowrap">
                <i class="fa-solid fa-book text-secondary mr-2"></i>{displayName(n.name)}
              </td>
              <td class="text-right">{fmtSize(n.size)}</td>
              <td class="text-right">{formatDateTime(n.updated_at)}</td>
              {#if role==='teacher' || role==='admin'}
                <td class="text-right whitespace-nowrap w-16">
                  <button class="btn btn-xs btn-circle btn-outline invisible group-hover:visible" title="Copy to Teachers' group" aria-label="Copy to Teachers' group" on:click|stopPropagation={() => openCopyToTeachers(n)}>
                    <i class="fa-solid fa-copy"></i>
                  </button>
                  <button class="btn btn-xs btn-circle invisible group-hover:visible" title="Rename" aria-label="Rename" on:click|stopPropagation={() => rename(n)}>
                    <i class="fa-solid fa-pen"></i>
                  </button>
                  <button class="btn btn-xs btn-circle btn-error invisible group-hover:visible" title="Delete" aria-label="Delete" on:click|stopPropagation={() => del(n)}>
                    <i class="fa-solid fa-trash"></i>
                  </button>
                </td>
              {/if}
            </tr>
          {/each}
      {#if !displayedNotes.length}
            <tr><td colspan={role==='teacher' || role==='admin' ? 4 : 3}><i>No notes</i></td></tr>
          {/if}
        </tbody>
      </table>
    </div>
{/if}
{/if}

<dialog bind:this={copyDialog} class="modal" on:close={resetCopyState}>
  <div class="modal-box max-w-2xl space-y-4">
    <h3 class="font-bold text-lg">Copy notebook to Teachers' group</h3>
    {#if copyItem}
      <p class="text-sm text-base-content/70 break-all">Source notebook: {copyItem.name}</p>
    {/if}
    <div>
      <div class="flex items-center justify-between">
        <span class="text-sm font-medium">Destination folder</span>
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
        <span class="label-text">Notebook name</span>
      </div>
      <input class="input input-bordered w-full" bind:value={copyName} />
    </label>
    <div class="border border-base-300 rounded-box max-h-64 overflow-y-auto">
      {#if copyLoading}
        <div class="p-4 text-sm">Loading folders…</div>
      {:else if !copyFolders.length}
        <div class="p-4 text-sm opacity-70">No subfolders. Notebook will be placed in {teacherDestinationPath()}.</div>
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

<dialog bind:this={importDialog} class="modal" on:close={resetImportState}>
  <div class="modal-box max-w-3xl space-y-4">
    <h3 class="font-bold text-lg">Copy notebook from Teachers' group</h3>
    <div>
      <div class="flex items-center justify-between">
        <span class="text-sm font-medium">Browse Teachers' notebooks</span>
        <button type="button" class="btn btn-ghost btn-xs" on:click={() => loadTeacherNoteEntries(importParent)} disabled={importLoading}>
          <i class="fa-solid fa-rotate-right mr-1"></i>Refresh
        </button>
      </div>
      <nav class="text-xs mt-1">
        <ul class="flex flex-wrap gap-1 items-center">
          {#each importBreadcrumbs as b, i}
            <li class="after:mx-1 after:content-['/'] last:after:hidden">
              <button type="button" class="link px-2 py-1 rounded hover:bg-base-300" on:click={() => importCrumbTo(i)}>
                {b.name}
              </button>
            </li>
          {/each}
        </ul>
      </nav>
      <p class="text-xs text-base-content/70 mt-1">Current folder: {importDestinationPath()}</p>
    </div>
    {#if importErr}
      <div class="alert alert-error text-sm">
        <i class="fa-solid fa-triangle-exclamation"></i>
        <span>{importErr}</span>
      </div>
    {/if}
    <div class="border border-base-300 rounded-box max-h-72 overflow-y-auto">
      {#if importLoading}
        <div class="p-4 text-sm">Loading…</div>
      {:else if !importFiles.length}
        <div class="p-4 text-sm opacity-70">No notebooks here.</div>
      {:else}
        <table class="table table-zebra table-sm w-full">
          <thead>
            <tr>
              <th class="text-left">Name</th>
              <th class="text-left">Type</th>
              <th class="text-right">Modified</th>
            </tr>
          </thead>
          <tbody>
            {#each importFiles as item}
              <tr
                class="hover:bg-base-200 cursor-pointer"
                class:bg-base-300={selectedTeacherNote?.id === item.id}
                on:click={() => item.is_dir ? openImportFolder(item) : selectTeacherNote(item)}
              >
                <td>
                  {#if item.is_dir}
                    <i class="fa-solid fa-folder text-warning mr-2"></i>{item.name}
                  {:else}
                    <i class="fa-solid fa-book text-secondary mr-2"></i>{item.name}
                  {/if}
                </td>
                <td>{item.is_dir ? 'Folder' : 'Notebook'}</td>
                <td class="text-right">{formatDateTime(item.updated_at)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
    <label class="form-control w-full">
      <div class="label">
        <span class="label-text">New notebook name</span>
      </div>
      <input class="input input-bordered w-full" bind:value={importName} placeholder="e.g. Lesson Plan.ipynb" />
    </label>
    <div class="modal-action">
      <form method="dialog"><button class="btn">Cancel</button></form>
      <button class="btn btn-primary" on:click|preventDefault={doImportFromTeachers} disabled={importing || !selectedTeacherNote}>
        {#if importing}<span class="loading loading-dots loading-sm mr-2"></span>{/if}
        Copy to class
      </button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

<ConfirmModal bind:this={confirmModal} />
<PromptModal bind:this={promptModal} />

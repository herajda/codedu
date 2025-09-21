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
      <button class="btn btn-sm btn-primary" on:click={promptNotebook}><i class="fa-solid fa-book-medical mr-2"></i>New notebook</button>
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
        <div role="button" tabindex="0" class="relative border rounded-box p-3 flex flex-col items-center group hover:shadow-lg hover:-translate-y-0.5 transition cursor-pointer" on:click={() => open(n)} on:keydown={(e)=> (e.key==='Enter'||e.key===' ') && open(n)} aria-label={`Open ${n.name}`}> 
          <div class="text-4xl mb-2 text-secondary"><i class="fa-solid fa-book"></i></div>
          <div class="text-sm font-medium break-all text-center">{n.name}</div>
          <div class="mt-1 text-xs text-gray-500 text-center">
            <span>{fmtSize(n.size)}</span>
            <span class="mx-1">·</span>
            <span>{formatDateTime(n.updated_at)}</span>
          </div>
          {#if role==='teacher' || role==='admin'}
            <div class="absolute top-1 right-1 hidden group-hover:flex gap-1">
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
                <i class="fa-solid fa-book text-secondary mr-2"></i>{n.name}
              </td>
              <td class="text-right">{fmtSize(n.size)}</td>
              <td class="text-right">{formatDateTime(n.updated_at)}</td>
              {#if role==='teacher' || role==='admin'}
                <td class="text-right whitespace-nowrap w-16">
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

<ConfirmModal bind:this={confirmModal} />
<PromptModal bind:this={promptModal} />

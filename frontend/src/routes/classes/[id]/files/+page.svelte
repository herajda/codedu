<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { get } from 'svelte/store';
  import { auth } from '$lib/auth';
  import { apiJSON, apiFetch } from '$lib/api';
  import '@fortawesome/fontawesome-free/css/all.min.css';

  // ──────────────────────────────
  //  State
  // ──────────────────────────────
  let id = $page.params.id;
  $: if ($page.params.id !== id) { id = $page.params.id; load(currentParent); }
  const role: string = get(auth)?.role ?? '';

  let items: any[] = [];
  let search = '';
  let searchOpen = false;
  let searchResults: any[] = [];

  $: if (searchOpen && search.trim() !== '') {
    fetchSearch(search.trim());
  } else {
    searchResults = [];
  }
  // use the same array for both view modes
  $: displayed = searchOpen && search.trim() !== '' ? searchResults : items;

  let breadcrumbs: { id: number|null; name: string }[] = [{ id: null, name: '🏠' }];
  let currentParent: number|null = null;
  let loading = false;
  let err = '';
  let uploadInput: HTMLInputElement;
  let viewMode: 'grid' | 'list' =
    typeof localStorage !== 'undefined' &&
    localStorage.getItem('fileViewMode') === 'list'
      ? 'list'
      : 'grid';

  // ──────────────────────────────
  //  Helpers (unchanged)
  // ──────────────────────────────
  /* … everything from isImage() down to onMount() is unchanged … */
</script>

<!-- ──────────────────────────────
     ⬆ script  |  ⬇ markup
     ────────────────────────────── -->
<nav class="mb-4 sticky top-16 z-30 bg-base-200 rounded-box shadow px-4 py-2 flex items-center flex-wrap gap-2">
  <!-- breadcrumbs -->
  <ul class="flex flex-wrap gap-1 text-sm items-center flex-grow">
    {#each breadcrumbs as b,i}
      <li class="after:mx-1 after:content-['/'] last:after:hidden">
        <a href="#" class="link px-2 py-1 rounded hover:bg-base-300" on:click|preventDefault={() => crumbTo(i)}>
          {b.name}
        </a>
      </li>
    {/each}
  </ul>

  <!-- right-hand controls -->
  <div class="flex items-center gap-2 ml-auto">
    <!-- search -->
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

    <!-- view-mode toggle -->
    <button class="btn btn-sm btn-circle" on:click={toggleView} title="Toggle view">
      {#if viewMode === 'grid'}
        <i class="fa-solid fa-list"></i>
      {:else}
        <i class="fa-solid fa-th"></i>
      {/if}
    </button>

    <!-- teacher/admin tools -->
    {#if role === 'teacher' || role === 'admin'}
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
  {#if viewMode === 'grid'}
    <!-- ───────────── GRID VIEW ───────────── -->
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
          {#if role === 'teacher' || role === 'admin'}
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

      {#if !displayed.length}
        <p class="col-span-full"><i>No files</i></p>
      {/if}
    </div>
  {:else}
    <!-- ───────────── LIST VIEW ───────────── -->
    <div class="overflow-x-auto mb-4">
      <table class="table table-zebra w-full">
        <thead>
          <tr>
            <th class="text-left">Name</th>
            <th class="text-right">Size</th>
            <th class="text-right">Modified</th>
            {#if role === 'teacher' || role === 'admin'}
              <th class="w-16"></th>
            {/if}
          </tr>
        </thead>
        <tbody>
          {#each displayed as it}
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
                  <span class="text-xs text-gray-500 ml-2">{it.path}</span>
                {/if}
              </td>
              <td class="text-right">{it.is_dir ? '' : fmtSize(it.size)}</td>
              <td class="text-right">{new Date(it.updated_at).toLocaleString()}</td>
              {#if role === 'teacher' || role === 'admin'}
                <td class="text-right whitespace-nowrap w-16">
                  <button class="btn btn-xs btn-circle invisible group-hover:visible" title="Rename" on:click|stopPropagation={() => rename(it)}>
                    <i class="fa-solid fa-pen"></i>
                  </button>
                  <button class="btn btn-xs btn-circle btn-error invisible group-hover:visible" title="Delete" on:click|stopPropagation={() => del(it)}>
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
{/if}
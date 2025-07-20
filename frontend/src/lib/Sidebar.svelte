<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import '@fortawesome/fontawesome-free/css/all.min.css';
  import { sidebarOpen, sidebarCollapsed } from '$lib/sidebar';
  import { auth } from '$lib/auth';
  let classes:any[] = [];
  let err = '';
  onMount(async () => {
    try {
      const result = await apiJSON('/api/classes');
      classes = Array.isArray(result) ? result : [];
    } catch(e:any){ err = e.message }
  });
</script>
<div class={`fixed top-0 left-0 z-40 h-screen pointer-events-none group
    ${$sidebarOpen ? 'block' : 'hidden sm:block'}`}
>
  <aside
    class={`relative w-60 bg-base-200 p-4 h-full overflow-y-auto transition-transform pointer-events-auto
        ${$sidebarCollapsed ? '-translate-x-full' : 'translate-x-0'}`}
  >
  <button
    class="btn btn-square btn-ghost absolute right-2 top-2 sm:hidden"
    on:click={() => sidebarOpen.set(false)}
    aria-label="Close sidebar"
  >
    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" class="w-5 h-5">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
    </svg>
  </button>
  <h2 class="font-bold mb-2">Classes</h2>
  <ul class="menu">
    {#each classes as c}
      <li>
        <details open={$page.url.pathname.startsWith(`/classes/${c.id}`)}>
          <summary class="flex items-center gap-2">
            <i class="fa-solid fa-book"></i>
            {c.name}
          </summary>
          <ul>
            {#if $auth?.role === 'student'}
              <li><a class={$page.url.pathname===`/classes/${c.id}/overview` ? 'active' : ''} href={`/classes/${c.id}/overview`} on:click={() => sidebarOpen.set(false)}>Overview</a></li>
            {/if}
            <li><a class={$page.url.pathname===`/classes/${c.id}` ? 'active' : ''} href={`/classes/${c.id}`} on:click={() => sidebarOpen.set(false)}>Assignments</a></li>
            <li><a class={$page.url.pathname===`/classes/${c.id}/files` ? 'active' : ''} href={`/classes/${c.id}/files`} on:click={() => sidebarOpen.set(false)}>Files</a></li>
            <li><a class={$page.url.pathname===`/classes/${c.id}/notes` ? 'active' : ''} href={`/classes/${c.id}/notes`} on:click={() => sidebarOpen.set(false)}>Notes</a></li>
            {#if $auth?.role !== 'student'}
              <li><a class={$page.url.pathname===`/classes/${c.id}/progress` ? 'active' : ''} href={`/classes/${c.id}/progress`} on:click={() => sidebarOpen.set(false)}>Progress</a></li>
              <li><a class={$page.url.pathname===`/classes/${c.id}/settings` ? 'active' : ''} href={`/classes/${c.id}/settings`} on:click={() => sidebarOpen.set(false)}>Settings</a></li>
            {/if}
          </ul>
        </details>
      </li>
    {/each}
    {#if !classes.length && !err}
      <li><i>No classes</i></li>
    {/if}
  </ul>
  {#if err}<p class="text-error mt-2">{err}</p>{/if}
  </aside>
  <button
    class="btn btn-square btn-xs hidden sm:flex absolute top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-auto"
    style:left={$sidebarCollapsed ? '0' : '15rem'}
    style:transform="translate(-50%, -50%)"
    on:click={() => sidebarCollapsed.update(v => !v)}
    aria-label="Toggle sidebar"
  >
    {#if $sidebarCollapsed}
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-4 h-4">
        <path d="M9.75 5.25L16.5 12l-6.75 6.75" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    {:else}
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-4 h-4">
        <path d="M14.25 5.25L7.5 12l6.75 6.75" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    {/if}
  </button>
</div>

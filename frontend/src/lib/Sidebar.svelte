<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import '@fortawesome/fontawesome-free/css/all.min.css';
  import { sidebarOpen, sidebarCollapsed } from '$lib/sidebar';
  import { auth } from '$lib/auth';
  let classes:any[] = [];
  let err = '';
  let loaded = false;
  $: if ($auth && !loaded) {
    loaded = true;
    (async () => {
      try {
        const result = await apiJSON('/api/classes');
        classes = Array.isArray(result) ? result : [];
      } catch(e:any){ err = e.message }
    })();
  }
</script>
<div class={`fixed left-0 z-40 pointer-events-none group sm:top-0 sm:h-screen top-16 h-[calc(100dvh-4rem)] ${$sidebarOpen ? 'block' : 'hidden sm:block'}`}>
  <aside class={`relative w-60 h-full overflow-y-auto transition-transform pointer-events-auto ${$sidebarCollapsed ? '-translate-x-full' : 'translate-x-0'} p-3`}>
  <div class="surface h-full p-4">
  <button
    class="btn btn-square btn-ghost absolute right-2 top-2 sm:hidden"
    on:click={() => sidebarOpen.set(false)}
    aria-label="Close sidebar"
  >
    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" class="w-5 h-5">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
    </svg>
  </button>
  <h2 class="font-semibold text-sm uppercase tracking-wider text-base-content/70 mb-3">Navigate</h2>
  <ul class="menu mb-3">
    <li>
      <a href="/messages" class={ $page.url.pathname.startsWith('/messages') ? 'active' : ''} on:click={() => sidebarOpen.set(false)}>
        <i class="fa-solid fa-message"></i>
        Messages
      </a>
    </li>
  </ul>
  <h2 class="font-semibold text-sm uppercase tracking-wider text-base-content/70 mb-2">Classes</h2>
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
  </div>
  </aside>
</div>

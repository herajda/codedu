<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import { auth } from '$lib/auth';
  import type { Page } from '@sveltejs/kit';

  let classes:any[] = [];
  let err = '';
  $: role = $auth?.role ?? '';

  onMount(async () => {
    try {
      const result = await apiJSON('/api/classes');
      classes = Array.isArray(result) ? result : [];
    } catch (e:any) { err = e.message }
  });
</script>

<div class="flex min-h-full">
  <aside class="w-60 bg-base-200 p-4 h-screen sticky top-0 overflow-y-auto">
    <h2 class="font-bold mb-2">Classes</h2>
    <ul class="menu">
      {#each classes as c}
        <li>
          {#if role === 'student' && $page.url.pathname.startsWith(`/classes/${c.id}`)}
            <details open>
              <summary>{c.name}</summary>
              <ul>
                <li><a class={$page.url.pathname === `/classes/${c.id}/overview` ? 'active' : ''} href={`/classes/${c.id}/overview`}>Overview</a></li>
                <li><a class={$page.url.pathname === `/classes/${c.id}` ? 'active' : ''} href={`/classes/${c.id}`}>Assignments</a></li>
              </ul>
            </details>
          {:else}
            <a class={($page.params.id == c.id.toString()) ? 'active' : ''} href={`/classes/${c.id}`}>{c.name}</a>
          {/if}
        </li>
      {/each}
      {#if !classes.length && !err}
        <li><i>No classes</i></li>
      {/if}
    </ul>
    {#if err}<p class="text-error mt-2">{err}</p>{/if}
  </aside>
  <div class="flex-1 p-4">
    <slot />
  </div>
</div>

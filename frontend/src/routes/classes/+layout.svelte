<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import type { Page } from '@sveltejs/kit';

  let classes:any[] = [];
  let err = '';

  onMount(async () => {
    try {
      classes = await apiJSON('/api/classes');
    } catch (e:any) { err = e.message }
  });
</script>

<div class="flex min-h-full">
  <aside class="w-60 bg-base-200 p-4">
    <h2 class="font-bold mb-2">Classes</h2>
    <ul class="menu">
      {#each classes as c}
        <li>
          <a class={($page.params.id == c.id.toString()) ? 'active' : ''} href={`/classes/${c.id}`}>{c.name}</a>
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

<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import { sidebarOpen } from '$lib/sidebar';
  let classes:any[] = [];
  let err = '';
  onMount(async () => {
    try {
      const result = await apiJSON('/api/classes');
      classes = Array.isArray(result) ? result : [];
    } catch(e:any){ err = e.message }
  });
</script>
<aside
  class={`fixed top-0 left-0 z-40 w-60 bg-base-200 p-4 h-screen overflow-y-auto transition-transform
    ${$sidebarOpen ? 'block translate-x-0' : 'hidden -translate-x-full'}
    sm:block sm:translate-x-0`}
>
  <h2 class="font-bold mb-2">Classes</h2>
  <ul class="menu">
    {#each classes as c}
      <li>
        <a class={$page.params.id == c.id.toString() ? 'active' : ''} href={`/classes/${c.id}`}>{c.name}</a>
      </li>
    {/each}
    {#if !classes.length && !err}
      <li><i>No classes</i></li>
    {/if}
  </ul>
  {#if err}<p class="text-error mt-2">{err}</p>{/if}
</aside>

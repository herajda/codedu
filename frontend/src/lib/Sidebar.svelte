<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import '@fortawesome/fontawesome-free/css/all.min.css';
  let classes:any[] = [];
  let err = '';
  onMount(async () => {
    try {
      const result = await apiJSON('/api/classes');
      classes = Array.isArray(result) ? result : [];
    } catch(e:any){ err = e.message }
  });
</script>
<aside class="w-60 bg-base-200 p-4 h-screen fixed top-0 left-0 overflow-y-auto">
  <h2 class="font-bold mb-2">Classes</h2>
  <ul class="menu">
    {#each classes as c}
      <li>
        <a
          class={`flex items-center gap-2 ${$page.params.id == c.id.toString() ? 'bg-primary/20 font-semibold' : ''}`}
          href={`/classes/${c.id}`}
        >
          <i class="fa-solid fa-book"></i>
          {c.name}
        </a>
      </li>
    {/each}
    {#if !classes.length && !err}
      <li><i>No classes</i></li>
    {/if}
  </ul>
  {#if err}<p class="text-error mt-2">{err}</p>{/if}
</aside>

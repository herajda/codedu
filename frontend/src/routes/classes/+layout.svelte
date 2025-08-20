<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import type { Page } from '@sveltejs/kit';
  import { auth } from '$lib/auth';

  let classes:any[] = [];
  let err = '';

  let boot = false;
  $: if ($auth && !boot) {
    boot = true;
    (async () => {
      try {
        const result = await apiJSON('/api/classes');
        classes = Array.isArray(result) ? result : [];
      } catch (e:any) { err = e.message }
    })();
  }
</script>

<div class="flex min-h-full">
  <div class="flex-1 p-3 sm:p-4">
    <slot />
  </div>
</div>

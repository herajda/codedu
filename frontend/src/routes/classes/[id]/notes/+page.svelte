<script lang="ts">
import { onMount } from 'svelte';
import { apiJSON } from '$lib/api';
import { page } from '$app/stores';

let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(); }

let notes:any[] = [];
let loading = true;
let err = '';

async function load(){
  loading = true; err = '';
  try {
    notes = await apiJSON(`/api/classes/${id}/notebooks`);
  } catch(e:any){ err = e.message; }
  loading = false;
}

onMount(load);
</script>

<h1 class="text-2xl font-bold mb-4">Notes</h1>
{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <ul class="space-y-2">
    {#each notes as n}
      <li class="flex justify-between items-center">
        <a class="link" href={`/files/${n.id}`}>{n.path}</a>
        <span class="text-sm text-gray-500">{new Date(n.updated_at).toLocaleString()}</span>
      </li>
    {/each}
    {#if !notes.length}
      <li><i>No notes</i></li>
    {/if}
  </ul>
{/if}


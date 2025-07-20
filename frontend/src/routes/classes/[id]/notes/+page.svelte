<script lang="ts">
import { onMount } from 'svelte';
import { apiJSON } from '$lib/api';
import { page } from '$app/stores';
import { goto } from '$app/navigation';

let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(); }

let notes:any[] = [];
let loading = true;
let err = '';

function open(n: any) {
  goto(`/files/${n.id}`);
}

function fmtSize(bytes: number | null | undefined, decimals = 1) {
  if (bytes == null) return '';
  if (bytes < 1024) return `${bytes} B`;

  const units = ['KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
  let i = -1;
  do {
    bytes /= 1024;
    i++;
  } while (bytes >= 1024 && i < units.length - 1);

  return `${bytes.toFixed(decimals)} ${units[i]}`;
}

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
  <div class="overflow-x-auto mb-4">
    <table class="table table-zebra w-full">
      <thead>
        <tr>
          <th class="text-left">Name</th>
          <th class="text-right">Size</th>
          <th class="text-right">Modified</th>
        </tr>
      </thead>
      <tbody>
        {#each notes as n}
          <tr class="hover:bg-base-200 cursor-pointer" on:click={() => open(n)}>
            <td class="whitespace-nowrap">
              <i class="fa-solid fa-book text-secondary mr-2"></i>{n.path}
            </td>
            <td class="text-right">{fmtSize(n.size)}</td>
            <td class="text-right">{new Date(n.updated_at).toLocaleString()}</td>
          </tr>
        {/each}
        {#if !notes.length}
          <tr><td colspan="3"><i>No notes</i></td></tr>
        {/if}
      </tbody>
    </table>
  </div>
{/if}


<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import type { User } from '$lib/auth';
  import { goto } from '$app/navigation';
  import { getKey, decryptText } from '$lib/e2ee';

  let searchTerm = '';
  let results: User[] = [];
  let convos: any[] = [];

  onMount(() => { loadConvos(); });

  async function loadConvos() {
    const list = await apiJSON('/api/messages');
    const k = getKey();
    for (const c of list) {
      if (c.content === '') {
        c.text = '';
      } else if (k) {
        try { c.text = await decryptText(k, c.content); }
        catch { c.text = '[decrypt error]'; }
      } else {
        c.text = '[locked]';
      }
    }
    convos = list;
  }

  async function search() {
    const r = await apiJSON(`/api/user-search?q=${encodeURIComponent(searchTerm)}`);
    results = Array.isArray(r) ? r : [];
  }

  function openChat(u: any) {
    const p = new URLSearchParams();
    if (u.name) p.set('name', u.name);
    else if (u.email) p.set('email', u.email);
    const id = u.id ?? u.other_id;
    goto(`/messages/${id}?${p.toString()}`);
  }
</script>

<h1 class="text-2xl font-bold mb-4">Messages</h1>
<div class="mb-4 space-x-2">
  <input
    class="input input-bordered"
    placeholder="Search"
    bind:value={searchTerm}
    on:keydown={(e) => e.key === 'Enter' && search()}
  />
  <button class="btn" on:click={search}>Search</button>
</div>
{#if results.length}
  <h2 class="font-semibold mb-2">Search results</h2>
  {#each results as u}
    <div class="mb-2"><button class="link" on:click={() => openChat(u)}>{u.name ?? u.email}</button></div>
  {/each}
{/if}

<div class="space-y-2 mt-4">
  {#each convos as c}
    <div class="flex items-center gap-3 p-2 rounded-lg hover:bg-base-200 cursor-pointer" on:click={() => openChat(c)}>
      <div class="avatar">
        <div class="w-12 h-12 rounded-full overflow-hidden">
          {#if c.avatar}
            <img src={c.avatar} alt="Avatar" class="w-full h-full object-cover" />
          {:else}
            <img src="/placeholder.svg?height=48&width=48" alt="Avatar" />
          {/if}
        </div>
      </div>
      <div class="flex-1">
        <div class="font-semibold">{c.name ?? c.email}</div>
        <div class="text-sm opacity-70 truncate">{c.text || (c.image ? '[image]' : '')}</div>
      </div>
      <div class="text-xs opacity-60 whitespace-nowrap">{new Date(c.created_at).toLocaleString()}</div>
    </div>
  {/each}
</div>

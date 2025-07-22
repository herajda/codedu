<script lang="ts">
  import { apiJSON } from '$lib/api';
  import { goto } from '$app/navigation';
  import type { User } from '$lib/auth';

  let searchTerm = '';
  let results: User[] = [];

  async function search() {
    const r = await apiJSON(`/api/user-search?q=${encodeURIComponent(searchTerm)}`);
    results = Array.isArray(r) ? r : [];
  }

  function openChat(u: User) {
    const name = encodeURIComponent(u.name ?? '');
    const email = encodeURIComponent(u.email ?? '');
    goto(`/messages/${u.id}?name=${name}&email=${email}`);
  }
</script>

<h1 class="text-2xl font-bold mb-4">Messages</h1>
<div class="mb-4 space-x-2">
  <input class="input input-bordered" placeholder="Search" bind:value={searchTerm} />
  <button class="btn" on:click={search}>Search</button>
</div>
{#each results as u}
  <div class="mb-2"><button class="link" on:click={() => openChat(u)}>{u.name ?? u.email}</button></div>
{/each}

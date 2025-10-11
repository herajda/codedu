<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { apiJSON } from '$lib/api';
  import type { User } from '$lib/auth';
  import { translator } from '$lib/i18n';

  let translate;
  $: translate = $translator;

  let searchTerm = $page.url.searchParams.get('q') ?? '';
  let results: User[] = [];
  let inputEl: HTMLInputElement | null = null;

  $: if (searchTerm.trim() !== '') {
    fetchResults(searchTerm);
  } else {
    results = [];
  }

  async function fetchResults(q: string) {
    const r = await apiJSON(`/api/user-search?q=${encodeURIComponent(q)}`);
    results = Array.isArray(r) ? r : [];
  }

  function handleInput(e: Event) {
    searchTerm = (e.target as HTMLInputElement).value;
  }

  onMount(() => { inputEl?.focus(); });

  function openChat(u: any) {
    const p = new URLSearchParams();
    if (u.name) p.set('name', u.name);
    else if (u.email) p.set('email', u.email);
    const id = u.other_id ?? u.id;
    goto(`/messages/${id}?${p.toString()}`);
  }
</script>

<h1 class="text-2xl font-bold mb-4">{translate('frontend/src/routes/messages/search/+page.svelte::new_message')}</h1>
<div class="mb-4">
  <input
    class="input input-bordered w-full sm:max-w-xs"
    placeholder={translate('frontend/src/routes/messages/search/+page.svelte::search_placeholder')}
    bind:value={searchTerm}
    bind:this={inputEl}
    on:input={handleInput}
  />
</div>

<div class="space-y-2">
  {#each results as u (u.id)}
    {#if u}
    <div class="flex items-center gap-3 p-2 rounded-lg hover:bg-base-200 cursor-pointer" on:click={() => openChat(u)}>
      <div class="avatar">
        <div class="w-10 h-10 rounded-full overflow-hidden">
          {#if u.avatar}
            <img src={u.avatar} alt={translate('frontend/src/routes/messages/search/+page.svelte::avatar_alt')} class="w-full h-full object-cover" />
          {:else}
            <img src="/placeholder.svg?height=40&width=40" alt={translate('frontend/src/routes/messages/search/+page.svelte::avatar_alt')} />
          {/if}
        </div>
      </div>
      <div>{u.name ?? u.email}</div>
    </div>
    {/if}
  {/each}
</div>

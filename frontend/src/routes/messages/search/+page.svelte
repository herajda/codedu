<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { apiJSON } from '$lib/api';
  import type { User } from '$lib/auth';
  import { t, translator } from '$lib/i18n';
  import { Search, ChevronLeft, UserPlus, MessageSquare, Sparkles } from 'lucide-svelte';
  import { fade, slide, scale } from 'svelte/transition';

  let translate;
  $: translate = $translator;

  let searchTerm = $page.url.searchParams.get('q') ?? '';
  let results: User[] = [];
  let inputEl: HTMLInputElement | null = null;
  let isLoading = false;

  $: if (searchTerm.trim() !== '') {
    isLoading = true;
    fetchResults(searchTerm);
  } else {
    results = [];
    isLoading = false;
  }

  async function fetchResults(q: string) {
    try {
      const r = await apiJSON(`/api/user-search?q=${encodeURIComponent(q)}`);
      results = Array.isArray(r) ? r : [];
    } finally {
      isLoading = false;
    }
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

  function back() {
    goto('/messages');
  }
</script>

<svelte:head>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@100..900&display=swap" rel="stylesheet">
  <title>{translate('frontend/src/routes/messages/search/+page.svelte::new_message')} | CodEdu</title>
</svelte:head>

<div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8 h-[calc(100vh-4rem)] overflow-y-auto custom-scrollbar">
  <!-- Premium Header -->
  <section class="relative overflow-hidden bg-base-100 rounded-[2.5rem] border border-base-200 shadow-xl shadow-base-300/30 mb-8 p-6 sm:p-10 shrink-0">
    <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
    <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none"></div>
    
    <div class="relative flex flex-col md:flex-row items-center justify-between gap-6">
      <div class="flex-1 text-center md:text-left">
        <div class="flex items-center justify-center md:justify-start gap-4 mb-2">
            <button class="btn btn-ghost btn-circle bg-base-200 hover:bg-base-300 border-none transition-all" on:click={back}>
                <ChevronLeft size={24} />
            </button>
            <h1 class="text-3xl sm:text-4xl font-black tracking-tight tracking-tight">
              {translate('frontend/src/routes/messages/search/+page.svelte::new_message')}
            </h1>
        </div>
        <p class="text-base-content/60 font-medium max-w-xl mx-auto md:mx-0 ml-0 md:ml-14">
          {translate('frontend/src/routes/messages/search/+page.svelte::search_info')}
        </p>
      </div>
    </div>
  </section>

  <!-- Search Input Section -->
  <div class="card bg-base-100 rounded-[2rem] border border-base-200 shadow-xl shadow-base-300/20 mb-8 overflow-hidden">
      <div class="p-8">
          <div class="relative">
              <Search class="w-6 h-6 absolute left-5 top-1/2 transform -translate-y-1/2 text-primary opacity-40" />
              <input
                class="input bg-base-200/50 border-transparent focus:border-primary/40 w-full pl-14 h-16 rounded-2xl font-bold text-lg shadow-inner transition-all"
                placeholder={translate('frontend/src/routes/messages/search/+page.svelte::search_placeholder')}
                bind:value={searchTerm}
                bind:this={inputEl}
                on:input={handleInput}
              />
              {#if isLoading}
                <div class="absolute right-5 top-1/2 transform -translate-y-1/2">
                    <span class="loading loading-spinner text-primary"></span>
                </div>
              {/if}
          </div>
      </div>
  </div>

  <!-- Results Section -->
  <div class="space-y-4">
    {#if searchTerm.trim() !== '' && results.length === 0 && !isLoading}
        <div class="py-20 text-center bg-base-100/50 rounded-[3rem] border-2 border-dashed border-base-200" in:fade>
            <div class="w-20 h-20 bg-base-200 rounded-full flex items-center justify-center mx-auto mb-4 opacity-30">
                <Search size={32} />
            </div>
            <p class="text-sm font-black uppercase tracking-widest opacity-30 italic">{translate('frontend/src/routes/messages/search/+page.svelte::no_results_for', { values: { searchTerm } })}</p>
        </div>
    {:else if results.length > 0}
        <div class="grid gap-4 sm:grid-cols-2">
          {#each results as u (u.id)}
            {#if u}
                <div 
                    class="group relative bg-base-100 p-5 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-2xl hover:shadow-primary/5 hover:border-primary/30 transition-all cursor-pointer flex items-center gap-4 overflow-hidden" 
                    on:click={() => openChat(u)}
                    in:fade={{ duration: 200 }}
                >
                    <div class="absolute top-0 right-0 w-24 h-24 bg-primary/5 rounded-bl-[100%] pointer-events-none group-hover:scale-150 transition-transform duration-700"></div>
                    
                    <div class="avatar shadow-lg shadow-base-300/40 rounded-full relative">
                        <div class="w-14 h-14 rounded-2xl overflow-hidden group-hover:scale-105 transition-transform duration-500 bg-base-200">
                            {#if u.avatar}
                                <img src={u.avatar} alt={translate('frontend/src/routes/messages/search/+page.svelte::avatar_alt')} class="w-full h-full object-cover" />
                            {:else}
                                <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-xl font-black text-primary">
                                    {(u.name ?? u.email ?? '?').charAt(0).toUpperCase()}
                                </div>
                            {/if}
                        </div>
                    </div>

                    <div class="flex-1 min-w-0">
                        <h3 class="font-black text-lg tracking-tight truncate group-hover:text-primary transition-colors">{u.name ?? translate('frontend/src/routes/messages/+page.svelte::unknown_user')}</h3>
                        <p class="text-[10px] font-bold uppercase tracking-widest opacity-40 truncate">{u.email}</p>
                    </div>


                </div>
            {/if}
          {/each}
        </div>
    {:else if !searchTerm.trim()}
        <div class="py-24 text-center bg-base-100/50 rounded-[4rem] border-2 border-dashed border-base-200 px-6" in:fade>
            <div class="p-6 bg-gradient-to-br from-primary/10 to-secondary/10 rounded-[2.5rem] w-24 h-24 mx-auto mb-6 flex items-center justify-center shadow-inner">
                <Sparkles class="w-10 h-10 text-primary animate-pulse" />
            </div>
            <h3 class="text-2xl font-black mb-2 tracking-tight">{translate('frontend/src/routes/messages/search/+page.svelte::search_prompt')}</h3>
            <p class="text-base-content/60 font-medium mb-0 max-w-sm mx-auto">{translate('frontend/src/routes/messages/search/+page.svelte::search_help')}</p>
        </div>
    {/if}
  </div>
</div>

<style>
  :global(body) {
    font-family: 'Outfit', sans-serif;
  }

  .custom-scrollbar::-webkit-scrollbar {
    width: 6px;
  }
  .custom-scrollbar::-webkit-scrollbar-track {
    background: transparent;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb {
    background: hsl(var(--bc) / 0.1);
    border-radius: 10px;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb:hover {
    background: hsl(var(--bc) / 0.2);
  }
</style>

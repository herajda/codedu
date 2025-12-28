<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, apiFetch } from '$lib/api';
  import { goto } from '$app/navigation';
  import { Search, MessageCircle, Plus, MoreVertical, Archive, Trash2, Star, StarOff, RefreshCw, X, UserPlus, ShieldAlert, Sparkles, AlertCircle } from 'lucide-svelte';
  import NewChatModal from '$lib/components/NewChatModal.svelte';
  import { onlineUsers } from '$lib/stores/onlineUsers';
  import { formatDate as formatDisplayDate } from '$lib/date';
  import { t, translator } from '$lib/i18n';
  import { fade, slide, scale } from 'svelte/transition';

  let translate;
  $: translate = $translator;

  let convos: any[] = [];
  let filteredConvos: any[] = [];
  let searchQuery = '';
  let isLoading = true;
  let selectedFilter = 'all'; // all, unread, starred, archived
  let showArchived = false;
  let showNewChatModal = false;
  let totalUnreadCount = 0;
  let blockedUsers: any[] = [];
  let showBlockedModal = false;

  onMount(() => {
    loadConvos();
    loadBlocked();
  });

  async function loadConvos() {
    isLoading = true;
    try {
      const list = await apiJSON('/api/messages');
      for (const c of list) {
        c.text = c.text ?? '';
        c.lastMessageTime = new Date(c.created_at);
        c.status = getStatus(c.lastMessageTime);
        c.displayName = c.name ?? c.email?.split('@')[0] ?? t('frontend/src/routes/messages/+page.svelte::unknown_user');
      }
      convos = list.sort((a: any, b: any) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
      totalUnreadCount = convos.reduce((sum, c) => sum + (c.unread_count || 0), 0);
      applyFilters();
    } catch (error) {
      console.error('Failed to load conversations:', error);
    } finally {
      isLoading = false;
    }
  }

  async function loadBlocked() {
    try {
      blockedUsers = await apiJSON('/api/blocked-users');
    } catch (e) {
      console.error('Failed to load blocked users', e);
    }
  }

  async function unblock(id: number) {
    try {
      await apiFetch(`/api/users/${id}/block`, { method: 'DELETE' });
      blockedUsers = blockedUsers.filter(u => u.id !== id);
      loadConvos();
    } catch (e) {
      console.error('Failed to unblock user', e);
    }
  }

  function applyFilters() {
    let filtered = [...convos];
    
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      filtered = filtered.filter(c => 
        c.displayName.toLowerCase().includes(query) ||
        c.email?.toLowerCase().includes(query) ||
        c.text?.toLowerCase().includes(query)
      );
    }
    
    switch (selectedFilter) {
      case 'unread':
        filtered = filtered.filter(c => c.unread_count > 0);
        break;
      case 'starred':
        filtered = filtered.filter(c => c.starred);
        break;
      case 'archived':
        filtered = filtered.filter(c => c.archived);
        break;
      default:
        filtered = filtered.filter(c => !c.archived);
    }
    
    filteredConvos = filtered;
  }

  $: searchQuery, applyFilters();
  $: selectedFilter, applyFilters();

  function isToday(date: Date): boolean {
    const today = new Date();
    return date.toDateString() === today.toDateString();
  }

  function isYesterday(date: Date): boolean {
    const yesterday = new Date();
    yesterday.setDate(yesterday.getDate() - 1);
    return date.toDateString() === yesterday.toDateString();
  }

  function isThisWeek(date: Date): boolean {
    const today = new Date();
    const weekAgo = new Date();
    weekAgo.setDate(today.getDate() - 7);
    return date >= weekAgo;
  }

  function getStatus(date: Date): string {
    if (isToday(date)) return 'today';
    if (isYesterday(date)) return 'yesterday';
    if (isThisWeek(date)) return 'this-week';
    return 'older';
  }

  function formatTime(date: Date): string {
    if (isToday(date)) {
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } else if (isYesterday(date)) {
      return t('frontend/src/routes/messages/+page.svelte::yesterday');
    } else if (isThisWeek(date)) {
      return date.toLocaleDateString([], { weekday: 'short' });
    } else {
      return formatDisplayDate(date);
    }
  }

  function getStatusColor(status: string): string {
    switch (status) {
      case 'today': return 'text-success';
      case 'yesterday': return 'text-primary';
      case 'this-week': return 'text-secondary';
      default: return 'text-base-content/40';
    }
  }

  function openChat(u: any) {
    const p = new URLSearchParams();
    if (u.name) p.set('name', u.name);
    else if (u.email) p.set('email', u.email);
    const id = u.other_id ?? u.id;
    goto(`/messages/${id}?${p.toString()}`);
  }

  function startNewChat() {
    showNewChatModal = true;
  }

  function handleNewChat(event: CustomEvent) {
    const { user } = event.detail;
    showNewChatModal = false;
    const p = new URLSearchParams();
    if (user.name) p.set('name', user.name);
    else if (user.email) p.set('email', user.email);
    goto(`/messages/${user.id}?${p.toString()}`);
  }

  function closeNewChatModal() {
    showNewChatModal = false;
  }

  function openBlockedModal() {
    showBlockedModal = true;
    loadBlocked();
  }
  function closeBlockedModal() { showBlockedModal = false; }

  async function toggleArchive(convo: any, event: Event) {
    event.stopPropagation();
    const id = convo.other_id ?? convo.id;
    try {
      if (convo.archived) {
        await apiFetch(`/api/messages/${id}/archive`, { method: 'DELETE' });
        convo.archived = false;
      } else {
        await apiFetch(`/api/messages/${id}/archive`, { method: 'POST' });
        convo.archived = true;
      }
      applyFilters();
    } catch (e) {
      console.error('Failed to toggle archive', e);
    }
  }

  async function toggleStar(convo: any, event: Event) {
    event.stopPropagation();
    const id = convo.other_id ?? convo.id;
    try {
      if (convo.starred) {
        await apiFetch(`/api/messages/${id}/star`, { method: 'DELETE' });
        convo.starred = false;
      } else {
        await apiFetch(`/api/messages/${id}/star`, { method: 'POST' });
        convo.starred = true;
      }
      applyFilters();
    } catch (e) {
      console.error('Failed to toggle star', e);
    }
  }

  function deleteChat(convo: any, event: Event) {
    event.stopPropagation();
    // TODO: Implement delete functionality
  }
</script>

<svelte:head>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@100..900&display=swap" rel="stylesheet">
  <title>{t('frontend/src/routes/messages/+page.svelte::messages_title')} | CodEdu</title>
</svelte:head>

<div class="messages-page max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-8 h-[calc(100vh-4rem)] overflow-y-auto custom-scrollbar">
  <!-- Premium Header Section -->
  <section class="relative overflow-hidden bg-base-100 rounded-[2.5rem] border border-base-200 shadow-xl shadow-base-300/30 mb-8 p-6 sm:p-10 shrink-0">
    <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
    <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none"></div>
    <div class="absolute -bottom-24 -left-24 w-48 h-48 bg-secondary/10 rounded-full blur-3xl pointer-events-none"></div>
    
    <div class="relative flex flex-col md:flex-row items-center justify-between gap-6">
      <div class="flex-1 text-center md:text-left">
        <div class="flex items-center justify-center md:justify-start gap-3 mb-2">
            <div class="p-2.5 bg-primary/10 rounded-2xl">
              <MessageCircle class="w-6 h-6 text-primary" />
            </div>
            <h1 class="text-3xl sm:text-4xl font-black tracking-tight">
              {t('frontend/src/routes/messages/+page.svelte::messages_title')}
            </h1>
        </div>
        <p class="text-base-content/60 font-medium max-w-xl mx-auto md:mx-0">
          {t('frontend/src/routes/messages/+page.svelte::connect_message')}
        </p>
      </div>
      
      <div class="flex flex-wrap items-center gap-3">
        <button 
          class="btn btn-ghost btn-circle bg-base-200/50 hover:bg-base-200 border-none transition-all" 
          on:click={loadConvos}
          aria-label={t('frontend/src/routes/messages/+page.svelte::refresh_label')}
          disabled={isLoading}
        >
          <RefreshCw class={`w-5 h-5 ${isLoading ? 'animate-spin' : ''}`} />
        </button>
        <button 
          class="btn btn-primary rounded-2xl gap-2 font-black uppercase tracking-widest text-[11px] h-12 px-6 shadow-lg shadow-primary/20" 
          on:click={startNewChat}
        >
          <Plus size={18} />
          {t('frontend/src/routes/messages/+page.svelte::new_chat_label')}
        </button>
        
        <div class="dropdown dropdown-end">
          <button class="btn btn-ghost btn-circle bg-base-200/50 hover:bg-base-200 border-none transition-all" aria-label={t('frontend/src/routes/messages/+page.svelte::settings_label')}>
            <MoreVertical class="w-5 h-5" />
          </button>
          <ul class="dropdown-content menu p-2 shadow-2xl bg-base-100 rounded-2xl w-56 z-50 border border-base-200 mt-2">
            <li class="menu-title px-4 py-2 text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/routes/messages/+page.svelte::settings_label')}</li>
            <li>
              <button class="gap-3 py-3 rounded-xl" on:click={openBlockedModal}>
                <ShieldAlert size={18} class="text-error" />
                <span class="font-bold">{t('frontend/src/routes/messages/+page.svelte::view_blocked_users')}</span>
              </button>
            </li>
          </ul>
        </div>
      </div>
    </div>
  </section>

  <!-- Search and Filters -->
  <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 mb-8 px-2 shrink-0">
    <div class="relative flex-1 max-w-lg">
      <Search class="w-4 h-4 absolute left-4 top-1/2 transform -translate-y-1/2 text-base-content/40" />
      <input
        class="input bg-base-100 border-base-200 focus:border-primary/30 w-full pl-11 rounded-[1.25rem] font-medium text-sm h-12 shadow-sm transition-all"
        placeholder={t('frontend/src/routes/messages/+page.svelte::search_placeholder')}
        bind:value={searchQuery}
      />
    </div>

    <div class="flex items-center gap-3">
      <div class="flex items-center bg-base-200/50 p-1.5 rounded-[1.25rem] h-12 w-full lg:w-auto overflow-x-auto no-scrollbar">
          {#each ['all', 'unread', 'starred', 'archived'] as filter}
            <button 
                class={`btn btn-xs border-none rounded-xl h-9 px-4 font-black uppercase tracking-widest text-[10px] transition-all whitespace-nowrap ${selectedFilter === filter ? 'bg-base-100 shadow-sm text-primary' : 'bg-transparent opacity-50 hover:opacity-100'}`} 
                on:click={() => selectedFilter = filter}
            >
                {t(`frontend/src/routes/messages/+page.svelte::filter_${filter}`)}
                {#if filter === 'unread' && totalUnreadCount > 0}
                    <span class="ml-1.5 px-1.5 py-0.5 bg-primary text-primary-content rounded-lg text-[9px] tabular-nums">{totalUnreadCount}</span>
                {/if}
            </button>
          {/each}
      </div>
    </div>
  </div>

  <!-- Conversations List -->
  <div class="flex-1 min-h-0">
    {#if isLoading}
      <div class="py-20 flex flex-col items-center justify-center text-center bg-base-100/50 rounded-[3rem] border-2 border-dashed border-base-200">
        <div class="loading loading-spinner loading-lg text-primary mb-4"></div>
        <p class="text-sm font-black uppercase tracking-widest opacity-40">{t('frontend/src/routes/messages/+page.svelte::loading_conversations')}</p>
      </div>
    {:else if filteredConvos.length === 0}
      <div class="py-24 text-center bg-base-100/50 rounded-[3.5rem] border-2 border-dashed border-base-200 px-6" in:fade>
        <div class="p-6 bg-gradient-to-br from-primary/10 to-secondary/10 rounded-[2.5rem] w-24 h-24 mx-auto mb-6 flex items-center justify-center shadow-inner">
          <Sparkles class="w-10 h-10 text-primary animate-pulse" />
        </div>
        <h3 class="text-2xl font-black mb-3 tracking-tight">
          {#if searchQuery}
            {translate('frontend/src/routes/messages/+page.svelte::no_conversations_found')}
          {:else}
            {t('frontend/src/routes/messages/+page.svelte::ready_to_chat_title')}
          {/if}
        </h3>
        <p class="text-base-content/60 font-medium mb-10 max-w-md mx-auto leading-relaxed">
          {#if searchQuery}
            {translate('frontend/src/routes/messages/+page.svelte::no_conversations_match_search', { values: { searchQuery } })}
          {:else}
            {t('frontend/src/routes/messages/+page.svelte::connect_message')}
          {/if}
        </p>
        <div class="flex flex-col sm:flex-row gap-4 justify-center">
            <button class="btn btn-primary rounded-[1.5rem] h-14 px-8 font-black uppercase tracking-widest text-[11px] gap-3 shadow-xl shadow-primary/20" on:click={startNewChat}>
              <UserPlus class="w-5 h-5" />
              {t('frontend/src/routes/messages/+page.svelte::start_new_chat_button')}
            </button>
            {#if searchQuery}
              <button class="btn btn-outline rounded-[1.5rem] h-14 px-8 font-black uppercase tracking-widest text-[11px]" on:click={() => searchQuery = ''}>
                {t('frontend/src/routes/messages/+page.svelte::clear_search_button')}
              </button>
            {/if}
        </div>
      </div>
    {:else}
      <div class="grid gap-6 md:grid-cols-2">
        {#each filteredConvos as convo (convo.id)}
          <div 
            class="group relative bg-base-100 p-6 rounded-[2.5rem] border border-base-200 shadow-sm hover:shadow-2xl hover:shadow-primary/5 hover:border-primary/20 transition-all duration-300 cursor-pointer overflow-hidden flex flex-col"
            on:click={() => openChat(convo)}
            in:fade={{ duration: 200 }}
          >
            <!-- Decorative gradient on hover -->
            <div class="absolute top-0 right-0 w-32 h-32 bg-primary/5 rounded-bl-[100%] pointer-events-none group-hover:scale-150 transition-transform duration-700"></div>
            
            <div class="flex items-start gap-4 mb-4">
              <!-- Avatar -->
              <div class="relative flex-shrink-0">
                <div class="avatar shadow-lg shadow-base-300/40 rounded-full">
                  <div class="w-16 h-16 rounded-[1.5rem] overflow-hidden group-hover:scale-105 transition-transform duration-500 bg-base-200">
                    {#if convo.avatar}
                      <img src={convo.avatar} alt={t('frontend/src/routes/messages/+page.svelte::avatar_alt')} class="w-full h-full object-cover" />
                    {:else}
                      <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-2xl font-black text-primary">
                        {convo.displayName.charAt(0).toUpperCase()}
                      </div>
                    {/if}
                  </div>
                </div>
                <!-- Online indicator -->
                {#if $onlineUsers.some(u => u.id === convo.other_id)}
                  <div class="absolute -bottom-1 -right-1 w-5 h-5 bg-success rounded-full border-[3px] border-base-100 shadow-sm animate-pulse"></div>
                {:else}
                  <div class="absolute -bottom-1 -right-1 w-5 h-5 bg-base-300 rounded-full border-[3px] border-base-100 shadow-sm"></div>
                {/if}
              </div>

              <!-- Content -->
              <div class="flex-1 min-w-0 pt-1">
                <div class="flex items-start justify-between gap-2 overflow-hidden mb-1">
                  <h3 class="font-black text-xl tracking-tight truncate group-hover:text-primary transition-colors">
                    {convo.displayName}
                  </h3>
                  <div class="flex flex-col items-end shrink-0">
                    <span class={`text-[10px] font-black uppercase tracking-widest ${getStatusColor(convo.status)}`}>
                      {formatTime(convo.lastMessageTime)}
                    </span>
                  </div>
                </div>
                
                <div class="flex items-center gap-2">
                   {#if convo.unread_count > 0}
                     <div class="w-2.5 h-2.5 bg-primary rounded-full shadow-lg shadow-primary/50 shrink-0"></div>
                   {/if}
                   <p class="text-sm font-medium text-base-content/60 truncate italic">
                    {#if convo.text}
                      "{convo.text}"
                    {:else if convo.image}
                      {t('frontend/src/routes/messages/+page.svelte::image')}
                    {:else}
                      {t('frontend/src/routes/messages/+page.svelte::no_messages_yet')}
                    {/if}
                   </p>
                </div>
              </div>
            </div>

            <div class="mt-auto flex items-center justify-between pt-4 border-t border-base-300/30">
                <div class="flex items-center gap-4 min-w-0">
                     <div class="flex items-center gap-1.5 px-3 py-1 bg-base-200/50 rounded-full text-[10px] font-black uppercase tracking-widest text-base-content/40 truncate">
                         {convo.email ? convo.email : 'id: ' + convo.other_id}
                     </div>
                     {#if convo.unread_count > 0}
                        <div class="badge badge-primary rounded-lg font-black text-[10px] tabular-nums px-2 border-none shrink-0">
                            {convo.unread_count} {t('frontend/src/routes/messages/+page.svelte::filter_unread')}
                        </div>
                     {/if}
                </div>

                <!-- Hover Actions -->
                <div class="flex items-center gap-1 shrink-0">
                    <button 
                        class="btn btn-ghost btn-xs btn-circle hover:bg-warning/20 hover:text-warning transition-colors"
                        on:click={(e) => toggleStar(convo, e)}
                        title={t('frontend/src/routes/messages/+page.svelte::star_conversation_title')}
                    >
                        {#if convo.starred}
                            <Star class="w-4 h-4 text-warning fill-current" />
                        {:else}
                            <StarOff class="w-4 h-4" />
                        {/if}
                    </button>
                    <div class="dropdown dropdown-left">
                        <button 
                            class="btn btn-ghost btn-xs btn-circle hover:bg-base-200 transition-colors"
                            on:click={(e) => e.stopPropagation()}
                        >
                            <MoreVertical class="w-4 h-4" />
                        </button>
                        <ul class="dropdown-content menu p-2 shadow-2xl bg-base-100 rounded-2xl w-48 z-50 border border-base-200 mb-2">
                            <li>
                                <button class="gap-2 py-2.5 rounded-xl font-bold" on:click={(e) => toggleArchive(convo, e)}>
                                    <Archive class="w-4 h-4" />
                                    {convo.archived ? t('frontend/src/routes/messages/+page.svelte::unarchive') : t('frontend/src/routes/messages/+page.svelte::archive')}
                                </button>
                            </li>
                            <li>
                                <button class="gap-2 py-2.5 rounded-xl font-bold text-error hover:bg-error/10" on:click={(e) => deleteChat(convo, e)}>
                                    <Trash2 class="w-4 h-4" />
                                    {t('frontend/src/routes/messages/+page.svelte::delete')}
                                </button>
                            </li>
                        </ul>
                    </div>
                </div>
            </div>
            
          </div>
        {/each}
      </div>
      
      <!-- Stats Footer -->
      <div class="mt-12 py-8 text-center border-t border-base-300/30">
        <p class="text-[11px] font-black uppercase tracking-[0.2em] text-base-content/30 italic">
          {translate('frontend/src/routes/messages/+page.svelte::showing_conversations_count', { values: { filteredCount: filteredConvos.length, totalCount: convos.length } })}
          {#if searchQuery}
            {translate('frontend/src/routes/messages/+page.svelte::matching_search_query', { values: { searchQuery } })}
          {/if}
        </p>
      </div>
    {/if}
  </div>
</div>

<!-- New Chat Modal -->
{#if showNewChatModal}
  <NewChatModal 
    on:startChat={handleNewChat}
    on:close={closeNewChatModal}
  />
{/if}

<!-- Blocked Users Modal -->
{#if showBlockedModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-md p-4" transition:fade>
    <div class="bg-base-100 w-full max-w-lg rounded-[2.5rem] shadow-2xl border border-base-200 overflow-hidden" in:scale={{ duration: 300, start: 0.9 }}>
      <div class="flex items-center justify-between p-8 border-b border-base-200">
        <div class="flex items-center gap-3">
            <div class="p-2 bg-error/10 rounded-xl">
                <ShieldAlert class="w-5 h-5 text-error" />
            </div>
            <h3 class="text-xl font-black tracking-tight">{t('frontend/src/routes/messages/+page.svelte::blocked_users_title')}</h3>
        </div>
        <button class="btn btn-ghost btn-sm btn-circle bg-base-200 hover:bg-base-300 border-none transition-all" on:click={closeBlockedModal} aria-label={t('frontend/src/routes/messages/+page.svelte::close_label')}>
          <X class="w-4 h-4" />
        </button>
      </div>
      
      <div class="p-4">
          {#if blockedUsers.length === 0}
            <div class="py-12 text-center text-base-content/40 font-bold italic">
                {t('frontend/src/routes/messages/+page.svelte::no_blocked_users')}
            </div>
          {:else}
            <div class="max-h-96 overflow-y-auto space-y-2 pr-2 custom-scrollbar">
              {#each blockedUsers as u (u.id)}
                <div class="p-4 bg-base-200/50 rounded-2xl flex items-center justify-between group">
                  <div class="flex items-center gap-4 min-w-0">
                    <div class="avatar">
                      <div class="w-12 h-12 rounded-xl overflow-hidden ring-4 ring-base-100 shadow-sm">
                        {#if u.avatar}
                          <img src={u.avatar} alt={t('frontend/src/routes/messages/+page.svelte::avatar_alt')} class="w-full h-full object-cover" />
                        {:else}
                          <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-base font-black text-primary">
                            {(u.name ?? u.email ?? '?').charAt(0).toUpperCase()}
                          </div>
                        {/if}
                      </div>
                    </div>
                    <div class="flex flex-col min-w-0">
                        <span class="font-black tracking-tight truncate">{u.name ?? u.email}</span>
                        <span class="text-[10px] font-bold uppercase tracking-widest opacity-40">{u.email}</span>
                    </div>
                  </div>
                  <button class="btn btn-sm btn-outline rounded-xl font-black text-[10px] uppercase tracking-widest px-4 hover:bg-primary hover:text-primary-content hover:border-primary transition-all" on:click={() => unblock(u.id)}>
                    {t('frontend/src/routes/messages/+page.svelte::unblock_button')}
                  </button>
                </div>
              {/each}
            </div>
          {/if}
      </div>

      <div class="p-8 border-t border-base-200 flex justify-end">
        <button class="btn btn-ghost rounded-xl font-black uppercase tracking-widest text-[10px] px-6" on:click={closeBlockedModal}>{t('frontend/src/routes/messages/+page.svelte::close_button')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .messages-page {
    font-family: 'Outfit', sans-serif;
  }

  .no-scrollbar::-webkit-scrollbar {
    display: none;
  }
  .no-scrollbar {
    -ms-overflow-style: none;
    scrollbar-width: none;
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

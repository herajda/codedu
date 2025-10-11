<script lang="ts">
  import { onMount } from 'svelte';
import { apiJSON, apiFetch } from '$lib/api';
import { goto } from '$app/navigation';
import { Search, MessageCircle, Plus, MoreVertical, Archive, Trash2, Star, StarOff, RefreshCw, X } from 'lucide-svelte';
import NewChatModal from '$lib/components/NewChatModal.svelte';
import { onlineUsers } from '$lib/stores/onlineUsers';
import { formatDate as formatDisplayDate } from '$lib/date';
  import { t, translator } from '$lib/i18n'; // Added import

  let translate; // Added declaration for reactive translation
  $: translate = $translator; // Added reactive assignment

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
        // Add computed properties for better UX
        c.lastMessageTime = new Date(c.created_at);
        c.isToday = isToday(c.lastMessageTime);
        c.isYesterday = isYesterday(c.lastMessageTime);
        c.isThisWeek = isThisWeek(c.lastMessageTime);
        // Translate 'Unknown'
        c.displayName = c.name ?? c.email?.split('@')[0] ?? t('frontend/src/routes/messages/+page.svelte::unknown_user');
        c.status = getStatus(c.lastMessageTime);
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
    
    // Apply search filter
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      filtered = filtered.filter(c => 
        c.displayName.toLowerCase().includes(query) ||
        c.email?.toLowerCase().includes(query) ||
        c.text?.toLowerCase().includes(query)
      );
    }
    
    // Apply status filter
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
      // Translated 'Yesterday'
      return t('frontend/src/routes/messages/+page.svelte::yesterday');
    } else if (isThisWeek(date)) {
      return date.toLocaleDateString([], { weekday: 'short' });
    } else {
      return formatDisplayDate(date);
    }
  }

  function getStatusColor(status: string): string {
    switch (status) {
      case 'today': return 'text-green-500';
      case 'yesterday': return 'text-blue-500';
      case 'this-week': return 'text-yellow-500';
      default: return 'text-gray-400';
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
    // Navigate to the chat with the selected user
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
    // Ensure latest list is loaded when opening
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
    console.log('Delete chat:', convo.id); // Internal log, not translated
  }
</script>

<div class="max-w-4xl mx-auto px-3 sm:px-0">
  <!-- Header Section -->
  <div class="card-elevated mb-6">
    <div class="p-6">
        <div class="flex items-center justify-between mb-6 flex-wrap gap-3">
        <div class="flex items-center gap-3">
          <div class="p-2 bg-primary/10 rounded-lg">
            <MessageCircle class="w-6 h-6 text-primary" />
          </div>
          <div>
            <h1 class="text-2xl font-bold">{t('frontend/src/routes/messages/+page.svelte::messages_title')}</h1>
          </div>
        </div>
        <div class="flex gap-2 w-full sm:w-auto justify-end sm:justify-normal">
          <button 
            class="btn btn-ghost btn-sm btn-circle" 
            on:click={loadConvos}
            aria-label={t('frontend/src/routes/messages/+page.svelte::refresh_label')}
            disabled={isLoading}
          >
            <RefreshCw class={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
          <button 
            class="btn btn-primary btn-sm btn-circle" 
            on:click={startNewChat}
            aria-label={t('frontend/src/routes/messages/+page.svelte::new_chat_label')}
          >
            <Plus class="w-4 h-4" />
          </button>
          <!-- Settings Dropdown (opens Blocked Users modal) -->
          <div class="dropdown dropdown-end">
            <button class="btn btn-ghost btn-sm btn-circle" aria-label={t('frontend/src/routes/messages/+page.svelte::settings_label')}>
              <MoreVertical class="w-4 h-4" />
            </button>
            <ul class="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-56 z-50 border border-base-300/30">
              <li>
                <button class="gap-2" on:click={openBlockedModal}>{t('frontend/src/routes/messages/+page.svelte::view_blocked_users')}</button>
              </li>
            </ul>
          </div>
        </div>
      </div>

      <!-- Search and Filters -->
        <div class="flex flex-col sm:flex-row gap-4">
        <div class="flex-1 relative">
          <Search class="w-4 h-4 absolute left-3 top-1/2 transform -translate-y-1/2 text-base-content/40" />
            <input
              class="input input-bordered w-full pl-10"
            placeholder={t('frontend/src/routes/messages/+page.svelte::search_placeholder')}
            bind:value={searchQuery}
          />
        </div>
          <div class="flex gap-2">
          <select 
            class="select select-bordered select-sm" 
            bind:value={selectedFilter}
          >
            <option value="all">{t('frontend/src/routes/messages/+page.svelte::filter_all')}</option>
            <option value="unread">{t('frontend/src/routes/messages/+page.svelte::filter_unread')}</option>
            <option value="starred">{t('frontend/src/routes/messages/+page.svelte::filter_starred')}</option>
            <option value="archived">{t('frontend/src/routes/messages/+page.svelte::filter_archived')}</option>
          </select>
          <button 
            class={`btn btn-outline btn-sm ${showArchived ? 'btn-active' : ''}`} 
            on:click={() => showArchived = !showArchived}
          >
            <Archive class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  </div>

  <!-- Conversations List -->
  <div class="card-elevated">
    {#if isLoading}
      <div class="p-8 text-center">
        <div class="loading loading-spinner loading-lg text-primary"></div>
        <p class="mt-4 text-base-content/60">{t('frontend/src/routes/messages/+page.svelte::loading_conversations')}</p>
      </div>
    {:else if filteredConvos.length === 0}
       <div class="p-8 text-center">
        <div class="p-4 bg-gradient-to-br from-primary/10 to-secondary/10 rounded-full w-20 h-20 mx-auto mb-6 flex items-center justify-center">
          <MessageCircle class="w-10 h-10 text-primary" />
        </div>
        <h3 class="text-xl font-semibold mb-3">
          {#if searchQuery}
            {translate('frontend/src/routes/messages/+page.svelte::no_conversations_found')}
          {:else}
            {t('frontend/src/routes/messages/+page.svelte::ready_to_chat_title')}
          {/if}
        </h3>
        <p class="text-base-content/60 mb-6 max-w-md mx-auto">
          {#if searchQuery}
            {translate('frontend/src/routes/messages/+page.svelte::no_conversations_match_search', { values: { searchQuery } })}
          {:else}
            {t('frontend/src/routes/messages/+page.svelte::connect_message')}
          {/if}
        </p>
        {#if !searchQuery}
           <div class="flex flex-col sm:flex-row gap-3 justify-center">
            <button class="btn btn-primary gap-2" on:click={startNewChat}>
              <Plus class="w-4 h-4" />
              {t('frontend/src/routes/messages/+page.svelte::start_first_chat_button')}
            </button>
            <button class="btn btn-outline gap-2" on:click={loadConvos}>
              <RefreshCw class="w-4 h-4" />
              {t('frontend/src/routes/messages/+page.svelte::refresh_button')}
            </button>
          </div>
                 {:else}
           <div class="flex gap-2 justify-center">
             <button class="btn btn-outline" on:click={() => searchQuery = ''}>
               {t('frontend/src/routes/messages/+page.svelte::clear_search_button')}
             </button>
             <button class="btn btn-primary gap-2" on:click={startNewChat}>
               <Plus class="w-4 h-4" />
               {t('frontend/src/routes/messages/+page.svelte::start_new_chat_button')}
             </button>
           </div>
         {/if}
      </div>
    {:else}
      <div class="divide-y divide-base-300/60">
        {#each filteredConvos as convo (convo.id)}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div 
            class="group relative p-4 hover:bg-base-200/50 transition-colors cursor-pointer"
            role="button"
            tabindex="0"
            on:click={() => openChat(convo)}
            on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openChat(convo); } }}
          >
            <div class="flex items-start gap-4">
              <!-- Avatar -->
              <div class="relative flex-shrink-0">
                <div class="avatar">
                  <div class="w-12 h-12 rounded-full overflow-hidden ring-2 ring-base-300/60">
                    {#if convo.avatar}
                      <img src={convo.avatar} alt={t('frontend/src/routes/messages/+page.svelte::avatar_alt')} class="w-full h-full object-cover" />
                    {:else}
                      <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-lg font-semibold text-primary">
                        {convo.displayName.charAt(0).toUpperCase()}
                      </div>
                    {/if}
                  </div>
                </div>
                <!-- Online indicator -->
                {#if $onlineUsers.some(u => u.id === convo.other_id)}
                  <div class="absolute -bottom-1 -right-1 w-4 h-4 bg-success rounded-full border-2 border-base-100 animate-pulse"></div>
                {:else}
                  <div class="absolute -bottom-1 -right-1 w-4 h-4 bg-base-300 rounded-full border-2 border-base-100"></div>
                {/if}
              </div>

              <!-- Content -->
              <div class="flex-1 min-w-0">
                <div class="flex items-center justify-between mb-1">
                  <h3 class="font-semibold truncate">
                    {convo.displayName}
                  </h3>
                  <div class="flex items-center gap-2 text-xs text-base-content/60">
                    <span class={getStatusColor(convo.status)}>
                      {formatTime(convo.lastMessageTime)}
                    </span>
                    {#if convo.unread_count > 0}
                      <span class="badge badge-primary badge-sm">{convo.unread_count}</span>
                    {/if}
                  </div>
                </div>
                
                <div class="flex items-center gap-2 mb-1">
                  <p class="text-sm text-base-content/70 truncate flex-1">
                    {#if convo.text}
                      {convo.text}
                    {:else if convo.image}
                      {t('frontend/src/routes/messages/+page.svelte::image')}
                    {:else}
                      {t('frontend/src/routes/messages/+page.svelte::no_messages_yet')}
                    {/if}
                  </p>
                  {#if convo.unread_count > 0}
                    <div class="w-2 h-2 bg-primary rounded-full flex-shrink-0"></div>
                  {/if}
                </div>

                <!-- Additional info -->
                <div class="flex items-center gap-3 text-xs text-base-content/50">
                  {#if convo.email}
                    <span class="truncate">{convo.email}</span>
                  {/if}
                  {#if convo.lastMessageTime}
                    <span>•</span>
                    <span>{convo.status === 'today' ? t('frontend/src/routes/messages/+page.svelte::today') : formatTime(convo.lastMessageTime)}</span>
                  {/if}
                </div>
              </div>

              <!-- Action buttons (visible on hover) -->
              <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                <button 
                  class="btn btn-ghost btn-sm btn-square"
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
                    class="btn btn-ghost btn-sm btn-square"
                    on:click={(e) => e.stopPropagation()}
                  >
                    <MoreVertical class="w-4 h-4" />
                  </button>
                  <ul class="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-52 z-50">
                    <li>
                      <button class="gap-2" on:click={(e) => toggleArchive(convo, e)}>
                        <Archive class="w-4 h-4" />
                        {convo.archived ? t('frontend/src/routes/messages/+page.svelte::unarchive') : t('frontend/src/routes/messages/+page.svelte::archive')}
                      </button>
                    </li>
                    <li>
                      <button class="gap-2 text-error" on:click={(e) => deleteChat(convo, e)}>
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
    {/if}
  </div>

  
  
  <!-- Stats Footer -->
  {#if !isLoading && filteredConvos.length > 0}
    <div class="mt-4 text-center text-sm text-base-content/60">
      <p>
        {translate('frontend/src/routes/messages/+page.svelte::showing_conversations_count', { values: { filteredCount: filteredConvos.length, totalCount: convos.length } })}
        {#if searchQuery}
          {translate('frontend/src/routes/messages/+page.svelte::matching_search_query', { values: { searchQuery } })}
        {/if}
      </p>
    </div>
  {/if}
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
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div class="bg-base-100 w-full max-w-lg rounded-xl shadow-xl border border-base-300/30">
        <div class="flex items-center justify-between p-4 border-b">
          <h3 class="text-lg font-semibold">{t('frontend/src/routes/messages/+page.svelte::blocked_users_title')}</h3>
          <button class="btn btn-ghost btn-sm btn-circle" on:click={closeBlockedModal} aria-label={t('frontend/src/routes/messages/+page.svelte::close_label')}>
            <X class="w-4 h-4" />
          </button>
        </div>
        {#if blockedUsers.length === 0}
          <div class="p-4 text-base-content/60">{t('frontend/src/routes/messages/+page.svelte::no_blocked_users')}</div>
        {:else}
          <div class="max-h-80 overflow-y-auto divide-y divide-base-300/60">
            {#each blockedUsers as u (u.id)}
              <div class="p-4 flex items-center justify-between">
                <div class="flex items-center gap-3 min-w-0">
                  <div class="avatar">
                    <div class="w-10 h-10 rounded-full overflow-hidden ring-2 ring-base-300/60">
                      {#if u.avatar}
                        <img src={u.avatar} alt={t('frontend/src/routes/messages/+page.svelte::avatar_alt')} class="w-full h-full object-cover" />
                      {:else}
                        <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-sm font-semibold text-primary">
                          {(u.name ?? u.email ?? '?').charAt(0).toUpperCase()}
                        </div>
                      {/if}
                    </div>
                  </div>
                  <span class="font-medium truncate max-w-[12rem]">{u.name ?? u.email}</span>
                </div>
                <button class="btn btn-sm" on:click={() => unblock(u.id)}>{t('frontend/src/routes/messages/+page.svelte::unblock_button')}</button>
              </div>
            {/each}
          </div>
        {/if}
        <div class="p-4 border-t flex justify-end">
          <button class="btn" on:click={closeBlockedModal}>{t('frontend/src/routes/messages/+page.svelte::close_button')}</button>
        </div>
      </div>
    </div>
  {/if}

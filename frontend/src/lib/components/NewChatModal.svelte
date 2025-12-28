<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { Search, User, X, Loader2, Sparkles, ArrowRight, UserPlus } from 'lucide-svelte';
  import { t, translator } from '$lib/i18n';
  import { fade, slide, scale } from 'svelte/transition';

  const dispatch = createEventDispatcher();

  let searchQuery = '';
  let users: any[] = [];
  let isLoading = false;
  let selectedUser: any = null;
  let searchTimeout: ReturnType<typeof setTimeout> | null = null;

  let translate;
  $: translate = $translator;

  async function searchUsers(query: string) {
    if (!query.trim()) {
      users = [];
      return;
    }

    isLoading = true;
    try {
      const userList = await apiJSON(`/api/user-search?q=${encodeURIComponent(query)}`);
      users = userList.map((user: any) => ({
        ...user,
        displayName: user.name ?? user.email?.split('@')[0] ?? t('frontend/src/lib/components/NewChatModal.svelte::unknown-user')
      }));
    } catch (error) {
      console.error('Failed to search users:', error);
      users = [];
    } finally {
      isLoading = false;
    }
  }

  function handleSearchInput() {
    if (searchTimeout) {
      clearTimeout(searchTimeout);
    }

    searchTimeout = setTimeout(() => {
      searchUsers(searchQuery);
    }, 300);
  }

  $: searchQuery, handleSearchInput();

  function selectUser(user: any) {
    selectedUser = user;
  }

  function startChat() {
    if (selectedUser) {
      dispatch('startChat', { user: selectedUser });
    }
  }

  function close() {
    dispatch('close');
  }

  onDestroy(() => {
    if (searchTimeout) {
      clearTimeout(searchTimeout);
    }
  });
</script>

<div class="modal modal-open bg-black/60 backdrop-blur-md transition-all">
  <div class="modal-box w-full max-w-lg bg-base-100 rounded-[2.5rem] border border-base-200 shadow-2xl p-0 overflow-hidden" in:scale={{ duration: 300, start: 0.9 }}>
    <div class="relative overflow-hidden p-8 border-b border-base-200">
        <div class="absolute top-0 right-0 w-32 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
        <div class="relative flex items-center justify-between">
            <div class="flex items-center gap-3">
                <div class="p-2.5 bg-primary/10 rounded-xl">
                    <UserPlus class="w-6 h-6 text-primary" />
                </div>
                <h3 class="text-2xl font-black tracking-tight">{t('frontend/src/lib/components/NewChatModal.svelte::start-new-chat-heading')}</h3>
            </div>
            <button class="btn btn-ghost btn-circle bg-base-200 hover:bg-base-300 border-none transition-all" on:click={close}>
              <X class="w-5 h-5" />
            </button>
        </div>
    </div>

    <div class="p-8">
        <!-- Search -->
        <div class="relative mb-8">
          <Search class="w-5 h-5 absolute left-5 top-1/2 transform -translate-y-1/2 text-primary opacity-40" />
          <input
            class="input bg-base-200/50 border-transparent focus:border-primary/30 w-full pl-14 h-14 rounded-2xl font-bold text-base shadow-inner transition-all"
            placeholder={t('frontend/src/lib/components/NewChatModal.svelte::search-placeholder')}
            bind:value={searchQuery}
          />
        </div>

        <!-- Users List -->
        <div class="max-h-[400px] overflow-y-auto px-1 custom-scrollbar">
          {#if isLoading}
            <div class="flex flex-col items-center justify-center py-12 gap-4">
              <span class="loading loading-spinner loading-lg text-primary"></span>
              <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/components/NewChatModal.svelte::loading-users')}</span>
            </div>
          {:else if users.length === 0}
            <div class="text-center py-12 bg-base-200/30 rounded-[2rem] border-2 border-dashed border-base-200" in:fade>
              <div class="w-16 h-16 bg-base-200 rounded-full flex items-center justify-center mx-auto mb-4 opacity-30">
                  <User size={32} />
              </div>
              <p class="text-sm font-black uppercase tracking-widest opacity-30 italic">
                {searchQuery ? translate('frontend/src/lib/components/NewChatModal.svelte::no-users-found') : translate('frontend/src/lib/components/NewChatModal.svelte::start-typing-to-search')}
              </p>
            </div>
          {:else}
            <div class="grid gap-3">
              {#each users as user (user.id)}
                <div
                  class="group relative flex items-center gap-4 p-4 rounded-2xl transition-all cursor-pointer border-2 {selectedUser?.id === user.id ? 'bg-primary/5 border-primary/40 shadow-xl shadow-primary/5' : 'bg-base-100 border-transparent hover:bg-base-200/50'}"
                  on:click={() => selectUser(user)}
                >
                  <div class="avatar shadow-lg shadow-base-300/40 rounded-full">
                    <div class="w-12 h-12 rounded-xl overflow-hidden group-hover:scale-105 transition-transform duration-500 bg-base-200">
                      {#if user.avatar}
                        <img src={user.avatar} alt={t('frontend/src/lib/components/NewChatModal.svelte::user-avatar-alt')} class="w-full h-full object-cover" />
                      {:else}
                        <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-lg font-black text-primary">
                          {user.displayName.charAt(0).toUpperCase()}
                        </div>
                      {/if}
                    </div>
                  </div>
                  <div class="flex-1 min-w-0">
                    <h4 class="font-black text-lg tracking-tight truncate group-hover:text-primary transition-colors">{user.displayName}</h4>
                    {#if user.email}
                      <p class="text-[10px] font-bold uppercase tracking-widest opacity-40 truncate">{user.email}</p>
                    {/if}
                  </div>
                  {#if selectedUser?.id === user.id}
                    <div class="w-2.5 h-2.5 bg-primary rounded-full shadow-lg shadow-primary/50" in:scale></div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>
    </div>

    <!-- Actions -->
    <div class="p-8 bg-base-200/30 border-t border-base-200 flex items-center justify-end gap-3">
      <button class="btn btn-ghost rounded-xl px-6 font-black uppercase tracking-widest text-[11px]" on:click={close}>
        {t('frontend/src/lib/components/NewChatModal.svelte::cancel-button')}
      </button>
      <button
        class="btn btn-primary rounded-xl px-8 h-12 font-black uppercase tracking-widest text-[11px] gap-2 shadow-xl shadow-primary/20 disabled:grayscale disabled:opacity-30 disabled:shadow-none"
        disabled={!selectedUser}
        on:click={startChat}
      >
        {t('frontend/src/lib/components/NewChatModal.svelte::start-chat-button')}
        <ArrowRight size={16} />
      </button>
    </div>
  </div>
</div>

<style>
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
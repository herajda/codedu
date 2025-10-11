<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { Search, User, X, Loader2 } from 'lucide-svelte';
  import { t, translator } from '$lib/i18n'; // Added t and translator

  const dispatch = createEventDispatcher();

  let searchQuery = '';
  let users: any[] = [];
  let isLoading = false;
  let selectedUser: any = null;
  let searchTimeout: ReturnType<typeof setTimeout> | null = null;

  let translate; // Declare translate
  $: translate = $translator; // Assign translate reactively

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
        displayName: user.name ?? user.email?.split('@')[0] ?? t('frontend/src/lib/components/NewChatModal.svelte::unknown-user') // Translated 'Unknown User'
      }));
    } catch (error) {
      console.error('Failed to search users:', error);
      users = [];
    } finally {
      isLoading = false;
    }
  }

  function handleSearchInput() {
    // Clear previous timeout
    if (searchTimeout) {
      clearTimeout(searchTimeout);
    }

    // Set new timeout for debounced search
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

<div class="modal modal-open">
  <div class="modal-box w-full max-w-md">
    <div class="flex items-center justify-between mb-4">
      <h3 class="font-bold text-lg">{t('frontend/src/lib/components/NewChatModal.svelte::start-new-chat-heading')}</h3>
      <button class="btn btn-ghost btn-sm btn-square" on:click={close}>
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- Search -->
    <div class="relative mb-4">
      <Search class="w-4 h-4 absolute left-3 top-1/2 transform -translate-y-1/2 text-base-content/40" />
      <input
        class="input input-bordered w-full pl-10"
        placeholder={t('frontend/src/lib/components/NewChatModal.svelte::search-placeholder')}
        bind:value={searchQuery}
      />
    </div>

    <!-- Users List -->
    <div class="max-h-64 overflow-y-auto">
      {#if isLoading}
        <div class="flex items-center justify-center py-8">
          <Loader2 class="w-6 h-6 animate-spin text-primary" />
          <span class="ml-2 text-base-content/60">{t('frontend/src/lib/components/NewChatModal.svelte::loading-users')}</span>
        </div>
      {:else if users.length === 0}
        <div class="text-center py-8">
          <User class="w-12 h-12 text-base-content/30 mx-auto mb-2" />
          <p class="text-base-content/60">
            {searchQuery ? translate('frontend/src/lib/components/NewChatModal.svelte::no-users-found') : translate('frontend/src/lib/components/NewChatModal.svelte::start-typing-to-search')}
          </p>
        </div>
      {:else}
        <div class="space-y-2">
          {#each users as user (user.id)}
            <div
              class="flex items-center gap-3 p-3 rounded-lg hover:bg-base-200 cursor-pointer transition-colors {selectedUser?.id === user.id ? 'bg-primary/10 ring-2 ring-primary/20' : ''}"
              on:click={() => selectUser(user)}
            >
              <div class="avatar">
                <div class="w-10 h-10 rounded-full overflow-hidden">
                  {#if user.avatar}
                    <img src={user.avatar} alt={t('frontend/src/lib/components/NewChatModal.svelte::user-avatar-alt')} class="w-full h-full object-cover" />
                  {:else}
                    <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-sm font-semibold text-primary">
                      {user.displayName.charAt(0).toUpperCase()}
                    </div>
                  {/if}
                </div>
              </div>
              <div class="flex-1 min-w-0">
                <h4 class="font-medium truncate">{user.displayName}</h4>
                {#if user.email}
                  <p class="text-sm text-base-content/60 truncate">{user.email}</p>
                {/if}
              </div>
              {#if selectedUser?.id === user.id}
                <div class="w-2 h-2 bg-primary rounded-full"></div>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Actions -->
    <div class="modal-action">
      <button class="btn btn-ghost" on:click={close}>{t('frontend/src/lib/components/NewChatModal.svelte::cancel-button')}</button>
      <button
        class="btn btn-primary"
        disabled={!selectedUser}
        on:click={startChat}
      >
        {t('frontend/src/lib/components/NewChatModal.svelte::start-chat-button')}
      </button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop" on:click={close}>
    <button>{t('frontend/src/lib/components/NewChatModal.svelte::close-backdrop-button')}</button>
  </form>
</div>
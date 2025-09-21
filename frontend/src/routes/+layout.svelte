<script lang="ts">
  import { auth } from '$lib/auth';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import '../app.css';
  import Sidebar from '$lib/Sidebar.svelte';
  import Background from '$lib/components/Background.svelte';
  import { sidebarOpen, sidebarCollapsed } from '$lib/sidebar';
  import { apiFetch, apiJSON } from '$lib/api';
  import { sha256 } from '$lib/hash';
  import { onDestroy, tick } from 'svelte';
  import { page } from '$app/stores';
  import { createEventSource } from '$lib/sse';
  import { incrementUnreadMessages, resetUnreadMessages, setUnreadMessages } from '$lib/stores/messages';
  import { onlineUsers } from '$lib/stores/onlineUsers';
  import { browser } from '$app/environment';
  import { login as bkLogin, hasBakalari } from '$lib/bakalari';

  let settingsDialog: HTMLDialogElement;
  let passwordDialog: HTMLDialogElement;
  let bkLinkDialog: HTMLDialogElement;
  let avatarInput: HTMLInputElement;
  let name = '';
  let avatarFile: string | null = null;
  let oldPassword = '';
  let newPassword = '';
  let newPassword2 = '';
  let passwordError = '';
  let avatarChoices: string[] = [];
  let selectedAvatarFromCatalog: string | null = null;
  let linkEmail = '';
  let linkPassword = '';
  let linkPassword2 = '';
  let linkError = '';
  let bkLinkUsername = '';
  let bkLinkPassword = '';
  let bkLinkError = '';
  let linkingBakalari = false;
  let emailNotifications = true;
  let emailMessageDigest = true;

  const PUBLIC_AUTH_PREFIXES = ['/login', '/register', '/forgot-password', '/reset-password'];
  // Determine if current route is an auth-related public page
  $: isAuthPage = PUBLIC_AUTH_PREFIXES.some((prefix) => $page.url.pathname.startsWith(prefix));

  function isValidEmail(email: string | null | undefined): boolean {
    return !!email && /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
  }

  async function logout() {
    await auth.logout();
    goto('/login', { replaceState: true });
  }

  function openSettings() {
    if (user) {
      name = user.name ?? '';
    }
    avatarFile = null;
    selectedAvatarFromCatalog = null;
    linkEmail = '';
    linkPassword = '';
    linkPassword2 = '';
    linkError = '';
    bkLinkUsername = '';
    bkLinkPassword = '';
    bkLinkError = '';
    linkingBakalari = false;
    emailNotifications = user?.email_notifications ?? true;
    emailMessageDigest = user?.email_message_digest ?? true;
    // load catalog
    fetch('/api/avatars').then(r => r.ok ? r.json() : []).then((list) => { avatarChoices = list; });
    settingsDialog.showModal();
  }

  async function onAvatarChange(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) { avatarFile = null; return; }
    const reader = new FileReader();
    reader.onload = () => {
      // When a custom image is chosen, clear any catalog selection
      selectedAvatarFromCatalog = null;
      avatarFile = reader.result as string;
    };
    // Read the original file to avoid client-side downscaling artifacts
    reader.readAsDataURL(file);
  }

  function chooseAvatar() {
    avatarInput.click();
  }

  function openPasswordDialog() {
    oldPassword = '';
    newPassword = '';
    newPassword2 = '';
    passwordError = '';
    passwordDialog.showModal();
  }

  async function savePassword() {
    passwordError = '';
    if (newPassword !== newPassword2) {
      passwordError = 'Passwords do not match';
      return;
    }
    try {
      const res = await apiFetch('/api/me/password', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          old_password: await sha256(oldPassword),
          new_password: await sha256(newPassword)
        })
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        passwordError = data.error ?? res.statusText;
        return;
      }
      passwordDialog.close();
    } catch (e: any) {
      passwordError = e.message;
    }
  }

  async function saveSettings() {
    const body: any = {};
    if (selectedAvatarFromCatalog) {
      body.avatar = selectedAvatarFromCatalog;
    } else if (avatarFile !== null) {
      body.avatar = avatarFile;
    }
    if (user && user.bk_uid == null) body.name = name;
    body.email_notifications = emailNotifications;
    if (user?.role === 'student') {
      body.email_message_digest = emailMessageDigest;
    }
    const res = await apiFetch('/api/me', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    if (res.ok) {
      const meRes = await apiFetch('/api/me');
      if (meRes.ok) {
        const me = await meRes.json();
        auth.login(me.id, me.role, me.name ?? null, me.avatar ?? null, me.bk_uid ?? null, me.email ?? null, me.theme ?? null, me.email_notifications ?? true, me.email_message_digest ?? true);
      }
    }
    settingsDialog.close();
  }

  async function linkLocal() {
    linkError = '';
    if (linkPassword !== linkPassword2) {
      linkError = 'Passwords do not match';
      return;
    }
    try {
      const res = await apiFetch('/api/me/link-local', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: linkEmail, password: await sha256(linkPassword) })
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        linkError = data.error ?? res.statusText;
        return;
      }
      const meRes = await apiFetch('/api/me');
      if (meRes.ok) {
        const me = await meRes.json();
        auth.login(me.id, me.role, me.name ?? null, me.avatar ?? null, me.bk_uid ?? null, me.email ?? null, me.theme ?? null, me.email_notifications ?? true, me.email_message_digest ?? true);
      }
      settingsDialog.close();
    } catch (e: any) {
      linkError = e.message;
    }
  }

  async function linkBakalari() {
    bkLinkError = '';
    if (!hasBakalari) {
      bkLinkError = 'Bakaláři integration is not configured.';
      return;
    }
    if (!bkLinkUsername || !bkLinkPassword) {
      bkLinkError = 'Please provide your Bakaláři credentials.';
      return;
    }
    linkingBakalari = true;
    try {
      const { info } = await bkLogin(bkLinkUsername, bkLinkPassword);
      const parts = [info?.FirstName, info?.MiddleName, info?.LastName].filter(Boolean).join(' ').trim();
      const derivedName = (info?.FullName ?? info?.DisplayName ?? info?.UserName) ?? (parts.length > 0 ? parts : null);
      const res = await apiFetch('/api/me/link-bakalari', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          uid: info.UserUID,
          role: info.UserType,
          class: info.Class?.Abbrev ?? null,
          name: derivedName
        })
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        bkLinkError = data.error ?? res.statusText;
        return;
      }
      const meRes = await apiFetch('/api/me');
      if (meRes.ok) {
        const me = await meRes.json();
        auth.login(me.id, me.role, me.name ?? null, me.avatar ?? null, me.bk_uid ?? null, me.email ?? null, me.theme ?? null, me.email_notifications ?? true, me.email_message_digest ?? true);
      }
      bkLinkUsername = '';
      bkLinkPassword = '';
      settingsDialog.close();
    } catch (e: any) {
      bkLinkError = e?.message ?? 'Unable to link account';
    } finally {
      linkingBakalari = false;
    }
  }

  $: user = $auth;

  onMount(() => {
    if (!isAuthPage) {
      auth.init();
    }
  });

  async function maybeShowBakalariLinkPrompt() {
    if (!browser || !user) return;
    // Show only for Bakalari users with no valid local email
    const needsLink = user.bk_uid != null && !isValidEmail(user.email);
    if (!needsLink) return;
    const key = `bk-link-shown:${user.id}`;
    if (localStorage.getItem(key) === 'yes') return;
    // Ensure the dialog element is mounted before opening
    await tick();
    if (!bkLinkDialog) return;
    try {
      bkLinkDialog.showModal();
      // Mark as shown only after successfully opening the dialog
      localStorage.setItem(key, 'yes');
    } catch {}
  }
  
  // Initialize online users and set up periodic updates
  let presenceInterval: NodeJS.Timeout | null = null;
  $: if (user) {
    // Load online users when user logs in
    onlineUsers.loadOnlineUsers();
    
    // Set up periodic updates for user presence
    if (presenceInterval) {
      clearInterval(presenceInterval);
    }
    presenceInterval = setInterval(() => {
      onlineUsers.updateLastSeen();
      onlineUsers.loadOnlineUsers();
    }, 10000); // Update every 10 seconds
    // Check if we should prompt Bakalari users to link a local account
    maybeShowBakalariLinkPrompt();
  } else {
    // Clear interval when user logs out
    if (presenceInterval) {
      clearInterval(presenceInterval);
      presenceInterval = null;
    }
  }
  
  onDestroy(() => {
    if (presenceInterval) {
      clearInterval(presenceInterval);
    }
  });
  
  // Handle browser close/refresh to mark user as offline
  if (browser) {
    window.addEventListener('beforeunload', () => {
      if (user) {
        try {
          // Use keepalive fetch with DELETE so backend sets is_online = FALSE
          // Include credentials to ensure auth cookie is sent even on keepalive
          fetch('/api/presence', { method: 'DELETE', keepalive: true, credentials: 'include' });
        } catch {}
      }
    });
  }
  
  let unreadInitUserId: number | null = null;
  async function initUnreadCount() {
    try {
      const list: any[] = await apiJSON('/api/messages');
      const total = Array.isArray(list) ? list.reduce((sum, c) => sum + (c.unread_count || 0), 0) : 0;
      setUnreadMessages(total);
    } catch {}
  }

  $: if (!emailNotifications && emailMessageDigest) {
    emailMessageDigest = false;
  }

  $: if (user && user.id !== unreadInitUserId) {
    unreadInitUserId = user.id;
    initUnreadCount();
  }
  
  let msgES: { close: () => void } | null = null;
  $: if (user && !msgES) {
    msgES = createEventSource(
      '/api/messages/events',
      (src) => {
        src.addEventListener('message', (ev) => {
          try {
            const d = JSON.parse((ev as MessageEvent).data);
            if (d && d.recipient_id === user?.id) {
              if (!$page.url.pathname.startsWith('/messages')) {
                incrementUnreadMessages();
              }
            }
          } catch {}
        });
      },
      { onError: () => {} }
    );
  }
  
  // Clear unread indicator when viewing messages routes
  $: if ($page.url.pathname.startsWith('/messages')) {
    resetUnreadMessages();
  }

  $: if (!user && msgES) { msgES.close(); msgES = null; }
  
  let prefersDark = false;
  let media: MediaQueryList;
  function applyThemeFromPreference() {
    // Force light theme on auth pages (login/register)
    if (isAuthPage) {
      document.documentElement.setAttribute('data-theme', 'light');
      return;
    }
    document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
  }
  
  let isScrolled = false;
  function handleScroll() {
    isScrolled = typeof window !== 'undefined' && window.scrollY > 0;
  }
  onMount(() => {
    media = window.matchMedia('(prefers-color-scheme: dark)');
    prefersDark = media.matches;
    applyThemeFromPreference();
    const handler = (e: MediaQueryListEvent) => { if (!user) { prefersDark = e.matches; applyThemeFromPreference(); } };
    media.addEventListener('change', handler);
    onDestroy(() => media.removeEventListener('change', handler));
    // Initialize and watch scroll to adjust appbar opacity
    handleScroll();
    window.addEventListener('scroll', handleScroll, { passive: true });
  });
  $: if (user) { prefersDark = user.theme === 'dark'; applyThemeFromPreference(); }

  onDestroy(() => { msgES?.close(); });
  onDestroy(() => { if (typeof window !== 'undefined') window.removeEventListener('scroll', handleScroll); });

  async function toggleTheme() {
    prefersDark = !prefersDark;
    applyThemeFromPreference();
    if (user) {
      auth.login(user.id, user.role, user.name ?? null, user.avatar ?? null, user.bk_uid ?? null, user.email ?? null, prefersDark ? 'dark' : 'light', user.email_notifications ?? true, user.email_message_digest ?? true);
      try {
        await apiFetch('/api/me', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ theme: prefersDark ? 'dark' : 'light' }) });
      } catch (e) {
        // ignore
      }
    }
  }
  </script>

  <Background />
  {#if user}
    <Sidebar />
  {/if}

  <div class={`relative z-10 min-h-screen flex flex-col ${user && !$sidebarCollapsed ? 'sm:ml-64' : ''}`} class:auth-page={isAuthPage}>
    <div class="sticky top-0 z-50 px-3 py-1">
      <div class="appbar w-full h-14 px-3 flex items-center" class:appbar--scrolled={isScrolled}>
        <div class="flex items-center gap-2 min-w-0">
        {#if user}
          <button
            class="btn btn-square btn-ghost sm:hidden"
            on:click={() => sidebarOpen.update((v) => !v)}
            aria-label="Open sidebar"
            type="button"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              class="w-6 h-6"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 6h16M4 12h16M4 18h16"
              />
            </svg>
          </button>
          <!-- Desktop sidebar collapse/expand toggle -->
          <button
            class="btn btn-square btn-ghost hidden sm:inline-flex"
            on:click={() => sidebarCollapsed.update((v) => !v)}
            aria-label="Toggle sidebar"
            type="button"
          >
            <!-- Icon: Menu (hamburger) -->
            <svg
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              class="w-5 h-5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 6h16M4 12h16M4 18h16"
              />
            </svg>
          </button>
        {/if}
      </div>
      <a href={user ? '/dashboard' : '/login'} class="appbar-center min-w-0">
        <span class="logo truncate font-semibold tracking-tight text-3xl sm:text-4xl">
          <span class="logo-bracket">&lt;</span>
          <span class="logo-text">CodEdu</span>
          <span class="logo-bracket">&gt;</span>
        </span>
      </a>
      <div class="flex-1"></div>
      <div class="flex items-center gap-2 shrink-0">
        {#if user}
          <button class="btn btn-ghost" aria-label="Toggle theme" type="button" on:click={toggleTheme}>
            {#if prefersDark}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5"><path d="M21.64 13A9 9 0 1111 2.36 7 7 0 0021.64 13z"/></svg>
            {:else}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5"><path d="M12 2.25a.75.75 0 01.75.75v2.25a.75.75 0 01-1.5 0V3a.75.75 0 01.75-.75zM7.5 12a4.5 4.5 0 119 0 4.5 4.5 0 01-9 0zM18.894 6.166a.75.75 0 00-1.06-1.06l-1.591 1.59a.75.75 0 101.06 1.061l1.591-1.59zM21.75 12a.75.75 0 01-.75.75h-2.25a.75.75 0 010-1.5H21a.75.75 0 01.75.75zM17.834 18.894a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 10-1.061 1.06l1.59 1.591zM12 18a.75.75 0 01.75.75V21a.75.75 0 01-1.5 0v-2.25A.75.75 0 0112 18zM7.758 17.303a.75.75 0 00-1.061-1.06l-1.591 1.59a.75.75 0 001.06 1.061l1.591-1.59zM6 12a.75.75 0 01-.75.75H3a.75.75 0 010-1.5h2.25A.75.75 0 016 12zM6.697 7.757a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 00-1.061 1.06l1.59 1.591z"/></svg>
            {/if}
          </button>
          <div class="dropdown dropdown-end">
            <button class="btn btn-ghost btn-circle avatar" aria-haspopup="menu" type="button" tabindex="0">
              {#if user.avatar}
                <div class="w-10 rounded-full ring-1 ring-base-300/60"><img src={user.avatar} alt="User avatar" /></div>
              {:else}
                <div class="w-10 rounded-full bg-neutral text-neutral-content ring-1 ring-base-300/60 flex items-center justify-center">
                  {user.role.slice(0,1).toUpperCase()}
                </div>
              {/if}
            </button>
            <ul class="mt-3 z-[60] p-2 shadow menu menu-sm dropdown-content bg-base-100 rounded-box w-36 border border-base-300/30" role="menu" tabindex="0">
              <li><button on:click={openSettings}>Settings</button></li>
              <li><button on:click={logout}>Logout</button></li>
            </ul>
          </div>
          <dialog bind:this={settingsDialog} class="modal">
            <div class="modal-box space-y-4">
              <h3 class="font-bold text-lg">Settings</h3>
              <div class="flex items-center space-x-4">
                <button type="button" class="avatar cursor-pointer" on:click={chooseAvatar} aria-label="Choose avatar">
                  {#if avatarFile}
                    <div class="w-16 h-16 rounded-full overflow-hidden ring-1 ring-base-300/60">
                      <img src={avatarFile} alt="New avatar preview" class="w-full h-full object-cover" />
                    </div>
                  {:else if user.avatar}
                    <div class="w-16 h-16 rounded-full overflow-hidden ring-1 ring-base-300/60">
                      <img src={user.avatar} alt="Current avatar" class="w-full h-full object-cover" />
                    </div>
                  {:else}
                    <div class="w-16 h-16 rounded-full bg-neutral text-neutral-content flex items-center justify-center ring-1 ring-base-300/60">
                      {user.role.slice(0,1).toUpperCase()}
                    </div>
                  {/if}
                  <input type="file" accept="image/*" on:change={onAvatarChange} bind:this={avatarInput} class="hidden" />
                </button>
                <div class="flex-1 space-y-1">
                  {#if user.bk_uid == null}
                    <input class="input input-bordered w-full" bind:value={name} />
                  {:else}
                    <p class="font-bold">{user.name}</p>
                  {/if}
                  {#if user.email}
                    <p class="text-sm text-base-content/70">{user.email}</p>
                  {/if}
                </div>
              </div>
              {#if avatarChoices.length > 0}
              <div class="space-y-2">
                <div class="flex items-center justify-between">
                  <h4 class="font-semibold">Choose a default avatar</h4>
                  <div class="flex items-center gap-2">
                    <span class="text-sm text-base-content/60">or</span>
                    <button type="button" class="btn btn-outline btn-sm" on:click={chooseAvatar}>
                      <i class="fa-solid fa-image mr-2"></i>Upload your own
                    </button>
                  </div>
                </div>
                <div class="max-h-64 overflow-y-auto">
                  <div class="grid grid-cols-8 gap-2">
                    {#each avatarChoices as a}
                      <button type="button" class={`avatar w-12 h-12 rounded-full ring-2 ${selectedAvatarFromCatalog === a ? 'ring-primary' : 'ring-base-200'}`} on:click={() => { selectedAvatarFromCatalog = a; avatarFile = null; }}>
                        <img src={a} alt="avatar" class="w-full h-full object-cover rounded-full" />
                      </button>
                    {/each}
                  </div>
                </div>
              </div>
              {/if}
              {#if user.bk_uid == null}
                <div class="space-y-4">
                  <button class="btn" on:click={openPasswordDialog}>Change password</button>
                  {#if hasBakalari}
                    <div class="space-y-2">
                      <h4 class="font-semibold flex items-center gap-2">
                        <img src="/bakalari-logo.svg" alt="Bakaláři" class="w-6 h-6" />
                        Link with Bakaláři
                      </h4>
                      <input class="input input-bordered w-full" bind:value={bkLinkUsername} placeholder="Bakaláři username" autocomplete="username" />
                      <input type="password" class="input input-bordered w-full" bind:value={bkLinkPassword} placeholder="Bakaláři password" autocomplete="current-password" />
                      {#if bkLinkError}
                        <p class="text-error">{bkLinkError}</p>
                      {/if}
                      <button class="btn" on:click={linkBakalari} disabled={linkingBakalari} class:loading={linkingBakalari}>
                        {linkingBakalari ? 'Linking...' : 'Link Bakaláři account'}
                      </button>
                    </div>
                  {/if}
                </div>
              {:else if !isValidEmail(user.email)}
                <div class="space-y-2">
                  <h4 class="font-semibold">Create local account</h4>
                  <input type="email" class="input input-bordered w-full" bind:value={linkEmail} placeholder="Email" />
                  <input type="password" class="input input-bordered w-full" bind:value={linkPassword} placeholder="Password" />
                  <input type="password" class="input input-bordered w-full" bind:value={linkPassword2} placeholder="Repeat Password" />
                  {#if linkError}
                    <p class="text-error">{linkError}</p>
                  {/if}
                  <button class="btn" on:click={linkLocal}>Link account</button>
                </div>
              {/if}
              {#if user?.role === 'student'}
                <div class="space-y-2">
                  <div class="flex items-center justify-between gap-4">
                    <div class="space-y-1">
                      <h4 class="font-semibold">Email notifications</h4>
                      <p class="text-sm text-base-content/70">Get reminders about new assignments, upcoming deadlines, and direct messages.</p>
                    </div>
                    <input
                      type="checkbox"
                      class="toggle toggle-primary"
                      bind:checked={emailNotifications}
                      aria-label="Toggle email notifications"
                    />
                  </div>
                  <div class="flex items-center justify-between gap-4">
                    <div class="space-y-1">
                      <h4 class="font-semibold">Daily message digest</h4>
                      <p class="text-sm text-base-content/70">Receive one email per day summarising new direct messages.</p>
                    </div>
                    <input
                      type="checkbox"
                      class="toggle toggle-primary"
                      bind:checked={emailMessageDigest}
                      aria-label="Toggle message digest emails"
                      disabled={!emailNotifications}
                    />
                  </div>
                </div>
              {/if}
              <div class="modal-action">
                <button class="btn" on:click={saveSettings}>Save</button>
              </div>
            </div>
            <form method="dialog" class="modal-backdrop"><button>close</button></form>
          </dialog>
          <!-- First-time Bakaláři link prompt -->
          <dialog bind:this={bkLinkDialog} class="modal">
            <div class="modal-box space-y-4">
              <div class="flex items-center gap-3">
                <img src="/bakalari-logo.svg" alt="Bakaláři" class="w-8 h-8" />
                <h3 class="font-bold text-lg">Create a local account</h3>
              </div>
              <p class="text-base-content/80">
                You are signed in via Bakaláři. To keep access if Bakaláři is unavailable and to enable password login,
                link your account to an email and password.
              </p>
              <div class="modal-action">
                <form method="dialog">
                  <button class="btn btn-ghost" aria-label="Dismiss">Later</button>
                </form>
                <button class="btn btn-primary" on:click={() => { bkLinkDialog.close(); openSettings(); }}>
                  Link account now
                </button>
              </div>
            </div>
            <form method="dialog" class="modal-backdrop"><button>close</button></form>
          </dialog>
          <dialog bind:this={passwordDialog} class="modal">
            <div class="modal-box space-y-4">
              <h3 class="font-bold text-lg">Change Password</h3>
              <label class="form-control w-full space-y-1">
                <span class="label-text">Current Password</span>
                <input type="password" class="input input-bordered w-full" bind:value={oldPassword} />
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text">New Password</span>
                <input type="password" class="input input-bordered w-full" bind:value={newPassword} />
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text">Repeat Password</span>
                <input type="password" class="input input-bordered w-full" bind:value={newPassword2} />
              </label>
              {#if passwordError}
                <p class="text-error">{passwordError}</p>
              {/if}
              <div class="modal-action">
                <button class="btn" on:click={savePassword}>Save</button>
              </div>
            </div>
            <form method="dialog" class="modal-backdrop"><button>close</button></form>
          </dialog>
        {:else}
          <a href="/login" class="btn btn-ghost">Login</a>
          <a href="/register" class="btn btn-outline">Register</a>
        {/if}
      </div>
      </div>
    </div>

    <main class="container mx-auto flex-1 p-4 sm:p-6 gap-6">
      <slot />
    </main>

  </div>

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
  import { applyRuntimeI18n, locale as localeStore } from '$lib/i18n';
  import type { LayoutData } from './$types';

  export let data: LayoutData;

  $: if (data?.locale && data?.messages && data?.fallbackMessages) {
    applyRuntimeI18n(data.locale, data.messages, data.fallbackMessages);
  }

  let settingsDialog: HTMLDialogElement;
  let passwordDialog: HTMLDialogElement;
  let bkLinkDialog: HTMLDialogElement;
  let avatarInput: HTMLInputElement;
  let name = '';
  let avatarFile: string | null = null;
  let avatarProcessing = false;
  let avatarError = '';
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
  let editingAvatar = false;
  let editingName = false;
  let editingNotifications = false;
  let showBakalariForm = false;
  let showLocalAccountForm = false;

  $: trimmedLinkEmail = linkEmail.trim();
  $: linkHasMinLength = linkPassword.length > 8;
  $: linkHasLetter = /[A-Za-z]/.test(linkPassword);
  $: linkHasNumber = /\d/.test(linkPassword);
  $: linkMeetsPasswordRules = linkHasMinLength && linkHasLetter && linkHasNumber;
  $: linkPasswordsMatch = linkPassword2.length === 0 ? false : linkPassword === linkPassword2;
  $: canLinkLocal = isValidEmail(trimmedLinkEmail) && linkMeetsPasswordRules && linkPasswordsMatch;

  const PUBLIC_AUTH_PREFIXES = ['/login', '/register', '/forgot-password', '/reset-password', '/verify-email'];
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
    avatarProcessing = false;
    avatarError = '';
    selectedAvatarFromCatalog = null;
    linkEmail = user?.email_verified === false && user?.email ? user.email : '';
    linkPassword = '';
    linkPassword2 = '';
    linkError = '';
    bkLinkUsername = '';
    bkLinkPassword = '';
    bkLinkError = '';
    linkingBakalari = false;
    emailNotifications = user?.email_notifications ?? true;
    emailMessageDigest = user?.email_message_digest ?? true;
    editingAvatar = false;
    editingName = false;
    editingNotifications = false;
    showBakalariForm = false;
    showLocalAccountForm = false;
    if (!isValidEmail(user?.email) || user?.email_verified === false) {
      showLocalAccountForm = true;
    }
    // load catalog
    fetch('/api/avatars').then(r => r.ok ? r.json() : []).then((list) => { avatarChoices = list; });
    settingsDialog.showModal();
  }

  const MAX_AVATAR_DIMENSION = 512;

  function loadImageElement(file: File): Promise<HTMLImageElement> {
    return new Promise((resolve, reject) => {
      const url = URL.createObjectURL(file);
      const img = new Image();
      img.onload = () => {
        URL.revokeObjectURL(url);
        resolve(img);
      };
      img.onerror = (err) => {
        URL.revokeObjectURL(url);
        reject(err);
      };
      img.src = url;
    });
  }

  async function getCanvasImageSource(file: File): Promise<{ source: CanvasImageSource; width: number; height: number; cleanup: () => void; }> {
    if (browser && 'createImageBitmap' in window && typeof createImageBitmap === 'function') {
      try {
        const bitmap = await createImageBitmap(file);
        return {
          source: bitmap,
          width: bitmap.width,
          height: bitmap.height,
          cleanup: () => bitmap.close()
        };
      } catch {
        // fall through to image element loader
      }
    }
    const img = await loadImageElement(file);
    return {
      source: img,
      width: img.naturalWidth,
      height: img.naturalHeight,
      cleanup: () => {}
    };
  }

  async function compressAvatar(file: File): Promise<string> {
    if (!browser) {
      throw new Error('Cannot process image on server');
    }
    const { source, width, height, cleanup } = await getCanvasImageSource(file);
    const maxEdge = Math.max(width, height);
    const scale = maxEdge > MAX_AVATAR_DIMENSION ? MAX_AVATAR_DIMENSION / maxEdge : 1;
    const targetWidth = Math.max(1, Math.round(width * scale));
    const targetHeight = Math.max(1, Math.round(height * scale));
    const canvas = document.createElement('canvas');
    canvas.width = targetWidth;
    canvas.height = targetHeight;
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      cleanup();
      throw new Error('Canvas not supported');
    }
    ctx.drawImage(source, 0, 0, targetWidth, targetHeight);
    cleanup();
    const preferredType = file.type === 'image/png' || file.type === 'image/webp' || file.type === 'image/gif' ? 'image/png' : 'image/jpeg';
    const quality = preferredType === 'image/jpeg' ? 0.82 : undefined;
    return canvas.toDataURL(preferredType, quality);
  }

  async function onAvatarChange(e: Event) {
    avatarError = '';
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) {
      avatarFile = null;
      return;
    }
    avatarProcessing = true;
    try {
      const processed = await compressAvatar(file);
      selectedAvatarFromCatalog = null;
      avatarFile = processed;
    } catch (err) {
      console.error('Failed to process avatar', err);
      avatarFile = null;
      avatarError = 'We could not process that image. Try a different file.';
    } finally {
      avatarProcessing = false;
    }
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
    if (avatarProcessing) {
      return;
    }
    const body: any = {};
    if (selectedAvatarFromCatalog) {
      body.avatar = selectedAvatarFromCatalog;
    } else if (avatarFile !== null) {
      body.avatar = avatarFile;
    }
    if (user && user.bk_uid == null) body.name = name;
    body.email_notifications = emailNotifications;
    if (user?.role === 'student' || user?.role === 'teacher') {
      body.email_message_digest = emailMessageDigest;
    }
    const res = await apiFetch('/api/me', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    if (res.ok) {
      const meRes = await apiFetch('/api/me');
      if (meRes.ok) {
        const me = await meRes.json();
        auth.login(
          me.id,
          me.role,
          me.name ?? null,
          me.avatar ?? null,
          me.bk_uid ?? null,
          me.email ?? null,
          me.email_verified ?? null,
          me.theme ?? null,
          me.email_notifications ?? true,
          me.email_message_digest ?? true,
        );
      }
    }
    settingsDialog.close();
  }

  async function linkLocal() {
    linkError = '';
    if (!isValidEmail(trimmedLinkEmail)) {
      linkError = 'Please provide a valid email address.';
      return;
    }
    if (!linkMeetsPasswordRules) {
      linkError = 'Password must be longer than 8 characters and include letters and numbers.';
      return;
    }
    if (!linkPasswordsMatch) {
      linkError = 'Passwords must match.';
      return;
    }
    try {
      const res = await apiFetch('/api/me/link-local', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: trimmedLinkEmail, password: await sha256(linkPassword) })
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        linkError = data.error ?? res.statusText;
        return;
      }
      const meRes = await apiFetch('/api/me');
      if (meRes.ok) {
        const me = await meRes.json();
        auth.login(
          me.id,
          me.role,
          me.name ?? null,
          me.avatar ?? null,
          me.bk_uid ?? null,
          me.email ?? null,
          me.email_verified ?? null,
          me.theme ?? null,
          me.email_notifications ?? true,
          me.email_message_digest ?? true,
        );
      }
      settingsDialog.close();
      const email = typeof data.email === 'string' && data.email.length > 0 ? data.email : trimmedLinkEmail;
      if (email) {
        goto(`/verify-email?email=${encodeURIComponent(email)}&resent=1`);
      }
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
        auth.login(
          me.id,
          me.role,
          me.name ?? null,
          me.avatar ?? null,
          me.bk_uid ?? null,
          me.email ?? null,
          me.email_verified ?? null,
          me.theme ?? null,
          me.email_notifications ?? true,
          me.email_message_digest ?? true,
        );
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

  $: if (user?.role === 'student' && !emailNotifications && emailMessageDigest) {
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
      auth.login(
        user.id,
        user.role,
        user.name ?? null,
        user.avatar ?? null,
        user.bk_uid ?? null,
        user.email ?? null,
        user.email_verified ?? null,
        prefersDark ? 'dark' : 'light',
        user.email_notifications ?? true,
        user.email_message_digest ?? true,
      );
      try {
        await apiFetch('/api/me', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ theme: prefersDark ? 'dark' : 'light' }) });
      } catch (e) {
        // ignore
      }
    }
  }
</script>

<svelte:head>
  <html lang={$localeStore} />
</svelte:head>

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
            <div class="modal-box max-w-3xl p-0 overflow-hidden flex flex-col max-h-[85vh]">
              <header class="relative bg-gradient-to-r from-sky-600 via-sky-500 to-cyan-500 text-white px-6 py-6 shadow-lg sticky top-0 z-20">
                <div class="flex flex-col gap-6 sm:flex-row sm:items-center sm:justify-between">
                  <div class="flex items-center gap-4">
                    <div class="avatar">
                      <div class="w-20 h-20 rounded-full ring-4 ring-white/30 shadow-lg overflow-hidden">
                        {#if avatarFile}
                          <img src={avatarFile} alt="New avatar preview" class="w-full h-full object-cover" />
                        {:else if selectedAvatarFromCatalog}
                          <img src={selectedAvatarFromCatalog} alt="New avatar preview" class="w-full h-full object-cover" />
                        {:else if user?.avatar}
                          <img src={user.avatar} alt="Current avatar" class="w-full h-full object-cover" />
                        {:else}
                          <div class="w-full h-full flex items-center justify-center text-3xl font-semibold bg-white/20 text-white">
                            {user?.role.slice(0, 1).toUpperCase()}
                          </div>
                        {/if}
                      </div>
                    </div>
                    <div class="space-y-1">
                      <h3 class="text-2xl font-semibold tracking-tight flex items-center gap-2">
                        {user?.name ?? 'User'}
                        {#if user?.role}
                          <span class="badge badge-outline badge-sm border-white/40 text-white/80 bg-white/10 uppercase tracking-wide">
                            {user.role}
                          </span>
                        {/if}
                      </h3>
                      {#if user?.email}
                        <p class="text-sm text-white/80">{user.email}</p>
                      {/if}
                    </div>
                  </div>
                  <div class="flex flex-wrap gap-2">
                    <button
                      type="button"
                      class="btn btn-sm btn-outline border-white/40 text-white hover:border-white"
                      class:btn-active={editingAvatar}
                      on:click={() => {
                        editingAvatar = !editingAvatar;
                        if (editingAvatar && avatarChoices.length === 0) {
                          fetch('/api/avatars').then((r) => (r.ok ? r.json() : [])).then((list) => { avatarChoices = list; });
                        }
                      }}
                    >
                      {editingAvatar ? 'Hide avatar tools' : 'Change avatar'}
                    </button>
                    {#if user && user.bk_uid == null}
                      <button
                        type="button"
                        class="btn btn-sm btn-outline border-white/40 text-white hover:border-white"
                        class:btn-active={editingName}
                        on:click={() => {
                          if (editingName) {
                            name = user?.name ?? '';
                          }
                          editingName = !editingName;
                        }}
                      >
                        {editingName ? 'Cancel name edit' : 'Change name'}
                      </button>
                    {/if}
                  </div>
                </div>
              </header>

              <div class="space-y-6 bg-base-200/40 px-6 py-6 flex-1 overflow-y-auto">
                {#if editingAvatar}
                  <section class="card border border-primary/20 bg-base-100 shadow-lg">
                    <div class="card-body gap-4">
                      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                        <div>
                          <h4 class="card-title text-lg">Update avatar</h4>
                          <p class="text-sm text-base-content/70">Upload your own image or choose one from the catalog.</p>
                        </div>
                        <div class="flex flex-wrap gap-2">
                          <button type="button" class="btn btn-sm btn-outline" on:click={chooseAvatar}>
                            <i class="fa-solid fa-image mr-2"></i>Select image
                          </button>
                          <button type="button" class="btn btn-sm btn-ghost" on:click={() => {
                            avatarFile = null;
                            selectedAvatarFromCatalog = null;
                            avatarError = '';
                            if (avatarInput) {
                              avatarInput.value = '';
                            }
                          }}>
                            Reset
                          </button>
                        </div>
                      </div>
                      <div class="flex flex-col gap-4 sm:flex-row sm:items-center">
                        <div class="avatar">
                          <div class="w-24 h-24 rounded-full ring-2 ring-primary/60 shadow-inner overflow-hidden">
                            {#if avatarFile}
                              <img src={avatarFile} alt="New avatar preview" class="w-full h-full object-cover" />
                            {:else if selectedAvatarFromCatalog}
                              <img src={selectedAvatarFromCatalog} alt="New avatar preview" class="w-full h-full object-cover" />
                            {:else if user?.avatar}
                              <img src={user.avatar} alt="Current avatar" class="w-full h-full object-cover" />
                            {:else}
                              <div class="w-full h-full flex items-center justify-center text-2xl font-semibold bg-primary/20 text-primary">
                                {user?.role.slice(0, 1).toUpperCase()}
                              </div>
                            {/if}
                          </div>
                        </div>
                        <div class="text-sm text-base-content/70 space-y-2">
                          <p>Use a square image for the best result. Larger files are automatically resized.</p>
                          <p class="text-xs">Supported formats: JPG, PNG, GIF, WebP.</p>
                          {#if avatarProcessing}
                            <p class="text-xs text-primary">Processing image…</p>
                          {/if}
                          {#if avatarError}
                            <p class="text-xs text-error">{avatarError}</p>
                          {/if}
                        </div>
                        <input type="file" accept="image/*" on:change={onAvatarChange} bind:this={avatarInput} class="hidden" />
                      </div>
                      {#if avatarChoices.length > 0}
                        <div class="space-y-3">
                          <div class="flex items-center justify-between">
                            <h5 class="text-xs uppercase tracking-[0.08em] text-base-content/60">Default avatars</h5>
                            <button type="button" class="btn btn-sm btn-outline" on:click={chooseAvatar}>
                              <i class="fa-solid fa-upload mr-2"></i>Upload instead
                            </button>
                          </div>
                          <div class="grid gap-3 max-h-56 overflow-y-auto pr-1" style="grid-template-columns: repeat(auto-fit, minmax(48px, 1fr));">
                            {#each avatarChoices as a}
                              <button
                                type="button"
                                class={`avatar w-12 h-12 rounded-full ring-2 transition duration-150 ${selectedAvatarFromCatalog === a ? 'ring-primary ring-offset-2 ring-offset-base-100 scale-105' : 'ring-base-300/80 hover:ring-primary/60'}`}
                                on:click={() => { selectedAvatarFromCatalog = a; avatarFile = null; }}
                                aria-label="Select avatar"
                              >
                                <img src={a} alt="avatar" class="w-full h-full object-cover rounded-full" />
                              </button>
                            {/each}
                          </div>
                        </div>
                      {/if}
                    </div>
                  </section>
                {/if}

                <section class="card border border-base-300/60 bg-base-100 shadow-sm">
                  <div class="card-body gap-4">
                    <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                      <div>
                        <h4 class="card-title text-lg">Profile details</h4>
                        <p class="text-sm text-base-content/70">Control how your profile appears across CodEdu.</p>
                      </div>
                      {#if user && user.bk_uid == null}
                        <button
                          type="button"
                          class="btn btn-sm btn-outline"
                          class:btn-active={editingName}
                          on:click={() => {
                            if (editingName) {
                              name = user?.name ?? '';
                            }
                            editingName = !editingName;
                          }}
                        >
                          {editingName ? 'Cancel' : 'Change name'}
                        </button>
                      {/if}
                    </div>
                    {#if editingName && user && user.bk_uid == null}
                      <div class="space-y-3 max-w-md">
                        <label class="form-control w-full space-y-1">
                          <span class="label-text">Display name</span>
                          <input class="input input-bordered w-full" bind:value={name} placeholder="Enter your name" />
                        </label>
                        <p class="text-xs text-base-content/60">This is shown to your classmates and teachers.</p>
                      </div>
                    {:else}
                      <div class="grid gap-4 sm:grid-cols-2">
                        <div class="space-y-1">
                          <span class="text-xs uppercase tracking-[0.08em] text-base-content/60">Display name</span>
                          <p class="font-medium text-base-content">{user?.name ?? 'Not set'}</p>
                        </div>
                        {#if user?.email}
                          <div class="space-y-1">
                            <span class="text-xs uppercase tracking-[0.08em] text-base-content/60">Email</span>
                            <p class="font-medium text-base-content">{user.email}</p>
                          </div>
                        {/if}
                      </div>
                      {#if user && user.bk_uid != null}
                        <p class="text-xs text-base-content/60">Your name is managed by Bakaláři.</p>
                      {/if}
                    {/if}
                  </div>
                </section>

                {#if user?.role === 'student'}
                  <section class="card border border-base-300/60 bg-base-100 shadow-sm">
                    <div class="card-body gap-4">
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                          <h4 class="card-title text-lg">Email notifications</h4>
                          <p class="text-sm text-base-content/70">Choose how CodEdu keeps you in the loop.</p>
                        </div>
                        <button
                          type="button"
                          class="btn btn-sm btn-outline"
                          class:btn-active={editingNotifications}
                          on:click={() => { editingNotifications = !editingNotifications; }}
                        >
                          {editingNotifications ? 'Done' : 'Adjust'}
                        </button>
                      </div>
                      {#if editingNotifications}
                        <div class="space-y-4">
                          <div class="flex flex-col gap-2 rounded-xl border border-base-300/60 bg-base-100 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                            <div>
                              <h5 class="font-medium text-base-content">Assignment alerts</h5>
                              <p class="text-sm text-base-content/70">Receive emails for new assignments and deadlines.</p>
                            </div>
                            <input
                              type="checkbox"
                              class="toggle toggle-primary"
                              bind:checked={emailNotifications}
                              aria-label="Toggle email notifications"
                            />
                          </div>
                          <div class="flex flex-col gap-2 rounded-xl border border-base-300/60 bg-base-100 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                            <div>
                              <h5 class="font-medium text-base-content">Daily message digest</h5>
                              <p class="text-sm text-base-content/70">Get one digest email each day summarising new direct messages.</p>
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
                      {:else}
                        <div class="grid gap-4 sm:grid-cols-2">
                          <div class="space-y-1">
                            <span class="text-xs uppercase tracking-[0.08em] text-base-content/60">Alerts</span>
                            <span class={`badge ${emailNotifications ? 'badge-success badge-outline' : 'badge-neutral badge-outline'}`}>
                              {emailNotifications ? 'Enabled' : 'Disabled'}
                            </span>
                          </div>
                          <div class="space-y-1">
                            <span class="text-xs uppercase tracking-[0.08em] text-base-content/60">Daily digest</span>
                            <span class={`badge ${emailNotifications && emailMessageDigest ? 'badge-success badge-outline' : 'badge-neutral badge-outline'}`}>
                              {emailNotifications && emailMessageDigest ? 'Enabled' : 'Disabled'}
                            </span>
                          </div>
                        </div>
                      {/if}
                    </div>
                  </section>
                {:else if user?.role === 'teacher'}
                  <section class="card border border-base-300/60 bg-base-100 shadow-sm">
                    <div class="card-body gap-4">
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                          <h4 class="card-title text-lg">Email notifications</h4>
                          <p class="text-sm text-base-content/70">Get a once-a-day digest of new direct messages.</p>
                        </div>
                        <button
                          type="button"
                          class="btn btn-sm btn-outline"
                          class:btn-active={editingNotifications}
                          on:click={() => { editingNotifications = !editingNotifications; }}
                        >
                          {editingNotifications ? 'Done' : 'Adjust'}
                        </button>
                      </div>
                      {#if editingNotifications}
                        <div class="flex flex-col gap-2 rounded-xl border border-base-300/60 bg-base-100 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                          <div>
                            <h5 class="font-medium text-base-content">Daily message digest</h5>
                            <p class="text-sm text-base-content/70">Receive an email summary of incoming messages every morning.</p>
                          </div>
                          <input
                            type="checkbox"
                            class="toggle toggle-primary"
                            bind:checked={emailMessageDigest}
                            aria-label="Toggle message digest emails for teachers"
                          />
                        </div>
                      {:else}
                        <div class="space-y-1">
                          <span class="text-xs uppercase tracking-[0.08em] text-base-content/60">Daily digest</span>
                          <span class={`badge ${emailMessageDigest ? 'badge-success badge-outline' : 'badge-neutral badge-outline'}`}>
                            {emailMessageDigest ? 'Enabled' : 'Disabled'}
                          </span>
                        </div>
                      {/if}
                    </div>
                  </section>
                {/if}

                {#if user && user.bk_uid == null}
                  <section class="card border border-base-300/60 bg-base-100 shadow-sm">
                    <div class="card-body gap-4">
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                        <div>
                          <h4 class="card-title text-lg">Security</h4>
                          <p class="text-sm text-base-content/70">Keep your account protected with a strong password.</p>
                        </div>
                        <button type="button" class="btn btn-sm btn-outline" on:click={openPasswordDialog}>
                          Change password
                        </button>
                      </div>
                      <p class="text-xs text-base-content/60">Passwords are stored securely using industry-standard hashing.</p>
                    </div>
                  </section>

                  {#if hasBakalari}
                    <section class="card border border-base-300/60 bg-base-100 shadow-sm">
                      <div class="card-body gap-4">
                        <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                          <div class="flex items-center gap-3">
                            <img src="/bakalari-logo.svg" alt="Bakaláři" class="w-8 h-8" />
                            <div>
                              <h4 class="card-title text-lg">Link with Bakaláři</h4>
                              <p class="text-sm text-base-content/70">Sync your school information by connecting your Bakaláři account.</p>
                            </div>
                          </div>
                          <button
                            type="button"
                            class="btn btn-sm btn-outline"
                            class:btn-active={showBakalariForm}
                            on:click={() => { showBakalariForm = !showBakalariForm; }}
                          >
                            {showBakalariForm ? 'Cancel' : 'Link account'}
                          </button>
                        </div>
                        {#if showBakalariForm}
                          <div class="space-y-3 max-w-md">
                            <input class="input input-bordered w-full" bind:value={bkLinkUsername} placeholder="Bakaláři username" autocomplete="username" />
                            <input type="password" class="input input-bordered w-full" bind:value={bkLinkPassword} placeholder="Bakaláři password" autocomplete="current-password" />
                            {#if bkLinkError}
                              <p class="text-error text-sm">{bkLinkError}</p>
                            {/if}
                            <button class="btn btn-primary" on:click={linkBakalari} disabled={linkingBakalari} class:loading={linkingBakalari}>
                              {linkingBakalari ? 'Linking...' : 'Link Bakaláři account'}
                            </button>
                          </div>
                        {:else}
                          <p class="text-xs text-base-content/60">Currently not linked.</p>
                        {/if}
                      </div>
                    </section>
                  {/if}
                {:else if user && (!isValidEmail(user.email) || user.email_verified === false)}
                  <section class="card border border-base-300/60 bg-base-100 shadow-sm">
                    <div class="card-body gap-4">
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                          <h4 class="card-title text-lg">{user.email_verified === false && isValidEmail(user.email) ? 'Verify your email' : 'Create local account'}</h4>
                          {#if user.email_verified === false && isValidEmail(user.email)}
                            <p class="text-sm text-base-content/70">
                              We sent a verification link to <span class="font-medium">{user.email}</span>. If you made a typo, update the email below and we'll resend it.
                            </p>
                          {:else}
                            <p class="text-sm text-base-content/70">Add an email and password in case Bakaláři is unavailable.</p>
                          {/if}
                        </div>
                        <button
                          type="button"
                          class="btn btn-sm btn-outline"
                          class:btn-active={showLocalAccountForm}
                          on:click={() => { showLocalAccountForm = !showLocalAccountForm; }}
                        >
                          {showLocalAccountForm ? 'Cancel' : 'Set up'}
                        </button>
                      </div>
                      {#if showLocalAccountForm}
                        <div class="space-y-3 max-w-md">
                          <input type="email" class="input input-bordered w-full" bind:value={linkEmail} placeholder="Email" autocomplete="email" />
                          <div class="space-y-2">
                            <input type="password" class="input input-bordered w-full" bind:value={linkPassword} placeholder="Password" autocomplete="new-password" />
                            <div class="bg-base-200 rounded-lg p-3 text-sm space-y-2">
                              <p class="font-semibold text-base-content">Password requirements</p>
                              <ul class="space-y-1">
                                <li class={`flex items-center gap-2 ${linkHasMinLength ? 'text-success' : 'text-base-content/70'}`}>
                                  <span class={`inline-flex w-2 h-2 rounded-full ${linkHasMinLength ? 'bg-success' : 'bg-base-300'}`}></span>
                                  <span>At least 9 characters</span>
                                </li>
                                <li class={`flex items-center gap-2 ${linkHasLetter ? 'text-success' : 'text-base-content/70'}`}>
                                  <span class={`inline-flex w-2 h-2 rounded-full ${linkHasLetter ? 'bg-success' : 'bg-base-300'}`}></span>
                                  <span>Includes a letter</span>
                                </li>
                                <li class={`flex items-center gap-2 ${linkHasNumber ? 'text-success' : 'text-base-content/70'}`}>
                                  <span class={`inline-flex w-2 h-2 rounded-full ${linkHasNumber ? 'bg-success' : 'bg-base-300'}`}></span>
                                  <span>Includes a number</span>
                                </li>
                                <li class={`flex items-center gap-2 ${linkPassword2.length === 0 ? 'text-base-content/70' : linkPasswordsMatch ? 'text-success' : 'text-error'}`}>
                                  <span class={`inline-flex w-2 h-2 rounded-full ${linkPassword2.length === 0 ? 'bg-base-300' : linkPasswordsMatch ? 'bg-success' : 'bg-error'}`}></span>
                                  <span>Passwords match</span>
                                </li>
                              </ul>
                            </div>
                          </div>
                          <input type="password" class="input input-bordered w-full" bind:value={linkPassword2} placeholder="Repeat password" autocomplete="new-password" />
                          {#if linkError}
                            <p class="text-error text-sm">{linkError}</p>
                          {/if}
                          <button type="button" class="btn btn-primary" on:click={linkLocal} disabled={!canLinkLocal}>
                            {user.email_verified === false && isValidEmail(user.email) ? 'Update email & resend' : 'Link account'}
                          </button>
                        </div>
                      {:else}
                        {#if user.email_verified === false && isValidEmail(user.email)}
                          <p class="text-xs text-base-content/60">Verification pending for <span class="font-medium">{user.email}</span>. You can update the email if needed.</p>
                        {:else}
                          <p class="text-xs text-base-content/60">Set up a fallback login to access CodEdu without Bakaláři.</p>
                        {/if}
                      {/if}
                    </div>
                  </section>
                {/if}
              </div>

              <div class="modal-action bg-base-100 px-6 py-4 border-t border-base-300/60 justify-between sticky bottom-0 z-20">
                <button type="button" class="btn btn-ghost" on:click={() => settingsDialog.close()}>Cancel</button>
                <button class="btn btn-primary" on:click={saveSettings} disabled={avatarProcessing}>Save changes</button>
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

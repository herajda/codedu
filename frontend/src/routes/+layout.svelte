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
  import { applyRuntimeI18n, locale as localeStore, t, translator } from '$lib/i18n';
  import type { LayoutData } from './$types';

  export let data: LayoutData;

  $: if (data?.locale && data?.messages && data?.fallbackMessages) {
    applyRuntimeI18n(data.locale, data.messages, data.fallbackMessages);
  }

  let translate; // Declare translate
  $: translate = $translator; // Assign reactive translate

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
      avatarError = t('frontend/src/routes/+layout.svelte::avatar_processing_error');
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
      passwordError = t('frontend/src/routes/+layout.svelte::passwords_do_not_match');
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
      linkError = t('frontend/src/routes/+layout.svelte::provide_valid_email');
      return;
    }
    if (!linkMeetsPasswordRules) {
      linkError = t('frontend/src/routes/+layout.svelte::password_rules_not_met');
      return;
    }
    if (!linkPasswordsMatch) {
      linkError = t('frontend/src/routes/+layout.svelte::passwords_must_match');
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
      bkLinkError = t('frontend/src/routes/+layout.svelte::bakalari_not_configured');
      return;
    }
    if (!bkLinkUsername || !bkLinkPassword) {
      bkLinkError = t('frontend/src/routes/+layout.svelte::provide_bakalari_credentials');
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
      bkLinkError = e?.message ?? t('frontend/src/routes/+layout.svelte::unable_to_link_account');
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

  $: if (user && Number(user.id) !== unreadInitUserId) {
    unreadInitUserId = Number(user.id);
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
        Number(user.id),
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
  <html lang={$localeStore}></html>
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
            aria-label={t('frontend/src/routes/+layout.svelte::open_sidebar_aria')}
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
            aria-label={t('frontend/src/routes/+layout.svelte::toggle_sidebar_aria')}
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
          <button class="btn btn-ghost" aria-label={t('frontend/src/routes/+layout.svelte::toggle_theme_aria')} type="button" on:click={toggleTheme}>
            {#if prefersDark}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5"><path d="M21.64 13A9 9 0 1111 2.36 7 7 0 0021.64 13z"/></svg>
            {:else}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5"><path d="M12 2.25a.75.75 0 01.75.75v2.25a.75.75 0 01-1.5 0V3a.75.75 0 01.75-.75zM7.5 12a4.5 4.5 0 119 0 4.5 4.5 0 01-9 0zM18.894 6.166a.75.75 0 00-1.06-1.06l-1.591 1.59a.75.75 0 101.06 1.061l1.591-1.59zM21.75 12a.75.75 0 01-.75.75h-2.25a.75.75 0 010-1.5H21a.75.75 0 01.75.75zM17.834 18.894a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 10-1.061 1.06l1.59 1.591zM12 18a.75.75 0 01.75.75V21a.75.75 0 01-1.5 0v-2.25A.75.75 0 0112 18zM7.758 17.303a.75.75 0 00-1.061-1.06l-1.591 1.59a.75.75 0 001.06 1.061l1.591-1.59zM6 12a.75.75 0 01-.75.75H3a.75.75 0 010-1.5h2.25A.75.75 0 016 12zM6.697 7.757a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 00-1.061 1.06l1.59 1.591z"/></svg>
            {/if}
          </button>
          <div class="dropdown dropdown-end">
            <button class="btn btn-ghost btn-circle avatar" aria-haspopup="menu" type="button" tabindex="0">
              {#if user.avatar}
                <div class="w-10 rounded-full ring-1 ring-base-300/60"><img src={user.avatar} alt={t('frontend/src/routes/+layout.svelte::user_avatar_alt')} /></div>
              {:else}
                <div class="w-10 rounded-full bg-neutral text-neutral-content ring-1 ring-base-300/60 flex items-center justify-center">
                  {user.role.slice(0,1).toUpperCase()}
                </div>
              {/if}
            </button>
            <ul class="mt-3 z-[60] p-2 shadow menu menu-sm dropdown-content bg-base-100 rounded-box w-36 border border-base-300/30" role="menu" tabindex="0">
              <li><button on:click={openSettings}>{translate('frontend/src/routes/+layout.svelte::settings_button')}</button></li>
              <li><button on:click={logout}>{translate('frontend/src/routes/+layout.svelte::logout_button')}</button></li>
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
                          <img src={avatarFile} alt={translate('frontend/src/routes/+layout.svelte::new_avatar_preview_alt')} class="w-full h-full object-cover" />
                        {:else if selectedAvatarFromCatalog}
                          <img src={selectedAvatarFromCatalog} alt={translate('frontend/src/routes/+layout.svelte::new_avatar_preview_alt')} class="w-full h-full object-cover" />
                        {:else if user?.avatar}
                          <img src={user.avatar} alt={translate('frontend/src/routes/+layout.svelte::current_avatar_alt')} class="w-full h-full object-cover" />
                        {:else}
                          <div class="w-full h-full flex items-center justify-center text-3xl font-semibold bg-white/20 text-white">
                            {user?.role.slice(0, 1).toUpperCase()}
                          </div>
                        {/if}
                      </div>
                    </div>
                    <div class="space-y-1">
                      <h3 class="text-2xl font-semibold tracking-tight flex items-center gap-2">
                        {user?.name ?? translate('frontend/src/routes/+layout.svelte::user_fallback_name')}
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
                      {#if editingAvatar}{translate('frontend/src/routes/+layout.svelte::hide_avatar_tools_button')}{:else}{translate('frontend/src/routes/+layout.svelte::change_avatar_button')}{/if}
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
                        {#if editingName}{translate('frontend/src/routes/+layout.svelte::cancel_name_edit_button')}{:else}{translate('frontend/src/routes/+layout.svelte::change_name_button')}{/if}
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
                          <h4 class="card-title text-lg">{translate('frontend/src/routes/+layout.svelte::update_avatar_title')}</h4>
                          <p class="text-sm text-base-content/70">{translate('frontend/src/routes/+layout.svelte::update_avatar_description')}</p>
                        </div>
                        <div class="flex flex-wrap gap-2">
                          <button type="button" class="btn btn-sm btn-outline" on:click={chooseAvatar}>
                            <i class="fa-solid fa-image mr-2"></i>{translate('frontend/src/routes/+layout.2svelte::select_image_button')}
                          </button>
                          <button type="button" class="btn btn-sm btn-ghost" on:click={() => {
                            avatarFile = null;
                            selectedAvatarFromCatalog = null;
                            avatarError = '';
                            if (avatarInput) {
                              avatarInput.value = '';
                            }
                          }}>
                            {translate('frontend/src/routes/+layout.svelte::reset_button')}
                          </button>
                        </div>
                      </div>
                      <div class="flex flex-col gap-4 sm:flex-row sm:items-center">
                        <div class="avatar">
                          <div class="w-24 h-24 rounded-full ring-2 ring-primary/60 shadow-inner overflow-hidden">
                            {#if avatarFile}
                              <img src={avatarFile} alt={translate('frontend/src/routes/+layout.svelte::new_avatar_preview_alt')} class="w-full h-full object-cover" />
                            {:else if selectedAvatarFromCatalog}
                              <img src={selectedAvatarFromCatalog} alt={translate('frontend/src/routes/+layout.svelte::new_avatar_preview_alt')} class="w-full h-full object-cover" />
                            {:else if user?.avatar}
                              <img src={user.avatar} alt={translate('frontend/src/routes/+layout.svelte::current_avatar_alt')} class="w-full h-full object-cover" />
                            {:else}
                              <div class="w-full h-full flex items-center justify-center text-2xl font-semibold bg-primary/20 text-primary">
                                {user?.role.slice(0, 1).toUpperCase()}
                              </div>
                            {/if}
                          </div>
                        </div>
                        <div class="text-sm text-base-content/70 space-y-2">
                          <p>{translate('frontend/src/routes/+layout.svelte::avatar_tip_square_image')}</p>
                          <p class="text-xs">{translate('frontend/src/routes/+layout.svelte::avatar_supported_formats')}</p>
                          {#if avatarProcessing}
                            <p class="text-xs text-primary">{translate('frontend/src/routes/+layout.svelte::processing_image')}</p>
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
                            <h5 class="text-xs uppercase tracking-[0.08em] text-base-content/60">{translate('frontend/src/routes/+layout.svelte::default_avatars_title')}</h5>
                            <button type="button" class="btn btn-sm btn-outline" on:click={chooseAvatar}>
                              <i class="fa-solid fa-upload mr-2"></i>{translate('frontend/src/routes/+layout.svelte::upload_instead_button')}
                            </button>
                          </div>
                          <div class="grid gap-3 max-h-56 overflow-y-auto pr-1" style="grid-template-columns: repeat(auto-fit, minmax(48px, 1fr));">
                            {#each avatarChoices as a}
                              <button
                                type="button"
                                class={`avatar w-12 h-12 rounded-full ring-2 transition duration-150 ${selectedAvatarFromCatalog === a ? 'ring-primary ring-offset-2 ring-offset-base-100 scale-105' : 'ring-base-300/80 hover:ring-primary/60'}`}
                                on:click={() => { selectedAvatarFromCatalog = a; avatarFile = null; }}
                                aria-label={translate('frontend/src/routes/+layout.svelte::select_avatar_aria')}
                              >
                                <img src={a} alt={translate('frontend/src/routes/+layout.svelte::avatar_image_alt')} class="w-full h-full object-cover rounded-full" />
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
                        <h4 class="card-title text-lg">{translate('frontend/src/routes/+layout.svelte::profile_details_title')}</h4>
                        <p class="text-sm text-base-content/70">{translate('frontend/src/routes/+layout.svelte::profile_details_description')}</p>
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
                          {#if editingName}{translate('frontend/src/routes/+layout.svelte::cancel_button')}{:else}{translate('frontend/src/routes/+layout.svelte::change_name_button_short')}{/if}
                        </button>
                      {/if}
                    </div>
                    {#if editingName && user && user.bk_uid == null}
                      <div class="space-y-3 max-w-md">
                        <label class="form-control w-full space-y-1">
                          <span class="label-text">{translate('frontend/src/routes/+layout.svelte::display_name_label')}</span>
                          <input class="input input-bordered w-full" bind:value={name} placeholder={translate('frontend/src/routes/+layout.svelte::enter_your_name_placeholder')} />
                        </label>
                        <p class="text-xs text-base-content/60">{translate('frontend/src/routes/+layout.svelte::display_name_tip')}</p>
                      </div>
                    {:else}
                      <div class="grid gap-4 sm:grid-cols-2">
                        <div class="space-y-1">
                          <span class="text-xs uppercase tracking-[0.08em] text-base-content/60">{translate('frontend/src/routes/+layout.svelte::display_name_label')}</span>
                          <p class="font-medium text-base-content">{user?.name ?? translate('frontend/src/routes/+layout.svelte::not_set_status')}</p>
                        </div>
                        {#if user?.email}
                          <div class="space-y-1">
                            <span class="text-xs uppercase tracking-[0.08em] text-base-content/60">{translate('frontend/src/routes/+layout.svelte::email_label')}</span>
                            <p class="font-medium text-base-content">{user.email}</p>
                          </div>
                        {/if}
                      </div>
                      {#if user && user.bk_uid != null}
                        <p class="text-xs text-base-content/60">{translate('frontend/src/routes/+layout.svelte::bakalari_managed_name_tip')}</p>
                      {/if}
                    {/if}
                  </div>
                </section>

                {#if user?.role === 'student'}
                  <section class="card border border-base-300/60 bg-base-100 shadow-sm">
                    <div class="card-body gap-4">
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                          <h4 class="card-title text-lg">{translate('frontend/src/routes/+layout.svelte::email_notifications_title')}</h4>
                          <p class="text-sm text-base-content/70">{translate('frontend/src/routes/+layout.svelte::email_notifications_description')}</p>
                        </div>
                        <button
                          type="button"
                          class="btn btn-sm btn-outline"
                          class:btn-active={editingNotifications}
                          on:click={() => { editingNotifications = !editingNotifications; }}
                        >
                          {#if editingNotifications}{translate('frontend/src/routes/+layout.svelte::done_button')}{:else}{translate('frontend/src/routes/+layout.svelte::adjust_button')}{/if}
                        </button>
                      </div>
                      {#if editingNotifications}
                        <div class="space-y-4">
                          <div class="flex flex-col gap-2 rounded-xl border border-base-300/60 bg-base-100 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                            <div>
                              <h5 class="font-medium text-base-content">{translate('frontend/src/routes/+layout.svelte::assignment_alerts_title')}</h5>
                              <p class="text-sm text-base-content/70">{translate('frontend/src/routes/+layout.svelte::assignment_alerts_description')}</p>
                            </div>
                            <input
                              type="checkbox"
                              class="toggle toggle-primary"
                              bind:checked={emailNotifications}
                              aria-label={translate('frontend/src/routes/+layout.svelte::toggle_email_notifications_aria')}
                            />
                          </div>
                          <div class="flex flex-col gap-2 rounded-xl border border-base-300/60 bg-base-100 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                            <div>
                              <h5 class="font-medium text-base-content">{translate('frontend/src/routes/+layout.svelte::daily_message_digest_title')}</h5>
                              <p class="text-sm text-base-content/70">{translate('frontend/src/routes/+layout.svelte::daily_message_digest_description')}</p>
                            </div>
                            <input
                              type="checkbox"
                              class="toggle toggle-primary"
                              bind:checked={emailMessageDigest}
                              aria-label={translate('frontend/src/routes/+layout.svelte::toggle_message_digest_aria')}
                              disabled={!emailNotifications}
                            />
                          </div>
                        </div>
                      {:else}
                        <div class="grid gap-4 sm:grid-cols-2">
                          <div class="space-y-1">
                            <span class="text-xs uppercase tracking-[0.08em] text-base-content/60">{translate('frontend/src/routes/+layout.svelte::alerts_label')}</span>
                            <span class={`badge ${emailNotifications ? 'badge-success badge-outline' : 'badge-neutral badge-outline'}`}>
                              {emailNotifications ? translate('frontend/src/routes/+layout.svelte::enabled_status') : translate('frontend/src/routes/+layout.svelte::disabled_status')}
                            </span>
                          </div>
                          <div class="space-y-1">
                            <span class="text-xs uppercase tracking-[0.08em] text-base-content/60">{translate('frontend/src/routes/+layout.svelte::daily_digest_label')}</span>
                            <span class={`badge ${emailNotifications && emailMessageDigest ? 'badge-success badge-outline' : 'badge-neutral badge-outline'}`}>
                              {emailNotifications && emailMessageDigest ? translate('frontend/src/routes/+layout.svelte::enabled_status') : translate('frontend/src/routes/+layout.svelte::disabled_status')}
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
                          <h4 class="card-title text-lg">{translate('frontend/src/routes/+layout.svelte::email_notifications_title')}</h4>
                          <p class="text-sm text-base-content/70">{translate('frontend/src/routes/+layout.svelte::teacher_email_notifications_description')}</p>
                        </div>
                        <button
                          type="button"
                          class="btn btn-sm btn-outline"
                          class:btn-active={editingNotifications}
                          on:click={() => { editingNotifications = !editingNotifications; }}
                        >
                          {#if editingNotifications}{translate('frontend/src/routes/+layout.svelte::done_button')}{:else}{translate('frontend/src/routes/+layout.svelte::adjust_button')}{/if}
                        </button>
                      </div>
                      {#if editingNotifications}
                        <div class="flex flex-col gap-2 rounded-xl border border-base-300/60 bg-base-100 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                          <div>
                            <h5 class="font-medium text-base-content">{translate('frontend/src/routes/+layout.svelte::daily_message_digest_title')}</h5>
                            <p class="text-sm text-base-content/70">{translate('frontend/src/routes/+layout.svelte::teacher_daily_message_digest_description')}</p>
                          </div>
                          <input
                            type="checkbox"
                            class="toggle toggle-primary"
                            bind:checked={emailMessageDigest}
                            aria-label={translate('frontend/src/routes/+layout.svelte::toggle_message_digest_teachers_aria')}
                          />
                        </div>
                      {:else}
                        <div class="space-y-1">
                          <span class="text-xs uppercase tracking-[0.08em] text-base-content/60">{translate('frontend/src/routes/+layout.svelte::daily_digest_label')}</span>
                          <span class={`badge ${emailMessageDigest ? 'badge-success badge-outline' : 'badge-neutral badge-outline'}`}>
                            {emailMessageDigest ? translate('frontend/src/routes/+layout.svelte::enabled_status') : translate('frontend/src/routes/+layout.svelte::disabled_status')}
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
                          <h4 class="card-title text-lg">{translate('frontend/src/routes/+layout.svelte::security_title')}</h4>
                          <p class="text-sm text-base-content/70">{translate('frontend/src/routes/+layout.svelte::security_description')}</p>
                        </div>
                        <button type="button" class="btn btn-sm btn-outline" on:click={openPasswordDialog}>
                          {translate('frontend/src/routes/+layout.svelte::change_password_button')}
                        </button>
                      </div>
                      <p class="text-xs text-base-content/60">{translate('frontend/src/routes/+layout.svelte::password_security_tip')}</p>
                    </div>
                  </section>

                  {#if hasBakalari}
                    <section class="card border border-base-300/60 bg-base-100 shadow-sm">
                      <div class="card-body gap-4">
                        <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                          <div class="flex items-center gap-3">
                            <img src="/bakalari-logo.svg" alt="Bakaláři" class="w-8 h-8" />
                            <div>
                              <h4 class="card-title text-lg">{translate('frontend/src/routes/+layout.svelte::link_bakalari_title')}</h4>
                              <p class="text-sm text-base-content/70">{translate('frontend/src/routes/+layout.svelte::link_bakalari_description')}</p>
                            </div>
                          </div>
                          <button
                            type="button"
                            class="btn btn-sm btn-outline"
                            class:btn-active={showBakalariForm}
                            on:click={() => { showBakalariForm = !showBakalariForm; }}
                          >
                            {#if showBakalariForm}{translate('frontend/src/routes/+layout.svelte::cancel_button')}{:else}{translate('frontend/src/routes/+layout.svelte::link_account_button')}{/if}
                          </button>
                        </div>
                        {#if showBakalariForm}
                          <div class="space-y-3 max-w-md">
                            <input class="input input-bordered w-full" bind:value={bkLinkUsername} placeholder={translate('frontend/src/routes/+layout.svelte::bakalari_username_placeholder')} autocomplete="username" />
                            <input type="password" class="input input-bordered w-full" bind:value={bkLinkPassword} placeholder={translate('frontend/src/routes/+layout.svelte::bakalari_password_placeholder')} autocomplete="current-password" />
                            {#if bkLinkError}
                              <p class="text-error text-sm">{bkLinkError}</p>
                            {/if}
                            <button class="btn btn-primary" on:click={linkBakalari} disabled={linkingBakalari} class:loading={linkingBakalari}>
                              {#if linkingBakalari}{translate('frontend/src/routes/+layout.svelte::linking_bakalari_button')}{:else}{translate('frontend/src/routes/+layout.svelte::link_bakalari_account_button')}{/if}
                            </button>
                          </div>
                        {:else}
                          <p class="text-xs text-base-content/60">{translate('frontend/src/routes/+layout.svelte::not_linked_status')}</p>
                        {/if}
                      </div>
                    </section>
                  {/if}
                {:else if user && (!isValidEmail(user.email) || user.email_verified === false)}
                  <section class="card border border-base-300/60 bg-base-100 shadow-sm">
                    <div class="card-body gap-4">
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                          <h4 class="card-title text-lg">{user.email_verified === false && isValidEmail(user.email) ? translate('frontend/src/routes/+layout.svelte::verify_your_email_title') : translate('frontend/src/routes/+layout.svelte::create_local_account_title')}</h4>
                          {#if user.email_verified === false && isValidEmail(user.email)}
                            <p class="text-sm text-base-content/70">
                              {translate('frontend/src/routes/+layout.svelte::verification_link_sent_to')} <span class="font-medium">{user.email}</span>. {translate('frontend/src/routes/+layout.svelte::typo_email_update_resend_tip')}
                            </p>
                          {:else}
                            <p class="text-sm text-base-content/70">{translate('frontend/src/routes/+layout.svelte::add_email_password_fallback_tip')}</p>
                          {/if}
                        </div>
                        <button
                          type="button"
                          class="btn btn-sm btn-outline"
                          class:btn-active={showLocalAccountForm}
                          on:click={() => { showLocalAccountForm = !showLocalAccountForm; }}
                        >
                          {#if showLocalAccountForm}{translate('frontend/src/routes/+layout.svelte::cancel_button')}{:else}{translate('frontend/src/routes/+layout.svelte::set_up_button')}{/if}
                        </button>
                      </div>
                      {#if showLocalAccountForm}
                        <div class="space-y-3 max-w-md">
                          <input type="email" class="input input-bordered w-full" bind:value={linkEmail} placeholder={translate('frontend/src/routes/+layout.svelte::email_placeholder')} autocomplete="email" />
                          <div class="space-y-2">
                            <input type="password" class="input input-bordered w-full" bind:value={linkPassword} placeholder={translate('frontend/src/routes/+layout.svelte::password_placeholder')} autocomplete="new-password" />
                            <div class="bg-base-200 rounded-lg p-3 text-sm space-y-2">
                              <p class="font-semibold text-base-content">{translate('frontend/src/routes/+layout.svelte::password_requirements_title')}</p>
                              <ul class="space-y-1">
                                <li class={`flex items-center gap-2 ${linkHasMinLength ? 'text-success' : 'text-base-content/70'}`}>
                                  <span class={`inline-flex w-2 h-2 rounded-full ${linkHasMinLength ? 'bg-success' : 'bg-base-300'}`}></span>
                                  <span>{translate('frontend/src/routes/+layout.svelte::password_min_length_requirement')}</span>
                                </li>
                                <li class={`flex items-center gap-2 ${linkHasLetter ? 'text-success' : 'text-base-content/70'}`}>
                                  <span class={`inline-flex w-2 h-2 rounded-full ${linkHasLetter ? 'bg-success' : 'bg-base-300'}`}></span>
                                  <span>{translate('frontend/src/routes/+layout.svelte::password_includes_letter_requirement')}</span>
                                </li>
                                <li class={`flex items-center gap-2 ${linkHasNumber ? 'text-success' : 'text-base-content/70'}`}>
                                  <span class={`inline-flex w-2 h-2 rounded-full ${linkHasNumber ? 'bg-success' : 'bg-base-300'}`}></span>
                                  <span>{translate('frontend/src/routes/+layout.svelte::password_includes_number_requirement')}</span>
                                </li>
                                <li class={`flex items-center gap-2 ${linkPassword2.length === 0 ? 'text-base-content/70' : linkPasswordsMatch ? 'text-success' : 'text-error'}`}>
                                  <span class={`inline-flex w-2 h-2 rounded-full ${linkPassword2.length === 0 ? 'bg-base-300' : linkPasswordsMatch ? 'bg-success' : 'bg-error'}`}></span>
                                  <span>{translate('frontend/src/routes/+layout.svelte::passwords_match_requirement')}</span>
                                </li>
                              </ul>
                            </div>
                          </div>
                          <input type="password" class="input input-bordered w-full" bind:value={linkPassword2} placeholder={translate('frontend/src/routes/+layout.svelte::repeat_password_placeholder')} autocomplete="new-password" />
                          {#if linkError}
                            <p class="text-error text-sm">{linkError}</p>
                          {/if}
                          <button type="button" class="btn btn-primary" on:click={linkLocal} disabled={!canLinkLocal}>
                            {user.email_verified === false && isValidEmail(user.email) ? translate('frontend/src/routes/+layout.svelte::update_email_resend_button') : translate('frontend/src/routes/+layout.svelte::link_account_button')}
                          </button>
                        </div>
                      {:else}
                        {#if user.email_verified === false && isValidEmail(user.email)}
                          <p class="text-xs text-base-content/60">{translate('frontend/src/routes/+layout.svelte::verification_pending_email_tip', { values: { email: user.email } })}</p>
                        {:else}
                          <p class="text-xs text-base-content/60">{translate('frontend/src/routes/+layout.svelte::set_up_fallback_login_tip')}</p>
                        {/if}
                      {/if}
                    </div>
                  </section>
                {/if}
              </div>

              <div class="modal-action bg-base-100 px-6 py-4 border-t border-base-300/60 justify-between sticky bottom-0 z-20">
                <button type="button" class="btn btn-ghost" on:click={() => settingsDialog.close()}>{translate('frontend/src/routes/+layout.svelte::cancel_button')}</button>
                <button class="btn btn-primary" on:click={saveSettings} disabled={avatarProcessing}>{translate('frontend/src/routes/+layout.svelte::save_changes_button')}</button>
              </div>
            </div>
            <form method="dialog" class="modal-backdrop"><button>{translate('frontend/src/routes/+layout.svelte::close_dialog_button')}</button></form>
          </dialog>
          <!-- First-time Bakaláři link prompt -->
          <dialog bind:this={bkLinkDialog} class="modal">
            <div class="modal-box space-y-4">
              <div class="flex items-center gap-3">
                <img src="/bakalari-logo.svg" alt="Bakaláři" class="w-8 h-8" />
                <h3 class="font-bold text-lg">{translate('frontend/src/routes/+layout.svelte::create_local_account_prompt_title')}</h3>
              </div>
              <p class="text-base-content/80">
                {translate('frontend/src/routes/+layout.svelte::bakalari_link_prompt_description')}
              </p>
              <div class="modal-action">
                <form method="dialog">
                  <button class="btn btn-ghost" aria-label={translate('frontend/src/routes/+layout.svelte::dismiss_prompt_aria')}>{translate('frontend/src/routes/+layout.svelte::later_button')}</button>
                </form>
                <button class="btn btn-primary" on:click={() => { bkLinkDialog.close(); openSettings(); }}>
                  {translate('frontend/src/routes/+layout.svelte::link_account_now_button')}
                </button>
              </div>
            </div>
            <form method="dialog" class="modal-backdrop"><button>{translate('frontend/src/routes/+layout.svelte::close_dialog_button')}</button></form>
          </dialog>
          <dialog bind:this={passwordDialog} class="modal">
            <div class="modal-box space-y-4">
              <h3 class="font-bold text-lg">{translate('frontend/src/routes/+layout.svelte::change_password_dialog_title')}</h3>
              <label class="form-control w-full space-y-1">
                <span class="label-text">{translate('frontend/src/routes/+layout.svelte::current_password_label')}</span>
                <input type="password" class="input input-bordered w-full" bind:value={oldPassword} />
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text">{translate('frontend/src/routes/+layout.svelte::new_password_label')}</span>
                <input type="password" class="input input-bordered w-full" bind:value={newPassword} />
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text">{translate('frontend/src/routes/+layout.svelte::repeat_password_label')}</span>
                <input type="password" class="input input-bordered w-full" bind:value={newPassword2} />
              </label>
              {#if passwordError}
                <p class="text-error">{passwordError}</p>
              {/if}
              <div class="modal-action">
                <button class="btn" on:click={savePassword}>{translate('frontend/src/routes/+layout.svelte::save_button')}</button>
              </div>
            </div>
            <form method="dialog" class="modal-backdrop"><button>{translate('frontend/src/routes/+layout.svelte::close_dialog_button')}</button></form>
          </dialog>
        {:else}
          <a href="/login" class="btn btn-ghost">{t('frontend/src/routes/+layout.svelte::login_button')}</a>
          <a href="/register" class="btn btn-outline">{t('frontend/src/routes/+layout.svelte::register_button')}</a>
        {/if}
      </div>
      </div>
    </div>

    <main class="container mx-auto flex-1 p-4 sm:p-6 gap-6">
      <slot />
    </main>

  </div>
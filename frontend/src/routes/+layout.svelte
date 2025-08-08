<script lang="ts">
  import { auth } from '$lib/auth';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import '../app.css';
  import Sidebar from '$lib/Sidebar.svelte';
  import Background from '$lib/components/Background.svelte';
  import { sidebarOpen, sidebarCollapsed } from '$lib/sidebar';
  import { apiFetch } from '$lib/api';
  import { sha256 } from '$lib/hash';
import { initKey } from '$lib/e2ee';
import { compressImage } from '$lib/utils/compressImage';
  import { onDestroy } from 'svelte';

  let settingsDialog: HTMLDialogElement;
  let passwordDialog: HTMLDialogElement;
  let avatarInput: HTMLInputElement;
  let name = '';
  let avatarFile: string | null = null;
  let oldPassword = '';
  let newPassword = '';
  let newPassword2 = '';
  let passwordError = '';

  function logout() {
    auth.logout();
    goto('/login');
  }

  function openSettings() {
    if (user) {
      name = user.name ?? '';
    }
    avatarFile = null;
    settingsDialog.showModal();
  }

  async function onAvatarChange(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) { avatarFile = null; return; }
    const compressed = await compressImage(file, 512, 0.8);
    const reader = new FileReader();
    reader.onload = () => { avatarFile = reader.result as string; };
    reader.readAsDataURL(compressed);
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
    if (avatarFile !== null) body.avatar = avatarFile;
    if (user && user.bk_uid == null) body.name = name;
    const res = await apiFetch('/api/me', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    if (res.ok) {
      const meRes = await apiFetch('/api/me');
      if (meRes.ok) {
        const me = await meRes.json();
        auth.login(me.id, me.role, me.name ?? null, me.avatar ?? null, me.bk_uid ?? null, me.email ?? null);
      }
    }
    settingsDialog.close();
  }

  $: user = $auth;

  onMount(() => {
    auth.init();
    initKey();
  });
  
  let prefersDark = false;
  let media: MediaQueryList;
  function applyThemeFromPreference() {
    document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
  }
  onMount(() => {
    media = window.matchMedia('(prefers-color-scheme: dark)');
    prefersDark = media.matches;
    applyThemeFromPreference();
    const handler = (e: MediaQueryListEvent) => { prefersDark = e.matches; applyThemeFromPreference(); };
    media.addEventListener('change', handler);
    onDestroy(() => media.removeEventListener('change', handler));
  });
</script>

  <Background />
  {#if user}
    <Sidebar />
  {/if}

  <div class={`relative z-10 min-h-screen flex flex-col ${user && !$sidebarCollapsed ? 'sm:ml-60' : ''}`}>
    <div class="sticky top-0 z-50 px-3 py-2">
      <div class="appbar container mx-auto h-12 px-3 flex items-center justify-between">
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
        <a href="/dashboard" class="appbar-title flex items-center gap-2 min-w-0">
          <span class="brand-dot"></span>
          <span class="truncate font-semibold tracking-tight">CodeGrader</span>
        </a>
      </div>
      <div class="flex items-center gap-2">
        {#if user}
          <button class="btn btn-ghost" aria-label="Toggle theme" type="button" on:click={() => { prefersDark = !prefersDark; applyThemeFromPreference(); }}>
            {#if prefersDark}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5"><path d="M21.64 13A9 9 0 1111 2.36 7 7 0 0021.64 13z"/></svg>
            {:else}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5"><path d="M12 2.25a.75.75 0 01.75.75v2.25a.75.75 0 01-1.5 0V3a.75.75 0 01.75-.75zM7.5 12a4.5 4.5 0 119 0 4.5 4.5 0 01-9 0zM18.894 6.166a.75.75 0 00-1.06-1.06l-1.591 1.59a.75.75 0 101.06 1.061l1.591-1.59zM21.75 12a.75.75 0 01-.75.75h-2.25a.75.75 0 010-1.5H21a.75.75 0 01.75.75zM17.834 18.894a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 10-1.061 1.06l1.59 1.591zM12 18a.75.75 0 01.75.75V21a.75.75 0 01-1.5 0v-2.25A.75.75 0 0112 18zM7.758 17.303a.75.75 0 00-1.061-1.06l-1.591 1.59a.75.75 0 001.06 1.061l1.591-1.59zM6 12a.75.75 0 01-.75.75H3a.75.75 0 010-1.5h2.25A.75.75 0 016 12zM6.697 7.757a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 00-1.061 1.06l1.59 1.591z"/></svg>
            {/if}
          </button>
          <div class="dropdown dropdown-end">
            <button class="btn btn-ghost btn-circle avatar" aria-haspopup="menu" type="button">
              {#if user.avatar}
                <div class="w-10 rounded-full ring-1 ring-base-300/60"><img src={user.avatar} alt="User avatar" /></div>
              {:else}
                <div class="w-10 rounded-full bg-neutral text-neutral-content ring-1 ring-base-300/60 flex items-center justify-center">
                  {user.role.slice(0,1).toUpperCase()}
                </div>
              {/if}
            </button>
            <ul class="mt-3 z-[1] p-2 shadow menu menu-sm dropdown-content bg-base-100 rounded-box w-36" role="menu">
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
                    <div class="w-16 rounded-full"><img src={avatarFile} alt="New avatar preview" /></div>
                  {:else if user.avatar}
                    <div class="w-16 rounded-full"><img src={user.avatar} alt="Current avatar" /></div>
                  {:else}
                    <div class="w-16 rounded-full bg-neutral text-neutral-content flex items-center justify-center">
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
              {#if user.bk_uid == null}
                <button class="btn" on:click={openPasswordDialog}>Change password</button>
              {/if}
              <div class="modal-action">
                <button class="btn" on:click={saveSettings}>Save</button>
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

    <main class="container mx-auto flex-1 p-6 gap-6">
      <slot />
    </main>

  </div>


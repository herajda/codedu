<script lang="ts">
  import { auth } from '$lib/auth';
  import { goto } from '$app/navigation';
import { onMount } from 'svelte';
import AvatarCropper from '$lib/AvatarCropper.svelte';
  import '../app.css';
  import Sidebar from '$lib/Sidebar.svelte';
  import { sidebarOpen, sidebarCollapsed } from '$lib/sidebar';
  import { apiFetch } from '$lib/api';
  import { sha256 } from '$lib/hash';

  let settingsDialog: HTMLDialogElement;
  let passwordDialog: HTMLDialogElement;
  let avatarInput: HTMLInputElement;
  let cropDialog: HTMLDialogElement;
  let cropperComp: any;
  let cropSrc = '';
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

  function onAvatarChange(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) { return; }
    const reader = new FileReader();
    reader.onload = () => {
      cropSrc = reader.result as string;
      cropDialog.showModal();
    };
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

  function applyCrop() {
    if (cropperComp) {
      avatarFile = cropperComp.getDataURL();
    }
    cropDialog.close();
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
  });
</script>

  {#if user}
    <Sidebar />
  {/if}

  <div class={`min-h-screen flex flex-col ${user && !$sidebarCollapsed ? 'sm:ml-60' : ''}`}>
    <div class="navbar bg-base-200 shadow sticky top-0 z-50">
      <div class="flex-1">
        {#if user}
          <button
            class="btn btn-square btn-ghost mr-2 sm:hidden"
            on:click={() => sidebarOpen.update((v) => !v)}
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
        {/if}
        <a href="/dashboard" class="btn btn-ghost text-xl">CodeGrader</a>
      </div>
      <div class="flex-none gap-2">
        {#if user}
          <div class="dropdown dropdown-end">
            <label tabindex="0" class="btn btn-ghost btn-circle avatar">
              {#if user.avatar}
                <div class="w-10 rounded-full"><img src={user.avatar} /></div>
              {:else}
                <div class="w-10 rounded-full bg-neutral text-neutral-content flex items-center justify-center">
                  {user.role.slice(0,1).toUpperCase()}
                </div>
              {/if}
            </label>
            <ul tabindex="0" class="mt-3 z-[1] p-2 shadow menu menu-sm dropdown-content bg-base-100 rounded-box w-36">
              <li><button on:click={openSettings}>Settings</button></li>
              <li><button on:click={logout}>Logout</button></li>
            </ul>
          </div>
          <dialog bind:this={settingsDialog} class="modal">
            <div class="modal-box space-y-4">
              <h3 class="font-bold text-lg">Settings</h3>
              <div class="flex items-center space-x-4">
                <div class="avatar cursor-pointer" on:click={chooseAvatar}>
                  {#if avatarFile}
                    <div class="w-16 rounded-full"><img src={avatarFile} /></div>
                  {:else if user.avatar}
                    <div class="w-16 rounded-full"><img src={user.avatar} /></div>
                  {:else}
                    <div class="w-16 rounded-full bg-neutral text-neutral-content flex items-center justify-center">
                      {user.role.slice(0,1).toUpperCase()}
                    </div>
                  {/if}
                  <input type="file" accept="image/*" on:change={onAvatarChange} bind:this={avatarInput} class="hidden" />
                </div>
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
          <dialog bind:this={cropDialog} class="modal">
            <div class="modal-box space-y-4">
              <h3 class="font-bold text-lg">Crop Avatar</h3>
              {#if cropSrc}
                <AvatarCropper src={cropSrc} bind:this={cropperComp} />
              {/if}
              <div class="modal-action">
                <button class="btn" on:click={applyCrop}>Use</button>
              </div>
            </div>
            <form method="dialog" class="modal-backdrop"><button>close</button></form>
          </dialog>
        {:else}
          <a href="/login" class="btn">Login</a>
          <a href="/register" class="btn btn-outline">Register</a>
        {/if}
      </div>
    </div>

    <main class="container mx-auto flex-1 p-4">
      <slot />
    </main>

  </div>


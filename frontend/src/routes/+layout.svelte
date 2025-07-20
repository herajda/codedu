<script lang="ts">
  import { auth } from '$lib/auth';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import '../app.css';
  import Sidebar from '$lib/Sidebar.svelte';
  import { sidebarOpen, sidebarCollapsed } from '$lib/sidebar';

  function logout() {
    auth.logout();
    goto('/login');
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
            class="btn btn-square btn-ghost mr-2 hidden sm:inline-flex"
            on:click={() => sidebarCollapsed.update(v => !v)}
            aria-label="Toggle sidebar"
          >
            {#if $sidebarCollapsed}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-6 h-6">
                <path d="M9.75 5.25L16.5 12l-6.75 6.75" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            {:else}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-6 h-6">
                <path d="M14.25 5.25L7.5 12l6.75 6.75" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            {/if}
          </button>
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
          <details class="dropdown dropdown-end">
            <summary class="btn" role="button">{user.role}</summary>
            <ul class="menu menu-sm dropdown-content mt-3 z-[1] p-2 shadow bg-base-100 rounded-box w-32">
              <li><button on:click={logout}>Logout</button></li>
            </ul>
          </details>
        {:else}
          <a href="/login" class="btn">Login</a>
          <a href="/register" class="btn btn-outline">Register</a>
        {/if}
      </div>
    </div>

    <main class="container mx-auto flex-1 p-4">
      <slot />
    </main>

    <footer class="footer footer-center p-4 bg-base-200 text-base-content">
      <aside>
        <p>© 2025 CodeGrader</p>
      </aside>
    </footer>
  </div>


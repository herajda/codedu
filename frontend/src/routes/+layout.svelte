<script lang="ts">
  import { auth } from '$lib/auth';
  import { get } from 'svelte/store';
  import { goto } from '$app/navigation';
  import '../app.css';

  function logout() {
    auth.logout();
    goto('/login');
  }

  $: user = get(auth);
</script>

  <div class="min-h-screen flex flex-col">
    <div class="navbar bg-base-200 shadow">
      <div class="flex-1">
        <a href="/dashboard" class="btn btn-ghost text-xl">CodeGrader</a>
        {#if user?.role === 'admin'}
          <a href="/admin" class="btn btn-ghost">Admin</a>
        {:else if user?.role === 'teacher'}
          <a href="/classes" class="btn btn-ghost">Classes</a>
        {:else if user?.role === 'student'}
          <a href="/my-classes" class="btn btn-ghost">My Classes</a>
        {/if}
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


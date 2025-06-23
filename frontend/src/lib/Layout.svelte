<script lang="ts">
  import { auth } from './auth';
  import { get } from 'svelte/store';
  import { push } from 'svelte-spa-router';

  function logout() {
    auth.logout();
    push('/login');
  }

  $: user = get(auth);
</script>

<nav class="bg-gray-800 text-white px-4 py-2 flex items-center justify-between">
  <div class="flex items-center space-x-4">
    <a href="#/dashboard" class="font-semibold text-lg">CodeGrader</a>
    {#if user?.role === 'admin'}
      <a href="#/admin" class="hover:text-blue-400">Admin</a>
    {:else if user?.role === 'teacher'}
      <a href="#/classes" class="hover:text-blue-400">Classes</a>
    {:else if user?.role === 'student'}
      <a href="#/my-classes" class="hover:text-blue-400">My Classes</a>
    {/if}
  </div>
  <div class="flex items-center space-x-4">
    {#if user}
      <span>{user.role}</span>
      <button on:click={logout} class="hover:text-blue-400">Logout</button>
    {:else}
      <a href="#/login" class="hover:text-blue-400">Login</a>
      <a href="#/register" class="hover:text-blue-400">Register</a>
    {/if}
  </div>
</nav>

<main class="p-4">
  <slot />
</main>

<style>
  main {
    @apply container mx-auto;
  }
</style>

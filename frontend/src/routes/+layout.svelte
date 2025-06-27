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

  <nav>
  <div class="left">
    <a href="/dashboard" class="brand">CodeGrader</a>
    {#if user?.role === 'admin'}
      <a href="/admin">Admin</a>
    {:else if user?.role === 'teacher'}
      <a href="/my-classes">Classes</a>
    {:else if user?.role === 'student'}
      <a href="/my-classes">My Classes</a>
    {/if}
  </div>
  <div class="right">
    {#if user}
      <span>{user.role}</span>
      <button on:click={logout}>Logout</button>
    {:else}
      <a href="/login">Login</a>
      <a href="/register">Register</a>
    {/if}
  </div>
</nav>

<main>
  <slot />
</main>


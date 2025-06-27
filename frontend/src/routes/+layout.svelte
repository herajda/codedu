<script lang="ts">
  import { auth } from '$lib/auth';
  import { get } from 'svelte/store';
  import { goto } from '$app/navigation';
  import '../app.css';
  import {
    Navbar,
    NavBrand,
    NavHamburger,
    NavUl,
    NavLi,
    Button
  } from 'flowbite-svelte';
  const ActionButton: any = Button

  function logout() {
    auth.logout();
    goto('/login');
  }

  $: user = get(auth);
</script>

<Navbar class="border-b mb-4">
  <NavBrand href="/dashboard" class="ml-2">
    <span class="self-center whitespace-nowrap text-xl font-semibold dark:text-white">CodeGrader</span>
  </NavBrand>
  <NavHamburger />
  <NavUl class="flex-1 justify-end">
    {#if user?.role === 'admin'}
      <NavLi href="/admin">Admin</NavLi>
    {:else if user?.role === 'teacher'}
      <NavLi href="/my-classes">Classes</NavLi>
    {:else if user?.role === 'student'}
      <NavLi href="/my-classes">My Classes</NavLi>
    {/if}
    {#if user}
      <NavLi class="flex items-center">
        <span class="text-sm mr-2">{user.role}</span>
        <ActionButton color="gray" size="sm" on:click={logout}>Logout</ActionButton>
      </NavLi>
    {:else}
      <NavLi><ActionButton size="sm" href="/login">Login</ActionButton></NavLi>
      <NavLi><ActionButton color="light" size="sm" href="/register">Register</ActionButton></NavLi>
    {/if}
  </NavUl>
</Navbar>

<main class="container mx-auto p-4">
  <slot />
</main>


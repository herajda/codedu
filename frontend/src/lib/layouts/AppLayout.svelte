<script lang="ts">
  import { auth } from '../auth'
  import Button from '../ui/Button.svelte'
  import { push } from 'svelte-spa-router'

  function logout() {
    auth.logout()
    push('/login')
  }
</script>

<div class="min-h-screen flex flex-col bg-gray-50">
  <nav class="bg-gray-800 text-white px-4 py-3 flex items-center gap-4">
    <a href="#/dashboard" class="font-bold">Code-Grader</a>
    <span class="flex-1"></span>
    {#if $auth}
      <Button variant="secondary" on:click={() => logout()}>Logout</Button>
    {/if}
  </nav>
  <main class="flex-1 p-4 max-w-5xl w-full mx-auto">
    <slot />
  </main>
</div>

<script lang="ts">
  import { auth } from '../auth'
  import Button from '../ui/Button.svelte'
  import { push } from 'svelte-spa-router'

  function logout() {
    auth.logout()
    push('/login')
  }
</script>

<nav class="bg-gray-800 text-white px-4 py-2 flex items-center gap-4">
  <a href="#/dashboard" class="font-bold">Code-Grader</a>
  <span class="flex-1"></span>
  {#if $auth}
    <Button variant="secondary" on:click={() => logout()}>Logout</Button>
  {/if}
</nav>
<div class="p-4 max-w-5xl mx-auto">
  <slot />
</div>

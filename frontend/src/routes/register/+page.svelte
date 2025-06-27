<script lang="ts">
    import { goto } from '$app/navigation'
    import { Card, Button, Input } from 'flowbite-svelte'
    let email = ''
    let password = ''
    let error = ''
  
    async function submit() {
      error = ''
      const res = await fetch('/api/register', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ email, password })
      })
      if (res.status === 201) {
        goto('/login')
      } else {
        error = (await res.json()).error
      }
    }
  </script>
  
<Card class="max-w-md mx-auto p-6">
  <h1 class="text-xl font-semibold mb-4 text-center">Register</h1>
  <form on:submit|preventDefault={submit} class="flex flex-col gap-4">
    <Input type="email" bind:value={email} placeholder="Email" required />
    <Input type="password" bind:value={password} placeholder="Password" required />
    <Button type="submit" class="w-full">Register</Button>
  </form>
  {#if error}
    <p class="text-red-600 mt-2">{error}</p>
  {/if}
  <p class="mt-4 text-sm text-center">
    Already have an account? <a href="/login" class="text-blue-600 hover:underline">Log in</a>
  </p>
</Card>
  
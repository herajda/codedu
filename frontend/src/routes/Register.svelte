<script lang="ts">
    import { push } from 'svelte-spa-router'
    import Button from '../lib/ui/Button.svelte'
    import Input  from '../lib/ui/Input.svelte'
    let email = ''
    let password = ''
    let error = ''
  
    async function submit() {
      error = ''
      const res = await fetch('/register', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ email, password })
      })
      if (res.status === 201) {
        push('/login')
      } else {
        error = (await res.json()).error
      }
    }
  </script>
  
  <div class="flex min-h-[calc(100vh-4rem)] items-center justify-center">
    <div class="w-full max-w-md space-y-6 rounded-lg bg-white p-6 shadow">
      <h1 class="text-center text-2xl font-bold">Register</h1>
      <form on:submit|preventDefault={submit} class="space-y-4">
        <Input type="email" bind:value={email} placeholder="Email" required />
        <Input type="password" bind:value={password} placeholder="Password" required />
        <Button type="submit" class="w-full">Register</Button>
      </form>
      {#if error}
        <p class="text-center text-red-600">{error}</p>
      {/if}
      <p class="text-center text-sm">
        Already have an account? <a href="#/login" class="underline">Log in</a>
      </p>
    </div>
  </div>
  
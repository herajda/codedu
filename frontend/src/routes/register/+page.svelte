<script lang="ts">
    import { goto } from '$app/navigation'
    import { sha256 } from '$lib/hash'
    let email = ''
    let password = ''
    let error = ''
  
    async function submit() {
      error = ''
      const res = await fetch('/api/register', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ email, password: await sha256(password) })
      })
      if (res.status === 201) {
        goto('/login')
      } else {
        error = (await res.json()).error
      }
    }
  </script>
  
  <h1 class="text-3xl font-bold text-center mb-6">Register</h1>
  <div class="flex justify-center">
    <form on:submit|preventDefault={submit} class="card w-full max-w-sm bg-base-100 shadow p-6 space-y-4">
      <input type="email" bind:value={email} placeholder="Email" required class="input input-bordered w-full" />
      <input type="password" bind:value={password} placeholder="Password" required class="input input-bordered w-full" />
      <button type="submit" class="btn btn-primary w-full">Register</button>
    </form>
  </div>
  {#if error}
    <p class="text-error text-center mt-4">{error}</p>
  {/if}
  <p class="text-center mt-4">
    Already have an account? <a href="/login" class="link link-primary">Log in</a>
  </p>
  
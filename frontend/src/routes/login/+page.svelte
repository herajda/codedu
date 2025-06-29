<script lang="ts">
    import { auth } from '$lib/auth'
    import { apiFetch } from '$lib/api'
    import { sha256 } from '$lib/hash'
    import { goto } from '$app/navigation'
    let email = ''
    let password = ''
    let bkUser = ''
    let bkPass = ''
    let error = ''
    let mode: 'local' | 'bakalari' = 'local'
  
    async function submit() {
      error = ''
      // 1. Log in
      const res = await fetch('/api/login', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ email, password: await sha256(password) })
      })
      if (!res.ok) {
        error = (await res.json()).error
        return
      }
      // 2. Fetch /api/me
      const meRes = await apiFetch('/api/me')
      if (!meRes.ok) {
        error = 'Couldn’t fetch user info'
        return
      }
      const me = await meRes.json()

      // 3. Store & smart-redirect
      auth.login(me.id, me.role)
      goto('/dashboard')
    }
    async function submitBk() {
      error = ''
      const res = await fetch('/api/login-bakalari', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ username: bkUser, password: bkPass })
      })
      if (!res.ok) {
        error = (await res.json()).error
        return
      }
      const meRes = await apiFetch('/api/me')
      if (!meRes.ok) {
        error = 'Couldn\u2019t fetch user info'
        return
      }
      const me = await meRes.json()
      auth.login(me.id, me.role)
      goto('/dashboard')
    }
  </script>
  
  <h1 class="text-3xl font-bold text-center mb-6">Log In</h1>
  <div role="tablist" class="tabs tabs-boxed justify-center mb-6">
    <a role="tab" class="tab {mode==='local' ? 'tab-active' : ''}" on:click={() => mode = 'local'}>Local</a>
    <a role="tab" class="tab {mode==='bakalari' ? 'tab-active' : ''}" on:click={() => mode = 'bakalari'}>Bakalari</a>
  </div>
  <div class="flex justify-center">
    {#if mode === 'local'}
      <form on:submit|preventDefault={submit} class="card w-full max-w-sm bg-base-100 shadow p-6 space-y-4">
        <input type="email" bind:value={email} placeholder="Email" required class="input input-bordered w-full" />
        <input type="password" bind:value={password} placeholder="Password" required class="input input-bordered w-full" />
        <button type="submit" class="btn btn-primary w-full">Log In</button>
      </form>
    {:else}
      <form on:submit|preventDefault={submitBk} class="card w-full max-w-sm bg-base-100 shadow p-6 space-y-4">
        <input bind:value={bkUser} placeholder="Username" required class="input input-bordered w-full" />
        <input type="password" bind:value={bkPass} placeholder="Password" required class="input input-bordered w-full" />
        <button type="submit" class="btn btn-primary w-full">Log In</button>
      </form>
    {/if}
  </div>
  {#if error}
    <p class="text-error text-center mt-4">{error}</p>
  {/if}
  <p class="text-center mt-4">
    Don’t have an account? <a href="/register" class="link link-primary">Register here</a>
  </p>

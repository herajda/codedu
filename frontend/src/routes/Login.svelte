<script lang="ts">
    import { auth } from '../lib/auth'
    import { apiFetch } from '../lib/api'
    import { push } from 'svelte-spa-router'
    import Button from '../lib/ui/Button.svelte'
    import Input  from '../lib/ui/Input.svelte'
    let email = ''
    let password = ''
    let bkUser = ''
    let bkPass = ''
    let error = ''
    let mode: 'local' | 'bakalari' = 'local'
  
    async function submit() {
      error = ''
      // 1. Log in
      const res = await fetch('/login', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ email, password })
      })
      if (!res.ok) {
        error = (await res.json()).error
        return
      }
      const { token } = await res.json()
      localStorage.setItem('jwt', token)
      // 2. Fetch /api/me
      const meRes = await apiFetch('/api/me')
      if (!meRes.ok) {
        error = 'Couldn’t fetch user info'
        return
      }
      const me = await meRes.json()
  
      // 3. Store & smart-redirect
      auth.login(token, me.id, me.role)
      if      (me.role === 'admin')   push('/admin')
      else if (me.role === 'teacher') push('/classes')
      else                            push('/my-classes')
    }
    async function submitBk() {
      error = ''
      const res = await fetch('/login-bakalari', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ username: bkUser, password: bkPass })
      })
      if (!res.ok) {
        error = (await res.json()).error
        return
      }
      const { token } = await res.json()
      localStorage.setItem('jwt', token)
      const meRes = await apiFetch('/api/me')
      if (!meRes.ok) {
        error = 'Couldn\u2019t fetch user info'
        return
      }
      const me = await meRes.json()
      auth.login(token, me.id, me.role)
      if      (me.role === 'admin')   push('/admin')
      else if (me.role === 'teacher') push('/classes')
      else                            push('/my-classes')
    }
  </script>
  
  <div class="flex min-h-[calc(100vh-4rem)] items-center justify-center">
    <div class="w-full max-w-md space-y-6 rounded-lg bg-white p-6 shadow">
      <h1 class="text-center text-2xl font-bold">Log In</h1>
      <div class="flex justify-center gap-2">
        <Button on:click={() => mode = 'local'} variant={mode==='local' ? 'default' : 'outline'}>Local</Button>
        <Button on:click={() => mode = 'bakalari'} variant={mode==='bakalari' ? 'default' : 'outline'}>Bakalari</Button>
      </div>
      {#if mode === 'local'}
        <form on:submit|preventDefault={submit} class="space-y-4">
          <Input type="email" bind:value={email} placeholder="Email" required />
          <Input type="password" bind:value={password} placeholder="Password" required />
          <Button type="submit" class="w-full">Log In</Button>
        </form>
      {:else}
        <form on:submit|preventDefault={submitBk} class="space-y-4">
          <Input bind:value={bkUser} placeholder="Username" required />
          <Input type="password" bind:value={bkPass} placeholder="Password" required />
          <Button type="submit" class="w-full">Log In</Button>
        </form>
      {/if}
      {#if error}
        <p class="text-center text-red-600">{error}</p>
      {/if}
      <p class="text-center text-sm">
        Don’t have an account? <a href="#/register" class="underline">Register here</a>
      </p>
    </div>
  </div>

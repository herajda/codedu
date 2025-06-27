<script lang="ts">
    import { auth } from '$lib/auth'
    import { apiFetch } from '$lib/api'
    import { goto } from '$app/navigation'
    import { Button, Input, Card } from 'flowbite-svelte'
    const ActionButton: any = Button
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
      if      (me.role === 'admin')   goto('/admin')
      else if (me.role === 'teacher') goto('/classes')
      else                            goto('/my-classes')
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
      const { token } = await res.json()
      localStorage.setItem('jwt', token)
      const meRes = await apiFetch('/api/me')
      if (!meRes.ok) {
        error = 'Couldn\u2019t fetch user info'
        return
      }
      const me = await meRes.json()
      auth.login(token, me.id, me.role)
      if      (me.role === 'admin')   goto('/admin')
      else if (me.role === 'teacher') goto('/my-classes')
      else                            goto('/my-classes')
    }
  </script>
  
<Card class="max-w-md mx-auto p-6">
  <div class="flex justify-center mb-4 gap-2">
    <ActionButton size="sm" on:click={() => mode = 'local'} color={mode==='local' ? 'primary' : 'light'}>Local</ActionButton>
    <ActionButton size="sm" on:click={() => mode = 'bakalari'} color={mode==='bakalari' ? 'primary' : 'light'}>Bakalari</ActionButton>
  </div>
  {#if mode === 'local'}
    <form on:submit|preventDefault={submit} class="flex flex-col gap-4">
      <Input type="email" bind:value={email} placeholder="Email" required />
      <Input type="password" bind:value={password} placeholder="Password" required />
      <Button type="submit" class="w-full">Log In</Button>
    </form>
  {:else}
    <form on:submit|preventDefault={submitBk} class="flex flex-col gap-4">
      <Input bind:value={bkUser} placeholder="Username" required />
      <Input type="password" bind:value={bkPass} placeholder="Password" required />
      <Button type="submit" class="w-full">Log In</Button>
    </form>
  {/if}
  {#if error}
    <p class="text-red-600 mt-2">{error}</p>
  {/if}
  <p class="mt-4 text-sm text-center">
    Don’t have an account? <a href="/register" class="text-blue-600 hover:underline">Register here</a>
  </p>
</Card>

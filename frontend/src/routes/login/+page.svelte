<script lang="ts">
    import { auth } from '$lib/auth'
    import { apiFetch } from '$lib/api'
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
      else if (me.role === 'teacher') goto('/classes')
      else                            goto('/my-classes')
    }
  </script>
  
  <h1>Log In</h1>
  <div>
    <button on:click={() => mode = 'local'} disabled={mode==='local'}>Local</button>
    <button on:click={() => mode = 'bakalari'} disabled={mode==='bakalari'}>Bakalari</button>
  </div>
  {#if mode === 'local'}
    <form on:submit|preventDefault={submit}>
      <input type="email" bind:value={email} placeholder="Email" required />
      <input type="password" bind:value={password} placeholder="Password" required />
      <button type="submit">Log In</button>
    </form>
  {:else}
    <form on:submit|preventDefault={submitBk}>
      <input bind:value={bkUser} placeholder="Username" required />
      <input type="password" bind:value={bkPass} placeholder="Password" required />
      <button type="submit">Log In</button>
    </form>
  {/if}
  {#if error}
    <p style="color: red">{error}</p>
  {/if}
  <p>
    Don’t have an account? <a href="/register">Register here</a>
  </p>

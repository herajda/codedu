<script lang="ts">
    import { auth } from '../lib/auth'
    import { apiFetch } from '../lib/api'
    import { push } from 'svelte-spa-router'
    let email = ''
    let password = ''
    let error = ''
  
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
  
      // 3. Store in auth and redirect
      auth.login(token, me.id, me.role)
      push('/dashboard')
    }
  </script>
  
  <h1>Log In</h1>
  <form on:submit|preventDefault={submit}>
    <input type="email" bind:value={email} placeholder="Email" required />
    <input type="password" bind:value={password} placeholder="Password" required />
    <button type="submit">Log In</button>
  </form>
  {#if error}
    <p style="color: red">{error}</p>
  {/if}
  <p>
    Don’t have an account? <a href="#/register">Register here</a>
  </p>
  
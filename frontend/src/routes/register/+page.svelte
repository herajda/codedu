<script lang="ts">
    import { push } from 'svelte-spa-router'
    import { goto } from '$app/navigation'
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
        goto('/login')
      } else {
        error = (await res.json()).error
      }
    }
  </script>
  
  <h1>Register</h1>
  <form on:submit|preventDefault={submit}>
    <input type="email" bind:value={email} placeholder="Email" required />
    <input type="password" bind:value={password} placeholder="Password" required />
    <button type="submit">Register</button>
  </form>
  {#if error}
    <p style="color: red">{error}</p>
  {/if}
  <p>
    Already have an account? <a href="/login">Log in</a>
  </p>
  
<script lang="ts">
    import { auth } from '$lib/auth'
    import { apiFetch } from '$lib/api'
    import { onMount } from 'svelte'
    import { goto } from '$app/navigation'
  
  
    let me = { id: 0, role: '' }
    let msg = ''
  
    onMount(async () => {
      // fetch /api/me again in case we need fresh data
      const r1 = await apiFetch('/api/me')
      if (r1.ok) me = await r1.json()
  
      // fetch ping
      const r2 = await apiFetch('/api/ping')
      if (r2.ok) {
        const j = await r2.json()
        msg = j.msg
      }
    })
  
    function logout() {
      auth.logout()
      goto('/login')
    }

  </script>
  
  <h1>Dashboard</h1>
  <p><strong>Your ID:</strong> {me.id}</p>
  <p><strong>Your role:</strong> {me.role}</p>
  <p><strong>Ping says:</strong> {msg}</p>
  <button on:click={logout}>Log out</button>

  
<script lang="ts">
    import { onMount } from 'svelte';
    import { apiFetch } from '../lib/api';
    import { push } from 'svelte-spa-router';
  
    let email = '';
    let password = '';
    let ok = '';
    let error = '';
  
    async function submit() {
      error = ok = '';
      const r = await apiFetch('/api/teachers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password })
      });
      if (r.status === 201) { ok = 'Teacher created ✔'; email = password = ''; }
      else { error = (await r.json()).error; }
    }
  </script>
  
  <h1>Add teacher</h1>
  <form on:submit|preventDefault={submit}>
    <input type="email" bind:value={email} placeholder="Email" required />
    <input type="password" bind:value={password} placeholder="Password" required />
    <button>Add</button>
  </form>
  {#if ok}<p style="color: green">{ok}</p>{/if}
  {#if error}<p style="color: red">{error}</p>{/if}
  
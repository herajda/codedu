<script lang="ts">
  import { apiFetch } from '$lib/api'
  import { sha256 } from '$lib/hash'
  import { page } from '$app/stores'

  let password = ''
  let confirm = ''
  let error = ''
  let success = false
  let loading = false

  $: token = $page.url.searchParams.get('token') ?? ''

  async function submit() {
    error = ''
    if (!token) {
      error = 'This reset link is invalid. Request a new one.'
      return
    }
    if (password.length < 6) {
      error = 'Password must be at least 6 characters'
      return
    }
    if (password !== confirm) {
      error = 'Passwords do not match'
      return
    }
    loading = true
    try {
      const hashed = await sha256(password)
      const res = await apiFetch('/api/password-reset/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, password: hashed })
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        error = data.error ?? res.statusText
        return
      }
      success = true
    } catch (e: any) {
      error = e.message ?? 'Something went wrong'
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>Reset Password</title>
</svelte:head>

<h1 class="text-3xl font-bold text-center mb-6">Reset Password</h1>
<div class="flex justify-center">
  <div class="card w-full max-w-md bg-base-100 shadow p-6 space-y-4">
    {#if !success}
      <form on:submit|preventDefault={submit} class="space-y-4">
        <p class="text-sm text-center text-base-content/70">
          Choose a new password for your account.
        </p>
        <input type="password" bind:value={password} placeholder="New password" required class="input input-bordered w-full" />
        <input type="password" bind:value={confirm} placeholder="Confirm password" required class="input input-bordered w-full" />
        <button type="submit" class="btn btn-primary w-full" disabled={loading}>
          {#if loading}
            Saving...
          {:else}
            Reset password
          {/if}
        </button>
      </form>
      {#if error}
        <p class="text-error text-center text-sm">{error}</p>
      {/if}
    {:else}
      <p class="text-center text-base-content">
        Your password has been updated. You can now log in with your new credentials.
      </p>
    {/if}
    <p class="text-center text-sm">
      <a href="/login" class="link link-primary">Return to login</a>
    </p>
  </div>
</div>

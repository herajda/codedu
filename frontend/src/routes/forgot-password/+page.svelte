<script lang="ts">
  import { apiFetch } from '$lib/api'

  let email = ''
  let error = ''
  let submitted = false
  let loading = false

  async function submit() {
    error = ''
    if (!email) return
    loading = true
    try {
      const res = await apiFetch('/api/password-reset/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email })
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        error = data.error ?? res.statusText
        return
      }
      submitted = true
    } catch (e: any) {
      error = e.message ?? 'Something went wrong'
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>Forgot Password</title>
</svelte:head>

<h1 class="text-3xl font-bold text-center mb-6">Forgot Password</h1>
<div class="flex justify-center">
  <div class="card w-full max-w-md bg-base-100 shadow p-6 space-y-4">
    {#if !submitted}
      <p class="text-sm text-center text-base-content/70">
        Enter the email you use for your local account. We will send a link to reset your password if it exists.
      </p>
      <form on:submit|preventDefault={submit} class="space-y-4">
        <input type="email" bind:value={email} placeholder="Email" required class="input input-bordered w-full" />
        <button type="submit" class="btn btn-primary w-full" disabled={loading}>
          {#if loading}
            Sending...
          {:else}
            Send reset email
          {/if}
        </button>
      </form>
      {#if error}
        <p class="text-error text-center text-sm">{error}</p>
      {/if}
    {:else}
      <p class="text-center text-base-content">
        If we found an account with that email, a reset link is on its way. Check your inbox and spam folder.
      </p>
    {/if}
    <p class="text-center text-sm">
      <a href="/login" class="link link-primary">Back to login</a>
    </p>
  </div>
</div>

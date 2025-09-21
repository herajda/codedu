<script lang="ts">
  import { goto } from '$app/navigation'
  import { onMount } from 'svelte'

  type Status = 'idle' | 'verifying' | 'success' | 'error' | 'sent'

  let status: Status = 'idle'
  let message = ''
  let email = ''
  let resent = false

  onMount(() => {
    const params = new URLSearchParams(window.location.search)
    const token = params.get('token') ?? ''
    email = params.get('email') ?? ''
    resent = params.get('resent') === '1' || params.get('resent')?.toLowerCase() === 'true'

    if (token) {
      verify(token)
    } else if (email) {
      status = 'sent'
      message = resent
        ? `We just sent a verification email to ${email}.`
        : `We sent a verification email to ${email}.`
    } else {
      status = 'idle'
      message = 'Use the link from your verification email to activate your account.'
    }
  })

  async function verify(token: string) {
    status = 'verifying'
    message = 'Verifying your email…'
    try {
      const res = await fetch('/api/verify-email', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token })
      })
      if (res.ok) {
        status = 'success'
        message = 'Your email has been verified. You can now log in.'
      } else {
        const payload = await res.json().catch(() => ({}))
        status = 'error'
        message = payload?.error ?? 'We could not verify your email. The link may have expired.'
      }
    } catch (err) {
      console.error(err)
      status = 'error'
      message = 'We could not verify your email right now. Please try again later.'
    }
  }
</script>

<h1 class="text-3xl font-bold text-center mb-6">Verify your email</h1>
<div class="card bg-base-100 shadow p-8 max-w-xl mx-auto space-y-4 text-center">
  {#if status === 'verifying'}
    <p class="text-base-content/80">{message}</p>
    <progress class="progress progress-primary w-full" />
  {:else if status === 'success'}
    <p class="text-success text-lg font-semibold">{message}</p>
    <button class="btn btn-primary" on:click={() => goto('/login')}>Go to login</button>
  {:else if status === 'error'}
    <p class="text-error text-lg font-semibold">{message}</p>
    <p class="text-sm text-base-content/70">
      Try logging in again to request a fresh verification email or contact support if the issue persists.
    </p>
    <div class="flex justify-center gap-2">
      <button class="btn" on:click={() => goto('/login')}>Back to login</button>
    </div>
  {:else}
    <p class="text-base-content/80">{message}</p>
    {#if email}
      <p class="text-sm text-base-content/70">
        Check your inbox for a message from CodEdu. Click the link inside to activate your account.
      </p>
      <p class="text-sm text-base-content/70">
        Make sure to look in your spam or promotions folder if you don’t see an email for <span class="font-semibold">{email}</span>.
      </p>
    {/if}
    <div class="flex justify-center gap-2">
      <button class="btn" on:click={() => goto('/login')}>Return to login</button>
    </div>
  {/if}
</div>

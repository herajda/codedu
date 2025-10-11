<script lang="ts">
  import { goto } from '$app/navigation'
  import { onMount } from 'svelte'
  import { t, translator } from '$lib/i18n'

  type Status = 'idle' | 'verifying' | 'success' | 'error' | 'sent'

  let status: Status = 'idle'
  let message = ''
  let email = ''
  let resent = false

  let translate; // Declare translate
  $: translate = $translator; // Reactive assignment

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
        ? t('frontend/src/routes/verify-email/+page.svelte::email_sent_resent', { email })
        : t('frontend/src/routes/verify-email/+page.svelte::email_sent', { email })
    } else {
      status = 'idle'
      message = t('frontend/src/routes/verify-email/+page.svelte::activate_account_instruction')
    }
  })

  async function verify(token: string) {
    status = 'verifying'
    message = t('frontend/src/routes/verify-email/+page.svelte::verifying_email')
    try {
      const res = await fetch('/api/verify-email', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token })
      })
      if (res.ok) {
        status = 'success'
        message = t('frontend/src/routes/verify-email/+page.svelte::email_verified_success')
      } else {
        const payload = await res.json().catch(() => ({}))
        status = 'error'
        message = payload?.error ?? t('frontend/src/routes/verify-email/+page.svelte::verify_error_expired')
      }
    } catch (err) {
      console.error(err)
      status = 'error'
      message = t('frontend/src/routes/verify-email/+page.svelte::verify_error_temporary')
    }
  }
</script>

<h1 class="text-3xl font-bold text-center mb-6">{translate('frontend/src/routes/verify-email/+page.svelte::title')}</h1>
<div class="card bg-base-100 shadow p-8 max-w-xl mx-auto space-y-4 text-center">
  {#if status === 'verifying'}
    <p class="text-base-content/80">{message}</p>
    <progress class="progress progress-primary w-full" />
  {:else if status === 'success'}
    <p class="text-success text-lg font-semibold">{message}</p>
    <button class="btn btn-primary" on:click={() => goto('/login')}>{translate('frontend/src/routes/verify-email/+page.svelte::go_to_login_btn')}</button>
  {:else if status === 'error'}
    <p class="text-error text-lg font-semibold">{message}</p>
    <p class="text-sm text-base-content/70">
      {translate('frontend/src/routes/verify-email/+page.svelte::login_again_or_contact_support')}
    </p>
    <div class="flex justify-center gap-2">
      <button class="btn" on:click={() => goto('/login')}>{translate('frontend/src/routes/verify-email/+page.svelte::back_to_login_btn')}</button>
    </div>
  {:else}
    <p class="text-base-content/80">{message}</p>
    {#if email}
      <p class="text-sm text-base-content/70">
        {translate('frontend/src/routes/verify-email/+page.svelte::check_inbox_for_codedu')}
      </p>
      <p class="text-sm text-base-content/70">
        {translate('frontend/src/routes/verify-email/+page.svelte::check_spam_promotions', { email: email })}
      </p>
    {/if}
    <div class="flex justify-center gap-2">
      <button class="btn" on:click={() => goto('/login')}>{translate('frontend/src/routes/verify-email/+page.svelte::return_to_login_btn')}</button>
    </div>
  {/if}
</div>

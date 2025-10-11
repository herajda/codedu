<script lang="ts">
  import { PUBLIC_TURNSTILE_SITE_KEY } from '$env/static/public'
  import { onMount } from 'svelte'
  import { apiFetch } from '$lib/api'
  import { t } from '$lib/i18n'

  const TURNSTILE_SCRIPT_URL = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
  const TURNSTILE_PLACEHOLDER = 'change_me'

  let email = ''
  let error = ''
  let submitted = false
  let loading = false
  let turnstileToken = ''
  let turnstileWidgetId: string | null = null
  let turnstileContainer: HTMLDivElement | null = null
  let turnstileScriptPromise: Promise<void> | null = null

  $: canSubmit = email.trim().length > 0 && turnstileToken.length > 0 && !loading

  function ensureTurnstileScript(): Promise<void> {
    if (typeof window === 'undefined') {
      return Promise.resolve()
    }
    if ((window as typeof window & { turnstile?: unknown }).turnstile) {
      return Promise.resolve()
    }
    if (turnstileScriptPromise) {
      return turnstileScriptPromise
    }
    turnstileScriptPromise = new Promise((resolve, reject) => {
      const script = document.createElement('script')
      script.src = TURNSTILE_SCRIPT_URL
      script.async = true
      script.defer = true
      script.onload = () => resolve()
      script.onerror = () => {
        turnstileScriptPromise = null
        reject(new Error('Turnstile script failed to load'))
      }
      document.head.appendChild(script)
    })
    return turnstileScriptPromise
  }

  function renderTurnstile() {
    if (typeof window === 'undefined') {
      return
    }
    const turnstile = (window as typeof window & { turnstile?: any }).turnstile
    if (!turnstile || typeof turnstile.render !== 'function' || !PUBLIC_TURNSTILE_SITE_KEY) {
      return
    }
    if (!turnstileContainer) {
      requestAnimationFrame(renderTurnstile)
      return
    }
    if (turnstileWidgetId) {
      return
    }
    turnstileWidgetId = turnstile.render(turnstileContainer, {
      sitekey: PUBLIC_TURNSTILE_SITE_KEY,
      callback: (token: string) => {
        turnstileToken = token
      },
      'expired-callback': () => {
        turnstileToken = ''
      },
      'error-callback': () => {
        turnstileToken = ''
      }
    })
  }

  function resetTurnstile() {
    turnstileToken = ''
    if (typeof window === 'undefined') {
      return
    }
    const turnstile = (window as typeof window & { turnstile?: any }).turnstile
    if (turnstile && typeof turnstile.reset === 'function') {
      if (turnstileWidgetId) {
        turnstile.reset(turnstileWidgetId)
      } else {
        turnstile.reset()
      }
    }
  }

  onMount(() => {
    if (!PUBLIC_TURNSTILE_SITE_KEY || PUBLIC_TURNSTILE_SITE_KEY === TURNSTILE_PLACEHOLDER) {
      error = t('frontend/src/routes/forgot-password/+page.svelte::verification_misconfigured_error')
      return
    }

    let cancelled = false

    ensureTurnstileScript()
      .then(() => {
        if (!cancelled) {
          renderTurnstile()
        }
      })
      .catch(() => {
        if (!cancelled) {
          error = t('frontend/src/routes/forgot-password/+page.svelte::verification_challenge_load_error')
        }
      })

    return () => {
      cancelled = true
      if (typeof window === 'undefined') {
        return
      }
      const turnstile = (window as typeof window & { turnstile?: any }).turnstile
      if (turnstile && typeof turnstile.remove === 'function' && turnstileWidgetId) {
        turnstile.remove(turnstileWidgetId)
        turnstileWidgetId = null
      }
    }
  })

  async function submit() {
    error = ''
    if (!email.trim()) {
      error = t('frontend/src/routes/forgot-password/+page.svelte::enter_email_error')
      return
    }
    if (!turnstileToken) {
      error = t('frontend/src/routes/forgot-password/+page.svelte::complete_verification_error')
      return
    }
    loading = true
    try {
      const res = await apiFetch('/api/password-reset/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim(), turnstileToken })
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        error = data.error ?? res.statusText
        resetTurnstile()
        return
      }
      submitted = true
    } catch (e: any) {
      error = e?.message ?? t('frontend/src/routes/forgot-password/+page.svelte::something_went_wrong')
      resetTurnstile()
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>{t('frontend/src/routes/forgot-password/+page.svelte::forgot_password_title')}</title>
</svelte:head>

<h1 class="text-3xl font-bold text-center mb-6">{t('frontend/src/routes/forgot-password/+page.svelte::forgot_password_heading')}</h1>
<div class="flex justify-center">
  <div class="card w-full max-w-md bg-base-100 shadow p-6 space-y-4">
    {#if !submitted}
      <p class="text-sm text-center text-base-content/70">
        {t('frontend/src/routes/forgot-password/+page.svelte::forgot_password_intro')}
      </p>
      <form on:submit|preventDefault={submit} class="space-y-4">
        <input type="email" bind:value={email} placeholder={t('frontend/src/routes/forgot-password/+page.svelte::email_placeholder')} required class="input input-bordered w-full" />
        <div class="min-h-[80px]" bind:this={turnstileContainer}></div>
        <button type="submit" class="btn btn-primary w-full" disabled={!canSubmit}>
          {#if loading}
            {t('frontend/src/routes/forgot-password/+page.svelte::sending')}
          {:else}
            {t('frontend/src/routes/forgot-password/+page.svelte::send_reset_email')}
          {/if}
        </button>
      </form>
      {#if error}
        <p class="text-error text-center text-sm">{error}</p>
      {/if}
    {:else}
      <p class="text-center text-base-content">
        {t('frontend/src/routes/forgot-password/+page.svelte::reset_link_sent_message')}
      </p>
    {/if}
    <p class="text-center text-sm">
      <a href="/login" class="link link-primary">{t('frontend/src/routes/forgot-password/+page.svelte::back_to_login_link')}</a>
    </p>
  </div>
</div>

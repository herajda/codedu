<script lang="ts">
    import { goto } from '$app/navigation'
    import { PUBLIC_TURNSTILE_SITE_KEY } from '$env/static/public'
    import { onMount } from 'svelte'
    import { sha256 } from '$lib/hash'
    import { t, translator } from '$lib/i18n'

    let translate;
    $: translate = $translator;

    const TURNSTILE_SCRIPT_URL = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'

    let firstName = ''
    let lastName = ''
    let email = ''
    let password = ''
    let passwordConfirm = ''
    let error = ''
    let turnstileToken = ''
    let turnstileWidgetId: string | null = null
    let turnstileContainer: HTMLDivElement | null = null
    let turnstileScriptPromise: Promise<void> | null = null

    $: hasMinLength = password.length > 8
    $: hasLetter = /[A-Za-z]/.test(password)
    $: hasNumber = /\d/.test(password)
    $: meetsPasswordRules = hasMinLength && hasLetter && hasNumber
    $: passwordsMatch = passwordConfirm.length === 0 ? false : password === passwordConfirm
    $: canSubmit =
      firstName.trim().length > 0 &&
      lastName.trim().length > 0 &&
      email.trim().length > 0 &&
      meetsPasswordRules &&
      passwordsMatch &&
      turnstileToken.length > 0

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

    const TURNSTILE_PLACEHOLDER = 'change_me'

    onMount(() => {
      if (!PUBLIC_TURNSTILE_SITE_KEY || PUBLIC_TURNSTILE_SITE_KEY === TURNSTILE_PLACEHOLDER) {
        error = t('frontend/src/routes/register/+page.svelte::registration_temporarily_unavailable_error')
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
            error = t('frontend/src/routes/register/+page.svelte::unable_to_load_verification_challenge_error')
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
      if (!firstName.trim() || !lastName.trim()) {
        error = t('frontend/src/routes/register/+page.svelte::provide_first_and_last_name_error')
        return
      }
      if (!meetsPasswordRules) {
        error = t('frontend/src/routes/register/+page.svelte::password_rules_error')
        return
      }
      if (!passwordsMatch) {
        error = t('frontend/src/routes/register/+page.svelte::passwords_must_match_error')
        return
      }
      if (!turnstileToken) {
        error = t('frontend/src/routes/register/+page.svelte::complete_verification_challenge_error')
        return
      }

      const trimmedEmail = email.trim()
      const res = await fetch('/api/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          firstName: firstName.trim(),
          lastName: lastName.trim(),
          email: trimmedEmail,
          password: await sha256(password),
          turnstileToken
        })
      })
      if (res.status === 201) {
        goto(`/verify-email?email=${encodeURIComponent(trimmedEmail)}`)
      } else {
        const { error: message } = await res.json()
        error = message ?? t('frontend/src/routes/register/+page.svelte::registration_failed_try_again_error')
        resetTurnstile()
      }
    }
  </script>
  
  <h1 class="text-3xl font-bold text-center mb-6">{translate('frontend/src/routes/register/+page.svelte::register_title')}</h1>
  <div class="flex justify-center">
    <form on:submit|preventDefault={submit} class="card w-full max-w-sm bg-base-100 shadow p-6 space-y-4">
      <input type="text" bind:value={firstName} placeholder={translate('frontend/src/routes/register/+page.svelte::first_name_placeholder')} required class="input input-bordered w-full" />
      <input type="text" bind:value={lastName} placeholder={translate('frontend/src/routes/register/+page.svelte::last_name_placeholder')} required class="input input-bordered w-full" />
      <input type="email" bind:value={email} placeholder={translate('frontend/src/routes/register/+page.svelte::email_placeholder')} required class="input input-bordered w-full" />
      <div class="space-y-2">
        <input type="password" bind:value={password} placeholder={translate('frontend/src/routes/register/+page.svelte::password_placeholder')} required class="input input-bordered w-full" />
        <div class="bg-base-200 rounded-lg p-3 text-sm space-y-2">
          <p class="font-semibold text-base-content">{translate('frontend/src/routes/register/+page.svelte::password_requirements_heading')}</p>
          <ul class="space-y-1">
            <li class={`flex items-center gap-2 ${hasMinLength ? 'text-success' : 'text-base-content/70'}`}>
              <span class={`inline-flex w-2 h-2 rounded-full ${hasMinLength ? 'bg-success' : 'bg-base-300'}`}></span>
              <span>{translate('frontend/src/routes/register/+page.svelte::at_least_9_characters')}</span>
            </li>
            <li class={`flex items-center gap-2 ${hasLetter ? 'text-success' : 'text-base-content/70'}`}>
              <span class={`inline-flex w-2 h-2 rounded-full ${hasLetter ? 'bg-success' : 'bg-base-300'}`}></span>
              <span>{translate('frontend/src/routes/register/+page.svelte::includes_a_letter')}</span>
            </li>
            <li class={`flex items-center gap-2 ${hasNumber ? 'text-success' : 'text-base-content/70'}`}>
              <span class={`inline-flex w-2 h-2 rounded-full ${hasNumber ? 'bg-success' : 'bg-base-300'}`}></span>
              <span>{translate('frontend/src/routes/register/+page.svelte::includes_a_number')}</span>
            </li>
            <li class={`flex items-center gap-2 ${passwordConfirm.length === 0 ? 'text-base-content/70' : passwordsMatch ? 'text-success' : 'text-error'}`}>
              <span class={`inline-flex w-2 h-2 rounded-full ${passwordConfirm.length === 0 ? 'bg-base-300' : passwordsMatch ? 'bg-success' : 'bg-error'}`}></span>
              <span>{translate('frontend/src/routes/register/+page.svelte::passwords_match')}</span>
            </li>
          </ul>
        </div>
      </div>
      <input type="password" bind:value={passwordConfirm} placeholder={translate('frontend/src/routes/register/+page.svelte::confirm_password_placeholder')} required class="input input-bordered w-full" />
      <div class="min-h-[80px]" bind:this={turnstileContainer}></div>
      <button type="submit" class="btn btn-primary w-full" disabled={!canSubmit}>{translate('frontend/src/routes/register/+page.svelte::register_button')}</button>
    </form>
  </div>
  {#if error}
    <p class="text-error text-center mt-4">{error}</p>
  {/if}
  <p class="text-center mt-4">
    {translate('frontend/src/routes/register/+page.svelte::already_have_account_question')} <a href="/login" class="link link-primary">{translate('frontend/src/routes/register/+page.svelte::log_in_link')}</a>
  </p>
  
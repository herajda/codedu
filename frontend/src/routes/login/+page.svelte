<script lang="ts">
    import { auth } from '$lib/auth'
    import { apiFetch } from '$lib/api'
    import { sha256 } from '$lib/hash'
    import { goto } from '$app/navigation'
    import { login as bkLogin, hasBakalari } from '$lib/bakalari'
    import { t, translator } from '$lib/i18n'

    let translate;
    $: translate = $translator;
    
    let email = ''
    let password = ''
    let bkUser = ''
    let bkPass = ''
    let error = ''
    let needsVerification = false
    let verificationEmailSent = false
    let lastSubmittedEmail = ''
    let verificationHelpLink = '/verify-email'
    let mode: 'local' | 'bakalari' = 'local'
    
    // Get school name from environment
    const schoolName = import.meta.env.BAKALARI_SCHOOL_NAME || 'School'

    $: verificationHelpLink = (() => {
      const params: string[] = []
      if (lastSubmittedEmail) {
        params.push(`email=${encodeURIComponent(lastSubmittedEmail)}`)
      }
      if (verificationEmailSent) {
        params.push('resent=1')
      }
      const query = params.join('&')
      return query ? `/verify-email?${query}` : '/verify-email'
    })()

    async function submit() {
      error = ''
      needsVerification = false
      verificationEmailSent = false
      lastSubmittedEmail = email.trim()
      // 1. Log in (use apiFetch so credentials are consistently included)
      const res = await apiFetch('/api/login', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ email, password: await sha256(password) })
      })
      if (!res.ok) {
        try {
          const payload = await res.json()
          error = payload?.error ?? t('frontend/src/routes/login/+page.svelte::login_failed')
          needsVerification = Boolean(payload?.needsVerification)
          verificationEmailSent = Boolean(payload?.verificationEmailSent)
        } catch (parseErr) {
          console.error(parseErr)
          error = t('frontend/src/routes/login/+page.svelte::login_failed')
        }
        return
      }
      // 2. Fetch /api/me
      const meRes = await apiFetch('/api/me')
      if (!meRes.ok) {
        error = t('frontend/src/routes/login/+page.svelte::couldnt_fetch_user_info')
        return
      }
      const me = await meRes.json()

      // 3. Store & smart-redirect
      auth.login(
        me.id,
        me.role,
        me.name ?? null,
        me.avatar ?? null,
        me.bk_uid ?? null,
      me.email ?? null,
      me.email_verified ?? null,
      me.theme ?? null,
      me.email_notifications ?? true,
      me.email_message_digest ?? true,
      me.preferred_locale ?? null,
      )
      goto('/dashboard')
    }
    async function submitBk() {
      error = ''
      try {
        const { info } = await bkLogin(bkUser, bkPass)
        const parts = [info?.FirstName, info?.MiddleName, info?.LastName].filter(Boolean).join(' ').trim()
        const derivedName = (info?.FullName ?? info?.DisplayName ?? info?.UserName) ?? (parts.length > 0 ? parts : null)
        const res = await fetch('/api/login-bakalari', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({ uid: info.UserUID, role: info.UserType, class: info.Class?.Abbrev ?? null, name: derivedName })
        })
        if (!res.ok) {
          error = (await res.json()).error
          return
        }
        const meRes = await apiFetch('/api/me')
        if (!meRes.ok) {
          error = t('frontend/src/routes/login/+page.svelte::couldnt_fetch_user_info')
          return
        }
        const me = await meRes.json()
        auth.login(
          me.id,
          me.role,
          me.name ?? null,
          me.avatar ?? null,
          me.bk_uid ?? null,
      me.email ?? null,
      me.email_verified ?? null,
      me.theme ?? null,
      me.email_notifications ?? true,
      me.email_message_digest ?? true,
      me.preferred_locale ?? null,
        )
        goto('/dashboard')
      } catch (e: any) {
        error = e.message
      }
    }
  </script>
  
  <h1 class="text-3xl font-bold text-center mb-6">{t('frontend/src/routes/login/+page.svelte::log_in_title')}</h1>
  <div role="tablist" aria-label="Login method" class="flex justify-center mb-6">
    <div class="inline-flex items-center gap-1 rounded-lg bg-base-200 p-1">
      <button
        role="tab"
        aria-selected={mode === 'local'}
        class="px-4 py-2 rounded-md transition font-medium focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/60 {mode==='local' ? 'bg-base-100 text-primary shadow ring-1 ring-primary/30' : 'text-base-content/70 hover:text-base-content'}"
        on:click={() => mode = 'local'}
      >
        {t('frontend/src/routes/login/+page.svelte::email_tab')}
      </button>
      {#if hasBakalari}
        <button
          role="tab"
          aria-selected={mode === 'bakalari'}
          class="px-4 py-2 rounded-md transition font-medium focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/60 {mode==='bakalari' ? 'bg-base-100 text-primary shadow ring-1 ring-primary/30' : 'text-base-content/70 hover:text-base-content'}"
          on:click={() => mode = 'bakalari'}
        >
          <span class="inline-flex items-center gap-2">
            {t('frontend/src/routes/login/+page.svelte::bakalari_tab')}
          </span>
        </button>
      {/if}
    </div>
  </div>
  <div class="flex justify-center">
    {#if mode === 'local' || !hasBakalari}
      <div class="w-full max-w-sm">
        <form on:submit|preventDefault={submit} class="card w-full bg-base-100 shadow p-6 space-y-4">
          <input type="email" bind:value={email} placeholder={t('frontend/src/routes/login/+page.svelte::email_placeholder')} required class="input input-bordered w-full" />
          <input type="password" bind:value={password} placeholder={t('frontend/src/routes/login/+page.svelte::password_placeholder')} required class="input input-bordered w-full" />
          <button type="submit" class="btn btn-primary w-full">{t('frontend/src/routes/login/+page.svelte::log_in_button')}</button>
        </form>
      </div>
    {:else}
      <div class="w-full max-w-sm">
        <!-- Bakalari Header with Logo and School Name -->
        <div class="text-center mb-6">
          <img src="/bakalari-logo.svg" alt="Bakalari" class="w-40 h-40 mx-auto mb-4" />
          <h2 class="text-xl font-semibold text-gray-700">{schoolName}</h2>
        </div>
        
        <!-- Login Form -->
        <form on:submit|preventDefault={submitBk} class="card bg-base-100 shadow p-6 space-y-4">
          <input bind:value={bkUser} placeholder={t('frontend/src/routes/login/+page.svelte::username_placeholder')} required class="input input-bordered w-full" />
          <input type="password" bind:value={bkPass} placeholder={t('frontend/src/routes/login/+page.svelte::password_placeholder')} required class="input input-bordered w-full" />
          <button type="submit" class="btn btn-primary w-full">{t('frontend/src/routes/login/+page.svelte::log_in_button')}</button>
        </form>
      </div>
    {/if}
  </div>
  {#if error}
    <p class="text-error text-center mt-4">{error}</p>
  {/if}
  {#if needsVerification}
    <div class="alert alert-info mx-auto mt-4 max-w-sm">
      <div>
        <p class="font-semibold">{translate('frontend/src/routes/login/+page.svelte::verify_email_title')}</p>
        <p class="text-sm">
          {#if verificationEmailSent && lastSubmittedEmail}
            {translate('frontend/src/routes/login/+page.svelte::verification_email_sent', { email: lastSubmittedEmail })}
          {:else if lastSubmittedEmail}
            {translate('frontend/src/routes/login/+page.svelte::account_needs_verification_email', { email: lastSubmittedEmail })}
          {:else}
            {translate('frontend/src/routes/login/+page.svelte::account_needs_verification')}
          {/if}
        </p>
      </div>
      <div class="mt-3 flex justify-end">
        <button type="button" class="btn btn-sm btn-primary" on:click={() => goto(verificationHelpLink)}>
          {translate('frontend/src/routes/login/+page.svelte::view_instructions_button')}
        </button>
      </div>
    </div>
  {/if}
  <div class="mt-6 space-y-2 text-center text-sm text-base-content/80">
    <p>
      {t('frontend/src/routes/login/+page.svelte::no_account_question')}
      <a href="/register" class="link link-primary">{t('frontend/src/routes/login/+page.svelte::register_here_link')}</a>
    </p>
    {#if mode === 'local' || !hasBakalari}
      <p>
        <a href="/forgot-password" class="link link-secondary">{t('frontend/src/routes/login/+page.page.svelte::forgot_password_link')}</a>
      </p>
    {/if}
  </div>

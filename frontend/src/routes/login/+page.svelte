<script lang="ts">
    import { auth } from '$lib/auth'
    import { apiFetch } from '$lib/api'
    import { sha256 } from '$lib/hash'
    import { goto } from '$app/navigation'
    import { login as bkLogin, hasBakalari } from '$lib/bakalari'
    
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
          error = payload?.error ?? 'Login failed'
          needsVerification = Boolean(payload?.needsVerification)
          verificationEmailSent = Boolean(payload?.verificationEmailSent)
        } catch (parseErr) {
          console.error(parseErr)
          error = 'Login failed'
        }
        return
      }
      // 2. Fetch /api/me
      const meRes = await apiFetch('/api/me')
      if (!meRes.ok) {
        error = 'Couldn\'t fetch user info'
        return
      }
      const me = await meRes.json()

      // 3. Store & smart-redirect
      auth.login(me.id, me.role, me.name ?? null, me.avatar ?? null, me.bk_uid ?? null, me.email ?? null, me.theme ?? null, me.email_notifications ?? true, me.email_message_digest ?? true)
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
          error = 'Couldn\'t fetch user info'
          return
        }
        const me = await meRes.json()
        auth.login(me.id, me.role, me.name ?? null, me.avatar ?? null, me.bk_uid ?? null, me.email ?? null, me.theme ?? null, me.email_notifications ?? true, me.email_message_digest ?? true)
        goto('/dashboard')
      } catch (e: any) {
        error = e.message
      }
    }
  </script>
  
  <h1 class="text-3xl font-bold text-center mb-6">Log In</h1>
    <div role="tablist" class="tabs tabs-boxed justify-center mb-6">
    <a role="tab" class="tab {mode==='local' ? 'tab-active' : ''}" on:click={() => mode = 'local'}>Email</a>
    {#if hasBakalari}
      <a role="tab" class="tab {mode==='bakalari' ? 'tab-active' : ''}" on:click={() => mode = 'bakalari'}>
        <img src="/bakalari-logo.svg" alt="Bakalari" class="w-16 h-16" />
      </a>
    {/if}
  </div>
  <div class="flex justify-center">
    {#if mode === 'local' || !hasBakalari}
      <div class="w-full max-w-sm">
        <form on:submit|preventDefault={submit} class="card w-full bg-base-100 shadow p-6 space-y-4">
          <input type="email" bind:value={email} placeholder="Email" required class="input input-bordered w-full" />
          <input type="password" bind:value={password} placeholder="Password" required class="input input-bordered w-full" />
          <button type="submit" class="btn btn-primary w-full">Log In</button>
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
          <input bind:value={bkUser} placeholder="Username" required class="input input-bordered w-full" />
          <input type="password" bind:value={bkPass} placeholder="Password" required class="input input-bordered w-full" />
          <button type="submit" class="btn btn-primary w-full">Log In</button>
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
        <p class="font-semibold">Verify your email to continue</p>
        <p class="text-sm">
          {#if verificationEmailSent && lastSubmittedEmail}
            We just sent a new verification email to {lastSubmittedEmail}. Check your inbox and spam folder.
          {:else if lastSubmittedEmail}
            Your account for {lastSubmittedEmail} needs to be verified before you can log in.
          {:else}
            Your account needs to be verified before you can log in.
          {/if}
        </p>
      </div>
      <div class="mt-3 flex justify-end">
        <button type="button" class="btn btn-sm btn-primary" on:click={() => goto(verificationHelpLink)}>
          View instructions
        </button>
      </div>
    </div>
  {/if}
  <div class="mt-6 space-y-2 text-center text-sm text-base-content/80">
    <p>
      Don't have an account?
      <a href="/register" class="link link-primary">Register here</a>
    </p>
    {#if mode === 'local' || !hasBakalari}
      <p>
        <a href="/forgot-password" class="link link-secondary">Forgot your password?</a>
      </p>
    {/if}
  </div>

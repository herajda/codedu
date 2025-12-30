<script lang="ts">
    import { auth } from '$lib/auth'
    import { apiFetch } from '$lib/api'
    import { sha256 } from '$lib/hash'
    import { goto } from '$app/navigation'
    import { login as bkLogin, hasBakalari } from '$lib/bakalari'
    import { t, translator } from '$lib/i18n'
    import { onMount, tick } from 'svelte'
    import { fade, slide, scale } from 'svelte/transition'
    import { 
        Mail, 
        Lock, 
        User, 
        LogIn, 
        AlertCircle, 
        ChevronRight, 
        UserPlus, 
        ShieldCheck,
        Sparkles,
        GraduationCap
    } from 'lucide-svelte'

    let translate;
    $: translate = $translator;
    
    let email = ''
    let password = ''
    let bkUser = ''
    let bkPass = ''
    let error = ''
    let isLoading = false
    let needsVerification = false
    let verificationEmailSent = false
    let lastSubmittedEmail = ''
    let verificationHelpLink = '/verify-email'
    let mode: 'local' | 'bakalari' = 'local'
    let allowMicrosoftLogin = true
    let allowBakalariLogin = true
    let passwordInput: HTMLInputElement | null = null
    
    onMount(async () => {
      try {
        const res = await apiFetch('/api/public-settings')
        if (res.ok) {
          const config = await res.json()
          allowMicrosoftLogin = config.allow_microsoft_login
          allowBakalariLogin = config.allow_bakalari_login
        }
      } catch (e) {
        console.error('Failed to fetch public settings', e)
      }
    })
    
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
      if (isLoading) return
      error = ''
      isLoading = true
      needsVerification = false
      verificationEmailSent = false
      lastSubmittedEmail = email.trim()
      
      try {
        // 1. Log in
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
          isLoading = false
          return
        }

        // 2. Fetch /api/me
        const meRes = await apiFetch('/api/me')
        if (!meRes.ok) {
          error = t('frontend/src/routes/login/+page.svelte::couldnt_fetch_user_info')
          isLoading = false
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
          me.force_bakalari_email ?? true,
          me.allow_microsoft_login ?? true,
        )
        goto('/dashboard')
      } catch (e: any) {
        error = e.message
        isLoading = false
      }
    }

    async function submitBk() {
      if (isLoading) return
      error = ''
      isLoading = true
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
          isLoading = false
          return
        }
        const meRes = await apiFetch('/api/me')
        if (!meRes.ok) {
          error = t('frontend/src/routes/login/+page.svelte::couldnt_fetch_user_info')
          isLoading = false
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
          me.force_bakalari_email ?? true,
          me.allow_microsoft_login ?? true,
        )
        goto('/dashboard')
      } catch (e: any) {
        error = e.message
        isLoading = false
      }
    }
</script>

<svelte:head>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@100..900&display=swap" rel="stylesheet">
  <title>{t('frontend/src/routes/login/+page.svelte::log_in_title')} | CodEdu</title>
</svelte:head>

<div class="auth-page">


  <div class="flex-1 flex items-center justify-center p-4 min-h-[calc(100vh-4rem)]">
      <div class="w-full max-w-[460px] relative" in:scale={{duration: 600, start: 0.98, opacity: 0}}>
        
        <!-- Header Section -->
        <div class="text-center mb-6">
            <h1 class="text-6xl font-black tracking-tight mb-4 pb-2 bg-clip-text text-transparent bg-gradient-to-br from-base-content via-base-content/80 to-base-content/50 dark:from-white dark:to-white/80" in:fade={{delay: 300}}>
                {t('frontend/src/routes/login/+page.svelte::log_in_title')}
            </h1>
            <p class="text-base-content/50 dark:text-white/70 font-bold uppercase tracking-[0.2em] text-xs" in:fade={{delay: 400}}>
                {t('frontend/src/routes/messages/+page.svelte::connect_message')}
            </p>
        </div>

        <!-- Main Login Card -->
        <div class="bg-white/80 dark:bg-base-100/40 backdrop-blur-2xl border border-white dark:border-white/10 shadow-[0_32px_64px_-16px_rgba(0,0,0,0.1)] dark:shadow-[0_32px_64px_-16px_rgba(0,0,0,0.5)] rounded-[2.5rem] overflow-hidden relative">
            


            <div class="p-10 relative">
                {#if mode === 'local'}
                    <form on:submit|preventDefault={submit} class="space-y-8" in:fade={{duration: 300}}>
                        
                        <!-- Email Input -->
                        <div class="space-y-2">
                            <label class="text-xs font-black uppercase tracking-[0.3em] text-base-content/40 dark:text-white/70 ml-5 block">
                                {t('frontend/src/routes/login/+page.svelte::email_placeholder')}
                            </label>
                            <div class="relative group">
                                <div class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center text-base-content/30 group-focus-within:text-primary transition-colors duration-300">
                                    <Mail size={18} />
                                </div>
                                <input 
                                    type="email" 
                                    bind:value={email} 
                                    placeholder="your@email.com" 
                                    required 
                                    class="w-full bg-base-200/50 dark:bg-white/5 border-2 border-transparent focus:border-primary/20 focus:bg-white dark:focus:bg-base-100 pl-14 pr-6 h-16 rounded-2xl font-bold transition-all outline-none text-lg dark:text-white placeholder:opacity-30" 
                                    on:keydown={(event) => {
                                      if (event.key === 'Tab' && !event.shiftKey) {
                                        event.preventDefault()
                                        passwordInput?.focus()
                                      }
                                    }}
                                />
                            </div>
                        </div>

                        <!-- Password Input -->
                        <div class="space-y-2">
                            <div class="flex items-center justify-between pr-5">
                                <label class="text-xs font-black uppercase tracking-[0.3em] text-base-content/40 dark:text-white/70 ml-5 block">
                                    {t('frontend/src/routes/login/+page.svelte::password_placeholder')}
                                </label>
                                <a href="/forgot-password" class="text-xs font-black uppercase tracking-widest text-primary hover:opacity-70 transition-opacity">
                                    {t('frontend/src/routes/login/+page.svelte::forgot_password_link')}
                                </a>
                            </div>
                            <div class="relative group">
                                <div class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center text-base-content/30 group-focus-within:text-primary transition-colors duration-300">
                                    <Lock size={18} />
                                </div>
                                <input 
                                    type="password" 
                                    bind:value={password} 
                                    placeholder="••••••••" 
                                    required 
                                    class="w-full bg-base-200/50 dark:bg-white/5 border-2 border-transparent focus:border-primary/20 focus:bg-white dark:focus:bg-base-100 pl-14 pr-6 h-16 rounded-2xl font-bold transition-all outline-none text-lg dark:text-white placeholder:opacity-30" 
                                    bind:this={passwordInput}
                                />
                            </div>
                        </div>

                        <!-- Submit Button -->
                        <button 
                            type="submit" 
                            disabled={isLoading}
                            class="group relative w-full h-16 rounded-2xl font-black uppercase tracking-[0.2em] text-sm transition-all duration-500 overflow-hidden active:scale-95 disabled:opacity-70"
                        >
                            <div class="absolute inset-0 bg-gradient-to-r from-primary via-primary-focus to-primary group-hover:bg-pos-100 bg-pos-0 transition-all duration-500 shadow-[0_8px_24px_-8px_rgba(var(--p),0.5)]"></div>
                            <div class="relative flex items-center justify-center gap-3 text-primary-content">
                                {#if isLoading}
                                    <span class="loading loading-spinner loading-sm"></span>
                                {:else}
                                    {t('frontend/src/routes/login/+page.svelte::log_in_button')}
                                    <LogIn size={18} class="transition-transform group-hover:translate-x-1" />
                                {/if}
                            </div>
                        </button>

                        {#if allowMicrosoftLogin || (hasBakalari && allowBakalariLogin)}
                             <div class="flex items-center gap-4 py-2">
                                 <div class="h-px flex-1 bg-base-content/5 dark:bg-white/5"></div>
                                 <span class="text-[10px] font-black uppercase tracking-[0.4em] text-base-content/20 dark:text-white/40">OR</span>
                                 <div class="h-px flex-1 bg-base-content/5 dark:bg-white/5"></div>
                             </div>

                             <div class="space-y-3">
                                 {#if allowMicrosoftLogin}
                                     <a href="/api/auth/microsoft/login" class="flex items-center justify-center gap-3 w-full h-16 rounded-2xl font-bold transition-all duration-300 bg-white dark:bg-white/5 border border-base-200/50 dark:border-white/5 hover:bg-base-50 dark:hover:bg-white/10 shadow-sm group">
                                        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 23 23" class="group-hover:scale-110 transition-transform">
                                            <path fill="#f35325" d="M1 1h10v10H1z"/><path fill="#81bc06" d="M12 1h10v10H12z"/><path fill="#05a6f0" d="M1 12h10v10H1z"/><path fill="#ffba08" d="M12 12h10v10H12z"/>
                                        </svg>
                                        <span class="text-base">{t('frontend/src/routes/login/+page.svelte::sign_in_with_microsoft')}</span>
                                     </a>
                                 {/if}

                                 {#if hasBakalari && allowBakalariLogin}
                                     <button on:click={() => mode = 'bakalari'} type="button" class="flex items-center justify-center gap-3 w-full h-16 rounded-2xl font-bold transition-all duration-300 bg-white dark:bg-white/5 border border-base-200/50 dark:border-white/5 hover:bg-base-50 dark:hover:bg-white/10 shadow-sm group text-black dark:text-white">
                                        <img src="/bakalari_logo_small.webp" alt="Bakaláři" class="w-8 h-8 object-contain group-hover:scale-110 transition-transform" />
                                        <span class="text-base">{t('frontend/src/routes/login/+page.svelte::log_in_with_bakalari')}</span>
                                     </button>
                                 {/if}
                             </div>
                        {/if}
                    </form>
                {:else}
                    <div in:fade={{duration: 300}} class="space-y-8">
                         <!-- Bakalari specific header -->
                         <div class="text-center group">
                             <div class="w-40 h-40 mx-auto mb-6 flex items-center justify-center">
                                 <img src="/bakalari_logo_small.webp" alt="Bakalari" class="w-full h-full object-contain transition-all duration-500 group-hover:scale-110" />
                             </div>
                             <h2 class="text-xs font-black uppercase tracking-[0.4em] text-primary">{schoolName}</h2>
                         </div>

                         <form on:submit|preventDefault={submitBk} class="space-y-6">
                            <div class="space-y-2">
                                <label class="text-xs font-black uppercase tracking-[0.3em] text-base-content/40 dark:text-white/70 ml-5 block">
                                    {t('frontend/src/routes/login/+page.svelte::username_placeholder')}
                                </label>
                                <div class="relative group">
                                    <div class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center text-base-content/30 group-focus-within:text-primary transition-colors duration-300">
                                        <User size={18} />
                                    </div>
                                    <input 
                                        bind:value={bkUser} 
                                        placeholder={t('frontend/src/routes/login/+page.svelte::username_placeholder')} 
                                        required 
                                        class="w-full bg-base-200/50 dark:bg-white/5 border-2 border-transparent focus:border-primary/20 focus:bg-white dark:focus:bg-base-100 pl-14 pr-6 h-16 rounded-2xl font-bold transition-all outline-none text-base dark:text-white placeholder:opacity-30" 
                                    />
                                </div>
                            </div>

                            <div class="space-y-2">
                                <label class="text-xs font-black uppercase tracking-[0.3em] text-base-content/40 dark:text-white/70 ml-5 block">
                                    {t('frontend/src/routes/login/+page.svelte::password_placeholder')}
                                </label>
                                <div class="relative group">
                                    <div class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center text-base-content/30 group-focus-within:text-primary transition-colors duration-300">
                                        <Lock size={18} />
                                    </div>
                                    <input 
                                        type="password" 
                                        bind:value={bkPass} 
                                        placeholder={t('frontend/src/routes/login/+page.svelte::password_placeholder')} 
                                        required 
                                        class="w-full bg-base-200/50 dark:bg-white/5 border-2 border-transparent focus:border-primary/20 focus:bg-white dark:focus:bg-base-100 pl-14 pr-6 h-16 rounded-2xl font-bold transition-all outline-none text-base dark:text-white placeholder:opacity-30" 
                                    />
                                </div>
                            </div>

                            <button 
                                type="submit" 
                                disabled={isLoading}
                                class="group relative w-full h-16 rounded-2xl font-black uppercase tracking-[0.2em] text-sm transition-all duration-500 overflow-hidden active:scale-95 disabled:opacity-70"
                            >
                                <div class="absolute inset-0 bg-gradient-to-r from-primary via-primary-focus to-primary group-hover:bg-pos-100 bg-pos-0 transition-all duration-500 shadow-[0_8px_24px_-8px_rgba(var(--p),0.5)]"></div>
                                <div class="relative flex items-center justify-center gap-3 text-primary-content">
                                    {#if isLoading}
                                        <span class="loading loading-spinner loading-sm"></span>
                                    {:else}
                                        {t('frontend/src/routes/login/+page.svelte::log_in_button')}
                                        <LogIn size={18} class="transition-transform group-hover:translate-x-1" />
                                    {/if}
                                </div>
                            </button>

                            <button type="button" on:click={() => mode = 'local'} class="w-full text-xs font-black uppercase tracking-widest text-base-content/40 hover:text-primary transition-colors py-2">
                                {t('frontend/src/routes/login/+page.svelte::back_to_login')}
                            </button>
                         </form>
                    </div>
                {/if}
            </div>
        </div>

        <!-- Feedback & Footer -->
        <div class="mt-8 space-y-6">
            {#if error}
                <div class="p-5 rounded-2xl bg-error/10 border border-error/20 flex items-start gap-4 text-error shadow-xl shadow-error/5" in:slide>
                    <AlertCircle size={20} class="shrink-0 mt-0.5" />
                    <div class="space-y-1">
                        <p class="text-xs font-black uppercase tracking-widest leading-none">{t('frontend/src/routes/login/+page.svelte::error_occurred')}</p>
                        <p class="text-xs font-bold opacity-90 leading-relaxed">{error}</p>
                    </div>
                </div>
            {/if}

            {#if needsVerification}
                <div class="p-6 rounded-[2rem] bg-info/10 border border-info/20 shadow-xl shadow-info/5 overflow-hidden relative group" in:slide>
                    <div class="absolute top-0 right-0 w-32 h-32 bg-info/10 rounded-full blur-3xl -mr-16 -mt-16"></div>
                    <div class="flex items-center gap-4 text-info mb-4">
                        <div class="w-10 h-10 rounded-xl bg-info/20 flex items-center justify-center">
                            <ShieldCheck size={22} />
                        </div>
                        <p class="font-black uppercase tracking-[0.2em] text-xs">{translate('frontend/src/routes/login/+page.svelte::verify_email_title')}</p>
                    </div>
                    <p class="text-xs font-bold opacity-80 mb-5 leading-relaxed">
                        {#if verificationEmailSent && lastSubmittedEmail}
                            {translate('frontend/src/routes/login/+page.svelte::verification_email_sent', { email: lastSubmittedEmail })}
                        {:else if lastSubmittedEmail}
                            {translate('frontend/src/routes/login/+page.svelte::account_needs_verification_email', { email: lastSubmittedEmail })}
                        {:else}
                            {translate('frontend/src/routes/login/+page.svelte::account_needs_verification')}
                        {/if}
                    </p>
                    <button type="button" class="btn btn-info btn-block rounded-xl font-black uppercase tracking-[0.2em] text-xs h-12" on:click={() => goto(verificationHelpLink)}>
                        {translate('frontend/src/routes/login/+page.svelte::view_instructions_button')}
                    </button>
                </div>
            {/if}

            <div class="text-center" in:fade={{delay: 600}}>
                 <div class="text-base font-bold bg-white/40 dark:bg-white/5 backdrop-blur-md inline-flex items-center gap-2 px-6 py-3 rounded-full border border-white/50 dark:border-white/5 shadow-sm">
                     <span class="text-base-content/40 dark:text-white/70">{t('frontend/src/routes/login/+page.svelte::no_account_question')}</span>
                     <a href="/register" class="text-primary font-black hover:opacity-70 transition-opacity">
                         {t('frontend/src/routes/login/+page.svelte::register_here_link')}
                     </a>
                 </div>
            </div>
        </div>
      </div>
  </div>
</div>

<style>
  .auth-page {
    font-family: 'Outfit', sans-serif;
  }
  .bg-pos-0 { background-size: 200% 100%; background-position: 0% 0%; }
  .bg-pos-100 { background-size: 200% 100%; background-position: 100% 0%; }
</style>

<script lang="ts">
    import { goto } from '$app/navigation'
    import { PUBLIC_TURNSTILE_SITE_KEY } from '$env/static/public'
    import { onMount, tick } from 'svelte'
    import { sha256 } from '$lib/hash'
    import { t, translator } from '$lib/i18n'
    import { fade, slide, scale } from 'svelte/transition'
    import { 
        User, 
        Mail, 
        Lock, 
        UserPlus, 
        ShieldCheck, 
        ChevronRight, 
        AlertCircle, 
        Info,
        CheckCircle2,
        XCircle
    } from 'lucide-svelte'

    let translate;
    $: translate = $translator;

    const TURNSTILE_SCRIPT_URL = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'

    let firstName = ''
    let lastName = ''
    let email = ''
    let password = ''
    let passwordConfirm = ''
    let error = ''
    let isLoading = false
    let turnstileToken = ''
    let turnstileWidgetId: string | null = null
    let turnstileContainer: HTMLDivElement | null = null
    let turnstileScriptPromise: Promise<void> | null = null

    $: hasMinLength = password.length >= 9
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
      turnstileToken.length > 0 &&
      !isLoading

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
      if (!canSubmit) return
      error = ''
      isLoading = true
      
      const trimmedEmail = email.trim()
      try {
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
          const payload = await res.json()
          error = payload.error ?? t('frontend/src/routes/register/+page.svelte::registration_failed_try_again_error')
          resetTurnstile()
          isLoading = false
        }
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
  <title>{translate('frontend/src/routes/register/+page.svelte::register_title')} | CodEdu</title>
</svelte:head>

<!-- Animated Background -->
<div class="fixed inset-0 -z-10 overflow-hidden bg-[#fafafa] dark:bg-[#050505]">
    <div class="absolute inset-0 opacity-[0.4] dark:opacity-[0.2]" style="background-image: url('https://www.transparenttextures.com/patterns/cubes.png');"></div>
    <div class="absolute top-[-10%] right-[-10%] w-[40%] h-[40%] rounded-full bg-secondary/20 blur-[120px] animate-pulse"></div>
    <div class="absolute bottom-[-10%] left-[-10%] w-[40%] h-[40%] rounded-full bg-primary/20 blur-[120px] animate-pulse" style="animation-delay: 2s;"></div>
    <div class="absolute bottom-[20%] right-[10%] w-[25%] h-[25%] rounded-full bg-accent/10 blur-[100px] animate-pulse" style="animation-delay: 1s;"></div>
</div>

<div class="flex-1 flex items-center justify-center p-4 min-h-[calc(100vh-4rem)]">
    <div class="w-full max-w-[520px] relative" in:scale={{duration: 600, start: 0.98, opacity: 0}}>
        
        <!-- Header Section -->
        <div class="text-center mb-8">
            <div class="inline-flex items-center justify-center p-4 bg-white dark:bg-base-100 shadow-2xl rounded-[2rem] mb-6 relative group border border-base-200/50 dark:border-white/5" in:fade={{delay: 200}}>
                <div class="absolute inset-0 bg-primary/20 rounded-[2rem] blur-xl opacity-0 group-hover:opacity-100 transition-opacity duration-500"></div>
                <UserPlus class="w-10 h-10 text-primary relative" />
                <div class="absolute -bottom-1 -left-1 flex h-4 w-4">
                    <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-secondary opacity-20"></span>
                    <span class="relative inline-flex rounded-full h-4 w-4 bg-secondary/40"></span>
                </div>
            </div>
            <h1 class="text-5xl font-black tracking-tight mb-4 bg-clip-text text-transparent bg-gradient-to-br from-base-content via-base-content/80 to-base-content/50 dark:from-white dark:to-white/40" in:fade={{delay: 300}}>
                {translate('frontend/src/routes/register/+page.svelte::register_title')}
            </h1>
            <p class="text-base-content/50 dark:text-white/40 font-bold uppercase tracking-[0.2em] text-[10px]" in:fade={{delay: 400}}>
                {translate('frontend/src/routes/register/+page.svelte::register_intro')}
            </p>
        </div>

        <!-- Main Register Card -->
        <div class="bg-white/80 dark:bg-base-100/40 backdrop-blur-2xl border border-white dark:border-white/10 shadow-[0_32px_64px_-16px_rgba(0,0,0,0.1)] dark:shadow-[0_32px_64px_-16px_rgba(0,0,0,0.5)] rounded-[2.5rem] overflow-hidden relative">
            
            <div class="p-10 relative">
                <form on:submit|preventDefault={submit} class="space-y-6 relative z-10">
                    
                    <!-- Name Row -->
                    <div class="grid grid-cols-1 sm:grid-cols-2 gap-6">
                        <div class="space-y-2">
                            <label class="text-[10px] font-black uppercase tracking-[0.3em] opacity-40 ml-5 block text-left">
                                {translate('frontend/src/routes/register/+page.svelte::first_name_placeholder')}
                            </label>
                            <div class="relative group">
                                <div class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center text-base-content/30 group-focus-within:text-primary transition-colors duration-300">
                                    <User size={18} />
                                </div>
                                <input 
                                    type="text" 
                                    bind:value={firstName} 
                                    placeholder="John" 
                                    required 
                                    class="w-full bg-base-200/50 dark:bg-white/5 border-2 border-transparent focus:border-primary/20 focus:bg-white dark:focus:bg-base-100 pl-14 pr-6 h-16 rounded-2xl font-bold transition-all outline-none text-base placeholder:opacity-30" 
                                />
                            </div>
                        </div>
                        <div class="space-y-2">
                            <label class="text-[10px] font-black uppercase tracking-[0.3em] opacity-40 ml-5 block text-left">
                                {translate('frontend/src/routes/register/+page.svelte::last_name_placeholder')}
                            </label>
                            <div class="relative group">
                                <div class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center text-base-content/30 group-focus-within:text-primary transition-colors duration-300">
                                    <User size={18} />
                                </div>
                                <input 
                                    type="text" 
                                    bind:value={lastName} 
                                    placeholder="Doe" 
                                    required 
                                    class="w-full bg-base-200/50 dark:bg-white/5 border-2 border-transparent focus:border-primary/20 focus:bg-white dark:focus:bg-base-100 pl-14 pr-6 h-16 rounded-2xl font-bold transition-all outline-none text-base placeholder:opacity-30" 
                                />
                            </div>
                        </div>
                    </div>

                    <!-- Email -->
                    <div class="space-y-2">
                        <label class="text-[10px] font-black uppercase tracking-[0.3em] opacity-40 ml-5 block text-left">
                            {translate('frontend/src/routes/register/+page.svelte::email_placeholder')}
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
                                class="w-full bg-base-200/50 dark:bg-white/5 border-2 border-transparent focus:border-primary/20 focus:bg-white dark:focus:bg-base-100 pl-14 pr-6 h-16 rounded-2xl font-bold transition-all outline-none text-base placeholder:opacity-30" 
                            />
                        </div>
                    </div>

                    <!-- Password and Rules -->
                    <div class="space-y-6">
                        <div class="grid grid-cols-1 sm:grid-cols-2 gap-6">
                            <div class="space-y-2">
                                <label class="text-[10px] font-black uppercase tracking-[0.3em] opacity-40 ml-5 block text-left">
                                    {translate('frontend/src/routes/register/+page.svelte::password_placeholder')}
                                </label>
                                <div class="relative group">
                                    <div class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center text-base-content/30 group-focus-within:text-primary transition-colors duration-300">
                                        <Lock size={18} />
                                    </div>
                                    <input 
                                        type="password" 
                                        bind:value={password} 
                                        placeholder="••••••••" 
                                        required 
                                        class="w-full bg-base-200/50 dark:bg-white/5 border-2 border-transparent focus:border-primary/20 focus:bg-white dark:focus:bg-base-100 pl-14 pr-6 h-16 rounded-2xl font-bold transition-all outline-none text-base placeholder:opacity-30" 
                                    />
                                </div>
                            </div>
                            <div class="space-y-2">
                                <label class="text-[10px] font-black uppercase tracking-[0.3em] opacity-40 ml-5 block text-left">
                                    {translate('frontend/src/routes/register/+page.svelte::confirm_password_placeholder')}
                                </label>
                                <div class="relative group">
                                    <div class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center text-base-content/30 group-focus-within:text-primary transition-colors duration-300">
                                        <Lock size={18} />
                                    </div>
                                    <input 
                                        type="password" 
                                        bind:value={passwordConfirm} 
                                        placeholder="••••••••" 
                                        required 
                                        class="w-full bg-base-200/50 dark:bg-white/5 border-2 border-transparent focus:border-primary/20 focus:bg-white dark:focus:bg-base-100 pl-14 pr-6 h-16 rounded-2xl font-bold transition-all outline-none text-base placeholder:opacity-30" 
                                    />
                                </div>
                            </div>
                        </div>

                        <!-- Requirements Visual -->
                        <div class="bg-base-200/30 dark:bg-white/5 backdrop-blur-xl rounded-[2rem] p-6 border border-base-200/50 dark:border-white/5">
                            <div class="flex items-center gap-3 mb-4">
                                <div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center text-primary">
                                    <ShieldCheck size={16} />
                                </div>
                                <p class="text-[10px] font-black uppercase tracking-[0.2em] opacity-60">
                                    {translate('frontend/src/routes/register/+page.svelte::password_requirements_heading')}
                                </p>
                            </div>
                            <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3">
                                <div class={`flex items-center gap-3 text-[11px] font-bold transition-all duration-300 ${hasMinLength ? 'text-success' : 'opacity-30'}`}>
                                    <div class={`w-5 h-5 rounded-full flex items-center justify-center border-2 ${hasMinLength ? 'bg-success/10 border-success' : 'border-current opacity-20'}`}>
                                        {#if hasMinLength}<CheckCircle2 size={12} />{/if}
                                    </div>
                                    <span>{translate('frontend/src/routes/register/+page.svelte::at_least_9_characters')}</span>
                                </div>
                                <div class={`flex items-center gap-3 text-[11px] font-bold transition-all duration-300 ${hasLetter ? 'text-success' : 'opacity-30'}`}>
                                    <div class={`w-5 h-5 rounded-full flex items-center justify-center border-2 ${hasLetter ? 'bg-success/10 border-success' : 'border-current opacity-20'}`}>
                                        {#if hasLetter}<CheckCircle2 size={12} />{/if}
                                    </div>
                                    <span>{translate('frontend/src/routes/register/+page.svelte::includes_a_letter')}</span>
                                </div>
                                <div class={`flex items-center gap-3 text-[11px] font-bold transition-all duration-300 ${hasNumber ? 'text-success' : 'opacity-30'}`}>
                                    <div class={`w-5 h-5 rounded-full flex items-center justify-center border-2 ${hasNumber ? 'bg-success/10 border-success' : 'border-current opacity-20'}`}>
                                        {#if hasNumber}<CheckCircle2 size={12} />{/if}
                                    </div>
                                    <span>{translate('frontend/src/routes/register/+page.svelte::includes_a_number')}</span>
                                </div>
                                <div class={`flex items-center gap-3 text-[11px] font-bold transition-all duration-300 ${passwordConfirm.length > 0 ? (passwordsMatch ? 'text-success' : 'text-error animate-pulse') : 'opacity-30'}`}>
                                    <div class={`w-5 h-5 rounded-full flex items-center justify-center border-2 ${passwordConfirm.length > 0 ? (passwordsMatch ? 'bg-success/10 border-success' : 'bg-error/10 border-error') : 'border-current opacity-20'}`}>
                                        {#if passwordConfirm.length > 0 && passwordsMatch}<CheckCircle2 size={12} />{:else if passwordConfirm.length > 0}<XCircle size={12} />{/if}
                                    </div>
                                    <span>{translate('frontend/src/routes/register/+page.svelte::passwords_match')}</span>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Turnstile -->
                    <div class="flex justify-center bg-base-200/20 dark:bg-white/5 rounded-2xl py-3 border border-dashed border-base-200/50 dark:border-white/5" bind:this={turnstileContainer}></div>

                    <!-- Submit -->
                    <button 
                        type="submit" 
                        disabled={!canSubmit || isLoading}
                        class="group relative w-full h-16 rounded-2xl font-black uppercase tracking-[0.2em] text-xs transition-all duration-500 overflow-hidden active:scale-95 disabled:opacity-50"
                    >
                        <div class="absolute inset-0 bg-gradient-to-r from-primary via-primary-focus to-primary group-hover:bg-pos-100 bg-pos-0 transition-all duration-500 shadow-[0_8px_24px_-8px_rgba(var(--p),0.5)]"></div>
                        <div class="relative flex items-center justify-center gap-3 text-primary-content">
                            {#if isLoading}
                                <span class="loading loading-spinner loading-sm"></span>
                            {:else}
                                {translate('frontend/src/routes/register/+page.svelte::register_button')}
                                <ChevronRight size={18} class="transition-transform group-hover:translate-x-1" />
                            {/if}
                        </div>
                    </button>
                </form>
            </div>
        </div>

        <!-- Footer -->
        <div class="mt-8 space-y-6">
            {#if error}
                <div class="p-5 rounded-2xl bg-error/10 border border-error/20 flex items-start gap-4 text-error shadow-xl shadow-error/5" in:slide>
                    <AlertCircle size={20} class="shrink-0 mt-0.5" />
                    <div class="space-y-1">
                        <p class="text-[10px] font-black uppercase tracking-widest leading-none">{translate('frontend/src/routes/register/+page.svelte::registration_error')}</p>
                        <p class="text-xs font-bold opacity-90 leading-relaxed">{error}</p>
                    </div>
                </div>
            {/if}

            <div class="text-center" in:fade={{delay: 600}}>
                 <div class="text-sm font-bold bg-white/40 dark:bg-white/5 backdrop-blur-md inline-flex items-center gap-2 px-6 py-3 rounded-full border border-white/50 dark:border-white/5 shadow-sm">
                     <span class="opacity-40">{translate('frontend/src/routes/register/+page.svelte::already_have_account_question')}</span>
                     <a href="/login" class="text-primary font-black hover:opacity-70 transition-opacity">
                         {translate('frontend/src/routes/register/+page.svelte::log_in_link')}
                     </a>
                 </div>
            </div>
        </div>
    </div>
</div>

<style>
  :global(body) {
    font-family: 'Outfit', sans-serif;
  }
  .bg-pos-0 { background-size: 200% 100%; background-position: 0% 0%; }
  .bg-pos-100 { background-size: 200% 100%; background-position: 100% 0%; }
</style>

  
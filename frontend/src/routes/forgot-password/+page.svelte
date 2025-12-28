<script lang="ts">
  import { PUBLIC_TURNSTILE_SITE_KEY } from '$env/static/public'
  import { onMount, tick } from 'svelte'
  import { apiFetch } from '$lib/api'
  import { t, translator } from '$lib/i18n'
  import { fade, slide, scale } from 'svelte/transition'
  import { 
      Mail, 
      KeyRound, 
      ArrowLeft, 
      AlertCircle, 
      Send, 
      CheckCircle2 
  } from 'lucide-svelte'

  let translate;
  $: translate = $translator;

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
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@100..900&display=swap" rel="stylesheet">
  <title>{t('frontend/src/routes/forgot-password/+page.svelte::forgot_password_title')} | CodEdu</title>
</svelte:head>

<!-- Animated Background -->
<div class="fixed inset-0 -z-10 overflow-hidden bg-[#fafafa] dark:bg-[#050505]">
    <div class="absolute inset-0 opacity-[0.4] dark:opacity-[0.2]" style="background-image: url('https://www.transparenttextures.com/patterns/cubes.png');"></div>
    <div class="absolute top-[20%] left-[-10%] w-[40%] h-[40%] rounded-full bg-accent/10 blur-[120px] animate-pulse"></div>
    <div class="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] rounded-full bg-primary/20 blur-[120px] animate-pulse" style="animation-delay: 2s;"></div>
</div>

<div class="flex-1 flex items-center justify-center p-4 min-h-[calc(100vh-4rem)]">
    <div class="w-full max-w-[460px] relative" in:scale={{duration: 600, start: 0.98, opacity: 0}}>
        
        <!-- Header Section -->
        <div class="text-center mb-10">
            <div class="inline-flex items-center justify-center p-4 bg-white dark:bg-base-100 shadow-2xl rounded-[2rem] mb-6 relative group border border-base-200/50 dark:border-white/5" in:fade={{delay: 200}}>
                <div class="absolute inset-0 bg-primary/20 rounded-[2rem] blur-xl opacity-0 group-hover:opacity-100 transition-opacity duration-500"></div>
                <KeyRound class="w-10 h-10 text-primary relative" />
            </div>
            <h1 class="text-5xl font-black tracking-tight mb-4 bg-clip-text text-transparent bg-gradient-to-br from-base-content via-base-content/80 to-base-content/50 dark:from-white dark:to-white/40" in:fade={{delay: 300}}>
                {t('frontend/src/routes/forgot-password/+page.svelte::forgot_password_heading')}
            </h1>
            <p class="text-base-content/50 dark:text-white/40 font-bold uppercase tracking-[0.2em] text-[10px] max-w-xs mx-auto" in:fade={{delay: 400}}>
                {t('frontend/src/routes/forgot-password/+page.svelte::forgot_password_intro')}
            </p>
        </div>

        <!-- Main Card -->
        <div class="bg-white/80 dark:bg-base-100/40 backdrop-blur-2xl border border-white dark:border-white/10 shadow-[0_32px_64px_-16px_rgba(0,0,0,0.1)] dark:shadow-[0_32px_64px_-16px_rgba(0,0,0,0.5)] rounded-[2.5rem] overflow-hidden relative">
            
            <div class="p-10 relative">
                {#if !submitted}
                  <form on:submit|preventDefault={submit} class="space-y-8 relative z-10" in:fade>
                      <div class="space-y-2">
                          <label class="text-[10px] font-black uppercase tracking-[0.3em] opacity-40 ml-5 block">
                              {t('frontend/src/routes/forgot-password/+page.svelte::email_placeholder')}
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

                      <!-- Turnstile -->
                      <div class="flex justify-center bg-base-200/20 dark:bg-white/5 rounded-2xl py-3 border border-dashed border-base-200/50 dark:border-white/5" bind:this={turnstileContainer}></div>

                      <button 
                          type="submit" 
                          disabled={!canSubmit || loading}
                          class="group relative w-full h-16 rounded-2xl font-black uppercase tracking-[0.2em] text-xs transition-all duration-500 overflow-hidden active:scale-95 disabled:opacity-50"
                      >
                          <div class="absolute inset-0 bg-gradient-to-r from-primary via-primary-focus to-primary group-hover:bg-pos-100 bg-pos-0 transition-all duration-500 shadow-[0_8px_24px_-8px_rgba(var(--p),0.5)]"></div>
                          <div class="relative flex items-center justify-center gap-3 text-primary-content">
                              {#if loading}
                                  <span class="loading loading-spinner loading-sm"></span>
                              {:else}
                                  {t('frontend/src/routes/forgot-password/+page.svelte::send_reset_email')}
                                  <Send size={18} class="transition-transform group-hover:translate-x-1 -rotate-45" />
                              {/if}
                          </div>
                      </button>
                  </form>
                {:else}
                  <div class="text-center py-6 relative z-10" in:scale>
                      <div class="w-20 h-20 bg-success/10 rounded-[2rem] flex items-center justify-center mx-auto mb-8 text-success relative group">
                          <div class="absolute inset-0 bg-success/20 rounded-[2rem] blur-xl group-hover:scale-125 transition-transform duration-500"></div>
                          <CheckCircle2 size={40} class="relative" />
                      </div>
                      <h3 class="text-2xl font-black tracking-tight mb-4 bg-clip-text text-transparent bg-gradient-to-br from-success to-success/60">{t('frontend/src/routes/forgot-password/+page.svelte::check_your_email_title')}</h3>
                      <p class="text-sm font-bold opacity-60 leading-relaxed px-4">
                          {t('frontend/src/routes/forgot-password/+page.svelte::reset_link_sent_message')}
                      </p>
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
                        <p class="text-[10px] font-black uppercase tracking-widest leading-none">{t('frontend/src/routes/forgot-password/+page.svelte::error_occurred')}</p>
                        <p class="text-xs font-bold opacity-90 leading-relaxed">{error}</p>
                    </div>
                </div>
            {/if}

            <div class="text-center" in:fade={{delay: 600}}>
                 <a href="/login" class="text-sm font-bold bg-white/40 dark:bg-white/5 backdrop-blur-md inline-flex items-center gap-3 px-8 py-3 rounded-full border border-white/50 dark:border-white/5 shadow-sm hover:opacity-70 transition-opacity group">
                     <ArrowLeft size={16} class="transition-transform group-hover:-translate-x-1" />
                     <span class="font-black uppercase tracking-[0.2em] text-[10px]">{t('frontend/src/routes/forgot-password/+page.svelte::back_to_login_link')}</span>
                 </a>
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


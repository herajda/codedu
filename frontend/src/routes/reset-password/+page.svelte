<script lang="ts">
  import { apiFetch } from '$lib/api'
  import { sha256 } from '$lib/hash'
  import { page } from '$app/stores'
  import { t, translator } from '$lib/i18n'

  let password = ''
  let confirm = ''
  let error = ''
  let success = false
  let loading = false

  let translate;
  $: translate = $translator;

  $: token = $page.url.searchParams.get('token') ?? ''

  async function submit() {
    error = ''
    if (!token) {
      error = t('frontend/src/routes/reset-password/+page.svelte::Invalid_Reset_Link')
      return
    }
    if (password.length < 6) {
      error = t('frontend/src/routes/reset-password/+page.svelte::Password_Min_Length')
      return
    }
    if (password !== confirm) {
      error = t('frontend/src/routes/reset-password/+page.svelte::Passwords_Do_Not_Match')
      return
    }
    loading = true
    try {
      const hashed = await sha256(password)
      const res = await apiFetch('/api/password-reset/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, password: hashed })
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        error = data.error ?? t('frontend/src/routes/reset-password/+page.svelte::Something_Went_Wrong')
        return
      }
      success = true
    } catch (e: any) {
      error = e.message ?? t('frontend/src/routes/reset-password/+page.svelte::Something_Went_Wrong')
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>{t('frontend/src/routes/reset-password/+page.svelte::Reset_Password')}</title>
</svelte:head>

<h1 class="text-3xl font-bold text-center mb-6">{t('frontend/src/routes/reset-password/+page.svelte::Reset_Password')}</h1>
<div class="flex justify-center">
  <div class="card w-full max-w-md bg-base-100 shadow p-6 space-y-4">
    {#if !success}
      <form on:submit|preventDefault={submit} class="space-y-4">
        <p class="text-sm text-center text-base-content/70">
          {t('frontend/src/routes/reset-password/+page.svelte::Choose_New_Password_Account')}
        </p>
        <input type="password" bind:value={password} placeholder={t('frontend/src/routes/reset-password/+page.svelte::New_Password')} required class="input input-bordered w-full" />
        <input type="password" bind:value={confirm} placeholder={t('frontend/src/routes/reset-password/+page.svelte::Confirm_Password')} required class="input input-bordered w-full" />
        <button type="submit" class="btn btn-primary w-full" disabled={loading}>
          {#if loading}
            {translate('frontend/src/routes/reset-password/+page.svelte::Saving')}
          {:else}
            {translate('frontend/src/routes/reset-password/+page.svelte::Reset_Password_Button')}
          {/if}
        </button>
      </form>
      {#if error}
        <p class="text-error text-center text-sm">{error}</p>
      {/if}
    {:else}
      <p class="text-center text-base-content">
        {t('frontend/src/routes/reset-password/+page.svelte::Password_Updated_Login')}
      </p>
    {/if}
    <p class="text-center text-sm">
      <a href="/login" class="link link-primary">{t('frontend/src/routes/reset-password/+page.svelte::Return_To_Login')}</a>
    </p>
  </div>
</div>

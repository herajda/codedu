<script lang="ts">
  import { tick } from 'svelte';
  import { t, translator } from '$lib/i18n';
  import { X, CheckCircle2, XCircle, ShieldCheck } from 'lucide-svelte';

  export type PromptModalOptions = {
    title?: string;
    body?: string;
    label?: string;
    placeholder?: string;
    initialValue?: string;
    confirmLabel?: string;
    cancelLabel?: string;
    confirmClass?: string;
    cancelClass?: string;
    icon?: any;
    helpText?: string;
    inputType?: string;
    selectOnOpen?: boolean;
    maxLength?: number;
    allowEmpty?: boolean;
    validate?: (value: string) => string | null;
    transform?: (value: string) => string;
    withConfirmation?: boolean;
    confirmationLabel?: string;
    confirmationPlaceholder?: string;
    showPasswordRequirements?: boolean;
  };

  let dialog: HTMLDialogElement | undefined;
  let inputEl: HTMLInputElement | HTMLTextAreaElement | null = null;
  let resolver: ((value: string | null) => void) | null = null;
  let currentOptions: PromptModalOptions = {};

  let translate;
  $: translate = $translator;

  let title = t('frontend/src/lib/components/PromptModal.svelte::enter_a_value');
  let body = '';
  let label = t('frontend/src/lib/components/PromptModal.svelte::value_label');
  let placeholder = '';
  let value = '';
  let confirmLabel = t('frontend/src/lib/components/PromptModal.svelte::save');
  let cancelLabel = t('frontend/src/lib/components/PromptModal.svelte::cancel');
  let confirmClass = 'btn btn-primary';
  let cancelClass = 'btn btn-ghost';
  let icon: any;
  let helpText = '';
  let inputType = 'text';
  let maxLength: number | undefined;
  let error = '';
  let allowEmpty = false;

  let withConfirmation = false;
  let confirmationLabel = '';
  let confirmationPlaceholder = '';
  let showPasswordRequirements = false;
  let confirmationValue = '';

  $: hasMinLength = value.length >= 9;
  $: hasLetter = /[A-Za-z]/.test(value);
  $: hasNumber = /\d/.test(value);
  $: meetsPasswordRules = hasMinLength && hasLetter && hasNumber;
  $: passwordsMatch = confirmationValue.length === 0 ? false : value === confirmationValue;

  export async function open(options: PromptModalOptions = {}): Promise<string | null> {
    currentOptions = options;
    title = options.title ?? t('frontend/src/lib/components/PromptModal.svelte::enter_a_value');
    body = options.body ?? '';
    label = options.label ?? t('frontend/src/lib/components/PromptModal.svelte::value_label');
    placeholder = options.placeholder ?? '';
    value = options.initialValue ?? '';
    confirmLabel = options.confirmLabel ?? t('frontend/src/lib/components/PromptModal.svelte::save');
    cancelLabel = options.cancelLabel ?? t('frontend/src/lib/components/PromptModal.svelte::cancel');
    confirmClass = options.confirmClass ?? 'btn btn-primary';
    cancelClass = options.cancelClass ?? 'btn btn-ghost hover:bg-base-200';
    icon = options.icon;
    helpText = options.helpText ?? '';
    inputType = options.inputType ?? 'text';
    maxLength = options.maxLength;
    allowEmpty = options.allowEmpty ?? false;
    withConfirmation = options.withConfirmation ?? false;
    confirmationLabel = options.confirmationLabel ?? '';
    confirmationPlaceholder = options.confirmationPlaceholder ?? '';
    showPasswordRequirements = options.showPasswordRequirements ?? false;
    confirmationValue = '';
    error = '';

    await tick();
    if (!dialog) throw new Error('PromptModal not mounted');
    dialog.showModal();
    await tick();
    if (inputEl) {
      if (options.selectOnOpen ?? true) {
        if ('select' in inputEl) inputEl.select();
      }
      inputEl.focus();
    }

    return new Promise<string | null>((resolve) => {
      resolver = resolve;
    });
  }

  function settle(result: string | null) {
    if (!dialog) return;
    if (resolver) {
      const resolve = resolver;
      resolver = null;
      resolve(result);
    }
    if (dialog.open) {
      dialog.close();
    }
  }

  function handleClose() {
    if (resolver) {
      const resolve = resolver;
      resolver = null;
      resolve(null);
    }
  }

  function handleCancel(event: Event) {
    event.preventDefault();
    settle(null);
  }

  function handleConfirm(event: Event) {
    event.preventDefault();
    const inputValue = value;
    if (!allowEmpty && (inputValue === null || inputValue === undefined || (typeof inputValue === 'string' && inputValue.trim() === ''))) {
      // Treat empty input as cancel to match legacy prompt behaviour
      settle(null);
      return;
    }

    if (withConfirmation && value !== confirmationValue) {
      error = translate('frontend/src/routes/+layout.svelte::passwords_must_match');
      return;
    }

    if (showPasswordRequirements && !meetsPasswordRules) {
      error = translate('frontend/src/routes/+layout.svelte::password_rules_not_met');
      return;
    }
    if (currentOptions.validate) {
      const validationMessage = currentOptions.validate(inputValue);
      if (validationMessage) {
        error = validationMessage;
        return;
      }
    }
    const transformed = currentOptions.transform ? currentOptions.transform(inputValue) : inputValue;
    settle(transformed);
  }

  function handleDialogCancel(event: Event) {
    event.preventDefault();
    settle(null);
  }
</script>

<dialog 
  bind:this={dialog} 
  class="modal" 
  on:close={handleClose} 
  on:cancel={handleDialogCancel}
>
  <div class="modal-box bg-base-100 rounded-[2.5rem] border border-base-200 shadow-2xl p-0 overflow-hidden max-w-md w-full">
    <!-- Header with Gradient Background -->
    <div class="bg-gradient-to-br from-primary/5 via-transparent to-transparent p-8 pb-4">
      <div class="flex items-start justify-between gap-4">
         <div class="flex items-center gap-4">
            {#if icon}
              <div class="w-12 h-12 rounded-2xl bg-primary/10 text-primary flex items-center justify-center shadow-sm shrink-0">
                {#if typeof icon === 'string'}
                  <i class={icon}></i>
                {:else}
                  <svelte:component this={icon} size={24} />
                {/if}
              </div>
            {/if}
            <div>
              <h2 class="text-xl font-black tracking-tight text-base-content">{title}</h2>
              {#if body}
                <p class="mt-1 text-xs font-bold opacity-40 uppercase tracking-widest leading-relaxed">{body}</p>
              {/if}
            </div>
         </div>
         <button 
           type="button" 
           class="btn btn-ghost btn-circle btn-sm opacity-30 hover:opacity-100 transition-all"
           on:click={handleCancel}
         >
           <X size={18} />
         </button>
      </div>
    </div>

    <!-- Content -->
    <form class="p-8 pt-4 space-y-6" on:submit={handleConfirm}>
      <div class="form-control w-full">
        {#if label}
          <label class="label pt-0 px-1">
            <span class="label-text font-black text-[10px] uppercase tracking-[0.15em] opacity-40">{label}</span>
          </label>
        {/if}
        
        <div class="relative group">
          <input
            bind:this={inputEl as HTMLInputElement}
            type={inputType}
            class="input w-full h-12 bg-base-200/50 border-base-300/50 focus:border-primary focus:bg-base-100 rounded-xl font-medium transition-all duration-300 px-4
                   {error ? 'border-error ring-2 ring-error/5' : ''}"
            bind:value
            placeholder={placeholder}
            maxlength={maxLength}
          />
          {#if helpText && !error}
            <span class="label-text-alt mt-2 text-[10px] font-bold opacity-30 uppercase tracking-wider block px-1 leading-relaxed">{helpText}</span>
          {/if}
          {#if error}
            <span class="text-error text-[10px] font-bold mt-2 uppercase tracking-wider block px-1 animate-pulse">{error}</span>
          {/if}
        </div>
      </div>

      {#if withConfirmation}
        <div class="form-control w-full">
          {#if confirmationLabel}
            <label class="label pt-0 px-1">
              <span class="label-text font-black text-[10px] uppercase tracking-[0.15em] opacity-40">{confirmationLabel}</span>
            </label>
          {/if}
          <div class="relative group">
            <input
              type={inputType}
              class="input w-full h-12 bg-base-200/50 border-base-300/50 focus:border-primary focus:bg-base-100 rounded-xl font-medium transition-all duration-300 px-4"
              bind:value={confirmationValue}
              placeholder={confirmationPlaceholder}
            />
          </div>
        </div>
      {/if}

      {#if showPasswordRequirements && inputType === 'password'}
        <div class="bg-base-200/30 rounded-[2rem] p-6 border border-base-200/50">
            <div class="flex items-center gap-3 mb-4">
                <div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center text-primary">
                    <ShieldCheck size={16} />
                </div>
                <p class="text-[10px] font-black uppercase tracking-[0.2em] opacity-60">
                    {translate('frontend/src/routes/register/+page.svelte::password_requirements_heading')}
                </p>
            </div>
            <div class="grid grid-cols-1 gap-y-3">
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
                {#if withConfirmation}
                  <div class={`flex items-center gap-3 text-[11px] font-bold transition-all duration-300 ${confirmationValue.length > 0 ? (passwordsMatch ? 'text-success' : 'text-error animate-pulse') : 'opacity-30'}`}>
                      <div class={`w-5 h-5 rounded-full flex items-center justify-center border-2 ${confirmationValue.length > 0 ? (passwordsMatch ? 'bg-success/10 border-success' : 'bg-error/10 border-error') : 'border-current opacity-20'}`}>
                          {#if confirmationValue.length > 0 && passwordsMatch}<CheckCircle2 size={12} />{:else if confirmationValue.length > 0}<XCircle size={12} />{/if}
                      </div>
                      <span>{translate('frontend/src/routes/register/+page.svelte::passwords_match')}</span>
                  </div>
                {/if}
            </div>
        </div>
      {/if}

      <!-- Actions -->
      <div class="flex items-center justify-end gap-3 pt-2">
        <button 
          type="button" 
          class="h-11 px-6 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all {cancelClass}" 
          on:click={handleCancel}
        >
          {cancelLabel}
        </button>
        <button 
          type="submit" 
          class="h-11 px-8 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all shadow-lg shadow-primary/20 hover:shadow-primary/30 hover:scale-[1.02] active:scale-[0.98] {confirmClass}"
        >
          {confirmLabel}
        </button>
      </div>
    </form>
  </div>
  
  <form method="dialog" class="modal-backdrop bg-base-content/20 backdrop-blur-sm" on:submit={handleCancel}>
    <button aria-label="Close">
      {translate('frontend/src/lib/components/PromptModal.svelte::close_button')}
    </button>
  </form>
</dialog>

<style>
  :global(.modal-box) {
    font-family: 'Plus Jakarta Sans', sans-serif;
  }
</style>

<script lang="ts">
  import { tick } from 'svelte';
  import { t, translator } from '$lib/i18n';

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
    icon?: string;
    helpText?: string;
    inputType?: string;
    selectOnOpen?: boolean;
    maxLength?: number;
    allowEmpty?: boolean;
    validate?: (value: string) => string | null;
    transform?: (value: string) => string;
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
  let cancelClass = 'btn';
  let icon: string | undefined;
  let helpText = '';
  let inputType = 'text';
  let maxLength: number | undefined;
  let error = '';
  let allowEmpty = false;

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
    cancelClass = options.cancelClass ?? 'btn';
    icon = options.icon;
    helpText = options.helpText ?? '';
    inputType = options.inputType ?? 'text';
    maxLength = options.maxLength;
    allowEmpty = options.allowEmpty ?? false;
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
    if (!allowEmpty && (inputValue === null || inputValue === undefined || inputValue.trim() === '')) {
      // Treat empty input as cancel to match legacy prompt behaviour
      settle(null);
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

<dialog bind:this={dialog} class="modal" on:close={handleClose} on:cancel={handleDialogCancel}>
  <form class="modal-box space-y-4" on:submit={handleConfirm}>
    <div class="flex items-start gap-3">
      {#if icon}
        <div class="mt-1 text-2xl">
          <i class={icon}></i>
        </div>
      {/if}
      <div class="flex-1">
        <h2 class="font-semibold text-lg">{title}</h2>
        {#if body}
          <p class="mt-2 text-sm text-base-content/80 whitespace-pre-line">{body}</p>
        {/if}
      </div>
    </div>

    <div class="form-control">
      <label class="label"><span class="label-text font-semibold">{label}</span></label>
      <input
        bind:this={inputEl as HTMLInputElement}
        type={inputType}
        class="input input-bordered w-full"
        bind:value
        placeholder={placeholder}
        maxlength={maxLength}
      />
      {#if helpText}
        <span class="label-text-alt mt-1 text-xs text-base-content/60">{helpText}</span>
      {/if}
      {#if error}
        <span class="text-error text-sm mt-1">{error}</span>
      {/if}
    </div>

    <div class="modal-action">
      <button type="button" class={cancelClass} on:click={handleCancel}>{cancelLabel}</button>
      <button type="submit" class={confirmClass}>{confirmLabel}</button>
    </div>
  </form>
  <form method="dialog" class="modal-backdrop" on:submit={handleCancel}><button aria-label="Close">{translate('frontend/src/lib/components/PromptModal.svelte::close_button')}</button></form>
</dialog>

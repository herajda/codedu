<script lang="ts">
  import { tick } from 'svelte';
  import { t, translator } from '$lib/i18n';

  export type UnsavedChangesAction = 'save' | 'discard' | null;

  export type UnsavedChangesModalOptions = {
    title?: string;
    body?: string;
    saveLabel?: string;
    discardLabel?: string;
    cancelLabel?: string;
    icon?: string;
  };

  let dialog: HTMLDialogElement | undefined;
  let resolver: ((value: UnsavedChangesAction) => void) | null = null;

  let translate;
  $: translate = $translator;

  let title = t('frontend/src/lib/components/UnsavedChangesModal.svelte::unsaved_changes_title');
  let body = t('frontend/src/lib/components/UnsavedChangesModal.svelte::unsaved_changes_body');
  let saveLabel = t('frontend/src/lib/components/UnsavedChangesModal.svelte::save_and_leave_button');
  let discardLabel = t('frontend/src/lib/components/UnsavedChangesModal.svelte::leave_without_saving_button');
  let cancelLabel = t('frontend/src/lib/components/UnsavedChangesModal.svelte::cancel_button');
  let icon: string | undefined;

  export async function open(options: UnsavedChangesModalOptions = {}): Promise<UnsavedChangesAction> {
    title = options.title ?? translate('frontend/src/lib/components/UnsavedChangesModal.svelte::unsaved_changes_title');
    body = options.body ?? translate('frontend/src/lib/components/UnsavedChangesModal.svelte::unsaved_changes_body');
    saveLabel = options.saveLabel ?? translate('frontend/src/lib/components/UnsavedChangesModal.svelte::save_and_leave_button');
    discardLabel = options.discardLabel ?? translate('frontend/src/lib/components/UnsavedChangesModal.svelte::leave_without_saving_button');
    cancelLabel = options.cancelLabel ?? translate('frontend/src/lib/components/UnsavedChangesModal.svelte::cancel_button');
    icon = options.icon;

    await tick();
    if (!dialog) throw new Error('UnsavedChangesModal not mounted');
    dialog.showModal();

    return new Promise<UnsavedChangesAction>((resolve) => {
      resolver = resolve;
    });
  }

  function settle(result: UnsavedChangesAction) {
    if (!dialog) return;
    if (resolver) {
      const resolve = resolver;
      resolver = null;
      resolve(result);
    }
    if (dialog.open) dialog.close();
  }

  function handleCancel(event: Event) {
    event.preventDefault();
    settle(null);
  }

  function handleDialogCancel(event: Event) {
    event.preventDefault();
    settle(null);
  }
</script>

<dialog bind:this={dialog} class="modal" on:cancel={handleDialogCancel}>
  <div class="modal-box space-y-4">
    <div class="flex items-start gap-3">
      {#if icon}
        <div class="mt-1 text-2xl">
          <i class={icon}></i>
        </div>
      {/if}
      <div>
        <h2 class="font-semibold text-lg">{title}</h2>
        <p class="mt-2 text-sm text-base-content/80 whitespace-pre-line">{body}</p>
      </div>
    </div>
    <div class="modal-action justify-between">
      <button class="btn" on:click={() => settle(null)}>{cancelLabel}</button>
      <div class="flex gap-2">
        <button class="btn" on:click={() => settle('discard')}>{discardLabel}</button>
        <button class="btn btn-primary" on:click={() => settle('save')}>{saveLabel}</button>
      </div>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop" on:submit={handleCancel}><button aria-label={translate('frontend/src/lib/components/UnsavedChangesModal.svelte::close_button')}>{translate('frontend/src/lib/components/UnsavedChangesModal.svelte::close_button')}</button></form>
</dialog>

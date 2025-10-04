<script lang="ts">
  import { tick } from 'svelte';

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

  let title = 'Unsaved changes';
  let body = 'You have unsaved changes. What would you like to do?';
  let saveLabel = 'Save and leave';
  let discardLabel = 'Leave without saving';
  let cancelLabel = 'Cancel';
  let icon: string | undefined;

  export async function open(options: UnsavedChangesModalOptions = {}): Promise<UnsavedChangesAction> {
    title = options.title ?? 'Unsaved changes';
    body = options.body ?? 'You have unsaved changes. What would you like to do?';
    saveLabel = options.saveLabel ?? 'Save and leave';
    discardLabel = options.discardLabel ?? 'Leave without saving';
    cancelLabel = options.cancelLabel ?? 'Cancel';
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
  <form method="dialog" class="modal-backdrop" on:submit={handleCancel}><button aria-label="Close">close</button></form>
</dialog>

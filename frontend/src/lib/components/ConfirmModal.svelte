<script lang="ts">
  import { tick } from 'svelte';

  export type ConfirmModalOptions = {
    title?: string;
    body?: string;
    confirmLabel?: string;
    cancelLabel?: string;
    confirmClass?: string;
    cancelClass?: string;
    icon?: string;
  };

  let dialog: HTMLDialogElement | undefined;
  let resolver: ((value: boolean) => void) | null = null;

  let title = 'Are you sure?';
  let body = '';
  let confirmLabel = 'Confirm';
  let cancelLabel = 'Cancel';
  let confirmClass = 'btn btn-primary';
  let cancelClass = 'btn';
  let icon: string | undefined;

  export async function open(options: ConfirmModalOptions = {}): Promise<boolean> {
    title = options.title ?? 'Are you sure?';
    body = options.body ?? '';
    confirmLabel = options.confirmLabel ?? 'Confirm';
    cancelLabel = options.cancelLabel ?? 'Cancel';
    confirmClass = options.confirmClass ?? 'btn btn-primary';
    cancelClass = options.cancelClass ?? 'btn';
    icon = options.icon;

    await tick();
    if (!dialog) throw new Error('ConfirmModal not mounted');
    dialog.showModal();

    return new Promise<boolean>((resolve) => {
      resolver = resolve;
    });
  }

  function settle(result: boolean) {
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
      resolve(false);
    }
  }

  function handleCancel(event: Event) {
    event.preventDefault();
    settle(false);
  }

  function handleConfirm(event: Event) {
    event.preventDefault();
    settle(true);
  }

  function handleDialogCancel(event: Event) {
    event.preventDefault();
    settle(false);
  }
</script>

<dialog bind:this={dialog} class="modal" on:close={handleClose} on:cancel={handleDialogCancel}>
  <div class="modal-box space-y-4">
    <div class="flex items-start gap-3">
      {#if icon}
        <div class="mt-1 text-2xl">
          <i class={icon}></i>
        </div>
      {/if}
      <div>
        <h2 class="font-semibold text-lg">{title}</h2>
        {#if body}
          <p class="mt-2 text-sm text-base-content/80 whitespace-pre-line">{body}</p>
        {/if}
      </div>
    </div>
    <div class="modal-action">
      <button class={cancelClass} on:click={handleCancel}>{cancelLabel}</button>
      <button class={confirmClass} on:click={handleConfirm}>{confirmLabel}</button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop" on:submit={handleCancel}><button aria-label="Close">close</button></form>
</dialog>

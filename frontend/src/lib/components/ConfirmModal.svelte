<script lang="ts">
  import { tick } from 'svelte';
  import { t } from '$lib/i18n';
  import { X } from 'lucide-svelte';

  export type ConfirmModalOptions = {
    title?: string;
    body?: string;
    confirmLabel?: string;
    cancelLabel?: string;
    confirmClass?: string;
    cancelClass?: string;
    icon?: any;
  };

  let dialog: HTMLDialogElement | undefined;
  let resolver: ((value: boolean) => void) | null = null;

  let title = t('frontend/src/lib/components/ConfirmModal.svelte::are_you_sure');
  let body = '';
  let confirmLabel = t('frontend/src/lib/components/ConfirmModal.svelte::confirm');
  let cancelLabel = t('frontend/src/lib/components/ConfirmModal.svelte::cancel');
  let confirmClass = 'btn btn-primary';
  let cancelClass = 'btn btn-ghost';
  let icon: any;

  export async function open(options: ConfirmModalOptions = {}): Promise<boolean> {
    title = options.title ?? t('frontend/src/lib/components/ConfirmModal.svelte::are_you_sure');
    body = options.body ?? '';
    confirmLabel = options.confirmLabel ?? t('frontend/src/lib/components/ConfirmModal.svelte::confirm');
    cancelLabel = options.cancelLabel ?? t('frontend/src/lib/components/ConfirmModal.svelte::cancel');
    confirmClass = options.confirmClass ?? 'btn btn-primary';
    cancelClass = options.cancelClass ?? 'btn btn-ghost hover:bg-base-200';
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
    <div class="p-8 pt-2 pb-6 space-y-6">
      {#if body}
        <p class="text-sm font-bold text-base-content/70 leading-relaxed whitespace-pre-line">{body}</p>
      {/if}

      <!-- Actions -->
      <div class="flex items-center justify-end gap-3 pt-2">
        <button 
          class="h-11 px-6 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all {cancelClass}" 
          on:click={handleCancel}
        >
          {cancelLabel}
        </button>
        <button 
          class="h-11 px-8 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all shadow-lg shadow-primary/20 hover:shadow-primary/30 hover:scale-[1.02] active:scale-[0.98] {confirmClass}" 
          on:click={handleConfirm}
        >
          {confirmLabel}
        </button>
      </div>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop bg-base-content/20 backdrop-blur-sm" on:submit={handleCancel}><button aria-label="Close">{t('frontend/src/lib/components/ConfirmModal.svelte::close')}</button></form>
</dialog>

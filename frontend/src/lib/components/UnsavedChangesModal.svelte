<script lang="ts">
  import { tick } from 'svelte';
  import { t, translator } from '$lib/i18n';
  import { AlertTriangle, X } from 'lucide-svelte';

  export type UnsavedChangesAction = 'save' | 'discard' | null;

  export type UnsavedChangesModalOptions = {
    title?: string;
    body?: string;
    saveLabel?: string;
    discardLabel?: string;
    cancelLabel?: string;
    icon?: any;
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
  let icon: any = AlertTriangle;

  export async function open(options: UnsavedChangesModalOptions = {}): Promise<UnsavedChangesAction> {
    title = options.title ?? translate('frontend/src/lib/components/UnsavedChangesModal.svelte::unsaved_changes_title');
    body = options.body ?? translate('frontend/src/lib/components/UnsavedChangesModal.svelte::unsaved_changes_body');
    saveLabel = options.saveLabel ?? translate('frontend/src/lib/components/UnsavedChangesModal.svelte::save_and_leave_button');
    discardLabel = options.discardLabel ?? translate('frontend/src/lib/components/UnsavedChangesModal.svelte::leave_without_saving_button');
    cancelLabel = options.cancelLabel ?? translate('frontend/src/lib/components/UnsavedChangesModal.svelte::cancel_button');
    icon = options.icon ?? AlertTriangle;

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
  <div class="modal-box bg-base-100 rounded-[2.5rem] border border-base-200 shadow-2xl p-0 overflow-hidden max-w-md w-full">
    <!-- Header with Gradient Background -->
    <div class="bg-gradient-to-br from-warning/5 via-transparent to-transparent p-8 pb-4">
      <div class="flex items-start justify-between gap-4">
         <div class="flex items-center gap-4">
            <div class="w-12 h-12 rounded-2xl bg-warning/10 text-warning flex items-center justify-center shadow-sm shrink-0">
              {#if typeof icon === 'string'}
                <i class={icon}></i>
              {:else}
                <svelte:component this={icon} size={24} />
              {/if}
            </div>
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
      <p class="text-sm font-bold text-base-content/70 leading-relaxed whitespace-pre-line">{body}</p>

      <!-- Actions -->
      <div class="flex flex-col gap-3 pt-2">
        <button 
          class="btn btn-primary w-full h-11 px-8 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all shadow-lg shadow-primary/20 hover:shadow-primary/30 hover:scale-[1.02] active:scale-[0.98]" 
          on:click={() => settle('save')}
        >
          {saveLabel}
        </button>
        <button 
          class="btn btn-error btn-outline w-full h-11 px-8 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all border-error/30 hover:bg-error hover:text-white" 
          on:click={() => settle('discard')}
        >
          {discardLabel}
        </button>
        <button 
          class="btn btn-ghost w-full h-11 px-6 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all opacity-60 hover:opacity-100" 
          on:click={() => settle(null)}
        >
          {translate('frontend/src/lib/components/UnsavedChangesModal.svelte::cancel_button')}
        </button>
      </div>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop bg-base-content/20 backdrop-blur-sm" on:submit={handleCancel}><button aria-label={translate('frontend/src/lib/components/UnsavedChangesModal.svelte::close_button')}>{translate('frontend/src/lib/components/UnsavedChangesModal.svelte::close_button')}</button></form>
</dialog>

<script lang="ts">
  import { tick } from 'svelte';
  import { t, translator } from '$lib/i18n';
  import { X, Calendar, Clock, User, FileText, Trash2, Check } from 'lucide-svelte';
  import { DeadlinePicker } from "$lib";

  export type ExtendDeadlineResult = {
    newDeadline: string;
    note: string;
    clear: boolean;
  };

  let translate;
  $: translate = $translator;

  let dialog: HTMLDialogElement | undefined;
  let deadlinePicker: InstanceType<typeof DeadlinePicker>;
  let resolver: ((value: ExtendDeadlineResult | null) => void) | null = null;

  let student: any = null;
  let deadline = "";
  let note = "";
  let hasExisting = false;

  export async function open(s: any, existing: any, defaultDeadline: string): Promise<ExtendDeadlineResult | null> {
    student = s;
    hasExisting = !!existing;
    deadline = existing ? String(existing.new_deadline).slice(0, 16) : (defaultDeadline?.slice(0, 16) || "");
    note = existing?.note || "";

    await tick();
    if (!dialog) throw new Error('ExtendDeadlineModal not mounted');
    dialog.showModal();

    return new Promise<ExtendDeadlineResult | null>((resolve) => {
      resolver = resolve;
    });
  }

  function settle(result: ExtendDeadlineResult | null) {
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

  async function pickDeadline() {
    const picked = await deadlinePicker.open({
      title: t("frontend/src/routes/assignments/[id]/+page.svelte::select_new_deadline"),
      initial: deadline,
    });
    if (picked) {
      deadline = picked;
    }
  }

  function handleSave() {
    if (!student || !deadline) return;
    settle({ newDeadline: deadline, note: note.trim(), clear: false });
  }

  function handleClear() {
    settle({ newDeadline: "", note: "", clear: true });
  }

  function handleCancel() {
    settle(null);
  }

  function handleClose() {
    if (resolver) {
      const resolve = resolver;
      resolver = null;
      resolve(null);
    }
  }

  function euLabel(d: string): string {
    if (!d) return "";
    const [date, time] = d.split('T');
    if (!date) return "";
    const parts = date.split('-');
    if (parts.length !== 3) return d;
    const [y, m, day] = parts;
    return `${day}. ${m}. ${y} ${time || ""}`;
  }
</script>

<dialog 
  bind:this={dialog} 
  class="modal" 
  on:close={handleClose} 
  on:cancel|preventDefault={handleCancel}
>
  <div class="modal-box bg-base-100 rounded-[2.5rem] border border-base-200 shadow-2xl p-0 overflow-hidden max-w-md w-full">
    <!-- Header with Gradient Background -->
    <div class="bg-gradient-to-br from-primary/5 via-transparent to-transparent p-8 pb-4">
      <div class="flex items-start justify-between gap-4">
         <div class="flex items-center gap-4">
            <div class="w-12 h-12 rounded-2xl bg-primary/10 text-primary flex items-center justify-center shadow-sm shrink-0">
              <Clock size={24} />
            </div>
            <div>
              <h2 class="text-xl font-black tracking-tight text-base-content whitespace-nowrap">
                {t("frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_heading")}
              </h2>
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
    <div class="p-8 pt-4 space-y-6">
      <!-- Student Info -->
      <div class="form-control w-full">
        <label class="label pt-0 px-1">
          <span class="label-text font-black text-[10px] uppercase tracking-[0.15em] opacity-40">
            {t("frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_student_label")}
          </span>
        </label>
        <div class="flex items-center gap-3 p-4 bg-base-200/50 rounded-2xl border border-base-300/30">
          <div class="w-10 h-10 rounded-full bg-base-100 flex items-center justify-center text-base-content/40 border border-base-300/50 shadow-sm">
            <User size={20} />
          </div>
          <div class="flex flex-col">
            <span class="text-sm font-bold text-base-content">
              {student?.name ?? (student?.email?.split('@')[0] || "Student")}
            </span>
            <span class="text-[10px] font-medium opacity-40 truncate max-w-[200px]">
              {student?.email || ""}
            </span>
          </div>
        </div>
      </div>

      <!-- Deadline Picker -->
      <div class="form-control w-full">
        <label class="label pt-0 px-1">
          <span class="label-text font-black text-[10px] uppercase tracking-[0.15em] opacity-40">
            {t("frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_new_deadline_label")}
          </span>
        </label>
        <div class="flex items-center gap-2">
          <div class="relative flex-grow group">
            <div class="absolute left-4 top-1/2 -translate-y-1/2 text-primary/40 group-focus-within:text-primary transition-colors">
              <Calendar size={18} />
            </div>
            <input
              class="input w-full h-12 bg-base-200/50 border-base-300/50 focus:border-primary focus:bg-base-100 rounded-xl font-mono text-xs font-bold transition-all duration-300 pl-11"
              readonly
              placeholder="dd. mm. yyyy hh:mm"
              value={euLabel(deadline)}
              on:click={pickDeadline}
            />
          </div>
          <button 
            type="button"
            class="h-12 px-4 rounded-xl bg-primary/10 text-primary hover:bg-primary hover:text-primary-content font-black uppercase tracking-widest text-[10px] transition-all shadow-sm flex items-center gap-2" 
            on:click={pickDeadline}
          >
            {t("frontend/src/routes/assignments/[id]/+page.svelte::pick_button")}
          </button>
        </div>
      </div>

      <!-- Note -->
      <div class="form-control w-full">
        <label class="label pt-0 px-1">
          <span class="label-text font-black text-[10px] uppercase tracking-[0.15em] opacity-40">
            {t("frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_note_label")}
          </span>
        </label>
        <div class="relative group">
          <div class="absolute left-4 top-4 text-base-content/20 group-focus-within:text-primary transition-colors">
            <FileText size={18} />
          </div>
          <textarea
            class="textarea w-full min-h-[100px] bg-base-200/50 border-base-300/50 focus:border-primary focus:bg-base-100 rounded-2xl font-medium transition-all duration-300 pl-11 pt-3.5 resize-none"
            placeholder={t("frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_note_placeholder")}
            bind:value={note}
          ></textarea>
        </div>
      </div>

      <!-- Actions -->
      <div class="flex items-center justify-between gap-3 pt-2">
        <div class="flex items-center gap-2">
          {#if hasExisting}
            <button 
              type="button" 
              class="h-11 px-4 rounded-xl bg-error/10 text-error hover:bg-error hover:text-error-content font-black uppercase tracking-widest text-[10px] transition-all flex items-center gap-2" 
              on:click={handleClear}
            >
              <Trash2 size={16} />
              {t("frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_clear_button")}
            </button>
          {/if}
        </div>
        
        <div class="flex items-center gap-3">
          <button 
            type="button" 
            class="h-11 px-6 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all btn btn-ghost hover:bg-base-200" 
            on:click={handleCancel}
          >
            {t("frontend/src/lib/components/ConfirmModal.svelte::cancel")}
          </button>
          <button 
            type="button" 
            class="h-11 px-8 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all shadow-lg shadow-primary/20 hover:shadow-primary/30 hover:scale-[1.02] active:scale-[0.98] btn btn-primary flex items-center gap-2" 
            on:click={handleSave}
            disabled={!deadline}
          >
            <Check size={16} />
            {t("frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_save_button")}
          </button>
        </div>
      </div>
    </div>
  </div>
  
  <form method="dialog" class="modal-backdrop bg-base-content/20 backdrop-blur-sm" on:submit={handleCancel}>
    <button aria-label="Close">
      {t('frontend/src/lib/components/ConfirmModal.svelte::close')}
    </button>
  </form>
</dialog>

<DeadlinePicker bind:this={deadlinePicker} />

<style>
  :global(.modal-box) {
    font-family: 'Plus Jakarta Sans', sans-serif;
  }
</style>

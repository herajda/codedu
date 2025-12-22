<script context="module" lang="ts">
  export type Offset = { label: string; minutes?: number; hours?: number; days?: number; weeks?: number };
  
  export type DeadlinePickerOptions = {
    title?: string;
    initial?: string | number | Date | null;
    min?: string | number | Date | null;
    // Show quick relative shortcuts like +1 day, +1 week, etc.
    shortcuts?: Offset[];
  };
</script>

<script lang="ts">
  import { tick, onMount } from 'svelte';
  import { browser } from '$app/environment';
  import { t, translator } from '$lib/i18n';
  import { 
    Calendar, 
    Clock, 
    Zap, 
    Keyboard, 
    Check, 
    X,
    ChevronLeft,
    ChevronRight,
    ArrowRight
  } from "lucide-svelte";

  let translate;
  $: translate = $translator;

  onMount(async () => {
    if (!browser) return;
    await import('cally');
  });


  let dialog: HTMLDialogElement | undefined;
  let resolver: ((value: string | null) => void) | null = null;

  let title = t('frontend/src/lib/components/DeadlinePicker.svelte::select_deadline_title');
  let minDate: Date | null = null;

  // Internal state
  let selected = new Date();
  let hour = selected.getHours();
  let minute = selected.getMinutes();
  let manual = '';

  let calendarValue = '';
  const todayValue = toCalendarValue(new Date());

  const defaultShortcuts: Offset[] = [
    { label: t('frontend/src/lib/components/DeadlinePicker.svelte::plus_one_hour_shortcut'), hours: 1 },
    { label: t('frontend/src/lib/components/DeadlinePicker.svelte::in_one_day_shortcut'), days: 1 },
    { label: t('frontend/src/lib/components/DeadlinePicker.svelte::in_three_days_shortcut'), days: 3 },
    { label: t('frontend/src/lib/components/DeadlinePicker.svelte::in_one_week_shortcut'), weeks: 1 },
    { label: t('frontend/src/lib/components/DeadlinePicker.svelte::in_two_weeks_shortcut'), weeks: 2 }
  ];
  let shortcuts: Offset[] = defaultShortcuts;

  // Public API
  export async function open(options: DeadlinePickerOptions = {}): Promise<string | null> {
    title = options.title ?? t('frontend/src/lib/components/DeadlinePicker.svelte::select_deadline_title');
    const init = ensureDate(options.initial) ?? new Date();
    minDate = ensureDate(options.min) ?? null;
    selected = new Date(init);
    hour = clamp0_23(selected.getHours());
    minute = clamp0_59(selected.getMinutes());
    shortcuts = options.shortcuts && options.shortcuts.length ? options.shortcuts : defaultShortcuts;

    syncManual();
    await tick();
    if (!dialog) throw new Error('DeadlinePicker not mounted');
    dialog.showModal();
    return new Promise<string | null>((resolve) => {
      resolver = resolve;
    });
  }

  function ensureDate(v: string | number | Date | null | undefined): Date | null {
    if (v === null || v === undefined) return null;
    const d = new Date(v);
    return Number.isNaN(d.getTime()) ? null : d;
  }

  function formatTwo(n: number): string { return String(n).padStart(2, '0'); }
  function clamp0_23(n: number) { return Math.max(0, Math.min(23, Math.floor(n))); }
  function clamp0_59(n: number) { return Math.max(0, Math.min(59, Math.floor(n))); }

  function toCalendarValue(d: Date): string {
    const y = d.getFullYear();
    const m = formatTwo(d.getMonth() + 1);
    const day = formatTwo(d.getDate());
    return `${y}-${m}-${day}`;
  }

  function toLocalYMDHM(d: Date): string {
    const y = d.getFullYear();
    const m = formatTwo(d.getMonth() + 1);
    const day = formatTwo(d.getDate());
    const h = formatTwo(hour);
    const min = formatTwo(minute);
    return `${y}-${m}-${day}T${h}:${min}`;
  }

  function euDateLabel(d: Date): string {
    const day = formatTwo(d.getDate());
    const mon = formatTwo(d.getMonth() + 1);
    const y = d.getFullYear();
    return `${day}. ${mon}. ${y}`;
  }

  function currentLabel(): string {
    return `${euDateLabel(selected)} ${formatTwo(hour)}:${formatTwo(minute)}`;
  }

  function syncManual() {
    manual = currentLabel();
    calendarValue = toCalendarValue(selected);
  }

  function settle(result: string | null) {
    if (!dialog) return;
    if (resolver) {
      const resolve = resolver;
      resolver = null;
      resolve(result);
    }
    if (dialog.open) dialog.close();
  }

  function handleCancel(ev: Event) {
    ev.preventDefault();
    settle(null);
  }
  function handleClose() {
    if (resolver) { const r = resolver; resolver = null; r(null); }
  }

  function confirm(ev: Event) {
    ev.preventDefault();
    // Construct final date from selected date + hour/minute
    const d = new Date(selected);
    d.setHours(hour, minute, 0, 0);
    // Respect minDate if set
    if (minDate && d < minDate) {
      // Snap to minDate if earlier
      const md = new Date(minDate);
      hour = md.getHours();
      minute = md.getMinutes();
      selected = new Date(md);
      // keep open and just update UI
      return;
    }
    syncManual();
    settle(toLocalYMDHM(d));
  }

  function applyPresetTime(hh: number, mm: number) {
    hour = clamp0_23(hh); minute = clamp0_59(mm);
    syncManual();
  }

  function applyShortcut(off: Offset) {
    const base = new Date();
    const d = new Date(base);
    if (off.minutes) d.setMinutes(d.getMinutes() + off.minutes);
    if (off.hours) d.setHours(d.getHours() + off.hours);
    if (off.days) d.setDate(d.getDate() + off.days);
    if (off.weeks) d.setDate(d.getDate() + (off.weeks * 7));
    selected = d;
    hour = d.getHours();
    minute = d.getMinutes();
    syncManual();
  }

  function onHourInput(e: Event) {
    const v = parseInt((e.target as HTMLInputElement).value.replace(/[^0-9]/g, ''), 10);
    hour = isNaN(v) ? 0 : clamp0_23(v);
    syncManual();
  }
  function onMinuteInput(e: Event) {
    const v = parseInt((e.target as HTMLInputElement).value.replace(/[^0-9]/g, ''), 10);
    minute = isNaN(v) ? 0 : clamp0_59(v);
    syncManual();
  }

  function parseManualInput(value: string) {
    // Flexible regex for European dates with dots or slashes
    // Supports dd.mm.yyyy, dd. mm. yyyy, dd/mm/yyyy
    const trimmed = value.trim();
    const m = trimmed.match(/^([0-9]{1,2})[\.\/]\s*([0-9]{1,2})[\.\/]\s*([0-9]{2,4})\s+([0-9]{1,2}):([0-9]{2})$/);
    if (!m) return;
    
    const dd = parseInt(m[1], 10);
    const mm = parseInt(m[2], 10);
    let yyyy = parseInt(m[3], 10);
    const hh = parseInt(m[4], 10);
    const min = parseInt(m[5], 10);
    
    const y = yyyy < 100 ? 2000 + yyyy : yyyy;
    const d = new Date(y, (mm - 1), dd, hh, min, 0, 0);
    if (!Number.isNaN(d.getTime())) {
      selected = d;
      hour = clamp0_23(hh);
      minute = clamp0_59(min);
      calendarValue = toCalendarValue(selected);
    }
  }

  function handleCalendarChange(event: Event) {
    const target = event.currentTarget as HTMLElement & { value?: string };
    const value = target?.value;
    if (!value) return;
    const [y, m, d] = value.split('-').map((part) => parseInt(part, 10));
    if ([y, m, d].some((part) => Number.isNaN(part))) return;
    const next = new Date(selected);
    next.setFullYear(y, m - 1, d);
    selected = next;
    syncManual();
  }
</script>

<dialog bind:this={dialog} class="modal" on:close={handleClose} on:cancel|preventDefault={() => settle(null)}>
  <div class="modal-box max-w-2xl p-0 overflow-hidden border-none shadow-2xl rounded-2xl">
    <!-- Header with Gradient -->
    <div class="bg-gradient-to-r from-primary/10 via-base-200 to-secondary/10 p-5 border-b border-base-300/50">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div class="p-2.5 rounded-xl bg-primary text-primary-content shadow-lg shadow-primary/20">
            <Calendar size={20} />
          </div>
          <div>
            <h3 class="font-black text-lg tracking-tight leading-tight">{title}</h3>
            <div class="flex items-center gap-1.5 mt-0.5 whitespace-nowrap overflow-hidden">
               <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/components/DeadlinePicker.svelte::currently_selected')}</span>
               <span class="text-xs font-bold font-mono text-primary truncate">{manual}</span>
            </div>
          </div>
        </div>
        <form method="dialog">
          <button class="btn btn-sm btn-ghost btn-circle hover:bg-base-300/50" aria-label={t('frontend/src/lib/components/DeadlinePicker.svelte::close_button_label')}>
            <X size={18} />
          </button>
        </form>
      </div>
    </div>

    <div class="p-5">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <!-- Calendar Section -->
        <div class="space-y-3">
          <div class="flex items-center gap-2 px-1">
            <div class="p-1.5 rounded-lg bg-primary/10 text-primary">
              <Calendar size={14} />
            </div>
            <h4 class="font-black text-[10px] uppercase tracking-widest opacity-60">
              {t('frontend/src/lib/components/DeadlinePicker.svelte::date_section')}
            </h4>
          </div>

          <div class="bg-base-200/40 rounded-xl p-3 border border-base-300/30">
            <calendar-date
              class="cally w-full"
              value={calendarValue}
              min={minDate ? toCalendarValue(minDate) : undefined}
              today={todayValue}
              firstDayOfWeek={1}
              showOutsideDays={true}
              on:change={handleCalendarChange}
            >
              <div slot="previous">
                <ChevronLeft size={16} />
              </div>
              <div slot="next">
                <ChevronRight size={16} />
              </div>
              <calendar-month class="w-full"></calendar-month>
            </calendar-date>
          </div>
        </div>

        <!-- Time & shortcuts Section -->
        <div class="space-y-5">
          <!-- Time Picker -->
          <div class="space-y-3">
            <div class="flex items-center gap-2 px-1">
              <div class="p-1.5 rounded-lg bg-secondary/10 text-secondary">
                <Clock size={14} />
              </div>
              <h4 class="font-black text-[10px] uppercase tracking-widest opacity-60">
                {translate('frontend/src/lib/components/DeadlinePicker.svelte::time_24h_label')}
              </h4>
            </div>
            
            <div class="bg-base-200/40 rounded-xl p-3 border border-base-300/30">
              <div class="flex items-center justify-center gap-3">
                <div class="join shadow-sm border border-base-300/20">
                  <button class="btn btn-sm join-item bg-base-100 hover:bg-base-200 border-none px-2" type="button" on:click={() => hour = clamp0_23(hour - 1)}>-</button>
                  <input class="input input-sm w-12 text-center join-item bg-base-100 border-x border-base-300/20 font-bold font-mono p-0" value={formatTwo(hour)} on:input={onHourInput} />
                  <button class="btn btn-sm join-item bg-base-100 hover:bg-base-200 border-none px-2" type="button" on:click={() => hour = clamp0_23(hour + 1)}>+</button>
                </div>
                <span class="font-black opacity-30 animate-pulse">:</span>
                <div class="join shadow-sm border border-base-300/20">
                  <button class="btn btn-sm join-item bg-base-100 hover:bg-base-200 border-none px-2" type="button" on:click={() => minute = clamp0_59(minute - 5)}>-</button>
                  <input class="input input-sm w-12 text-center join-item bg-base-100 border-x border-base-300/20 font-bold font-mono p-0" value={formatTwo(minute)} on:input={onMinuteInput} />
                  <button class="btn btn-sm join-item bg-base-100 hover:bg-base-200 border-none px-2" type="button" on:click={() => minute = clamp0_59(minute + 5)}>+</button>
                </div>
              </div>

              <div class="grid grid-cols-4 gap-1.5 mt-3">
                {#each ["08:00", "12:00", "17:00", "23:59"] as time}
                  <button 
                    type="button" 
                    class="btn btn-xs btn-ghost bg-base-100/50 hover:bg-primary/10 hover:text-primary border border-base-300/30 font-bold" 
                    on:click={() => applyPresetTime(parseInt(time.split(':')[0]), parseInt(time.split(':')[1]))}
                  >
                    {time}
                  </button>
                {/each}
              </div>
            </div>
          </div>

          <!-- Shortcuts -->
          <div class="space-y-3">
            <div class="flex items-center gap-2 px-1">
              <div class="p-1.5 rounded-lg bg-accent/10 text-accent">
                <Zap size={14} />
              </div>
              <h4 class="font-black text-[10px] uppercase tracking-widest opacity-60">
                {translate('frontend/src/lib/components/DeadlinePicker.svelte::shortcuts_label')}
              </h4>
            </div>

            <div class="flex flex-wrap gap-1.5">
              {#each shortcuts as s}
                <button type="button" class="btn btn-xs bg-base-200/50 border-base-300/30 hover:bg-accent/10 hover:text-accent font-medium rounded-lg" on:click={() => applyShortcut(s)}>{s.label}</button>
              {/each}
              <button type="button" class="btn btn-xs bg-base-200/50 border-base-300/30 hover:bg-accent/10 hover:text-accent font-medium rounded-lg px-2" on:click={() => { const d=new Date(); selected=d; hour=d.getHours(); minute=d.getMinutes(); syncManual(); }}>{translate('frontend/src/lib/components/DeadlinePicker.svelte::now_button')}</button>
            </div>
          </div>

          <!-- Manual Entry -->
          <div class="space-y-3">
            <div class="flex items-center gap-2 px-1">
              <div class="p-1.5 rounded-lg bg-info/10 text-info">
                <Keyboard size={14} />
              </div>
              <h4 class="font-black text-[10px] uppercase tracking-widest opacity-60">
                {translate('frontend/src/lib/components/DeadlinePicker.svelte::manual_entry_label')}
              </h4>
              <span class="text-[9px] font-bold opacity-30 ml-auto uppercase tracking-tighter">{translate('frontend/src/lib/components/DeadlinePicker.svelte::date_format_hint')}</span>
            </div>
            <div class="relative group">
              <input 
                class="input input-bordered input-sm w-full bg-base-100/50 focus:bg-base-100 transition-all font-mono text-xs pr-8" 
                placeholder={t('frontend/src/lib/components/DeadlinePicker.svelte::manual_entry_placeholder')} 
                bind:value={manual} 
                on:input={() => parseManualInput(manual)} 
                on:blur={() => manual = currentLabel()} 
              />
              <ArrowRight size={12} class="absolute right-3 top-1/2 -translate-y-1/2 opacity-20 group-focus-within:opacity-100 transition-opacity" />
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Actions -->
    <div class="modal-action bg-base-200/50 p-4 mt-0 border-t border-base-300/30 flex items-center justify-end gap-3">
      <button class="btn btn-sm btn-ghost hover:bg-base-300/50 gap-2 font-bold" type="button" on:click={handleCancel}>
        <X size={16} />
        {translate('frontend/src/lib/components/DeadlinePicker.svelte::cancel_button')}
      </button>
      <button class="btn btn-sm btn-primary shadow-lg shadow-primary/20 gap-2 font-bold" type="button" on:click={confirm}>
        <Check size={16} />
        {translate('frontend/src/lib/components/DeadlinePicker.svelte::set_deadline_button')}
      </button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button aria-label={t('frontend/src/lib/components/DeadlinePicker.svelte::close_modal_label')}>close</button></form>
</dialog>

<style>
  calendar-date.cally {
    --color-accent: oklch(var(--p));
    --color-text-on-accent: oklch(var(--pc));
    --color-text-muted: oklch(var(--bc) / 0.4);
    --color-bg: transparent;
    font-family: inherit;
    border: none;
  }

  calendar-month {
    --c-bg: transparent;
    padding: 0;
  }

  :global(calendar-month::part(day)) {
    font-size: 0.75rem;
    font-weight: 600;
    border-radius: 0.5rem;
    transition: all 0.2s;
  }
  
  :global(calendar-month::part(day):hover:not([aria-disabled="true"])) {
    background-color: oklch(var(--p) / 0.1);
    color: oklch(var(--p));
    cursor: pointer;
  }

  :global(calendar-month::part(day-today)) {
    color: oklch(var(--p));
    font-weight: 800;
    box-shadow: inset 0 0 0 1px oklch(var(--p) / 0.2);
  }

  :global(calendar-month::part(day-selected)) {
    background-color: oklch(var(--p)) !important;
    color: oklch(var(--pc)) !important;
    box-shadow: 0 4px 12px oklch(var(--p) / 0.3) !important;
  }

  :global(calendar-month::part(button)) {
    border-radius: 0.5rem;
    transition: all 0.2s;
  }
  
  /* Target navigation buttons specifically instead of generic button part */
  :global(calendar-month::part(previous):hover),
  :global(calendar-month::part(next):hover) {
    background-color: oklch(var(--p) / 0.1);
    color: oklch(var(--p));
  }

  :global(calendar-month::part(day-selected):hover) {
    background-color: oklch(var(--p)) !important;
    color: oklch(var(--pc)) !important;
    box-shadow: 0 4px 12px oklch(var(--p) / 0.3) !important;
  }

  :global(calendar-month::part(day-today):hover) {
    background-color: transparent !important;
    color: oklch(var(--p)) !important;
    box-shadow: inset 0 0 0 1px oklch(var(--p) / 0.2) !important;
  }
</style>


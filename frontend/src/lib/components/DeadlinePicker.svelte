<script lang="ts">
  import { tick } from 'svelte';
  import 'cally';

  type Offset = { label: string; minutes?: number; hours?: number; days?: number; weeks?: number };
  
  export type DeadlinePickerOptions = {
    title?: string;
    initial?: string | number | Date | null;
    min?: string | number | Date | null;
    // Show quick relative shortcuts like +1 day, +1 week, etc.
    shortcuts?: Offset[];
  };

  let dialog: HTMLDialogElement | undefined;
  let resolver: ((value: string | null) => void) | null = null;

  let title = 'Select deadline';
  let minDate: Date | null = null;

  // Internal state
  let selected = new Date();
  let hour = selected.getHours();
  let minute = selected.getMinutes();
  let manual = '';

  let calendarValue = '';
  const todayValue = toCalendarValue(new Date());

  const defaultShortcuts: Offset[] = [
    { label: '+1 hour', hours: 1 },
    { label: 'In 1 day', days: 1 },
    { label: 'In 3 days', days: 3 },
    { label: 'In 1 week', weeks: 1 },
    { label: 'In 2 weeks', weeks: 2 }
  ];
  let shortcuts: Offset[] = defaultShortcuts;

  // Public API
  export async function open(options: DeadlinePickerOptions = {}): Promise<string | null> {
    title = options.title ?? 'Select deadline';
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
    return `${day}/${mon}/${y}`;
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
    // Accept dd/mm/yyyy hh:mm (24h)
    const m = value.trim().match(/^([0-9]{1,2})\/[0-9]{1,2}\/[0-9]{2,4}\s+[0-9]{1,2}:[0-9]{2}$/);
    if (!m) return; // do nothing
    const [dateStr, timeStr] = value.trim().split(/\s+/);
    const [dd, mm, yyyy] = dateStr.split('/').map((x) => parseInt(x, 10));
    const [hh, min] = timeStr.split(':').map((x) => parseInt(x, 10));
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
  <div class="modal-box max-w-3xl">
    <div class="flex items-start justify-between gap-3 mb-2">
      <div>
        <h3 class="font-semibold text-lg">{title}</h3>
        <div class="text-sm opacity-70">{manual}</div>
      </div>
      <form method="dialog"><button class="btn btn-sm btn-ghost" aria-label="Close">✕</button></form>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <!-- Calendar -->
      <div>
        <calendar-date
          class="cally bg-base-100 border border-base-300 shadow-lg rounded-box"
          value={calendarValue}
          min={minDate ? toCalendarValue(minDate) : undefined}
          today={todayValue}
          firstDayOfWeek={1}
          showOutsideDays={true}
          on:change={handleCalendarChange}
        >
          <svg
            aria-label="Previous"
            class="fill-current size-4"
            slot="previous"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
          >
            <path fill="currentColor" d="M15.75 19.5 8.25 12l7.5-7.5"></path>
          </svg>
          <svg
            aria-label="Next"
            class="fill-current size-4"
            slot="next"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
          >
            <path fill="currentColor" d="m8.25 4.5 7.5 7.5-7.5 7.5"></path>
          </svg>
          <calendar-month></calendar-month>
        </calendar-date>
      </div>

      <!-- Time & shortcuts -->
      <div class="space-y-4">
        <div>
          <div class="label mb-1 py-0">
            <span class="label-text">Time (24h)</span>
          </div>
          <div class="flex items-center gap-2">
            <div class="join">
              <button class="btn btn-sm join-item" type="button" on:click={() => hour = clamp0_23(hour - 1)}>-</button>
              <input class="input input-bordered input-sm w-16 text-center join-item" value={formatTwo(hour)} on:input={onHourInput} aria-label="Hours" />
              <button class="btn btn-sm join-item" type="button" on:click={() => hour = clamp0_23(hour + 1)}>+</button>
            </div>
            <span class="opacity-70">:</span>
            <div class="join">
              <button class="btn btn-sm join-item" type="button" on:click={() => minute = clamp0_59(minute - 5)}>-</button>
              <input class="input input-bordered input-sm w-16 text-center join-item" value={formatTwo(minute)} on:input={onMinuteInput} aria-label="Minutes" />
              <button class="btn btn-sm join-item" type="button" on:click={() => minute = clamp0_59(minute + 5)}>+</button>
            </div>
          </div>
          <div class="flex flex-wrap gap-2 mt-2">
            <div class="join">
              <button type="button" class="btn btn-xs join-item" on:click={() => applyPresetTime(8,0)}>08:00</button>
              <button type="button" class="btn btn-xs join-item" on:click={() => applyPresetTime(12,0)}>12:00</button>
              <button type="button" class="btn btn-xs join-item" on:click={() => applyPresetTime(17,0)}>17:00</button>
              <button type="button" class="btn btn-xs join-item" on:click={() => applyPresetTime(23,59)}>23:59</button>
            </div>
          </div>
        </div>

        <div>
          <div class="label mb-1 py-0">
            <span class="label-text">Shortcuts</span>
          </div>
          <div class="flex flex-wrap gap-2">
            {#each shortcuts as s}
              <button type="button" class="btn btn-ghost btn-sm" on:click={() => applyShortcut(s)}>{s.label}</button>
            {/each}
            <button type="button" class="btn btn-ghost btn-sm" on:click={() => { const d=new Date(); selected=d; hour=d.getHours(); minute=d.getMinutes(); syncManual(); }}>Now</button>
            <button type="button" class="btn btn-ghost btn-sm" on:click={() => { const d=new Date(); d.setHours(23,59,0,0); selected=d; hour=23; minute=59; syncManual(); }}>Today 23:59</button>
          </div>
        </div>

        <div>
          <div class="label mb-1 py-0">
            <span class="label-text">Manual entry</span>
            <span class="label-text-alt">dd/mm/yyyy hh:mm</span>
          </div>
          <input class="input input-bordered w-full" placeholder="e.g. 25/12/2025 16:30" bind:value={manual} on:input={() => parseManualInput(manual)} on:blur={() => manual = currentLabel()} />
        </div>
      </div>
    </div>

    <div class="modal-action">
      <button class="btn" type="button" on:click={handleCancel}>Cancel</button>
      <button class="btn btn-primary" type="button" on:click={confirm}>Set deadline</button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button aria-label="Close">close</button></form>
</dialog>

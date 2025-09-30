<script lang="ts">
  import { tick } from 'svelte';

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
  let viewYear = selected.getFullYear();
  let viewMonth = selected.getMonth(); // 0-11
  let hour = selected.getHours();
  let minute = selected.getMinutes();
  let manual = '';
  let explicitSelectionKey: string | null = null;

  const dowShort = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

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
    const initDate = ensureDate(options.initial);
    const init = initDate ?? new Date();
    minDate = ensureDate(options.min) ?? null;
    selected = new Date(init);
    hour = clamp0_23(selected.getHours());
    minute = clamp0_59(selected.getMinutes());
    viewYear = selected.getFullYear();
    viewMonth = selected.getMonth();
    shortcuts = options.shortcuts && options.shortcuts.length ? options.shortcuts : defaultShortcuts;
    explicitSelectionKey = initDate ? dateKey(selected) : null;

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
      viewYear = selected.getFullYear();
      viewMonth = selected.getMonth();
      explicitSelectionKey = dateKey(selected);
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
    viewYear = d.getFullYear();
    viewMonth = d.getMonth();
    explicitSelectionKey = dateKey(selected);
    syncManual();
  }

  function changeMonth(delta: number) {
    const d = new Date(viewYear, viewMonth + delta, 1);
    viewYear = d.getFullYear();
    viewMonth = d.getMonth();
  }

  function daysGrid(): { date: Date; inMonth: boolean; disabled: boolean; today: boolean; selected: boolean; explicit: boolean }[] {
    const firstOfMonth = new Date(viewYear, viewMonth, 1);
    const firstDow = (firstOfMonth.getDay() + 6) % 7; // 0=Mon .. 6=Sun
    const prevDays = firstDow;
    const totalCells = 42; // 6 weeks
    const startDate = new Date(viewYear, viewMonth, 1 - prevDays);
    const selectedKey = dateKey(selected);
    const cells: { date: Date; inMonth: boolean; disabled: boolean; today: boolean; selected: boolean; explicit: boolean }[] = [];
    for (let i = 0; i < totalCells; i++) {
      const d = new Date(startDate);
      d.setDate(startDate.getDate() + i);
      const inMonth = d.getMonth() === viewMonth;
      const isToday = sameDate(d, new Date());
      const key = dateKey(d);
      const isSelected = key === selectedKey;
      const isExplicit = explicitSelectionKey === key;
      const disabled = !!minDate && trimTime(d) < trimTime(minDate);
      cells.push({ date: d, inMonth, disabled, today: isToday, selected: isSelected, explicit: isExplicit });
    }
    return cells;
  }

  function sameDate(a: Date, b: Date): boolean {
    return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
  }

  function dateKey(d: Date): string {
    return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
  }
  function trimTime(d: Date): number {
    const c = new Date(d);
    c.setHours(0, 0, 0, 0);
    return c.getTime();
  }

  function pickDay(d: Date, disabled: boolean) {
    if (disabled) return;
    const keepH = hour, keepM = minute;
    selected = new Date(d);
    hour = keepH; minute = keepM;
    // If user clicked a day from adjacent month, jump view to that month
    viewYear = selected.getFullYear();
    viewMonth = selected.getMonth();
    explicitSelectionKey = dateKey(selected);
    syncManual();
  }

  function dayButtonClass(c: { inMonth: boolean; disabled: boolean; today: boolean; selected: boolean; explicit: boolean }): string {
    const base = ['btn', 'btn-sm', 'min-h-0', 'h-10', 'w-full', 'font-semibold'];

    if (c.disabled) {
      base.push('btn-disabled', 'opacity-60');
      return base.join(' ');
    }

    if (c.explicit) {
      base.push('btn-primary', 'text-primary-content', 'shadow');
      return base.join(' ');
    }

    if (c.today && c.inMonth) {
      base.push('btn-neutral', 'btn-active', 'text-neutral-content');
      return base.join(' ');
    }

    if (!c.inMonth) {
      base.push('btn-ghost', 'text-base-content/40');
      return base.join(' ');
    }

    base.push('btn-ghost', 'text-base-content', 'hover:bg-base-200');
    return base.join(' ');
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
      viewYear = d.getFullYear();
      viewMonth = d.getMonth();
      explicitSelectionKey = dateKey(selected);
    }
  }
</script>

<dialog bind:this={dialog} class="modal" on:close={handleClose} on:cancel|preventDefault={handleCancel}>
  <div class="modal-box w-full max-w-4xl space-y-6 bg-base-100">
    <div class="flex flex-col gap-1">
      <div class="flex items-center justify-between gap-4">
        <div>
          <h3 class="text-lg font-semibold text-base-content">{title}</h3>
          <div class="text-sm text-base-content/60">{manual}</div>
        </div>
        <button type="button" class="btn btn-sm btn-circle btn-ghost" aria-label="Close" on:click={handleCancel}>
          ✕
        </button>
      </div>
    </div>

    <div class="grid gap-6 md:grid-cols-[minmax(0,1fr)_minmax(0,0.9fr)]">
      <!-- Calendar -->
      <section class="calendar calendar-bordered rounded-box border border-base-300 bg-base-200/50 p-4">
        <div class="mb-4 flex items-center justify-between">
          <button
            type="button"
            class="btn btn-sm btn-square btn-ghost"
            on:click={() => changeMonth(-1)}
            aria-label="Previous month"
          >
            «
          </button>
          <div class="text-base font-semibold text-base-content">
            {new Date(viewYear, viewMonth, 1).toLocaleString(undefined, { month: 'long', year: 'numeric' })}
          </div>
          <button
            type="button"
            class="btn btn-sm btn-square btn-ghost"
            on:click={() => changeMonth(1)}
            aria-label="Next month"
          >
            »
          </button>
        </div>
        <div class="mb-2 grid grid-cols-7 gap-2 text-center text-xs font-semibold text-base-content/60">
          {#each dowShort as d}
            <div class="uppercase">{d}</div>
          {/each}
        </div>
        <div class="grid grid-cols-7 gap-2">
          {#each daysGrid() as c}
            <button
              type="button"
              class={dayButtonClass(c)}
              disabled={c.disabled}
              on:click={() => pickDay(c.date, c.disabled)}
              aria-pressed={c.explicit}
              aria-current={c.today ? 'date' : undefined}
            >
              {c.date.getDate()}
            </button>
          {/each}
        </div>
      </section>

      <!-- Time & shortcuts -->
      <section class="space-y-6">
        <div class="rounded-box border border-base-300 bg-base-200/40 p-4">
          <div class="mb-3 flex items-center justify-between text-sm font-medium text-base-content/70">
            <span>Time (24h)</span>
            <span class="text-xs font-normal text-base-content/50">Adjust hours & minutes</span>
          </div>
          <div class="flex items-center gap-3">
            <div class="join">
              <button
                class="btn btn-sm join-item"
                type="button"
                on:click={() => hour = clamp0_23(hour - 1)}
                aria-label="Decrease hour"
              >
                -
              </button>
              <input
                class="input input-sm join-item w-16 text-center"
                value={formatTwo(hour)}
                on:input={onHourInput}
                aria-label="Hours"
              />
              <button
                class="btn btn-sm join-item"
                type="button"
                on:click={() => hour = clamp0_23(hour + 1)}
                aria-label="Increase hour"
              >
                +
              </button>
            </div>
            <span class="text-lg font-semibold text-base-content/40">:</span>
            <div class="join">
              <button
                class="btn btn-sm join-item"
                type="button"
                on:click={() => minute = clamp0_59(minute - 5)}
                aria-label="Decrease minutes"
              >
                -
              </button>
              <input
                class="input input-sm join-item w-16 text-center"
                value={formatTwo(minute)}
                on:input={onMinuteInput}
                aria-label="Minutes"
              />
              <button
                class="btn btn-sm join-item"
                type="button"
                on:click={() => minute = clamp0_59(minute + 5)}
                aria-label="Increase minutes"
              >
                +
              </button>
            </div>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <button type="button" class="btn btn-xs btn-outline" on:click={() => applyPresetTime(8, 0)}>
              08:00
            </button>
            <button type="button" class="btn btn-xs btn-outline" on:click={() => applyPresetTime(12, 0)}>
              12:00
            </button>
            <button type="button" class="btn btn-xs btn-outline" on:click={() => applyPresetTime(17, 0)}>
              17:00
            </button>
            <button type="button" class="btn btn-xs btn-outline" on:click={() => applyPresetTime(23, 59)}>
              23:59
            </button>
          </div>
        </div>

        <div class="rounded-box border border-base-300 bg-base-200/40 p-4">
          <div class="mb-3 text-sm font-medium text-base-content/70">Shortcuts</div>
          <div class="flex flex-wrap gap-2">
            {#each shortcuts as s}
              <button type="button" class="btn btn-sm btn-outline" on:click={() => applyShortcut(s)}>
                {s.label}
              </button>
            {/each}
            <button
              type="button"
              class="btn btn-sm btn-outline"
              on:click={() => {
                const d = new Date();
                selected = d;
                hour = d.getHours();
                minute = d.getMinutes();
                viewYear = d.getFullYear();
                viewMonth = d.getMonth();
                explicitSelectionKey = dateKey(selected);
                syncManual();
              }}
            >
              Now
            </button>
            <button
              type="button"
              class="btn btn-sm btn-outline"
              on:click={() => {
                const d = new Date();
                d.setHours(23, 59, 0, 0);
                selected = d;
                hour = 23;
                minute = 59;
                viewYear = d.getFullYear();
                viewMonth = d.getMonth();
                explicitSelectionKey = dateKey(selected);
                syncManual();
              }}
            >
              Today 23:59
            </button>
          </div>
        </div>

        <div class="rounded-box border border-base-300 bg-base-200/40 p-4">
          <div class="mb-2 flex items-center justify-between text-sm font-medium text-base-content/70">
            <span>Manual entry</span>
            <span class="text-xs font-normal text-base-content/50">dd/mm/yyyy hh:mm</span>
          </div>
          <input
            class="input input-bordered input-sm w-full"
            placeholder="e.g. 25/12/2025 16:30"
            bind:value={manual}
            on:input={() => parseManualInput(manual)}
            on:blur={() => manual = currentLabel()}
          />
        </div>
      </section>
    </div>

    <div class="modal-action">
      <button class="btn btn-ghost" type="button" on:click={handleCancel}>
        Cancel
      </button>
      <button class="btn btn-primary" type="button" on:click={confirm}>
        Set deadline
      </button>
    </div>
  </div>
</dialog>

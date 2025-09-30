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
    const base = [
      'h-9',
      'w-full',
      'rounded-md',
      'text-sm',
      'font-medium',
      'transition-colors',
      'focus-visible:outline-none',
      'focus-visible:ring-2',
      'focus-visible:ring-blue-500',
      'focus-visible:ring-offset-2',
      'focus-visible:ring-offset-white',
      'flex',
      'items-center',
      'justify-center'
    ];

    if (c.disabled) {
      base.push('text-gray-300', 'cursor-not-allowed', 'opacity-50');
      return base.join(' ');
    }

    if (c.selected) {
      base.push('bg-blue-600', 'text-white', 'shadow');
      return base.join(' ');
    }

    if (!c.inMonth) {
      base.push('text-gray-400');
    } else {
      base.push('text-gray-700', 'hover:bg-gray-100');
    }

    if (c.today && c.inMonth) {
      base.push('bg-gray-200', 'text-gray-800');
    }

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

<dialog bind:this={dialog} class="deadline-picker-dialog" on:close={handleClose} on:cancel|preventDefault={handleCancel}>
  <div class="deadline-picker-container w-full max-w-3xl rounded-2xl border border-gray-200 bg-white p-6 shadow-xl">
    <div class="flex items-start justify-between gap-4 border-b border-gray-200 pb-4">
      <div>
        <h3 class="text-lg font-semibold text-gray-900">{title}</h3>
        <div class="text-sm text-gray-500">{manual}</div>
      </div>
      <button
        type="button"
        class="inline-flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        aria-label="Close"
        on:click={handleCancel}
      >
        ✕
      </button>
    </div>

    <div class="mt-6 grid gap-6 md:grid-cols-2">
      <!-- Calendar -->
      <div>
        <div class="mb-4 flex items-center justify-between">
          <button
            type="button"
            class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-gray-300 bg-white text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            on:click={() => changeMonth(-1)}
            aria-label="Previous month"
          >
            «
          </button>
          <div class="text-base font-semibold text-gray-900">
            {new Date(viewYear, viewMonth, 1).toLocaleString(undefined, { month: 'long', year: 'numeric' })}
          </div>
          <button
            type="button"
            class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-gray-300 bg-white text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            on:click={() => changeMonth(1)}
            aria-label="Next month"
          >
            »
          </button>
        </div>
        <div class="mb-2 grid grid-cols-7 gap-2 text-center text-xs font-semibold text-gray-500">
          {#each dowShort as d}
            <div>{d}</div>
          {/each}
        </div>
        <div class="grid grid-cols-7 gap-2">
          {#each daysGrid() as c}
            <button
              type="button"
              class={dayButtonClass(c)}
              disabled={c.disabled}
              on:click={() => pickDay(c.date, c.disabled)}
              aria-pressed={c.selected}
              aria-current={c.today ? 'date' : undefined}
            >
              {c.date.getDate()}
            </button>
          {/each}
        </div>
      </div>

      <!-- Time & shortcuts -->
      <div class="space-y-6">
        <div>
          <div class="mb-2 flex items-center justify-between text-sm font-medium text-gray-600">
            <span>Time (24h)</span>
            <span class="text-xs font-normal text-gray-400">Adjust hours & minutes</span>
          </div>
          <div class="flex items-center gap-3">
            <div class="flex items-center gap-1">
              <button
                class="inline-flex h-8 w-8 items-center justify-center rounded-md border border-gray-300 text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                type="button"
                on:click={() => hour = clamp0_23(hour - 1)}
                aria-label="Decrease hour"
              >
                -
              </button>
              <input
                class="h-8 w-16 rounded-md border border-gray-300 text-center text-sm text-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={formatTwo(hour)}
                on:input={onHourInput}
                aria-label="Hours"
              />
              <button
                class="inline-flex h-8 w-8 items-center justify-center rounded-md border border-gray-300 text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                type="button"
                on:click={() => hour = clamp0_23(hour + 1)}
                aria-label="Increase hour"
              >
                +
              </button>
            </div>
            <span class="text-lg font-semibold text-gray-400">:</span>
            <div class="flex items-center gap-1">
              <button
                class="inline-flex h-8 w-8 items-center justify-center rounded-md border border-gray-300 text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                type="button"
                on:click={() => minute = clamp0_59(minute - 5)}
                aria-label="Decrease minutes"
              >
                -
              </button>
              <input
                class="h-8 w-16 rounded-md border border-gray-300 text-center text-sm text-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={formatTwo(minute)}
                on:input={onMinuteInput}
                aria-label="Minutes"
              />
              <button
                class="inline-flex h-8 w-8 items-center justify-center rounded-md border border-gray-300 text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                type="button"
                on:click={() => minute = clamp0_59(minute + 5)}
                aria-label="Increase minutes"
              >
                +
              </button>
            </div>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <button
              type="button"
              class="inline-flex items-center rounded-md border border-gray-300 px-3 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              on:click={() => applyPresetTime(8, 0)}
            >
              08:00
            </button>
            <button
              type="button"
              class="inline-flex items-center rounded-md border border-gray-300 px-3 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              on:click={() => applyPresetTime(12, 0)}
            >
              12:00
            </button>
            <button
              type="button"
              class="inline-flex items-center rounded-md border border-gray-300 px-3 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              on:click={() => applyPresetTime(17, 0)}
            >
              17:00
            </button>
            <button
              type="button"
              class="inline-flex items-center rounded-md border border-gray-300 px-3 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              on:click={() => applyPresetTime(23, 59)}
            >
              23:59
            </button>
          </div>
        </div>

        <div>
          <div class="mb-2 text-sm font-medium text-gray-600">Shortcuts</div>
          <div class="flex flex-wrap gap-2">
            {#each shortcuts as s}
              <button
                type="button"
                class="inline-flex items-center rounded-md border border-gray-300 px-3 py-1 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                on:click={() => applyShortcut(s)}
              >
                {s.label}
              </button>
            {/each}
            <button
              type="button"
              class="inline-flex items-center rounded-md border border-gray-300 px-3 py-1 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
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
              class="inline-flex items-center rounded-md border border-gray-300 px-3 py-1 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
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

        <div>
          <div class="mb-2 flex items-center justify-between text-sm font-medium text-gray-600">
            <span>Manual entry</span>
            <span class="text-xs font-normal text-gray-400">dd/mm/yyyy hh:mm</span>
          </div>
          <input
            class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="e.g. 25/12/2025 16:30"
            bind:value={manual}
            on:input={() => parseManualInput(manual)}
            on:blur={() => manual = currentLabel()}
          />
        </div>
      </div>
    </div>

    <div class="mt-8 flex justify-end gap-3 border-t border-gray-200 pt-4">
      <button
        class="inline-flex items-center justify-center rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        type="button"
        on:click={handleCancel}
      >
        Cancel
      </button>
      <button
        class="inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white shadow transition-colors hover:bg-blue-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        type="button"
        on:click={confirm}
      >
        Set deadline
      </button>
    </div>
  </div>
</dialog>

<style>
  .deadline-picker-dialog {
    border: none;
    padding: 0;
    background: transparent;
  }

  .deadline-picker-dialog[open] {
    display: grid;
    place-items: center;
    width: 100%;
    height: 100%;
  }

  .deadline-picker-dialog::backdrop {
    background: rgba(15, 23, 42, 0.45);
    backdrop-filter: blur(2px);
  }

  .deadline-picker-container {
    width: min(100vw - 2rem, 48rem);
  }
</style>

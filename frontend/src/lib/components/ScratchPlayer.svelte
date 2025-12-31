<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Play, Square } from "lucide-svelte";
  import { t, translator } from "$lib/i18n";

  export let projectData: Uint8Array | ArrayBuffer | null = null;
  export let projectName: string | null = null;
  export let fullScreen = false;

  let canvas: HTMLCanvasElement;
  let vm: any = null;
  let renderer: any = null;
  let audioEngine: any = null;
  let storage: any = null;
  let loading = false;
  let error = "";
  let ready = false;
  let running = false;
  let resizeObserver: ResizeObserver | null = null;
  let stageResizeObserver: ResizeObserver | null = null;
  let lastProjectRef: Uint8Array | ArrayBuffer | null = null;
  let pointerIsDown = false;
  let monitors: MonitorView[] = [];
  let visibleMonitors: MonitorView[] = [];
  let monitorListener: ((monitorList: any) => void) | null = null;
  let stageShell: HTMLDivElement | null = null;
  let stageSizeStyle = "";
  const STAGE_ASPECT = 4 / 3;

  const MONITORS_UPDATE_EVENT = "MONITORS_UPDATE";
  const MONITOR_OPCODES = new Set(["data_variable", "data_listcontents"]);

  type MonitorView = {
    id: string;
    opcode: string;
    mode: string;
    value: any;
    params: Record<string, any> | null;
    spriteName: string | null;
    visible: boolean;
    label: string;
  };

  let translate = t;
  $: translate = $translator;

  function resolveCtor(mod: any, names: string[]) {
    const candidates: any[] = [];
    if (mod) {
      candidates.push(mod.default, mod);
      for (const name of names) {
        candidates.push(mod[name]);
      }
      if (mod.default && typeof mod.default === "object") {
        for (const name of names) {
          candidates.push(mod.default[name]);
        }
      }
    }
    return candidates.find((c) => typeof c === "function") ?? null;
  }

  function getMonitorLabel(monitor: any) {
    const params = monitor?.params ?? {};
    const name =
      typeof params.VARIABLE === "string"
        ? params.VARIABLE
        : typeof params.LIST === "string"
          ? params.LIST
          : monitor?.id ?? "";
    if (monitor?.spriteName && monitor.spriteName !== "Stage") {
      return `${name} (${monitor.spriteName})`;
    }
    return name;
  }

  function normalizeMonitor(monitor: any): MonitorView | null {
    if (!monitor) return null;
    const data = typeof monitor.toJS === "function" ? monitor.toJS() : monitor;
    if (!data?.id || !data?.opcode) return null;
    return {
      id: data.id,
      opcode: data.opcode,
      mode: data.mode ?? "default",
      value: data.value,
      params: data.params ?? null,
      spriteName: data.spriteName ?? null,
      visible: data.visible !== false,
      label: getMonitorLabel(data),
    };
  }

  function handleMonitorsUpdate(monitorList: any) {
    if (!monitorList) {
      monitors = [];
      return;
    }
    let list: any[] = [];
    if (typeof monitorList.valueSeq === "function") {
      list = monitorList.valueSeq().toArray();
    } else if (Array.isArray(monitorList)) {
      list = monitorList;
    } else if (typeof monitorList.values === "function") {
      list = Array.from(monitorList.values());
    }
    const next: MonitorView[] = [];
    for (const monitor of list) {
      const normalized = normalizeMonitor(monitor);
      if (!normalized) continue;
      if (!MONITOR_OPCODES.has(normalized.opcode)) continue;
      next.push(normalized);
    }
    monitors = next;
  }

  function isListValue(value: any): value is any[] {
    return Array.isArray(value);
  }

  function formatMonitorValue(value: any) {
    if (value === null || value === undefined) return "";
    return typeof value === "string" ? value : String(value);
  }

  async function setupVM() {
    const [vmMod, renderMod, audioMod, storageMod, svgRendererMod] = await Promise.all([
      import("scratch-vm"),
      import("scratch-render"),
      import("scratch-audio"),
      import("scratch-storage"),
      import("scratch-svg-renderer"),
    ]);
    const VM = resolveCtor(vmMod, ["VirtualMachine"]);
    const RenderWebGL = resolveCtor(renderMod, ["RenderWebGL", "ScratchRender"]);
    const AudioEngine = resolveCtor(audioMod, ["AudioEngine"]);
    const ScratchStorage = resolveCtor(storageMod, ["ScratchStorage"]);
    const BitmapAdapter = resolveCtor(svgRendererMod, ["BitmapAdapter"]);

    if (!VM || !RenderWebGL || !AudioEngine || !ScratchStorage || !BitmapAdapter) {
      throw new Error(
        translate("frontend/src/lib/components/ScratchPlayer.svelte::scratch_player_runtime_error"),
      );
    }

    vm = new VM();
    if (typeof vm.attachV2BitmapAdapter === "function") {
      vm.attachV2BitmapAdapter(new BitmapAdapter());
    }
    renderer = new RenderWebGL(canvas);
    vm.attachRenderer(renderer);

    storage = new ScratchStorage();
    const AssetType = storage?.AssetType;
    if (AssetType && storage.addWebStore) {
      storage.addWebStore([AssetType.Project], storage.WebStore);
      storage.addWebStore(
        [AssetType.ImageVector, AssetType.ImageBitmap, AssetType.Sound],
        storage.AssetWebStore,
      );
    }
    if (storage.setBitmapAdapter) {
      storage.setBitmapAdapter(new BitmapAdapter());
    }
    vm.attachStorage(storage);

    audioEngine = new AudioEngine();
    vm.attachAudioEngine(audioEngine);

    vm.start();
    ready = true;
    resizeStage();

    monitorListener = (monitorList) => {
      handleMonitorsUpdate(monitorList);
    };
    vm.on?.(MONITORS_UPDATE_EVENT, monitorListener);
  }

  function resizeStage() {
    if (!renderer || !canvas) return;
    const rect = canvas.getBoundingClientRect();
    const width = Math.max(1, Math.floor(rect.width));
    const height = Math.max(1, Math.floor(rect.height));
    canvas.width = width;
    canvas.height = height;
    renderer.resize(width, height);
  }

  function updateStageSize() {
    if (!fullScreen || !stageShell) return;
    const rect = stageShell.getBoundingClientRect();
    const availableWidth = Math.max(1, rect.width);
    const availableHeight = Math.max(1, rect.height);
    const widthFromHeight = availableHeight * STAGE_ASPECT;
    let width = availableWidth;
    let height = availableWidth / STAGE_ASPECT;
    if (widthFromHeight < availableWidth) {
      width = widthFromHeight;
      height = availableHeight;
    }
    stageSizeStyle = `width: ${Math.floor(width)}px; height: ${Math.floor(height)}px;`;
  }

  function postMouse(event: PointerEvent, isDown: boolean) {
    if (!vm || !canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    vm.postIOData("mouse", {
      x,
      y,
      isDown,
      canvasWidth: rect.width,
      canvasHeight: rect.height,
    });
  }

  function handlePointerDown(event: PointerEvent) {
    if (!canvas) return;
    pointerIsDown = true;
    canvas.focus();
    canvas.setPointerCapture?.(event.pointerId);
    postMouse(event, true);
  }

  function handlePointerMove(event: PointerEvent) {
    postMouse(event, pointerIsDown);
  }

  function handlePointerUp(event: PointerEvent) {
    pointerIsDown = false;
    canvas?.releasePointerCapture?.(event.pointerId);
    postMouse(event, false);
  }

  function handlePointerCancel(event: PointerEvent) {
    pointerIsDown = false;
    canvas?.releasePointerCapture?.(event.pointerId);
    postMouse(event, false);
  }

  function handlePointerLeave(event: PointerEvent) {
    if (!pointerIsDown) {
      postMouse(event, false);
      return;
    }
    if (!canvas?.hasPointerCapture?.(event.pointerId)) {
      pointerIsDown = false;
      postMouse(event, false);
    }
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (!vm) return;
    if (!event.key) return;
    vm.postIOData("keyboard", { key: event.key, isDown: true });
    event.preventDefault();
  }

  function handleKeyUp(event: KeyboardEvent) {
    if (!vm) return;
    if (!event.key) return;
    vm.postIOData("keyboard", { key: event.key, isDown: false });
    event.preventDefault();
  }

  async function loadProject() {
    if (!vm || !projectData) return;
    if (projectData === lastProjectRef) return;
    lastProjectRef = projectData;
    loading = true;
    error = "";
    running = false;
    try {
      vm.stopAll();
      vm.clear();
      const data = projectData instanceof Uint8Array
        ? projectData
        : new Uint8Array(projectData);
      await vm.loadProject(data);
    } catch (e: any) {
      error = e?.message || translate("frontend/src/lib/components/ScratchPlayer.svelte::scratch_player_load_error");
    } finally {
      loading = false;
    }
  }

  function runProject() {
    if (!vm || loading) return;
    canvas?.focus();
    vm.greenFlag();
    running = true;
  }

  function stopProject() {
    if (!vm) return;
    vm.stopAll();
    running = false;
  }

  onMount(async () => {
    try {
      await setupVM();
      if (projectData) await loadProject();
    } catch (e: any) {
      error =
        e?.message ||
        translate(
          "frontend/src/lib/components/ScratchPlayer.svelte::scratch_player_runtime_error",
        );
    }
    if (typeof ResizeObserver !== "undefined") {
      resizeObserver = new ResizeObserver(() => resizeStage());
      resizeObserver.observe(canvas);
    }
    if (typeof ResizeObserver !== "undefined" && stageShell) {
      stageResizeObserver = new ResizeObserver(() => updateStageSize());
      stageResizeObserver.observe(stageShell);
    }
    updateStageSize();
    return () => {
      resizeObserver?.disconnect();
      resizeObserver = null;
      stageResizeObserver?.disconnect();
      stageResizeObserver = null;
    };
  });

  onDestroy(() => {
    try {
      if (vm && monitorListener) {
        vm.off?.(MONITORS_UPDATE_EVENT, monitorListener);
        vm.removeListener?.(MONITORS_UPDATE_EVENT, monitorListener);
      }
      vm?.stopAll();
      vm?.clear();
    } catch {}
    monitorListener = null;
    monitors = [];
    vm = null;
    renderer = null;
    audioEngine = null;
    storage = null;
  });

  $: if (ready && projectData) {
    loadProject();
  }

  $: visibleMonitors = monitors.filter((monitor) => monitor.visible);
  $: if (!fullScreen) {
    stageSizeStyle = "";
  } else {
    updateStageSize();
  }
</script>

<div class={`space-y-4 ${fullScreen ? "flex flex-col h-full min-h-0" : ""}`}>
  <div class="flex flex-wrap items-center gap-3">
    <button
      class="btn btn-primary btn-sm rounded-lg gap-2 font-black uppercase tracking-widest text-[9px]"
      on:click={runProject}
      disabled={!ready || loading || !projectData}
    >
      <Play size={12} />
      {translate("frontend/src/lib/components/ScratchPlayer.svelte::scratch_player_play")}
    </button>
    <button
      class="btn btn-ghost btn-sm rounded-lg gap-2 font-black uppercase tracking-widest text-[9px]"
      on:click={stopProject}
      disabled={!ready}
    >
      <Square size={12} />
      {translate("frontend/src/lib/components/ScratchPlayer.svelte::scratch_player_stop")}
    </button>
    {#if loading}
      <div class="flex items-center gap-2 text-xs font-black uppercase tracking-widest opacity-60">
        <span class="loading loading-spinner loading-xs"></span>
        {translate("frontend/src/lib/components/ScratchPlayer.svelte::scratch_player_loading")}
      </div>
    {:else if running}
      <div class="badge badge-sm badge-success font-black text-[9px] uppercase tracking-wider">
        {translate("frontend/src/lib/components/ScratchPlayer.svelte::scratch_player_live_badge")}
      </div>
    {/if}
  </div>

  {#if error}
    <div class="alert bg-error/10 border-error/20 text-error-content rounded-2xl">
      <span class="text-sm font-medium">{error}</span>
    </div>
  {/if}

  <div
    class={`w-full ${fullScreen ? "flex-1 flex items-center justify-center min-h-0" : "max-w-3xl"}`}
    bind:this={stageShell}
  >
    <div
      class={`relative w-full aspect-[4/3] rounded-2xl overflow-hidden border border-base-300 bg-base-200/60 ${fullScreen ? "mx-auto" : ""}`}
      style={fullScreen ? stageSizeStyle : ""}
    >
      {#if visibleMonitors.length}
        <div class="absolute left-3 top-3 z-10 flex flex-col gap-2 pointer-events-none">
          {#each visibleMonitors as monitor (monitor.id)}
            {#if monitor.mode === "list" || monitor.opcode === "data_listcontents"}
              <div class="pointer-events-auto rounded-xl border border-base-300 bg-base-100/90 shadow-sm p-2 text-xs min-w-[160px] max-w-[220px]">
                <div class="text-[10px] font-black uppercase tracking-widest opacity-70">{monitor.label}</div>
                {#if isListValue(monitor.value)}
                  <div class="mt-1 max-h-40 overflow-auto text-[11px] leading-snug space-y-1">
                    {#each monitor.value as item, index (index)}
                      <div class="flex gap-2">
                        <span class="opacity-50">{index + 1}.</span>
                        <span class="break-words">{formatMonitorValue(item)}</span>
                      </div>
                    {/each}
                  </div>
                {:else}
                  <div class="mt-1 text-[11px]">{formatMonitorValue(monitor.value)}</div>
                {/if}
              </div>
            {:else}
              <div class="pointer-events-auto rounded-full border border-base-300 bg-base-100/90 shadow-sm px-3 py-1 text-xs flex items-center gap-2">
                <span class="font-semibold">{monitor.label}</span>
                <span class="opacity-70">=</span>
                <span class="font-mono text-[11px]">{formatMonitorValue(monitor.value)}</span>
              </div>
            {/if}
          {/each}
        </div>
      {/if}
      <canvas
        bind:this={canvas}
        class="w-full h-full outline-none"
        tabindex="0"
        aria-label={projectName ?? translate("frontend/src/lib/components/ScratchPlayer.svelte::scratch_player_stage_label")}
        on:pointerdown={handlePointerDown}
        on:pointermove={handlePointerMove}
        on:pointerup={handlePointerUp}
        on:pointercancel={handlePointerCancel}
        on:pointerleave={handlePointerLeave}
        on:keydown={handleKeyDown}
        on:keyup={handleKeyUp}
      ></canvas>
    </div>
  </div>
</div>

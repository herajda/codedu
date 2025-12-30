<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Play, Square } from "lucide-svelte";
  import { t, translator } from "$lib/i18n";

  export let projectData: Uint8Array | ArrayBuffer | null = null;
  export let projectName: string | null = null;

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
  let lastProjectRef: Uint8Array | ArrayBuffer | null = null;

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

  async function setupVM() {
    const [vmMod, renderMod, audioMod, storageMod] = await Promise.all([
      import("scratch-vm"),
      import("scratch-render"),
      import("scratch-audio"),
      import("scratch-storage"),
    ]);
    const VM = resolveCtor(vmMod, ["VirtualMachine"]);
    const RenderWebGL = resolveCtor(renderMod, ["RenderWebGL", "ScratchRender"]);
    const AudioEngine = resolveCtor(audioMod, ["AudioEngine"]);
    const ScratchStorage = resolveCtor(storageMod, ["ScratchStorage"]);

    if (!VM || !RenderWebGL || !AudioEngine || !ScratchStorage) {
      throw new Error(
        translate("frontend/src/lib/components/ScratchPlayer.svelte::scratch_player_runtime_error"),
      );
    }

    vm = new VM();
    renderer = new RenderWebGL(canvas);
    vm.attachRenderer(renderer);

    storage = new ScratchStorage();
    vm.attachStorage(storage);

    audioEngine = new AudioEngine();
    vm.attachAudioEngine(audioEngine);

    vm.start();
    ready = true;
    resizeStage();
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

  function normalizeKey(key: string) {
    if (key === " ") return "space";
    if (key === "ArrowUp") return "up arrow";
    if (key === "ArrowDown") return "down arrow";
    if (key === "ArrowLeft") return "left arrow";
    if (key === "ArrowRight") return "right arrow";
    if (key.length === 1) return key.toLowerCase();
    return key.toLowerCase();
  }

  function postMouse(event: PointerEvent, isDown: boolean) {
    if (!vm || !canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = ((event.clientX - rect.left) / rect.width) * 480 - 240;
    const y = 180 - ((event.clientY - rect.top) / rect.height) * 360;
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
    canvas.focus();
    postMouse(event, true);
  }

  function handlePointerMove(event: PointerEvent) {
    postMouse(event, event.buttons === 1);
  }

  function handlePointerUp(event: PointerEvent) {
    postMouse(event, false);
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (!vm) return;
    vm.postIOData("keyboard", { key: normalizeKey(event.key), isDown: true });
    event.preventDefault();
  }

  function handleKeyUp(event: KeyboardEvent) {
    if (!vm) return;
    vm.postIOData("keyboard", { key: normalizeKey(event.key), isDown: false });
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
    return () => {
      resizeObserver?.disconnect();
      resizeObserver = null;
    };
  });

  onDestroy(() => {
    try {
      vm?.stopAll();
      vm?.clear();
    } catch {}
    vm = null;
    renderer = null;
    audioEngine = null;
    storage = null;
  });

  $: if (ready && projectData) {
    loadProject();
  }
</script>

<div class="space-y-4">
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

  <div class="w-full max-w-3xl">
    <div class="relative w-full aspect-[4/3] rounded-2xl overflow-hidden border border-base-300 bg-base-200/60">
      <canvas
        bind:this={canvas}
        class="w-full h-full outline-none"
        tabindex="0"
        aria-label={projectName ?? translate("frontend/src/lib/components/ScratchPlayer.svelte::scratch_player_stage_label")}
        on:pointerdown={handlePointerDown}
        on:pointermove={handlePointerMove}
        on:pointerup={handlePointerUp}
        on:pointerleave={handlePointerUp}
        on:keydown={handleKeyDown}
        on:keyup={handleKeyUp}
      ></canvas>
    </div>
  </div>
</div>

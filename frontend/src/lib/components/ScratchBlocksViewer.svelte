<script lang="ts">
  import { onMount, onDestroy, tick } from "svelte";
  import { browser } from "$app/environment";
  import { t, translator, locale as localeStore } from "$lib/i18n";
  import { LayoutGrid, X } from "lucide-svelte";

  export let projectData: Uint8Array | ArrayBuffer | null = null;
  export let fullHeight = false;

  type TargetInfo = {
    id: string;
    name: string;
    isStage: boolean;
    hasBlocks: boolean;
    thumbnailUrl: string | null;
    costumeName: string | null;
  };

  let workspaceHost: HTMLDivElement | null = null;
  let workspace: any = null;
  let ScratchBlocks: any = null;
  let vm: any = null;
  let storage: any = null;
  let loading = false;
  let error = "";
  let targets: TargetInfo[] = [];
  let selectedTargetId = "";
  let activeSideTab: "sprites" | "stages" = "sprites";
  let xmlByTarget = new Map<string, string>();
  let lastProjectRef: Uint8Array | ArrayBuffer | null = null;
  let resizeObserver: ResizeObserver | null = null;
  let translate = t;
  $: translate = $translator;
  let activeLocale = "en";
  let localeUnsub: (() => void) | null = null;
  $: spriteTargets = targets.filter((target) => !target.isStage);
  $: stageTargets = targets.filter((target) => target.isStage);

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

  function normalizeLocale(value: string) {
    return value === "cs" ? "cs" : "en";
  }

  function getTargetName(target: any) {
    return typeof target.getName === "function"
      ? target.getName()
      : target.name ?? "Sprite";
  }

  function isAutoSpriteName(name: string) {
    return /^Sprite\d+$/i.test(name);
  }

  function getTargetDedupeKey(target: any, name: string) {
    if (target?.isStage) return "stage";
    if (!isAutoSpriteName(name)) {
      return `sprite:${name}`;
    }
    const costume = getTargetCostume(target);
    const assetId = costume?.assetId ?? costume?.md5 ?? "";
    const blockCount = target?.blocks?._blocks
      ? Object.keys(target.blocks._blocks).length
      : 0;
    const scriptCount = Array.isArray(target?.blocks?._scripts)
      ? target.blocks._scripts.length
      : 0;
    return `sprite:auto:${assetId}:${blockCount}:${scriptCount}`;
  }

  async function ensureStorage() {
    if (storage) return storage;
    try {
      const storageMod = await import("scratch-storage");
      const StorageCtor = resolveCtor(storageMod, ["ScratchStorage"]);
      if (!StorageCtor) return null;
      storage = new StorageCtor();
      return storage;
    } catch {
      return null;
    }
  }

  async function ensureScratchBlocks() {
    if (ScratchBlocks) return ScratchBlocks;
    const mod = await import("scratch-blocks/dist/vertical");
    ScratchBlocks = mod?.default ?? mod;
    return ScratchBlocks;
  }

  function applyScratchLocale() {
    if (ScratchBlocks?.ScratchMsgs?.setLocale) {
      ScratchBlocks.ScratchMsgs.setLocale(activeLocale);
    }
  }

  async function ensureWorkspace() {
    if (!workspaceHost) return null;
    await ensureScratchBlocks();
    applyScratchLocale();
    if (workspace) return workspace;
    workspace = ScratchBlocks.inject(workspaceHost, {
      readOnly: true,
      scrollbars: true,
      sounds: false,
      media: "/scratch-blocks-media/",
      zoom: {
        controls: true,
        wheel: true,
        startScale: 0.85,
        maxScale: 1.4,
        minScale: 0.3,
      },
    });
    return workspace;
  }

  function cleanupVM() {
    if (!vm) return;
    try {
      vm.stopAll?.();
      vm.clear?.();
    } catch {}
    vm = null;
  }

  function cleanupWorkspace() {
    if (!workspace) return;
    try {
      workspace.dispose?.();
    } catch {}
    workspace = null;
    if (workspaceHost) {
      workspaceHost.innerHTML = "";
    }
  }

  function cleanup() {
    cleanupVM();
    cleanupWorkspace();
  }

  function buildWorkspaceXml(target: any) {
    const stage = vm?.runtime?.getTargetForStage?.() ?? null;
    const globalVarMap = stage?.variables ?? {};
    const localVarMap = target?.isStage ? {} : target?.variables ?? {};
    const globalVariables = Object.keys(globalVarMap).map((k) => globalVarMap[k]);
    const localVariables = Object.keys(localVarMap).map((k) => localVarMap[k]);
    const workspaceComments = Object.keys(target?.comments ?? {})
      .map((k) => target.comments[k])
      .filter((c: any) => c?.blockId === null);

    return `<xml xmlns="http://www.w3.org/1999/xhtml">
      <variables>
        ${globalVariables.map((v) => v.toXML()).join("")}
        ${localVariables.map((v) => v.toXML(true)).join("")}
      </variables>
      ${workspaceComments.map((c: any) => c.toXML()).join("")}
      ${target.blocks.toXML(target.comments)}
    </xml>`;
  }

  function getTargetCostume(target: any) {
    if (!target) return null;
    if (typeof target.getCurrentCostume === "function") {
      return target.getCurrentCostume();
    }
    const costumes = target?.sprite?.costumes ?? target?.costumes ?? [];
    const index =
      typeof target?.currentCostume === "number" ? target.currentCostume : 0;
    return costumes[index] ?? costumes[0] ?? null;
  }

  function getCostumeThumbnail(costume: any) {
    const asset = costume?.asset;
    if (!asset || typeof asset.encodeDataURI !== "function") return null;
    try {
      return asset.encodeDataURI();
    } catch {
      return null;
    }
  }

  function selectTarget(targetId: string) {
    selectedTargetId = targetId;
  }

  function renderSelected() {
    if (!browser || !workspace || !ScratchBlocks || !selectedTargetId) return;
    const xml = xmlByTarget.get(selectedTargetId) ?? "";
    workspace.clear?.();
    if (xml.trim().length) {
      const dom = ScratchBlocks.Xml.textToDom(xml);
      ScratchBlocks.Xml.domToWorkspace(dom, workspace);
    }
    requestAnimationFrame(() => {
      ScratchBlocks.svgResize?.(workspace);
    });
  }

  function resizeWorkspace() {
    if (!browser || !workspace || !ScratchBlocks) return;
    ScratchBlocks.svgResize?.(workspace);
  }

  async function loadProject() {
    if (!browser || !projectData) return;
    loading = true;
    error = "";
    targets = [];
    selectedTargetId = "";
    xmlByTarget = new Map<string, string>();

    try {
      await tick();
      await ensureWorkspace();
      const vmMod = await import("scratch-vm");
      const VM = resolveCtor(vmMod, ["VirtualMachine"]);
      if (!VM) {
        throw new Error(
          translate("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_runtime_error"),
        );
      }
      cleanupVM();
      vm = new VM();
      const storageInstance = await ensureStorage();
      if (storageInstance && typeof vm.attachStorage === "function") {
        vm.attachStorage(storageInstance);
      }
      await vm.loadProject(projectData);

      const rawTargets = (vm?.runtime?.targets ?? []).filter((target: any) => {
        if (target?.isStage) return true;
        if (typeof target?.isSprite === "function") {
          return target.isSprite();
        }
        return (
          !Object.prototype.hasOwnProperty.call(target, "isOriginal") ||
          target.isOriginal
        );
      });
      const uniqueTargets: any[] = [];
      const seenTargets = new Set<string>();
      const seenSprites = new WeakSet<any>();
      for (const target of rawTargets) {
        const name = getTargetName(target);
        if (!target?.isStage && target?.sprite) {
          if (seenSprites.has(target.sprite)) continue;
          seenSprites.add(target.sprite);
        }
        const key = getTargetDedupeKey(target, name);
        if (seenTargets.has(key)) continue;
        seenTargets.add(key);
        uniqueTargets.push(target);
      }
      const mapped: TargetInfo[] = uniqueTargets.map((target: any) => {
        const costume = getTargetCostume(target);
        const name = getTargetName(target);
        return {
          id: target.id,
          name,
          isStage: !!target.isStage,
          hasBlocks: Array.isArray(target?.blocks?._scripts)
            ? target.blocks._scripts.length > 0
            : true,
          thumbnailUrl: getCostumeThumbnail(costume),
          costumeName: costume?.name ?? null,
        };
      });
      targets = mapped.sort((a, b) => {
        if (a.isStage && !b.isStage) return 1;
        if (!a.isStage && b.isStage) return -1;
        return a.name.localeCompare(b.name);
      });

      for (const target of uniqueTargets) {
        xmlByTarget.set(target.id, buildWorkspaceXml(target));
      }

      selectedTargetId =
        targets.find((t) => !t.isStage)?.id ?? targets[0]?.id ?? "";
      renderSelected();
    } catch (e: any) {
      error =
        e?.message ||
        translate("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_load_error");
    } finally {
      loading = false;
    }
  }

  $: if (browser && projectData && projectData !== lastProjectRef) {
    lastProjectRef = projectData;
    void loadProject();
  }

  $: if (!projectData && lastProjectRef) {
    lastProjectRef = null;
    targets = [];
    selectedTargetId = "";
    xmlByTarget = new Map<string, string>();
    cleanup();
  }

  $: if (!loading && selectedTargetId) {
    renderSelected();
  }

  onMount(() => {
    localeUnsub = localeStore.subscribe((value) => {
      activeLocale = normalizeLocale(value);
      applyScratchLocale();
      if (!loading && selectedTargetId) {
        renderSelected();
      }
    });
    if (typeof ResizeObserver !== "undefined" && workspaceHost) {
      resizeObserver = new ResizeObserver(() => {
        resizeWorkspace();
      });
      resizeObserver.observe(workspaceHost);
    }
    if (projectData) {
      void loadProject();
    }
  });

  onDestroy(() => {
    localeUnsub?.();
    resizeObserver?.disconnect();
    cleanup();
  });
</script>

<div class={`flex flex-col md:flex-row gap-6 ${fullHeight ? "scratch-blocks-full h-full" : "min-h-[600px] md:max-h-[650px]"}`}>
  <!-- Blocks area -->
  <div class="flex-1 flex flex-col min-w-0 gap-4 min-h-0">
    {#if loading}
      <div class="flex items-center gap-2 text-sm opacity-70 p-4">
        <span class="loading loading-spinner loading-sm"></span>
        {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_loading")}
      </div>
    {:else if error}
      <div class="alert bg-error/10 border-error/20 text-error rounded-2xl m-4">
        <span class="font-medium text-sm">{error}</span>
      </div>
    {:else if !targets.length}
      <div class="alert bg-warning/10 border-warning/20 text-warning rounded-2xl m-4">
        <span class="font-medium text-sm">
          {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_empty_project")}
        </span>
      </div>
    {/if}

    {#if targets.find((t) => t.id === selectedTargetId && !t.hasBlocks)}
      <div class="alert bg-warning/10 border-warning/20 text-warning rounded-2xl m-4">
        <span class="font-medium text-sm">
          {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_empty_target")}
        </span>
      </div>
    {/if}

    <div class="scratch-blocks-host flex-1 min-h-[500px]" bind:this={workspaceHost}></div>
  </div>

  <!-- Side Menu -->
  {#if targets.length}
    <aside class="w-full md:w-72 flex flex-col bg-base-100 rounded-3xl border border-base-200 shadow-xl overflow-hidden shrink-0 min-h-0 max-h-[400px] md:max-h-none">
      <div class="p-4 border-b border-base-200 bg-base-50/50 flex-none">
        <div class="flex items-center gap-3 mb-4">
          <div class="w-8 h-8 rounded-xl bg-primary/10 text-primary flex items-center justify-center">
            <LayoutGrid class="size-4" />
          </div>
          <h3 class="text-xs font-black tracking-tight uppercase tracking-widest opacity-80">
            {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_target_picker_title")}
          </h3>
        </div>

        <!-- Tab Switcher -->
        <div class="flex p-1 bg-base-200 rounded-xl">
          <button 
            type="button"
            class={`flex-1 py-2 px-3 rounded-lg text-[9px] font-black uppercase tracking-widest transition-all ${activeSideTab === 'sprites' ? 'bg-base-100 text-primary shadow-sm' : 'opacity-50 hover:opacity-100'}`}
            on:click={() => activeSideTab = 'sprites'}
          >
            {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_target_picker_sprites")}
          </button>
          <button 
            type="button"
            class={`flex-1 py-2 px-3 rounded-lg text-[9px] font-black uppercase tracking-widest transition-all ${activeSideTab === 'stages' ? 'bg-base-100 text-primary shadow-sm' : 'opacity-50 hover:opacity-100'}`}
            on:click={() => activeSideTab = 'stages'}
          >
            {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_target_picker_stages")}
          </button>
        </div>
      </div>

      <div class="flex-1 overflow-y-auto p-4 custom-scrollbar bg-base-100/30">
        <div class="grid grid-cols-2 gap-3">
          {#each (activeSideTab === 'sprites' ? spriteTargets : stageTargets) as target}
            <button
              type="button"
              class={`scratch-target-card ${selectedTargetId === target.id ? "is-active" : ""} ${target.isStage ? "is-stage" : ""}`}
              on:click={() => selectTarget(target.id)}
              title={target.costumeName ? `${target.name} - ${target.costumeName}` : target.name}
            >
              <div class="scratch-target-thumb">
                {#if target.thumbnailUrl}
                  <img src={target.thumbnailUrl} alt="" loading="lazy" />
                {:else}
                  <div class="scratch-target-fallback">
                    <span>{target.name?.slice(0, 1) || "?"}</span>
                  </div>
                {/if}
              </div>
              <div class="scratch-target-name">{target.name}</div>
            </button>
          {/each}
        </div>
        
        {#if (activeSideTab === 'sprites' ? spriteTargets : stageTargets).length === 0}
          <div class="flex flex-col items-center justify-center py-12 text-center opacity-30 gap-3">
            <div class="p-3 bg-base-200 rounded-full">
              <LayoutGrid class="size-6" />
            </div>
            <p class="text-[10px] font-black uppercase tracking-widest">
              {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::no_options", "No items found")}
            </p>
          </div>
        {/if}
      </div>
    </aside>
  {/if}
</div>

<style>
  .scratch-target-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(110px, 110px));
    gap: 0.6rem;
    justify-content: center;
  }

  .scratch-target-card {
    cursor: pointer;
    background: hsl(var(--b2) / 0.55);
    border: 1px solid hsl(var(--b3) / 0.7);
    border-radius: 0.75rem;
    padding: 0.25rem 0.25rem 0.3rem;
    display: grid;
    gap: 0.2rem;
    text-align: center;
    position: relative;
    transition: all 160ms ease;
  }

  .scratch-target-card:hover {
    transform: translateY(-1px);
    background: hsl(var(--b2) / 0.8);
    box-shadow: 0 8px 16px -12px hsl(var(--bc) / 0.55);
  }

  .scratch-target-card:focus-visible {
    outline: 2px solid hsl(var(--p));
    outline-offset: 2px;
  }

  .scratch-target-card.is-active {
    border-color: hsl(var(--p) / 0.7);
    background: hsl(var(--p) / 0.14);
    box-shadow: 0 10px 20px -15px hsl(var(--p) / 0.45);
  }

  .scratch-target-thumb {
    width: 100%;
    aspect-ratio: 4 / 3;
    border-radius: 0.5rem;
    border: 1px solid hsl(var(--b3) / 0.5);
    background:
      radial-gradient(120% 120% at 0% 0%, hsl(var(--b2) / 0.7), transparent 45%),
      linear-gradient(160deg, hsl(var(--b2) / 0.85), hsl(var(--b1)));
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.5rem;
    overflow: hidden;
  }

  .scratch-target-card.is-stage .scratch-target-thumb {
    padding: 0; /* Stages should fill the area more */
    background:
      radial-gradient(120% 120% at 20% 0%, hsl(var(--a) / 0.18), transparent 55%),
      linear-gradient(160deg, hsl(var(--b2) / 0.85), hsl(var(--b1)));
  }

  .scratch-target-thumb img {
    width: 100%;
    height: 100%;
    object-fit: contain;
    object-position: center;
  }

  .scratch-target-fallback {
    width: 100%;
    height: 100%;
    display: grid;
    place-items: center;
    font-size: 0.7rem;
    font-weight: 800;
    text-transform: uppercase;
    color: hsl(var(--bc) / 0.55);
  }

  .scratch-target-name {
    font-size: 0.55rem;
    font-weight: 700;
    letter-spacing: 0.01em;
    color: hsl(var(--bc) / 0.75);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .scratch-blocks-host {
    width: 100%;
    border-radius: 1.5rem;
    overflow: hidden;
    background: hsl(var(--b1));
    border: 1px solid hsl(var(--b3) / 0.6);
    box-shadow: inset 0 2px 4px 0 rgb(0 0 0 / 0.05);
  }

  .scratch-blocks-full .scratch-blocks-host {
    flex: 1 1 auto;
    height: auto;
    min-height: 0;
  }

  .custom-scrollbar::-webkit-scrollbar {
    width: 4px;
  }
  .custom-scrollbar::-webkit-scrollbar-track {
    background: transparent;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb {
    background: hsl(var(--bc) / 0.1);
    border-radius: 10px;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb:hover {
    background: hsl(var(--bc) / 0.2);
  }
</style>

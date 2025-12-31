<script lang="ts">
  import { onMount, onDestroy, tick } from "svelte";
  import { browser } from "$app/environment";
  import { t, translator, locale as localeStore } from "$lib/i18n";

  export let projectData: Uint8Array | ArrayBuffer | null = null;

  type TargetInfo = {
    id: string;
    name: string;
    isStage: boolean;
    hasBlocks: boolean;
  };

  let workspaceHost: HTMLDivElement | null = null;
  let workspace: any = null;
  let ScratchBlocks: any = null;
  let vm: any = null;
  let loading = false;
  let error = "";
  let targets: TargetInfo[] = [];
  let selectedTargetId = "";
  let xmlByTarget = new Map<string, string>();
  let lastProjectRef: Uint8Array | ArrayBuffer | null = null;
  let translate = t;
  $: translate = $translator;
  let activeLocale = "en";
  let localeUnsub: (() => void) | null = null;

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
      await vm.loadProject(projectData);

      const rawTargets = (vm?.runtime?.targets ?? []).filter(
        (target: any) =>
          !Object.prototype.hasOwnProperty.call(target, "isOriginal") ||
          target.isOriginal,
      );
      const mapped: TargetInfo[] = rawTargets.map((target: any) => ({
        id: target.id,
        name: typeof target.getName === "function" ? target.getName() : target.name ?? "Sprite",
        isStage: !!target.isStage,
        hasBlocks: Array.isArray(target?.blocks?._scripts)
          ? target.blocks._scripts.length > 0
          : true,
      }));
      targets = mapped.sort((a, b) => {
        if (a.isStage && !b.isStage) return -1;
        if (!a.isStage && b.isStage) return 1;
        return a.name.localeCompare(b.name);
      });

      for (const target of rawTargets) {
        xmlByTarget.set(target.id, buildWorkspaceXml(target));
      }

      selectedTargetId =
        targets.find((t) => t.isStage)?.id ?? targets[0]?.id ?? "";
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
    if (projectData) {
      void loadProject();
    }
  });

  onDestroy(() => {
    localeUnsub?.();
    cleanup();
  });
</script>

<div class="space-y-4">
  {#if loading}
    <div class="flex items-center gap-2 text-sm opacity-70">
      <span class="loading loading-spinner loading-sm"></span>
      {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_loading")}
    </div>
  {:else if error}
    <div class="alert bg-error/10 border-error/20 text-error-content rounded-2xl">
      <span class="font-medium text-sm">{error}</span>
    </div>
  {:else if !targets.length}
    <div class="alert bg-warning/10 border-warning/20 text-warning-content rounded-2xl">
      <span class="font-medium text-sm">
        {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_empty_project")}
      </span>
    </div>
  {/if}

  {#if targets.length}
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="text-[9px] font-black uppercase tracking-widest opacity-40">
        {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_target_label")}
      </div>
      <select
        class="select select-bordered select-sm min-w-[180px] font-black text-xs"
        bind:value={selectedTargetId}
      >
        {#each targets as target}
          <option value={target.id}>{target.name}</option>
        {/each}
      </select>
    </div>
  {/if}

  {#if targets.find((t) => t.id === selectedTargetId && !t.hasBlocks)}
    <div class="alert bg-warning/10 border-warning/20 text-warning-content rounded-2xl">
      <span class="font-medium text-sm">
        {t("frontend/src/lib/components/ScratchBlocksViewer.svelte::scratch_blocks_empty_target")}
      </span>
    </div>
  {/if}

  <div class="scratch-blocks-host" bind:this={workspaceHost}></div>
</div>

<style>
  .scratch-blocks-host {
    width: 100%;
    height: 520px;
    border-radius: 1rem;
    overflow: hidden;
    background: hsl(var(--b1));
    border: 1px solid hsl(var(--b3) / 0.6);
  }

  @media (min-width: 768px) {
    .scratch-blocks-host {
      height: 620px;
    }
  }
</style>

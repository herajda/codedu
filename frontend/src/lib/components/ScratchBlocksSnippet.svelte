<script context="module" lang="ts">
  let scratchblocksInstance: any = null;
  let scratchblocksPromise: Promise<any> | null = null;
  let languagesLoaded = false;

  async function getScratchblocks() {
    if (scratchblocksInstance) return scratchblocksInstance;
    if (!scratchblocksPromise) {
      scratchblocksPromise = import("scratchblocks").then(
        (mod) => mod.default ?? mod,
      );
    }
    scratchblocksInstance = await scratchblocksPromise;
    return scratchblocksInstance;
  }

  async function ensureLanguages(scratchblocks: any) {
    if (languagesLoaded) return;
    if (typeof scratchblocks?.loadLanguages !== "function") {
      languagesLoaded = true;
      return;
    }
    try {
      const csLocale = await import("scratchblocks/locales/cs.json");
      scratchblocks.loadLanguages({
        cs: csLocale.default ?? csLocale,
      });
    } catch {
      // Fallback to English if the locale fails to load.
    }
    languagesLoaded = true;
  }
</script>

<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { browser } from "$app/environment";
  import { locale as localeStore } from "$lib/i18n";

  export let script: string | string[] = "";
  export let inline = false;
  export let scale = 0.85;

  let host: HTMLDivElement | null = null;
  let activeLocale = "en";
  let localeUnsub: (() => void) | null = null;
  let scriptText = "";
  let renderToken = 0;

  function normalizeLocale(value: string) {
    return value === "cs" ? "cs" : "en";
  }

  function normalizeScript(value: string | string[]) {
    const raw = Array.isArray(value) ? value.join("\n") : String(value ?? "");
    return raw.replace(/\r\n?/g, "\n");
  }

  async function renderBlocks() {
    if (!browser || !host) return;
    const text = scriptText.trim();
    if (!text) {
      host.innerHTML = "";
      return;
    }

    const currentToken = ++renderToken;
    try {
      const scratchblocks = await getScratchblocks();
      await ensureLanguages(scratchblocks);
      if (currentToken !== renderToken || !host) return;

      const hasCs = !!scratchblocks.allLanguages?.cs;
      const languages =
        activeLocale === "cs" && hasCs ? ["cs", "en"] : ["en"];
      const options = { style: "scratch3", inline, languages, scale };
      const doc = scratchblocks.parse(text, options);
      const svg = scratchblocks.render(doc, options);
      if (currentToken !== renderToken || !host) return;
      scratchblocks.replace(host, svg, doc, options);
    } catch {
      if (currentToken !== renderToken || !host) return;
      host.innerHTML = "";
      const fallback = document.createElement("pre");
      fallback.className = "scratchblocks-fallback";
      fallback.textContent = text;
      host.appendChild(fallback);
    }
  }

  $: scriptText = normalizeScript(script);
  $: if (browser) {
    scriptText;
    void renderBlocks();
  }

  onMount(() => {
    localeUnsub = localeStore.subscribe((value) => {
      activeLocale = normalizeLocale(value);
      void renderBlocks();
    });
  });

  onDestroy(() => {
    localeUnsub?.();
  });
</script>

<div bind:this={host}></div>

<style>
  .scratchblocks-fallback {
    margin: 0;
    white-space: pre-wrap;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
      "Liberation Mono", "Courier New", monospace;
    font-size: 0.75rem;
    line-height: 1.2;
    color: hsl(var(--bc) / 0.7);
  }
</style>

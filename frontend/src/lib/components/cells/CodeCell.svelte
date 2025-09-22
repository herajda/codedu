<script lang="ts">
  import {
    notebookStore,
    moveCellUp,
    moveCellDown,
    insertCell,
    deleteCell
  } from "$lib/stores/notebookStore";
  import { initPyodide, terminatePyodide } from "$lib/pyodide";
  import { onMount } from 'svelte';
  import { writable } from "svelte/store";
  import { cellSourceToString } from "$lib/notebook";
  import { python } from "@codemirror/lang-python";
  import CodeMirror from "../ui/CodeMirror.svelte";
  import OutputBlock from "./OutputBlock.svelte";
  import ImageOutput from "./ImageOutput.svelte";

  export let cell: import("$lib/notebook").NotebookCell;
  export let index: number;
  let showInsert = false;
  let insertPos: 'above' | 'below' | null = null;

  // always keep a local string copy
  let sourceStr = cellSourceToString(cell.source);

  const running = writable(false);
  const stdoutStore = writable<string>("");
  const stderrStore = writable<string>("");
  const resultStore = writable<any>(null);
  const resultTextStore = writable<string | null>(null);
  const imagesStore = writable<string[]>([]);

  onMount(() => {
    if (!Array.isArray(cell.outputs)) return;
    let stdout = "";
    let stderr = "";
    let result: any = null;
    let resultText: string | null = null;
    const imgs: string[] = [];
    for (const o of cell.outputs) {
      if (o.output_type === 'stream') {
        if (o.name === 'stdout') stdout += o.text ?? '';
        if (o.name === 'stderr') stderr += o.text ?? '';
      } else if (o.output_type === 'execute_result') {
        resultText = o.data?.['text/plain'] ?? resultText;
        if (result === null) {
          result = resultText;
        }
      } else if (o.output_type === 'display_data') {
        if (o.data?.['image/png']) imgs.push(o.data['image/png']);
      }
    }
    stdoutStore.set(stdout);
    stderrStore.set(stderr);
    resultStore.set(result);
    resultTextStore.set(resultText ?? (result !== null && result !== undefined ? String(result) : null));
    imagesStore.set(imgs);
  });

  function onChange(value: string) {
    sourceStr = value;
    cell.source = sourceStr; // keep store canonical
  }

  async function run() {
    running.set(true);
    const py = await initPyodide();
    try {
      const { result, resultText, stdout, stderr, images } = await py.runCell(sourceStr);
      stdoutStore.set(stdout);
      stderrStore.set(stderr);
      resultStore.set(result);
      const displayResult = resultText ?? (result !== null && result !== undefined ? String(result) : null);
      resultTextStore.set(displayResult);
      imagesStore.set(images ?? []);

      // Save in a nbformat-esque shape so we can rehydrate later.
      const outputs: any[] = [];
      if (stdout) {
        outputs.push({
          output_type: "stream",
          name: "stdout",
          text: stdout
        });
      }
      if (stderr) {
        outputs.push({
          output_type: "stream",
          name: "stderr",
          text: stderr
        });
      }
      // Represent result (if any) as a display_data-ish object
      if (displayResult !== null && displayResult !== undefined) {
        outputs.push({
          output_type: "execute_result",
          data: { "text/plain": displayResult },
          metadata: {},
          execution_count: null
        });
      }
      if (images && images.length) {
        for (const img of images) {
          outputs.push({
            output_type: "display_data",
            data: { "image/png": img },
            metadata: {}
          });
        }
      }
      cell.outputs = outputs;
    } catch (err) {
      stderrStore.set(String(err));
      resultStore.set(null);
      resultTextStore.set(null);
      cell.outputs = [
        {
          output_type: "stream",
          name: "stderr",
          text: String(err)
        }
      ];
    } finally {
      running.set(false);
      // trigger store update so parent re-renders
      notebookStore.update((n) => ({ ...n! }));
    }
  }

  /** Allow parent components to trigger execution. */
  export async function runFromParent() {
    await run();
  }

  function stop() {
    terminatePyodide();
    running.set(false);
  }
</script>

<div class="border rounded-lg p-3 space-y-2 bg-white shadow-inner group relative">
  <CodeMirror
    class="w-full text-sm"
    bind:value={sourceStr}
    lang={python()}
    on:change={(e) => onChange(e.detail)}
  />
  <div class="flex gap-2 items-center">
    <button
      size="sm"
      aria-label="Run cell"
      title="Run cell"
      on:click={run}
      disabled={$running}
      class="p-1 rounded text-green-600 hover:text-white hover:bg-green-600 hover:scale-110 transition-transform disabled:opacity-50"
    >
      <svg
        class="w-4 h-4"
        viewBox="0 0 24 24"
        fill="currentColor"
        aria-hidden="true"
      >
        <path d="M5 3l14 9-14 9V3z" />
      </svg>
    </button>
    <button
      size="sm"
      variant="destructive"
      aria-label="Stop cell"
      title="Stop cell"
      on:click={stop}
      class="p-1 rounded text-red-600 hover:text-white hover:bg-red-600 hover:scale-110 transition-transform"
    >
      <svg
        class="w-4 h-4"
        viewBox="0 0 24 24"
        fill="currentColor"
        aria-hidden="true"
      >
        <path d="M6 6h12v12H6z" />
      </svg>
    </button>
    {#if $running}
      <span class="animate-pulse text-xs ml-2">Running…</span>
    {/if}
    <div class="flex gap-2 ml-auto opacity-0 group-hover:opacity-100 items-center">
      <button
        aria-label="Move cell up"
        title="Move cell up"
        on:click={() => moveCellUp(index)}
        class="p-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M12 4l-6 6h4v6h4v-6h4l-6-6z" />
        </svg>
      </button>
      <button
        aria-label="Move cell down"
        title="Move cell down"
        on:click={() => moveCellDown(index)}
        class="p-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M12 20l6-6h-4v-6h-4v6H6l6 6z" />
        </svg>
      </button>
      <button
        aria-label="Delete cell"
        title="Delete cell"
        on:click={() => deleteCell(index)}
        class="p-1 rounded text-gray-600 hover:text-white hover:bg-red-600 hover:scale-110 transition-transform"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M6 7h12M9 7v10m6-10v10M4 7h16l-1 12a2 2 0 01-2 2H7a2 2 0 01-2-2L4 7zM10 4h4" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round"/>
        </svg>
      </button>
      <div class="relative">
        <button
          aria-label="Insert cell"
          title="Insert cell"
          on:click={() => { showInsert = !showInsert; if (!showInsert) insertPos = null; }}
          class="p-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
        >
          <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" />
          </svg>
        </button>
        {#if showInsert}
          <div class="absolute right-0 mt-1 z-10 bg-white border rounded shadow flex flex-col text-sm">
            {#if !insertPos}
              <button class="p-1 hover:bg-gray-100" aria-label="Insert above" title="Insert above" on:click={() => (insertPos = 'above')}>
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M12 4l-6 6h4v6h4v-6h4l-6-6z" />
                </svg>
              </button>
              <button class="p-1 hover:bg-gray-100" aria-label="Insert below" title="Insert below" on:click={() => (insertPos = 'below')}>
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M12 20l6-6h-4v-6h-4v6H6l6 6z" />
                </svg>
              </button>
            {:else}
              <button class="p-1 hover:bg-gray-100" aria-label="Insert code" title="Insert code" on:click={() => {insertCell(index, 'code', insertPos); showInsert = false; insertPos = null;}}>
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M16 18l6-6-6-6M8 6L2 12l6 6" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              </button>
              <button class="p-1 hover:bg-gray-100" aria-label="Insert markdown" title="Insert markdown" on:click={() => {insertCell(index, 'markdown', insertPos); showInsert = false; insertPos = null;}}>
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M6 2a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6H6z" />
                </svg>
              </button>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  </div>

  <!-- Outputs -->
  {#if $stdoutStore}
  <OutputBlock label="stdout" text={$stdoutStore} />
  {/if}
  {#if $stderrStore}
  <OutputBlock label="stderr" text={$stderrStore} />
  {/if}
  {#if $resultTextStore !== null && $resultTextStore !== undefined}
  <OutputBlock label="result" text={$resultTextStore} />
  {/if}
  {#each $imagesStore as img}
  <ImageOutput src={img} />
  {/each}
</div>

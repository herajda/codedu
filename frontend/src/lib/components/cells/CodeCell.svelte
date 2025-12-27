<script lang="ts">
  import {
    notebookStore,
    moveCellUp,
    moveCellDown,
    insertCell,
    deleteCell,
  } from "$lib/stores/notebookStore";
  import { initPyodide, terminatePyodide } from "$lib/pyodide";
  import { onMount, tick } from "svelte";
  import { writable } from "svelte/store";
  import { cellSourceToString } from "$lib/notebook";
  import { python } from "@codemirror/lang-python";
  import CodeMirror from "../ui/CodeMirror.svelte";
  import OutputBlock from "./OutputBlock.svelte";
  import ImageOutput from "./ImageOutput.svelte";
  import { t } from "$lib/i18n";
  import { Play, Square, ArrowUp, ArrowDown, Trash2, Plus, Code as CodeIcon, FileText } from 'lucide-svelte';

  export let cell: import("$lib/notebook").NotebookCell;
  export let index: number;
  let showInsert = false;
  let insertPos: "above" | "below" | null = null;

  // always keep a local string copy
  let sourceStr = cellSourceToString(cell.source);

  let awaitingInput = false;
  let inputValue = "";
  let inputTextarea: HTMLTextAreaElement | null = null;
  let inputPrompt: string = "";
  // Accumulate provided input lines across multiple submissions so that
  // re-executions (which start from the top) get all prior inputs.
  let accumulatedInputs: string[] = [];

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
      if (o.output_type === "stream") {
        if (o.name === "stdout") stdout += o.text ?? "";
        if (o.name === "stderr") stderr += o.text ?? "";
      } else if (o.output_type === "execute_result") {
        resultText = o.data?.["text/plain"] ?? resultText;
        if (result === null) {
          result = resultText;
        }
      } else if (o.output_type === "display_data") {
        if (o.data?.["image/png"]) imgs.push(o.data["image/png"]);
      }
    }
    stdoutStore.set(stdout);
    stderrStore.set(stderr);
    resultStore.set(result);
    resultTextStore.set(
      resultText ??
        (result !== null && result !== undefined ? String(result) : null),
    );
    imagesStore.set(imgs);
  });

  function onChange(value: string) {
    sourceStr = value;
    cell.source = sourceStr; // keep store canonical
  }

  // Simple heuristic for UI affordance; actual runtime input needs are
  // detected by the worker and reported back via `inputRequired`.
  $: expectsInput = /\binput\s*\(/.test(sourceStr);

  async function executeCell(stdin?: string) {
    awaitingInput = false;
    running.set(true);
    const py = await initPyodide();
    try {
      const {
        result,
        resultText,
        stdout,
        stderr,
        images,
        inputRequired,
        prompt,
      } = await py.runCell(sourceStr, stdin);
      if (inputRequired) {
        // Runtime requested input. Open the UI and return without mutating outputs.
        awaitingInput = true;
        inputPrompt = prompt ?? "";
        running.set(false);
        return;
      }
      stdoutStore.set(stdout);
      stderrStore.set(stderr);
      resultStore.set(result);
      const displayResult =
        resultText ??
        (result !== null && result !== undefined ? String(result) : null);
      resultTextStore.set(displayResult);
      imagesStore.set(images ?? []);

      // Save in a nbformat-esque shape so we can rehydrate later.
      const outputs: any[] = [];
      if (stdout) {
        outputs.push({
          output_type: "stream",
          name: "stdout",
          text: stdout,
        });
      }
      if (stderr) {
        outputs.push({
          output_type: "stream",
          name: "stderr",
          text: stderr,
        });
      }
      // Represent result (if any) as a display_data-ish object
      if (displayResult !== null && displayResult !== undefined) {
        outputs.push({
          output_type: "execute_result",
          data: { "text/plain": displayResult },
          metadata: {},
          execution_count: null,
        });
      }
      if (images && images.length) {
        for (const img of images) {
          outputs.push({
            output_type: "display_data",
            data: { "image/png": img },
            metadata: {},
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
          text: String(err),
        },
      ];
    } finally {
      running.set(false);
      // trigger store update so parent re-renders
      notebookStore.update((n) => ({ ...n! }));
    }
  }

  // If called by parent (Run All), try executing; if runtime reports
  // that input is required, open the input UI and wait for user.
  let parentAwaitResolve: (() => void) | null = null;
  let waitingForParent = false;
  /** Allow parent components to trigger execution. */
  export async function runFromParent(interactive: boolean = true) {
    // Attempt to execute; if runtime signals input is required, open the UI
    // and pause until the user provides it. Do not move focus for Run All.
    await executeCell();
    if (interactive && awaitingInput) {
      waitingForParent = true;
      await new Promise<void>((resolve) => {
        parentAwaitResolve = resolve;
      });
      waitingForParent = false;
    }
  }

  async function handleRunClick() {
    await executeCell();
    if (awaitingInput) {
      await tick();
      inputTextarea?.focus();
    }
  }

  async function submitInput() {
    // Normalize new lines (CRLF -> LF), split, drop trailing empty line
    const normalized = (inputValue ?? "").replace(/\r\n/g, "\n");
    const pieces = normalized.split("\n");
    if (pieces.length && pieces[pieces.length - 1] === "") pieces.pop();
    accumulatedInputs.push(...pieces);

    const toSend = accumulatedInputs.join("\n");
    await executeCell(toSend);
    inputValue = "";
    // If more input is still required, stay in awaiting state and keep accumulating.
    if (!awaitingInput) {
      // Completed successfully; clear accumulation and resolve Run All if waiting.
      accumulatedInputs = [];
      inputPrompt = "";
      if (parentAwaitResolve) {
        const resolve = parentAwaitResolve;
        parentAwaitResolve = null;
        resolve();
      }
    }
  }

  function cancelInput() {
    awaitingInput = false;
    inputValue = "";
    inputPrompt = "";
    accumulatedInputs = [];
    if (waitingForParent && parentAwaitResolve) {
      const resolve = parentAwaitResolve;
      parentAwaitResolve = null;
      waitingForParent = false;
      resolve();
    }
  }

  function stop() {
    terminatePyodide();
    running.set(false);
    accumulatedInputs = [];
    inputPrompt = "";
  }
</script>

<div
  class="bg-base-100/50 backdrop-blur-sm rounded-[2rem] border-2 border-primary/10 p-5 shadow-xl shadow-primary/5 hover:shadow-2xl hover:shadow-primary/10 transition-all duration-300 group relative hover:border-primary/30 mx-1 {showInsert ? 'z-50' : 'z-auto'}"
>
  <!-- Cell Type Indicator -->
  <div class="absolute -top-3 left-8 flex items-center gap-2">
    {#if $running}
      <div class="bg-success text-success-content px-3 py-1 rounded-full text-[10px] font-black tracking-widest flex items-center gap-1.5 shadow-lg shadow-success/20 animate-pulse">
        <div class="w-1.5 h-1.5 rounded-full bg-current"></div>
        RUNNING
      </div>
    {/if}
  </div>

  <div class="relative rounded-2xl border border-base-200 shadow-inner bg-base-300/30 mt-2 focus-within:border-primary/40 focus-within:ring-4 focus-within:ring-primary/5 focus-within:bg-base-300/50 transition-all duration-300 group/editor">
     <CodeMirror
       class="w-full text-base"
       bind:value={sourceStr}
       lang={python()}
       on:change={(e) => onChange(e.detail)}
     />
     <div class="absolute right-3 top-3 opacity-20 pointer-events-none group-focus-within/editor:opacity-100 transition-opacity">
        <CodeIcon size={14} class="text-primary" />
     </div>
  </div>

  <div class="flex gap-2 items-center mt-5">
    <div class="flex items-center gap-1 bg-base-200/50 p-1 rounded-xl border border-base-content/5">
      <button
        type="button"
        aria-label={t("frontend/src/lib/components/cells/CodeCell.svelte::run_cell")}
        title={t("frontend/src/lib/components/cells/CodeCell.svelte::run_cell")}
        on:click={handleRunClick}
        disabled={$running}
        class="btn btn-sm btn-circle btn-ghost text-success hover:bg-success/20 hover:text-success disabled:opacity-50 transition-all"
      >
        <Play size={16} fill="currentColor" class="ml-0.5" />
      </button>
      <button
        type="button"
        aria-label={t("frontend/src/lib/components/cells/CodeCell.svelte::stop_cell")}
        title={t("frontend/src/lib/components/cells/CodeCell.svelte::stop_cell")}
        on:click={stop}
        class="btn btn-sm btn-circle btn-ghost text-error hover:bg-error/20 hover:text-error transition-all"
      >
        <Square size={16} fill="currentColor" />
      </button>
    </div>
    
    <div
      class="flex gap-1 ml-auto items-center transition-all duration-300 {showInsert ? 'opacity-100 translate-x-0' : 'opacity-0 group-hover:opacity-100 translate-x-2 group-hover:translate-x-0'}"
    >
      <div class="flex items-center gap-1 bg-base-200/50 p-1 rounded-xl border border-base-content/5 shadow-sm">
        <button
          aria-label={t("frontend/src/lib/components/cells/CodeCell.svelte::move_cell_up")}
          title={t("frontend/src/lib/components/cells/CodeCell.svelte::move_cell_up")}
          on:click={() => moveCellUp(index)}
          class="btn btn-xs btn-circle btn-ghost hover:bg-base-300"
        >
          <ArrowUp size={14} />
        </button>
        <button
          aria-label={t("frontend/src/lib/components/cells/CodeCell.svelte::move_cell_down")}
          title={t("frontend/src/lib/components/cells/CodeCell.svelte::move_cell_down")}
          on:click={() => moveCellDown(index)}
          class="btn btn-xs btn-circle btn-ghost hover:bg-base-300"
        >
          <ArrowDown size={14} />
        </button>
      </div>

      <button
        aria-label={t("frontend/src/lib/components/cells/CodeCell.svelte::delete_cell")}
        title={t("frontend/src/lib/components/cells/CodeCell.svelte::delete_cell")}
        on:click={() => deleteCell(index)}
        class="btn btn-sm btn-circle btn-ghost text-error hover:bg-error/10 hover:text-error mx-1"
      >
        <Trash2 size={16} />
      </button>

      <div class="relative dropdown dropdown-end dropdown-bottom {showInsert ? 'dropdown-open' : ''}">
        <button
          aria-label={t("frontend/src/lib/components/cells/CodeCell.svelte::insert_cell")}
          title={t("frontend/src/lib/components/cells/CodeCell.svelte::insert_cell")}
          on:click={() => { showInsert = !showInsert; if (!showInsert) insertPos = null; }}
          class="btn btn-sm btn-circle btn-ghost bg-primary/10 text-primary hover:bg-primary/20"
        >
          <Plus size={18} />
        </button>
        
        {#if showInsert}
          <ul class="dropdown-content z-50 menu p-2 shadow-2xl bg-base-100 rounded-2xl w-52 border border-base-200 mt-2 ring-1 ring-black/5 animate-in fade-in zoom-in duration-200">
            {#if !insertPos}
              <li class="menu-title text-[10px] font-black tracking-widest uppercase opacity-50 px-4 py-2">{t("frontend/src/lib/components/cells/CodeCell.svelte::position")}</li>
              <li>
                  <button class="rounded-xl py-3" on:click={() => (insertPos = "above")}>
                    <ArrowUp size={14} class="text-primary" /> 
                    {t("frontend/src/lib/components/cells/CodeCell.svelte::insert_above")}
                  </button>
              </li>
              <li>
                  <button class="rounded-xl py-3" on:click={() => (insertPos = "below")}>
                    <ArrowDown size={14} class="text-primary" /> 
                    {t("frontend/src/lib/components/cells/CodeCell.svelte::insert_below")}
                  </button>
              </li>
            {:else}
              <li class="menu-title text-[10px] font-black tracking-widest uppercase opacity-50 px-4 py-2">Type ({insertPos})</li>
              <li>
                  <button class="rounded-xl py-3" on:click={() => {
                        insertCell(index, "code", insertPos);
                        showInsert = false;
                        insertPos = null;
                      }}>
                    <CodeIcon size={14} class="text-primary" /> 
                    {t("frontend/src/lib/components/cells/CodeCell.svelte::insert_code")}
                  </button>
              </li>
              <li>
                  <button class="rounded-xl py-3" on:click={() => {
                        insertCell(index, "markdown", insertPos);
                        showInsert = false;
                        insertPos = null;
                      }}>
                    <FileText size={14} class="text-secondary" /> 
                    {t("frontend/src/lib/components/cells/CodeCell.svelte::insert_markdown")}
                  </button>
              </li>
            {/if}
          </ul>
        {/if}
      </div>
    </div>
  </div>

  {#if awaitingInput}
    <div class="border-2 border-primary/20 rounded-[2rem] p-6 bg-primary/5 space-y-4 mt-6 animate-in slide-in-from-top-4 duration-300">
      <div class="flex items-center gap-3">
         <div class="w-10 h-10 rounded-xl bg-primary/20 flex items-center justify-center text-primary">
            <Plus size={20} />
         </div>
         <div>
            <div class="text-xs font-black uppercase tracking-widest text-primary">
              {t("frontend/src/lib/components/cells/CodeCell.svelte::input_for_this_run")}
            </div>
            {#if inputPrompt}
              <div class="text-sm font-bold opacity-80 mt-0.5">{inputPrompt}</div>
            {/if}
         </div>
      </div>
      
      <textarea
        class="textarea textarea-bordered w-full rounded-2xl bg-base-100/50 focus:ring-4 focus:ring-primary/10 transition-all border-2 text-base"
        rows="2"
        bind:this={inputTextarea}
        bind:value={inputValue}
        tabindex={waitingForParent ? -1 : 0}
        placeholder={t("frontend/src/lib/components/cells/CodeCell.svelte::input_placeholder")}
      ></textarea>
      
      <div class="flex items-center gap-3">
        <button
          type="button"
          class="btn btn-primary rounded-xl px-6 font-bold shadow-lg shadow-primary/20"
          on:click={submitInput}
        >
          {t("frontend/src/lib/components/cells/CodeCell.svelte::send")}
        </button>
        <button
          type="button"
          class="btn btn-ghost rounded-xl px-6 font-bold"
          on:click={cancelInput}
        >
          {t("frontend/src/lib/components/cells/CodeCell.svelte::cancel")}
        </button>
        <span class="text-[10px] opacity-40 uppercase tracking-widest font-black ml-auto"
          >{t("frontend/src/lib/components/cells/CodeCell.svelte::leave_blank_for_empty_line")}</span
        >
      </div>
    </div>
  {/if}

  <!-- Outputs -->
  <div class="space-y-4 mt-6">
    {#if $stdoutStore}
      <div class="rounded-2xl overflow-hidden border border-emerald-500/20 bg-emerald-500/5 shadow-sm ring-1 ring-emerald-500/10">
        <OutputBlock
          label={t("frontend/src/lib/components/cells/CodeCell.svelte::stdout_label")}
          text={$stdoutStore}
        />
      </div>
    {/if}
    {#if $stderrStore}
      <div class="rounded-2xl overflow-hidden border border-rose-500/20 bg-rose-500/5 shadow-sm ring-1 ring-rose-500/10">
        <OutputBlock
          label={t("frontend/src/lib/components/cells/CodeCell.svelte::stderr_label")}
          text={$stderrStore}
        />
      </div>
    {/if}
    {#if $resultTextStore !== null && $resultTextStore !== undefined}
      <div class="rounded-2xl overflow-hidden border border-primary/20 bg-primary/5 shadow-sm ring-1 ring-primary/10">
        <OutputBlock
          label={t("frontend/src/lib/components/cells/CodeCell.svelte::result_label")}
          text={$resultTextStore}
        />
      </div>
    {/if}
    
    {#if $imagesStore.length > 0}
      <div class="flex flex-wrap gap-4 pt-2">
        {#each $imagesStore as img}
          <div class="rounded-[2rem] overflow-hidden shadow-2xl border border-base-200 bg-white p-2 hover:scale-[1.02] transition-transform duration-300">
             <ImageOutput src={img} />
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

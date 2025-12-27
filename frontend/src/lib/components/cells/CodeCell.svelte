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
  class="bg-base-100 rounded-2xl border-y border-r border-l-4 border-l-primary/60 border-base-200 p-4 shadow-sm hover:shadow-lg transition-all group relative hover:border-l-primary"
>
  <div class="relative rounded-xl overflow-hidden border border-base-200">
     <CodeMirror
       class="w-full text-sm"
       bind:value={sourceStr}
       lang={python()}
       on:change={(e) => onChange(e.detail)}
     />
  </div>

  <div class="flex gap-2 items-center mt-3">
    <button
      type="button"
      aria-label={t(
        "frontend/src/lib/components/cells/CodeCell.svelte::run_cell",
      )}
      title={t("frontend/src/lib/components/cells/CodeCell.svelte::run_cell")}
      on:click={handleRunClick}
      disabled={$running}
      class="btn btn-xs btn-circle btn-ghost text-success hover:bg-success/10 hover:text-success disabled:opacity-50"
    >
      <Play size={14} class="ml-0.5" />
    </button>
    <button
      type="button"
      aria-label={t(
        "frontend/src/lib/components/cells/CodeCell.svelte::stop_cell",
      )}
      title={t("frontend/src/lib/components/cells/CodeCell.svelte::stop_cell")}
      on:click={stop}
      class="btn btn-xs btn-circle btn-ghost text-error hover:bg-error/10 hover:text-error"
    >
      <Square size={14} />
    </button>
    {#if $running}
      <span class="loading loading-spinner loading-xs text-primary ml-2"></span>
      <span class="text-xs opacity-50 font-bold uppercase tracking-widest">{t("frontend/src/lib/components/cells/CodeCell.svelte::running")}</span>
    {/if}
    
    <div
      class="flex gap-1 ml-auto opacity-0 group-hover:opacity-100 items-center transition-opacity"
    >
      <button
        aria-label={t(
          "frontend/src/lib/components/cells/CodeCell.svelte::move_cell_up",
        )}
        title={t(
          "frontend/src/lib/components/cells/CodeCell.svelte::move_cell_up",
        )}
        on:click={() => moveCellUp(index)}
        class="btn btn-xs btn-circle btn-ghost opacity-60 hover:opacity-100"
      >
        <ArrowUp size={14} />
      </button>
      <button
        aria-label={t(
          "frontend/src/lib/components/cells/CodeCell.svelte::move_cell_down",
        )}
        title={t(
          "frontend/src/lib/components/cells/CodeCell.svelte::move_cell_down",
        )}
        on:click={() => moveCellDown(index)}
        class="btn btn-xs btn-circle btn-ghost opacity-60 hover:opacity-100"
      >
        <ArrowDown size={14} />
      </button>
      <button
        aria-label={t(
          "frontend/src/lib/components/cells/CodeCell.svelte::delete_cell",
        )}
        title={t(
          "frontend/src/lib/components/cells/CodeCell.svelte::delete_cell",
        )}
        on:click={() => deleteCell(index)}
        class="btn btn-xs btn-circle btn-ghost text-error hover:bg-error/10 hover:text-error opacity-60 hover:opacity-100"
      >
        <Trash2 size={14} />
      </button>

      <div class="relative dropdown dropdown-end dropdown-bottom {showInsert ? 'dropdown-open' : ''}">
        <button
          aria-label={t(
            "frontend/src/lib/components/cells/CodeCell.svelte::insert_cell",
          )}
          title={t(
            "frontend/src/lib/components/cells/CodeCell.svelte::insert_cell",
          )}
          on:click={() => { showInsert = !showInsert; if (!showInsert) insertPos = null; }}
          class="btn btn-xs btn-circle btn-ghost opacity-60 hover:opacity-100"
        >
          <Plus size={14} />
        </button>
        
        {#if showInsert}
          <ul class="dropdown-content z-50 menu p-2 shadow-xl bg-base-100 rounded-box w-48 border border-base-200 mt-1">
            {#if !insertPos}
              <li>
                  <button on:click={() => (insertPos = "above")}>
                    <ArrowUp size={14} /> 
                    {t("frontend/src/lib/components/cells/CodeCell.svelte::insert_above")}
                  </button>
              </li>
              <li>
                  <button on:click={() => (insertPos = "below")}>
                    <ArrowDown size={14} /> 
                    {t("frontend/src/lib/components/cells/CodeCell.svelte::insert_below")}
                  </button>
              </li>
            {:else}
              <li>
                  <button on:click={() => {
                        insertCell(index, "code", insertPos);
                        showInsert = false;
                        insertPos = null;
                      }}>
                    <CodeIcon size={14} /> 
                    {t("frontend/src/lib/components/cells/CodeCell.svelte::insert_code")}
                  </button>
              </li>
              <li>
                  <button on:click={() => {
                        insertCell(index, "markdown", insertPos);
                        showInsert = false;
                        insertPos = null;
                      }}>
                    <FileText size={14} /> 
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
    <div class="border border-primary/20 rounded-xl p-4 bg-primary/5 space-y-3 mt-4">
      <div class="text-xs font-black uppercase tracking-widest text-primary">
        {t(
          "frontend/src/lib/components/cells/CodeCell.svelte::input_for_this_run",
        )}
      </div>
      {#if inputPrompt}
        <div class="text-sm font-medium opacity-80">{inputPrompt}</div>
      {/if}
      <textarea
        class="textarea textarea-bordered w-full"
        rows="3"
        bind:this={inputTextarea}
        bind:value={inputValue}
        tabindex={waitingForParent ? -1 : 0}
        placeholder={t(
          "frontend/src/lib/components/cells/CodeCell.svelte::input_placeholder",
        )}
      ></textarea>
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="btn btn-sm btn-primary"
          on:click={submitInput}
        >
          {t("frontend/src/lib/components/cells/CodeCell.svelte::send")}
        </button>
        <button
          type="button"
          class="btn btn-sm btn-ghost"
          on:click={cancelInput}
        >
          {t("frontend/src/lib/components/cells/CodeCell.svelte::cancel")}
        </button>
        <span class="text-[10px] opacity-40 uppercase tracking-widest font-bold ml-auto"
          >{t(
            "frontend/src/lib/components/cells/CodeCell.svelte::leave_blank_for_empty_line",
          )}</span
        >
      </div>
    </div>
  {/if}

  <!-- Outputs -->
  {#if $stdoutStore}
    <OutputBlock
      label={t(
        "frontend/src/lib/components/cells/CodeCell.svelte::stdout_label",
      )}
      text={$stdoutStore}
    />
  {/if}
  {#if $stderrStore}
    <OutputBlock
      label={t(
        "frontend/src/lib/components/cells/CodeCell.svelte::stderr_label",
      )}
      text={$stderrStore}
    />
  {/if}
  {#if $resultTextStore !== null && $resultTextStore !== undefined}
    <div class="mt-2">
      <OutputBlock
        label={t(
          "frontend/src/lib/components/cells/CodeCell.svelte::result_label",
        )}
        text={$resultTextStore}
      />
    </div>
  {/if}
  {#each $imagesStore as img}
    <div class="mt-2 rounded-xl overflow-hidden shadow-sm inline-block">
       <ImageOutput src={img} />
    </div>
  {/each}
</div>

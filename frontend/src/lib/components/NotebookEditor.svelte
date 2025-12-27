<script lang="ts">
import { notebookStore, insertCell } from "$lib/stores/notebookStore";
import pkg from "file-saver";
import CodeCell from "./cells/CodeCell.svelte";
import MarkdownCell from "./cells/MarkdownCell.svelte";
import { v4 as uuid } from "uuid";
import { serializeNotebook } from "$lib/notebook";
import { apiFetch } from "$lib/api";
import { auth } from "$lib/auth";
import { onMount, afterUpdate, createEventDispatcher } from 'svelte';
import { fade, fly } from 'svelte/transition';
import { preloadPyodide } from '$lib/pyodide';
import { t, translator } from '$lib/i18n';
import { Play, Download, Save, Plus, FileText, Code as CodeIcon, Check } from 'lucide-svelte';

  export let fileId: string | number | undefined;
  export let fileName: string | undefined = undefined;
  $: nb = $notebookStore;

  let translate: any;
  $: translate = $translator;

  let cellRefs: any[] = [];
  const { saveAs } = pkg;
  const dispatch = createEventDispatcher();

  let showSavedMessage = false;
  let saveTimeout: any;

  let container: HTMLDivElement | null = null;
  // Removed showBottomButton logic as we use sticky header now

  onMount(() => {
    // Preload the Pyodide runtime and common packages in the background.
    // Loading happens inside a Web Worker so it does not block the UI.
    preloadPyodide();
  });

  async function runAllCells(interactive: boolean = true) {
    for (const ref of cellRefs) {
      if (ref && typeof ref.runFromParent === 'function') {
        await ref.runFromParent(interactive);
      }
    }
  }

  function addCell(type: "code" | "markdown") {
    notebookStore.update((current) => {
      if (!current) return current;
      const cells = [...current.cells, { id: uuid(), cell_type: type, source: "", outputs: [] }];
      return { ...current, cells };
    });

  }

  function deriveDownloadName(): string {
    if (!nb) return 'notebook.ipynb';
    const nameFields = [
      fileName,
      nb?.metadata?.name,
      nb?.metadata?.title
    ];
    for (const candidate of nameFields) {
      if (typeof candidate === 'string') {
        const trimmed = candidate.trim();
        if (trimmed) {
          return trimmed.toLowerCase().endsWith('.ipynb') ? trimmed : `${trimmed}.ipynb`;
        }
      }
    }
    return 'notebook.ipynb';
  }

  function exportNotebook() {
    if (!nb) return;
    const json = serializeNotebook(nb);
    const blob = new Blob([json], { type: 'application/json' });
    saveAs(blob, deriveDownloadName());
  }

  async function saveNotebook() {
    // Run all cells to include outputs; avoid blocking on input for save.
    await runAllCells(false);

    const json = serializeNotebook(nb);
    if (fileId && $auth?.role === 'teacher') {
      await apiFetch(`/api/files/${fileId}/content`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: json
      });
      
      showSavedMessage = true;
      if (saveTimeout) clearTimeout(saveTimeout);
      saveTimeout = setTimeout(() => {
        showSavedMessage = false;
      }, 3000);
      
      dispatch('saved');
    } else {
      const blob = new Blob([json], { type: 'application/json' });
      saveAs(blob, 'notebook.ipynb');
    }
  }

  export async function save() {
    await saveNotebook();
  }
</script>

{#if nb}
  <div class="flex flex-col gap-8 relative pb-20" bind:this={container}>
    <!-- Sticky Toolbar -->
    <div class="sticky top-4 z-40 bg-base-100/80 backdrop-blur-2xl rounded-[2rem] border border-white/20 p-3 flex flex-wrap items-center justify-between gap-4 shadow-2xl shadow-primary/10 ring-1 ring-base-content/5 transition-all duration-300 mx-1">
       
       <!-- Left: Add Cells -->
       <div class="flex items-center gap-1 bg-base-content/[0.03] rounded-2xl p-1 border border-base-content/5">
          <div class="tooltip tooltip-bottom tooltip-delayed px-1" data-tip={translate('frontend/src/lib/components/NotebookEditor.svelte::add_code_cell')}>
             <button class="btn btn-sm btn-ghost hover:bg-primary/[0.08] hover:text-primary rounded-[14px] gap-2.5 text-[11px] font-black tracking-wider transition-all h-9 min-h-0 px-4 group/btn" on:click={() => addCell("code")}>
                <CodeIcon size={15} strokeWidth={2.5} class="transition-transform group-hover/btn:scale-110" />
                {translate('frontend/src/lib/components/NotebookEditor.svelte::code_label')}
             </button>
          </div>
          <div class="w-[1px] h-4 bg-base-content/10 mx-0.5"></div>
          <div class="tooltip tooltip-bottom tooltip-delayed px-1" data-tip={translate('frontend/src/lib/components/NotebookEditor.svelte::add_markdown_cell')}>
             <button class="btn btn-sm btn-ghost hover:bg-secondary/[0.08] hover:text-secondary rounded-[14px] gap-2.5 text-[11px] font-black tracking-wider transition-all h-9 min-h-0 px-4 group/btn" on:click={() => addCell("markdown")}>
                <FileText size={15} strokeWidth={2.5} class="transition-transform group-hover/btn:scale-110" />
                {translate('frontend/src/lib/components/NotebookEditor.svelte::text_label')}
             </button>
          </div>
       </div>

       <!-- Right: Actions -->
       <div class="flex items-center gap-2">
          <button class="btn btn-sm bg-success/[0.08] hover:bg-success/[0.15] border border-success/20 hover:border-success/40 text-success gap-2.5 font-black px-6 rounded-xl transition-all h-9 min-h-0 shadow-sm shadow-success/5" on:click={() => runAllCells(true)}>
             <Play size={14} fill="currentColor" />
             {translate('frontend/src/lib/components/NotebookEditor.svelte::run_all')}
          </button>
          
          <div class="h-5 w-[1px] bg-base-content/10 mx-1.5"></div>

          <button class="btn btn-sm btn-ghost gap-2.5 font-bold opacity-60 hover:opacity-100 hover:bg-base-content/[0.05] rounded-xl px-4 h-9 min-h-0 transition-all" on:click={exportNotebook} aria-label={translate('frontend/src/lib/components/NotebookEditor.svelte::download')}>
             <Download size={15} />
             <span class="hidden sm:inline text-xs">{translate('frontend/src/lib/components/NotebookEditor.svelte::download').replace(' (.ipynb)', '')}</span>
          </button>

          {#if $auth?.role === 'teacher' && fileId}
            <button class="btn btn-sm btn-primary gap-2.5 font-black shadow-lg shadow-primary/20 rounded-xl px-6 h-9 min-h-0 hover:scale-[1.02] active:scale-95 transition-all" on:click={saveNotebook} aria-label={translate('frontend/src/lib/components/NotebookEditor.svelte::save_notebook_aria_label')}>
               <Save size={15} />
               <span class="text-xs">{translate('frontend/src/lib/components/NotebookEditor.svelte::save')}</span>
            </button>
          {/if}
       </div>
    </div>

    <!-- Cells Container -->
    <div class="flex flex-col gap-4">
      {#each nb.cells as cell, i (cell.id)}
        <div class="relative group">
           {#if cell.cell_type === "code"}
             <CodeCell
               {cell}
               index={i}
               bind:this={cellRefs[i]}
             />
           {:else}
             <MarkdownCell
               {cell}
               index={i}
               bind:this={cellRefs[i]}
             />
           {/if}
        </div>
        {#if i < nb.cells.length - 1}
          {#if $auth?.role === 'teacher'}
            <div class="relative py-2 flex items-center justify-center group/divider h-10 transition-all">
                <div class="absolute inset-x-0 h-[2px] bg-gradient-to-r from-transparent via-base-content/5 to-transparent group-hover/divider:via-primary/20 transition-all duration-500"></div>
                <div class="flex gap-1 relative bg-base-100/90 backdrop-blur-md p-1 rounded-[18px] border border-base-200 shadow-2xl opacity-0 scale-95 group-hover/divider:opacity-100 group-hover/divider:scale-100 pointer-events-none group-hover/divider:pointer-events-auto transition-all duration-300 z-10 hover:border-primary/30">
                  <button class="btn btn-xs btn-ghost hover:bg-primary/[0.08] text-primary gap-2 px-3 h-8 min-h-0 rounded-xl font-black text-[10px] tracking-wider transition-all" on:click={() => insertCell(i, 'code', 'below')}>
                    <CodeIcon size={13} strokeWidth={2.5} />
                    {translate('frontend/src/lib/components/NotebookEditor.svelte::code_label')}
                  </button>
                  <div class="w-[1px] h-3.5 bg-base-content/10 my-auto mx-0.5"></div>
                  <button class="btn btn-xs btn-ghost hover:bg-secondary/[0.08] text-secondary gap-2 px-3 h-8 min-h-0 rounded-xl font-black text-[10px] tracking-wider transition-all" on:click={() => insertCell(i, 'markdown', 'below')}>
                    <FileText size={13} strokeWidth={2.5} />
                    {translate('frontend/src/lib/components/NotebookEditor.svelte::text_label')}
                  </button>
                </div>
            </div>
          {/if}
        {/if}
      {/each}
    </div>

    <!-- Bottom Add Buttons -->
    <div class="flex flex-col items-center justify-center gap-6 py-16 mt-10 rounded-[3rem] border-2 border-dashed border-base-300 bg-base-200/20 hover:bg-base-200/40 hover:border-primary/30 transition-all group/bottom">
        <div class="flex items-center gap-4">
             <div class="w-12 h-12 rounded-2xl bg-gradient-to-br from-primary/10 to-secondary/10 flex items-center justify-center text-base-content/30 group-hover/bottom:text-primary transition-colors ring-1 ring-base-content/5">
                <Plus size={24} />
             </div>
             <div class="text-center">
                <h3 class="font-black text-lg tracking-tight">{translate('frontend/src/lib/components/NotebookEditor.svelte::need_more_space')}</h3>
                <p class="text-sm opacity-50 font-medium">{translate('frontend/src/lib/components/NotebookEditor.svelte::add_cell_to_continue')}</p>
             </div>
        </div>
        <div class="flex items-center gap-3">
            <button class="btn btn-primary btn-md rounded-2xl gap-2 px-8 shadow-lg shadow-primary/20 hover:scale-105 transition-all" on:click={() => addCell("code")}>
               <CodeIcon size={18} /> 
               {translate('frontend/src/lib/components/NotebookEditor.svelte::add_code_cell')}
            </button>
            <button class="btn btn-secondary btn-md rounded-2xl gap-2 px-8 shadow-lg shadow-secondary/20 hover:scale-105 transition-all" on:click={() => addCell("markdown")}>
               <FileText size={18} />
               {translate('frontend/src/lib/components/NotebookEditor.svelte::add_markdown_cell')}
            </button>
        </div>
    </div>
  </div>
{:else}
  <div class="flex flex-col items-center justify-center py-32 text-center gap-6">
     <div class="relative">
       <div class="absolute inset-0 bg-primary/20 blur-3xl rounded-full"></div>
       <span class="loading loading-spinner loading-lg text-primary relative z-10"></span>
     </div>
     <div>
       <p class="font-black opacity-50 tracking-[0.2em] uppercase text-sm">
         {translate('frontend/src/lib/components/NotebookEditor.svelte::loading')}
       </p>
       <p class="text-xs opacity-40 mt-1">{translate('frontend/src/lib/components/NotebookEditor.svelte::preparing_workspace')}</p>
     </div>
  </div>
{/if}

{#if showSavedMessage}
  <div class="fixed bottom-8 left-1/2 -translate-x-1/2 z-[100]" in:fly={{ y: 20, duration: 400 }} out:fade={{ duration: 300 }}>
    <div class="bg-base-100/90 backdrop-blur-2xl border border-success/30 px-6 py-3.5 rounded-2xl shadow-2xl flex items-center gap-4 ring-1 ring-black/5">
      <div class="w-10 h-10 rounded-xl bg-success/20 flex items-center justify-center text-success shadow-inner">
        <Check size={20} strokeWidth={3} />
      </div>
      <div class="flex flex-col">
        <span class="font-black text-sm tracking-tight text-base-content uppercase">{translate('frontend/src/lib/components/NotebookEditor.svelte::saved_success')}</span>
        <span class="text-[10px] opacity-60 font-medium">{translate('frontend/src/lib/components/NotebookEditor.svelte::save_notebook_aria_label')}</span>
      </div>
      <div class="ml-4 w-12 h-1 bg-success/10 rounded-full overflow-hidden relative">
        <div class="absolute inset-0 bg-success/40 animate-progress"></div>
      </div>
    </div>
  </div>
{/if}

<style>
  @keyframes progress {
    from { width: 100%; }
    to { width: 0%; }
  }
  .animate-progress {
    animation: progress 3s linear forwards;
  }
</style>

<script lang="ts">
import { notebookStore } from "$lib/stores/notebookStore";
import pkg from "file-saver";
import CodeCell from "./cells/CodeCell.svelte";
import MarkdownCell from "./cells/MarkdownCell.svelte";
import { v4 as uuid } from "uuid";
import { serializeNotebook } from "$lib/notebook";
import { apiFetch } from "$lib/api";
import { auth } from "$lib/auth";
import { onMount, afterUpdate, createEventDispatcher } from 'svelte';
import { preloadPyodide } from '$lib/pyodide';
import { t, translator } from '$lib/i18n';
import { Play, Download, Save, Plus, FileText, Code as CodeIcon } from 'lucide-svelte';

  export let fileId: string | number | undefined;
  $: nb = $notebookStore;

  let translate: any;
  $: translate = $translator;

  let cellRefs: any[] = [];
  const { saveAs } = pkg;
  const dispatch = createEventDispatcher();

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
      alert(t('frontend/src/lib/components/NotebookEditor.svelte::saved_alert'));
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
  <div class="flex flex-col gap-6 relative" bind:this={container}>
    <!-- Sticky Toolbar -->
    <div class="sticky top-0 z-40 bg-base-100/80 backdrop-blur-md rounded-2xl border border-base-200 shadow-sm p-2 flex flex-wrap items-center justify-between gap-3">
       
       <!-- Left: Add Cells -->
       <div class="flex items-center gap-1 bg-base-200/50 rounded-xl p-1">
          <div class="tooltip tooltip-bottom" data-tip={translate('frontend/src/lib/components/NotebookEditor.svelte::add_code_cell')}>
             <button class="btn btn-sm btn-ghost rounded-lg gap-2 text-xs font-bold" on:click={() => addCell("code")}>
                <CodeIcon size={14} class="text-primary" />
                Code
             </button>
          </div>
          <div class="w-px h-4 bg-base-content/10 mx-1"></div>
          <div class="tooltip tooltip-bottom" data-tip={translate('frontend/src/lib/components/NotebookEditor.svelte::add_markdown_cell')}>
             <button class="btn btn-sm btn-ghost rounded-lg gap-2 text-xs font-bold" on:click={() => addCell("markdown")}>
                <FileText size={14} class="text-secondary" />
                Markdown
             </button>
          </div>
       </div>

       <!-- Right: Actions -->
       <div class="flex items-center gap-2">
          <button class="btn btn-sm btn-success btn-outline gap-2 font-bold" on:click={() => runAllCells(true)}>
             <Play size={14} />
             {translate('frontend/src/lib/components/NotebookEditor.svelte::run_all')}
          </button>
          
          <button class="btn btn-sm btn-ghost gap-2 font-bold opacity-70 hover:opacity-100" on:click={exportNotebook} aria-label={translate('frontend/src/lib/components/NotebookEditor.svelte::download')}>
             <Download size={14} />
             <span class="hidden sm:inline">{translate('frontend/src/lib/components/NotebookEditor.svelte::download').replace(' (.ipynb)', '')}</span>
          </button>

          {#if $auth?.role === 'teacher' && fileId}
            <button class="btn btn-sm btn-primary gap-2 font-bold shadow-lg shadow-primary/20" on:click={saveNotebook} aria-label={translate('frontend/src/lib/components/NotebookEditor.svelte::save_notebook_aria_label')}>
               <Save size={14} />
               {translate('frontend/src/lib/components/NotebookEditor.svelte::save')}
            </button>
          {/if}
       </div>
    </div>

    <!-- Cells Container -->
    <div class="space-y-6">
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
      {/each}
    </div>

    <!-- Bottom Add Buttons -->
    <div class="flex items-center justify-center gap-4 py-12 border-t border-base-200 border-dashed opacity-60 hover:opacity-100 transition-opacity">
        <button class="btn btn-ghost gap-2" on:click={() => addCell("code")}>
           <Plus size={16} /> 
           {translate('frontend/src/lib/components/NotebookEditor.svelte::add_code_cell')}
        </button>
        <button class="btn btn-ghost gap-2" on:click={() => addCell("markdown")}>
           <Plus size={16} />
           {translate('frontend/src/lib/components/NotebookEditor.svelte::add_markdown_cell')}
        </button>
    </div>
  </div>
{:else}
  <div class="flex flex-col items-center justify-center py-20 text-center gap-4">
     <span class="loading loading-spinner loading-lg text-primary"></span>
     <p class="font-medium opacity-50 tracking-widest uppercase text-xs">
       {translate('frontend/src/lib/components/NotebookEditor.svelte::loading')}
     </p>
  </div>
{/if}

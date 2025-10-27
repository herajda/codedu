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

  export let fileId: string | number | undefined;
  $: nb = $notebookStore;

  let translate;
  $: translate = $translator;

  let cellRefs: any[] = [];
  const { saveAs } = pkg;
  const dispatch = createEventDispatcher();

  let container: HTMLDivElement | null = null;
  let showBottomButton = false;
  function checkHeight() {
    if (!container) return;
    showBottomButton = container.scrollHeight > window.innerHeight;
  }

  onMount(() => {
    // Preload the Pyodide runtime and common packages in the background.
    // Loading happens inside a Web Worker so it does not block the UI.
    preloadPyodide();
    checkHeight();
    window.addEventListener('resize', checkHeight);
    return () => window.removeEventListener('resize', checkHeight);
  });

  afterUpdate(() => {
    checkHeight();
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
  <div class="space-y-3" bind:this={container}>
    <div class="flex justify-end gap-2">
      <button
        on:click={runAllCells}
        class="px-3 py-1 rounded text-green-600 hover:text-white hover:bg-green-600 hover:scale-110 transition-transform"
      >
        {translate('frontend/src/lib/components/NotebookEditor.svelte::run_all')}
      </button>
      <button
        on:click={exportNotebook}
        aria-label={translate('frontend/src/lib/components/NotebookEditor.svelte::download')}
        class="px-3 py-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      >
        {translate('frontend/src/lib/components/NotebookEditor.svelte::download')}
      </button>
    </div>
    {#each nb.cells as cell, i (cell.id)}
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
    {/each}

    <div class="flex gap-2">
      <button
        on:click={() => addCell("markdown")}
        aria-label={translate('frontend/src/lib/components/NotebookEditor.svelte::add_markdown_cell')}
        title={translate('frontend/src/lib/components/NotebookEditor.svelte::add_markdown_cell')}
        class="p-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      >
        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M6 2a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6H6z" />
          <path d="M12 11v5M9 13h6" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" />
        </svg>
      </button>
      <button
        on:click={() => addCell("code")}
        aria-label={translate('frontend/src/lib/components/NotebookEditor.svelte::add_code_cell')}
        title={translate('frontend/src/lib/components/NotebookEditor.svelte::add_code_cell')}
        class="p-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      >
        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M16 18l6-6-6-6M8 6L2 12l6 6" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </button>
      <button
        on:click={exportNotebook}
        aria-label={translate('frontend/src/lib/components/NotebookEditor.svelte::download')}
        class="px-3 py-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      >
        {translate('frontend/src/lib/components/NotebookEditor.svelte::download')}
      </button>
      {#if $auth?.role === 'teacher' && fileId}
        <button
          on:click={saveNotebook}
          aria-label={translate('frontend/src/lib/components/NotebookEditor.svelte::save_notebook_aria_label')}
          class="px-3 py-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
        >
          {translate('frontend/src/lib/components/NotebookEditor.svelte::save')}
        </button>
      {/if}
    </div>
    {#if showBottomButton}
      <div class="flex justify-end gap-2">
        <button
          on:click={runAllCells}
          class="px-3 py-1 rounded text-green-600 hover:text-white hover:bg-green-600 hover:scale-110 transition-transform"
        >
          {translate('frontend/src/lib/components/NotebookEditor.svelte::run_all')}
        </button>
        <button
          on:click={exportNotebook}
          aria-label={translate('frontend/src/lib/components/NotebookEditor.svelte::download')}
          class="px-3 py-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
        >
          {translate('frontend/src/lib/components/NotebookEditor.svelte::download')}
        </button>
      </div>
    {/if}
  </div>
{:else}
  <p>{translate('frontend/src/lib/components/NotebookEditor.svelte::loading')}</p>
{/if}

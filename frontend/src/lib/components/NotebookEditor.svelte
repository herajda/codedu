
<script lang="ts">
  import { notebookStore } from "$lib/stores/notebookStore";
  import { saveAs } from "file-saver";
  import CodeCell from "./cells/CodeCell.svelte";
  import MarkdownCell from "./cells/MarkdownCell.svelte";
  import { v4 as uuid } from "uuid";
  import { serializeNotebook } from "$lib/notebook";
  $: nb = $notebookStore;

  let cellRefs: any[] = [];

  async function runAllCells() {
    for (const ref of cellRefs) {
      if (ref && typeof ref.runFromParent === 'function') {
        await ref.runFromParent();
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

  function saveNotebook() {
    const json = serializeNotebook(nb);
    const blob = new Blob([json], { type: "application/json" });
    saveAs(blob, "notebook.ipynb");
  }
</script>

{#if nb}
  <div class="space-y-4">
    <div class="flex justify-end">
      <button
        on:click={runAllCells}
        class="px-3 py-1 rounded text-green-600 hover:text-white hover:bg-green-600 hover:scale-110 transition-transform"
      >
        Run All
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
        aria-label="Add markdown cell"
        title="Add markdown cell"
        class="p-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      >
        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M6 2a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6H6z" />
          <path d="M12 11v5M9 13h6" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" />
        </svg>
      </button>
      <button
        on:click={() => addCell("code")}
        aria-label="Add code cell"
        title="Add code cell"
        class="p-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      >
        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M16 18l6-6-6-6M8 6L2 12l6 6" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </button>
      <button
        on:click={saveNotebook}
        aria-label="Save notebook"
        class="px-3 py-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      >
        Save
      </button>
    </div>
    <div class="flex justify-end">
      <button
        on:click={runAllCells}
        class="px-3 py-1 rounded text-green-600 hover:text-white hover:bg-green-600 hover:scale-110 transition-transform"
      >
        Run All
      </button>
    </div>
  </div>
{:else}
  <p>Loading…</p>
{/if}


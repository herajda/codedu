<script lang="ts">
  import { marked } from "marked";
  import { tick } from 'svelte';
  import { MarkdownEditor } from '$lib';
  import {
    notebookStore,
    moveCellUp,
    moveCellDown,
    insertCell,
    deleteCell
  } from "$lib/stores/notebookStore";
  export let cell: import("$lib/notebook").NotebookCell;
  export let index: number;
  let showInsert = false;
  let insertPos: 'above' | 'below' | null = null;

  let editing = !cell.source;

  let editorRef: any;

  // keep a local string bound to the editor
  let sourceStr = Array.isArray(cell.source) ? cell.source.join("") : (cell.source ?? "");

  $: {
    // when cell.source changes externally, keep local in sync
    const s = Array.isArray(cell.source) ? cell.source.join("") : (cell.source ?? "");
    if (s !== sourceStr) sourceStr = s;
  }

  async function toggle() {
    if (editing) {
      // clean up the EasyMDE instance before the component unmounts
      editorRef?.destroyEditor?.();
    }
    editing = !editing;
    if (editing) {
      sourceStr = Array.isArray(cell.source)
        ? cell.source.join("")
        : (cell.source ?? "");
      await tick();
      editorRef?.focus?.();
    }
  }

  function onInput() {
    cell.source = sourceStr;
    // trigger store update so parent re-renders
    notebookStore.update((n) => n ? ({ ...n }) : n);
  }
</script>

<div
  class="border rounded-lg p-3 bg-white shadow-inner group relative"
  on:dblclick={() => { if (!editing) toggle(); }}
>
  <div class="flex gap-2 justify-end mb-2 opacity-0 group-hover:opacity-100 transition-opacity">
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
    {#if !editing}
      <button
        aria-label="Edit cell"
        title="Edit cell"
        on:click={toggle}
        class="p-1 rounded text-gray-600 hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M4 17.25V21h3.75L17.81 10.94l-3.75-3.75L4 17.25zM20.71 7.04a1.003 1.003 0 0 0 0-1.42l-2.34-2.34a1.003 1.003 0 0 0-1.42 0l-1.83 1.83 3.75 3.75 1.84-1.82z" />
        </svg>
      </button>
    {/if}
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
  {#if editing}
    <MarkdownEditor
      bind:this={editorRef}
      bind:value={sourceStr}
      className="w-full bg-gray-100 p-2 rounded"
      on:input={onInput}
    />
    <button
      class="text-blue-600 mt-2 p-1 rounded hover:text-white hover:bg-gray-600 hover:scale-110 transition-transform"
      on:click={toggle}
    >Preview</button>
  {:else}
  <div class="markdown">
    {@html marked.parse(sourceStr)}
    </div>
  {/if}
</div>

<script lang="ts">
  import { renderMarkdown } from "$lib/markdown";
  import { tick } from 'svelte';
  import { MarkdownEditor } from '$lib';
  import {
    notebookStore,
    moveCellUp,
    moveCellDown,
    insertCell,
    deleteCell
  } from "$lib/stores/notebookStore";
  import { t } from '$lib/i18n';
  import { FileText, Edit, ArrowUp, ArrowDown, Trash2, Plus, Code as CodeIcon, Eye } from 'lucide-svelte';

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
  class="bg-base-100 rounded-2xl border-y border-r border-l-4 border-l-secondary/60 border-base-200 p-4 shadow-sm hover:shadow-lg transition-all group relative hover:border-l-secondary"
  on:dblclick={() => { if (!editing) toggle(); }}
>
  {#if !editing}
  <div class="absolute right-2 top-2 z-10 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity bg-base-100/80 backdrop-blur-sm p-1 rounded-full border border-base-200 shadow-sm">
    <button
      aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::move_cell_up')}
      title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::move_cell_up')}
      on:click={() => moveCellUp(index)}
      class="btn btn-xs btn-circle btn-ghost"
    >
      <ArrowUp size={14} />
    </button>
    <button
      aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::move_cell_down')}
      title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::move_cell_down')}
      on:click={() => moveCellDown(index)}
      class="btn btn-xs btn-circle btn-ghost"
    >
      <ArrowDown size={14} />
    </button>
    <button
      aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::delete_cell')}
      title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::delete_cell')}
      on:click={() => deleteCell(index)}
      class="btn btn-xs btn-circle btn-ghost text-error hover:bg-error/10"
    >
      <Trash2 size={14} />
    </button>
    {#if !editing}
      <button
        aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::edit_cell')}
        title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::edit_cell')}
        on:click={toggle}
        class="btn btn-xs btn-circle btn-ghost text-primary"
      >
        <Edit size={14} />
      </button>
    {/if}
    <div class="relative dropdown dropdown-end dropdown-bottom {showInsert ? 'dropdown-open' : ''}">
      <button
        aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_cell')}
        title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_cell')}
        on:click={() => { showInsert = !showInsert; if (!showInsert) insertPos = null; }}
        class="btn btn-xs btn-circle btn-ghost"
      >
        <Plus size={14} />
      </button>
      {#if showInsert}
        <ul class="dropdown-content z-[2] menu p-2 shadow-xl bg-base-100 rounded-box w-48 border border-base-200 mt-1">
          {#if !insertPos}
            <li>
                <button on:click={() => (insertPos = 'above')}>
                  <ArrowUp size={14} /> 
                  {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_above')}
                </button>
            </li>
            <li>
                <button on:click={() => (insertPos = 'below')}>
                  <ArrowDown size={14} /> 
                  {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_below')}
                </button>
            </li>
          {:else}
            <li>
                <button on:click={() => {insertCell(index, 'code', insertPos); showInsert = false; insertPos = null;}}>
                  <CodeIcon size={14} /> 
                  {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_code')}
                </button>
            </li>
            <li>
                <button on:click={() => {insertCell(index, 'markdown', insertPos); showInsert = false; insertPos = null;}}>
                  <FileText size={14} /> 
                  {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_markdown')}
                </button>
            </li>
          {/if}
        </ul>
      {/if}
    </div>
  </div>
  {/if}
  
  {#if editing}
    <div class="space-y-2">
      <div class="bg-base-200 rounded-lg p-1">
        <MarkdownEditor
          bind:this={editorRef}
          bind:value={sourceStr}
          className="w-full bg-base-100 p-2 rounded-md min-h-[100px] border border-base-200 focus:outline-none focus:ring-2 focus:ring-primary/20"
          on:input={onInput}
        />
      </div>
      <div class="flex items-center gap-2">
        <button
          class="btn btn-sm btn-primary"
          on:click={toggle}
        >
          <Eye size={16} />
          {t('frontend/src/lib/components/cells/MarkdownCell.svelte::preview')}
        </button>
        
        <div class="flex gap-1 items-center ml-auto">
          <button
            aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::move_cell_up')}
            title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::move_cell_up')}
            on:click={() => moveCellUp(index)}
            class="btn btn-sm btn-square btn-ghost"
          >
            <ArrowUp size={16} />
          </button>
          <button
            aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::move_cell_down')}
            title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::move_cell_down')}
            on:click={() => moveCellDown(index)}
            class="btn btn-sm btn-square btn-ghost"
          >
            <ArrowDown size={16} />
          </button>
          <button
            aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::delete_cell')}
            title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::delete_cell')}
            on:click={() => deleteCell(index)}
            class="btn btn-sm btn-square btn-ghost text-error"
          >
            <Trash2 size={16} />
          </button>
          
          <div class="relative dropdown dropdown-end dropdown-top {showInsert ? 'dropdown-open' : ''}">
            <button
              aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_cell')}
              title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_cell')}
              on:click={() => { showInsert = !showInsert; if (!showInsert) insertPos = null; }}
              class="btn btn-sm btn-square btn-ghost"
            >
              <Plus size={16} />
            </button>
            {#if showInsert}
              <ul class="dropdown-content z-[2] menu p-2 shadow-xl bg-base-100 rounded-box w-48 border border-base-200 mb-1">
                {#if !insertPos}
                  <li><button on:click={() => (insertPos = 'above')}><ArrowUp size={14} /> {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_above')}</button></li>
                  <li><button on:click={() => (insertPos = 'below')}><ArrowDown size={14} /> {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_below')}</button></li>
                {:else}
                  <li><button on:click={() => {insertCell(index, 'code', insertPos); showInsert = false; insertPos = null;}}><CodeIcon size={14} /> {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_code')}</button></li>
                  <li><button on:click={() => {insertCell(index, 'markdown', insertPos); showInsert = false; insertPos = null;}}><FileText size={14} /> {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_markdown')}</button></li>
                {/if}
              </ul>
            {/if}
          </div>
        </div>
      </div>
    </div>
  {:else}
    <div class="prose prose-sm max-w-none p-2">
      {@html renderMarkdown(sourceStr)}
    </div>
  {/if}
</div>

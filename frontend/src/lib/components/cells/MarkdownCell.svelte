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
  let containerRef: HTMLDivElement;


  // keep a local string bound to the editor
  let sourceStr = Array.isArray(cell.source) ? cell.source.join("") : (cell.source ?? "");

  $: {
    // when cell.source changes externally, keep local in sync
    const s = Array.isArray(cell.source) ? cell.source.join("") : (cell.source ?? "");
    if (s !== sourceStr) sourceStr = s;
  }

  async function toggle() {
    editing = !editing;
    if (editing) {
      sourceStr = Array.isArray(cell.source)
        ? cell.source.join("")
        : (cell.source ?? "");
      await tick();
      editorRef?.focus?.();
    }
  }

  function handleFocusOut() {
    // Small timeout to allow focus to settle (e.g. when clicking toolbar buttons)
    setTimeout(() => {
      if (editing && containerRef && !containerRef.contains(document.activeElement)) {
        toggle();
      }
    }, 150);
  }


  function onInput() {
    cell.source = sourceStr;
    // trigger store update so parent re-renders
    notebookStore.update((n) => n ? ({ ...n }) : n);
  }
</script>

<div
  bind:this={containerRef}
  class="bg-base-100/50 backdrop-blur-sm rounded-[2rem] border-2 border-secondary/10 p-5 shadow-xl shadow-secondary/5 hover:shadow-2xl hover:shadow-secondary/10 transition-all duration-300 group relative hover:border-secondary/30 mx-1"
  on:dblclick={() => { if (!editing) toggle(); }}
  on:focusout={handleFocusOut}
>
  <!-- Cell Type Indicator -->
  <div class="absolute -top-3 left-8 flex items-center gap-2">
    {#if editing}
      <div class="bg-primary text-primary-content px-3 py-1 rounded-full text-[10px] font-black tracking-widest flex items-center gap-1.5 shadow-lg shadow-primary/20">
        <Edit size={12} />
        EDITING
      </div>
    {/if}
  </div>

  {#if !editing}
  <div class="absolute right-4 top-4 z-10 flex gap-1 opacity-0 group-hover:opacity-100 transition-all duration-300 translate-x-2 group-hover:translate-x-0 bg-base-100/80 backdrop-blur-md p-1.5 rounded-2xl border border-base-200 shadow-xl">
    <div class="flex items-center gap-1 bg-base-200/50 p-1 rounded-xl border border-base-content/5 shadow-sm mr-1">
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
    </div>
    <button
      aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::delete_cell')}
      title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::delete_cell')}
      on:click={() => deleteCell(index)}
      class="btn btn-xs btn-circle btn-ghost text-error hover:bg-error/10 mx-1"
    >
      <Trash2 size={14} />
    </button>
    
    <button
      aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::edit_cell')}
      title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::edit_cell')}
      on:click={toggle}
      class="btn btn-xs btn-circle btn-ghost text-primary bg-primary/10 hover:bg-primary/20 mx-1"
    >
      <Edit size={14} />
    </button>
    
    <div class="relative dropdown dropdown-end dropdown-bottom {showInsert ? 'dropdown-open' : ''}">
      <button
        aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_cell')}
        title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_cell')}
        on:click={() => { showInsert = !showInsert; if (!showInsert) insertPos = null; }}
        class="btn btn-xs btn-circle btn-ghost bg-secondary/10 text-secondary hover:bg-secondary/20 ml-1"
      >
        <Plus size={18} />
      </button>
      {#if showInsert}
        <ul class="dropdown-content z-50 menu p-2 shadow-2xl bg-base-100 rounded-2xl w-52 border border-base-200 mt-2 ring-1 ring-black/5 animate-in fade-in zoom-in duration-200">
          {#if !insertPos}
            <li class="menu-title text-[10px] font-black tracking-widest uppercase opacity-50 px-4 py-2">Position</li>
            <li>
                <button class="rounded-xl py-3" on:click={() => (insertPos = 'above')}>
                  <ArrowUp size={14} class="text-primary" /> 
                  {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_above')}
                </button>
            </li>
            <li>
                <button class="rounded-xl py-3" on:click={() => (insertPos = 'below')}>
                  <ArrowDown size={14} class="text-primary" /> 
                  {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_below')}
                </button>
            </li>
          {:else}
            <li class="menu-title text-[10px] font-black tracking-widest uppercase opacity-50 px-4 py-2">Type ({insertPos})</li>
            <li>
                <button class="rounded-xl py-3" on:click={() => {insertCell(index, 'code', insertPos); showInsert = false; insertPos = null;}}>
                  <CodeIcon size={14} class="text-primary" /> 
                  {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_code')}
                </button>
            </li>
            <li>
                <button class="rounded-xl py-3" on:click={() => {insertCell(index, 'markdown', insertPos); showInsert = false; insertPos = null;}}>
                  <FileText size={14} class="text-secondary" /> 
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
    <div class="space-y-4 pt-2">
      <div class="bg-base-300/30 rounded-2xl p-2 border border-base-200 shadow-inner focus-within:border-secondary/40 focus-within:ring-4 focus-within:ring-secondary/5 focus-within:bg-base-300/50 transition-all duration-300 group/editor relative">
        <MarkdownEditor
          bind:this={editorRef}
          bind:value={sourceStr}
          className="w-full bg-base-100/50 p-4 rounded-xl min-h-[150px] border-none focus:outline-none text-base leading-relaxed"
          on:input={onInput}
        />
        <div class="absolute right-5 top-16 opacity-20 pointer-events-none group-focus-within/editor:opacity-100 transition-opacity">
          <FileText size={14} class="text-secondary" />
        </div>
      </div>
      <div class="flex items-center gap-3">
        <button
          class="btn btn-primary rounded-xl px-6 font-bold shadow-lg shadow-primary/20"
          on:click={toggle}
        >
          <Eye size={18} />
          {t('frontend/src/lib/components/cells/MarkdownCell.svelte::preview')}
        </button>
        
        <div class="flex gap-2 items-center ml-auto">
          <div class="flex items-center gap-1 bg-base-200/50 p-1 rounded-xl border border-base-content/5">
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
          </div>
          <button
            aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::delete_cell')}
            title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::delete_cell')}
            on:click={() => deleteCell(index)}
            class="btn btn-sm btn-square btn-ghost text-error"
          >
            <Trash2 size={18} />
          </button>
          
          <div class="relative dropdown dropdown-end dropdown-top {showInsert ? 'dropdown-open' : ''}">
            <button
              aria-label={t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_cell')}
              title={t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_cell')}
              on:click={() => { showInsert = !showInsert; if (!showInsert) insertPos = null; }}
              class="btn btn-sm btn-square btn-ghost bg-secondary/10 text-secondary"
            >
              <Plus size={20} />
            </button>
            {#if showInsert}
              <ul class="dropdown-content z-50 menu p-2 shadow-2xl bg-base-100 rounded-2xl w-52 border border-base-200 mb-2 ring-1 ring-black/5 animate-in fade-in zoom-in duration-200">
                {#if !insertPos}
                  <li class="menu-title text-[10px] font-black tracking-widest uppercase opacity-50 px-4 py-2">Position</li>
                  <li><button class="rounded-xl py-3" on:click={() => (insertPos = 'above')}><ArrowUp size={14} class="text-primary" /> {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_above')}</button></li>
                  <li><button class="rounded-xl py-3" on:click={() => (insertPos = 'below')}><ArrowDown size={14} class="text-primary" /> {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_below')}</button></li>
                {:else}
                  <li class="menu-title text-[10px] font-black tracking-widest uppercase opacity-50 px-4 py-2">Type ({insertPos})</li>
                  <li><button class="rounded-xl py-3" on:click={() => {insertCell(index, 'code', insertPos); showInsert = false; insertPos = null;}}><CodeIcon size={14} class="text-primary" /> {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_code')}</button></li>
                  <li><button class="rounded-xl py-3" on:click={() => {insertCell(index, 'markdown', insertPos); showInsert = false; insertPos = null;}}><FileText size={14} class="text-secondary" /> {t('frontend/src/lib/components/cells/MarkdownCell.svelte::insert_markdown')}</button></li>
                {/if}
              </ul>
            {/if}
          </div>
        </div>
      </div>
    </div>
  {:else}
    <div class="prose prose-md max-w-none px-4 py-2 prose-headings:font-black prose-headings:tracking-tight prose-headings:text-base-content prose-p:text-base-content/80 prose-li:marker:text-secondary group-hover:prose-headings:text-secondary transition-colors duration-300">
      {@html renderMarkdown(sourceStr)}
    </div>
  {/if}
</div>

<style>
  :global(.prose ul) {
    list-style-type: disc;
    padding-left: 1.5em;
  }

  :global(.prose ol) {
    list-style-type: decimal;
    padding-left: 1.5em;
  }

  :global(.prose h1) {
    font-size: 2.25rem;
    font-weight: 900;
    margin-top: 1.5rem;
    margin-bottom: 1.5rem;
    line-height: 1.2;
  }

  :global(.prose h2) {
    font-size: 1.75rem;
    font-weight: 800;
    margin-top: 1.25rem;
    margin-bottom: 1rem;
    line-height: 1.3;
  }

  :global(.prose p) {
    margin-top: 1rem;
    margin-bottom: 1rem;
    line-height: 1.7;
  }
</style>

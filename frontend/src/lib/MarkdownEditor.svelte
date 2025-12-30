
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import { 
    Bold, 
    Italic, 
    Strikethrough,
    Heading1,
    Heading2,
    List, 
    ListOrdered,
    Code,
    // Quote,
    Link as LinkIcon,
    Image as ImageIcon,
    Undo, 
    Redo,
    Maximize,
    Minimize,
    Terminal
  } from 'lucide-svelte';
  import { Editor } from '@tiptap/core';
  import StarterKit from '@tiptap/starter-kit';
  import Link from '@tiptap/extension-link';
  import Image from '@tiptap/extension-image';
  import Placeholder from '@tiptap/extension-placeholder';
  import { Markdown } from 'tiptap-markdown';
  import { compressImage } from '$lib/utils/compressImage';
  import { t } from '$lib/i18n';
  import { sidebarCollapsed, sidebarOpen } from '$lib/sidebar';

  export let value = '';
  export let placeholder = '';
  export let className: string = '';
  export let showExtraButtons: boolean = true;

  const dispatch = createEventDispatcher();

  let element: HTMLElement;
  let editor: Editor | null = null;
  let fileInput: HTMLInputElement;
  let isFullscreen = false;
  let showLinkModal = false;
  let linkUrl = '';

  onMount(() => {
    editor = new Editor({
      element,
      extensions: [
        StarterKit.configure({
          heading: { levels: [1, 2, 3] },
        }),
        Link.configure({
          openOnClick: false,
          HTMLAttributes: {
            class: 'text-primary underline decoration-primary/30 underline-offset-2 hover:decoration-primary transition-colors',
          },
        }),
        Image.configure({
          HTMLAttributes: {
            class: 'rounded-lg max-w-full shadow-lg my-4',
          },
        }),
        Placeholder.configure({
          placeholder: placeholder,
        }),
        Markdown.configure({
            html: true,
            transformPastedText: true,
            transformCopiedText: true,
        }),
      ],
      content: value,
      editorProps: {
        attributes: {
          class: `prose prose-sm max-w-none focus:outline-none min-h-[120px] p-4 ${className}`,
        },
      },
      onUpdate: ({ editor }) => {
        // Get markdown content
        const markdown = (editor.storage as any).markdown.getMarkdown();
        if (markdown !== value) {
            value = markdown;
            dispatch('input', value);
        }
      },
      onTransaction: () => {
        // Trigger update to refresh toolbar state
        editor = editor; 
      },
    });
  });

  onDestroy(() => {
    if (editor) {
      editor.destroy();
    }
  });

  // Update editor content if value changes externally
  $: if (editor && value !== (editor.storage as any).markdown.getMarkdown()) {
     // Check if the difference is significant to avoid cursor jumps or loops
     // For simple cases, we just set it. Tiptap handles diffing reasonably well?
     // Actually setContent from markdown might be heavy.
     // Let's rely on the assumption that usually value updates come from the editor itself.
     // But if we clear the input (e.g. after send), we need to update.
     if (value === '') {
         editor.commands.setContent('');
     } else {
        // Only update if standard content is different to avoid loops
        // This is tricky with markdown conversion. 
        // We'll leave it for now, primarily handling the reset case.
     }
  }

  function toggleFullscreen() {
      isFullscreen = !isFullscreen;
      const side = isFullscreen;
      sidebarCollapsed.set(side);
      if (side) {
        sidebarOpen.set(false);
      }
      document.body.classList.toggle('hide-navbar', isFullscreen);
  }

  // Toolbar helpers
  const toggleBold = () => editor?.chain().focus().toggleBold().run();
  const toggleItalic = () => editor?.chain().focus().toggleItalic().run();
  const toggleStrike = () => editor?.chain().focus().toggleStrike().run();
  const toggleHeading = (level: 1 | 2) => editor?.chain().focus().toggleHeading({ level }).run();
  const toggleBulletList = () => editor?.chain().focus().toggleBulletList().run();
  const toggleOrderedList = () => editor?.chain().focus().toggleOrderedList().run();
  const toggleCodeBlock = () => editor?.chain().focus().toggleCodeBlock().run();
  const toggleCode = () => editor?.chain().focus().toggleCode().run();
  // const toggleBlockquote = () => editor?.chain().focus().toggleBlockquote().run();
  
  const openLinkModal = () => {
    linkUrl = editor?.getAttributes('link').href || '';
    showLinkModal = true;
  }

  const applyLink = () => {
    if (linkUrl === '') {
      editor?.chain().focus().extendMarkRange('link').unsetLink().run();
    } else {
      let finalUrl = linkUrl;
      // If the URL doesn't start with a protocol (http:// or https://) or mailto:, assume https://
      if (!/^https?:\/\//i.test(finalUrl) && !/^mailto:/i.test(finalUrl)) {
        finalUrl = 'https://' + finalUrl;
      }
      editor?.chain().focus().extendMarkRange('link').setLink({ href: finalUrl }).run();
    }
    showLinkModal = false;
  }
  
  const cancelLink = () => {
    showLinkModal = false;
  }

  const addImage = () => {
      fileInput?.click();
  }

  async function onFileChange(e: Event) {
    const target = e.target as HTMLInputElement;
    const f = target.files && target.files[0];
    if (!f) return;
    
    try {
        const processed = await compressImage(f, 1600, 0.85);
        const reader = new FileReader();
        reader.onload = (e) => {
            const result = e.target?.result as string;
            if (result) {
                editor?.chain().focus().setImage({ src: result }).run();
            }
        };
        reader.readAsDataURL(processed);
    } catch (error) {
        console.error('Image processing failed', error);
        alert(t('frontend/src/lib/MarkdownEditor.svelte::could_not_read_image_file'));
    }
    
    // Reset input
    target.value = '';
  }

  export function focus() {
    editor?.chain().focus().run();
  }

</script>

<div class={`markdown-editor-container bg-base-100 border border-base-300 rounded-xl overflow-hidden shadow-sm transition-all duration-200 ${isFullscreen ? 'fixed inset-0 z-50 rounded-none' : ''}`}>
  <!-- Toolbar -->
  {#if editor}
    <div class="editor-toolbar flex items-center gap-1 p-2 border-b border-base-200 bg-base-100/50 backdrop-blur-sm overflow-x-auto">
        <button class="btn btn-square btn-sm btn-ghost {editor.isActive('bold') ? 'bg-base-200 text-primary' : ''}" on:click={toggleBold} title="Bold">
          <Bold class="w-4 h-4" />
        </button>
        <button class="btn btn-square btn-sm btn-ghost {editor.isActive('italic') ? 'bg-base-200 text-primary' : ''}" on:click={toggleItalic} title="Italic">
          <Italic class="w-4 h-4" />
        </button>
        <button class="btn btn-square btn-sm btn-ghost {editor.isActive('strike') ? 'bg-base-200 text-primary' : ''}" on:click={toggleStrike} title="Strikethrough">
          <Strikethrough class="w-4 h-4" />
        </button>
        
        <div class="w-px h-4 bg-base-300 mx-1"></div>

        <button class="btn btn-square btn-sm btn-ghost {editor.isActive('heading', { level: 1 }) ? 'bg-base-200 text-primary' : ''}" on:click={() => toggleHeading(1)} title="Heading 1">
          <Heading1 class="w-4 h-4" />
        </button>
        <button class="btn btn-square btn-sm btn-ghost {editor.isActive('heading', { level: 2 }) ? 'bg-base-200 text-primary' : ''}" on:click={() => toggleHeading(2)} title="Heading 2">
          <Heading2 class="w-4 h-4" />
        </button>

        <div class="w-px h-4 bg-base-300 mx-1"></div>

        <button class="btn btn-square btn-sm btn-ghost {editor.isActive('bulletList') ? 'bg-base-200 text-primary' : ''}" on:click={toggleBulletList} title="Bullet List">
          <List class="w-4 h-4" />
        </button>
        <button class="btn btn-square btn-sm btn-ghost {editor.isActive('orderedList') ? 'bg-base-200 text-primary' : ''}" on:click={toggleOrderedList} title="Ordered List">
          <ListOrdered class="w-4 h-4" />
        </button>

        <div class="w-px h-4 bg-base-300 mx-1"></div>

        <button class="btn btn-square btn-sm btn-ghost {editor.isActive('code') ? 'bg-base-200 text-primary' : ''}" on:click={toggleCode} title="Inline Code">
          <Code class="w-4 h-4" />
        </button>
        <button class="btn btn-square btn-sm btn-ghost {editor.isActive('codeBlock') ? 'bg-base-200 text-primary' : ''}" on:click={toggleCodeBlock} title="Code Block">
          <Terminal class="w-4 h-4" />
        </button>
        <!-- <button class="btn btn-square btn-sm btn-ghost {editor.isActive('blockquote') ? 'bg-base-200 text-primary' : ''}" on:click={toggleBlockquote} title="Quote">
          <Quote class="w-4 h-4" />
        </button> -->
        
        <div class="w-px h-4 bg-base-300 mx-1"></div>

        <button class="btn btn-square btn-sm btn-ghost {editor.isActive('link') ? 'bg-base-200 text-primary' : ''}" on:click={openLinkModal} title="Link">
          <LinkIcon class="w-4 h-4" />
        </button>
        <button class="btn btn-square btn-sm btn-ghost" on:click={addImage} title="Image">
          <ImageIcon class="w-4 h-4" />
        </button>

        <div class="flex-1"></div>

        <button class="btn btn-square btn-sm btn-ghost" on:click={() => editor?.chain().focus().undo().run()} disabled={!editor.can().chain().focus().undo().run()} title="Undo">
          <Undo class="w-4 h-4" />
        </button>
        <button class="btn btn-square btn-sm btn-ghost" on:click={() => editor?.chain().focus().redo().run()} disabled={!editor.can().chain().focus().redo().run()} title="Redo">
          <Redo class="w-4 h-4" />
        </button>
        
        {#if showExtraButtons}
            <div class="w-px h-4 bg-base-300 mx-1"></div>
            <button class="btn btn-square btn-sm btn-ghost" on:click={toggleFullscreen} title={isFullscreen ? "Minimize" : "Maximize"}>
                {#if isFullscreen}
                    <Minimize class="w-4 h-4" />
                {:else}
                    <Maximize class="w-4 h-4" />
                {/if}
            </button>
        {/if}
    </div>
  {/if}

  <!-- Editor Content -->
  <div bind:this={element} class="h-full bg-base-100/30"></div>
  
  <input type="file" bind:this={fileInput} on:change={onFileChange} accept="image/*" class="hidden" />

  {#if showLinkModal}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
    <div class="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/50" 
         role="dialog"
         aria-modal="true"
         tabindex="0"
         on:click={cancelLink}>
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div class="bg-base-100 rounded-xl shadow-2xl p-4 w-full max-w-sm" on:click|stopPropagation>
        <h3 class="font-bold text-lg mb-2">{t('frontend/src/lib/MarkdownEditor.svelte::insert_link_title')}</h3>
        <input 
          type="text" 
          placeholder="https://example.com" 
          class="input input-bordered w-full mb-4" 
          bind:value={linkUrl}
          on:keydown={(e) => e.key === 'Enter' && applyLink()}
          autofocus
        />
        <div class="flex justify-end gap-2">
          <button class="btn btn-sm btn-ghost" on:click={cancelLink}>{t('frontend/src/lib/MarkdownEditor.svelte::cancel_link')}</button>
          <button class="btn btn-sm btn-primary" on:click={applyLink}>{t('frontend/src/lib/MarkdownEditor.svelte::save_link')}</button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  :global(.ProseMirror) {
    min-height: 120px;
    height: 100%;
    outline: none !important;
  }
  
  :global(.ProseMirror p.is-editor-empty:first-child::before) {
    content: attr(data-placeholder);
    float: left;
    color: hsl(var(--bc) / 0.4);
    pointer-events: none;
    height: 0;
  }

  :global(.ProseMirror ul) {
    list-style-type: disc;
    padding-left: 1.5em;
  }

  :global(.ProseMirror ol) {
    list-style-type: decimal;
    padding-left: 1.5em;
  }

  :global(.ProseMirror h1) {
    font-size: 1.5em;
    font-weight: bold;
    margin-top: 0.5em;
    margin-bottom: 0.5em;
  }

  :global(.ProseMirror h2) {
    font-size: 1.25em;
    font-weight: bold;
    margin-top: 0.5em;
    margin-bottom: 0.5em;
  }

  :global(.ProseMirror h3) {
    font-size: 1.1em;
    font-weight: bold;
    margin-top: 0.5em;
    margin-bottom: 0.5em;
  }

  /* Scrollbar styling for the toolbar */
  .editor-toolbar::-webkit-scrollbar {
    height: 0px;
  }

  :global(.ProseMirror a) {
    color: hsl(var(--p));
    text-decoration: underline;
    text-decoration-color: hsl(var(--p) / 0.3);
    text-underline-offset: 2px;
    cursor: pointer;
  }
  :global(.ProseMirror a:hover) {
    text-decoration-color: hsl(var(--p));
  }

  :global(.ProseMirror code) {
    background-color: var(--assignment-inline-code-bg);
    padding: 0.15rem 0.4rem;
    border-radius: 0.5rem;
    font-family: "JetBrains Mono", "Fira Code", "Source Code Pro", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
    font-size: 0.9em;
  }

  :global(.ProseMirror pre) {
    background-color: var(--assignment-code-bg, oklch(0.95 0 0)); /* oklch fallback or #f2f2f2 */
    /* Fallback to legacy hex if oklch fails or variable missing */
    background-color: var(--assignment-code-bg, #f2f2f2);
    padding: 0.85rem 1rem;
    border-radius: 0.75rem;
    border: 1px solid var(--assignment-code-border, transparent);
    margin: 1rem 0;
  }

  :global(.ProseMirror pre code) {
    background-color: transparent;
    padding: 0;
    font-size: 0.9em;
    border: none;
    color: inherit;
  }
</style>

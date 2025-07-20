<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import '@fortawesome/fontawesome-free/css/all.min.css';

  let EasyMDE: any;

  export let value = '';
  export let placeholder = '';
  export let className: string = '';

  let textarea: HTMLTextAreaElement;
  let editor: any = null;
  const dispatch = createEventDispatcher();

  onMount(async () => {
    const mod = await import('easymde');
    await import('easymde/dist/easymde.min.css');
    EasyMDE = mod.default;
    editor = new EasyMDE({
      element: textarea,
      initialValue: value,
      placeholder,
      autoDownloadFontAwesome: false,
      spellChecker: false,
      toolbar: [
        'bold',
        'italic',
        'heading',
        '|',
        'code',
        'unordered-list',
        'ordered-list',
        '|',
        'link',
        'image',
        '|',
        'preview',
        'side-by-side',
        'fullscreen',
        '|',
        'guide'
      ]
    });
    editor.codemirror.on('change', () => {
      value = editor!.value();
      dispatch('input', value);
    });
  });

  export function destroyEditor() {
    if (editor) {
      // EasyMDE leaves additional DOM when destroyed from preview mode
      if (typeof editor.isPreviewActive === 'function' && editor.isPreviewActive()) {
        editor.togglePreview();
      }
      editor.toTextArea();
      editor = null;
    }
  }

  onDestroy(() => {
    destroyEditor();
  });

  $: if (editor && value !== editor.value()) {
    editor.value(value);
  }

  export function focus() {
    textarea?.focus();
  }
</script>

<textarea bind:this={textarea} class={className}></textarea>

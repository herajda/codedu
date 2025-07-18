<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';

  export let value = '';
  export let placeholder = '';
  export let options: any = {};

  let textarea: HTMLTextAreaElement;
  let editor: any;
  const dispatch = createEventDispatcher();

  onMount(async () => {
    const CodeMirror = (await import('codemirror')).default;
    // expose global for addons expecting window.CodeMirror
    (window as any).CodeMirror = CodeMirror;

    await Promise.all([
      import('codemirror/lib/codemirror.css'),
      import('codemirror/mode/python/python'),
      import('codemirror/addon/edit/closebrackets'),
      import('codemirror/theme/base16-dark.css')
    ]);

    editor = CodeMirror.fromTextArea(textarea, {
      value,
      lineNumbers: true,
      mode: 'python',
      theme: 'base16-dark',
      autoCloseBrackets: true,
      ...options
    });

    editor.on('change', () => {
      value = editor.getValue();
      dispatch('input', value);
    });
  });

  onDestroy(() => {
    editor?.toTextArea();
    editor = null;
  });

  $: if (editor && editor.getValue() !== value) {
    editor.setValue(value);
  }
</script>

<textarea bind:this={textarea} {placeholder}></textarea>

<style>
  textarea {
    visibility: hidden;
  }
  .CodeMirror {
    height: auto;
  }
</style>

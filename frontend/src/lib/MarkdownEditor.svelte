<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import EasyMDE from 'easymde';
  import 'easymde/dist/easymde.min.css';
  import '@fortawesome/fontawesome-free/css/all.min.css';

  export let value = '';
  export let placeholder = '';

  let textarea: HTMLTextAreaElement;
  let editor: EasyMDE;
  const dispatch = createEventDispatcher();

  onMount(() => {
    editor = new EasyMDE({
      element: textarea,
      initialValue: value,
      placeholder,
      autoDownloadFontAwesome: false,
      spellChecker: false
    });
    editor.codemirror.on('change', () => {
      value = editor.value();
      dispatch('input', value);
    });
  });

  onDestroy(() => {
    editor?.toTextArea();
  });

  $: if (editor && value !== editor.value()) {
    editor.value(value);
  }
</script>

<textarea bind:this={textarea}></textarea>

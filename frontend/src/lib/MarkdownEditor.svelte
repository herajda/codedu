<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import '@fortawesome/fontawesome-free/css/all.min.css';

  let EasyMDE: typeof import('easymde');

  export let value = '';
  export let placeholder = '';

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
      spellChecker: false
    });
    editor.codemirror.on('change', () => {
      value = editor!.value();
      dispatch('input', value);
    });
  });

  onDestroy(() => {
    editor?.toTextArea();
    editor = null;
  });

  $: if (editor && value !== editor.value()) {
    editor.value(value);
  }
</script>

<textarea bind:this={textarea}></textarea>

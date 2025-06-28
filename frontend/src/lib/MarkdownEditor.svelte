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

  function insertImageWithSize(editor: any) {
    const url = prompt('Enter image URL');
    if (!url) return;
    const width = prompt('Width (optional)');
    const height = prompt('Height (optional)');
    let attrs = '';
    if (width) attrs += ` width="${width}"`;
    if (height) attrs += ` height="${height}"`;
    const html = `<img src="${url}"${attrs}>`;
    editor.codemirror.replaceSelection(html);
  }

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
        {
          name: 'image',
          action: insertImageWithSize,
          className: 'fa fa-image',
          title: 'Insert image'
        },
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

  onDestroy(() => {
    editor?.toTextArea();
    editor = null;
  });

  $: if (editor && value !== editor.value()) {
    editor.value(value);
  }
</script>

<textarea bind:this={textarea} class={className}></textarea>

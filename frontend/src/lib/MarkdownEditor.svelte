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
  let observer: ResizeObserver;
  const dispatch = createEventDispatcher();

  function insertResizableImage(editor: any) {
    const url = prompt('Enter image URL');
    if (!url) return;
    const id = crypto.randomUUID();
    const html = `<div class="resizable-wrapper" data-img-id="${id}" style="width:200px;height:auto;"><img src="${url}"></div>`;
    editor.codemirror.replaceSelection(html);
    attachObservers();
  }

  function attachObservers() {
    const previews = document.querySelectorAll('.editor-preview, .editor-preview-side');
    previews.forEach((preview) => {
      preview.querySelectorAll('.resizable-wrapper').forEach((el) => {
        observer.observe(el as Element);
      });
    });
  }

  function updateMarkdown(id: string, width: number, height: number) {
    if (!editor) return;
    const doc = editor.codemirror.getValue();
    const regex = new RegExp(`<div class=\"resizable-wrapper\" data-img-id=\"${id}\"[^>]*>(<img[^>]*>)<\\/div>`);
    const newDiv = `<div class="resizable-wrapper" data-img-id="${id}" style="width:${width}px;height:${height}px;">$1</div>`;
    const newDoc = doc.replace(regex, newDiv);
    if (newDoc !== doc) {
      editor.codemirror.setValue(newDoc);
      value = newDoc;
      dispatch('input', value);
    }
  }

  onMount(async () => {
    const mod = await import('easymde');
    await import('easymde/dist/easymde.min.css');
    EasyMDE = mod.default;
    observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const el = entry.target as HTMLElement;
        const id = el.dataset.imgId;
        if (!id) continue;
        const w = el.offsetWidth;
        const h = el.offsetHeight;
        el.style.width = w + 'px';
        el.style.height = h + 'px';
        updateMarkdown(id, w, h);
      }
    });
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
          action: insertResizableImage,
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
      attachObservers();
    });
    attachObservers();
  });

  onDestroy(() => {
    editor?.toTextArea();
    observer?.disconnect();
    editor = null;
  });

  $: if (editor && value !== editor.value()) {
    editor.value(value);
  }
</script>

<textarea bind:this={textarea} class={className}></textarea>

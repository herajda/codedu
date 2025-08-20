<script lang="ts">
  // @ts-nocheck
  import { onMount, onDestroy } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import { sidebarCollapsed, sidebarOpen } from '$lib/sidebar';
  import '@fortawesome/fontawesome-free/css/all.min.css';
  import { compressImage } from '$lib/utils/compressImage';

  let EasyMDE: any;

  export let value = '';
  export let placeholder = '';
  export let className: string = '';

  let textarea: HTMLTextAreaElement;
  let editor: any = null;
  const dispatch = createEventDispatcher();

  let fileInput: HTMLInputElement;
  let showImageDialog = false;
  let pendingImageDataUrl: string | null = null;
  let imageNaturalWidth = 800;
  let imageWidthPx = 600;
  let imageMarkers: any[] = [];

  function openFilePicker() {
    fileInput?.click();
  }

  async function onFileChange(e: Event) {
    const target = e.target as HTMLInputElement;
    const f = target.files && target.files[0];
    if (!f) return;
    try {
      const processed = await compressImage(f, 1600, 0.85);
      const dataUrl: string = await new Promise((resolve, reject) => {
        const fr = new FileReader();
        fr.onload = () => resolve(fr.result as string);
        fr.onerror = () => reject(fr.error);
        fr.readAsDataURL(processed);
      });
      pendingImageDataUrl = dataUrl;
      const img = new Image();
      await new Promise<void>((resolve, reject) => {
        img.onload = () => resolve();
        img.onerror = () => reject(new Error('Failed to load image'));
        img.src = dataUrl;
      });
      imageNaturalWidth = Math.max(50, img.naturalWidth || 800);
      imageWidthPx = Math.min(800, imageNaturalWidth);
      showImageDialog = true;
    } catch (err) {
      console.error(err);
      alert('Could not read image file.');
    } finally {
      // reset input so picking the same file again will retrigger change
      if (fileInput) fileInput.value = '';
    }
  }

  function insertImage() {
    if (!editor || !pendingImageDataUrl) return;
    const html = `<img src="${pendingImageDataUrl}" alt="" width="${Math.round(imageWidthPx)}">`;
    // insert at cursor
    if (editor.codemirror) {
      editor.codemirror.replaceSelection(html);
      editor.codemirror.focus();
    } else {
      // fallback
      const cur = editor.value();
      editor.value(cur + "\n" + html + "\n");
    }
    // cleanup
    pendingImageDataUrl = null;
    showImageDialog = false;
    // refresh markers so the inserted image shows as a widget in edit mode
    queueMicrotask(refreshImageMarkers);
  }

  function clearImageMarkers() {
    if (!editor?.codemirror) return;
    for (const m of imageMarkers) try { m.clear(); } catch {}
    imageMarkers = [];
  }

  function createImageWidget(src: string, widthFromTag?: number | null) {
    const container = document.createElement('span');
    container.className = 'md-img-widget';
    container.style.position = 'relative';
    container.style.display = 'inline-block';
    const img = document.createElement('img');
    img.src = src;
    img.alt = '';
    img.style.maxWidth = '100%';
    img.style.height = 'auto';
    if (typeof widthFromTag === 'number' && widthFromTag > 0) {
      img.style.width = widthFromTag + 'px';
    }
    container.appendChild(img);
    return container;
  }

  function attachDeleteButton(container: HTMLElement, marker: any, cm: any) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.title = 'Delete image';
    btn.textContent = '×';
    btn.setAttribute('aria-label', 'Delete image');
    btn.style.position = 'absolute';
    btn.style.top = '4px';
    btn.style.right = '4px';
    btn.style.padding = '0 6px';
    btn.style.lineHeight = '18px';
    btn.style.height = '18px';
    btn.style.fontSize = '14px';
    btn.style.border = '1px solid rgba(0,0,0,0.2)';
    btn.style.borderRadius = '9999px';
    btn.style.background = 'rgba(255,255,255,0.9)';
    btn.style.cursor = 'pointer';
    btn.style.boxShadow = '0 1px 2px rgba(0,0,0,0.1)';
    btn.addEventListener('click', (ev) => {
      ev.preventDefault();
      ev.stopPropagation();
      try {
        const range = marker.find?.();
        if (range && range.from && range.to) {
          cm.replaceRange('', range.from, range.to);
        }
      } finally {
        try { marker.clear?.(); } catch {}
      }
    });
    container.appendChild(btn);
  }

  function parseWidthFromImgTag(tag: string): number | null {
    // width="123"
    const m = tag.match(/\bwidth\s*=\s*(["']?)(\d{2,5})\1/i);
    if (m) return parseInt(m[2], 10);
    // style="... width: 123px ..."
    const s = tag.match(/style=["'][^"']*width\s*:\s*(\d{2,5})\s*px/i);
    if (s) return parseInt(s[1], 10);
    return null;
  }

  function refreshImageMarkers() {
    if (!editor?.codemirror) return;
    const cm = editor.codemirror;
    clearImageMarkers();
    const doc = cm.getValue();

    // 1) Replace full <img ... src="data:..."> tag with widget
    const reImgDq = /<img\b[^>]*?src\s*=\s*"(data:image\/[^"]+)"[^>]*?>/g;
    const reImgSq = /<img\b[^>]*?src\s*=\s*'(data:image\/'[^']+)'[^>]*?>/g; // keep simple; rarely used
    for (const re of [reImgDq, reImgSq]) {
      for (const m of doc.matchAll(re)) {
        const full = m[0];
        const src = m[1];
        const start = m.index ?? 0;
        const end = start + full.length;
        const from = cm.posFromIndex(start);
        const to = cm.posFromIndex(end);
        const width = parseWidthFromImgTag(full);
        const node = createImageWidget(src, width);
        const mark = cm.markText(from, to, { replacedWith: node, atomic: true, clearOnEnter: true, inclusiveLeft: false, inclusiveRight: false });
        attachDeleteButton(node, mark, cm);
        imageMarkers.push(mark);
      }
    }

    // 2) Replace markdown image ![alt](data:...)
    const reMd = /!\[[^\]]*\]\((data:image\/[^)]+)\)/g;
    for (const m of doc.matchAll(reMd)) {
      const full = m[0];
      const src = m[1];
      const start = m.index ?? 0;
      const end = start + full.length;
      const from = cm.posFromIndex(start);
      const to = cm.posFromIndex(end);
      const node = createImageWidget(src, null);
      const mark = cm.markText(from, to, { replacedWith: node, atomic: true, clearOnEnter: true, inclusiveLeft: false, inclusiveRight: false });
      attachDeleteButton(node, mark, cm);
      imageMarkers.push(mark);
    }
  }

  onMount(async () => {
    const mod = await import('easymde');
    // @ts-ignore - CSS has no types, runtime-only
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
          name: 'upload-image',
          action: () => openFilePicker(),
          className: 'fa-solid fa-image',
          title: 'Insert image from computer'
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
      // delay to let CM apply the change, then refresh markers
      queueMicrotask(refreshImageMarkers);
    });

    const updateSidebar = () => {
      const side = editor?.isSideBySideActive && editor.isSideBySideActive();
      const fs = editor?.isFullscreenActive && editor.isFullscreenActive();
      sidebarCollapsed.set(side || fs);
      if (side || fs) {
        sidebarOpen.set(false);
      }
      document.body.classList.toggle('hide-navbar', fs);
    };

    const handleSideBySide = () => {
      if (!editor) return;
      const active = editor.isSideBySideActive();
      if (!active && editor.isFullscreenActive()) {
        editor.toggleFullScreen();
      }
      updateSidebar();
    };

    const btnSide = editor.toolbarElements?.['side-by-side'];
    btnSide?.addEventListener('click', handleSideBySide);
    const btnFull = editor.toolbarElements?.fullscreen;
    btnFull?.addEventListener('click', updateSidebar);

    const preview = editor.codemirror.getWrapperElement().nextSibling;
    const sideObserver = new MutationObserver(handleSideBySide);
    sideObserver.observe(preview, { attributes: true, attributeFilter: ['class'] });

    const wrapper = editor.codemirror.getWrapperElement();
    const fsObserver = new MutationObserver(updateSidebar);
    fsObserver.observe(wrapper, { attributes: true, attributeFilter: ['class'] });

    updateSidebar();
    // initial marker pass for any preexisting content
    refreshImageMarkers();
    onDestroy(() => {
      clearImageMarkers();
      btnSide?.removeEventListener('click', handleSideBySide);
      btnFull?.removeEventListener('click', updateSidebar);
      sideObserver.disconnect();
      fsObserver.disconnect();
    });
  });

  export function destroyEditor() {
    if (!editor) return;
    try {
      // EasyMDE leaves additional DOM when destroyed from preview mode
      if (typeof editor.isPreviewActive === 'function' && editor.isPreviewActive()) {
        editor.togglePreview();
      }
      editor.toTextArea();
    } catch (err) {
      console.warn('Failed to destroy editor', err);
    } finally {
      editor = null;
    }
    document.body.classList.remove('hide-navbar');
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
<input type="file" accept="image/*" bind:this={fileInput} on:change={onFileChange} style="display:none" />

{#if showImageDialog}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
    <div class="bg-white rounded-lg shadow-lg max-w-[90vw] w-[520px] p-4">
      <div class="font-medium mb-2">Insert image</div>
      {#if pendingImageDataUrl}
        <div class="mb-3 flex items-start gap-3">
          <div class="flex-1">
            <img src={pendingImageDataUrl} alt="preview" style={`max-width:100%; height:auto; width:${imageWidthPx}px`} />
          </div>
        </div>
      {/if}
      <div class="mb-3">
        <label class="block text-sm mb-1" for="md-img-width">Width: {Math.round(imageWidthPx)} px</label>
        <input id="md-img-width" type="range" min="50" max={Math.max(200, imageNaturalWidth)} step="10" bind:value={imageWidthPx} class="range range-xs w-full" />
      </div>
      <div class="flex justify-end gap-2 mt-2">
        <button class="btn btn-sm" on:click={() => { showImageDialog = false; pendingImageDataUrl = null; }}>Cancel</button>
        <button class="btn btn-sm btn-primary" on:click={insertImage}>Insert</button>
      </div>
    </div>
  </div>
{/if}

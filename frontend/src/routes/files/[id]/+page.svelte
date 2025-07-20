<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { notebookStore } from '$lib/stores/notebookStore';
  import { loadNotebook } from '$lib/notebook';
  import NotebookEditor from '$lib/components/NotebookEditor.svelte';
  import { apiFetch } from '$lib/api';
  import { onDestroy } from 'svelte';
  import '@fortawesome/fontawesome-free/css/all.min.css';

  function goBack() {
    if (history.length > 1) history.back();
    else window.close();
  }

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    load();
  }

  let isImage = false;
  let imgUrl: string | null = null;

  async function load() {
    if (imgUrl) {
      URL.revokeObjectURL(imgUrl);
      imgUrl = null;
    }
    const res = await apiFetch(`/api/files/${id}`);
    const ct = res.headers.get('Content-Type') || '';
    if (ct.startsWith('image/')) {
      const blob = await res.blob();
      imgUrl = URL.createObjectURL(blob);
      isImage = true;
    } else {
      const text = await res.text();
      const nb = loadNotebook(text);
      notebookStore.set(nb);
      isImage = false;
    }
  }

  onDestroy(() => {
    if (imgUrl) URL.revokeObjectURL(imgUrl);
  });

  onMount(load);
</script>

<button class="btn btn-sm btn-circle mb-4" on:click={goBack} aria-label="Back to files">
  <i class="fa-solid fa-arrow-left"></i>
</button>
{#if isImage}
  {#if imgUrl}
    <img src={imgUrl} alt="image" class="max-w-full" />
  {/if}
{:else}
  <NotebookEditor fileId={id} />
{/if}


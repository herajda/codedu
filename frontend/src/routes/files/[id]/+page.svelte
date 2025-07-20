<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { notebookStore } from '$lib/stores/notebookStore';
  import { loadNotebook } from '$lib/notebook';
  import NotebookEditor from '$lib/components/NotebookEditor.svelte';
  import { apiFetch } from '$lib/api';

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    load();
  }

  async function load() {
    const res = await apiFetch(`/api/files/${id}`);
    const text = await res.text();
    const nb = loadNotebook(text);
    notebookStore.set(nb);
  }

  onMount(load);
</script>

<h1 class="text-2xl font-bold mb-4">Notebook</h1>
<NotebookEditor />


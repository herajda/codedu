<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { notebookStore } from '$lib/stores/notebookStore';
  import { loadNotebook, serializeNotebook } from '$lib/notebook';
  import NotebookEditor from '$lib/components/NotebookEditor.svelte';
  import { apiFetch } from '$lib/api';
  import UnsavedChangesModal from '$lib/components/UnsavedChangesModal.svelte';
  import { auth } from '$lib/auth';
  import '@fortawesome/fontawesome-free/css/all.min.css';
  import { beforeNavigate, goto } from '$app/navigation';

  let notebookEditor: InstanceType<typeof NotebookEditor> | undefined;
  let unsavedModal: InstanceType<typeof UnsavedChangesModal> | undefined;

  let originalNotebookSerialized: string | null = null;
  let currentNotebookSerialized: string | null = null;
  let unsubscribe: (() => void) | null = null;
  let isDirty = false;
  let isTeacher = false;
  $: isTeacher = ['teacher', 'admin'].includes($auth?.role ?? '');
  const unsavedModalOptions = {
    title: 'Unsaved notebook changes',
    body: 'You have unsaved changes in this notebook. What would you like to do before leaving?',
    icon: 'fa-solid fa-triangle-exclamation text-warning'
  } as const;
  let ignoreNavigationGuard = false;

  function navigateBack(skipGuard = false) {
    if (history.length > 1) {
      if (skipGuard) ignoreNavigationGuard = true;
      history.back();
    } else {
      window.close();
    }
  }

  function handleBeforeUnload(event: BeforeUnloadEvent) {
    if (!isTeacher || !isDirty) return;
    event.preventDefault();
    event.returnValue = '';
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
      notebookStore.set(null);
      originalNotebookSerialized = null;
      currentNotebookSerialized = null;
      isDirty = false;
    } else {
      const text = await res.text();
      const nb = loadNotebook(text);
      originalNotebookSerialized = serializeNotebook(nb);
      notebookStore.set(nb);
      isImage = false;
      isDirty = false;
    }
  }

  function handleSaved() {
    if (currentNotebookSerialized) {
      originalNotebookSerialized = currentNotebookSerialized;
      isDirty = false;
    }
  }

  async function confirmUnsavedNavigation() {
    if (!isTeacher || !isDirty) return true;
    const modal = unsavedModal;
    if (!modal) return true;
    const choice = await modal.open(unsavedModalOptions);
    if (choice === 'save') {
      try {
        await notebookEditor?.save();
        if (currentNotebookSerialized) {
          originalNotebookSerialized = currentNotebookSerialized;
          isDirty = false;
        }
        return true;
      } catch (error) {
        console.error(error);
        alert('Failed to save notebook.');
        return false;
      }
    }
    if (choice === 'discard') return true;
    return false;
  }

  async function goBack() {
    const shouldLeave = await confirmUnsavedNavigation();
    if (!shouldLeave) return;
    navigateBack(true);
  }

  beforeNavigate((nav) => {
    if (!nav.to || nav.willUnload) return;
    if (ignoreNavigationGuard) {
      ignoreNavigationGuard = false;
      return;
    }
    if (!isTeacher || !isDirty) return;
    nav.cancel();
    const destination = nav.to;
    const navType = nav.type;
    const delta = nav.delta ?? 0;
    void (async () => {
      const shouldLeave = await confirmUnsavedNavigation();
      if (!shouldLeave) return;
      if (navType === 'popstate') {
        if (delta !== 0) {
          ignoreNavigationGuard = true;
          history.go(delta);
        } else {
          ignoreNavigationGuard = true;
          goto(destination.url);
        }
      } else {
        ignoreNavigationGuard = true;
        goto(destination.url);
      }
    })();
  });

  onMount(() => {
    unsubscribe = notebookStore.subscribe((nb) => {
      if (!nb) {
        currentNotebookSerialized = null;
        isDirty = false;
        return;
      }
      currentNotebookSerialized = serializeNotebook(nb);
      if (originalNotebookSerialized) {
        isDirty = currentNotebookSerialized !== originalNotebookSerialized;
      }
    });
    window.addEventListener('beforeunload', handleBeforeUnload);
    load();
  });

  onDestroy(() => {
    if (imgUrl) URL.revokeObjectURL(imgUrl);
    unsubscribe?.();
    window.removeEventListener('beforeunload', handleBeforeUnload);
  });
</script>

<button class="btn btn-sm btn-circle mb-4" on:click={goBack} aria-label="Back to files">
  <i class="fa-solid fa-arrow-left"></i>
</button>
{#if isImage}
  {#if imgUrl}
    <img src={imgUrl} alt="image" class="max-w-full" />
  {/if}
{:else}
  <NotebookEditor bind:this={notebookEditor} fileId={id} on:saved={handleSaved} />
{/if}

<UnsavedChangesModal bind:this={unsavedModal} />

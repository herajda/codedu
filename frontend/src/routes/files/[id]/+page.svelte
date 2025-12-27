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
  import { t } from '$lib/i18n';
  import { ArrowLeft, Image as ImageIcon, FileCode } from 'lucide-svelte';

  let notebookEditor: InstanceType<typeof NotebookEditor> | undefined;
  let unsavedModal: InstanceType<typeof UnsavedChangesModal> | undefined;

  let originalNotebookSerialized: string | null = null;
  let currentNotebookSerialized: string | null = null;
  let unsubscribe: (() => void) | null = null;
  let isDirty = false;
  let isTeacher = false;
  $: isTeacher = ['teacher', 'admin'].includes($auth?.role ?? '');
  const unsavedModalOptions = {
    title: t('frontend/src/routes/files/[id]/+page.svelte::Unsaved notebook changes'),
    body: t('frontend/src/routes/files/[id]/+page.svelte::You have unsaved changes in this notebook. What would you like to do before leaving?'),
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
        alert(t('frontend/src/routes/files/[id]/+page.svelte::Failed to save notebook.'));
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

<div class="min-h-screen bg-base-100 p-2 sm:p-4 lg:p-6">
  <div class="w-full max-w-[1920px] mx-auto space-y-6">
    <!-- Header -->
    <header class="relative bg-base-100 rounded-3xl border border-base-200 shadow-sm p-4 sm:p-6 flex items-center justify-between gap-4 overflow-hidden">
        <div class="absolute inset-0 pointer-events-none">
             <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent"></div>
        </div>
        
        <div class="relative z-10 flex items-center gap-4">
             <button class="btn btn-circle btn-ghost hover:bg-base-200/50" on:click={goBack} aria-label={t('frontend/src/routes/files/[id]/+page.svelte::Back to files')}>
                <ArrowLeft size={22} />
             </button>
             <div>
                <h1 class="text-xl sm:text-2xl font-black tracking-tight flex items-center gap-3">
                   {#if isImage}
                      <ImageIcon size={24} class="text-primary/70" />
                      {t('frontend/src/routes/files/[id]/+page.svelte::image_viewer') || 'Image Viewer'}
                   {:else}
                      <FileCode size={24} class="text-primary/70" />
                      {$notebookStore?.metadata?.name || $notebookStore?.metadata?.title || 'Notebook'}
                   {/if}
                </h1>
                <p class="text-xs font-bold uppercase tracking-widest opacity-40 ml-9">
                   {#if isImage}
                     {t('frontend/src/routes/files/[id]/+page.svelte::preview_label') || 'Preview'}
                   {:else}
                     {t('frontend/src/routes/files/[id]/+page.svelte::notebook_label') || 'Jupyter Notebook'}
                   {/if}
                </p>
             </div>
        </div>
    </header>

    <!-- Content -->
    <div class="relative min-h-[600px]">
       {#if isImage}
          <div class="p-8 flex items-center justify-center bg-base-100/30 rounded-[2rem] border border-base-200/50 min-h-[600px]">
             {#if imgUrl}
               <img src={imgUrl} alt={t('frontend/src/routes/files/[id]/+page.svelte::image')} class="max-w-full max-h-[85vh] rounded-xl shadow-lg border border-base-content/5" />
             {/if}
          </div>
       {:else}
          <!-- No extra container for notebook to let it blend -->
          <NotebookEditor bind:this={notebookEditor} fileId={id} on:saved={handleSaved} />
       {/if}
    </div>
  </div>
</div>

<UnsavedChangesModal bind:this={unsavedModal} />

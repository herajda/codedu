<script lang="ts">
  // @ts-nocheck
  import { onMount } from "svelte";
  import { auth } from "$lib/auth";
  import { apiJSON, apiFetch } from "$lib/api";
  import { formatDateTime } from "$lib/date";
  import { TEACHER_GROUP_ID } from "$lib/teacherGroup";
  import { marked } from "marked";
  import DOMPurify from "dompurify";
  import { goto } from "$app/navigation";
  import ConfirmModal from "$lib/components/ConfirmModal.svelte";
  import PromptModal from "$lib/components/PromptModal.svelte";
  import { t, translator } from "$lib/i18n";
  import { 
    Search, Folder, FileText, Plus, FolderPlus, 
    ArrowRight, Pencil, Trash2, Eye, Copy,
    ExternalLink, Link, ArrowUpDown, ChevronRight,
    Users, LayoutGrid, List, MoreVertical, Clock,
    Info, CheckCircle2, AlertTriangle, RefreshCw
  } from 'lucide-svelte';
  
  let translate;
  $: translate = $translator;

  type ClassFile = {
    id: string;
    class_id: string;
    parent_id?: string | null;
    name: string;
    path: string;
    is_dir: boolean;
    assignment_id?: string | null;
    size?: number;
    created_at: string;
    updated_at: string;
  };

  // Fixed Teachers' group ID
  const groupId = TEACHER_GROUP_ID;

  let role = "";
  $: role = $auth?.role ?? "";

  let items: ClassFile[] = [];
  let displayed: ClassFile[] = [];
  let breadcrumbs: { id: string | null; name: string }[] = [
    {
      id: null,
      name: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::home_icon_label",
      ),
    },
  ];
  let currentParent: string | null = null;
  let loading = false;
  let err = "";

  let confirmModal: InstanceType<typeof ConfirmModal>;
  let promptModal: InstanceType<typeof PromptModal>;

  let search = "";
  let searchOpen = false;
  $: if (searchOpen && search.trim() !== "") {
    fetchSearch(search.trim());
  } else {
    displayed = filterItems(items);
  }

  function filterItems(list: ClassFile[]): ClassFile[] {
    // Only folders and assignment references
    return list.filter((it) => it.is_dir || !!it.assignment_id);
  }

  async function load(parent: string | null) {
    // Normalize root marker from sessionStorage
    if ((parent as any) === "") parent = null;
    loading = true;
    err = "";
    try {
      const q = parent === null ? "" : `?parent=${parent}`;
      const list = await apiJSON(`/api/classes/${groupId}/files${q}`);
      items = list;
      displayed = filterItems(items);
      currentParent = parent;
    } catch (e: any) {
      err = e.message;
    }
    loading = false;
  }

  async function fetchSearch(q: string) {
    loading = true;
    err = "";
    try {
      const list = await apiJSON(
        `/api/classes/${groupId}/files?search=${encodeURIComponent(q)}`,
      );
      displayed = filterItems(list);
    } catch (e: any) {
      err = e.message;
    }
    loading = false;
  }

  async function openDir(item: ClassFile) {
    breadcrumbs = [...breadcrumbs, { id: item.id, name: item.name }];
    if (typeof sessionStorage !== "undefined") {
      sessionStorage.setItem(`tassign_bc`, JSON.stringify(breadcrumbs));
      sessionStorage.setItem(`tassign_parent`, String(item.id));
    }
    await load(item.id);
  }

  function crumbTo(i: number) {
    const b = breadcrumbs[i];
    breadcrumbs = breadcrumbs.slice(0, i + 1);
    if (typeof sessionStorage !== "undefined") {
      sessionStorage.setItem(`tassign_bc`, JSON.stringify(breadcrumbs));
      sessionStorage.setItem(
        `tassign_parent`,
        b.id === null ? "" : String(b.id),
      );
    }
    load(b.id);
  }

  // Create folder
  async function createDir(name: string) {
    await apiFetch(`/api/classes/${groupId}/files`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, parent_id: currentParent, is_dir: true }),
    });
    await load(currentParent);
  }
  async function promptDir() {
    const name = await promptModal?.open({
      title: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::new_folder_title",
      ),
      label: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::folder_name_label",
      ),
      placeholder: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::folder_name_placeholder",
      ),
      confirmLabel: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::create_button_label",
      ),
      icon: "fa-solid fa-folder-plus text-primary",
      validate: (value) =>
        value.trim()
          ? null
          : t(
              "frontend/src/routes/teachers/assignments/+page.svelte::folder_name_required",
            ),
      transform: (value) => value.trim(),
    });
    if (!name) return;
    await createDir(name);
  }

  // Rename
  async function rename(it: ClassFile) {
    const name = await promptModal?.open({
      title: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::rename_title",
      ),
      label: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::new_name_label",
      ),
      initialValue: it.name,
      confirmLabel: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::save_button_label",
      ),
      icon: it.is_dir
        ? "fa-solid fa-folder text-warning"
        : "fa-solid fa-pen text-primary",
      validate: (value) =>
        value.trim()
          ? null
          : t(
              "frontend/src/routes/teachers/assignments/+page.svelte::name_required",
            ),
      transform: (value) => value.trim(),
      selectOnOpen: true,
    });
    if (!name || name === it.name) return;
    await apiFetch(`/api/files/${it.id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    });
    await load(currentParent);
  }

  // Delete
  async function del(it: ClassFile) {
    const shouldDelete = await confirmModal.open({
      title: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::delete_assignment_ref_title",
      ),
      body: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::delete_assignment_ref_body",
        { name: it.name },
      ),
      confirmLabel: t(
        "frontend/src/routes/teachers/assignments/+page.svelte::delete_button_label",
      ),
      confirmClass: "btn btn-error",
      cancelClass: "btn",
    });
    if (!shouldDelete) return;
    await apiFetch(`/api/files/${it.id}`, { method: "DELETE" });
    await load(currentParent);
  }

  // Add assignment ref modal
  let addDialog: HTMLDialogElement;
  let myAssignments: any[] = [];
  let myAssignLoading = false;
  let selectedAssignmentId = "";
  let addName = "";
  let addErr = "";
  let addClasses: any[] = [];
  let selectedAddClassId = "";
  $: filteredAddAssignments = selectedAddClassId
    ? myAssignments.filter((a) => a.class_id === selectedAddClassId)
    : [];
  $: if (!selectedAddClassId && selectedAssignmentId) {
    selectedAssignmentId = "";
  }
  $: if (selectedAddClassId && selectedAssignmentId) {
    const matches = myAssignments.some(
      (a) => a.id === selectedAssignmentId && a.class_id === selectedAddClassId,
    );
    if (!matches) selectedAssignmentId = "";
  }

  async function openAdd() {
    addErr = "";
    addName = "";
    selectedAssignmentId = "";
    selectedAddClassId = "";
    addClasses = [];
    myAssignLoading = true;
    try {
      // Teachers get their own assignments; admins get all
      const [assignments, classes] = await Promise.all([
        apiJSON("/api/assignments"),
        apiJSON("/api/classes"),
      ]);
      myAssignments = assignments;
      addClasses = classes;
    } catch (e: any) {
      addErr = e.message;
    }
    myAssignLoading = false;
    addDialog.showModal();
  }

  async function doAdd() {
    if (!selectedAssignmentId) {
      addErr = t(
        "frontend/src/routes/teachers/assignments/+page.svelte::pick_assignment_error",
      );
      return;
    }
    const body: any = {
      parent_id: currentParent,
      assignment_id: selectedAssignmentId,
    };
    if (addName.trim() !== "") body.name = addName.trim();
    const res = await apiFetch(`/api/classes/${groupId}/files`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const js = await res.json().catch(() => ({}));
      addErr = js.error || res.statusText;
      return;
    }
    addDialog.close();
    await load(currentParent);
  }

  // Copy to class modal
  let copyDialog: HTMLDialogElement;
  let myClasses: any[] = [];
  let copyClassId: string | null = null;
  let copyErr = "";
  let copyItem: ClassFile | null = null;

  async function openCopy(it: ClassFile) {
    copyErr = "";
    copyItem = it;
    copyClassId = null;
    try {
      myClasses = await apiJSON("/api/classes");
    } catch (e: any) {
      copyErr = e.message;
    }
    copyDialog.showModal();
  }

  async function doCopy() {
    if (!copyItem || !copyItem.assignment_id) {
      copyErr = t(
        "frontend/src/routes/teachers/assignments/+page.svelte::invalid_item_error",
      );
      return;
    }
    if (!copyClassId) {
      copyErr = t(
        "frontend/src/routes/teachers/assignments/+page.svelte::choose_class_error",
      );
      return;
    }
    const res = await apiFetch(
      `/api/classes/${copyClassId}/assignments/import`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ source_assignment_id: copyItem.assignment_id }),
      },
    );
    let js: any = null;
    try {
      js = await res.json();
    } catch {}
    if (!res.ok) {
      copyErr = js?.error || res.statusText;
      return;
    }
    copyDialog.close();
    if (js?.assignment_id) {
      await goto(`/assignments/${js.assignment_id}?new=1`);
    } else {
      await goto(`/classes/${copyClassId}/assignments`);
    }
  }

  // Preview logic
  let previewDialog: HTMLDialogElement;
  let preview: any = null;
  let previewLoading = false;
  let previewErr = "";
  let previewItem: ClassFile | null = null;
  let previewClass: any = null;
  let safeDesc = "";

  async function openPreview(it: ClassFile) {
    if (!it.assignment_id) return;
    previewItem = it;
    previewErr = "";
    preview = null;
    previewClass = null;
    previewLoading = true;
    previewDialog.showModal();
    try {
      const data = await apiJSON(`/api/assignments/${it.assignment_id}`);
      preview = data;
      try {
        previewClass = await apiJSON(
          `/api/classes/${data.assignment.class_id}`,
        );
      } catch {}
      try {
        safeDesc = DOMPurify.sanitize(
          (marked.parse(preview.assignment.description) as string) || "",
        );
      } catch {
        safeDesc = "";
      }
    } catch (e: any) {
      previewErr = e.message;
    }
    previewLoading = false;
  }

  function copyAssignmentLink() {
    try {
      if (!preview?.assignment?.id) return;
      const url = `${location.origin}/teachers/assignments/preview/${preview.assignment.id}`;
      navigator.clipboard?.writeText(url);
      alert(
        t(
          "frontend/src/routes/teachers/assignments/+page.svelte::link_copied_to_clipboard",
        ),
      );
    } catch {}
  }

  function openFullPreview() {
    try {
      if (!preview?.assignment?.id) return;
      const url = `/teachers/assignments/preview/${preview.assignment.id}`;
      goto(url);
    } catch {}
  }

  // Enhanced UI state
  let viewMode: 'grid' | 'list' = 'grid';
  if (typeof localStorage !== 'undefined') {
    const stored = localStorage.getItem('teacher_assignments_view');
    if (stored === 'list') viewMode = 'list';
  }

  function toggleView() {
    viewMode = viewMode === 'grid' ? 'list' : 'grid';
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem('teacher_assignments_view', viewMode);
    }
  }

  onMount(() => {
    let storedParent: string | null = null;
    if (typeof sessionStorage !== "undefined") {
      const raw = sessionStorage.getItem("tassign_parent");
      storedParent = raw && raw.trim() !== "" ? raw : null;
      const bc = sessionStorage.getItem("tassign_bc");
      if (bc) {
        try {
          breadcrumbs = JSON.parse(bc);
        } catch {}
      }
    }
    load(storedParent);
  });
</script>

<svelte:head>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@100..900&display=swap" rel="stylesheet">
  <title>{translate('frontend/src/routes/teachers/assignments/+page.svelte::assignments_heading')} | CodEdu</title>
</svelte:head>

<div class="space-y-6 pb-12">
  <!-- Premium Header -->
  <section class="relative overflow-hidden bg-base-100 rounded-3xl border border-base-200 shadow-xl shadow-base-300/30 p-6 sm:p-10 mb-8">
    <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
    <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none"></div>
    <div class="relative flex flex-col md:flex-row items-center justify-between gap-6 text-center md:text-left">
      <div class="flex-1">
        <h1 class="text-3xl sm:text-4xl font-black tracking-tight mb-2">
          {translate('frontend/src/routes/teachers/assignments/+page.svelte::assignments_heading')}
        </h1>
        <p class="text-base-content/60 font-medium max-w-xl">
          {translate('frontend/src/routes/teachers/assignments/+page.svelte::assignments_description')}
        </p>
      </div>
      
      <div class="flex flex-wrap items-center gap-3">
        {#if role === 'teacher' || role === 'admin'}
          <button class="btn btn-primary btn-sm rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] h-10 px-4 shadow-lg shadow-primary/20" on:click={openAdd}>
            <Plus size={16} />
            {translate('frontend/src/routes/teachers/assignments/+page.svelte::add_assignment_button')}
          </button>
          <button class="btn btn-ghost btn-sm rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] h-10 border border-base-300 hover:bg-base-200" on:click={promptDir}>
            <FolderPlus size={16} />
            {translate('frontend/src/routes/teachers/assignments/+page.svelte::folder_button')}
          </button>
        {/if}
      </div>
    </div>
  </section>

  <!-- Navigation & Search Bar -->
  <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 px-2">
    <!-- Breadcrumbs matching Files style -->
    <nav class="flex items-center gap-1 overflow-x-auto pb-2 lg:pb-0 no-scrollbar max-w-full">
      {#each breadcrumbs as b, i}
        <div class="flex items-center gap-1 shrink-0">
          <button 
            type="button" 
            class={`btn btn-sm btn-ghost rounded-xl px-3 font-bold text-xs h-9 ${i === breadcrumbs.length - 1 ? 'bg-base-200/50' : 'opacity-60 hover:opacity-100'}`}
            on:click={() => crumbTo(i)}
          >
            {b.name}
          </button>
          {#if i < breadcrumbs.length - 1}
            <ChevronRight size={14} class="opacity-20" />
          {/if}
        </div>
      {/each}
    </nav>

    <div class="flex flex-wrap items-center gap-3 justify-end">
      <div class="relative flex items-center w-full sm:w-auto">
        <Search size={14} class="absolute left-3 opacity-40" />
        <input 
          type="text" 
          class="input input-sm bg-base-100 border-base-200 focus:border-primary/30 w-full sm:w-48 pl-9 rounded-xl font-medium text-xs h-9" 
          placeholder={translate('frontend/src/routes/teachers/assignments/+page.svelte::search_placeholder')} 
          bind:value={search}
          on:focus={() => searchOpen = true}
        />
      </div>

      <div class="flex items-center bg-base-200/50 p-1 rounded-xl h-9">
        <button 
          title={translate('frontend/src/routes/teachers/assignments/+page.svelte::grid_view_tooltip')}
          class={`btn btn-xs border-none rounded-lg w-8 h-7 px-0 ${viewMode === 'grid' ? 'bg-base-100 shadow-sm text-primary' : 'bg-transparent opacity-60'}`} 
          on:click={toggleView}
        >
          <LayoutGrid size={14} />
        </button>
        <button 
          title={translate('frontend/src/routes/teachers/assignments/+page.svelte::list_view_tooltip')}
          class={`btn btn-xs border-none rounded-lg w-8 h-7 px-0 ${viewMode === 'list' ? 'bg-base-100 shadow-sm text-primary' : 'bg-transparent opacity-60'}`} 
          on:click={toggleView}
        >
          <List size={14} />
        </button>
      </div>
    </div>
  </div>

  {#if err}
    <div class="alert alert-error rounded-2xl shadow-sm">
      <AlertTriangle size={20} />
      <span>{err}</span>
    </div>
  {/if}

  {#if loading && !displayed.length}
    <div class="flex justify-center py-12">
      <span class="loading loading-dots loading-lg text-primary"></span>
    </div>
  {:else}
    {#if viewMode === 'grid'}
      <!-- GRID VIEW -->
      <div class="grid gap-6 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {#each displayed as it (it.id)}
          <div 
            class="group relative bg-base-100 hover:bg-gradient-to-br hover:from-base-100 hover:to-base-200/50 border border-base-200/60 rounded-[2.5rem] p-5 flex flex-col items-center gap-4 hover:shadow-xl hover:shadow-primary/5 hover:border-primary/20 hover:-translate-y-1 transition-all duration-300 cursor-pointer overflow-hidden"
            on:click={() => it.is_dir ? openDir(it) : openPreview(it)}
          >
            <div class={`absolute top-0 right-0 w-24 h-24 rounded-bl-full transition-all duration-500 opacity-0 group-hover:opacity-100 ${it.is_dir ? 'bg-amber-400/10' : 'bg-primary/10'}`}></div>
            
            <div class="w-16 h-16 flex items-center justify-center relative z-10 shrink-0">
              {#if it.is_dir}
                <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-amber-100 to-orange-100 dark:from-amber-900/30 dark:to-orange-900/30 text-amber-600 dark:text-amber-500 flex items-center justify-center shadow-sm group-hover:scale-110 group-hover:shadow-amber-500/20 transition-all duration-300">
                  <Folder size={32} fill="currentColor" fill-opacity="0.2" />
                </div>
              {:else}
                <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 text-blue-600 dark:text-blue-400 flex items-center justify-center shadow-sm group-hover:scale-110 group-hover:shadow-blue-500/20 transition-all duration-300">
                  <FileText size={32} strokeWidth={1.5} />
                </div>
              {/if}
            </div>

            <div class="text-center w-full min-w-0 z-10">
              <h3 class="font-black text-base tracking-tight truncate px-2 text-base-content group-hover:text-primary transition-colors" title={it.name}>
                {it.name}
              </h3>
              <div class="flex items-center justify-center gap-2 mt-1.5 opacity-50 group-hover:opacity-70 transition-opacity">
                <span class="text-[10px] font-bold uppercase tracking-widest leading-none">
                  {it.is_dir ? translate('frontend/src/routes/teachers/assignments/+page.svelte::type_folder') : translate('frontend/src/routes/teachers/assignments/+page.svelte::type_assignment')}
                </span>
                <span class="w-1 h-1 rounded-full bg-base-content/20"></span>
                <span class="text-[10px] font-bold uppercase tracking-widest leading-none">
                  {formatDateTime(it.updated_at).split(' ')[0]}
                </span>
              </div>
            </div>

            {#if role === "teacher" || role === "admin"}
              <div class="absolute top-4 right-4 flex flex-col gap-2 opacity-0 group-hover:opacity-100 transition-all duration-300 translate-x-2 group-hover:translate-x-0">
                {#if it.assignment_id}
                  <button class="btn btn-xs btn-circle bg-white/80 dark:bg-black/50 backdrop-blur border-none hover:bg-primary hover:text-primary-content shadow-sm transition-colors" title={translate('frontend/src/routes/teachers/assignments/+page.svelte::copy_to_class')} on:click|stopPropagation={() => openCopy(it)}>
                    <Copy size={12} />
                  </button>
                {/if}
                <button class="btn btn-xs btn-circle bg-white/80 dark:bg-black/50 backdrop-blur border-none hover:bg-primary hover:text-primary-content shadow-sm transition-colors" title={translate('frontend/src/routes/teachers/assignments/+page.svelte::rename')} on:click|stopPropagation={() => rename(it)}>
                  <Pencil size={12} />
                </button>
                <button class="btn btn-xs btn-circle bg-white/80 dark:bg-black/50 backdrop-blur border-none hover:bg-error hover:text-error-content shadow-sm transition-colors" title={translate('frontend/src/routes/teachers/assignments/+page.svelte::delete')} on:click|stopPropagation={() => del(it)}>
                  <Trash2 size={12} />
                </button>
              </div>
            {/if}

            <div class="mt-auto w-full pt-4 border-t border-base-200/50 flex items-center justify-end">
              <div class="text-primary opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all duration-300">
                <ArrowRight size={16} />
              </div>
            </div>
          </div>
        {:else}
          <div class="col-span-full py-24 text-center bg-base-100/50 rounded-[3rem] border-2 border-dashed border-base-200 flex flex-col items-center justify-center">
            <div class="w-16 h-16 rounded-full bg-base-200/50 flex items-center justify-center mb-6">
              <FileText size={32} class="opacity-20" />
            </div>
            <p class="text-xs font-black opacity-30 uppercase tracking-[0.2em]">
              {translate('frontend/src/routes/teachers/assignments/+page.svelte::nothing_here_yet')}
            </p>
          </div>
        {/each}
      </div>
    {:else}
      <!-- LIST VIEW -->
      <div class="bg-base-100 rounded-[2.5rem] border border-base-200 shadow-xl shadow-base-200/50 overflow-hidden">
        <table class="table w-full">
          <thead>
            <tr class="border-b border-base-100">
              <th class="bg-base-100/50 text-[10px] font-black uppercase tracking-widest opacity-40 py-6 pl-8">{translate('frontend/src/routes/teachers/assignments/+page.svelte::table_header_name')}</th>
              <th class="bg-base-100/50 text-[10px] font-black uppercase tracking-widest opacity-40 text-left py-6">{translate('frontend/src/routes/teachers/assignments/+page.svelte::table_header_type')}</th>
              <th class="bg-base-100/50 text-[10px] font-black uppercase tracking-widest opacity-40 text-right py-6 pr-8">{translate('frontend/src/routes/teachers/assignments/+page.svelte::table_header_modified')}</th>
              {#if role === 'teacher' || role === 'admin'}<th class="bg-base-100/50 w-32 pr-8"></th>{/if}
            </tr>
          </thead>
          <tbody class="divide-y divide-base-100">
            {#each displayed as it (it.id)}
              <tr class="hover:bg-base-50 transition-colors cursor-pointer group" on:click={() => it.is_dir ? openDir(it) : openPreview(it)}>
                <td class="py-4 pl-8">
                  <div class="flex items-center gap-5">
                    <div class={`w-12 h-12 rounded-2xl flex items-center justify-center shrink-0 shadow-sm transition-transform group-hover:scale-110 duration-300 ${it.is_dir ? 'bg-gradient-to-br from-amber-100 to-orange-100 dark:from-amber-900/30 dark:to-orange-900/30 text-amber-600' : 'bg-gradient-to-br from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 text-blue-600'}`}>
                       {#if it.is_dir}<Folder size={22} fill="currentColor" fill-opacity="0.2" />{:else}<FileText size={22} />{/if}
                    </div>
                    <div class="min-w-0">
                       <div class="font-bold text-sm tracking-tight truncate text-base-content/90 group-hover:text-primary transition-colors">{it.name}</div>
                       <div class="text-[10px] opacity-40 truncate mt-0.5">{it.path}</div>
                    </div>
                  </div>
                </td>
                <td class="text-left text-xs font-bold opacity-50 uppercase tracking-widest py-4">
                  {it.is_dir ? translate('frontend/src/routes/teachers/assignments/+page.svelte::type_folder') : translate('frontend/src/routes/teachers/assignments/+page.svelte::type_assignment')}
                </td>
                <td class="text-right text-xs font-bold opacity-50 py-4 pr-8">{formatDateTime(it.updated_at)}</td>

                {#if role === 'teacher' || role === 'admin'}
                  <td class="text-right py-4 pr-8">
                    <div class="flex items-center justify-end gap-2 opacity-0 group-hover:opacity-100 transition-all duration-200 translate-x-2 group-hover:translate-x-0">
                      {#if it.assignment_id}
                        <button class="btn btn-xs btn-circle btn-ghost hover:bg-base-200 text-base-content/60 hover:text-primary" on:click|stopPropagation={() => openCopy(it)}>
                          <Copy size={14} />
                        </button>
                      {/if}
                      <button class="btn btn-xs btn-circle btn-ghost hover:bg-base-200 text-base-content/60 hover:text-primary" on:click|stopPropagation={() => rename(it)}>
                        <Pencil size={14} />
                      </button>
                      <button class="btn btn-xs btn-circle btn-ghost hover:bg-base-200 text-base-content/60 hover:text-error" on:click|stopPropagation={() => del(it)}>
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                {/if}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}

  <!-- Modals -->
  <!-- Redesign Add assignment modal -->
  <dialog bind:this={addDialog} class="modal backdrop-blur-sm">
    <div class="modal-box rounded-[2.5rem] p-8 max-w-lg border border-base-200 shadow-2xl">
      <div class="flex items-center gap-4 mb-8">
        <div class="w-12 h-12 rounded-2xl bg-primary/10 text-primary flex items-center justify-center">
          <Plus size={24} />
        </div>
        <div>
          <h3 class="text-xl font-black tracking-tight leading-none">
            {translate("frontend/src/routes/teachers/assignments/+page.svelte::add_assignment_modal_title")}
          </h3>
          <p class="text-sm opacity-50 mt-1 font-medium">{translate('frontend/src/routes/teachers/assignments/+page.svelte::add_assignment_subtitle')}</p>
        </div>
      </div>
      
      {#if myAssignLoading}
        <div class="flex flex-col items-center py-8">
          <span class="loading loading-spinner loading-lg text-primary"></span>
        </div>
      {:else}
        <div class="space-y-6">
          <div class="form-control">
            <label class="label pt-0 pb-3"
              ><span class="text-[11px] font-black uppercase tracking-[0.15em] opacity-40"
                >{translate("frontend/src/routes/teachers/assignments/+page.svelte::choose_class_label")}</span
              ></label
            >
            <div class="relative group">
              <select
                class="select select-bordered w-full rounded-2xl bg-base-200/50 border-base-300 group-focus-within:border-primary/30 transition-all pl-10"
                bind:value={selectedAddClassId}
              >
                <option value="" disabled>{translate("frontend/src/routes/teachers/assignments/+page.svelte::select_a_class_placeholder")}</option>
                {#each addClasses as c}
                  <option value={c.id}>{c.name}</option>
                {/each}
              </select>
              <Users size={16} class="absolute left-4 top-1/2 -translate-y-1/2 opacity-30" />
            </div>
            {#if !addClasses.length && !addErr}
              <p class="mt-3 text-xs opacity-50 font-medium px-1 flex items-center gap-2">
                <Info size={12} /> {translate("frontend/src/routes/teachers/assignments/+page.svelte::no_classes_yet")}
              </p>
            {/if}
          </div>

          <div class="form-control">
            <label class="label pt-0 pb-3"
              ><span class="text-[11px] font-black uppercase tracking-[0.15em] opacity-40"
                >{translate("frontend/src/routes/teachers/assignments/+page.svelte::choose_assignment_label")}</span
              ></label
            >
            <div class="relative group">
              <select
                class="select select-bordered w-full rounded-2xl bg-base-200/50 border-base-300 group-focus-within:border-primary/30 transition-all pl-10"
                bind:value={selectedAssignmentId}
                disabled={!selectedAddClassId || !filteredAddAssignments.length}
              >
                <option value="" disabled
                  >{!selectedAddClassId
                    ? translate("frontend/src/routes/teachers/assignments/+page.svelte::select_class_first_placeholder")
                    : !filteredAddAssignments.length
                      ? translate("frontend/src/routes/teachers/assignments/+page.svelte::no_assignments_in_class_placeholder")
                      : translate("frontend/src/routes/teachers/assignments/+page.svelte::select_assignment_placeholder")}</option
                >
                {#if selectedAddClassId}
                  {#each filteredAddAssignments as a (a.id)}
                    <option value={a.id}>{a.title}</option>
                  {/each}
                {/if}
              </select>
              <FileText size={16} class="absolute left-4 top-1/2 -translate-y-1/2 opacity-30" />
            </div>
          </div>

          <div class="form-control">
            <label class="label pt-0 pb-3"
              ><span class="text-[11px] font-black uppercase tracking-[0.15em] opacity-40"
                >{translate("frontend/src/routes/teachers/assignments/+page.svelte::display_name_optional_label")}</span
              ></label
            >
            <div class="relative group">
              <input
                class="input input-bordered w-full rounded-2xl bg-base-200/50 border-base-300 group-focus-within:border-primary/30 transition-all pl-10"
                bind:value={addName}
                placeholder={translate("frontend/src/routes/teachers/assignments/+page.svelte::defaults_to_assignment_title_placeholder")}
              />
              <Pencil size={16} class="absolute left-4 top-1/2 -translate-y-1/2 opacity-30" />
            </div>
          </div>

          {#if addErr}
            <div class="p-4 rounded-2xl bg-error/10 text-error flex items-start gap-3 mt-4">
              <AlertTriangle size={18} class="shrink-0 mt-0.5" />
              <p class="text-sm font-bold tracking-tight">{addErr}</p>
            </div>
          {/if}

          <div class="flex gap-3 justify-end mt-8">
            <form method="dialog"><button class="btn btn-ghost rounded-2xl px-6 font-black uppercase tracking-widest text-[10px]">{translate('frontend/src/routes/teachers/assignments/+page.svelte::close_button')}</button></form>
            <button class="btn btn-primary rounded-2xl px-8 font-black uppercase tracking-widest text-[10px] shadow-lg shadow-primary/20" on:click|preventDefault={doAdd}>{translate('frontend/src/routes/teachers/assignments/+page.svelte::add_assignment_confirm_button')}</button>
          </div>
        </div>
      {/if}
    </div>
    <form method="dialog" class="modal-backdrop bg-black/20 backdrop-blur-sm"><button>close</button></form>
  </dialog>

  <!-- Redesign Copy to class modal -->
  <dialog bind:this={copyDialog} class="modal backdrop-blur-sm">
    <div class="modal-box rounded-[2.5rem] p-8 max-w-lg border border-base-200 shadow-2xl">
      <div class="flex items-center gap-4 mb-8">
        <div class="w-12 h-12 rounded-2xl bg-primary/10 text-primary flex items-center justify-center">
          <Copy size={24} />
        </div>
        <div>
          <h3 class="text-xl font-black tracking-tight leading-none">
            {translate("frontend/src/routes/teachers/assignments/+page.svelte::copy_to_my_class_modal_title")}
          </h3>
          <p class="text-sm opacity-50 mt-1 font-medium">{translate('frontend/src/routes/teachers/assignments/+page.svelte::copy_subtitle')}</p>
        </div>
      </div>

      <div class="space-y-6">
        <div class="form-control">
          <label class="label pt-0 pb-3"
            ><span class="text-[11px] font-black uppercase tracking-[0.15em] opacity-40"
              >{translate("frontend/src/routes/teachers/assignments/+page.svelte::choose_class_label_short")}</span
            ></label
          >
          <div class="relative group">
            <select class="select select-bordered w-full rounded-2xl bg-base-200/50 border-base-300 group-focus-within:border-primary/30 transition-all pl-10" bind:value={copyClassId}>
              <option value="" disabled selected>{translate("frontend/src/routes/teachers/assignments/+page.svelte::select_ellipsis_placeholder")}</option>
              {#each myClasses as c}
                <option value={c.id}>{c.name}</option>
              {/each}
            </select>
            <Users size={16} class="absolute left-4 top-1/2 -translate-y-1/2 opacity-30" />
          </div>
        </div>

        {#if copyErr}
          <div class="p-4 rounded-2xl bg-error/10 text-error flex items-start gap-3 mt-4">
            <AlertTriangle size={18} class="shrink-0 mt-0.5" />
            <p class="text-sm font-bold tracking-tight">{copyErr}</p>
          </div>
        {/if}

        <div class="flex gap-3 justify-end mt-8">
          <form method="dialog"><button class="btn btn-ghost rounded-2xl px-6 font-black uppercase tracking-widest text-[10px]">{translate('frontend/src/routes/teachers/assignments/+page.svelte::cancel_button')}</button></form>
          <button class="btn btn-primary rounded-2xl px-8 font-black uppercase tracking-widest text-[10px] shadow-lg shadow-primary/20" on:click|preventDefault={doCopy}>{translate('frontend/src/routes/teachers/assignments/+page.svelte::copy_assignment_confirm_button')}</button>
        </div>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop bg-black/20 backdrop-blur-sm"><button>close</button></form>
  </dialog>

  <!-- Preview modal -->
  <dialog bind:this={previewDialog} class="modal backdrop-blur-sm">
    <div class="modal-box max-w-4xl rounded-[2.5rem] p-0 border border-base-200 shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
      {#if previewLoading}
        <div class="py-24 text-center">
          <span class="loading loading-dots loading-lg text-primary"></span>
        </div>
      {:else if previewErr}
        <div class="p-8 text-center bg-error/5 flex flex-col items-center">
          <AlertTriangle size={32} class="text-error mb-4" />
          <p class="text-error font-black uppercase tracking-widest text-xs mb-2">{translate('frontend/src/routes/teachers/assignments/+page.svelte::error_loading_preview')}</p>
          <p class="text-base-content/60">{previewErr}</p>
        </div>
      {:else if preview?.assignment}
        <!-- Premium Modal Header -->
        <div class="bg-base-100 p-8 border-b border-base-200 relative">
          <div class="absolute top-0 right-0 w-1/3 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
          
          <div class="flex flex-col md:flex-row items-start justify-between gap-6 relative">
            <div class="flex-1">
              <div class="flex flex-wrap items-center gap-2 mb-3">
                <span class="badge bg-primary/10 text-primary border-none font-black text-[9px] uppercase tracking-widest h-6 px-3">
                  <FileText size={10} class="mr-1.5" /> {translate('frontend/src/routes/teachers/assignments/+page.svelte::type_assignment', 'Assignment')}
                </span>
                {#if previewClass}
                  <a href={`/classes/${preview.assignment.class_id}`} class="badge bg-base-200 border-none hover:bg-base-300 transition-colors font-black text-[9px] uppercase tracking-widest h-6 px-3 flex items-center gap-1.5 no-underline text-current">
                    <Users size={10} /> {previewClass.name}
                  </a>
                {/if}
                <span class={`badge ${preview.assignment.published ? "bg-success/10 text-success" : "bg-base-200 opacity-60"} border-none font-black text-[9px] uppercase tracking-widest h-6 px-3`}>
                  {preview.assignment.published ? translate('frontend/src/routes/teachers/assignments/+page.svelte::published_text') : translate('frontend/src/routes/teachers/assignments/+page.svelte::unpublished_text')}
                </span>
              </div>
              <h3 class="text-2xl sm:text-3xl font-black tracking-tight text-base-content leading-tight group">
                {preview.assignment.title}
              </h3>
            </div>
            
            <div class="flex flex-wrap items-center gap-2 shrink-0 self-end md:self-start">
               <button class="btn btn-sm bg-base-100 border-base-200 hover:bg-base-200 rounded-xl px-4 gap-2 font-black uppercase tracking-widest text-[10px] h-9 border shadow-sm" on:click={copyAssignmentLink}>
                <Link size={14} /> {translate('frontend/src/routes/teachers/assignments/+page.svelte::copy_link_button')}
              </button>
              <button class="btn btn-sm btn-primary rounded-xl px-4 gap-2 font-black uppercase tracking-widest text-[10px] h-9 shadow-lg shadow-primary/20" on:click={() => { if (previewItem) openCopy(previewItem); }}>
                <Copy size={14} /> {translate('frontend/src/routes/teachers/assignments/+page.svelte::copy_to_class_button')}
              </button>
            </div>
          </div>
        </div>

        <div class="overflow-y-auto p-8 space-y-8 flex-1">
          {#if safeDesc}
            <div class="prose prose-sm max-w-none prose-headings:font-black prose-headings:tracking-tight prose-p:font-medium prose-p:leading-relaxed text-base-content/80 assignment-description">
              {@html safeDesc}
            </div>
          {/if}

          <!-- Info Cards -->
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            <div class="bg-base-200/50 rounded-3xl p-6 border border-base-200 flex flex-col shadow-sm">
              <div class="flex items-center gap-3 mb-4 opacity-40">
                <LayoutGrid size={16} />
                <span class="text-[10px] font-black uppercase tracking-widest">{translate('frontend/src/routes/teachers/assignments/+page.svelte::specifications_label')}</span>
              </div>
              <div class="space-y-4">
                <div class="flex justify-between items-center">
                  <span class="text-xs font-bold opacity-60 uppercase tracking-wider">{translate("frontend/src/routes/teachers/assignments/+page.svelte::max_points_label")}</span>
                  <span class="font-black text-lg">{preview.assignment.max_points}</span>
                </div>
                <div class="flex justify-between items-center">
                  <span class="text-xs font-bold opacity-60 uppercase tracking-wider">{translate("frontend/src/routes/teachers/assignments/+page.svelte::grading_label")}</span>
                  <span class="font-black text-xs uppercase tracking-widest text-primary">{preview.assignment.grading_policy}</span>
                </div>
              </div>
            </div>

            <div class="bg-base-200/50 rounded-3xl p-6 border border-base-200 flex flex-col shadow-sm">
              <div class="flex items-center gap-3 mb-4 opacity-40">
                <CheckCircle2 size={16} />
                <span class="text-[10px] font-black uppercase tracking-widest">{translate("frontend/src/routes/teachers/assignments/+page.svelte::tests_card_title")}</span>
              </div>
              <div class="flex-1 flex flex-col justify-center">
                <span class="font-black text-4xl mb-1 tabular-nums">{(preview.tests ?? []).length}</span>
                <span class="text-[10px] font-bold opacity-60 uppercase tracking-widest">{translate('frontend/src/routes/teachers/assignments/+page.svelte::automated_tests_defined_label')}</span>
              </div>
            </div>

            <div class="bg-base-200/50 rounded-3xl p-6 border border-base-200 flex flex-col shadow-sm">
              <div class="flex items-center gap-3 mb-4 opacity-40">
                <RefreshCw size={16} />
                <span class="text-[10px] font-black uppercase tracking-widest">{translate('frontend/src/routes/teachers/assignments/+page.svelte::activity_label')}</span>
              </div>
              <div class="space-y-4">
                <div class="flex justify-between items-center">
                  <span class="text-xs font-bold opacity-60 uppercase tracking-wider">{translate("frontend/src/routes/teachers/assignments/+page.svelte::student_submissions_label")}</span>
                  <span class="font-black tabular-nums">{(preview.submissions ?? []).length}</span>
                </div>
                <div class="flex justify-between items-center">
                  <span class="text-xs font-bold opacity-60 uppercase tracking-wider">{translate("frontend/src/routes/teachers/assignments/+page.svelte::teacher_runs_label")}</span>
                  <span class="font-black tabular-nums">{(preview.teacher_runs ?? []).length}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="p-6 bg-base-200/50 border-t border-base-200 flex flex-col sm:flex-row gap-4 items-center justify-between">
          <div class="flex items-center gap-4 text-[10px] font-bold opacity-40 uppercase tracking-widest">
            <span class="flex items-center gap-1.5"><Clock size={12} /> {translate('frontend/src/routes/teachers/assignments/+page.svelte::created_at_prefix')} {formatDateTime(preview.assignment.created_at)}</span>
          </div>
          <div class="flex gap-3 w-full sm:w-auto">
            <button class="btn btn-ghost rounded-2xl px-6 font-black uppercase tracking-widest text-[10px] flex-1 sm:flex-none" on:click={() => previewDialog.close()}>
              {translate('frontend/src/routes/teachers/assignments/+page.svelte::close_button')}
            </button>
            <button class="btn btn-primary rounded-2xl px-8 font-black uppercase tracking-widest text-[10px] shadow-lg shadow-primary/20 flex-1 sm:flex-none gap-2" on:click={openFullPreview}>
              <ExternalLink size={14} /> {translate('frontend/src/routes/teachers/assignments/+page.svelte::open_full_view_button')}
            </button>
          </div>
        </div>
      {/if}
    </div>
    <form method="dialog" class="modal-backdrop bg-black/40 backdrop-blur-sm"><button>close</button></form>
  </dialog>

  <ConfirmModal bind:this={confirmModal} />
  <PromptModal bind:this={promptModal} />
</div>

<style>
  :global(.assignment-description img) {
    border-radius: 1rem;
    box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1), 0 24px 48px -12px rgb(0 0 0 / 0.1);
    margin: 2rem auto;
  }
</style>

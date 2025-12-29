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
  import CustomSelect from "$lib/components/CustomSelect.svelte";
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
      name: "🏠",
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
  $: addClassOptions = addClasses.map((c) => ({ value: String(c.id), label: c.name }));
  $: filteredAddAssignments = selectedAddClassId
    ? myAssignments.filter((a) => a.class_id === selectedAddClassId)
    : [];
  $: filteredAddAssignmentOptions = filteredAddAssignments.map((a) => ({
    value: String(a.id),
    label: a.title,
  }));
  $: assignmentSelectPlaceholder = !selectedAddClassId
    ? translate("frontend/src/routes/teachers/assignments/+page.svelte::select_class_first_placeholder")
    : !filteredAddAssignments.length
      ? translate("frontend/src/routes/teachers/assignments/+page.svelte::no_assignments_in_class_placeholder")
      : translate("frontend/src/routes/teachers/assignments/+page.svelte::select_assignment_placeholder");
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
  $: copyClassOptions = myClasses.map((c) => ({ value: String(c.id), label: c.name }));
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
          const parsed = JSON.parse(bc);
          if (Array.isArray(parsed) && parsed.length > 0) {
            // Force the root breadcrumb to be the home icon
            parsed[0].name = "🏠";
            breadcrumbs = parsed;
          }
        } catch {}
      }
    }
    // Ensure breadcrumbs is never empty and root is always the icon
    if (breadcrumbs.length === 0) {
      breadcrumbs = [{ id: null, name: "🏠" }];
    } else {
      breadcrumbs[0].name = "🏠";
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
            class={`btn btn-sm btn-ghost rounded-xl px-3 font-bold text-xs h-9 select-none ${i === breadcrumbs.length - 1 ? 'bg-base-200/50' : 'opacity-60 hover:opacity-100'}`}
            on:click={() => crumbTo(i)}
          >
            <span class="pointer-events-none">
              {#if b.id === null}
                🏠
              {:else}
                {b.name}
              {/if}
            </span>
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
      <div class="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 mb-8">
        {#each displayed as it (it.id)}
          <div 
            class="group relative bg-base-100 border border-base-200 rounded-[2rem] p-4 flex flex-col items-center gap-3 hover:shadow-xl hover:shadow-primary/5 hover:border-primary/20 transition-all cursor-pointer overflow-hidden select-none"
            on:click={() => it.is_dir ? openDir(it) : openPreview(it)}
          >
            <div class="absolute top-0 right-0 w-12 h-12 bg-primary/5 rounded-bl-full opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none"></div>
            
            <div class="w-16 h-16 flex items-center justify-center relative">
              {#if it.is_dir}
                <div class="text-warning group-hover:scale-110 transition-transform duration-300">
                  <Folder size={48} fill="currentColor" fill-opacity="0.1" />
                </div>
              {:else}
                <div class="text-primary group-hover:scale-110 transition-transform duration-300">
                  <FileText size={40} />
                </div>
              {/if}
            </div>

            <div class="text-center w-full min-w-0">
              <h3 class="font-black text-xs tracking-tight truncate px-1 group-hover:text-primary transition-colors" title={it.name}>
                {it.name}
              </h3>
              <div class="text-[9px] font-bold uppercase tracking-widest opacity-40 mt-1 whitespace-nowrap">
                {it.is_dir ? translate('frontend/src/routes/teachers/assignments/+page.svelte::type_folder') : translate('frontend/src/routes/teachers/assignments/+page.svelte::type_assignment')}
                •
                {formatDateTime(it.updated_at).split(' ')[0]}
              </div>
            </div>

            {#if search.trim() !== ''}
              <div class="text-[8px] opacity-30 truncate w-full text-center mt-1">{it.path}</div>
            {/if}

            {#if role === "teacher" || role === "admin"}
              <div class="absolute top-2 right-2 flex flex-col gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                {#if it.assignment_id}
                  <button class="btn btn-xs btn-circle bg-base-100 border-base-200 shadow-sm hover:text-primary" title={translate('frontend/src/routes/teachers/assignments/+page.svelte::copy_to_class')} on:click|stopPropagation={() => openCopy(it)}>
                    <Copy size={10} />
                  </button>
                {/if}
                <button class="btn btn-xs btn-circle bg-base-100 border-base-200 shadow-sm hover:text-primary" title={translate('frontend/src/routes/teachers/assignments/+page.svelte::rename')} on:click|stopPropagation={() => rename(it)}>
                  <Pencil size={10} />
                </button>
                <button class="btn btn-xs btn-circle btn-error btn-outline border-none bg-base-100 shadow-sm" title={translate('frontend/src/routes/teachers/assignments/+page.svelte::delete')} on:click|stopPropagation={() => del(it)}>
                  <Trash2 size={10} />
                </button>
              </div>
            {/if}
          </div>
        {:else}
          <div 
            class="col-span-full py-20 text-center bg-base-100/50 rounded-[3rem] border-2 border-dashed border-base-200 flex flex-col items-center justify-center select-none {role === 'teacher' || role === 'admin' ? 'cursor-pointer hover:border-primary/30 transition-colors' : ''}"
            on:click={() => { if (role === 'teacher' || role === 'admin') openAdd(); }}
          >
             <div class="w-16 h-16 rounded-full bg-base-200 flex items-center justify-center mb-4 opacity-30 pointer-events-none">
                <FileText size={32} />
             </div>
             <p class="text-sm font-bold opacity-30 uppercase tracking-[0.2em] pointer-events-none">
               {translate('frontend/src/routes/teachers/assignments/+page.svelte::nothing_here_yet')}
             </p>
             {#if role === 'teacher' || role === 'admin'}
               <p class="text-[10px] font-black uppercase tracking-widest opacity-20 mt-2 pointer-events-none">Click to add your first assignment reference</p>
             {/if}
          </div>
        {/each}
      </div>
    {:else}
      <!-- LIST VIEW -->
      <div class="bg-base-100 rounded-[2rem] border border-base-200 shadow-sm overflow-hidden mb-8">
        <table class="table w-full">
          <thead>
            <tr class="border-b border-base-200 hover:bg-transparent">
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40 py-5 pl-8">{translate('frontend/src/routes/teachers/assignments/+page.svelte::table_header_name')}</th>
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40 text-left py-5">{translate('frontend/src/routes/teachers/assignments/+page.svelte::table_header_type')}</th>
              <th class="bg-base-100 text-[10px] font-black uppercase tracking-widest opacity-40 text-right py-5 pr-8">{translate('frontend/src/routes/teachers/assignments/+page.svelte::table_header_modified')}</th>
              {#if role === 'teacher' || role === 'admin'}<th class="bg-base-100 w-24 pr-8"></th>{/if}
            </tr>
          </thead>
          <tbody class="divide-y divide-base-100">
            {#each displayed as it (it.id)}
              <tr class="hover:bg-base-200/50 cursor-pointer group transition-colors select-none" on:click={() => it.is_dir ? openDir(it) : openPreview(it)}>
                <td class="py-4 pl-8">
                  <div class="flex items-center gap-4">
                    <div class={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${it.is_dir ? 'bg-warning/10 text-warning' : 'bg-primary/10 text-primary'} group-hover:scale-110 transition-transform`}>
                       {#if it.is_dir}<Folder size={18} fill="currentColor" fill-opacity="0.2" />{:else}<FileText size={18} />{/if}
                    </div>
                    <div class="min-w-0">
                       <div class="font-black text-sm tracking-tight truncate group-hover:text-primary transition-colors">{it.name}</div>
                       {#if search.trim() !== ''}
                        <div class="text-[10px] opacity-30 truncate">{it.path}</div>
                       {/if}
                    </div>
                  </div>
                </td>
                <td class="text-left text-xs font-medium opacity-60 uppercase tracking-widest py-4">
                  {it.is_dir ? translate('frontend/src/routes/teachers/assignments/+page.svelte::type_folder') : translate('frontend/src/routes/teachers/assignments/+page.svelte::type_assignment')}
                </td>
                <td class="text-right text-xs font-medium opacity-60 py-4 pr-8">{formatDateTime(it.updated_at)}</td>

                {#if role === 'teacher' || role === 'admin'}
                  <td class="text-right py-4 pr-8">
                    <div class="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      {#if it.assignment_id}
                        <button class="btn btn-xs btn-circle btn-ghost" title={translate('frontend/src/routes/teachers/assignments/+page.svelte::copy_to_class')} on:click|stopPropagation={() => openCopy(it)}>
                          <Copy size={12} />
                        </button>
                      {/if}
                      <button class="btn btn-xs btn-circle btn-ghost" title={translate('frontend/src/routes/teachers/assignments/+page.svelte::rename')} on:click|stopPropagation={() => rename(it)}>
                        <Pencil size={12} />
                      </button>
                      <button class="btn btn-xs btn-circle btn-ghost text-error hover:bg-error/10" title={translate('frontend/src/routes/teachers/assignments/+page.svelte::delete')} on:click|stopPropagation={() => del(it)}>
                        <Trash2 size={12} />
                      </button>
                    </div>
                  </td>
                {/if}
              </tr>
            {:else}
                <tr>
                  <td colspan={role === 'teacher' || role === 'admin' ? 4 : 3} class="py-20 text-center">
                    <div 
                      class="flex flex-col items-center justify-center opacity-30 select-none {role === 'teacher' || role === 'admin' ? 'cursor-pointer hover:opacity-100 transition-opacity' : ''}"
                      on:click={() => { if (role === 'teacher' || role === 'admin') openAdd(); }}
                    >
                       <FileText size={32} class="mb-2 pointer-events-none" />
                       <p class="text-xs font-bold uppercase tracking-widest pointer-events-none">{translate('frontend/src/routes/teachers/assignments/+page.svelte::nothing_here_yet')}</p>
                       {#if role === 'teacher' || role === 'admin'}
                          <p class="text-[10px] font-black uppercase tracking-widest mt-2 pointer-events-none">Click to add your first assignment reference</p>
                       {/if}
                    </div>
                  </td>
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
            <CustomSelect
              options={addClassOptions}
              bind:value={selectedAddClassId}
              placeholder={translate("frontend/src/routes/teachers/assignments/+page.svelte::select_a_class_placeholder")}
              icon={Users}
            />
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
            <CustomSelect
              options={filteredAddAssignmentOptions}
              bind:value={selectedAssignmentId}
              placeholder={assignmentSelectPlaceholder}
              icon={FileText}
              disabled={!selectedAddClassId || !filteredAddAssignments.length}
            />
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
          <CustomSelect
            options={copyClassOptions}
            bind:value={copyClassId}
            placeholder={translate("frontend/src/routes/teachers/assignments/+page.svelte::select_ellipsis_placeholder")}
            icon={Users}
          />
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

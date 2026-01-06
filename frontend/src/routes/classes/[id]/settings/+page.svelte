<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { auth } from '$lib/auth';
  import { apiFetch, apiJSON } from '$lib/api';
  import { MarkdownEditor } from '$lib';
  import { login as bkLogin, getAtoms, getStudents, hasBakalari } from '$lib/bakalari';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { classesStore } from '$lib/stores/classes';
  import { BookOpen, FileText, Pencil, Trash2, UserPlus, UserMinus, Search as SearchIcon, Loader2, Check, X, Users, Download, ArrowRight } from 'lucide-svelte';
  import DOMPurify from 'dompurify';
  import { marked } from 'marked';
  import { t, translator } from '$lib/i18n';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';

  let translate;
  $: translate = $translator;

  let id = $page.params.id;
  $: if ($page.params.id !== id) { id = $page.params.id; load(); }
  let role = '';
  $: role = $auth?.role ?? '';

  let cls: any = null;
  let loading = true;
  let students: any[] = [];
  let allStudents: any[] = [];
  let selectedIDs: number[] = [];
  let search = '';
  let addDialog: HTMLDialogElement;

  let existingStudentIds: Set<number> = new Set();
  $: existingStudentIds = new Set(students.map((s) => s.id));
  // reactive filtered students for add modal
  $: filtered = allStudents.filter(
    (s) => !existingStudentIds.has(s.id) && (s.name ?? s.email).toLowerCase().includes(search.toLowerCase())
  );
  let err = '';
  let newName = '';
  let renaming = false;
  let renameInput: HTMLInputElement;
  let descriptionDialog: HTMLDialogElement;
  let descriptionDraft = '';
  let savingDescription = false;
  let currentDescription = '';
  let safeCurrentDescription = '';
  let safeDescriptionPreview = '';
  let showBakalari = false;

  function sanitizeMarkdown(input: string): string {
    if (!input) return '';
    try {
      return DOMPurify.sanitize((marked.parse(input) as string) || '');
    } catch {
      return '';
    }
  }

  $: currentDescription = cls?.description ?? '';
  $: safeCurrentDescription = sanitizeMarkdown(currentDescription);
  $: safeDescriptionPreview = sanitizeMarkdown(descriptionDraft);

  function displayName(user: any): string {
    return user?.name ?? user?.email ?? t('frontend/src/routes/classes/[id]/settings/+page.svelte::unknown_user');
  }

  function getInitials(text: string): string {
    const base = (text ?? '').trim();
    if (base.length === 0) return '?';
    const parts = base.includes('@') ? base.replace(/@.+$/, '').split(/[",."_,-]+/) : base.split(/[",."_,-]+/);
    const first = parts[0]?.[0] ?? '';
    const last = parts[parts.length - 1]?.[0] ?? '';
    return (first + last).toUpperCase();
  }

  async function load() {
    loading = true; err = ''; cls = null;
    try {
      const data = await apiJSON(`/api/classes/${id}`);
      cls = data.class ?? data;
      newName = cls.name;
      students = data.students ?? [];
      if (role === 'teacher' || role === 'admin') allStudents = await apiJSON('/api/students');
    } catch (e: any) { err = e.message }
    loading = false;
  }

  // Defer initial load until auth role is known to avoid 401 flashes
  let bootstrapped = false;
  $: if (role && !bootstrapped) {
    bootstrapped = true;
    load();
  }

  function startRename() {
    renaming = true;
    tick().then(() => renameInput?.focus());
  }

  function openDescriptionModal() {
    descriptionDraft = cls?.description ?? '';
    descriptionDialog?.showModal();
  }

  function closeDescriptionModal() {
    descriptionDialog?.close();
  }

  async function saveDescription() {
    savingDescription = true;
    err = '';
    try {
      await apiFetch(`/api/classes/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ description: descriptionDraft })
      });
      if (cls) cls.description = descriptionDraft;
      classesStore.updateClass(id, { description: descriptionDraft });
      descriptionDialog?.close();
    } catch (e: any) {
      err = e.message;
    }
    savingDescription = false;
  }

  async function renameClass() {
    try {
      await apiFetch(`/api/classes/${id}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: newName }) });
      cls.name = newName;
      renaming = false;
      // Update the store
      classesStore.updateClass(id, { name: newName });
    } catch (e: any) { err = e.message }
  }

  async function deleteClass() {
    const confirmed = await confirmModal.open({
      title: t('frontend/src/routes/classes/[id]/settings/+page.svelte::delete_class'),
      body: translate('frontend/src/routes/classes/[id]/settings/+page.svelte::delete_class_confirmation', { name: cls.name }),
      confirmLabel: t('frontend/src/routes/classes/[id]/settings/+page.svelte::delete'),
      confirmClass: 'btn btn-error',
      icon: Trash2
    });
    if (!confirmed) return;

    try {
      await apiFetch(`/api/classes/${id}`, { method: 'DELETE' });
      // Remove from store before navigating away
      classesStore.removeClass(id);
      goto('/dashboard');
    } catch (e: any) { err = e.message }
  }

  async function addStudents() {
    try {
      await apiFetch(`/api/classes/${id}/students`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ student_ids: selectedIDs }) });
      selectedIDs = [];
      addDialog.close();
      await load();
    } catch (e: any) { err = e.message }
  }

  let studentToRemove: any = null;
  let confirmModal: InstanceType<typeof ConfirmModal>;

  async function promptRemoveStudent(student: any) {
    const confirmed = await confirmModal.open({
      title: t('frontend/src/routes/classes/[id]/settings/+page.svelte::remove_student_modal_title'),
      body: translate('frontend/src/routes/classes/[id]/settings/+page.svelte::remove_student_modal_body', {
        name: displayName(student)
      }),
      confirmLabel: t('frontend/src/routes/classes/[id]/settings/+page.svelte::remove_student_modal_confirm'),
      confirmClass: 'btn btn-error',
      icon: UserMinus
    });
    
    if (confirmed) {
      await removeStudent(student.id);
    }
  }

  async function removeStudent(sid: number) {
    try {
      await apiFetch(`/api/classes/${id}/students/${sid}`, { method: 'DELETE' });
      await load();
      return true;
    } catch (e: any) { err = e.message; return false; }
  }

  function openAddModal() {
    showBakalari = false;
    addDialog.showModal();
  }

  let bkUser = '';
  let bkPass = '';
  let bkAtoms: { Id: string; Name: string }[] = [];
  let bkToken: string | null = null;
  let loadingAtoms = false;

  async function fetchAtoms() {
    err = '';
    loadingAtoms = true;
    try {
      const { token } = await bkLogin(bkUser, bkPass);
      bkToken = token;
      bkAtoms = await getAtoms(token);
    } catch (e: any) { err = e.message }
    loadingAtoms = false;
  }

  async function importAtom(aid: string) {
    err = '';
    try {
      if (!bkToken) throw new Error(t('frontend/src/routes/classes/[id]/settings/+page.svelte::not_logged_in'));
      const students = await getStudents(bkToken, aid);
      const res = await apiJSON<{ added: number }>(`/api/classes/${id}/import-bakalari`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ Students: students }) });
      await load();
      alert(t('frontend/src/routes/classes/[id]/settings/+page.svelte::imported_students', { count: res.added }));
    } catch (e: any) { err = e.message }
  }
</script>

<svelte:head>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@100..900&display=swap" rel="stylesheet">
  <title>{cls?.name ? `${cls.name} | CodEdu` : 'Settings | CodEdu'}</title>
</svelte:head>

{#if loading}
  <div class="flex justify-center mt-8"><span class="loading loading-dots loading-lg"></span></div>
{:else if err}
  <div class="alert alert-error shadow-xl rounded-2xl border-none">
    <X class="size-5" />
    <span class="font-bold">{err}</span>
  </div>
{:else}
  <!-- Premium Class Header -->
  <section class="relative overflow-hidden bg-base-100 rounded-3xl border border-base-200 shadow-xl shadow-base-300/30 mb-8 p-6 sm:p-10">
    <div class="absolute inset-0 overflow-hidden rounded-3xl pointer-events-none">
      <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent"></div>
      <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl"></div>
    </div>
    
    <div class="relative flex flex-col md:flex-row items-center justify-between gap-6">
      <div class="flex-1 text-center md:text-left">
        {#if renaming}
          <div class="flex items-center gap-2 mb-2 justify-center md:justify-start">
            <input class="input input-bordered input-md bg-base-200/50 border-base-300 focus:border-primary/30 rounded-xl font-bold w-[min(24rem,80vw)]" bind:value={newName} bind:this={renameInput} />
            <button class="btn btn-primary btn-sm rounded-xl" on:click|preventDefault={renameClass}><Check class="size-4" /></button>
            <button class="btn btn-ghost btn-sm rounded-xl" on:click={() => { renaming = false; newName = cls.name; }}><X class="size-4" /></button>
          </div>
        {:else}
          <h1 class="text-3xl sm:text-4xl font-black tracking-tight mb-2">
            {cls.name} <span class="text-primary/40">/</span> {translate('frontend/src/routes/classes/[id]/settings/+page.svelte::settings_heading')}
          </h1>
        {/if}
        <p class="text-base-content/60 font-medium max-w-xl mx-auto md:mx-0">
          {translate('frontend/src/routes/classes/[id]/settings/+page.svelte::settings_description')}
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        {#if role === 'teacher' || role === 'admin'}
          {#if !renaming}
            <button class="btn btn-ghost btn-sm rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] h-10 border border-base-300 hover:bg-base-200" on:click={startRename}>
              <Pencil class="size-3.5" /> 
              {t('frontend/src/routes/classes/[id]/settings/+page.svelte::rename')}
            </button>
          {/if}
        {/if}
      </div>
    </div>
  </section>

  <section class="mt-8">
    <div class="bg-base-100 rounded-[2rem] border border-base-200 shadow-sm overflow-hidden">
      <div class="p-6 sm:p-8 space-y-6">
        <div class="flex items-center justify-between gap-4 flex-wrap">
          <div class="flex items-center gap-3">
            <div class="size-10 rounded-xl bg-primary/10 flex items-center justify-center text-primary">
              <FileText class="size-5" />
            </div>
            <h2 class="text-xl font-black tracking-tight tracking-tight">
              {t('frontend/src/routes/classes/[id]/settings/+page.svelte::class_description_heading')}
            </h2>
          </div>
          {#if role === 'teacher' || role === 'admin'}
            <button class="btn btn-ghost btn-xs rounded-lg h-8 px-3 gap-2 font-black uppercase tracking-widest text-[9px] border border-base-300 hover:bg-base-200" on:click={openDescriptionModal}>
              <Pencil class="size-3" />
              {t('frontend/src/routes/classes/[id]/settings/+page.svelte::edit_description')}
            </button>
          {/if}
        </div>

        {#if safeCurrentDescription}
          <div class="prose max-w-none assignment-description text-base-content/80 font-medium leading-relaxed bg-base-200/30 p-6 rounded-2xl border border-base-200">
            {@html safeCurrentDescription}
          </div>
        {:else}
          <div class="rounded-2xl border-2 border-dashed border-base-200 bg-base-200/20 p-8 text-center">
            <p class="text-[10px] font-black uppercase tracking-widest opacity-30">
              {t('frontend/src/routes/classes/[id]/settings/+page.svelte::class_description_empty')}
            </p>
          </div>
        {/if}
      </div>
    </div>
  </section>

  <div class="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-8">
    <div class="lg:col-span-2 space-y-8">
      <!-- Students management -->
      <div class="bg-base-100 rounded-[2rem] border border-base-200 shadow-sm overflow-hidden h-full flex flex-col">
        <div class="p-6 sm:p-8 space-y-6 flex-1">
          <div class="flex items-center justify-between gap-4 flex-wrap">
            <div class="flex items-center gap-3">
              <div class="size-10 rounded-xl bg-primary/10 flex items-center justify-center text-primary">
                <Users class="size-5" />
              </div>
              <h2 class="text-xl font-black tracking-tight">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::students')}</h2>
              <span class="px-2 py-1 rounded-lg bg-primary/10 text-primary text-[10px] font-black uppercase tracking-widest">{translate('frontend/src/routes/classes/[id]/settings/+page.svelte::student_count', { count: students.length })}</span>
            </div>
            {#if role === 'teacher' || role === 'admin'}
              <button class="btn btn-primary btn-sm rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] h-10 px-4" on:click={openAddModal}>
                <UserPlus class="size-4" /> 
                {t('frontend/src/routes/classes/[id]/settings/+page.svelte::add_students')}
              </button>
            {/if}
          </div>

          <div class="mt-4">
            {#if students.length}
              <div class="bg-base-200/30 rounded-[1.5rem] border border-base-200 overflow-hidden">
                <ul class="divide-y divide-base-200">
                  {#each students as s}
                    <li class="p-4 flex items-center gap-4 hover:bg-base-200/50 transition-colors group">
                      <div class="size-11 rounded-full overflow-hidden ring-2 ring-base-100 group-hover:ring-primary/20 transition-all flex items-center justify-center bg-base-100 text-sm font-black select-none shadow-sm">
                        {#if s.avatar}
                          <img src={s.avatar} alt={t('frontend/src/routes/classes/[id]/settings/+page.svelte::user_avatar', { name: displayName(s) })} class="w-full h-full object-cover" loading="lazy" />
                        {:else}
                          <span class="opacity-40">{getInitials(displayName(s))}</span>
                        {/if}
                      </div>
                      <div class="flex-1 min-w-0">
                        <p class="font-black text-sm tracking-tight truncate group-hover:text-primary transition-colors">{displayName(s)}</p>
                        <div class="flex items-center gap-2 mt-0.5">
                          {#if s.email}<p class="text-[10px] font-bold text-base-content/40 truncate tracking-wide">{s.email}</p>{/if}
                        </div>
                      </div>
                      {#if role === 'teacher' || role === 'admin'}
                        <button class="btn btn-ghost btn-circle btn-sm text-error/40 hover:text-error hover:bg-error/10 opacity-0 group-hover:opacity-100 transition-all" title={t('frontend/src/routes/classes/[id]/settings/+page.svelte::remove_student')} on:click={() => promptRemoveStudent(s)}>
                          <UserMinus class="size-4" />
                        </button>
                      {/if}
                    </li>
                  {/each}
                </ul>
              </div>
            {:else}
              <div class="rounded-[2rem] border-2 border-dashed border-base-200 p-12 text-center bg-base-200/20">
                <div class="size-16 rounded-full bg-base-200 flex items-center justify-center mx-auto mb-4 opacity-30">
                  <Users size={32} />
                </div>
                <p class="text-xs font-black uppercase tracking-[0.2em] opacity-30">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::no_students_yet')}</p>
                {#if role === 'teacher' || role === 'admin'}
                  <button class="btn btn-primary btn-sm mt-6 rounded-xl font-black uppercase tracking-widest text-[10px]" on:click={openAddModal}>
                    <UserPlus class="size-4" /> 
                    {t('frontend/src/routes/classes/[id]/settings/+page.svelte::add_students')}
                  </button>
                {/if}
              </div>
            {/if}
          </div>
        </div>
      </div>
    </div>

      <div class="space-y-8">
        <!-- Danger zone -->
        {#if role === 'teacher' || role === 'admin'}
        <div class="bg-base-100 rounded-[2rem] border border-error/20 shadow-sm overflow-hidden">
          <div class="p-6 sm:p-8 space-y-6">
            <div class="flex items-center gap-3">
              <div class="size-10 rounded-xl bg-error/10 flex items-center justify-center text-error">
                <Trash2 class="size-5" />
              </div>
              <h2 class="text-xl font-black tracking-tight text-error">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::danger_zone')}</h2>
            </div>
            
            <p class="text-xs font-bold text-base-content/50 leading-relaxed uppercase tracking-widest opacity-60">
              {t('frontend/src/routes/classes/[id]/settings/+page.svelte::delete_class_warning')}
            </p>
            
            <button class="btn btn-error btn-outline w-full rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] h-11 border-error/30 hover:bg-error hover:text-white transition-all shadow-lg shadow-error/10" on:click={deleteClass}>
              <Trash2 class="size-4" /> 
              {t('frontend/src/routes/classes/[id]/settings/+page.svelte::delete_class')}
            </button>
          </div>
        </div>
      {/if}
    </div>
  </div>

  <!-- Add students modal -->
  <dialog bind:this={addDialog} class="modal">
    <div class="modal-box w-11/12 max-w-2xl rounded-[2.5rem] p-8 space-y-6 shadow-2xl border border-base-200">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div class="size-10 rounded-xl bg-primary/10 flex items-center justify-center text-primary">
            <UserPlus class="size-5" />
          </div>
          <h3 class="text-xl font-black tracking-tight">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::add_students')}</h3>
        </div>
        <div class="flex items-center gap-2">
          {#if hasBakalari}
            <button 
              class="btn btn-ghost btn-sm rounded-xl font-black uppercase tracking-widest text-[10px] gap-2 border border-base-300/10" 
              on:click={() => showBakalari = !showBakalari}
            >
              <Download class="size-3" />
              {t('frontend/src/routes/classes/[id]/settings/+page.svelte::import_from_bakalari')}
            </button>
          {/if}
          <form method="dialog"><button class="btn btn-ghost btn-circle btn-sm"><X class="size-5" /></button></form>
        </div>
      </div>

      {#if showBakalari}
        <div class="bg-base-200/50 rounded-3xl p-6 border border-base-300/30 space-y-4 animate-in fade-in slide-in-from-top-4 duration-300">
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <input class="input input-bordered w-full h-11 rounded-xl bg-base-100 border-base-300 focus:border-primary/30 font-bold text-sm shadow-sm" placeholder={t('frontend/src/routes/classes/[id]/settings/+page.svelte::username')} bind:value={bkUser} />
            <input class="input input-bordered w-full h-11 rounded-xl bg-base-100 border-base-300 focus:border-primary/30 font-bold text-sm shadow-sm" type="password" placeholder={t('frontend/src/routes/classes/[id]/settings/+page.svelte::password')} bind:value={bkPass} />
          </div>
          
          <button class="btn btn-primary w-full rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] h-11 shadow-lg shadow-primary/20" on:click={fetchAtoms} disabled={loadingAtoms}>
            {#if loadingAtoms}<Loader2 class="size-4 animate-spin" />{:else}<Download class="size-4" />{/if}
            {t('frontend/src/routes/classes/[id]/settings/+page.svelte::load_classes')}
          </button>

          {#if bkAtoms.length}
            <div class="pt-2">
              <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3 px-1">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::load_classes')}</div>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                {#each bkAtoms as a}
                  <button class="rounded-xl p-3 flex items-center justify-between group font-bold bg-base-100 border border-base-200 hover:border-primary/30 hover:shadow-md transition-all text-sm text-left" on:click={() => importAtom(a.Id)}>
                    <span class="truncate pr-2">{a.Name}</span>
                    <ArrowRight class="size-3 opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all shrink-0" />
                  </button>
                {/each}
              </div>
            </div>
          {/if}
        </div>
      {/if}

      <div class="relative flex items-center">
        <SearchIcon size={14} class="absolute left-4 opacity-40" />
        <input 
          type="text" 
          class="input input-bordered w-full pl-11 rounded-xl bg-base-200/50 border-base-300 focus:border-primary/30 font-bold text-sm h-12" 
          placeholder={t('frontend/src/routes/classes/[id]/settings/+page.svelte::search_students')} 
          bind:value={search} 
        />
      </div>

      <div class="max-h-80 overflow-y-auto rounded-2xl border border-base-200 bg-base-200/20 divide-y divide-base-200 shadow-inner">
        {#if filtered.length}
          {#each filtered as s}
            <label class="flex items-center gap-4 p-4 cursor-pointer hover:bg-base-100/50 transition-colors group">
              <input type="checkbox" class="checkbox checkbox-primary rounded-lg" value={s.id} bind:group={selectedIDs} />
              <div class="size-10 rounded-full overflow-hidden ring-2 ring-base-100 group-hover:ring-primary/20 transition-all flex items-center justify-center bg-base-100 text-xs font-black shadow-sm">
                {#if s.avatar}
                  <img src={s.avatar} alt={t('frontend/src/routes/classes/[id]/settings/+page.svelte::user_avatar', { name: displayName(s) })} class="w-full h-full object-cover" loading="lazy" />
                {:else}
                  <span class="opacity-40">{getInitials(displayName(s))}</span>
                {/if}
              </div>
              <div class="flex-1 min-w-0">
                <div class="font-black text-sm tracking-tight truncate group-hover:text-primary transition-colors">{displayName(s)}</div>
                {#if s.email}<div class="text-[10px] font-bold text-base-content/40 truncate tracking-wide">{s.email}</div>{/if}
              </div>
            </label>
          {/each}
        {:else}
          <div class="p-12 text-center">
             <div class="size-12 rounded-full bg-base-200 flex items-center justify-center mx-auto mb-3 opacity-20">
               <SearchIcon size={24} />
             </div>
             <p class="text-[10px] font-black uppercase tracking-widest opacity-30">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::no_students')}</p>
          </div>
        {/if}
      </div>

      <div class="flex items-center justify-between gap-4 pt-2">
        <div class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate('frontend/src/routes/classes/[id]/settings/+page.svelte::selected_count', { count: selectedIDs.length })}</div>
        <div class="flex items-center gap-3">
          <button class="btn btn-ghost btn-sm rounded-xl font-black uppercase tracking-widest text-[10px]" on:click={() => { selectedIDs = []; }} disabled={!selectedIDs.length}>{t('frontend/src/routes/classes/[id]/settings/+page.svelte::clear')}</button>
          <button class="btn btn-primary btn-sm rounded-xl h-10 px-6 font-black uppercase tracking-widest text-[10px] shadow-lg shadow-primary/20" on:click={addStudents} disabled={!selectedIDs.length}>
            <Check class="size-4 mr-1" />
            {t('frontend/src/routes/classes/[id]/settings/+page.svelte::add_selected')}
          </button>
        </div>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>{t('frontend/src/routes/classes/[id]/settings/+page.svelte::close')}</button></form>
  </dialog>

  <!-- Class description modal -->
  <dialog bind:this={descriptionDialog} class="modal" on:close={() => { descriptionDraft = cls?.description ?? ''; savingDescription = false; }}>
    <div class="modal-box w-full max-w-4xl rounded-[2.5rem] p-8 space-y-6 shadow-2xl border border-base-200">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div class="size-10 rounded-xl bg-primary/10 flex items-center justify-center text-primary">
            <FileText class="size-5" />
          </div>
          <h3 class="text-xl font-black tracking-tight">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::description_modal_title')}</h3>
        </div>
        <form method="dialog"><button class="btn btn-ghost btn-circle btn-sm" on:click={closeDescriptionModal}><X class="size-5" /></button></form>
      </div>

      <p class="text-[10px] font-black uppercase tracking-widest opacity-40 leading-relaxed">
        {t('frontend/src/routes/classes/[id]/settings/+page.svelte::description_modal_help')}
      </p>

      <div class="grid lg:grid-cols-2 gap-8">
        <div class="space-y-4">
          <div class="flex items-center gap-2 text-[10px] font-black uppercase tracking-widest opacity-40">
            <Pencil class="size-3" /> Editor
          </div>
          <div class="rounded-2xl border border-base-200 overflow-hidden shadow-inner bg-base-200/20">
            <MarkdownEditor bind:value={descriptionDraft} placeholder={t('frontend/src/routes/classes/[id]/settings/+page.svelte::description_placeholder')} />
          </div>
        </div>

        <div class="space-y-4">
          <div class="flex items-center gap-2 text-[10px] font-black uppercase tracking-widest opacity-40">
            <FileText class="size-3" /> {t('frontend/src/routes/classes/[id]/settings/+page.svelte::description_preview_title')}
          </div>
          <div class="h-[400px] overflow-y-auto rounded-2xl border border-base-200 bg-base-100 p-6 shadow-inner">
            {#if safeDescriptionPreview}
              <div class="prose prose-sm max-w-none assignment-description text-base-content/80 font-medium">
                {@html safeDescriptionPreview}
              </div>
            {:else}
              <div class="h-full flex items-center justify-center text-center opacity-20">
                <p class="text-[10px] font-black uppercase tracking-widest leading-relaxed">
                  {t('frontend/src/routes/classes/[id]/settings/+page.svelte::description_preview_empty')}
                </p>
              </div>
            {/if}
          </div>
        </div>
      </div>
      <div class="flex items-center gap-3 pt-2">
        <form method="dialog" class="flex-1"><button class="btn btn-ghost w-full rounded-2xl font-black uppercase tracking-widest text-[10px]" on:click={closeDescriptionModal}>{t('frontend/src/routes/classes/[id]/settings/+page.svelte::cancel')}</button></form>
        <button class="btn btn-primary flex-1 rounded-2xl font-black uppercase tracking-widest text-[10px] h-12 shadow-lg shadow-primary/20" on:click|preventDefault={saveDescription} disabled={savingDescription}>
          {#if savingDescription}
            <Loader2 class="size-4 animate-spin mr-2" />
          {:else}
            <Check class="size-4 mr-2" />
          {/if}
          <span>{t('frontend/src/routes/classes/[id]/settings/+page.svelte::description_save')}</span>
        </button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>{t('frontend/src/routes/classes/[id]/settings/+page.svelte::close')}</button></form>
  </dialog>

  <ConfirmModal bind:this={confirmModal} />

{/if}

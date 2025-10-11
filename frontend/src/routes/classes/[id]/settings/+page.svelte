<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { auth } from '$lib/auth';
  import { apiFetch, apiJSON } from '$lib/api';
  import { login as bkLogin, getAtoms, getStudents, hasBakalari } from '$lib/bakalari';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { classesStore } from '$lib/stores/classes';
  import { BookOpen, Pencil, Trash2, UserPlus, UserMinus, Search as SearchIcon, Loader2, Check, X, Users, Download } from 'lucide-svelte';
  import { t, translator } from '$lib/i18n';

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
  let deleteDialog: HTMLDialogElement;
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
    try {
      await apiFetch(`/api/classes/${id}`, { method: 'DELETE' });
      // Remove from store before navigating away
      classesStore.removeClass(id);
      goto('/my-classes');
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

  async function removeStudent(sid: number) {
    try {
      await apiFetch(`/api/classes/${id}/students/${sid}`, { method: 'DELETE' });
      await load();
    } catch (e: any) { err = e.message }
  }

  function openAddModal() {
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

{#if loading}
  <div class="w-full grid gap-6">
    <div class="h-28 rounded-2xl bg-base-200/60 animate-pulse"></div>
    <div class="h-64 rounded-xl bg-base-200/60 animate-pulse"></div>
  </div>
{:else if err}
  <div class="alert alert-error shadow">
    <X class="size-5" />
    <span>{err}</span>
  </div>
{:else}
  <section class="relative overflow-hidden rounded-2xl bg-gradient-to-br from-primary/15 via-base-200 to-base-100 border border-base-300/60 shadow-xl">
    <div class="absolute -inset-24 opacity-40 blur-3xl pointer-events-none" aria-hidden="true">
      <div class="size-full bg-[conic-gradient(var(--fallback-p,oklch(var(--p))),transparent_50%)]"></div>
    </div>
    <div class="relative p-6 md:p-8 flex items-start justify-between gap-4 flex-wrap">
      <div class="flex items-center gap-4">
        <div class="size-14 md:size-16 rounded-xl bg-primary/20 ring-1 ring-primary/30 grid place-items-center">
          <BookOpen class="size-7 md:size-8 text-primary" />
        </div>
        <div>
          <div class="flex items-center gap-3">
            {#if renaming}
              <input class="input input-bordered input-md md:input-lg w-[min(28rem,90vw)]" bind:value={newName} bind:this={renameInput} />
              <button class="btn btn-primary btn-sm" on:click|preventDefault={renameClass}><Check class="size-4" /></button>
              <button class="btn btn-ghost btn-sm" on:click={() => { renaming = false; newName = cls.name; }}><X class="size-4" /></button>
            {:else}
              <h1 class="text-2xl md:text-3xl font-semibold tracking-tight">{cls.name}</h1>
            {/if}
          </div>
          <p class="text-base-content/60 text-sm mt-1 flex items-center gap-2"><Users class="size-4" /> {translate('frontend/src/routes/classes/[id]/settings/+page.svelte::student_count', { count: students.length })}</p>
        </div>
      </div>
      {#if role === 'teacher' || role === 'admin'}
        <div class="flex items-center gap-2 ml-auto">
          {#if !renaming}
            <button class="btn btn-ghost btn-sm" on:click={startRename}><Pencil class="size-4" /> {t('frontend/src/routes/classes/[id]/settings/+page.svelte::rename')}</button>
          {/if}
        </div>
      {/if}
    </div>
  </section>

  <div class="mt-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
    <div class="lg:col-span-2 space-y-6">
      <!-- Students management -->
      <div class="card bg-base-100/80 backdrop-blur border border-base-300/60 shadow-md">
        <div class="card-body">
          <div class="flex items-center justify-between gap-4 flex-wrap">
            <h2 class="card-title flex items-center gap-2"><Users class="size-5" /> {t('frontend/src/routes/classes/[id]/settings/+page.svelte::students')}</h2>
            {#if role === 'teacher' || role === 'admin'}
              <button class="btn btn-primary btn-sm" on:click={openAddModal}><UserPlus class="size-4" /> {t('frontend/src/routes/classes/[id]/settings/+page.svelte::add_students')}</button>
            {/if}
          </div>

          <div class="mt-4">
            {#if students.length}
              <ul class="divide-y divide-base-300/60">
                {#each students as s}
                  <li class="py-3 flex items-center gap-3">
                    <div class="size-9 rounded-full overflow-hidden ring-1 ring-base-300/70 flex items-center justify-center bg-base-200 text-sm font-semibold select-none">
                      {#if s.avatar}
                        <img src={s.avatar} alt={t('frontend/src/routes/classes/[id]/settings/+page.svelte::user_avatar', { name: displayName(s) })} class="w-full h-full object-cover" loading="lazy" />
                      {:else}
                        {getInitials(displayName(s))}
                      {/if}
                    </div>
                    <div class="flex-1 min-w-0">
                      <p class="truncate">{displayName(s)}</p>
                      {#if s.email}<p class="text-xs text-base-content/60 truncate">{s.email}</p>{/if}
                      <p class="text-xs text-base-content/60 truncate">ID: {s.id}</p>
                    </div>
                    {#if role === 'teacher' || role === 'admin'}
                      <button class="btn btn-ghost btn-xs text-error ml-auto" title={t('frontend/src/routes/classes/[id]/settings/+page.svelte::remove_student')} on:click={() => removeStudent(s.id)}>
                        <UserMinus class="size-4" />
                      </button>
                    {/if}
                  </li>
                {/each}
              </ul>
            {:else}
              <div class="rounded-xl border border-dashed border-base-300/80 p-8 text-center">
                <p class="text-base-content/70">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::no_students_yet')}</p>
                {#if role === 'teacher' || role === 'admin'}
                  <button class="btn btn-primary btn-sm mt-3" on:click={openAddModal}><UserPlus class="size-4" /> {t('frontend/src/routes/classes/[id]/settings/+page.svelte::add_students')}</button>
                {/if}
              </div>
            {/if}
          </div>
        </div>
      </div>
    </div>

    <!-- Integrations -->
    <div class="space-y-6">
      {#if hasBakalari}
        <div class="card bg-base-100/80 backdrop-blur border border-base-300/60 shadow-md">
          <div class="card-body">
            <h2 class="card-title flex items-center gap-2"><Download class="size-5" /> {t('frontend/src/routes/classes/[id]/settings/+page.svelte::import_from_bakalari')}</h2>
            <div class="form-control mt-2 space-y-2">
              <input class="input input-bordered w-full" placeholder={t('frontend/src/routes/classes/[id]/settings/+page.svelte::username')} bind:value={bkUser} />
              <input class="input input-bordered w-full" type="password" placeholder={t('frontend/src/routes/classes/[id]/settings/+page.svelte::password')} bind:value={bkPass} />
              <button class="btn btn-outline w-full" on:click={fetchAtoms} disabled={loadingAtoms}>
                {#if loadingAtoms}<Loader2 class="size-4 animate-spin" />{:else}<Download class="size-4" />{/if}
                <span class="ml-1">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::load_classes')}</span>
              </button>
            </div>
            {#if bkAtoms.length}
              <ul class="menu mt-3">
                {#each bkAtoms as a}
                  <li>
                    <button class="btn btn-sm btn-outline w-full justify-between" on:click={() => importAtom(a.Id)}>{a.Name}</button>
                  </li>
                {/each}
              </ul>
            {/if}
          </div>
        </div>
      {/if}

      <!-- Danger zone -->
      {#if role === 'teacher' || role === 'admin'}
        <div class="card bg-base-100/80 backdrop-blur border border-error/30 shadow-md">
          <div class="card-body">
            <h2 class="card-title text-error flex items-center gap-2"><Trash2 class="size-5" /> {t('frontend/src/routes/classes/[id]/settings/+page.svelte::danger_zone')}</h2>
            <p class="text-sm text-base-content/70">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::delete_class_warning')}</p>
            <div>
              <button class="btn btn-error btn-outline" on:click={() => deleteDialog.showModal()}><Trash2 class="size-4" /> {t('frontend/src/routes/classes/[id]/settings/+page.svelte::delete_class')}</button>
            </div>
          </div>
        </div>
      {/if}
    </div>
  </div>

  <!-- Add students modal -->
  <dialog bind:this={addDialog} class="modal">
    <div class="modal-box w-11/12 max-w-2xl">
      <div class="flex items-center justify-between mb-3">
        <h3 class="font-bold text-lg flex items-center gap-2"><UserPlus class="size-5" /> {t('frontend/src/routes/classes/[id]/settings/+page.svelte::add_students')}</h3>
        <form method="dialog"><button class="btn btn-ghost btn-sm"><X class="size-4" /></button></form>
      </div>
      <label class="input input-bordered flex items-center gap-2 mb-3">
        <SearchIcon class="size-4 opacity-70" />
        <input class="grow" placeholder={t('frontend/src/routes/classes/[id]/settings/+page.svelte::search_students')} bind:value={search} />
      </label>
      <div class="max-h-72 overflow-y-auto rounded-lg border border-base-300/60 divide-y divide-base-300/60">
        {#if filtered.length}
          {#each filtered as s}
            <label class="flex items-center gap-3 p-3 cursor-pointer">
              <input type="checkbox" class="checkbox checkbox-sm" value={s.id} bind:group={selectedIDs} />
              <div class="size-8 rounded-full overflow-hidden ring-[1.5px] ring-base-300/70 flex items-center justify-center bg-base-200 text-xs font-semibold">
                {#if s.avatar}
                  <img src={s.avatar} alt={t('frontend/src/routes/classes/[id]/settings/+page.svelte::user_avatar', { name: displayName(s) })} class="w-full h-full object-cover" loading="lazy" />
                {:else}
                  {getInitials(displayName(s))}
                {/if}
              </div>
              <div class="flex-1 min-w-0">
                <div class="truncate">{displayName(s)}</div>
                {#if s.email}<div class="text-xs text-base-content/60 truncate">{s.email}</div>{/if}
                <div class="text-xs text-base-content/50 truncate">ID: {s.id}</div>
              </div>
            </label>
          {/each}
        {:else}
          <div class="p-6 text-center text-base-content/70">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::no_students')}</div>
        {/if}
      </div>
      <div class="modal-action items-center justify-between">
        <div class="text-sm text-base-content/70">{translate('frontend/src/routes/classes/[id]/settings/+page.svelte::selected_count', { count: selectedIDs.length })}</div>
        <div class="flex items-center gap-2">
          <button class="btn btn-ghost" on:click={() => { selectedIDs = []; }} disabled={!selectedIDs.length}>{t('frontend/src/routes/classes/[id]/settings/+page.svelte::clear')}</button>
          <button class="btn btn-primary" on:click={addStudents} disabled={!selectedIDs.length}>{t('frontend/src/routes/classes/[id]/settings/+page.svelte::add_selected')}</button>
        </div>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>{t('frontend/src/routes/classes/[id]/settings/+page.svelte::close')}</button></form>
  </dialog>

  <!-- Delete confirm modal -->
  <dialog bind:this={deleteDialog} class="modal">
    <div class="modal-box">
      <h3 class="font-bold text-lg flex items-center gap-2 text-error"><Trash2 class="size-5" /> {t('frontend/src/routes/classes/[id]/settings/+page.svelte::delete_class')}</h3>
      <p class="py-3">{translate('frontend/src/routes/classes/[id]/settings/+page.svelte::delete_class_confirmation', { name: cls.name })}</p>
      <div class="modal-action">
        <form method="dialog" class="mr-auto"><button class="btn">{t('frontend/src/routes/classes/[id]/settings/+page.svelte::cancel')}</button></form>
        <button class="btn btn-error" on:click={deleteClass}>{t('frontend/src/routes/classes/[id]/settings/+page.svelte::delete')}</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>{t('frontend/src/routes/classes/[id]/settings/+page.svelte::close')}</button></form>
  </dialog>
{/if}

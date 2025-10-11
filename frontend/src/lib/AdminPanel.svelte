<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch, apiJSON } from '$lib/api';
  import { sha256 } from '$lib/hash';
  import { formatDate, formatDateTime } from "$lib/date";
  import { classesStore } from '$lib/stores/classes';
  import {
    Users2, GraduationCap, School, BookOpen, Plus, Trash2, RefreshCw,
    Shield, Search, Edit, ArrowRightLeft, Check
  } from 'lucide-svelte';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import PromptModal from '$lib/components/PromptModal.svelte';
  import { t, translator } from '$lib/i18n';

  let translate;
  $: translate = $translator;

  // Tabs
  type Tab = 'overview' | 'teachers' | 'users' | 'classes' | 'assignments'
  let tab: Tab = 'overview';

  // Data models
  type User = { id: string; email: string; name?: string | null; role: string; created_at: string };
  type Class = { id: string; name: string; teacher_id: string; created_at: string };
  type Assignment = {
    id: string; title: string; description: string; class_id: string;
    deadline: string; max_points: number; grading_policy: string; published: boolean;
    show_traceback: boolean; manual_review: boolean; created_at: string
  };

  // Collections
  let users: User[] = [];
  let classes: Class[] = [];
  let assignments: Assignment[] = [];

  // Filters/search
  let userQuery = '';
  let classQuery = '';
  let assignmentQuery = '';
  let assignmentFilter: 'all' | 'published' | 'unpublished' = 'all';

  // Derived
  $: teachers = users.filter(u => u.role === 'teacher');
  $: admins = users.filter(u => u.role === 'admin');
  $: students = users.filter(u => u.role === 'student');
  $: teacherIdToClassCount = classes.reduce<Record<string, number>>((m, c) => { m[c.teacher_id] = (m[c.teacher_id] || 0) + 1; return m; }, {});
  $: filteredUsers = users.filter(u => (u.email + ' ' + (u.name ?? '')).toLowerCase().includes(userQuery.toLowerCase()));
  $: filteredClasses = classes.filter(c => (c.name + ' ' + c.id).toLowerCase().includes(classQuery.toLowerCase()))
  $: filteredAssignments = assignments
    .filter(a => (a.title + ' ' + a.description).toLowerCase().includes(assignmentQuery.toLowerCase()))
    .filter(a => assignmentFilter === 'all' ? true : assignmentFilter === 'published' ? a.published : !a.published);

  // Loading states
  let loadingUsers = false, loadingClasses = false, loadingAssignments = false;
  let ok = '', err = '';
  let confirmModal: InstanceType<typeof ConfirmModal>;
  let promptModal: InstanceType<typeof PromptModal>;

  async function loadUsers() {
    loadingUsers = true; err = '';
    try { users = await apiJSON('/api/users'); } catch (e: any) { err = e.message; }
    loadingUsers = false;
  }
  async function loadClasses() {
    loadingClasses = true; err = '';
    try {
      classes = await apiJSON('/api/classes/all');
      // Update the store with all classes for admin view
      classesStore.setClasses(classes);
    } catch (e: any) { err = e.message; }
    loadingClasses = false;
  }
  async function loadAssignments() {
    loadingAssignments = true; err = '';
    try { assignments = await apiJSON('/api/assignments'); } catch (e: any) { err = e.message; }
    loadingAssignments = false;
  }

  onMount(() => { loadUsers(); loadClasses(); loadAssignments(); });

  // ───────────────────────────
  // Teacher management
  // ───────────────────────────
  let teacherEmail = '', teacherPassword = '';
  async function addTeacher() {
    err = ok = '';
    const r = await apiFetch('/api/teachers', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: teacherEmail, password: await sha256(teacherPassword) })
    });
    if (r.status === 201) {
      ok = t('frontend/src/lib/AdminPanel.svelte::teacher_created_success');
      teacherEmail = teacherPassword = '';
      await loadUsers();
    } else {
      const j = await r.json().catch(() => ({}));
      err = j.error || t('frontend/src/lib/AdminPanel.svelte::failed_to_create_teacher');
    }
  }

  // ───────────────────────────
  // User management
  // ───────────────────────────
  const roles = ['student', 'teacher', 'admin'];
  async function changeRole(id: string, role: string) {
    try {
      await apiFetch(`/api/users/${id}/role`, {
        method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ role })
      });
      ok = t('frontend/src/lib/AdminPanel.svelte::role_updated_success');
      await loadUsers();
    } catch (e: any) { err = e.message; }
  }
  async function deleteUser(id: string) {
    const confirmed = await confirmModal.open({
      title: t('frontend/src/lib/AdminPanel.svelte::delete_user_modal_title'),
      body: t('frontend/src/lib/AdminPanel.svelte::delete_user_modal_body'),
      confirmLabel: t('frontend/src/lib/AdminPanel.svelte::delete_user_confirm_label'),
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    });
    if (!confirmed) return;
    try { await apiFetch(`/api/users/${id}`, { method: 'DELETE' }); ok = t('frontend/src/lib/AdminPanel.svelte::user_deleted_success'); await loadUsers(); } catch (e: any) { err = e.message; }
  }
  function exportUsersCSV() {
    const rows = [[t('frontend/src/lib/AdminPanel.svelte::user_csv_header_id'), t('frontend/src/lib/AdminPanel.svelte::user_csv_header_email'), t('frontend/src/lib/AdminPanel.svelte::user_csv_header_name'), t('frontend/src/lib/AdminPanel.svelte::user_csv_header_role'), t('frontend/src/lib/AdminPanel.svelte::user_csv_header_created')]].concat(users.map(u => [String(u.id), u.email, u.name ?? '', u.role, formatDate(u.created_at)]));
    const csv = rows.map(r => r.map(v => '"' + v.replaceAll('"','""') + '"').join(',')).join('\n');
    const a = document.createElement('a');
    a.href = 'data:text/csv;charset=utf-8,' + encodeURIComponent(csv);
    a.download = 'users.csv';
    a.click();
  }

  // ───────────────────────────
  // Class management
  // ───────────────────────────
  let showCreateClass = false;
  let newClassName = '';
  let newClassTeacherId: string | null = null;
  async function createClass() {
    if (!newClassName.trim() || !newClassTeacherId) return;
    try {
      const created = await apiJSON('/api/admin/classes', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newClassName.trim(), teacher_id: newClassTeacherId })
      });
      ok = t('frontend/src/lib/AdminPanel.svelte::class_created_success', { id: created.id });
      showCreateClass = false; newClassName = ''; newClassTeacherId = null;
      // Update both local state and the store
      await loadClasses();
      classesStore.addClass(created);
    } catch (e: any) { err = e.message; }
  }
  async function renameClass(id: string) {
    const current = classes.find(c => c.id === id)?.name ?? '';
    const name = await promptModal?.open({
      title: t('frontend/src/lib/AdminPanel.svelte::rename_class_modal_title'),
      label: t('frontend/src/lib/AdminPanel.svelte::class_name_modal_label'),
      initialValue: current,
      confirmLabel: t('frontend/src/lib/AdminPanel.svelte::save_button'),
      icon: 'fa-solid fa-school text-primary',
      validate: (value) => value.trim() ? null : t('frontend/src/lib/AdminPanel.svelte::class_name_required_validation'),
      transform: (value) => value.trim()
    });
    if (!name || name === current) return;
    try {
      await apiFetch(`/api/classes/${id}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name }) });
      ok = t('frontend/src/lib/AdminPanel.svelte::class_renamed_success');
      await loadClasses();
      // Update the store
      classesStore.updateClass(id, { name });
    } catch (e: any) { err = e.message; }
  }
  async function deleteClassAction(id: string) {
    const confirmed = await confirmModal.open({
      title: t('frontend/src/lib/AdminPanel.svelte::delete_class_modal_title'),
      body: t('frontend/src/lib/AdminPanel.svelte::delete_class_modal_body'),
      confirmLabel: t('frontend/src/lib/AdminPanel.svelte::delete_class_confirm_label'),
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    });
    if (!confirmed) return;
    try {
      await apiFetch(`/api/classes/${id}`, { method: 'DELETE' });
      ok = t('frontend/src/lib/AdminPanel.svelte::class_deleted_success');
      await loadClasses();
      // Update the store
      classesStore.removeClass(id);
    } catch (e: any) { err = e.message; }
  }
  let transferTarget: { id: string, to: string | null } | null = null;
  async function transferClass() {
    if (!transferTarget || !transferTarget.to) return;
    try {
      const classId = transferTarget.id;
      const teacherId = transferTarget.to;
      await apiFetch(`/api/admin/classes/${classId}/transfer`, {
        method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ teacher_id: teacherId })
      });
      ok = t('frontend/src/lib/AdminPanel.svelte::class_ownership_transferred_success');
      transferTarget = null;
      await loadClasses();
      // Update the store
      classesStore.updateClass(classId, { teacher_id: teacherId });
    } catch (e: any) { err = e.message; }
  }

  // ───────────────────────────
  // Assignment management
  // ───────────────────────────
  async function publishAssignment(aid: string) {
    try { await apiFetch(`/api/assignments/${aid}/publish`, { method: 'PUT' }); ok = t('frontend/src/lib/AdminPanel.svelte::assignment_published_success'); await loadAssignments(); } catch (e: any) { err = e.message; }
  }
  async function deleteAssignment(aid: string) {
    const confirmed = await confirmModal.open({
      title: t('frontend/src/lib/AdminPanel.svelte::delete_assignment_modal_title'),
      body: t('frontend/src/lib/AdminPanel.svelte::delete_assignment_modal_body'),
      confirmLabel: t('frontend/src/lib/AdminPanel.svelte::delete_assignment_confirm_label'),
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    });
    if (!confirmed) return;
    try { await apiFetch(`/api/assignments/${aid}`, { method: 'DELETE' }); ok = t('frontend/src/lib/AdminPanel.svelte::assignment_deleted_success'); await loadAssignments(); } catch (e: any) { err = e.message; }
  }
</script>

<h1 class="text-2xl font-bold mb-6 flex items-center gap-2"><Shield class="w-6 h-6" aria-hidden="true" /> {t('frontend/src/lib/AdminPanel.svelte::admin')}</h1>

<div class="tabs tabs-boxed mb-6">
  <a role="tab" class="tab {tab==='overview' ? 'tab-active' : ''}" on:click={() => tab='overview'}>{t('frontend/src/lib/AdminPanel.svelte::overview_tab')}</a>
  <a role="tab" class="tab {tab==='teachers' ? 'tab-active' : ''}" on:click={() => tab='teachers'}>{t('frontend/src/lib/AdminPanel.svelte::teachers_tab')}</a>
  <a role="tab" class="tab {tab==='users' ? 'tab-active' : ''}" on:click={() => tab='users'}>{t('frontend/src/lib/AdminPanel.svelte::users_tab')}</a>
  <a role="tab" class="tab {tab==='classes' ? 'tab-active' : ''}" on:click={() => tab='classes'}>{t('frontend/src/lib/AdminPanel.svelte::classes_tab')}</a>
  <a role="tab" class="tab {tab==='assignments' ? 'tab-active' : ''}" on:click={() => tab='assignments'}>{t('frontend/src/lib/AdminPanel.svelte::assignments_tab')}</a>
</div>

{#if ok}
  <div class="alert alert-success mb-4"><Check class="w-4 h-4" aria-hidden="true" /> {ok}</div>
{/if}
{#if err}
  <div class="alert alert-error mb-4">{err}</div>
{/if}

{#if tab === 'overview'}
  <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <h2 class="card-title mb-2">{t('frontend/src/lib/AdminPanel.svelte::platform_stats_title')}</h2>
        <div class="stats stats-vertical sm:stats-horizontal shadow">
          <div class="stat"><div class="stat-figure"><Users2 class="w-5 h-5" /></div><div class="stat-title">{t('frontend/src/lib/AdminPanel.svelte::users_stat_title')}</div><div class="stat-value">{users.length}</div></div>
          <div class="stat"><div class="stat-figure"><GraduationCap class="w-5 h-5" /></div><div class="stat-title">{t('frontend/src/lib/AdminPanel.svelte::teachers_stat_title')}</div><div class="stat-value">{teachers.length}</div></div>
          <div class="stat"><div class="stat-figure"><School class="w-5 h-5" /></div><div class="stat-title">{t('frontend/src/lib/AdminPanel.svelte::classes_stat_title')}</div><div class="stat-value">{classes.length}</div></div>
          <div class="stat"><div class="stat-figure"><BookOpen class="w-5 h-5" /></div><div class="stat-title">{t('frontend/src/lib/AdminPanel.svelte::assignments_stat_title')}</div><div class="stat-value">{assignments.length}</div></div>
        </div>
      </div>
    </div>
    <div class="card bg-base-100 shadow">
      <div class="card-body gap-3">
        <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::quick_actions_title')}</h2>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-sm" on:click={() => { tab='teachers'; }}>{t('frontend/src/lib/AdminPanel.svelte::add_teacher_button')}</button>
          <button class="btn btn-sm" on:click={() => { tab='classes'; showCreateClass = true; }}>{t('frontend/src/lib/AdminPanel.svelte::create_class_button')}</button>
          <a class="btn btn-sm" href="/dashboard">{t('frontend/src/lib/AdminPanel.svelte::go_to_dashboard_button')}</a>
        </div>
      </div>
    </div>
    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::unpublished_assignments_title')}</h2>
        <ul class="space-y-2 max-h-60 overflow-auto">
          {#each assignments.filter(a => !a.published).slice(0, 8) as a}
            <li class="flex items-center justify-between gap-3">
              <a class="link" href={`/assignments/${a.id}`}>{a.title}</a>
              <button class="btn btn-ghost btn-xs" on:click={() => publishAssignment(a.id)}>{t('frontend/src/lib/AdminPanel.svelte::publish_button')}</button>
            </li>
          {/each}
          {#if !assignments.filter(a => !a.published).length}
            <li class="opacity-70 text-sm">{t('frontend/src/lib/AdminPanel.svelte::all_assignments_published')}</li>
          {/if}
        </ul>
      </div>
    </div>
  </div>
{/if}

<ConfirmModal bind:this={confirmModal} />
<PromptModal bind:this={promptModal} />

{#if tab === 'teachers'}
  <div class="grid grid-cols-1 xl:grid-cols-2 gap-6">
    <div class="card bg-base-100 shadow">
      <div class="card-body space-y-4">
        <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::add_teacher_card_title')}</h2>
        <form on:submit|preventDefault={addTeacher} class="space-y-3">
          <input type="email" bind:value={teacherEmail} placeholder={t('frontend/src/lib/AdminPanel.svelte::email_label')} required class="input input-bordered w-full" />
          <input type="password" bind:value={teacherPassword} placeholder={t('frontend/src/lib/AdminPanel.svelte::password_label')} required class="input input-bordered w-full" />
          <button class="btn btn-primary">{t('frontend/src/lib/AdminPanel.svelte::add_button')}</button>
        </form>
      </div>
    </div>
    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <div class="flex items-center justify-between mb-3">
          <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::teachers_card_title')}</h2>
          <button class="btn btn-ghost btn-sm" on:click={loadUsers}><RefreshCw class="w-4 h-4" /></button>
        </div>
        <div class="overflow-x-auto">
          <table class="table table-zebra">
            <thead><tr><th>{t('frontend/src/lib/AdminPanel.svelte::id_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::email_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::name_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::classes_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::created_table_header')}</th></tr></thead>
            <tbody>
              {#each teachers as t_user}
                <tr>
                  <td>{t_user.id}</td>
                  <td>{t_user.email}</td>
                  <td>{t_user.name ?? ''}</td>
                  <td>{teacherIdToClassCount[t_user.id] ?? 0}</td>
                  <td>{formatDate(t_user.created_at)}</td>
                </tr>
              {/each}
              {#if !teachers.length}
                <tr><td colspan="5"><i>{t('frontend/src/lib/AdminPanel.svelte::no_teachers_message')}</i></td></tr>
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
{/if}

{#if tab === 'users'}
  <div class="card bg-base-100 shadow">
    <div class="card-body">
      <div class="flex items-center gap-2 justify-between flex-wrap mb-3">
        <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::users_card_title')}</h2>
        <div class="flex items-center gap-2">
          <label class="input input-bordered input-sm flex items-center gap-2">
            <Search class="w-4 h-4" aria-hidden="true" />
            <input class="grow" placeholder={t('frontend/src/lib/AdminPanel.svelte::search_users_placeholder')} bind:value={userQuery} />
          </label>
          <button class="btn btn-sm" on:click={exportUsersCSV}>{t('frontend/src/lib/AdminPanel.svelte::export_csv_button')}</button>
          <button class="btn btn-ghost btn-sm" on:click={loadUsers}><RefreshCw class="w-4 h-4" /></button>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="table table-zebra">
          <thead><tr><th>{t('frontend/src/lib/AdminPanel.svelte::id_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::email_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::name_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::role_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::created_table_header')}</th><th></th></tr></thead>
          <tbody>
            {#each filteredUsers as u}
              <tr>
                <td>{u.id}</td>
                <td>{u.email}</td>
                <td class="max-w-[18rem] truncate">{u.name ?? ''}</td>
                <td>
                  <select bind:value={u.role} on:change={(e)=>changeRole(u.id, (e.target as HTMLSelectElement).value)} class="select select-bordered select-sm">
                    {#each roles as r}<option>{r}</option>{/each}
                  </select>
                </td>
                <td>{formatDate(u.created_at)}</td>
                <td class="text-right">
                  <button class="btn btn-ghost btn-xs text-error" on:click={()=>deleteUser(u.id)}><Trash2 class="w-4 h-4" /></button>
                </td>
              </tr>
            {/each}
            {#if !filteredUsers.length}
              <tr><td colspan="6"><i>{t('frontend/src/lib/AdminPanel.svelte::no_users_message')}</i></td></tr>
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  </div>
{/if}

{#if tab === 'classes'}
  <div class="card bg-base-100 shadow">
    <div class="card-body">
      <div class="flex items-center gap-2 justify-between flex-wrap mb-3">
        <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::classes_card_title')}</h2>
        <div class="flex items-center gap-2">
          <label class="input input-bordered input-sm flex items-center gap-2">
            <Search class="w-4 h-4" aria-hidden="true" />
            <input class="grow" placeholder={t('frontend/src/lib/AdminPanel.svelte::search_classes_placeholder')} bind:value={classQuery} />
          </label>
          <button class="btn btn-sm" on:click={() => showCreateClass = true}><Plus class="w-4 h-4" /> {t('frontend/src/lib/AdminPanel.svelte::new_button')}</button>
          <button class="btn btn-ghost btn-sm" on:click={loadClasses}><RefreshCw class="w-4 h-4" /></button>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="table table-zebra">
          <thead><tr><th>{t('frontend/src/lib/AdminPanel.svelte::id_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::name_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::teacher_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::created_table_header')}</th><th></th></tr></thead>
          <tbody>
            {#each filteredClasses as c}
              <tr>
                <td>{c.id}</td>
                <td><a href={`/classes/${c.id}`} class="link link-primary">{c.name}</a></td>
                <td>{c.teacher_id}</td>
                <td>{formatDate(c.created_at)}</td>
                <td class="text-right whitespace-nowrap">
                  <button class="btn btn-ghost btn-xs" on:click={()=>renameClass(c.id)}><Edit class="w-4 h-4" /></button>
                  <button class="btn btn-ghost btn-xs" on:click={()=>transferTarget={ id: c.id, to: null }}><ArrowRightLeft class="w-4 h-4" /></button>
                  <button class="btn btn-ghost btn-xs text-error" on:click={()=>deleteClassAction(c.id)}><Trash2 class="w-4 h-4" /></button>
                </td>
              </tr>
            {/each}
            {#if !filteredClasses.length}
              <tr><td colspan="5"><i>{t('frontend/src/lib/AdminPanel.svelte::no_classes_message')}</i></td></tr>
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  </div>

  {#if showCreateClass}
    <dialog open class="modal">
      <div class="modal-box space-y-4">
        <h3 class="font-semibold">{t('frontend/src/lib/AdminPanel.svelte::create_class_modal_title')}</h3>
        <input class="input input-bordered w-full" placeholder={t('frontend/src/lib/AdminPanel.svelte::class_name_placeholder')} bind:value={newClassName} />
        <select class="select select-bordered w-full" bind:value={newClassTeacherId}>
          <option value={null} disabled selected>{t('frontend/src/lib/AdminPanel.svelte::select_teacher_option')}</option>
          {#each teachers as t_teacher}
            <option value={t_teacher.id}>{t_teacher.name ?? t_teacher.email} (#{t_teacher.id})</option>
          {/each}
        </select>
        <div class="modal-action">
          <button class="btn" on:click={createClass}><Plus class="w-4 h-4" /> {t('frontend/src/lib/AdminPanel.svelte::create_button')}</button>
          <button class="btn btn-ghost" on:click={() => { showCreateClass = false; }}>{t('frontend/src/lib/AdminPanel.svelte::cancel_button')}</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" on:click={() => showCreateClass = false}><button>close</button></form>
    </dialog>
  {/if}

  {#if transferTarget}
    <dialog open class="modal">
      <div class="modal-box space-y-4">
        <h3 class="font-semibold">{t('frontend/src/lib/AdminPanel.svelte::transfer_ownership_modal_title')}</h3>
        <p>{t('frontend/src/lib/AdminPanel.svelte::transfer_ownership_modal_body', { classId: transferTarget.id })}</p>
        <select class="select select-bordered w-full" bind:value={transferTarget.to}>
          <option value={null} disabled selected>{t('frontend/src/lib/AdminPanel.svelte::select_teacher_option')}</option>
          {#each teachers as t_teacher}
            <option value={t_teacher.id}>{t_teacher.name ?? t_teacher.email} (#{t_teacher.id})</option>
          {/each}
        </select>
        <div class="modal-action">
          <button class="btn" on:click={transferClass}><ArrowRightLeft class="w-4 h-4" /> {t('frontend/src/lib/AdminPanel.svelte::transfer_button')}</button>
          <button class="btn btn-ghost" on:click={() => transferTarget = null}>{t('frontend/src/lib/AdminPanel.svelte::cancel_button')}</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" on:click={() => transferTarget = null}><button>close</button></form>
    </dialog>
  {/if}
{/if}

{#if tab === 'assignments'}
  <div class="card bg-base-100 shadow">
    <div class="card-body">
      <div class="flex items-center gap-2 justify-between flex-wrap mb-3">
        <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::assignments_card_title')}</h2>
        <div class="flex items-center gap-2">
          <label class="input input-bordered input-sm flex items-center gap-2">
            <Search class="w-4 h-4" aria-hidden="true" />
            <input class="grow" placeholder={t('frontend/src/lib/AdminPanel.svelte::search_assignments_placeholder')} bind:value={assignmentQuery} />
          </label>
          <select class="select select-sm select-bordered" bind:value={assignmentFilter}>
            <option value="all">{t('frontend/src/lib/AdminPanel.svelte::all_filter_option')}</option>
            <option value="published">{t('frontend/src/lib/AdminPanel.svelte::published_filter_option')}</option>
            <option value="unpublished">{t('frontend/src/lib/AdminPanel.svelte::unpublished_filter_option')}</option>
          </select>
          <button class="btn btn-ghost btn-sm" on:click={loadAssignments}><RefreshCw class="w-4 h-4" /></button>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="table table-zebra">
          <thead><tr><th>{t('frontend/src/lib/AdminPanel.svelte::id_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::title_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::class_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::deadline_table_header')}</th><th>{t('frontend/src/lib/AdminPanel.svelte::status_table_header')}</th><th></th></tr></thead>
          <tbody>
            {#each filteredAssignments as a}
              <tr>
                <td>{a.id}</td>
                <td><a href={`/assignments/${a.id}`} class="link link-primary">{a.title}</a></td>
                <td>{a.class_id}</td>
                <td>{formatDateTime(a.deadline)}</td>
                <td>{a.published ? t('frontend/src/lib/AdminPanel.svelte::published_filter_option') : t('frontend/src/lib/AdminPanel.svelte::unpublished_filter_option')}</td>
                <td class="text-right whitespace-nowrap">
                  {#if !a.published}
                    <button class="btn btn-xs" on:click={()=>publishAssignment(a.id)}>{t('frontend/src/lib/AdminPanel.svelte::publish_button')}</button>
                  {/if}
                  <button class="btn btn-ghost btn-xs text-error" on:click={()=>deleteAssignment(a.id)}><Trash2 class="w-4 h-4" /></button>
                </td>
              </tr>
            {/each}
            {#if !filteredAssignments.length}
              <tr><td colspan="6"><i>{t('frontend/src/lib/AdminPanel.svelte::no_assignments_message')}</i></td></tr>
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  </div>
{/if}

<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch, apiJSON } from '$lib/api';
  import { sha256 } from '$lib/hash';
  import { formatDate } from "$lib/date";
  import { classesStore } from '$lib/stores/classes';
  import {
    Users2, GraduationCap, School, Plus, Trash2, RefreshCw,
    Shield, Search, Edit, ArrowRightLeft, Check, KeyRound, MailCheck
  } from 'lucide-svelte';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import PromptModal from '$lib/components/PromptModal.svelte';
  import CustomSelect from '$lib/components/CustomSelect.svelte';
  import { t, translator } from '$lib/i18n';

  let translate;
  $: translate = $translator;

  // Tabs
  type Tab = 'overview' | 'people' | 'classes' | 'settings';
  let tab: Tab = 'overview';

  // Data models
  type RawUser = { id?: string; email?: string | null; name?: string | null; role?: string | null; created_at?: string | null; bk_uid?: string | null; ms_oid?: string | null };
  type User = { id: string; email: string; name?: string | null; role: string; created_at: string; bk_uid?: string | null; ms_oid?: string | null };
  type RawClass = { id?: string; name?: string | null; teacher_id?: string | null; created_at?: string | null };
  type Class = { id: string; name: string; teacher_id: string; created_at: string };
  type RawOnlineUser = { id?: string | null; name?: string | null; avatar?: string | null; email?: string | null };
  type OnlineUser = { id: string; name: string; avatar: string; email: string };
  type SystemVariable = { key: string; value: string };

  // Collections
  let users: User[] = [];
  let classes: Class[] = [];
  let onlineUsers: OnlineUser[] = [];
  let whitelist: { email: string; created_at: string }[] = [];
  let systemVariables: SystemVariable[] = [];

  // Filters/search
  let userQuery = '';
  let classQuery = '';
  let userRoleFilter: 'all' | 'student' | 'teacher' | 'admin' = 'all';

  // Derived
  $: teachers = users.filter(u => u.role === 'teacher');
  $: admins = users.filter(u => u.role === 'admin');
  $: students = users.filter(u => u.role === 'student');
  $: teacherIdToClassCount = classes.reduce<Record<string, number>>((m, c) => { m[c.teacher_id] = (m[c.teacher_id] || 0) + 1; return m; }, {});
  $: teacherLookup = teachers.reduce<Record<string, User>>((m, t_user) => { m[t_user.id] = t_user; return m; }, {});
  $: filteredUsers = users
    .filter(u => (u.email + ' ' + (u.name ?? '')).toLowerCase().includes(userQuery.toLowerCase()))
    .filter(u => userRoleFilter === 'all' ? true : u.role === userRoleFilter);
  $: filteredClasses = classes.filter(c => {
    const teacher = teacherLookup[c.teacher_id];
    const teacherText = teacher ? `${teacher.name ?? ''} ${teacher.email ?? ''}` : '';
    return (c.name + ' ' + teacherText).toLowerCase().includes(classQuery.toLowerCase());
  });
  $: onlineIds = new Set(onlineUsers.map(u => u.id));

  // Loading states
  let loadingUsers = false, loadingClasses = false, loadingVariables = false;
  let ok = '', err = '';
  let confirmModal: InstanceType<typeof ConfirmModal>;
  let promptModal: InstanceType<typeof PromptModal>;
  let emailPingTarget = '';
  let sendingEmailPing = false;
  let showEmailTools = false;
  let showCreateUsers = false;

  // System variable form state
  let variableKey = '';
  let variableValue = '';
  let editingVariableKey: string | null = null;

  function userPrimary(user?: User | null) {
    return (user?.name ?? '').trim() || user?.email || t('frontend/src/lib/AdminPanel.svelte::unknown_user_label');
  }

  function userSecondary(user?: User | null) {
    return user?.name ? user?.email : '';
  }

  function hasEmailLogin(user: User) {
    return !user.ms_oid && user.email.includes('@');
  }

  function sanitizeUsers(payload: unknown): User[] {
    if (!Array.isArray(payload)) return [];
    return payload
      .filter((entry): entry is RawUser => !!entry && typeof entry === 'object')
      .filter((entry) => typeof entry.id === 'string' && typeof entry.email === 'string' && typeof entry.role === 'string')
      .map((entry) => ({
        id: entry.id!,
        email: entry.email!,
        role: entry.role!,
        created_at: typeof entry.created_at === 'string' ? entry.created_at : entry.created_at ?? '',
        name: typeof entry.name === 'string' ? entry.name : entry.name ?? null,
        bk_uid: typeof entry.bk_uid === 'string' ? entry.bk_uid : entry.bk_uid ?? null,
        ms_oid: typeof entry.ms_oid === 'string' ? entry.ms_oid : entry.ms_oid ?? null
      }));
  }

  function sanitizeClasses(payload: unknown): Class[] {
    if (!Array.isArray(payload)) return [];
    return payload
      .filter((entry): entry is RawClass => !!entry && typeof entry === 'object')
      .filter((entry) => typeof entry.id === 'string')
      .map((entry) => ({
        id: entry.id!,
        name: typeof entry.name === 'string' ? entry.name : entry.name ?? '',
        teacher_id: typeof entry.teacher_id === 'string' ? entry.teacher_id : entry.teacher_id ?? '',
        created_at: typeof entry.created_at === 'string' ? entry.created_at : entry.created_at ?? ''
      }));
  }

  async function loadUsers() {
    loadingUsers = true; err = '';
    try { users = sanitizeUsers(await apiJSON('/api/users')); } catch (e: any) { err = e.message; users = []; }
    loadingUsers = false;
  }
  async function loadClasses() {
    loadingClasses = true; err = '';
    try {
      classes = sanitizeClasses(await apiJSON('/api/classes/all'));
      // Update the store with all classes for admin view
      classesStore.setClasses(classes);
    } catch (e: any) { err = e.message; classes = []; classesStore.setClasses([]); }
    loadingClasses = false;
  }

  async function loadSystemVariables() {
    loadingVariables = true; err = '';
    try { systemVariables = await apiJSON<SystemVariable[]>('/api/admin/system-variables'); } catch (e: any) { err = e.message; systemVariables = []; }
    loadingVariables = false;
  }

  function resetVariableForm() {
    variableKey = '';
    variableValue = '';
    editingVariableKey = null;
  }

  function startEditVariable(variable: SystemVariable) {
    editingVariableKey = variable.key;
    variableKey = variable.key;
    variableValue = variable.value;
  }

  async function saveSystemVariable() {
    const key = variableKey.trim();
    err = ok = '';
    if (!key) return;
    try {
      const res = await apiFetch('/api/admin/system-variables', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key, value: variableValue })
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || t('frontend/src/lib/AdminPanel.svelte::system_variable_save_failed'));
      }
      ok = t('frontend/src/lib/AdminPanel.svelte::system_variable_saved_success', { key });
      resetVariableForm();
      await loadSystemVariables();
    } catch (e: any) {
      err = e.message;
    }
  }

  async function deleteSystemVariable(key: string) {
    const confirmed = await confirmModal.open({
      title: t('frontend/src/lib/AdminPanel.svelte::system_variable_delete_title', { key }),
      body: t('frontend/src/lib/AdminPanel.svelte::system_variable_delete_body', { key }),
      confirmLabel: t('frontend/src/lib/AdminPanel.svelte::delete_button'),
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    });
    if (!confirmed) return;
    err = ok = '';
    try {
      const res = await apiFetch(`/api/admin/system-variables/${encodeURIComponent(key)}`, { method: 'DELETE' });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || t('frontend/src/lib/AdminPanel.svelte::system_variable_save_failed'));
      }
      ok = t('frontend/src/lib/AdminPanel.svelte::system_variable_deleted_success', { key });
      if (editingVariableKey === key) resetVariableForm();
      await loadSystemVariables();
    } catch (e: any) {
      err = e.message;
    }
  }

  // Whitelist management
  async function loadWhitelist() {
    try {
      const res = await apiJSON<{email: string, created_at: string}[]>('/api/admin/whitelist');
      whitelist = Array.isArray(res) ? res : [];
    } catch { whitelist = []; }
  }

  let whitelistEmail = '';
  async function addToWhitelist() {
    if (!whitelistEmail) return;
    err = ok = '';
    try {
      const res = await apiFetch('/api/admin/whitelist', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: whitelistEmail })
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        err = data.error || t('frontend/src/lib/AdminPanel.svelte::whitelist_add_failed');
        return;
      }
      ok = t('frontend/src/lib/AdminPanel.svelte::whitelist_added_success');
      whitelistEmail = '';
      await loadWhitelist();
    } catch (e: any) {
      const j = await e.response?.json().catch(() => ({}));
      err = j.error || e.message || t('frontend/src/lib/AdminPanel.svelte::whitelist_add_failed');
    }
  }

  async function removeFromWhitelist(email: string) {
    const confirmed = await confirmModal.open({
      title: t('frontend/src/lib/AdminPanel.svelte::whitelist_remove_title', { email }),
      body: t('frontend/src/lib/AdminPanel.svelte::whitelist_remove_body', { email }),
      confirmLabel: t('frontend/src/lib/AdminPanel.svelte::remove_button'),
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    });
    if (!confirmed) return;
    err = ok = '';
    try {
      const res = await apiFetch(`/api/admin/whitelist/${encodeURIComponent(email)}`, { method: 'DELETE' });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        err = data.error || t('frontend/src/lib/AdminPanel.svelte::whitelist_remove_failed');
        return;
      }
      ok = t('frontend/src/lib/AdminPanel.svelte::whitelist_removed_success');
      await loadWhitelist();
    } catch (e: any) {
      err = e.message || t('frontend/src/lib/AdminPanel.svelte::whitelist_remove_failed');
    }
  }

  // System Settings
  let forceBakalariEmail = true;
  let allowMicrosoftLogin = true;
  async function loadSystemSettings() {
    try {
      const s = await apiJSON<{force_bakalari_email: boolean, allow_microsoft_login: boolean}>('/api/admin/system-settings');
      forceBakalariEmail = s.force_bakalari_email;
      allowMicrosoftLogin = s.allow_microsoft_login;
    } catch {}
  }
  async function updateSetting(key: 'force_bakalari_email' | 'allow_microsoft_login', val: boolean) {
    try {
      await apiFetch('/api/admin/system-settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ [key]: val })
      });
      if (key === 'force_bakalari_email') forceBakalariEmail = val;
      if (key === 'allow_microsoft_login') allowMicrosoftLogin = val;
      ok = t('frontend/src/lib/AdminPanel.svelte::settings_updated_success');
    } catch (e: any) {
      err = e.message;
    }
  }

  async function loadOnlineUsers() {
    try {
      const response = await apiJSON<RawOnlineUser[]>('/api/online-users');
      onlineUsers = Array.isArray(response)
        ? response
            .filter((entry): entry is RawOnlineUser => !!entry && typeof entry === 'object')
            .map((entry): OnlineUser => ({
              id: entry.id ?? '',
              name: entry.name ?? '',
              email: entry.email ?? '',
              avatar: entry.avatar ?? ''
            }))
            .filter((entry) => Boolean(entry.id))
        : [];
    } catch (e) {
      console.error('Failed to load online users', e);
      onlineUsers = [];
    }
  }

  onMount(() => {
    loadUsers();
    loadClasses();
    loadOnlineUsers();
    loadSystemSettings();
    loadWhitelist();
    loadSystemVariables();
    const presenceTimer = setInterval(loadOnlineUsers, 30000);
    return () => clearInterval(presenceTimer);
  });

  function refreshUsers() {
    loadUsers();
    loadOnlineUsers();
  }

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
  // Student management
  // ───────────────────────────
  let studentEmail = '', studentPassword = '', studentName = '';
  async function addStudent() {
    err = ok = '';
    const payload: Record<string, string> = {
      email: studentEmail,
      password: await sha256(studentPassword)
    };
    const trimmedName = studentName.trim();
    if (trimmedName) payload.name = trimmedName;
    const r = await apiFetch('/api/students', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (r.status === 201) {
      ok = t('frontend/src/lib/AdminPanel.svelte::student_created_success');
      studentEmail = studentPassword = studentName = '';
      await loadUsers();
    } else {
      const j = await r.json().catch(() => ({}));
      err = j.error || t('frontend/src/lib/AdminPanel.svelte::failed_to_create_student');
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

  $: roleOptions = roles.map(r => ({
    value: r,
    label: r.charAt(0).toUpperCase() + r.slice(1)
  }));

  $: teacherOptions = teachers.map(t_user => ({
    value: t_user.id,
    label: userPrimary(t_user) + (t_user.name ? ` - ${t_user.email}` : '')
  }));
  function exportUsersCSV() {
    const rows = [[t('frontend/src/lib/AdminPanel.svelte::user_csv_header_email'), t('frontend/src/lib/AdminPanel.svelte::user_csv_header_name'), t('frontend/src/lib/AdminPanel.svelte::user_csv_header_role'), t('frontend/src/lib/AdminPanel.svelte::user_csv_header_created')]]
      .concat(users.map(u => [u.email, u.name ?? '', u.role, formatDate(u.created_at)]));
    const csv = rows.map(r => r.map(v => '"' + v.replaceAll('"','""') + '"').join(',')).join('\n');
    const a = document.createElement('a');
    a.href = 'data:text/csv;charset=utf-8,' + encodeURIComponent(csv);
    a.download = 'users.csv';
    a.click();
  }

  async function sendEmailPing() {
    const target = emailPingTarget.trim();
    if (!target) return;
    err = ok = '';
    sendingEmailPing = true;
    try {
      const res = await apiFetch('/api/admin/email-ping', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: target })
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok) {
        ok = t('frontend/src/lib/AdminPanel.svelte::email_ping_success', { email: target });
        emailPingTarget = '';
      } else {
        err = data.error ?? t('frontend/src/lib/AdminPanel.svelte::email_ping_failed');
      }
    } catch (e: any) {
      err = e.message;
    } finally {
      sendingEmailPing = false;
    }
  }

  async function promptSetPassword(user: User) {
    if (!promptModal) return;
    const password = await promptModal.open({
      title: t('frontend/src/lib/AdminPanel.svelte::set_password_modal_title', { email: user.email }),
      label: t('frontend/src/lib/AdminPanel.svelte::set_password_modal_label'),
      helpText: t('frontend/src/lib/AdminPanel.svelte::set_password_modal_help'),
      inputType: 'password',
      confirmLabel: t('frontend/src/lib/AdminPanel.svelte::set_password_confirm_label'),
      validate: (value) => value.trim().length >= 6 ? null : t('frontend/src/lib/AdminPanel.svelte::set_password_validation_error')
    });
    if (!password) return;
    err = ok = '';
    const res = await apiFetch(`/api/users/${user.id}/password`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: await sha256(password) })
    });
    if (res.ok) {
      ok = t('frontend/src/lib/AdminPanel.svelte::set_password_success');
    } else {
      const data = await res.json().catch(() => ({}));
      err = data.error ?? t('frontend/src/lib/AdminPanel.svelte::set_password_failed');
    }
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
      ok = t('frontend/src/lib/AdminPanel.svelte::class_created_success', { name: newClassName.trim() });
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
  let transferTarget: { id: string, name: string, to: string | null } | null = null;
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
</script>

<h1 class="text-2xl font-bold mb-4 flex items-center gap-2"><Shield class="w-6 h-6" aria-hidden="true" /> {t('frontend/src/lib/AdminPanel.svelte::admin')}</h1>

<div class="mb-6 flex flex-wrap items-center gap-2 rounded-full bg-base-200/70 p-2" role="tablist">
  <button
    type="button"
    role="tab"
    class="btn btn-sm rounded-full"
    class:btn-primary={tab === 'overview'}
    class:btn-ghost={tab !== 'overview'}
    aria-selected={tab === 'overview'}
    on:click={() => tab='overview'}
  >
    {t('frontend/src/lib/AdminPanel.svelte::overview_tab')}
  </button>
  <button
    type="button"
    role="tab"
    class="btn btn-sm rounded-full"
    class:btn-primary={tab === 'people'}
    class:btn-ghost={tab !== 'people'}
    aria-selected={tab === 'people'}
    on:click={() => tab='people'}
  >
    {t('frontend/src/lib/AdminPanel.svelte::people_tab')}
    <span class="badge badge-sm ml-2">{users.length}</span>
  </button>
  <button
    type="button"
    role="tab"
    class="btn btn-sm rounded-full"
    class:btn-primary={tab === 'classes'}
    class:btn-ghost={tab !== 'classes'}
    aria-selected={tab === 'classes'}
    on:click={() => tab='classes'}
  >
    {t('frontend/src/lib/AdminPanel.svelte::classes_tab')}
    <span class="badge badge-sm ml-2">{classes.length}</span>
  </button>
  <button
    type="button"
    role="tab"
    class="btn btn-sm rounded-full"
    class:btn-primary={tab === 'settings'}
    class:btn-ghost={tab !== 'settings'}
    aria-selected={tab === 'settings'}
    on:click={() => tab='settings'}
  >
    {t('frontend/src/lib/AdminPanel.svelte::settings_tab')}
  </button>
</div>

{#if ok}
  <div class="alert alert-success mb-4"><Check class="w-4 h-4" aria-hidden="true" /> {ok}</div>
{/if}
{#if err}
  <div class="alert alert-error mb-4">{err}</div>
{/if}

{#if tab === 'overview'}
  <div class="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
    <div class="space-y-6">
      <div class="card bg-base-100 shadow">
        <div class="card-body space-y-4">
          <div class="flex items-center justify-between">
            <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::platform_stats_title')}</h2>
            <button class="btn btn-ghost btn-sm" on:click={refreshUsers} aria-label={t('frontend/src/lib/AdminPanel.svelte::refresh_online_users_button')}>
              <RefreshCw class="w-4 h-4" />
            </button>
          </div>
          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <div class="rounded-box border border-base-200/60 bg-base-100 p-4">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <div class="text-sm text-base-content/60">{t('frontend/src/lib/AdminPanel.svelte::users_stat_title')}</div>
                  <div class="text-2xl font-semibold">{users.length}</div>
                </div>
                <Users2 class="w-5 h-5 text-primary" aria-hidden="true" />
              </div>
            </div>
            <div class="rounded-box border border-base-200/60 bg-base-100 p-4">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <div class="text-sm text-base-content/60">{t('frontend/src/lib/AdminPanel.svelte::teachers_stat_title')}</div>
                  <div class="text-2xl font-semibold">{teachers.length}</div>
                </div>
                <GraduationCap class="w-5 h-5 text-primary" aria-hidden="true" />
              </div>
            </div>
            <div class="rounded-box border border-base-200/60 bg-base-100 p-4">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <div class="text-sm text-base-content/60">{t('frontend/src/lib/AdminPanel.svelte::students_stat_title')}</div>
                  <div class="text-2xl font-semibold">{students.length}</div>
                </div>
                <Users2 class="w-5 h-5 text-primary" aria-hidden="true" />
              </div>
            </div>
            <div class="rounded-box border border-base-200/60 bg-base-100 p-4">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <div class="text-sm text-base-content/60">{t('frontend/src/lib/AdminPanel.svelte::classes_stat_title')}</div>
                  <div class="text-2xl font-semibold">{classes.length}</div>
                </div>
                <School class="w-5 h-5 text-primary" aria-hidden="true" />
              </div>
            </div>
            <div class="rounded-box border border-base-200/60 bg-base-100 p-4">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <div class="text-sm text-base-content/60">{t('frontend/src/lib/AdminPanel.svelte::online_stat_title')}</div>
                  <div class="text-2xl font-semibold">{onlineUsers.length}</div>
                </div>
                <Users2 class="w-5 h-5 text-primary" aria-hidden="true" />
              </div>
            </div>
            <div class="rounded-box border border-base-200/60 bg-base-100 p-4">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <div class="text-sm text-base-content/60">{t('frontend/src/lib/AdminPanel.svelte::role_filter_admins')}</div>
                  <div class="text-2xl font-semibold">{admins.length}</div>
                </div>
                <Shield class="w-5 h-5 text-primary" aria-hidden="true" />
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="card bg-base-100 shadow">
        <div class="card-body gap-3">
          <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::quick_actions_title')}</h2>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-primary btn-sm" on:click={() => { tab='people'; showCreateUsers = true; }}>{t('frontend/src/lib/AdminPanel.svelte::add_teacher_button')}</button>
            <button class="btn btn-sm" on:click={() => { tab='people'; showCreateUsers = true; }}>{t('frontend/src/lib/AdminPanel.svelte::add_student_card_title')}</button>
            <button class="btn btn-sm" on:click={() => { tab='classes'; showCreateClass = true; }}>{t('frontend/src/lib/AdminPanel.svelte::create_class_button')}</button>
            <button class="btn btn-sm" on:click={() => { tab='settings'; }}>{t('frontend/src/lib/AdminPanel.svelte::settings_tab')}</button>
            <button class="btn btn-sm" on:click={() => { showEmailTools = true; }}>
              <MailCheck class="w-4 h-4" aria-hidden="true" />
              {t('frontend/src/lib/AdminPanel.svelte::email_ping_card_title')}
            </button>
            <a class="btn btn-ghost btn-sm" href="/dashboard">{t('frontend/src/lib/AdminPanel.svelte::go_to_dashboard_button')}</a>
          </div>
        </div>
      </div>
    </div>
    <div class="space-y-6">
      <div class="card bg-base-100 shadow">
        <div class="card-body space-y-3">
          <div class="flex items-center justify-between">
            <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::online_users_card_title')}</h2>
            <button class="btn btn-ghost btn-xs" on:click={loadOnlineUsers} aria-label={t('frontend/src/lib/AdminPanel.svelte::refresh_online_users_button')}>
              <RefreshCw class="w-4 h-4" />
            </button>
          </div>
          <ul class="space-y-3 max-h-72 overflow-auto">
            {#if onlineUsers.length}
              {#each onlineUsers as online}
                <li class="flex items-center gap-3">
                  <div class="avatar">
                    {#if online.avatar}
                      <div class="w-10 rounded-full">
                        <img src={online.avatar} alt={t('frontend/src/lib/AdminPanel.svelte::online_user_avatar_alt', { name: online.name || online.email })} loading="lazy" />
                      </div>
                    {:else}
                      <div class="placeholder w-10 rounded-full bg-primary/10 text-primary font-semibold flex items-center justify-center">
                        {(online.name || online.email || '?').charAt(0).toUpperCase()}
                      </div>
                    {/if}
                  </div>
                  <div class="flex flex-col leading-tight">
                    <span class="font-medium">{online.name || online.email || '?'}</span>
                    {#if online.name}
                      <span class="text-xs text-base-content/60">{online.email}</span>
                    {/if}
                  </div>
                </li>
              {/each}
            {:else}
              <li class="text-sm text-base-content/70">{t('frontend/src/lib/AdminPanel.svelte::no_online_users_message')}</li>
            {/if}
          </ul>
        </div>
      </div>
    </div>
  </div>
{/if}

<ConfirmModal bind:this={confirmModal} />
<PromptModal bind:this={promptModal} />

{#if tab === 'people'}
  <div class="grid gap-6 xl:grid-cols-[0.95fr_1.05fr]">
    <div class="space-y-6">
      <div class="card bg-base-100 shadow">
        <div class="card-body space-y-4">
          <div class="flex items-center justify-between gap-3">
            <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::account_creation_title')}</h2>
            <button class="btn btn-ghost btn-sm" on:click={() => { showCreateUsers = !showCreateUsers; }}>
              {showCreateUsers ? t('frontend/src/lib/AdminPanel.svelte::hide_account_tools_button') : t('frontend/src/lib/AdminPanel.svelte::show_account_tools_button')}
            </button>
          </div>
          {#if showCreateUsers}
            <div class="grid gap-4 md:grid-cols-2">
              <div class="rounded-box border border-base-200/60 bg-base-100 p-4 space-y-3">
                <h3 class="font-semibold">{t('frontend/src/lib/AdminPanel.svelte::add_teacher_card_title')}</h3>
                <form on:submit|preventDefault={addTeacher} class="space-y-3">
                  <input type="email" bind:value={teacherEmail} placeholder={t('frontend/src/lib/AdminPanel.svelte::email_label')} required class="input input-bordered w-full" />
                  <input type="password" bind:value={teacherPassword} placeholder={t('frontend/src/lib/AdminPanel.svelte::password_label')} required class="input input-bordered w-full" />
                  <button class="btn btn-primary btn-sm">{t('frontend/src/lib/AdminPanel.svelte::add_button')}</button>
                </form>
              </div>
              <div class="rounded-box border border-base-200/60 bg-base-100 p-4 space-y-3">
                <h3 class="font-semibold">{t('frontend/src/lib/AdminPanel.svelte::add_student_card_title')}</h3>
                <form on:submit|preventDefault={addStudent} class="space-y-3">
                  <input type="text" bind:value={studentName} placeholder={t('frontend/src/lib/AdminPanel.svelte::student_name_placeholder')} class="input input-bordered w-full" />
                  <input type="email" bind:value={studentEmail} placeholder={t('frontend/src/lib/AdminPanel.svelte::email_label')} required class="input input-bordered w-full" />
                  <input type="password" bind:value={studentPassword} placeholder={t('frontend/src/lib/AdminPanel.svelte::password_label')} required class="input input-bordered w-full" />
                  <button class="btn btn-primary btn-sm">{t('frontend/src/lib/AdminPanel.svelte::add_button')}</button>
                </form>
              </div>
            </div>
          {/if}
        </div>
      </div>
      <div class="card bg-base-100 shadow">
        <div class="card-body space-y-4">
          <div class="flex items-center justify-between">
            <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::teachers_card_title')}</h2>
            <button class="btn btn-ghost btn-sm" on:click={loadUsers}><RefreshCw class="w-4 h-4" /></button>
          </div>
          <div class="overflow-x-auto">
            <table class="table table-zebra table-sm">
              <thead>
                <tr>
                  <th>{t('frontend/src/lib/AdminPanel.svelte::name_table_header')}</th>
                  <th>{t('frontend/src/lib/AdminPanel.svelte::classes_table_header')}</th>
                  <th>{t('frontend/src/lib/AdminPanel.svelte::created_table_header')}</th>
                </tr>
              </thead>
              <tbody>
                {#each teachers as t_user}
                  <tr>
                    <td>
                      <div class="flex flex-col">
                        <span class="font-medium">{userPrimary(t_user)}</span>
                        {#if userSecondary(t_user)}
                          <span class="text-xs text-base-content/60">{userSecondary(t_user)}</span>
                        {/if}
                      </div>
                    </td>
                    <td>{teacherIdToClassCount[t_user.id] ?? 0}</td>
                    <td>{formatDate(t_user.created_at)}</td>
                  </tr>
                {/each}
                {#if !teachers.length}
                  <tr><td colspan="3"><i>{t('frontend/src/lib/AdminPanel.svelte::no_teachers_message')}</i></td></tr>
                {/if}
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <div class="card bg-base-100 shadow">
        <div class="card-body space-y-4">
          <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::whitelist_title')}</h2>
          <p class="text-sm text-base-content/70">{t('frontend/src/lib/AdminPanel.svelte::whitelist_description')}</p>
          <form on:submit|preventDefault={addToWhitelist} class="flex flex-col sm:flex-row gap-2 max-w-lg">
            <input type="email" bind:value={whitelistEmail} placeholder={t('frontend/src/lib/AdminPanel.svelte::whitelist_placeholder')} required class="input input-bordered w-full" />
            <button class="btn btn-primary btn-sm">{t('frontend/src/lib/AdminPanel.svelte::add_button')}</button>
          </form>
          <div class="overflow-x-auto max-h-60 border rounded-box">
            <table class="table table-compact w-full">
              <thead><tr><th>{t('frontend/src/lib/AdminPanel.svelte::email_table_header')}</th><th></th></tr></thead>
              <tbody>
                {#each whitelist as w}
                  <tr>
                    <td>{w.email}</td>
                    <td class="text-right">
                      <button class="btn btn-ghost btn-xs text-error" on:click={() => removeFromWhitelist(w.email)}><Trash2 class="w-4 h-4" /></button>
                    </td>
                  </tr>
                {/each}
                {#if !whitelist.length}
                  <tr><td colspan="2" class="text-center italic opacity-70">{t('frontend/src/lib/AdminPanel.svelte::whitelist_empty')}</td></tr>
                {/if}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
    <div class="card bg-base-100 shadow">
      <div class="card-body space-y-4">
        <div class="flex items-start justify-between flex-wrap gap-3">
          <div>
            <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::users_card_title')}</h2>
            <p class="text-sm text-base-content/60">{onlineUsers.length} {t('frontend/src/lib/AdminPanel.svelte::online_stat_title')}</p>
          </div>
          <div class="flex items-center gap-2 flex-wrap">
            <label class="input input-bordered input-sm flex items-center gap-2">
              <Search class="w-4 h-4" aria-hidden="true" />
              <input class="grow" placeholder={t('frontend/src/lib/AdminPanel.svelte::search_users_placeholder')} bind:value={userQuery} />
            </label>
            <button class="btn btn-sm" on:click={exportUsersCSV}>{t('frontend/src/lib/AdminPanel.svelte::export_csv_button')}</button>
            <button class="btn btn-ghost btn-sm" on:click={refreshUsers}><RefreshCw class="w-4 h-4" /></button>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button class="btn btn-xs" class:btn-primary={userRoleFilter === 'all'} class:btn-ghost={userRoleFilter !== 'all'} on:click={() => userRoleFilter = 'all'}>
            {t('frontend/src/lib/AdminPanel.svelte::all_filter_option')}
          </button>
          <button class="btn btn-xs" class:btn-primary={userRoleFilter === 'student'} class:btn-ghost={userRoleFilter !== 'student'} on:click={() => userRoleFilter = 'student'}>
            {t('frontend/src/lib/AdminPanel.svelte::role_filter_students')}
          </button>
          <button class="btn btn-xs" class:btn-primary={userRoleFilter === 'teacher'} class:btn-ghost={userRoleFilter !== 'teacher'} on:click={() => userRoleFilter = 'teacher'}>
            {t('frontend/src/lib/AdminPanel.svelte::role_filter_teachers')}
          </button>
          <button class="btn btn-xs" class:btn-primary={userRoleFilter === 'admin'} class:btn-ghost={userRoleFilter !== 'admin'} on:click={() => userRoleFilter = 'admin'}>
            {t('frontend/src/lib/AdminPanel.svelte::role_filter_admins')}
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="table table-zebra">
            <thead>
              <tr>
                <th>{t('frontend/src/lib/AdminPanel.svelte::name_table_header')}</th>
                <th>{t('frontend/src/lib/AdminPanel.svelte::role_table_header')}</th>
                <th>{t('frontend/src/lib/AdminPanel.svelte::auth_table_header')}</th>
                <th>{t('frontend/src/lib/AdminPanel.svelte::created_table_header')}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each filteredUsers as u}
                <tr>
                  <td>
                    <div class="flex items-center gap-3">
                      <span class={`h-2.5 w-2.5 rounded-full ${onlineIds.has(u.id) ? 'bg-success' : 'bg-base-300'}`} />
                      <div class="flex flex-col">
                        <span class="font-medium">{userPrimary(u)}</span>
                        {#if userSecondary(u)}
                          <span class="text-xs text-base-content/60">{userSecondary(u)}</span>
                        {/if}
                      </div>
                    </div>
                  </td>
                  <td>
                    <div class="w-32">
                      <CustomSelect 
                        small 
                        options={roleOptions} 
                        bind:value={u.role} 
                        on:change={(e) => changeRole(u.id, e.detail)}
                      />
                    </div>
                  </td>
                  <td>
                    <div class="flex flex-wrap gap-1">
                      {#if hasEmailLogin(u)}
                        <span class="badge badge-outline">{t('frontend/src/lib/AdminPanel.svelte::auth_email_label')}</span>
                      {/if}
                      {#if u.ms_oid}
                        <span class="badge badge-outline">{t('frontend/src/lib/AdminPanel.svelte::auth_microsoft_label')}</span>
                      {/if}
                      {#if u.bk_uid}
                        <span class="badge badge-outline">{t('frontend/src/lib/AdminPanel.svelte::auth_bakalari_label')}</span>
                      {/if}
                    </div>
                  </td>
                  <td>{formatDate(u.created_at)}</td>
                  <td class="text-right">
                    <div class="flex justify-end gap-1">
                      <button
                        class="btn btn-ghost btn-xs"
                        title={u.bk_uid ? t('frontend/src/lib/AdminPanel.svelte::set_password_disabled_tooltip') : t('frontend/src/lib/AdminPanel.svelte::set_password_button_label')}
                        disabled={Boolean(u.bk_uid)}
                        on:click={() => { if (!u.bk_uid) promptSetPassword(u); }}
                      >
                        <KeyRound class="w-4 h-4" />
                      </button>
                      <button class="btn btn-ghost btn-xs text-error" on:click={()=>deleteUser(u.id)}><Trash2 class="w-4 h-4" /></button>
                    </div>
                  </td>
                </tr>
              {/each}
              {#if !filteredUsers.length}
                <tr><td colspan="5"><i>{t('frontend/src/lib/AdminPanel.svelte::no_users_message')}</i></td></tr>
              {/if}
            </tbody>
          </table>
        </div>
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
          <thead>
            <tr>
              <th>{t('frontend/src/lib/AdminPanel.svelte::class_table_header')}</th>
              <th>{t('frontend/src/lib/AdminPanel.svelte::teacher_table_header')}</th>
              <th>{t('frontend/src/lib/AdminPanel.svelte::created_table_header')}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each filteredClasses as c}
              <tr>
                <td><a href={`/classes/${c.id}`} class="link link-primary">{c.name}</a></td>
                <td>
                  {#if teacherLookup[c.teacher_id]}
                    <div class="flex flex-col">
                      <span class="font-medium">{userPrimary(teacherLookup[c.teacher_id])}</span>
                      {#if userSecondary(teacherLookup[c.teacher_id])}
                        <span class="text-xs text-base-content/60">{userSecondary(teacherLookup[c.teacher_id])}</span>
                      {/if}
                    </div>
                  {:else}
                    <span class="text-sm text-base-content/60">{t('frontend/src/lib/AdminPanel.svelte::unassigned_teacher_label')}</span>
                  {/if}
                </td>
                <td>{formatDate(c.created_at)}</td>
                <td class="text-right whitespace-nowrap">
                  <button class="btn btn-ghost btn-xs" on:click={()=>renameClass(c.id)}><Edit class="w-4 h-4" /></button>
                  <button class="btn btn-ghost btn-xs" on:click={()=>transferTarget={ id: c.id, name: c.name, to: null }}><ArrowRightLeft class="w-4 h-4" /></button>
                  <button class="btn btn-ghost btn-xs text-error" on:click={()=>deleteClassAction(c.id)}><Trash2 class="w-4 h-4" /></button>
                </td>
              </tr>
            {/each}
            {#if !filteredClasses.length}
              <tr><td colspan="4"><i>{t('frontend/src/lib/AdminPanel.svelte::no_classes_message')}</i></td></tr>
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
        <CustomSelect 
          searchable
          options={teacherOptions} 
          bind:value={newClassTeacherId} 
          placeholder={t('frontend/src/lib/AdminPanel.svelte::select_teacher_option')}
        />
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
        <p>{t('frontend/src/lib/AdminPanel.svelte::transfer_ownership_modal_body', { className: transferTarget.name || t('frontend/src/lib/AdminPanel.svelte::class_table_header') })}</p>
        <CustomSelect 
          searchable
          options={teacherOptions} 
          bind:value={transferTarget.to} 
          placeholder={t('frontend/src/lib/AdminPanel.svelte::select_teacher_option')}
        />
        <div class="modal-action">
          <button class="btn" on:click={transferClass}><ArrowRightLeft class="w-4 h-4" /> {t('frontend/src/lib/AdminPanel.svelte::transfer_button')}</button>
          <button class="btn btn-ghost" on:click={() => transferTarget = null}>{t('frontend/src/lib/AdminPanel.svelte::cancel_button')}</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" on:click={() => transferTarget = null}><button>close</button></form>
    </dialog>
  {/if}
{/if}

{#if tab === 'settings'}
  <div class="space-y-6">
    <div class="card bg-base-100 shadow">
      <div class="card-body space-y-4">
        <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::system_settings_title')}</h2>
        <div class="grid gap-4 md:grid-cols-2">
          <label class="rounded-box border border-base-200/60 bg-base-100 p-4 flex items-start justify-between gap-4">
            <div class="flex flex-col">
              <span class="font-medium">{t('frontend/src/lib/AdminPanel.svelte::force_bakalari_email_label')}</span>
              <span class="text-sm text-base-content/70">{t('frontend/src/lib/AdminPanel.svelte::force_bakalari_email_description')}</span>
            </div>
            <input type="checkbox" class="toggle toggle-primary mt-1" checked={forceBakalariEmail} on:change={(e) => updateSetting('force_bakalari_email', (e.target as HTMLInputElement).checked)} />
          </label>
          <label class="rounded-box border border-base-200/60 bg-base-100 p-4 flex items-start justify-between gap-4">
            <div class="flex flex-col">
              <span class="font-medium">{t('frontend/src/lib/AdminPanel.svelte::allow_microsoft_login_label')}</span>
              <span class="text-sm text-base-content/70">{t('frontend/src/lib/AdminPanel.svelte::allow_microsoft_login_description')}</span>
            </div>
            <input type="checkbox" class="toggle toggle-primary mt-1" checked={allowMicrosoftLogin} on:change={(e) => updateSetting('allow_microsoft_login', (e.target as HTMLInputElement).checked)} />
          </label>
        </div>
      </div>
    </div>

    <div class="card bg-base-100 shadow">
      <div class="card-body space-y-4">
        <div>
          <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::system_variables_title')}</h2>
          <p class="text-sm text-base-content/70">{t('frontend/src/lib/AdminPanel.svelte::system_variables_description')}</p>
        </div>
        <form on:submit|preventDefault={saveSystemVariable} class="grid gap-3 md:grid-cols-[1fr_2fr_auto] items-end">
          <label class="form-control">
            <span class="label-text">{t('frontend/src/lib/AdminPanel.svelte::system_variable_key_label')}</span>
            <input
              class="input input-bordered"
              placeholder={t('frontend/src/lib/AdminPanel.svelte::system_variable_key_placeholder')}
              bind:value={variableKey}
              disabled={Boolean(editingVariableKey)}
              required
            />
          </label>
          <label class="form-control">
            <span class="label-text">{t('frontend/src/lib/AdminPanel.svelte::system_variable_value_label')}</span>
            <input
              class="input input-bordered"
              placeholder={t('frontend/src/lib/AdminPanel.svelte::system_variable_value_placeholder')}
              bind:value={variableValue}
            />
          </label>
          <div class="flex gap-2">
            <button class="btn btn-primary btn-sm" type="submit">
              {editingVariableKey ? t('frontend/src/lib/AdminPanel.svelte::save_button') : t('frontend/src/lib/AdminPanel.svelte::add_button')}
            </button>
            {#if editingVariableKey}
              <button class="btn btn-ghost btn-sm" type="button" on:click={resetVariableForm}>
                {t('frontend/src/lib/AdminPanel.svelte::cancel_button')}
              </button>
            {/if}
          </div>
        </form>
        <div class="overflow-x-auto">
          <table class="table table-zebra">
            <thead>
              <tr>
                <th>{t('frontend/src/lib/AdminPanel.svelte::system_variable_key_label')}</th>
                <th>{t('frontend/src/lib/AdminPanel.svelte::system_variable_value_label')}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#if loadingVariables}
                <tr><td colspan="3"><i>{t('frontend/src/lib/AdminPanel.svelte::system_variables_loading')}</i></td></tr>
              {:else}
                {#each systemVariables as variable}
                  <tr>
                    <td class="font-mono text-sm">{variable.key}</td>
                    <td class="font-mono text-sm max-w-[22rem] truncate" title={variable.value}>{variable.value}</td>
                    <td class="text-right">
                      <div class="flex justify-end gap-1">
                        <button class="btn btn-ghost btn-xs" on:click={() => startEditVariable(variable)}><Edit class="w-4 h-4" /></button>
                        <button class="btn btn-ghost btn-xs text-error" on:click={() => deleteSystemVariable(variable.key)}><Trash2 class="w-4 h-4" /></button>
                      </div>
                    </td>
                  </tr>
                {/each}
                {#if !systemVariables.length}
                  <tr><td colspan="3"><i>{t('frontend/src/lib/AdminPanel.svelte::system_variables_empty')}</i></td></tr>
                {/if}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <div class="card bg-base-100 shadow">
      <div class="card-body space-y-3">
        <h2 class="card-title">{t('frontend/src/lib/AdminPanel.svelte::email_ping_card_title')}</h2>
        <p class="text-sm text-base-content/70">{t('frontend/src/lib/AdminPanel.svelte::email_ping_description')}</p>
        <button class="btn btn-primary btn-sm" on:click={() => { showEmailTools = true; }}>
          <MailCheck class="w-4 h-4" aria-hidden="true" />
          {t('frontend/src/lib/AdminPanel.svelte::email_ping_card_title')}
        </button>
      </div>
    </div>
  </div>
{/if}

{#if showEmailTools}
  <dialog open class="modal">
    <div class="modal-box space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="font-semibold flex items-center gap-2">
          <MailCheck class="w-5 h-5" aria-hidden="true" />
          {t('frontend/src/lib/AdminPanel.svelte::email_ping_card_title')}
        </h3>
        <button class="btn btn-ghost btn-xs" on:click={() => { showEmailTools = false; }}>
          <span class="sr-only">{t('frontend/src/lib/AdminPanel.svelte::cancel_button')}</span>
          x
        </button>
      </div>
      <p class="text-sm text-base-content/70">{t('frontend/src/lib/AdminPanel.svelte::email_ping_description')}</p>
      <form class="space-y-2" on:submit|preventDefault={sendEmailPing}>
        <input type="email" class="input input-bordered w-full" placeholder={t('frontend/src/lib/AdminPanel.svelte::email_ping_placeholder')} bind:value={emailPingTarget} required />
        <button class="btn btn-primary btn-sm" disabled={sendingEmailPing}>
          {sendingEmailPing ? t('frontend/src/lib/AdminPanel.svelte::sending_button') : t('frontend/src/lib/AdminPanel.svelte::email_ping_button')}
        </button>
      </form>
      <div class="modal-action">
        <button class="btn btn-ghost" on:click={() => { showEmailTools = false; }}>{t('frontend/src/lib/AdminPanel.svelte::cancel_button')}</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop" on:click={() => { showEmailTools = false; }}><button>close</button></form>
  </dialog>
{/if}

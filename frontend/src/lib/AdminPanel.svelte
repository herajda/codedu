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
  let showSystemVariables = false;

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
  // ───────────────────────────
  // Helper for tab styling
  // ───────────────────────────
  $: tabId = (t: string) => `admin-tab-${t}`;
</script>

<!-- Premium Header -->
<section class="relative overflow-hidden bg-base-100 rounded-3xl border border-base-200 shadow-xl shadow-base-300/30 mb-8 p-6 sm:p-10">
  <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
  <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none"></div>
  <div class="relative flex flex-col md:flex-row items-center gap-6">
    <div class="flex-1 text-center md:text-left">
      <div class="flex items-center justify-center md:justify-start gap-3 mb-2">
        <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center shadow-inner">
          <Shield size={20} />
        </div>
        <h1 class="text-3xl sm:text-4xl font-black tracking-tight">
          {t('frontend/src/lib/AdminPanel.svelte::admin')}
        </h1>
      </div>
      <p class="text-base-content/60 font-medium max-w-xl mx-auto md:mx-0">
        {t('frontend/src/lib/AdminPanel.svelte::admin_subtitle')}
      </p>
    </div>
    
    <!-- Tab Navigation integrated into header area -->
    <div class="flex flex-wrap items-center justify-center gap-2 bg-base-200/50 p-1.5 rounded-[1.5rem] border border-base-200/50 backdrop-blur-sm">
      <button
        type="button"
        role="tab"
        class="px-5 py-2.5 rounded-2xl text-[10px] font-black uppercase tracking-widest transition-all {tab === 'overview' ? 'bg-primary text-primary-content shadow-lg shadow-primary/20 scale-105' : 'hover:bg-base-200'}"
        aria-selected={tab === 'overview'}
        on:click={() => tab='overview'}
      >
        {t('frontend/src/lib/AdminPanel.svelte::overview_tab')}
      </button>
      <button
        type="button"
        role="tab"
        class="px-5 py-2.5 rounded-2xl text-[10px] font-black uppercase tracking-widest transition-all {tab === 'people' ? 'bg-primary text-primary-content shadow-lg shadow-primary/20 scale-105' : 'hover:bg-base-200'}"
        aria-selected={tab === 'people'}
        on:click={() => tab='people'}
      >
        {t('frontend/src/lib/AdminPanel.svelte::people_tab')}
        <span class="ml-1.5 opacity-60">({users.length})</span>
      </button>
      <button
        type="button"
        role="tab"
        class="px-5 py-2.5 rounded-2xl text-[10px] font-black uppercase tracking-widest transition-all {tab === 'classes' ? 'bg-primary text-primary-content shadow-lg shadow-primary/20 scale-105' : 'hover:bg-base-200'}"
        aria-selected={tab === 'classes'}
        on:click={() => tab='classes'}
      >
        {t('frontend/src/lib/AdminPanel.svelte::classes_tab')}
        <span class="ml-1.5 opacity-60">({classes.length})</span>
      </button>
      <button
        type="button"
        role="tab"
        class="px-5 py-2.5 rounded-2xl text-[10px] font-black uppercase tracking-widest transition-all {tab === 'settings' ? 'bg-primary text-primary-content shadow-lg shadow-primary/20 scale-105' : 'hover:bg-base-200'}"
        aria-selected={tab === 'settings'}
        on:click={() => tab='settings'}
      >
        {t('frontend/src/lib/AdminPanel.svelte::settings_tab')}
      </button>
    </div>
  </div>
</section>

{#if ok}
  <div class="alert alert-success bg-success/10 border-success/20 text-success rounded-2xl mb-8 shadow-sm">
    <Check class="w-5 h-5 shrink-0" aria-hidden="true" /> 
    <span class="font-bold">{ok}</span>
  </div>
{/if}
{#if err}
  <div class="alert alert-error bg-error/10 border-error/20 text-error rounded-2xl mb-8 shadow-sm">
    <div class="flex-1 flex gap-2 items-center">
      <span class="font-bold">{err}</span>
    </div>
  </div>
{/if}


{#if tab === 'overview'}
  <!-- Platform Stats Grid -->
  <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6 mb-8">
    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-primary/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{t('frontend/src/lib/AdminPanel.svelte::users_stat_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center group-hover:bg-primary group-hover:text-primary-content transition-all duration-300">
          <Users2 size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{users.length}</div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-primary/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{t('frontend/src/lib/AdminPanel.svelte::teachers_stat_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center group-hover:bg-primary group-hover:text-primary-content transition-all duration-300">
          <GraduationCap size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{teachers.length}</div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-primary/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{t('frontend/src/lib/AdminPanel.svelte::students_stat_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center group-hover:bg-primary group-hover:text-primary-content transition-all duration-300">
          <Users2 size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{students.length}</div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-primary/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{t('frontend/src/lib/AdminPanel.svelte::classes_stat_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center group-hover:bg-primary group-hover:text-primary-content transition-all duration-300">
          <School size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{classes.length}</div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-success/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{t('frontend/src/lib/AdminPanel.svelte::online_stat_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-success/10 text-success flex items-center justify-center group-hover:bg-success group-hover:text-success-content transition-all duration-300">
          <Users2 size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{onlineUsers.length}</div>
      </div>
    </div>

  </section>

  <div class="grid gap-8 xl:grid-cols-12">
    <!-- Quick Actions -->
    <div class="xl:col-span-8 space-y-6">
      <div class="flex items-center justify-between px-2">
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/lib/AdminPanel.svelte::quick_actions_title')}</h2>
      </div>
      
      <div class="grid gap-4 sm:grid-cols-2">
        <button on:click={() => { tab='people'; showCreateUsers = true; }} class="flex items-center gap-4 p-6 bg-base-100 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 transition-all text-left relative overflow-hidden group">
          <div class="absolute top-0 right-0 w-24 h-24 bg-primary/5 rounded-bl-full pointer-events-none group-hover:scale-110 transition-transform"></div>
          <div class="w-12 h-12 rounded-2xl bg-primary/10 text-primary flex items-center justify-center shrink-0">
             <Plus size={24} />
          </div>
          <div class="min-w-0">
            <div class="font-black text-lg tracking-tight group-hover:text-primary transition-colors">{t('frontend/src/lib/AdminPanel.svelte::add_teacher_button')}</div>
            <div class="text-xs opacity-50">{t('frontend/src/lib/AdminPanel.svelte::add_teacher_desc')}</div>
          </div>
        </button>

        <button on:click={() => { tab='classes'; showCreateClass = true; }} class="flex items-center gap-4 p-6 bg-base-100 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 transition-all text-left relative overflow-hidden group">
          <div class="absolute top-0 right-0 w-24 h-24 bg-success/5 rounded-bl-full pointer-events-none group-hover:scale-110 transition-transform"></div>
          <div class="w-12 h-12 rounded-2xl bg-success/10 text-success flex items-center justify-center shrink-0">
             <School size={24} />
          </div>
          <div class="min-w-0">
            <div class="font-black text-lg tracking-tight group-hover:text-success transition-colors">{t('frontend/src/lib/AdminPanel.svelte::create_class_button')}</div>
            <div class="text-xs opacity-50">{t('frontend/src/lib/AdminPanel.svelte::create_class_desc')}</div>
          </div>
        </button>

        <button on:click={() => { showEmailTools = true; tab = 'settings'; }} class="flex items-center gap-4 p-6 bg-base-100 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 transition-all text-left relative overflow-hidden group">
          <div class="absolute top-0 right-0 w-24 h-24 bg-warning/5 rounded-bl-full pointer-events-none group-hover:scale-110 transition-transform"></div>
          <div class="w-12 h-12 rounded-2xl bg-warning/10 text-warning flex items-center justify-center shrink-0">
             <MailCheck size={24} />
          </div>
          <div class="min-w-0">
            <div class="font-black text-lg tracking-tight group-hover:text-warning transition-colors">{t('frontend/src/lib/AdminPanel.svelte::email_ping_card_title')}</div>
            <div class="text-xs opacity-50">{t('frontend/src/lib/AdminPanel.svelte::email_ping_desc')}</div>
          </div>
        </button>

        <a href="/dashboard" class="flex items-center gap-4 p-6 bg-base-100 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 transition-all text-left relative overflow-hidden group no-underline text-current">
          <div class="absolute top-0 right-0 w-24 h-24 bg-info/5 rounded-bl-full pointer-events-none group-hover:scale-110 transition-transform"></div>
          <div class="w-12 h-12 rounded-2xl bg-info/10 text-info flex items-center justify-center shrink-0">
             <RefreshCw size={24} />
          </div>
          <div class="min-w-0">
            <div class="font-black text-lg tracking-tight group-hover:text-info transition-colors">{t('frontend/src/lib/AdminPanel.svelte::go_to_dashboard_button')}</div>
            <div class="text-xs opacity-50">{t('frontend/src/lib/AdminPanel.svelte::go_to_dashboard_desc')}</div>
          </div>
        </a>
      </div>
    </div>

    <!-- Online Users Sidebar -->
    <div class="xl:col-span-4 space-y-6">
      <div class="flex items-center justify-between px-2">
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/lib/AdminPanel.svelte::online_users_card_title')}</h2>
        <button class="btn btn-ghost btn-xs opacity-50 hover:opacity-100 transition-all font-black text-[10px] uppercase tracking-widest h-8" on:click={loadOnlineUsers} aria-label={t('frontend/src/lib/AdminPanel.svelte::refresh_online_users_button')}>
          <RefreshCw class="w-3.5 h-3.5 mr-1" /> {t('frontend/src/lib/AdminPanel.svelte::refresh_button')}
        </button>
      </div>

      <div class="bg-base-100 rounded-[2.5rem] border border-base-200 shadow-sm overflow-hidden p-3">
        <div class="space-y-1 max-h-[400px] overflow-y-auto custom-scrollbar p-1">
          {#each onlineUsers as online}
            <div class="flex items-center gap-4 p-3 rounded-[1.5rem] hover:bg-base-200 transition-colors group">
              <div class="relative shrink-0">
                <div class="w-10 h-10 rounded-xl overflow-hidden ring-2 ring-base-200">
                  {#if online.avatar}
                    <img src={online.avatar} alt={online.name} class="w-full h-full object-cover" />
                  {:else}
                    <div class="w-full h-full bg-primary/10 text-primary flex items-center justify-center font-black text-sm">
                      {(online.name || online.email || '?').charAt(0).toUpperCase()}
                    </div>
                  {/if}
                </div>
                <div class="absolute -bottom-0.5 -right-0.5 w-3 h-3 bg-success rounded-full border-2 border-base-100"></div>
              </div>
              <div class="min-w-0 flex-1">
                <div class="font-black text-sm truncate group-hover:text-primary transition-colors">{online.name || online.email || '?'}</div>
                {#if online.name}
                  <div class="text-[10px] font-bold opacity-40 truncate">{online.email}</div>
                {/if}
              </div>
            </div>
          {:else}
            <div class="py-10 text-center space-y-3">
              <div class="w-10 h-10 rounded-full bg-base-200 flex items-center justify-center mx-auto opacity-30">
                 <Users2 size={18} />
              </div>
              <p class="text-[10px] font-black opacity-30 uppercase tracking-widest">{t('frontend/src/lib/AdminPanel.svelte::no_online_users_message')}</p>
            </div>
          {/each}
        </div>
      </div>
    </div>
  </div>
{/if}

<ConfirmModal bind:this={confirmModal} />
<PromptModal bind:this={promptModal} />

{#if tab === 'people'}
  <div class="grid gap-8 xl:grid-cols-12">
    <!-- Account Tools Side -->
    <div class="xl:col-span-4 space-y-6">
      <div class="flex items-center justify-between px-2">
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/lib/AdminPanel.svelte::account_creation_title')}</h2>
        <button class="btn btn-ghost btn-xs opacity-50 hover:opacity-100 transition-all font-black text-[10px] uppercase tracking-widest h-8" on:click={() => { showCreateUsers = !showCreateUsers; }}>
          {showCreateUsers ? t('frontend/src/lib/AdminPanel.svelte::hide_account_tools_button') : t('frontend/src/lib/AdminPanel.svelte::show_account_tools_button')}
        </button>
      </div>

      {#if showCreateUsers}
        <div class="space-y-4">
          <div class="bg-base-100 p-6 rounded-[2rem] border border-base-200 shadow-sm space-y-4">
            <h3 class="font-black text-sm uppercase tracking-widest opacity-40 flex items-center gap-2">
              <GraduationCap size={16} /> {t('frontend/src/lib/AdminPanel.svelte::add_teacher_card_title')}
            </h3>
            <form on:submit|preventDefault={addTeacher} class="space-y-3">
              <input type="email" bind:value={teacherEmail} placeholder={t('frontend/src/lib/AdminPanel.svelte::email_label')} required class="input input-sm bg-base-200/50 w-full rounded-xl focus:outline-none focus:ring-1 focus:ring-primary h-10 font-bold" />
              <input type="password" bind:value={teacherPassword} placeholder={t('frontend/src/lib/AdminPanel.svelte::password_label')} required class="input input-sm bg-base-200/50 w-full rounded-xl focus:outline-none focus:ring-1 focus:ring-primary h-10 font-bold" />
              <button class="btn btn-primary btn-sm w-full h-10 rounded-xl font-black uppercase tracking-widest text-[10px]">{t('frontend/src/lib/AdminPanel.svelte::add_button')}</button>
            </form>
          </div>

          <div class="bg-base-100 p-6 rounded-[2rem] border border-base-200 shadow-sm space-y-4">
            <h3 class="font-black text-sm uppercase tracking-widest opacity-40 flex items-center gap-2">
              <Users2 size={16} /> {t('frontend/src/lib/AdminPanel.svelte::add_student_card_title')}
            </h3>
            <form on:submit|preventDefault={addStudent} class="space-y-3">
              <input type="text" bind:value={studentName} placeholder={t('frontend/src/lib/AdminPanel.svelte::student_name_placeholder')} class="input input-sm bg-base-200/50 w-full rounded-xl focus:outline-none focus:ring-1 focus:ring-primary h-10 font-bold" />
              <input type="email" bind:value={studentEmail} placeholder={t('frontend/src/lib/AdminPanel.svelte::email_label')} required class="input input-sm bg-base-200/50 w-full rounded-xl focus:outline-none focus:ring-1 focus:ring-primary h-10 font-bold" />
              <input type="password" bind:value={studentPassword} placeholder={t('frontend/src/lib/AdminPanel.svelte::password_label')} required class="input input-sm bg-base-200/50 w-full rounded-xl focus:outline-none focus:ring-1 focus:ring-primary h-10 font-bold" />
              <button class="btn btn-primary btn-sm w-full h-10 rounded-xl font-black uppercase tracking-widest text-[10px]">{t('frontend/src/lib/AdminPanel.svelte::add_button')}</button>
            </form>
          </div>
        </div>
      {/if}

      <div class="flex items-center justify-between px-2 mt-8">
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/lib/AdminPanel.svelte::whitelist_title')}</h2>
      </div>
      <div class="bg-base-100 p-6 rounded-[2rem] border border-base-200 shadow-sm space-y-4">
        <p class="text-[10px] font-bold opacity-50 uppercase tracking-widest leading-relaxed">
          {t('frontend/src/lib/AdminPanel.svelte::whitelist_description')}
        </p>
        <form on:submit|preventDefault={addToWhitelist} class="flex gap-2">
          <input type="email" bind:value={whitelistEmail} placeholder={t('frontend/src/lib/AdminPanel.svelte::whitelist_placeholder')} required class="input input-sm bg-base-200/50 flex-1 rounded-xl focus:outline-none focus:ring-1 focus:ring-primary h-10 font-bold" />
          <button class="btn btn-primary btn-sm h-10 rounded-xl font-black uppercase tracking-widest text-[10px] px-4">{t('frontend/src/lib/AdminPanel.svelte::add_button')}</button>
        </form>
        <div class="overflow-x-auto max-h-60 custom-scrollbar mt-4 bg-base-200/20 rounded-2xl border border-base-200/50">
          <table class="table table-sm">
            <tbody>
              {#each whitelist as w}
                <tr class="hover:bg-base-200/50 transition-colors border-base-300/30">
                  <td class="font-bold text-xs py-3">{w.email}</td>
                  <td class="text-right py-3">
                    <button class="btn btn-ghost btn-xs text-error opacity-40 hover:opacity-100 transition-opacity" on:click={() => removeFromWhitelist(w.email)} aria-label={t('frontend/src/lib/AdminPanel.svelte::remove_button')}>
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              {:else}
                <tr><td colspan="2" class="text-center py-8 italic opacity-30 text-xs font-bold uppercase tracking-widest">{t('frontend/src/lib/AdminPanel.svelte::whitelist_empty')}</td></tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- Main Users List -->
    <div class="xl:col-span-8 space-y-6">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 px-2">
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/lib/AdminPanel.svelte::users_card_title')}</h2>
        <div class="flex items-center gap-2">
          <label class="relative group">
            <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 opacity-30 group-focus-within:opacity-100 transition-opacity" />
            <input 
              class="input input-sm bg-base-100 border-base-200 rounded-xl pl-10 focus:ring-1 focus:ring-primary w-full sm:w-64 h-9 font-bold text-xs" 
              placeholder={t('frontend/src/lib/AdminPanel.svelte::search_users_placeholder')} 
              bind:value={userQuery} 
            />
          </label>
          <button class="btn btn-ghost btn-xs opacity-50 hover:opacity-100 transition-all font-black text-[10px] uppercase tracking-widest h-9 border border-base-200 rounded-xl px-3" on:click={exportUsersCSV}>
             {t('frontend/src/lib/AdminPanel.svelte::export_csv_button')}
          </button>
          <button class="btn btn-ghost btn-xs opacity-50 hover:opacity-100 transition-all font-black text-[10px] uppercase tracking-widest h-9 border border-base-200 rounded-xl" on:click={refreshUsers} aria-label={t('frontend/src/lib/AdminPanel.svelte::refresh_online_users_button')}>
             <RefreshCw size={14} />
          </button>
        </div>
      </div>

      <div class="bg-base-100 rounded-[2.5rem] border border-base-200 shadow-sm overflow-hidden">
        <div class="p-4 border-b border-base-200 bg-base-200/20 flex flex-wrap items-center gap-1.5">
          <button 
            class="px-3 py-1.5 rounded-lg text-[9px] font-black uppercase tracking-widest transition-all {userRoleFilter === 'all' ? 'bg-primary text-primary-content' : 'hover:bg-base-200 text-base-content/60'}" 
            on:click={() => userRoleFilter = 'all'}
          >
            {t('frontend/src/lib/AdminPanel.svelte::all_filter_option')}
          </button>
          <button 
            class="px-3 py-1.5 rounded-lg text-[9px] font-black uppercase tracking-widest transition-all {userRoleFilter === 'student' ? 'bg-primary text-primary-content' : 'hover:bg-base-200 text-base-content/60'}" 
            on:click={() => userRoleFilter = 'student'}
          >
            {t('frontend/src/lib/AdminPanel.svelte::role_filter_students')}
          </button>
          <button 
            class="px-3 py-1.5 rounded-lg text-[9px] font-black uppercase tracking-widest transition-all {userRoleFilter === 'teacher' ? 'bg-primary text-primary-content' : 'hover:bg-base-200 text-base-content/60'}" 
            on:click={() => userRoleFilter = 'teacher'}
          >
            {t('frontend/src/lib/AdminPanel.svelte::role_filter_teachers')}
          </button>
          <button 
            class="px-3 py-1.5 rounded-lg text-[9px] font-black uppercase tracking-widest transition-all {userRoleFilter === 'admin' ? 'bg-primary text-primary-content' : 'hover:bg-base-200 text-base-content/60'}" 
            on:click={() => userRoleFilter = 'admin'}
          >
            {t('frontend/src/lib/AdminPanel.svelte::role_filter_admins')}
          </button>
        </div>

        <div class="overflow-x-auto custom-scrollbar">
          <table class="table table-zebra table-md">
            <thead>
              <tr class="border-base-200 bg-base-100/50">
                <th class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::name_table_header')}</th>
                <th class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::role_table_header')}</th>
                <th class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::auth_table_header')}</th>
                <th class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::created_table_header')}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each filteredUsers as u}
                <tr class="border-base-200 group hover:bg-base-200/30 transition-colors">
                  <td class="py-4">
                    <div class="flex items-center gap-4">
                      <div class="relative">
                        <div class="w-10 h-10 rounded-xl bg-primary/5 flex items-center justify-center font-black text-primary border border-primary/10">
                           {(u.name || u.email || '?').charAt(0).toUpperCase()}
                        </div>
                        {#if onlineIds.has(u.id)}
                          <div class="absolute -bottom-0.5 -right-0.5 w-3 h-3 bg-success rounded-full border-2 border-base-100 shadow-sm animate-pulse"></div>
                        {/if}
                      </div>
                      <div class="flex flex-col min-w-0">
                        <span class="font-black text-sm truncate">{userPrimary(u)}</span>
                        {#if userSecondary(u)}
                          <span class="text-[10px] font-bold opacity-40 truncate">{userSecondary(u)}</span>
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
                    <div class="flex flex-wrap gap-1.5">
                      {#if hasEmailLogin(u)}
                        <span class="badge badge-ghost border-none font-black text-[9px] uppercase tracking-widest px-2 h-5 bg-base-200/50 text-base-content/60">{t('frontend/src/lib/AdminPanel.svelte::auth_email_label')}</span>
                      {/if}
                      {#if u.ms_oid}
                        <span class="badge badge-info border-none font-black text-[9px] uppercase tracking-widest px-2 h-5 bg-info/10 text-info">{t('frontend/src/lib/AdminPanel.svelte::auth_microsoft_label')}</span>
                      {/if}
                      {#if u.bk_uid}
                        <span class="badge badge-warning border-none font-black text-[9px] uppercase tracking-widest px-2 h-5 bg-warning/10 text-warning">{t('frontend/src/lib/AdminPanel.svelte::auth_bakalari_label')}</span>
                      {/if}
                      {#if u.role === 'teacher' && teacherIdToClassCount[u.id]}
                        <span class="badge badge-primary border-none font-black text-[9px] uppercase tracking-widest px-2 h-5 bg-primary/10 text-primary">
                          {teacherIdToClassCount[u.id]} {t('frontend/src/lib/AdminPanel.svelte::classes_table_header')}
                        </span>
                      {/if}
                    </div>
                  </td>
                  <td class="text-xs font-bold opacity-50 whitespace-nowrap">{formatDate(u.created_at)}</td>

                  <td class="text-right">
                    <div class="flex justify-end gap-1 opacity-20 group-hover:opacity-100 transition-opacity">
                      <button
                        class="btn btn-ghost btn-xs text-info hover:bg-info/10"
                        title={u.bk_uid ? t('frontend/src/lib/AdminPanel.svelte::set_password_disabled_tooltip') : t('frontend/src/lib/AdminPanel.svelte::set_password_button_label')}
                        disabled={Boolean(u.bk_uid)}
                        on:click={() => { if (!u.bk_uid) promptSetPassword(u); }}
                      >
                        <KeyRound size={14} />
                      </button>
                      <button class="btn btn-ghost btn-xs text-error hover:bg-error/10" on:click={()=>deleteUser(u.id)} aria-label={t('frontend/src/lib/AdminPanel.svelte::delete_button')}>
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              {:else}
                <tr><td colspan="5" class="text-center py-20 italic opacity-30 font-black uppercase tracking-widest text-xs">{t('frontend/src/lib/AdminPanel.svelte::no_users_message')}</td></tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
{/if}


{#if tab === 'classes'}
  <div class="space-y-6">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 px-2">
      <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/lib/AdminPanel.svelte::classes_tab')}</h2>
      <div class="flex items-center gap-2">
        <label class="relative group">
          <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 opacity-30 group-focus-within:opacity-100 transition-opacity" />
          <input 
            class="input input-sm bg-base-100 border-base-200 rounded-xl pl-10 focus:ring-1 focus:ring-primary w-full sm:w-64 h-9 font-bold text-xs" 
            placeholder={t('frontend/src/lib/AdminPanel.svelte::search_classes_placeholder')} 
            bind:value={classQuery} 
          />
        </label>
        <button class="btn btn-primary btn-sm h-9 rounded-xl font-black uppercase tracking-widest text-[10px] px-4 gap-2" on:click={() => showCreateClass = true}>
           <Plus size={14} /> {t('frontend/src/lib/AdminPanel.svelte::new_button')}
        </button>
        <button class="btn btn-ghost btn-xs opacity-50 hover:opacity-100 transition-all font-black text-[10px] uppercase tracking-widest h-9 border border-base-200 rounded-xl" on:click={loadClasses} aria-label={t('frontend/src/lib/AdminPanel.svelte::refresh_online_users_button')}>
           <RefreshCw size={14} />
        </button>
      </div>
    </div>

    <div class="bg-base-100 rounded-[2.5rem] border border-base-200 shadow-sm overflow-hidden">
      <div class="overflow-x-auto custom-scrollbar">
        <table class="table table-zebra table-md">
          <thead>
            <tr class="border-base-200 bg-base-100/50">
              <th class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::class_table_header')}</th>
              <th class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::teacher_table_header')}</th>
              <th class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::created_table_header')}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each filteredClasses as c}
              <tr class="border-base-200 group hover:bg-base-200/30 transition-colors">
                <td class="py-4">
                  <div class="flex items-center gap-4">
                    <div class="w-10 h-10 rounded-xl bg-primary/5 flex items-center justify-center text-primary border border-primary/10">
                       <School size={18} />
                    </div>
                    <a href={`/classes/${c.id}`} class="font-black text-sm hover:text-primary transition-colors no-underline text-current">{c.name}</a>
                  </div>
                </td>
                <td>
                  {#if teacherLookup[c.teacher_id]}
                    <div class="flex items-center gap-3">
                      <div class="w-7 h-7 rounded-lg bg-base-200 flex items-center justify-center font-black text-[10px] text-base-content/40">
                         {(userPrimary(teacherLookup[c.teacher_id])).charAt(0).toUpperCase()}
                      </div>
                      <div class="flex flex-col min-w-0">
                        <span class="font-bold text-xs truncate">{userPrimary(teacherLookup[c.teacher_id])}</span>
                        {#if userSecondary(teacherLookup[c.teacher_id])}
                          <span class="text-[9px] font-bold opacity-40 truncate">{userSecondary(teacherLookup[c.teacher_id])}</span>
                        {/if}
                      </div>
                    </div>
                  {:else}
                    <span class="text-[10px] font-black uppercase tracking-widest opacity-30 italic">{t('frontend/src/lib/AdminPanel.svelte::unassigned_teacher_label')}</span>
                  {/if}
                </td>
                <td class="text-xs font-bold opacity-50">{formatDate(c.created_at)}</td>
                <td class="text-right whitespace-nowrap">
                  <div class="flex justify-end gap-1 opacity-20 group-hover:opacity-100 transition-opacity">
                    <button class="btn btn-ghost btn-xs text-info hover:bg-info/10" on:click={()=>renameClass(c.id)} aria-label={t('frontend/src/lib/AdminPanel.svelte::rename_button')}>
                      <Edit size={14} />
                    </button>
                    <button class="btn btn-ghost btn-xs text-primary hover:bg-primary/10" on:click={()=>transferTarget={ id: c.id, name: c.name, to: null }} aria-label={t('frontend/src/lib/AdminPanel.svelte::transfer_button')}>
                      <ArrowRightLeft size={14} />
                    </button>
                    <button class="btn btn-ghost btn-xs text-error hover:bg-error/10" on:click={()=>deleteClassAction(c.id)} aria-label={t('frontend/src/lib/AdminPanel.svelte::delete_button')}>
                      <Trash2 size={14} />
                    </button>
                  </div>
                </td>
              </tr>
            {:else}
              <tr><td colspan="4" class="text-center py-20 italic opacity-30 font-black uppercase tracking-widest text-xs">{t('frontend/src/lib/AdminPanel.svelte::no_classes_message')}</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  </div>

  {#if showCreateClass}
    <dialog open class="modal">
      <div class="modal-box bg-base-100 rounded-[2.5rem] border border-base-200 shadow-2xl p-8 max-w-md">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center">
            <Plus size={20} />
          </div>
          <h3 class="text-xl font-black tracking-tight">{t('frontend/src/lib/AdminPanel.svelte::create_class_modal_title')}</h3>
        </div>
        
        <div class="space-y-5">
          <div class="space-y-2">
            <label class="text-[10px] font-black uppercase tracking-widest opacity-40 ml-1" for="new-class-name">{t('frontend/src/lib/AdminPanel.svelte::class_name_placeholder')}</label>
            <input 
              id="new-class-name"
              class="input input-sm bg-base-200/50 w-full rounded-xl focus:outline-none focus:ring-1 focus:ring-primary h-12 font-bold px-4" 
              placeholder={t('frontend/src/lib/AdminPanel.svelte::class_name_placeholder')} 
              bind:value={newClassName} 
            />
          </div>

          <div class="space-y-2">
            <label class="text-[10px] font-black uppercase tracking-widest opacity-40 ml-1">{t('frontend/src/lib/AdminPanel.svelte::teacher_table_header')}</label>
            <div class="relative">
              <CustomSelect 
                searchable
                options={teacherOptions} 
                bind:value={newClassTeacherId} 
                placeholder={t('frontend/src/lib/AdminPanel.svelte::select_teacher_option')}
              />
            </div>
          </div>
        </div>

        <div class="modal-action gap-2 mt-8">
          <button class="btn btn-ghost rounded-xl font-black uppercase tracking-widest text-[10px] h-11" on:click={() => { showCreateClass = false; }}>{t('frontend/src/lib/AdminPanel.svelte::cancel_button')}</button>
          <button class="btn btn-primary rounded-xl px-8 font-black uppercase tracking-widest text-[10px] h-11 shadow-lg shadow-primary/20" on:click={createClass} disabled={!newClassName.trim() || !newClassTeacherId}>
            {t('frontend/src/lib/AdminPanel.svelte::create_button')}
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop bg-base-content/20 backdrop-blur-sm" on:click={() => showCreateClass = false}><button>close</button></form>
    </dialog>
  {/if}

  {#if transferTarget}
    <dialog open class="modal">
      <div class="modal-box bg-base-100 rounded-[2.5rem] border border-base-200 shadow-2xl p-8 max-w-md">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center">
            <ArrowRightLeft size={20} />
          </div>
          <h3 class="text-xl font-black tracking-tight">{t('frontend/src/lib/AdminPanel.svelte::transfer_ownership_modal_title')}</h3>
        </div>

        <p class="text-sm font-medium opacity-60 mb-6 leading-relaxed">
           {t('frontend/src/lib/AdminPanel.svelte::transfer_ownership_modal_body', { className: transferTarget.name || t('frontend/src/lib/AdminPanel.svelte::class_table_header') })}
        </p>
        
        <div class="space-y-2">
          <label class="text-[10px] font-black uppercase tracking-widest opacity-40 ml-1">{t('frontend/src/lib/AdminPanel.svelte::select_teacher_option')}</label>
          <CustomSelect 
            searchable
            options={teacherOptions} 
            bind:value={transferTarget.to} 
            placeholder={t('frontend/src/lib/AdminPanel.svelte::select_teacher_option')}
          />
        </div>

        <div class="modal-action gap-2 mt-8">
          <button class="btn btn-ghost rounded-xl font-black uppercase tracking-widest text-[10px] h-11" on:click={() => transferTarget = null}>{t('frontend/src/lib/AdminPanel.svelte::cancel_button')}</button>
          <button class="btn btn-primary rounded-xl px-8 font-black uppercase tracking-widest text-[10px] h-11 shadow-lg shadow-primary/20" on:click={transferClass} disabled={!transferTarget.to}>
            {t('frontend/src/lib/AdminPanel.svelte::transfer_button')}
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop bg-base-content/20 backdrop-blur-sm" on:click={() => transferTarget = null}><button>close</button></form>
    </dialog>
  {/if}
{/if}


{#if tab === 'settings'}
  <div class="space-y-8">
    <!-- General System Settings -->
    <section class="space-y-6">
      <div class="flex items-center justify-between px-2">
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/lib/AdminPanel.svelte::system_settings_title')}</h2>
      </div>
      
      <div class="grid gap-4 md:grid-cols-2">
        <label class="bg-base-100 p-6 rounded-[2rem] border border-base-200 shadow-sm flex items-center justify-between gap-6 group hover:border-primary/30 transition-all cursor-pointer">
          <div class="flex-1">
            <span class="font-black text-sm uppercase tracking-widest block mb-1">{t('frontend/src/lib/AdminPanel.svelte::force_bakalari_email_label')}</span>
            <span class="text-[10px] font-bold opacity-50 uppercase tracking-widest leading-relaxed">{t('frontend/src/lib/AdminPanel.svelte::force_bakalari_email_description')}</span>
          </div>
          <input type="checkbox" class="toggle toggle-primary toggle-lg scale-75" checked={forceBakalariEmail} on:change={(e) => updateSetting('force_bakalari_email', (e.target as HTMLInputElement).checked)} />
        </label>

        <label class="bg-base-100 p-6 rounded-[2rem] border border-base-200 shadow-sm flex items-center justify-between gap-6 group hover:border-primary/30 transition-all cursor-pointer">
          <div class="flex-1">
            <span class="font-black text-sm uppercase tracking-widest block mb-1">{t('frontend/src/lib/AdminPanel.svelte::allow_microsoft_login_label')}</span>
            <span class="text-[10px] font-bold opacity-50 uppercase tracking-widest leading-relaxed">{t('frontend/src/lib/AdminPanel.svelte::allow_microsoft_login_description')}</span>
          </div>
          <input type="checkbox" class="toggle toggle-primary toggle-lg scale-75" checked={allowMicrosoftLogin} on:change={(e) => updateSetting('allow_microsoft_login', (e.target as HTMLInputElement).checked)} />
        </label>
      </div>
    </section>

    <!-- System Variables -->
    <section class="space-y-6">
      <div class="flex items-center justify-between px-2">
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/lib/AdminPanel.svelte::system_variables_title')}</h2>
        <button class="btn btn-ghost btn-xs opacity-50 hover:opacity-100 transition-all font-black text-[10px] uppercase tracking-widest h-8" on:click={() => { showSystemVariables = !showSystemVariables; }}>
          {showSystemVariables ? t('frontend/src/lib/AdminPanel.svelte::hide_system_variables_button') : t('frontend/src/lib/AdminPanel.svelte::show_system_variables_button')}
        </button>
      </div>

      {#if showSystemVariables}
        <div class="bg-base-100 rounded-[2.5rem] border border-base-200 shadow-sm overflow-hidden">
          <div class="p-6 border-b border-base-200 bg-base-200/20">
            <p class="text-[10px] font-bold opacity-50 uppercase tracking-widest leading-relaxed mb-6">
              {t('frontend/src/lib/AdminPanel.svelte::system_variables_description')}
            </p>
            
            <form on:submit|preventDefault={saveSystemVariable} class="grid gap-4 md:grid-cols-[1fr_2fr_auto] items-end">
              <div class="space-y-1.5">
                <label class="text-[10px] font-black uppercase tracking-widest opacity-40 ml-1" for="var-key">{t('frontend/src/lib/AdminPanel.svelte::system_variable_key_label')}</label>
                <input 
                  id="var-key"
                  class="input input-sm bg-base-100 border-base-200 w-full rounded-xl focus:ring-1 focus:ring-primary h-11 font-bold px-4" 
                  placeholder={t('frontend/src/lib/AdminPanel.svelte::system_variable_key_placeholder')} 
                  bind:value={variableKey} 
                  disabled={Boolean(editingVariableKey)}
                  required
                />
              </div>
              <div class="space-y-1.5">
                <label class="text-[10px] font-black uppercase tracking-widest opacity-40 ml-1" for="var-val">{t('frontend/src/lib/AdminPanel.svelte::system_variable_value_label')}</label>
                <input 
                  id="var-val"
                  class="input input-sm bg-base-100 border-base-200 w-full rounded-xl focus:ring-1 focus:ring-primary h-11 font-bold px-4" 
                  placeholder={t('frontend/src/lib/AdminPanel.svelte::system_variable_value_placeholder')} 
                  bind:value={variableValue} 
                />
              </div>
              <div class="flex gap-2">
                <button class="btn btn-primary h-11 rounded-xl px-6 font-black uppercase tracking-widest text-[10px]" type="submit">
                  {editingVariableKey ? t('frontend/src/lib/AdminPanel.svelte::save_button') : t('frontend/src/lib/AdminPanel.svelte::add_button')}
                </button>
                {#if editingVariableKey}
                  <button class="btn btn-ghost h-11 rounded-xl font-black uppercase tracking-widest text-[10px]" type="button" on:click={resetVariableForm}>
                    {t('frontend/src/lib/AdminPanel.svelte::cancel_button')}
                  </button>
                {/if}
              </div>
            </form>
          </div>

          <div class="overflow-x-auto custom-scrollbar">
            <table class="table table-zebra table-md">
              <thead>
                <tr class="border-base-200 bg-base-100/50">
                  <th class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::system_variable_key_label')}</th>
                  <th class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::system_variable_value_label')}</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {#if loadingVariables}
                  <tr><td colspan="3" class="text-center py-12"><span class="loading loading-dots loading-md text-primary/30"></span></td></tr>
                {:else}
                  {#each systemVariables as variable}
                    <tr class="border-base-200 group hover:bg-base-200/30 transition-colors">
                      <td class="font-mono text-xs py-4 font-bold">{variable.key}</td>
                      <td class="font-mono text-xs py-4 text-base-content/60 max-w-[22rem] truncate" title={variable.value}>{variable.value}</td>
                      <td class="text-right">
                        <div class="flex justify-end gap-1 opacity-20 group-hover:opacity-100 transition-opacity">
                          <button class="btn btn-ghost btn-xs text-info hover:bg-info/10" on:click={() => startEditVariable(variable)} aria-label={t('frontend/src/lib/AdminPanel.svelte::edit_button')}><Edit size={14} /></button>
                          <button class="btn btn-ghost btn-xs text-error hover:bg-error/10" on:click={() => deleteSystemVariable(variable.key)} aria-label={t('frontend/src/lib/AdminPanel.svelte::delete_button')}><Trash2 size={14} /></button>
                        </div>
                      </td>
                    </tr>
                  {:else}
                    <tr><td colspan="3" class="text-center py-20 italic opacity-30 font-black uppercase tracking-widest text-xs">{t('frontend/src/lib/AdminPanel.svelte::system_variables_empty')}</td></tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>
        </div>
      {/if}
    </section>

    <!-- Email Tools Section -->
    <section class="space-y-6">
      <div class="flex items-center justify-between px-2">
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/lib/AdminPanel.svelte::email_ping_card_title')}</h2>
      </div>
      
      <div class="bg-base-100 p-8 rounded-[2rem] border border-base-200 shadow-sm flex flex-col md:flex-row items-center justify-between gap-6 group hover:border-primary/30 transition-all">
        <div class="flex items-center gap-6">
          <div class="w-14 h-14 rounded-2xl bg-warning/10 text-warning flex items-center justify-center shrink-0">
             <MailCheck size={28} />
          </div>
          <div class="text-center md:text-left">
            <div class="font-black text-lg tracking-tight mb-1">{t('frontend/src/lib/AdminPanel.svelte::email_ping_card_title')}</div>
            <div class="text-xs font-bold opacity-50 uppercase tracking-widest leading-relaxed">{t('frontend/src/lib/AdminPanel.svelte::email_ping_description')}</div>
          </div>
        </div>
        <button class="btn btn-primary h-12 rounded-xl px-8 font-black uppercase tracking-widest text-[10px] shadow-lg shadow-primary/20" on:click={() => { showEmailTools = true; }}>
          <MailCheck size={16} class="mr-2" />
          {t('frontend/src/lib/AdminPanel.svelte::email_ping_button')}
        </button>
      </div>
    </section>
  </div>
{/if}


{#if showEmailTools}
  <dialog open class="modal">
    <div class="modal-box bg-base-100 rounded-[2.5rem] border border-base-200 shadow-2xl p-8 max-w-md">
      <div class="flex items-center gap-3 mb-6">
        <div class="w-10 h-10 rounded-xl bg-warning/10 text-warning flex items-center justify-center">
          <MailCheck size={20} />
        </div>
        <h3 class="text-xl font-black tracking-tight">{t('frontend/src/lib/AdminPanel.svelte::email_ping_card_title')}</h3>
      </div>
      
      <p class="text-sm font-medium opacity-60 mb-6 leading-relaxed">
         {t('frontend/src/lib/AdminPanel.svelte::email_ping_description')}
      </p>
      
      <form class="space-y-4" on:submit|preventDefault={sendEmailPing}>
        <div class="space-y-1.5">
          <label class="text-[10px] font-black uppercase tracking-widest opacity-40 ml-1" for="ping-email">{t('frontend/src/lib/AdminPanel.svelte::email_label')}</label>
          <input 
            id="ping-email"
            type="email" 
            class="input input-sm bg-base-200/50 w-full rounded-xl focus:outline-none focus:ring-1 focus:ring-primary h-12 font-bold px-4" 
            placeholder={t('frontend/src/lib/AdminPanel.svelte::email_ping_placeholder')} 
            bind:value={emailPingTarget} 
            required 
          />
        </div>
        
        <div class="modal-action gap-2 mt-4">
          <button type="button" class="btn btn-ghost rounded-xl font-black uppercase tracking-widest text-[10px] h-11" on:click={() => { showEmailTools = false; }}>{t('frontend/src/lib/AdminPanel.svelte::cancel_button')}</button>
          <button type="submit" class="btn btn-primary rounded-xl px-8 font-black uppercase tracking-widest text-[10px] h-11 shadow-lg shadow-primary/20" disabled={sendingEmailPing}>
            {sendingEmailPing ? t('frontend/src/lib/AdminPanel.svelte::sending_button') : t('frontend/src/lib/AdminPanel.svelte::email_ping_button')}
          </button>
        </div>
      </form>
    </div>
    <form method="dialog" class="modal-backdrop bg-base-content/20 backdrop-blur-sm" on:click={() => { showEmailTools = false; }}><button>close</button></form>
  </dialog>
{/if}

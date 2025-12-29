<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch, apiJSON } from '$lib/api';
  import { sha256 } from '$lib/hash';
  import { formatDate } from "$lib/date";
  import { classesStore } from '$lib/stores/classes';
  import {
    Users2, GraduationCap, School, Plus, Trash2, RefreshCw,
    Shield, Search, Edit, ArrowRightLeft, Check, KeyRound, MailCheck, ChevronDown
  } from 'lucide-svelte';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import PromptModal from '$lib/components/PromptModal.svelte';
  import CustomSelect from '$lib/components/CustomSelect.svelte';
  import StylishInput from '$lib/components/StylishInput.svelte';
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
  let teacherEmail = '', teacherPassword = '', teacherName = '';
  async function addTeacher() {
    err = ok = '';
    const payload: Record<string, string> = {
      email: teacherEmail,
      password: await sha256(teacherPassword)
    };
    const trimmedName = teacherName.trim();
    if (trimmedName) payload.name = trimmedName;
    const r = await apiFetch('/api/teachers', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (r.status === 201) {
      ok = t('frontend/src/lib/AdminPanel.svelte::teacher_created_success');
      teacherEmail = teacherPassword = teacherName = '';
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
<div class="relative z-10 mb-10">
  <div class="absolute inset-x-0 -top-40 -z-10 h-96 bg-gradient-to-b from-primary/5 to-transparent opacity-60 blur-3xl pointer-events-none"></div>
  
  <div class="flex flex-col lg:flex-row items-center justify-between gap-8">
    <div class="text-center lg:text-left">
      <div class="flex items-center justify-center lg:justify-start gap-4 mb-2">
        <div class="w-12 h-12 rounded-2xl bg-gradient-to-br from-primary via-primary/80 to-primary/50 text-primary-content flex items-center justify-center shadow-lg shadow-primary/20 ring-4 ring-base-100">
          <Shield size={22} class="drop-shadow-sm" />
        </div>
        <div>
          <h1 class="text-4xl font-black tracking-tight text-base-content leading-none mb-1">
            {t('frontend/src/lib/AdminPanel.svelte::admin')}
          </h1>
          <div class="flex items-center gap-2 justify-center lg:justify-start">
            <span class="w-1.5 h-1.5 rounded-full bg-success animate-pulse"></span>
            <p class="text-xs font-bold uppercase tracking-widest text-base-content/40">
              {t('frontend/src/lib/AdminPanel.svelte::admin_subtitle')}
            </p>
          </div>
        </div>
      </div>
    </div>
    
    <!-- Tab Navigation -->
    <div class="bg-base-100/50 backdrop-blur-xl p-2 rounded-[2rem] border border-base-200/50 shadow-xl shadow-base-300/20 flex flex-wrap justify-center gap-1 relative overflow-hidden">
      {#each ['overview', 'people', 'classes', 'settings'] as t_name}
        <button
          type="button"
          class="relative px-6 py-3 rounded-[1.5rem] text-[10px] font-black uppercase tracking-widest transition-all duration-300 z-10 overflow-hidden group
            {tab === t_name ? 'text-primary-content shadow-md shadow-primary/25 translate-y-[-1px]' : 'text-base-content/60 hover:text-base-content hover:bg-base-200/50'}"
          on:click={() => tab = t_name}
        >
          {#if tab === t_name}
             <div class="absolute inset-0 bg-primary bg-gradient-to-br from-primary to-primary/90 transition-all duration-300"></div>
          {/if}
          <span class="relative z-20 flex items-center gap-2">
            {t(`frontend/src/lib/AdminPanel.svelte::${t_name}_tab`)}
            {#if t_name === 'people'}
              <span class="px-1.5 py-0.5 rounded-md bg-base-100/20 text-[9px] backdrop-blur-sm opacity-90">{users.length}</span>
            {/if}
            {#if t_name === 'classes'}
              <span class="px-1.5 py-0.5 rounded-md bg-base-100/20 text-[9px] backdrop-blur-sm opacity-90">{classes.length}</span>
            {/if}
          </span>
        </button>
      {/each}
    </div>
  </div>
</div>

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
  <section class="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-6 gap-4 sm:gap-6 mb-10">
    <div class="bg-base-100 p-5 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 hover:border-primary/20 transition-all duration-300 group">
      <div class="flex flex-col h-full justify-between gap-4">
        <div class="w-10 h-10 rounded-2xl bg-primary/10 text-primary flex items-center justify-center group-hover:scale-110 transition-transform duration-300">
          <Users2 size={20} />
        </div>
        <div>
          <div class="text-[32px] font-black tracking-tight text-base-content leading-none mb-1 group-hover:text-primary transition-colors">{users.length}</div>
          <div class="text-[10px] font-bold uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::users_stat_title')}</div>
        </div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 hover:border-primary/20 transition-all duration-300 group">
      <div class="flex flex-col h-full justify-between gap-4">
        <div class="w-10 h-10 rounded-2xl bg-secondary/10 text-secondary flex items-center justify-center group-hover:scale-110 transition-transform duration-300">
          <GraduationCap size={20} />
        </div>
        <div>
          <div class="text-[32px] font-black tracking-tight text-base-content leading-none mb-1 group-hover:text-secondary transition-colors">{teachers.length}</div>
          <div class="text-[10px] font-bold uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::teachers_stat_title')}</div>
        </div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 hover:border-primary/20 transition-all duration-300 group">
      <div class="flex flex-col h-full justify-between gap-4">
        <div class="w-10 h-10 rounded-2xl bg-accent/10 text-accent flex items-center justify-center group-hover:scale-110 transition-transform duration-300">
          <Users2 size={20} />
        </div>
        <div>
          <div class="text-[32px] font-black tracking-tight text-base-content leading-none mb-1 group-hover:text-accent transition-colors">{students.length}</div>
          <div class="text-[10px] font-bold uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::students_stat_title')}</div>
        </div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 hover:border-primary/20 transition-all duration-300 group">
      <div class="flex flex-col h-full justify-between gap-4">
        <div class="w-10 h-10 rounded-2xl bg-info/10 text-info flex items-center justify-center group-hover:scale-110 transition-transform duration-300">
          <School size={20} />
        </div>
        <div>
          <div class="text-[32px] font-black tracking-tight text-base-content leading-none mb-1 group-hover:text-info transition-colors">{classes.length}</div>
          <div class="text-[10px] font-bold uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::classes_stat_title')}</div>
        </div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-success/5 hover:border-success/20 transition-all duration-300 group col-span-2 lg:col-span-1 xl:col-span-2">
      <div class="flex items-center justify-between h-full">
        <div class="flex flex-col justify-between h-full">
           <div class="w-10 h-10 rounded-2xl bg-success/10 text-success flex items-center justify-center group-hover:scale-110 transition-transform duration-300">
            <Users2 size={20} />
          </div>
          <div>
            <div class="text-[32px] font-black tracking-tight text-base-content leading-none mb-1 group-hover:text-success transition-colors">{onlineUsers.length}</div>
            <div class="text-[10px] font-bold uppercase tracking-widest opacity-40">{t('frontend/src/lib/AdminPanel.svelte::online_stat_title')}</div>
          </div>
        </div>
        <div class="h-16 w-32 bg-success/5 rounded-xl flex items-center justify-center">
          <div class="flex -space-x-2 overflow-hidden px-2">
            {#each onlineUsers.slice(0, 4) as u}
              <div class="inline-block h-8 w-8 rounded-full ring-2 ring-base-100 bg-success/20 text-success flex items-center justify-center text-[10px] font-black uppercase">
                 {u.name.charAt(0)}
              </div>
            {/each}
            {#if onlineUsers.length > 4}
              <div class="inline-block h-8 w-8 rounded-full ring-2 ring-base-100 bg-base-200 text-base-content/40 flex items-center justify-center text-[8px] font-black">
                +{onlineUsers.length - 4}
              </div>
            {/if}
          </div>
        </div>
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
  <div class="grid gap-8 xl:grid-cols-12 items-start">
    <!-- Account Tools Side -->
    <div class="xl:col-span-4 space-y-8 sticky top-4">
      <details class="bg-base-100 rounded-[2.5rem] border border-base-200 shadow-sm overflow-hidden" bind:open={showCreateUsers}>
        <summary 
          class="flex items-center justify-between w-full px-6 py-4 group hover:bg-base-50 transition-all list-none cursor-pointer" 
        >
          <div class="flex items-center gap-3">
             <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center group-hover:scale-110 transition-transform">
               <Plus size={20} />
             </div>
             <div class="text-left">
                <h2 class="text-sm font-black uppercase tracking-widest">{t('frontend/src/lib/AdminPanel.svelte::account_creation_title')}</h2>
                <p class="text-[10px] font-bold opacity-40">{t('frontend/src/lib/AdminPanel.svelte::create_new_desc', {default: "Add new users to the system"})}</p>
             </div>
          </div>
          <ChevronDown size={18} class="transition-transform duration-300 opacity-40 group-hover:opacity-100 {showCreateUsers ? 'rotate-180' : ''}" />
        </summary>

        {#if showCreateUsers}
          <div class="px-6 pb-6 pt-2 space-y-6 border-t border-base-200" transition:slide>
            <!-- Add Teacher -->
            <div class="relative group">
              <div class="absolute -left-3 top-0 bottom-0 w-1 bg-gradient-to-b from-secondary to-transparent rounded-full opacity-0 group-hover:opacity-100 transition-opacity"></div>
              <h3 class="font-black text-xs uppercase tracking-widest opacity-40 mb-3 flex items-center gap-2">
                <GraduationCap size={14} /> {t('frontend/src/lib/AdminPanel.svelte::add_teacher_card_title')}
              </h3>
              <form on:submit|preventDefault={addTeacher} class="space-y-3">
                <StylishInput 
                  bind:value={teacherName} 
                  placeholder={t('frontend/src/lib/AdminPanel.svelte::teacher_name_placeholder')} 
                  icon={Users2}
                />
                <StylishInput 
                  bind:value={teacherEmail} 
                  placeholder={t('frontend/src/lib/AdminPanel.svelte::email_label')} 
                  type="email" 
                  required 
                  icon={MailCheck}
                />
                <StylishInput 
                  bind:value={teacherPassword} 
                  placeholder={t('frontend/src/lib/AdminPanel.svelte::password_label')} 
                  type="password" 
                  required 
                  icon={KeyRound}
                />
                <button class="btn btn-secondary btn-sm w-full h-10 rounded-xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-secondary/10 hover:shadow-secondary/20 hover:scale-[1.02] transition-all">
                  {t('frontend/src/lib/AdminPanel.svelte::add_button')}
                </button>
              </form>
            </div>

            <div class="divider opacity-50"></div>

            <!-- Add Student -->
            <div class="relative group">
              <div class="absolute -left-3 top-0 bottom-0 w-1 bg-gradient-to-b from-primary to-transparent rounded-full opacity-0 group-hover:opacity-100 transition-opacity"></div>
              <h3 class="font-black text-xs uppercase tracking-widest opacity-40 mb-3 flex items-center gap-2">
                <Users2 size={14} /> {t('frontend/src/lib/AdminPanel.svelte::add_student_card_title')}
              </h3>
              <form on:submit|preventDefault={addStudent} class="space-y-3">
                <StylishInput 
                  bind:value={studentName} 
                  placeholder={t('frontend/src/lib/AdminPanel.svelte::student_name_placeholder')} 
                  icon={Users2}
                />
                <StylishInput 
                  bind:value={studentEmail} 
                  placeholder={t('frontend/src/lib/AdminPanel.svelte::email_label')} 
                  type="email" 
                  required 
                  icon={MailCheck}
                />
                <StylishInput 
                  bind:value={studentPassword} 
                  placeholder={t('frontend/src/lib/AdminPanel.svelte::password_label')} 
                  type="password" 
                  required 
                  icon={KeyRound}
                />
                <button class="btn btn-primary btn-sm w-full h-10 rounded-xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-primary/10 hover:shadow-primary/20 hover:scale-[1.02] transition-all">
                  {t('frontend/src/lib/AdminPanel.svelte::add_button')}
                </button>
              </form>
            </div>
          </div>
        {/if}
      </details>

      <div class="bg-base-100 p-6 rounded-[2.5rem] border border-base-200 shadow-sm">
        <h2 class="text-sm font-black uppercase tracking-widest opacity-40 mb-6 flex items-center gap-2">
          <Shield size={16} />
          {t('frontend/src/lib/AdminPanel.svelte::whitelist_title')}
        </h2>
        
        <p class="text-[10px] font-bold opacity-50 uppercase tracking-widest leading-relaxed mb-4">
          {t('frontend/src/lib/AdminPanel.svelte::whitelist_description')}
        </p>
        
        <form on:submit|preventDefault={addToWhitelist} class="flex gap-2 mb-6">
          <div class="flex-1">
             <StylishInput 
              bind:value={whitelistEmail} 
              placeholder={t('frontend/src/lib/AdminPanel.svelte::whitelist_placeholder')} 
              required 
              icon={MailCheck}
            />
          </div>
          <button class="btn btn-primary h-[50px] aspect-square rounded-2xl flex items-center justify-center shadow-lg shadow-primary/20 hover:scale-105 transition-all">
            <Plus size={20} />
          </button>
        </form>

        <div class="bg-base-200/30 rounded-2xl border border-base-200/50 overflow-hidden">
          <div class="overflow-y-auto max-h-60 custom-scrollbar p-2 space-y-1">
             {#each whitelist as w}
                <div class="flex items-center justify-between p-3 rounded-xl hover:bg-base-100 transition-colors group">
                  <div class="flex items-center gap-3">
                    <div class="w-8 h-8 rounded-lg bg-success/10 text-success flex items-center justify-center">
                       <Check size={14} />
                    </div>
                    <span class="font-bold text-xs">{w.email}</span>
                  </div>
                  <button 
                    class="w-8 h-8 rounded-lg bg-error/10 text-error flex items-center justify-center opacity-0 group-hover:opacity-100 transition-all hover:bg-error hover:text-error-content" 
                    on:click={() => removeFromWhitelist(w.email)}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              {:else}
                <div class="text-center py-8 italic opacity-30 text-xs font-bold uppercase tracking-widest">
                  {t('frontend/src/lib/AdminPanel.svelte::whitelist_empty')}
                </div>
              {/each}
          </div>
        </div>
      </div>
    </div>

    <!-- Main Users List -->
    <div class="xl:col-span-8 space-y-6">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 px-2">
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/lib/AdminPanel.svelte::users_card_title')}</h2>
        <div class="flex items-center gap-2 flex-wrap sm:flex-nowrap">
          <div class="w-full sm:w-64">
            <StylishInput 
              bind:value={userQuery} 
              placeholder={t('frontend/src/lib/AdminPanel.svelte::search_users_placeholder')} 
              icon={Search}
            />
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <button class="btn btn-ghost btn-xs opacity-50 hover:opacity-100 transition-all font-black text-[10px] uppercase tracking-widest h-9 border border-base-200 rounded-xl px-3 flex-1 sm:flex-none" on:click={exportUsersCSV}>
               {t('frontend/src/lib/AdminPanel.svelte::export_csv_button')}
            </button>
            <button class="btn btn-ghost btn-xs opacity-50 hover:opacity-100 transition-all font-black text-[10px] uppercase tracking-widest h-9 border border-base-200 rounded-xl flex-none" on:click={refreshUsers} aria-label={t('frontend/src/lib/AdminPanel.svelte::refresh_online_users_button')}>
               <RefreshCw size={14} />
            </button>
          </div>
        </div>
      </div>

      <div class="bg-base-100 rounded-[2.5rem] border border-base-200 shadow-sm overflow-hidden">
        <div class="p-4 border-b border-base-200 bg-base-200/20 flex flex-wrap items-center gap-1.5">
          {#each ['all', 'student', 'teacher', 'admin'] as role}
             <button 
              class="px-3 py-1.5 rounded-lg text-[9px] font-black uppercase tracking-widest transition-all {userRoleFilter === role ? 'bg-primary text-primary-content shadow-lg shadow-primary/20' : 'hover:bg-base-200 text-base-content/60'}" 
              on:click={() => userRoleFilter = role}
            >
              {#if role === 'all'}
                {t('frontend/src/lib/AdminPanel.svelte::all_filter_option')}
              {:else}
                {t(`frontend/src/lib/AdminPanel.svelte::role_filter_${role}s`)}
              {/if}
            </button>
          {/each}
        </div>

        <div class="overflow-x-auto custom-scrollbar">
          <table class="table table-md w-full border-separate border-spacing-0">
            <thead>
              <tr class="bg-base-100/50">
                <th class="text-[10px] font-black uppercase tracking-widest opacity-40 border-b border-base-200 py-4 pl-6">{t('frontend/src/lib/AdminPanel.svelte::name_table_header')}</th>
                <th class="text-[10px] font-black uppercase tracking-widest opacity-40 border-b border-base-200 py-4">{t('frontend/src/lib/AdminPanel.svelte::role_table_header')}</th>
                <th class="text-[10px] font-black uppercase tracking-widest opacity-40 border-b border-base-200 py-4">{t('frontend/src/lib/AdminPanel.svelte::auth_table_header')}</th>
                <th class="text-[10px] font-black uppercase tracking-widest opacity-40 border-b border-base-200 py-4">{t('frontend/src/lib/AdminPanel.svelte::created_table_header')}</th>
                <th class="border-b border-base-200 py-4 pr-6"></th>
              </tr>
            </thead>
            <tbody>
              {#each filteredUsers as u}
                <tr class="group hover:bg-base-50 transition-colors">
                  <td class="py-4 pl-6 border-b border-base-100 group-hover:border-base-200/50">
                    <div class="flex items-center gap-4">
                      <div class="relative">
                        <div class="w-10 h-10 rounded-xl bg-primary/5 flex items-center justify-center font-black text-primary border border-primary/10 group-hover:bg-primary group-hover:text-primary-content transition-colors duration-300">
                           {(u.name || u.email || '?').charAt(0).toUpperCase()}
                        </div>
                        {#if onlineIds.has(u.id)}
                          <div class="absolute -bottom-0.5 -right-0.5 w-3 h-3 bg-success rounded-full ring-2 ring-base-100 shadow-sm animate-pulse"></div>
                        {/if}
                      </div>
                      <div class="flex flex-col min-w-0">
                        <span class="font-black text-xs sm:text-sm truncate">{userPrimary(u)}</span>
                        {#if userSecondary(u)}
                          <span class="text-[10px] font-bold opacity-40 truncate">{userSecondary(u)}</span>
                        {/if}
                      </div>
                    </div>
                  </td>
                  <td class="py-4 border-b border-base-100 group-hover:border-base-200/50">
                    <div class="w-32">
                      <CustomSelect 
                        small 
                        options={roleOptions} 
                        bind:value={u.role} 
                        on:change={(e) => changeRole(u.id, e.detail)}
                      />
                    </div>
                  </td>
                  <td class="py-4 border-b border-base-100 group-hover:border-base-200/50">
                    <div class="flex flex-wrap gap-1.5">
                      {#if hasEmailLogin(u)}
                        <span class="badge badge-sm border-none font-black text-[9px] uppercase tracking-widest px-2 h-5 bg-base-200/50 text-base-content/60">{t('frontend/src/lib/AdminPanel.svelte::auth_email_label')}</span>
                      {/if}
                      {#if u.ms_oid}
                        <span class="badge badge-sm border-none font-black text-[9px] uppercase tracking-widest px-2 h-5 bg-info/10 text-info">{t('frontend/src/lib/AdminPanel.svelte::auth_microsoft_label')}</span>
                      {/if}
                      {#if u.bk_uid}
                        <span class="badge badge-sm border-none font-black text-[9px] uppercase tracking-widest px-2 h-5 bg-warning/10 text-warning">{t('frontend/src/lib/AdminPanel.svelte::auth_bakalari_label')}</span>
                      {/if}
                      {#if u.role === 'teacher' && teacherIdToClassCount[u.id]}
                        <span class="badge badge-sm border-none font-black text-[9px] uppercase tracking-widest px-2 h-5 bg-primary/10 text-primary">
                          {teacherIdToClassCount[u.id]} {t('frontend/src/lib/AdminPanel.svelte::classes_table_header')}
                        </span>
                      {/if}
                    </div>
                  </td>
                  <td class="py-4 border-b border-base-100 group-hover:border-base-200/50 text-xs font-bold opacity-50 whitespace-nowrap">{formatDate(u.created_at)}</td>

                  <td class="py-4 pr-6 border-b border-base-100 group-hover:border-base-200/50 text-right">
                    <div class="flex justify-end gap-1 opacity-10 group-hover:opacity-100 transition-opacity">
                      <button
                        class="btn btn-ghost btn-xs w-8 h-8 rounded-lg text-info hover:bg-info/10"
                        title={u.bk_uid ? t('frontend/src/lib/AdminPanel.svelte::set_password_disabled_tooltip') : t('frontend/src/lib/AdminPanel.svelte::set_password_button_label')}
                        disabled={Boolean(u.bk_uid)}
                        on:click={() => { if (!u.bk_uid) promptSetPassword(u); }}
                      >
                        <KeyRound size={14} />
                      </button>
                      <button class="btn btn-ghost btn-xs w-8 h-8 rounded-lg text-error hover:bg-error/10" on:click={()=>deleteUser(u.id)} aria-label={t('frontend/src/lib/AdminPanel.svelte::delete_button')}>
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
        <div class="w-full sm:w-64">
          <StylishInput 
            bind:value={classQuery} 
            placeholder={t('frontend/src/lib/AdminPanel.svelte::search_classes_placeholder')} 
            icon={Search}
          />
        </div>
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
        <table class="table table-md w-full border-separate border-spacing-0">
          <thead>
            <tr class="bg-base-100/50">
              <th class="text-[10px] font-black uppercase tracking-widest opacity-40 border-b border-base-200 py-4 pl-6">{t('frontend/src/lib/AdminPanel.svelte::class_table_header')}</th>
              <th class="text-[10px] font-black uppercase tracking-widest opacity-40 border-b border-base-200 py-4">{t('frontend/src/lib/AdminPanel.svelte::teacher_table_header')}</th>
              <th class="text-[10px] font-black uppercase tracking-widest opacity-40 border-b border-base-200 py-4">{t('frontend/src/lib/AdminPanel.svelte::created_table_header')}</th>
              <th class="border-b border-base-200 py-4 pr-6"></th>
            </tr>
          </thead>
          <tbody>
            {#each filteredClasses as c}
              <tr class="group hover:bg-base-50 transition-colors">
                <td class="py-4 pl-6 border-b border-base-100 group-hover:border-base-200/50">
                  <div class="flex items-center gap-4">
                    <div class="w-10 h-10 rounded-xl bg-primary/5 flex items-center justify-center text-primary border border-primary/10 group-hover:bg-primary group-hover:text-primary-content transition-colors duration-300">
                       <School size={18} />
                    </div>
                    <a href={`/classes/${c.id}`} class="font-black text-sm hover:text-primary transition-colors no-underline text-current">{c.name}</a>
                  </div>
                </td>
                <td class="py-4 border-b border-base-100 group-hover:border-base-200/50">
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
                <td class="py-4 border-b border-base-100 group-hover:border-base-200/50 text-xs font-bold opacity-50">{formatDate(c.created_at)}</td>
                <td class="py-4 pr-6 border-b border-base-100 group-hover:border-base-200/50 text-right whitespace-nowrap">
                  <div class="flex justify-end gap-1 opacity-10 group-hover:opacity-100 transition-opacity">
                    <button class="btn btn-ghost btn-xs w-8 h-8 rounded-lg text-info hover:bg-info/10" on:click={()=>renameClass(c.id)} aria-label={t('frontend/src/lib/AdminPanel.svelte::rename_button')}>
                      <Edit size={14} />
                    </button>
                    <button class="btn btn-ghost btn-xs w-8 h-8 rounded-lg text-primary hover:bg-primary/10" on:click={()=>transferTarget={ id: c.id, name: c.name, to: null }} aria-label={t('frontend/src/lib/AdminPanel.svelte::transfer_button')}>
                      <ArrowRightLeft size={14} />
                    </button>
                    <button class="btn btn-ghost btn-xs w-8 h-8 rounded-lg text-error hover:bg-error/10" on:click={()=>deleteClassAction(c.id)} aria-label={t('frontend/src/lib/AdminPanel.svelte::delete_button')}>
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
        
        <div class="space-y-6">
          <StylishInput 
            bind:value={newClassName} 
            placeholder={t('frontend/src/lib/AdminPanel.svelte::class_name_placeholder')} 
            icon={School}
            label={t('frontend/src/lib/AdminPanel.svelte::class_name_placeholder')}
          />

          <div class="space-y-2">
            <CustomSelect 
              searchable
              options={teacherOptions} 
              bind:value={newClassTeacherId} 
              placeholder={t('frontend/src/lib/AdminPanel.svelte::select_teacher_option')}
              label={t('frontend/src/lib/AdminPanel.svelte::teacher_table_header')}
            />
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
      <button 
        type="button"
        class="flex items-center gap-2 px-2 group hover:opacity-100 transition-all relative z-20" 
        on:click|stopPropagation={() => { showSystemVariables = !showSystemVariables; }}
      >
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40 group-hover:opacity-100 transition-all flex items-center gap-2">
          {t('frontend/src/lib/AdminPanel.svelte::system_variables_title')}
          <ChevronDown size={14} class="transition-transform duration-300 {showSystemVariables ? 'rotate-180 opacity-100' : 'opacity-40'}" />
        </h2>
      </button>

      {#if showSystemVariables}
        <div class="bg-base-100 rounded-[2.5rem] border border-base-200 shadow-sm overflow-hidden">
          <div class="p-6 border-b border-base-200 bg-base-200/20">
            <p class="text-[10px] font-bold opacity-50 uppercase tracking-widest leading-relaxed mb-6">
              {t('frontend/src/lib/AdminPanel.svelte::system_variables_description')}
            </p>
            
            <form on:submit|preventDefault={saveSystemVariable} class="grid gap-4 md:grid-cols-[1fr_2fr_auto] items-end">
              <StylishInput 
                bind:value={variableKey} 
                placeholder={t('frontend/src/lib/AdminPanel.svelte::system_variable_key_placeholder')} 
                disabled={Boolean(editingVariableKey)}
                required
                label={t('frontend/src/lib/AdminPanel.svelte::system_variable_key_label')}
              />
              
              <StylishInput 
                bind:value={variableValue} 
                placeholder={t('frontend/src/lib/AdminPanel.svelte::system_variable_value_placeholder')} 
                label={t('frontend/src/lib/AdminPanel.svelte::system_variable_value_label')}
              />

              <div class="flex gap-2 pb-1.5">
                <button class="btn btn-primary h-[50px] rounded-xl px-6 font-black uppercase tracking-widest text-[10px] shadow-lg shadow-primary/20" type="submit">
                  {editingVariableKey ? t('frontend/src/lib/AdminPanel.svelte::save_button') : t('frontend/src/lib/AdminPanel.svelte::add_button')}
                </button>
                {#if editingVariableKey}
                  <button class="btn btn-ghost h-[50px] rounded-xl font-black uppercase tracking-widest text-[10px]" type="button" on:click={resetVariableForm}>
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
        <StylishInput 
            bind:value={emailPingTarget} 
            placeholder={t('frontend/src/lib/AdminPanel.svelte::email_ping_placeholder')} 
            type="email" 
            required 
            icon={MailCheck}
            label={t('frontend/src/lib/AdminPanel.svelte::email_label')}
          />
        
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

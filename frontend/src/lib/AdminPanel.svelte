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
      ok = 'Teacher created ✔';
      teacherEmail = teacherPassword = '';
      await loadUsers();
    } else {
      const j = await r.json().catch(() => ({}));
      err = j.error || 'Failed to create teacher';
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
      ok = 'Role updated';
      await loadUsers();
    } catch (e: any) { err = e.message; }
  }
  async function deleteUser(id: string) {
    if (!confirm('Delete this user? This cannot be undone.')) return;
    try { await apiFetch(`/api/users/${id}`, { method: 'DELETE' }); ok = 'User deleted'; await loadUsers(); } catch (e: any) { err = e.message; }
  }
  function exportUsersCSV() {
    const rows = [['ID','Email','Name','Role','Created']].concat(users.map(u => [String(u.id), u.email, u.name ?? '', u.role, formatDate(u.created_at)]));
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
      ok = `Class created (#${created.id})`;
      showCreateClass = false; newClassName = ''; newClassTeacherId = null;
      // Update both local state and the store
      await loadClasses();
      classesStore.addClass(created);
    } catch (e: any) { err = e.message; }
  }
  async function renameClass(id: string) {
    const name = prompt('New class name:');
    if (!name) return;
    try { 
      await apiFetch(`/api/classes/${id}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name }) }); 
      ok = 'Class renamed'; 
      await loadClasses();
      // Update the store
      classesStore.updateClass(id, { name });
    } catch (e: any) { err = e.message; }
  }
  async function deleteClassAction(id: string) {
    if (!confirm('Delete this class and its data?')) return;
    try { 
      await apiFetch(`/api/classes/${id}`, { method: 'DELETE' }); 
      ok = 'Class deleted'; 
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
      ok = 'Class ownership transferred';
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
    try { await apiFetch(`/api/assignments/${aid}/publish`, { method: 'PUT' }); ok = 'Assignment published'; await loadAssignments(); } catch (e: any) { err = e.message; }
  }
  async function deleteAssignment(aid: string) {
    if (!confirm('Delete this assignment?')) return;
    try { await apiFetch(`/api/assignments/${aid}`, { method: 'DELETE' }); ok = 'Assignment deleted'; await loadAssignments(); } catch (e: any) { err = e.message; }
  }
</script>

<h1 class="text-2xl font-bold mb-6 flex items-center gap-2"><Shield class="w-6 h-6" aria-hidden="true" /> Admin</h1>

<div class="tabs tabs-boxed mb-6">
  <a role="tab" class="tab {tab==='overview' ? 'tab-active' : ''}" on:click={() => tab='overview'}>Overview</a>
  <a role="tab" class="tab {tab==='teachers' ? 'tab-active' : ''}" on:click={() => tab='teachers'}>Teachers</a>
  <a role="tab" class="tab {tab==='users' ? 'tab-active' : ''}" on:click={() => tab='users'}>Users</a>
  <a role="tab" class="tab {tab==='classes' ? 'tab-active' : ''}" on:click={() => tab='classes'}>Classes</a>
  <a role="tab" class="tab {tab==='assignments' ? 'tab-active' : ''}" on:click={() => tab='assignments'}>Assignments</a>
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
        <h2 class="card-title mb-2">Platform stats</h2>
        <div class="stats stats-vertical sm:stats-horizontal shadow">
          <div class="stat"><div class="stat-figure"><Users2 class="w-5 h-5" /></div><div class="stat-title">Users</div><div class="stat-value">{users.length}</div></div>
          <div class="stat"><div class="stat-figure"><GraduationCap class="w-5 h-5" /></div><div class="stat-title">Teachers</div><div class="stat-value">{teachers.length}</div></div>
          <div class="stat"><div class="stat-figure"><School class="w-5 h-5" /></div><div class="stat-title">Classes</div><div class="stat-value">{classes.length}</div></div>
          <div class="stat"><div class="stat-figure"><BookOpen class="w-5 h-5" /></div><div class="stat-title">Assignments</div><div class="stat-value">{assignments.length}</div></div>
        </div>
      </div>
    </div>
    <div class="card bg-base-100 shadow">
      <div class="card-body gap-3">
        <h2 class="card-title">Quick actions</h2>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-sm" on:click={() => { tab='teachers'; }}>Add teacher</button>
          <button class="btn btn-sm" on:click={() => { tab='classes'; showCreateClass = true; }}>Create class</button>
          <a class="btn btn-sm" href="/dashboard">Go to dashboard</a>
        </div>
      </div>
    </div>
    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <h2 class="card-title">Unpublished assignments</h2>
        <ul class="space-y-2 max-h-60 overflow-auto">
          {#each assignments.filter(a => !a.published).slice(0, 8) as a}
            <li class="flex items-center justify-between gap-3">
              <a class="link" href={`/assignments/${a.id}`}>{a.title}</a>
              <button class="btn btn-ghost btn-xs" on:click={() => publishAssignment(a.id)}>Publish</button>
            </li>
          {/each}
          {#if !assignments.filter(a => !a.published).length}
            <li class="opacity-70 text-sm">All assignments published</li>
          {/if}
        </ul>
      </div>
    </div>
  </div>
{/if}

{#if tab === 'teachers'}
  <div class="grid grid-cols-1 xl:grid-cols-2 gap-6">
    <div class="card bg-base-100 shadow">
      <div class="card-body space-y-4">
        <h2 class="card-title">Add teacher</h2>
        <form on:submit|preventDefault={addTeacher} class="space-y-3">
          <input type="email" bind:value={teacherEmail} placeholder="Email" required class="input input-bordered w-full" />
          <input type="password" bind:value={teacherPassword} placeholder="Password" required class="input input-bordered w-full" />
          <button class="btn btn-primary">Add</button>
        </form>
      </div>
    </div>
    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <div class="flex items-center justify-between mb-3">
          <h2 class="card-title">Teachers</h2>
          <button class="btn btn-ghost btn-sm" on:click={loadUsers}><RefreshCw class="w-4 h-4" /></button>
        </div>
        <div class="overflow-x-auto">
          <table class="table table-zebra">
            <thead><tr><th>ID</th><th>Email</th><th>Name</th><th>Classes</th><th>Created</th></tr></thead>
            <tbody>
              {#each teachers as t}
                <tr>
                  <td>{t.id}</td>
                  <td>{t.email}</td>
                  <td>{t.name ?? ''}</td>
                  <td>{teacherIdToClassCount[t.id] ?? 0}</td>
                  <td>{formatDate(t.created_at)}</td>
                </tr>
              {/each}
              {#if !teachers.length}
                <tr><td colspan="5"><i>No teachers</i></td></tr>
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
        <h2 class="card-title">Users</h2>
        <div class="flex items-center gap-2">
          <label class="input input-bordered input-sm flex items-center gap-2">
            <Search class="w-4 h-4" aria-hidden="true" />
            <input class="grow" placeholder="Search users" bind:value={userQuery} />
          </label>
          <button class="btn btn-sm" on:click={exportUsersCSV}>Export CSV</button>
          <button class="btn btn-ghost btn-sm" on:click={loadUsers}><RefreshCw class="w-4 h-4" /></button>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="table table-zebra">
          <thead><tr><th>ID</th><th>Email</th><th>Name</th><th>Role</th><th>Created</th><th></th></tr></thead>
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
              <tr><td colspan="6"><i>No users</i></td></tr>
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
        <h2 class="card-title">Classes</h2>
        <div class="flex items-center gap-2">
          <label class="input input-bordered input-sm flex items-center gap-2">
            <Search class="w-4 h-4" aria-hidden="true" />
            <input class="grow" placeholder="Search classes" bind:value={classQuery} />
          </label>
          <button class="btn btn-sm" on:click={() => showCreateClass = true}><Plus class="w-4 h-4" /> New</button>
          <button class="btn btn-ghost btn-sm" on:click={loadClasses}><RefreshCw class="w-4 h-4" /></button>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="table table-zebra">
          <thead><tr><th>ID</th><th>Name</th><th>Teacher</th><th>Created</th><th></th></tr></thead>
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
              <tr><td colspan="5"><i>No classes</i></td></tr>
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  </div>

  {#if showCreateClass}
    <dialog open class="modal">
      <div class="modal-box space-y-4">
        <h3 class="font-semibold">Create class</h3>
        <input class="input input-bordered w-full" placeholder="Class name" bind:value={newClassName} />
        <select class="select select-bordered w-full" bind:value={newClassTeacherId}>
          <option value={null} disabled selected>Select teacher</option>
          {#each teachers as t}
            <option value={t.id}>{t.name ?? t.email} (#{t.id})</option>
          {/each}
        </select>
        <div class="modal-action">
          <button class="btn" on:click={createClass}><Plus class="w-4 h-4" /> Create</button>
          <button class="btn btn-ghost" on:click={() => { showCreateClass = false; }}>Cancel</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" on:click={() => showCreateClass = false}><button>close</button></form>
    </dialog>
  {/if}

  {#if transferTarget}
    <dialog open class="modal">
      <div class="modal-box space-y-4">
        <h3 class="font-semibold">Transfer ownership</h3>
        <p>Select the new teacher for class #{transferTarget.id}.</p>
        <select class="select select-bordered w-full" bind:value={transferTarget.to}>
          <option value={null} disabled selected>Select teacher</option>
          {#each teachers as t}
            <option value={t.id}>{t.name ?? t.email} (#{t.id})</option>
          {/each}
        </select>
        <div class="modal-action">
          <button class="btn" on:click={transferClass}><ArrowRightLeft class="w-4 h-4" /> Transfer</button>
          <button class="btn btn-ghost" on:click={() => transferTarget = null}>Cancel</button>
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
        <h2 class="card-title">Assignments</h2>
        <div class="flex items-center gap-2">
          <label class="input input-bordered input-sm flex items-center gap-2">
            <Search class="w-4 h-4" aria-hidden="true" />
            <input class="grow" placeholder="Search assignments" bind:value={assignmentQuery} />
          </label>
          <select class="select select-sm select-bordered" bind:value={assignmentFilter}>
            <option value="all">All</option>
            <option value="published">Published</option>
            <option value="unpublished">Unpublished</option>
          </select>
          <button class="btn btn-ghost btn-sm" on:click={loadAssignments}><RefreshCw class="w-4 h-4" /></button>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="table table-zebra">
          <thead><tr><th>ID</th><th>Title</th><th>Class</th><th>Deadline</th><th>Status</th><th></th></tr></thead>
          <tbody>
            {#each filteredAssignments as a}
              <tr>
                <td>{a.id}</td>
                <td><a href={`/assignments/${a.id}`} class="link link-primary">{a.title}</a></td>
                <td>{a.class_id}</td>
                <td>{formatDateTime(a.deadline)}</td>
                <td>{a.published ? 'Published' : 'Unpublished'}</td>
                <td class="text-right whitespace-nowrap">
                  {#if !a.published}
                    <button class="btn btn-xs" on:click={()=>publishAssignment(a.id)}>Publish</button>
                  {/if}
                  <button class="btn btn-ghost btn-xs text-error" on:click={()=>deleteAssignment(a.id)}><Trash2 class="w-4 h-4" /></button>
                </td>
              </tr>
            {/each}
            {#if !filteredAssignments.length}
              <tr><td colspan="6"><i>No assignments</i></td></tr>
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  </div>
{/if}

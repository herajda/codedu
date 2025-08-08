<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch, apiJSON } from '$lib/api';
  import { sha256 } from '$lib/hash';
  import { formatDate } from "$lib/date";

  // tabs state
  let tab: 'teachers' | 'users' | 'classes' = 'teachers';

  // 1) add teacher form
  let email = '', password = '';
  let ok = '', err = '';
  async function addTeacher() {
    err = ok = '';
    const r = await apiFetch('/api/teachers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password: await sha256(password) })
    });
    if (r.status === 201) {
      ok = 'Teacher created ✔';
      email = password = '';
      await loadUsers();
    } else {
      err = (await r.json()).error;
    }
  }

  // 2) users table
  type User = { id: number; email: string; role: string; created_at: string };
  let users: User[] = [];
  const roles = ['student', 'teacher', 'admin'];
  async function loadUsers() { users = await apiJSON('/api/users'); }
  async function changeRole(id: number, role: string) {
    try {
      await apiFetch(`/api/users/${id}/role`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ role })
      });
      loadUsers();
    } catch (e) { alert(e); }
  }

  // 3) classes overview
  type Class = { id: number; name: string; teacher_id: number; created_at: string };
  let classes: Class[] = [];
  async function loadClasses() { classes = await apiJSON('/api/classes/all'); }

  onMount(() => { loadUsers(); loadClasses(); });
</script>

<h1 class="text-2xl font-bold mb-6">Admin panel</h1>
<div role="tablist" class="tabs tabs-boxed mb-6">
  <a role="tab" class="tab {tab==='teachers' ? 'tab-active' : ''}" on:click={() => tab='teachers'}>Teachers</a>
  <a role="tab" class="tab {tab==='users' ? 'tab-active' : ''}" on:click={() => tab='users'}>Users</a>
  <a role="tab" class="tab {tab==='classes' ? 'tab-active' : ''}" on:click={() => tab='classes'}>Classes</a>
</div>

{#if tab === 'teachers'}
  <div class="card bg-base-100 shadow max-w-sm">
    <div class="card-body space-y-4">
      <h2 class="card-title">Add teacher</h2>
      <form on:submit|preventDefault={addTeacher} class="space-y-3">
        <input type="email" bind:value={email} placeholder="Email" required class="input input-bordered w-full" />
        <input type="password" bind:value={password} placeholder="Password" required class="input input-bordered w-full" />
        <button class="btn btn-primary w-full">Add</button>
      </form>
      {#if ok}<p class="text-success">{ok}</p>{/if}
      {#if err}<p class="text-error">{err}</p>{/if}
    </div>
  </div>
{/if}

{#if tab === 'users'}
  <div class="overflow-x-auto">
    <table class="table table-zebra">
      <thead>
        <tr><th>ID</th><th>Email</th><th>Role</th><th>Created</th></tr>
      </thead>
      <tbody>
        {#each users as u}
          <tr>
            <td>{u.id}</td>
            <td>{u.email}</td>
            <td>
              <select bind:value={u.role} on:change={(e)=>changeRole(u.id, (e.target as HTMLSelectElement).value)} class="select select-bordered select-sm">
                {#each roles as r}<option>{r}</option>{/each}
              </select>
            </td>
            <td>{formatDate(u.created_at)}</td>
          </tr>
        {/each}
        {#if !users.length}
          <tr><td colspan="4"><i>No users</i></td></tr>
        {/if}
      </tbody>
    </table>
  </div>
{/if}

{#if tab === 'classes'}
  <div class="overflow-x-auto">
    <table class="table table-zebra">
      <thead>
        <tr><th>ID</th><th>Name</th><th>Teacher ID</th><th>Created</th></tr>
      </thead>
      <tbody>
        {#each classes as c}
          <tr>
            <td>{c.id}</td>
            <td><a href={`/classes/${c.id}`} class="link link-primary">{c.name}</a></td>
            <td>{c.teacher_id}</td>
            <td>{formatDate(c.created_at)}</td>
          </tr>
        {/each}
        {#if !classes.length}
          <tr><td colspan="4"><i>No classes</i></td></tr>
        {/if}
      </tbody>
    </table>
  </div>
{/if}

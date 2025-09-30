<script lang="ts">
  import { onMount } from 'svelte';
  import { auth } from '$lib/auth';
  import { apiJSON, apiFetch } from '$lib/api';
  import { formatDateTime } from '$lib/date';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { Filter, Search, AlertTriangle, Clock, CheckCircle2, Copy } from 'lucide-svelte';
  import { TEACHER_GROUP_ID } from '$lib/teacherGroup';

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    load();
  }
  let role = '';
  $: role = $auth?.role ?? '';

  let cls: any = null;
  let loading = true;
  let assignments: any[] = [];
  let mySubs: any[] = [];
  let students: any[] = [];
  let progressCounts: Record<string, number> = {};
  let err = '';
  let now = Date.now();
  let search = '';
  type FilterMode = 'all' | 'upcoming' | 'late'
  let filterMode: FilterMode = 'all';
  type SortMode = 'deadline_asc' | 'deadline_desc' | 'title_asc'
  let sortMode: SortMode = 'deadline_asc';

  onMount(() => {
    const t = setInterval(() => (now = Date.now()), 60000);
    return () => clearInterval(t);
  });

  function countdown(deadline: string, completed?: boolean) {
    const diff = new Date(deadline).getTime() - now;
    if (diff <= 0) return completed ? 'deadline passed' : 'Overdue';
    const d = Math.floor(diff / 86400000);
    if (d >= 1) return `${d}d`;
    const h = Math.floor(diff / 3600000);
    const m = Math.floor((diff % 3600000) / 60000);
    return `${h}h ${m}m`;
  }

  async function load() {
    loading = true;
    err = '';
    cls = null;
    try {
      const data = await apiJSON(`/api/classes/${id}`);
      cls = data;
      students = data.students ?? [];
      assignments = [...(data.assignments ?? [])];
      if (role === 'student') {
        mySubs = await apiJSON('/api/my-submissions');
        assignments = assignments.map((a) => {
          const subs = mySubs.filter((s: any) => s.assignment_id === a.id);
          const best = subs.reduce((m: number, s: any) => {
            const p = s.override_points ?? s.points ?? 0;
            return p > m ? p : m;
          }, 0);
          return {
            ...a,
            best,
            completed: subs.some((s: any) => s.status === 'completed')
          };
        });
      } else if (role === 'teacher' || role === 'admin') {
        // compute progress counts per assignment
        const prog = await apiJSON(`/api/classes/${id}/progress`);
        progressCounts = {};
        for (const a of prog.assignments ?? []) {
          const done = (prog.scores ?? []).filter((sc: any) => sc.assignment_id === a.id && (sc.points ?? 0) >= a.max_points).length;
          progressCounts[a.id] = done;
        }
      }
    } catch (e: any) {
      err = e.message;
    }
    loading = false;
  }

  onMount(load);

  // Derived visible list with search/filter/sort
  $: visibleAssignments = assignments
    .filter((a) => (a.title ?? '').toLowerCase().includes(search.toLowerCase()))
    .filter((a) => {
      if (filterMode === 'all') return true;
      const isLate = new Date(a.deadline) < new Date();
      return filterMode === 'late' ? isLate : !isLate;
    })
    .slice()
    .sort((a, b) => {
      if (sortMode === 'title_asc') return String(a.title).localeCompare(String(b.title));
      const da = new Date(a.deadline).getTime();
      const db = new Date(b.deadline).getTime();
      return sortMode === 'deadline_desc' ? db - da : da - db;
    });

  // Quick create: one-click create and jump to assignment editor (teachers/admin only)
  async function quickCreateAssignment() {
    try {
      const created = await apiJSON(`/api/classes/${id}/assignments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: 'Untitled assignment', description: '', show_traceback: false, show_test_details: false })
      });
      goto(`/assignments/${created.id}?new=1`);
    } catch (e: any) {
      err = e.message;
    }
  }

  // Copy assignment from Teachers' group
  let copyDialog: HTMLDialogElement;
  let teacherFiles: any[] = [];
  let copyLoading = false;
  let copyErr = '';
  let selectedAssignmentId = '';
  let copyBreadcrumbs: { id: string | null; name: string }[] = [{ id: null, name: '🏠' }];
  let copyCurrentParent: string | null = null;

  async function openCopyFromTeachers() {
    copyErr = '';
    selectedAssignmentId = '';
    copyLoading = true;
    copyBreadcrumbs = [{ id: null, name: '🏠' }];
    copyCurrentParent = null;
    try {
      // Get files from Teachers' group root
      await loadTeacherFiles(null);
    } catch (e: any) {
      copyErr = e.message;
    }
    copyLoading = false;
    copyDialog.showModal();
  }

  async function loadTeacherFiles(parent: string | null) {
    copyLoading = true;
    copyErr = '';
    try {
      const q = parent === null ? '' : `?parent=${parent}`;
      const files = await apiJSON(`/api/classes/${TEACHER_GROUP_ID}/files${q}`);
      teacherFiles = files.filter((f: any) => f.is_dir || f.assignment_id);
      copyCurrentParent = parent;
    } catch (e: any) {
      copyErr = e.message;
    }
    copyLoading = false;
  }

  async function openTeacherFolder(item: any) {
    if (!item.is_dir) return;
    copyBreadcrumbs = [...copyBreadcrumbs, { id: item.id, name: item.name }];
    await loadTeacherFiles(item.id);
  }

  function copyCrumbTo(i: number) {
    const b = copyBreadcrumbs[i];
    copyBreadcrumbs = copyBreadcrumbs.slice(0, i + 1);
    loadTeacherFiles(b.id);
  }

  async function doCopyFromTeachers() {
    if (!selectedAssignmentId) {
      copyErr = 'Please select an assignment';
      return;
    }
    try {
      const res = await apiFetch(`/api/classes/${id}/assignments/import`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ source_assignment_id: selectedAssignmentId })
      });
      const result = await res.json();
      if (!res.ok) {
        copyErr = result.error || 'Failed to copy assignment';
        return;
      }
      copyDialog.close();
      // Navigate to the copied assignment
      if (result.assignment_id) {
        goto(`/assignments/${result.assignment_id}?new=1`);
      } else {
        // Refresh the page to show the new assignment
        load();
      }
    } catch (e: any) {
      copyErr = e.message;
    }
  }
</script>

{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <div class="mb-4">
    <h1 class="text-2xl font-semibold">{cls.name}</h1>
    {#if role === 'student'}
      <p class="opacity-70 text-sm">Teacher: {cls.teacher.name ?? cls.teacher.email}</p>
    {/if}
  </div>

  <div>
    <div class="card-elevated p-5">
      <div class="flex items-center justify-between mb-3 gap-2 flex-wrap">
        <h2 class="font-semibold">Assignments</h2>
        <div class="flex flex-wrap items-center gap-2 w-full sm:w-auto justify-end">
          <div class="join hidden sm:flex">
            <button class={`btn btn-sm join-item ${filterMode==='all' ? 'btn-active' : 'btn-ghost'}`} type="button" on:click={() => filterMode='all'}><Filter class="w-4 h-4" aria-hidden="true" /> All</button>
            <button class={`btn btn-sm join-item ${filterMode==='upcoming' ? 'btn-active' : 'btn-ghost'}`} type="button" on:click={() => filterMode='upcoming'}><Clock class="w-4 h-4" aria-hidden="true" /> Upcoming</button>
            <button class={`btn btn-sm join-item ${filterMode==='late' ? 'btn-active' : 'btn-ghost'}`} type="button" on:click={() => filterMode='late'}><AlertTriangle class="w-4 h-4" aria-hidden="true" /> Overdue</button>
          </div>
          <label class="input input-bordered input-sm flex items-center gap-2">
            <Search class="w-4 h-4" aria-hidden="true" />
            <input type="text" class="grow" placeholder="Search" bind:value={search} />
          </label>
          <select class="select select-sm select-bordered" bind:value={sortMode} aria-label="Sort assignments">
            <option value="deadline_asc">Deadline ↑</option>
            <option value="deadline_desc">Deadline ↓</option>
            <option value="title_asc">Title A→Z</option>
          </select>
          {#if role === 'teacher' || role === 'admin'}
            <button class="btn btn-sm" type="button" on:click={quickCreateAssignment}>New assignment</button>
            <button class="btn btn-sm btn-outline" type="button" on:click={openCopyFromTeachers}>
              <Copy class="w-4 h-4 mr-2" aria-hidden="true" />
              Copy from Teachers' group
            </button>
          {/if}
        </div>
      </div>
      <ul class="space-y-3">
        {#each visibleAssignments as a}
          <li>
            <a href={`/assignments/${a.id}`} class="block no-underline text-current">
              <div class="card-elevated p-4 hover:shadow-md transition">
                <div class="flex items-center justify-between gap-4">
                  <div class="min-w-0">
                    <div class="font-medium truncate">{a.title}</div>
                    <div class="text-sm opacity-70 flex items-center gap-2">
                      <span class={new Date(a.deadline) < new Date() && !a.completed ? 'text-error' : ''}>{formatDateTime(a.deadline)}</span>
                      <span>·</span>
                      <span>{countdown(a.deadline, a.completed)}</span>
                      {#if a.second_deadline}
                        <span>·</span>
                        <span class="text-warning">2nd: {formatDateTime(a.second_deadline)} ({Math.round(a.late_penalty_ratio * 100)}%)</span>
                      {/if}
                    </div>
                  </div>
                  <div class="flex items-center gap-3 shrink-0">
                    {#if role==='student'}
                      <div class="flex items-center gap-2">
                        <progress class="progress progress-primary w-20 sm:w-24" value={a.best || 0} max={a.max_points}></progress>
                        <span class="text-sm whitespace-nowrap">{a.best ?? 0}/{a.max_points}</span>
                      </div>
                    {/if}
                    {#if role==='teacher' || role==='admin'}
                      {#if a.published}
                        <div class="flex items-center gap-2">
                          <progress class="progress progress-primary w-20 sm:w-24" value={progressCounts[a.id] || 0} max={students.length}></progress>
                          <span class="text-sm whitespace-nowrap">{progressCounts[a.id] || 0}/{students.length}</span>
                        </div>
                      {/if}
                    {/if}
                    {#if a.completed}
                      <span class="badge badge-success"><CheckCircle2 class="w-3 h-3" aria-hidden="true" /> Done</span>
                    {/if}
                    {#if !a.published}
                      <span class="badge badge-warning">Unpublished</span>
                    {/if}
                  </div>
                </div>
              </div>
            </a>
          </li>
        {/each}
        {#if !visibleAssignments.length}
          <li class="text-sm opacity-70">No assignments yet</li>
        {/if}
      </ul>
    </div>
  </div>

  <!-- Copy from Teachers' group modal -->
  <dialog bind:this={copyDialog} class="modal">
    <div class="modal-box max-w-4xl">
      <h3 class="font-bold mb-3">Copy assignment from Teachers' group</h3>
      
      <!-- Breadcrumb navigation -->
      <div class="mb-4">
        <nav class="text-sm">
          <ul class="flex flex-wrap gap-1 items-center">
            {#each copyBreadcrumbs as b, i}
              <li class="after:mx-1 after:content-['/'] last:after:hidden">
                <button 
                  type="button" 
                  class="link px-2 py-1 rounded hover:bg-base-300" 
                  on:click={() => copyCrumbTo(i)} 
                  aria-label={`Open ${b.name}`}
                >
                  {b.name}
                </button>
              </li>
            {/each}
          </ul>
        </nav>
      </div>

      {#if copyLoading}
        <p>Loading...</p>
      {:else if copyErr}
        <p class="text-error">{copyErr}</p>
      {:else}
        <!-- File structure -->
        <div class="max-h-96 overflow-y-auto border border-base-300 rounded-lg">
          <table class="table table-zebra w-full">
            <thead>
              <tr>
                <th class="text-left">Name</th>
                <th class="text-left">Type</th>
                <th class="text-right">Modified</th>
                <th class="w-32 text-right">Action</th>
              </tr>
            </thead>
            <tbody>
              {#each teacherFiles as item}
                <tr class="hover:bg-base-200">
                  <td>
                    {#if item.is_dir}
                      <button 
                        class="link flex items-center" 
                        on:click={() => openTeacherFolder(item)}
                      >
                        <i class="fa-solid fa-folder text-warning mr-2"></i>
                        {item.name}
                      </button>
                    {:else}
                      <div class="flex items-center">
                        <i class="fa-solid fa-file-circle-check text-primary mr-2"></i>
                        {item.name}
                      </div>
                    {/if}
                    <div class="text-xs text-gray-500">{item.path}</div>
                  </td>
                  <td>{item.is_dir ? 'Folder' : 'Assignment'}</td>
                  <td class="text-right">{formatDateTime(item.updated_at)}</td>
                  <td class="text-right">
                    {#if !item.is_dir && item.assignment_id}
                      <button 
                        class="btn btn-xs btn-primary" 
                        on:click={() => selectedAssignmentId = item.assignment_id}
                        class:btn-active={selectedAssignmentId === item.assignment_id}
                      >
                        {selectedAssignmentId === item.assignment_id ? 'Selected' : 'Select'}
                      </button>
                    {/if}
                  </td>
                </tr>
              {/each}
              {#if !teacherFiles.length}
                <tr><td colspan="4"><i>No items in this folder</i></td></tr>
              {/if}
            </tbody>
          </table>
        </div>

        {#if selectedAssignmentId}
          <div class="mt-4 p-3 bg-primary/10 rounded-lg">
            <p class="text-sm">
              <i class="fa-solid fa-check-circle text-primary mr-2"></i>
              Assignment selected for copying
            </p>
          </div>
        {/if}

        <div class="modal-action">
          <form method="dialog"><button class="btn">Cancel</button></form>
          <button 
            class="btn btn-primary" 
            on:click|preventDefault={doCopyFromTeachers} 
            disabled={!selectedAssignmentId}
          >
            Copy assignment
          </button>
        </div>
      {/if}
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>
{/if}

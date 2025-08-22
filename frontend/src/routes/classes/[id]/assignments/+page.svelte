<script lang="ts">
  import { onMount } from 'svelte';
  import { auth } from '$lib/auth';
  import { apiJSON } from '$lib/api';
  import { formatDateTime } from '$lib/date';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { Filter, Search, AlertTriangle, Clock, CheckCircle2 } from 'lucide-svelte';

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
        body: JSON.stringify({ title: 'Untitled assignment', description: '', show_traceback: false })
      });
      goto(`/assignments/${created.id}?new=1`);
    } catch (e: any) {
      err = e.message;
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
{/if}

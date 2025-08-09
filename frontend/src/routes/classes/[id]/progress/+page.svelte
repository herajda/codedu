<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';

  let id = $page.params.id;
  $: if ($page.params.id !== id) { id = $page.params.id; load(); }

  let students: any[] = [];
  let assignments: any[] = [];
  let scores: any[] = [];
  let loading = true;
  let err = '';

  // UI state
  let search = '';
  type SortKey = 'name' | 'total';
  let sortKey: SortKey = 'total';
  let sortDir: 'asc' | 'desc' = 'desc';

  async function load() {
    loading = true; err = '';
    try {
      const data = await apiJSON(`/api/classes/${id}/progress`);
      students = data.students ?? [];
      assignments = data.assignments ?? [];
      scores = data.scores ?? [];
    } catch (e: any) { err = e.message }
    loading = false;
  }

  onMount(load);

  function score(studentId: number, assignmentId: number) {
    const cell = scores.find((c: any) => c.student_id === studentId && c.assignment_id === assignmentId);
    return cell?.points ?? 0;
  }

  function total(studentId: number) {
    return assignments.reduce((sum, a) => sum + (score(studentId, a.id) || 0), 0);
  }

  function totalPossible() {
    return assignments.reduce((sum, a) => sum + (a.max_points || 0), 0);
  }

  $: filteredStudents = (students ?? [])
    .filter((s: any) => (s.name ?? s.email).toLowerCase().includes(search.toLowerCase()));

  $: visibleStudents = [...filteredStudents].sort((a: any, b: any) => {
    if (sortKey === 'name') {
      const an = (a.name ?? a.email ?? '').toLowerCase();
      const bn = (b.name ?? b.email ?? '').toLowerCase();
      if (an < bn) return sortDir === 'asc' ? -1 : 1;
      if (an > bn) return sortDir === 'asc' ? 1 : -1;
      return 0;
    }
    // total
    const ta = total(a.id);
    const tb = total(b.id);
    if (ta < tb) return sortDir === 'asc' ? -1 : 1;
    if (ta > tb) return sortDir === 'asc' ? 1 : -1;
    return 0;
  });

  function openStudent(studentId: number) {
    goto(`/classes/${id}/progress/${studentId}`);
  }
</script>

<div class="flex items-start justify-between gap-3 mb-4 flex-wrap">
  <div>
    <h1 class="text-2xl font-semibold">Class progress</h1>
    <p class="opacity-70 text-sm">{students.length} students · {assignments.length} assignments</p>
  </div>
  <div class="flex items-center gap-2 w-full sm:w-auto justify-end">
    <label class="input input-bordered input-sm flex items-center gap-2">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-4 h-4 opacity-70" aria-hidden="true"><path fill-rule="evenodd" d="M10.5 3.75a6.75 6.75 0 1 0 4.243 11.964l3.271 3.272a.75.75 0 1 0 1.06-1.06l-3.272-3.272A6.75 6.75 0 0 0 10.5 3.75ZM5.25 10.5a5.25 5.25 0 1 1 10.5 0 5.25 5.25 0 0 1-10.5 0Z" clip-rule="evenodd"/></svg>
      <input type="text" class="grow" placeholder="Search students" bind:value={search} />
    </label>
    <div class="dropdown dropdown-end">
      <button type="button" class="btn btn-sm">Sort</button>
      <ul class="dropdown-content menu bg-base-200 rounded-box z-[1] w-48 p-2 shadow">
        <li><button type="button" class={sortKey==='total' ? 'active' : ''} on:click={() => sortKey='total'}>By total</button></li>
        <li><button type="button" class={sortKey==='name' ? 'active' : ''} on:click={() => sortKey='name'}>By name</button></li>
        <li class="mt-1"><button type="button" on:click={() => sortDir = sortDir==='asc' ? 'desc' : 'asc'}>Direction: {sortDir}</button></li>
      </ul>
    </div>
  </div>
</div>

{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <div class="card-elevated p-4 overflow-x-auto">
    <table class="table table-zebra">
      <thead class="sticky top-16 z-20 bg-base-200">
        <tr>
          <th class="min-w-40 sm:min-w-52 sticky left-0 z-10 bg-base-200">Student</th>
          {#each assignments as a}
            <th class="min-w-16 sm:min-w-20 text-center">
              <div class="truncate" title={a.title}>{a.title}</div>
              <div class="text-xs opacity-70">{a.max_points}</div>
            </th>
          {/each}
          <th class="min-w-36 sm:min-w-48 text-right">Total</th>
        </tr>
      </thead>
      <tbody>
        {#each visibleStudents as s (s.id)}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <tr class="hover bg-base-100/10 cursor-pointer" on:click={() => openStudent(s.id)}>
            <td class="sticky left-0 bg-base-200/60 backdrop-blur supports-[backdrop-filter]:bg-base-200/50">
              <div class="font-medium truncate"><a class="link" href={`/classes/${id}/progress/${s.id}`}>{s.name ?? s.email}</a></div>
              <div class="text-xs opacity-70 truncate">{s.email}</div>
            </td>
            {#each assignments as a}
              {#key `${s.id}-${a.id}`}
                <td class="text-center">
                  <span class="text-sm">{score(s.id, a.id)}</span>
                </td>
              {/key}
            {/each}
            <td class="text-right">
              <div class="flex flex-col items-end gap-1">
                <span class="font-semibold">{total(s.id)}/{totalPossible()}</span>
                <progress class="progress progress-primary w-32" value={total(s.id)} max={totalPossible()}></progress>
              </div>
            </td>
          </tr>
        {/each}
        {#if !visibleStudents.length}
          <tr><td colspan={assignments.length + 2}><i>No students</i></td></tr>
        {/if}
      </tbody>
    </table>
  </div>
{/if}

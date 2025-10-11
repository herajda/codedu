<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { t, translator } from '$lib/i18n';

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

  let translate;
  $: translate = $translator;

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

  function percent(studentId: number, assignmentId: number) {
    const a = assignments.find((x: any) => x.id === assignmentId);
    const max = a?.max_points ?? 0;
    const pts = score(studentId, assignmentId) || 0;
    return max > 0 ? Math.round((pts / max) * 100) : 0;
  }

  function completionRatio(studentId: number) {
    if (!assignments.length) return 0;
    const completed = assignments.filter((a: any) => score(studentId, a.id) >= (a.max_points ?? 0)).length;
    return completed / assignments.length;
  }

  function classAverageCompletion() {
    if (!visibleStudents.length) return 0;
    const sum = visibleStudents.reduce((acc: number, s: any) => acc + completionRatio(s.id), 0);
    return sum / visibleStudents.length;
  }

  function classAveragePointsPercent() {
    const totalMax = totalPossible();
    if (!visibleStudents.length || !totalMax) return 0;
    const sum = visibleStudents.reduce((acc: number, s: any) => acc + (total(s.id) / totalMax), 0);
    return (sum / visibleStudents.length) * 100;
  }

  function heatClass(pct: number) {
    if (pct >= 95) return 'bg-success/20';
    if (pct >= 80) return 'bg-success/10';
    if (pct >= 60) return 'bg-warning/10';
    if (pct > 0) return 'bg-error/10';
    return '';
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
    <h1 class="text-2xl font-semibold">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::class_progress_title')}</h1>
    <p class="opacity-70 text-sm">{students.length} {translate('frontend/src/routes/classes/[id]/progress/+page.svelte::label_students_plural')} · {assignments.length} {translate('frontend/src/routes/classes/[id]/progress/+page.svelte::label_assignments_plural')}</p>
  </div>
  <div class="flex items-center gap-2 w-full sm:w-auto justify-end">
    <label class="input input-bordered input-sm flex items-center gap-2">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-4 h-4 opacity-70" aria-hidden="true"><path fill-rule="evenodd" d="M10.5 3.75a6.75 6.75 0 1 0 4.243 11.964l3.271 3.272a.75.75 0 1 0 1.06-1.06l-3.272-3.272A6.75 6.75 0 0 0 10.5 3.75ZM5.25 10.5a5.25 5.25 0 1 1 10.5 0 5.25 5.25 0 0 1-10.5 0Z" clip-rule="evenodd"/></svg>
      <input type="text" class="grow" placeholder={translate('frontend/src/routes/classes/[id]/progress/+page.svelte::search_students_placeholder')} bind:value={search} />
    </label>
    <div class="dropdown dropdown-end">
      <button type="button" class="btn btn-sm">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_button')}</button>
      <ul class="dropdown-content menu bg-base-200 rounded-box z-[1] w-48 p-2 shadow">
        <li><button type="button" class={sortKey==='total' ? 'active' : ''} on:click={() => sortKey='total'}>{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_by_total')}</button></li>
        <li><button type="button" class={sortKey==='name' ? 'active' : ''} on:click={() => sortKey='name'}>{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_by_name')}</button></li>
        <li class="mt-1"><button type="button" on:click={() => sortDir = sortDir==='asc' ? 'desc' : 'asc'}>{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_direction_label')}{sortDir === 'asc' ? translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_direction_asc') : translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_direction_desc')}</button></li>
      </ul>
    </div>
  </div>
</div>

{#if loading}
  <p>{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::loading_message')}</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-4">
    <div class="card-elevated p-4">
      <div class="text-xs uppercase opacity-70">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::average_completion_title')}</div>
      <div class="text-xl font-semibold">{Math.round(classAverageCompletion()*100)}%</div>
    </div>
    <div class="card-elevated p-4">
      <div class="text-xs uppercase opacity-70">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::average_score_title')}</div>
      <div class="text-xl font-semibold">{Math.round(classAveragePointsPercent())}%</div>
      <div class="text-xs opacity-70">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::of_total_possible_label')}</div>
    </div>
    <div class="card-elevated p-4">
      <div class="text-xs uppercase opacity-70">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::total_points_title')}</div>
      <div class="text-xl font-semibold">{visibleStudents.reduce((acc: number, s: any) => acc + total(s.id), 0)}/{visibleStudents.length * totalPossible()}</div>
    </div>
  </section>

  <div class="card-elevated p-0">
    <div class="relative overflow-auto max-h-[70vh]">
      <table class="table table-compact w-full">
        <thead>
          <tr>
            <th class="min-w-28 w-28 sm:min-w-40 sm:w-40 sticky left-0 top-0 z-40 bg-base-200">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::table_header_student')}</th>
            {#each assignments as a}
              <th class="min-w-14 sm:min-w-16 text-center sticky top-0 z-20 bg-base-200">
                <div class="font-medium whitespace-normal break-words leading-snug text-xs sm:text-sm" title={a.title}>{a.title}</div>
                <div class="text-xs opacity-70">{a.max_points}</div>
              </th>
            {/each}
            <th class="min-w-28 sm:min-w-36 text-right sticky top-0 z-20 bg-base-200">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::table_header_total')}</th>
          </tr>
        </thead>
        <tbody>
          {#each visibleStudents as s (s.id)}
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <tr class="hover bg-base-100/10 cursor-pointer" on:click={() => openStudent(s.id)}>
              <td class="sticky left-0 z-30 min-w-28 w-28 sm:min-w-40 sm:w-40 bg-base-200">
                <div class="font-medium truncate"><a class="link" href={`/classes/${id}/progress/${s.id}`}>{s.name ?? s.email}</a></div>
                <div class="text-xs opacity-70">{Math.round(completionRatio(s.id)*100)}% {translate('frontend/src/routes/classes/[id]/progress/+page.svelte::completion_status_complete')}</div>
              </td>
              {#each assignments as a}
                {#key `${s.id}-${a.id}`}
                  {#if a.max_points > 0}
                    <td class={`text-center ${heatClass(percent(s.id, a.id))}`} title={`${percent(s.id, a.id)}%`}> 
                      <div class="flex items-center justify-center gap-1">
                        <span class="text-xs">{score(s.id, a.id)}</span>
                        <span class="text-[10px] opacity-60">/{a.max_points}</span>
                      </div>
                    </td>
                  {:else}
                    <td class="text-center">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::no_score_dash')}</td>
                  {/if}
                {/key}
              {/each}
              <td class="text-right">
                <div class="flex flex-col items-end gap-1">
                  <span class="font-semibold text-sm">{total(s.id)}/{totalPossible()}</span>
                  <progress class="progress progress-primary w-24" value={total(s.id)} max={totalPossible()}></progress>
                </div>
              </td>
            </tr>
          {/each}
          {#if !visibleStudents.length}
            <tr><td colspan={assignments.length + 2}><i>{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::no_students_message')}</i></td></tr>
          {/if}
        </tbody>
      </table>
    </div>
  </div>
{/if}

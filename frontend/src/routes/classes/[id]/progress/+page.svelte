<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { t, translator } from '$lib/i18n';
  import { Search, ArrowUpDown, Users, GraduationCap, Trophy, Activity, Percent, ArrowRight, CheckCircle2, AlertCircle } from 'lucide-svelte';
  import { fade } from 'svelte/transition';

  let id = $page.params.id;
  $: if ($page.params.id !== id) { id = $page.params.id; load(); }

  let students: any[] = [];
  let assignments: any[] = [];
  let scores: any[] = [];
  let loading = true;
  let err = '';
  let cls: any = null;

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
      
      // Load class details for header
      const classData = await apiJSON(`/api/classes/${id}`);
      cls = classData?.class ?? classData ?? null;
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
    if (pct >= 95) return 'bg-success/20 text-success font-bold shadow-inner';
    if (pct >= 80) return 'bg-success/10 text-success';
    if (pct >= 60) return 'bg-warning/10 text-warning';
    if (pct > 0) return 'bg-error/10 text-error';
    return 'opacity-30';
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

  // Deterministic placeholder avatar helper
  function placeholderAvatar(seed: string): string {
    let h = 0;
    for (let i = 0; i < seed.length; i++) h = ((h << 5) - h + seed.charCodeAt(i)) >>> 0;
    const n = (h % 50) + 1;
    return `/avatars/a${n}.svg`;
  }
  function avatarFor(s: any): string {
    return s.avatar ?? placeholderAvatar(String(s.id ?? s.email ?? 'x'));
  }
</script>

<svelte:head>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@100..900&display=swap" rel="stylesheet">
  <title>{cls?.name ? `${cls.name} | CodEdu` : 'Progress | CodEdu'}</title>
</svelte:head>

<div class="classes-progress-page flex flex-col min-h-0 h-full w-full overflow-hidden">
  {#if loading}
    <div class="flex justify-center mt-8"><span class="loading loading-dots loading-lg"></span></div>
  {:else if err}
    <div class="alert alert-error" in:fade>
      <AlertCircle size={20} />
      <span>{err}</span>
    </div>
  {:else}
    <!-- Premium Class Header -->
    <section class="relative overflow-hidden bg-base-100 rounded-3xl border border-base-200 shadow-xl shadow-base-300/30 mb-8 p-6 sm:p-10">
    <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
    <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none"></div>
    <div class="relative flex flex-col md:flex-row items-center justify-between gap-6">
      <div class="flex-1 text-center md:text-left">
        <h1 class="text-3xl sm:text-4xl font-black tracking-tight mb-2">
          {cls?.name} <span class="text-primary/40">/</span> {translate('frontend/src/routes/classes/[id]/progress/+page.svelte::class_progress_title')}
        </h1>
        <p class="text-base-content/60 font-medium max-w-xl mx-auto md:mx-0">
          {students.length} {translate('frontend/src/routes/classes/[id]/progress/+page.svelte::label_students_plural')} · {assignments.length} {translate('frontend/src/routes/classes/[id]/progress/+page.svelte::label_assignments_plural')}
        </p>
      </div>
      
      <div class="hidden lg:flex items-center gap-4">
        <div class="w-12 h-12 bg-primary/10 text-primary rounded-xl flex items-center justify-center shadow-lg shadow-primary/10">
          <GraduationCap size={24} />
        </div>
      </div>
    </div>
  </section>

  <!-- Stats Section -->
  <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-8">
    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-success/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::average_completion_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-success/10 text-success flex items-center justify-center group-hover:bg-success group-hover:text-success-content transition-all duration-300">
          <CheckCircle2 size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{Math.round(classAverageCompletion()*100)}%</div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-info/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::average_score_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-info/10 text-info flex items-center justify-center group-hover:bg-info group-hover:text-info-content transition-all duration-300">
          <Percent size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{Math.round(classAveragePointsPercent())}%</div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-warning/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::total_points_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-warning/10 text-warning flex items-center justify-center group-hover:bg-warning group-hover:text-warning-content transition-all duration-300">
          <Activity size={20} />
        </div>
        <div class="text-lg font-black tabular-nums leading-tight">
          {visibleStudents.reduce((acc: number, s: any) => acc + total(s.id), 0)}
          <span class="text-xs opacity-40 font-normal block">/ {visibleStudents.length * totalPossible()}</span>
        </div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-primary/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::label_students_plural')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center group-hover:bg-primary group-hover:text-primary-content transition-all duration-300">
          <Users size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{visibleStudents.length}</div>
      </div>
    </div>
  </section>

  <!-- Filters & List Section -->
  <div class="flex flex-col flex-1 min-h-0 min-w-0">
    <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 mb-6 px-2">
      <div class="flex items-center gap-3 w-full lg:w-auto">
        <div class="relative flex items-center w-full sm:w-auto">
          <Search size={14} class="absolute left-3 opacity-40" />
          <input 
            type="text" 
            class="input input-sm bg-base-100 border-base-200 focus:border-primary/30 w-full sm:w-64 pl-9 rounded-xl font-medium text-xs h-9" 
            placeholder={translate('frontend/src/routes/classes/[id]/progress/+page.svelte::search_students_placeholder')} 
            bind:value={search} 
          />
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-3 justify-end w-full lg:w-auto">
        <div class="dropdown dropdown-end">
          <button type="button" class="btn btn-sm bg-base-100 border-base-200 hover:bg-base-200 rounded-xl h-9 px-4 gap-2 border shadow-sm" tabindex="0">
            <ArrowUpDown size={14} class="opacity-60" />
            <span class="text-[10px] font-black uppercase tracking-widest leading-none">
              {sortKey === 'total' 
                ? translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_by_total')
                : translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_by_name')}
              ({sortDir === 'asc' ? translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_direction_asc') : translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_direction_desc')})
            </span>
          </button>
          <ul class="dropdown-content menu bg-base-100 rounded-2xl z-[50] w-56 p-2 shadow-2xl border border-base-200 mt-2" tabindex="0">
            <li><button type="button" class={sortKey==='total' ? 'active' : ''} on:click={() => sortKey='total'} class:bg-primary={sortKey==='total'} class:text-primary-content={sortKey==='total'}>{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_by_total')}</button></li>
            <li><button type="button" class={sortKey==='name' ? 'active' : ''} on:click={() => sortKey='name'} class:bg-primary={sortKey==='name'} class:text-primary-content={sortKey==='name'}>{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_by_name')}</button></li>
            <div class="divider my-0 opacity-10"></div>
            <li><button type="button" on:click={() => sortDir = sortDir==='asc' ? 'desc' : 'asc'}>{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_direction_label')}{sortDir === 'asc' ? translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_direction_asc') : translate('frontend/src/routes/classes/[id]/progress/+page.svelte::sort_direction_desc')}</button></li>
          </ul>
        </div>
      </div>
    </div>

    <div class="bg-base-100 p-0 rounded-[2rem] border border-base-200 shadow-sm overflow-hidden backdrop-blur-sm flex flex-col flex-1 min-h-0">
      <div class="relative overflow-auto custom-scrollbar flex-1 min-h-0 w-full">
        <table class="table table-compact w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <th class="p-5 bg-base-200 sticky top-0 left-0 z-50 border-b border-base-200 shadow-[4px_0_8px_-4px_rgba(0,0,0,0.05)]">
                <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::table_header_student')}</span>
              </th>
              {#each assignments as a}
                <th class="p-4 text-center sticky top-0 z-30 bg-base-200 border-b border-base-200 min-w-24">
                  <div class="font-black text-[10px] uppercase tracking-wider leading-snug mb-1 line-clamp-2" title={a.title}>{a.title}</div>
                  <div class="text-[9px] font-bold opacity-30">{a.max_points} pts</div>
                </th>
              {/each}
              <th class="p-5 text-right sticky top-0 right-0 z-40 bg-base-200 border-b border-base-200 min-w-32 shadow-[-4px_0_8px_-4px_rgba(0,0,0,0.05)]">
                <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::table_header_total')}</span>
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-base-200">
            {#each visibleStudents as s (s.id)}
              <tr class="group hover:bg-primary/5 transition-colors cursor-pointer" on:click={() => openStudent(s.id)}>
                <td class="p-5 sticky left-0 z-20 bg-base-100 transition-colors shadow-[4px_0_8px_-4px_rgba(0,0,0,0.05)]">
                  <div class="flex items-center gap-3">
                    <div class="avatar shrink-0">
                      <div class="w-10 h-10 rounded-2xl ring-2 ring-base-200 shadow-sm bg-base-200 overflow-hidden group-hover:ring-primary/20 transition-all">
                        <img src={avatarFor(s)} alt="" class="w-full h-full object-cover" />
                      </div>
                    </div>
                    <div class="min-w-0">
                      <div class="font-black text-sm tracking-tight text-base-content group-hover:text-primary transition-colors truncate">{s.name ?? s.email}</div>
                      <div class="flex items-center gap-2 mt-1">
                        <div class="w-16 h-1 bg-base-200 rounded-full overflow-hidden">
                          <div class="h-full bg-success transition-all duration-500" style={`width: ${Math.round(completionRatio(s.id)*100)}%`}></div>
                        </div>
                        <span class="text-[9px] font-bold uppercase tracking-widest opacity-40">{Math.round(completionRatio(s.id)*100)}%</span>
                      </div>
                    </div>
                  </div>
                </td>
                {#each assignments as a}
                  {#key `${s.id}-${a.id}`}
                    {#if a.max_points > 0}
                      <td class={`p-4 text-center transition-all duration-300 ${heatClass(percent(s.id, a.id))}`} title={`${percent(s.id, a.id)}%`}> 
                        <div class="flex flex-col items-center">
                          <span class="text-sm font-black">{score(s.id, a.id)}</span>
                          <span class="text-[9px] font-bold opacity-40">/ {a.max_points}</span>
                        </div>
                      </td>
                    {:else}
                      <td class="p-4 text-center opacity-20">{translate('frontend/src/routes/classes/[id]/progress/+page.svelte::no_score_dash')}</td>
                    {/if}
                  {/key}
                {/each}
                <td class="p-5 text-right sticky right-0 z-20 bg-base-100 transition-colors shadow-[-4px_0_8px_-4px_rgba(0,0,0,0.05)]">
                  <div class="flex flex-col items-end gap-1.5">
                    <div class="font-black text-sm tabular-nums">
                      {total(s.id)} <span class="text-[10px] opacity-40 font-bold">/ {totalPossible()}</span>
                    </div>
                    <div class="w-24 h-1.5 rounded-full bg-base-200 overflow-hidden">
                       <div class="h-full bg-primary transition-all duration-500 rounded-full shadow-[0_0_8px_rgba(var(--p),0.5)]" style={`width: ${totalPossible() > 0 ? (total(s.id)/totalPossible()*100) : 0}%`}></div>
                    </div>
                  </div>
                </td>
              </tr>
            {/each}
            {#if !visibleStudents.length}
              <tr>
                <td colspan={assignments.length + 2} class="p-20 text-center">
                  <div class="w-16 h-16 rounded-full bg-base-200 flex items-center justify-center mx-auto mb-4 opacity-30">
                    <Users size={32} />
                  </div>
                  <p class="text-sm font-black uppercase tracking-widest opacity-30">
                    {translate('frontend/src/routes/classes/[id]/progress/+page.svelte::no_students_message')}
                  </p>
                </td>
              </tr>
            {/if}
          </tbody>
        </table>
      </div>
    </div>
    </div>
  {/if}
</div>

<style>
  .classes-progress-page {
    font-family: 'Outfit', sans-serif;
  }

  .custom-scrollbar::-webkit-scrollbar {
    height: 8px;
    width: 8px;
  }
  .custom-scrollbar::-webkit-scrollbar-track {
    background: transparent;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb {
    background: hsl(var(--bc) / 0.1);
    border-radius: 10px;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb:hover {
    background: hsl(var(--bc) / 0.2);
  }

  /* Sticky column shadow effects */
  td.sticky:after {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    width: 20px;
    pointer-events: none;
    transition: opacity 0.3s;
  }

  /* Table borders and details */
  table {
    border-collapse: separate;
    border-spacing: 0;
  }
</style>

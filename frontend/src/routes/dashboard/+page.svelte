
<script lang="ts">
  import { onMount, tick, onDestroy } from 'svelte';
  import { apiJSON, apiFetch } from '$lib/api';
  import AdminPanel from '$lib/AdminPanel.svelte';
  import { formatDateTime } from "$lib/date";
  import { page } from '$app/stores';
  import { classesStore } from '$lib/stores/classes';
  import { BookOpen, CalendarClock, Trophy, Inbox, Users, LayoutGrid, MessageSquare, FolderOpen, ListChecks } from 'lucide-svelte';
  import { t, translator } from '$lib/i18n';

  let translate;
  $: translate = $translator;

  let role = '';
  let classes:any[] = [];
  let submissions:any[] = [];
  let upcoming:any[] = [];
  let loading = true;
  let err = '';
  let newClassName = '';
  let showNewClassInput = false;
  let newClassInput: HTMLInputElement | null = null;

  function percent(done:number,total:number){
    return total ? Math.round((done/total)*100) : 0;
  }

  let studentStats: any = null;
  let teacherStats: any = null;

  async function refreshData() {
    try {
      loading = true;
      err = '';
      // We still need /api/me to know who we are initially, or we can rely on dashboard returning role
      // But the current code uses /api/me first. Let's keep it for safety or just use dashboard.
      // Actually dashboard returns role.
      
      const data = await apiJSON('/api/dashboard');
      role = data.role;
      
      if (role === 'admin') {
        loading = false;
        return;
      }

      classes = data.classes || [];
      
      if (role === 'student') {
        studentStats = {
          totalClasses: data.student_stats.total_classes,
          totalAssignments: data.student_stats.total_assignments,
          completedAssignments: data.student_stats.completed_assignments,
          pointsEarned: data.student_stats.points_earned,
          pointsTotal: data.student_stats.points_total
        };
        upcoming = (data.upcoming || []).map((u:any) => ({
          ...u,
          class: u.class_name
        }));
        // Map backend fields to frontend expectations if needed
        classes = classes.map(c => ({
          ...c,
          completed: c.completed_count,
          assignments: new Array(c.assignments_count).fill(0), // Dummy for length checks if needed, or update UI to use counts
          assignmentProgress: c.assignment_progress || []
        }));
      } else if (role === 'teacher') {
        teacherStats = {
          studentsTotal: data.teacher_stats.students_total,
          activeAssignments: data.teacher_stats.active_assignments
        };
        classes = classes.map(c => ({
          ...c,
          students: new Array(c.students_count).fill(0), // Dummy
          assignments: c.assignment_progress || [], // Use the progress list which has titles
          progress: c.assignment_progress.map((p:any) => ({...p, done: p.done_count})), // Map done_count to done for UI compatibility
          notFinished: c.not_finished_count
        }));
      }
      
      // Force Svelte reactivity
      classes = [...classes];
    } catch(e:any){
      err = e.message;
    }
    loading = false;
  }

  onMount(async () => {
    await refreshData();
  });

  // Subscribe to page changes to refresh data when navigating to dashboard
  let previousPathname = '';
  const unsubscribe = page.subscribe(($page) => {
    const currentPathname = $page.url.pathname;
    // Only refresh if we're navigating TO the dashboard and it wasn't the previous path
    if (currentPathname === '/dashboard' && previousPathname !== '/dashboard' && previousPathname !== '') {
      refreshData();
    }
    previousPathname = currentPathname;
  });

  onDestroy(() => {
    unsubscribe();
  });

  async function createClass(){
    try{
      const cl = await apiJSON('/api/classes', {
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({name:newClassName})
      });
      // Add the new class to both local state and the store
      // For dashboard we might just reload to get proper stats structure
      // or try to append a dummy structure
      newClassName='';
      showNewClassInput = false;
      await refreshData();
    }catch(e:any){ err = e.message; }
  }

  async function showInput() {
    showNewClassInput = true;
    await tick();
    newClassInput?.focus();
  }

  // Derived small stats for nicer UI - NOW FROM API
  $: totalClasses = classes.length;
  // studentStats and teacherStats are now populated directly from API

</script>

{#if loading}
  <div class="flex justify-center mt-8"><span class="loading loading-dots loading-lg"></span></div>
{:else}

  {#if role === 'student'}
    <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 mb-6">
      <div class="card-elevated p-4 flex items-center gap-3">
        <BookOpen class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_classes')}</div>
          <div class="text-xl font-semibold">{totalClasses}</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <Trophy class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_progress')}</div>
          <div class="text-xl font-semibold">{percent(studentStats?.completedAssignments ?? 0, studentStats?.totalAssignments ?? 0)}%</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <CalendarClock class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_upcoming')}</div>
          <div class="text-xl font-semibold">{upcoming.length}</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <Inbox class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_points')}</div>
          <div class="text-xl font-semibold">{studentStats?.pointsEarned ?? 0}/{studentStats?.pointsTotal ?? 0}</div>
        </div>
      </div>
    </section>

    <section class="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
      {#each classes as c}
        <a href={`/classes/${c.id}/overview`} class="block no-underline text-current">
          <div class="card-elevated p-5 h-full">
            <div class="flex items-center justify-between mb-3">
              <h2 class="font-semibold text-lg truncate">{c.name}</h2>
              <span class="badge badge-outline">
                {(c.assignments ?? []).length} {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_assignments_label', {count: (c.assignments ?? []).length})}
              </span>
            </div>
            <div class="flex items-center gap-3 mb-3">
              <div class="flex-1 h-2 rounded-full bg-base-300/60 overflow-hidden">
                <div class="h-full bg-gradient-to-r from-cyan-400 via-sky-400 to-teal-400" style={`width: ${percent(c.completed, (c.assignments ?? []).length)}%`}></div>
              </div>
              <span class="text-sm whitespace-nowrap">{c.completed}/{(c.assignments ?? []).length}</span>
            </div>
            <ul class="space-y-2">
              {#each (c.assignmentProgress ?? []).slice(0, 3) as a}
                <li class="flex items-center gap-2 text-sm">
                  <span class={`w-2 h-2 rounded-full ${a.done ? 'bg-teal-400' : 'bg-base-300'}`}></span>
                  <span class="truncate">{a.title}</span>
                </li>
              {/each}
              {#if (c.assignmentProgress ?? []).length === 0}
                <li class="text-sm opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_no_assignments')}</li>
              {/if}
            </ul>
          </div>
        </a>
      {/each}
    </section>

    <section class="mt-8 grid gap-6 lg:grid-cols-2">
      <div class="card-elevated p-5">
        <div class="flex items-center justify-between mb-3">
          <h3 class="font-semibold">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_upcoming_deadlines')}</h3>
        </div>
        <ul class="divide-y divide-base-300/60">
          {#each upcoming as a}
            <li>
              <a href={`/assignments/${a.id}`} class="flex items-center justify-between py-3 hover:opacity-90">
                <div class="min-w-0">
                  <div class="font-medium truncate">{a.title}</div>
                  <div class="text-sm opacity-70 truncate">{a.class}</div>
                </div>
                <span class="badge badge-outline">{formatDateTime(a.deadline)}</span>
              </a>
            </li>
          {/each}
          {#if !upcoming.length}
            <li class="py-3 text-sm opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_no_upcoming_deadlines')}</li>
          {/if}
        </ul>
      </div>
    </section>
  {:else if role === 'teacher'}
    <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-6">
      <div class="card-elevated p-4 flex items-center gap-3">
        <BookOpen class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_classes')}</div>
          <div class="text-xl font-semibold">{totalClasses}</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <Users class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_students_label', {count: teacherStats?.studentsTotal ?? 0})}</div>
          <div class="text-xl font-semibold">{teacherStats?.studentsTotal ?? 0}</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <CalendarClock class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_active_assignments')}</div>
          <div class="text-xl font-semibold">{teacherStats?.activeAssignments ?? 0}</div>
        </div>
      </div>
    </section>

    <section class="mb-6">
      <div class="card-elevated p-4 flex items-center justify-between gap-3">
        <div class="flex items-center gap-3 min-w-0">
          <div class="p-2 bg-primary/10 rounded-lg">
            <LayoutGrid class="w-5 h-5 text-primary" aria-hidden="true" />
          </div>
          <div class="min-w-0">
            <div class="text-xs uppercase tracking-wide text-base-content/60">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_teachers')}</div>
            <div class="text-sm text-base-content/70 whitespace-nowrap truncate">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_collaborate_with_colleagues')}</div>
          </div>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          <a href="/teachers/forum" class="btn btn-outline btn-sm"><MessageSquare class="w-4 h-4 mr-1" />{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_forum_button')}</a>
          <a href="/teachers/files" class="btn btn-outline btn-sm"><FolderOpen class="w-4 h-4 mr-1" />{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_files_button')}</a>
          <a href="/teachers/assignments" class="btn btn-outline btn-sm"><ListChecks class="w-4 h-4 mr-1" />{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_assignments_button')}</a>
        </div>
      </div>
    </section>

    <section class="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
      {#each classes as c}
        <a href={`/classes/${c.id}`} class="block no-underline text-current">
          <div class="card-elevated p-5 h-full">
            <div class="flex items-center justify-between mb-2">
              <h2 class="font-semibold text-lg truncate">{c.name}</h2>
              <span class="badge badge-ghost">{c.students.length} {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_students_label', {count: c.students.length})}</span>
            </div>
            <ul class="space-y-2">
              {#each (c.assignments ?? []).slice(0,5) as a}
                <li class="flex items-center gap-3">
                  <span class="truncate flex-1">{a.title}</span>
                  <progress class="progress progress-primary flex-1" value={c.progress.find((x:any)=>x.id===a.id)?.done || 0} max={c.students.length}></progress>
                  <span class="text-sm whitespace-nowrap">{c.progress.find((x:any)=>x.id===a.id)?.done || 0}/{c.students.length}</span>
                </li>
              {/each}
              {#if !(c.assignments ?? []).length}
                <li class="text-sm opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_no_assignments')}</li>
              {/if}
              {#if (c.assignments ?? []).length > 5}
                <li class="text-xs opacity-70">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_assignments_not_finished_by_all_students', {n: c.notFinished})}</li>
              {/if}
            </ul>
          </div>
        </a>
      {/each}
    </section>

    <section class="mt-6">
      {#if showNewClassInput}
        <form class="flex gap-2 max-w-sm" on:submit|preventDefault={createClass}>
          <input class="input input-bordered flex-1" placeholder={translate('frontend/src/routes/dashboard/+page.svelte::dashboard_new_class_placeholder')} bind:value={newClassName} bind:this={newClassInput} required />
          <button class="btn" type="submit">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_create_button')}</button>
        </form>
      {:else}
        <button class="btn" on:click={showInput} type="button">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_create_new_class_button')}</button>
      {/if}
    </section>
  {:else if role === 'admin'}
    <AdminPanel />
  {/if}

  {#if err}
    <p class="text-error mt-4">{err}</p>
  {/if}
{/if}

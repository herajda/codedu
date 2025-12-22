
<script lang="ts">
  import { onMount, tick, onDestroy } from 'svelte';
  import { apiJSON, apiFetch } from '$lib/api';
  import AdminPanel from '$lib/AdminPanel.svelte';
  import { formatDateTime } from "$lib/date";
  import { page } from '$app/stores';
  import { classesStore } from '$lib/stores/classes';
  import { auth } from '$lib/auth';
  import { BookOpen, CalendarClock, Trophy, Inbox, Users, LayoutGrid, MessageSquare, FolderOpen, ListChecks, ArrowRight, Sparkles, ChevronRight, Plus } from 'lucide-svelte';
  import { t, translator } from '$lib/i18n';
  import PromptModal from '$lib/components/PromptModal.svelte';

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
  let promptModal: PromptModal;

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
      const newRole = data.role;
      let newClasses = data.classes || [];
      
      if (newRole === 'admin') {
        role = newRole;
        loading = false;
        return;
      }

      if (newRole === 'student') {
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
        newClasses = newClasses.map((c: any) => ({
          ...c,
          completed: c.completed_count,
          assignments: c.assignment_progress || [],
          assignmentProgress: c.assignment_progress || []
        }));
      } else if (newRole === 'teacher') {
        teacherStats = {
          studentsTotal: data.teacher_stats.students_total,
          activeAssignments: data.teacher_stats.active_assignments
        };
        newClasses = newClasses.map((c: any) => ({
          ...c,
          assignments: c.assignment_progress || [],
          progress: (c.assignment_progress || []).map((p:any) => ({...p, done: p.done_count})),
          notFinished: c.not_finished_count
        }));
      }
      
      role = newRole;
      classes = newClasses;
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

  async function createClass(name?: string){
    const classNameToCreate = name || newClassName;
    if (!classNameToCreate.trim()) return;
    try{
      const cl = await apiJSON('/api/classes', {
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({name:classNameToCreate})
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
    if (!promptModal) return;
    const name = await promptModal.open({
      title: translate('frontend/src/routes/dashboard/+page.svelte::dashboard_create_new_class_button'),
      label: translate('frontend/src/routes/dashboard/+page.svelte::dashboard_new_class_placeholder'),
      confirmLabel: translate('frontend/src/routes/dashboard/+page.svelte::dashboard_create_button'),
    });
    if (name) {
      await createClass(name);
    }
  }

  // Derived small stats for nicer UI - NOW FROM API
  $: totalClasses = classes.length;
  // studentStats and teacherStats are now populated directly from API

</script>

{#if loading}
  <div class="flex justify-center mt-8"><span class="loading loading-dots loading-lg"></span></div>
{:else}

  {#if role === 'student'}
    <!-- Premium Header -->
    <section class="relative overflow-hidden bg-base-100 rounded-3xl border border-base-200 shadow-xl shadow-base-300/30 mb-8 p-6 sm:p-10">
      <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
      <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none"></div>
      <div class="relative flex flex-col md:flex-row items-center gap-6">
        <div class="flex-1 text-center md:text-left">
          <h1 class="text-3xl sm:text-4xl font-black tracking-tight mb-2">
            {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_welcome_back', {name: ($auth as any)?.display_name || ($auth as any)?.name})}
          </h1>
          <p class="text-base-content/60 font-medium max-w-xl mx-auto md:mx-0">
            {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_student_subtitle')}
          </p>
        </div>
        <div class="hidden lg:flex items-center gap-4">
           <div class="flex -space-x-3">
             {#each classes.slice(0, 3) as c}
               <div class="w-10 h-10 rounded-full border-2 border-base-100 bg-base-200 flex items-center justify-center text-xs font-bold text-base-content/40">
                 {c.name.slice(0, 1)}
               </div>
             {/each}
           </div>
        </div>
      </div>
    </section>

    <!-- Stats Section -->
    <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-8">
      <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-primary/30 transition-all">
        <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_classes')}</div>
        <div class="flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center group-hover:bg-primary group-hover:text-primary-content transition-all duration-300">
            <BookOpen size={20} />
          </div>
          <div class="text-2xl font-black tabular-nums">{totalClasses}</div>
        </div>
      </div>

      <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-success/30 transition-all">
        <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_progress')}</div>
        <div class="flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-success/10 text-success flex items-center justify-center group-hover:bg-success group-hover:text-success-content transition-all duration-300">
            <Trophy size={20} />
          </div>
          <div>
            <div class="text-2xl font-black tabular-nums">{percent(studentStats?.completedAssignments ?? 0, studentStats?.totalAssignments ?? 0)}%</div>
            <div class="w-24 h-1 bg-success/10 rounded-full mt-1 overflow-hidden">
               <div class="h-full bg-success" style={`width: ${percent(studentStats?.completedAssignments ?? 0, studentStats?.totalAssignments ?? 0)}%`}></div>
            </div>
          </div>
        </div>
      </div>

      <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-warning/30 transition-all">
        <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_upcoming')}</div>
        <div class="flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-warning/10 text-warning flex items-center justify-center group-hover:bg-warning group-hover:text-warning-content transition-all duration-300">
            <CalendarClock size={20} />
          </div>
          <div class="text-2xl font-black tabular-nums">{upcoming.length}</div>
        </div>
      </div>

      <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-info/30 transition-all">
        <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_points')}</div>
        <div class="flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-info/10 text-info flex items-center justify-center group-hover:bg-info group-hover:text-info-content transition-all duration-300">
            <Inbox size={20} />
          </div>
          <div class="text-2xl font-black tabular-nums">{studentStats?.pointsEarned ?? 0} <span class="text-sm opacity-40 font-normal">/ {studentStats?.pointsTotal ?? 0}</span></div>
        </div>
      </div>
    </section>

    <!-- Classes Grid -->
    <div class="grid grid-cols-1 xl:grid-cols-12 gap-8">
      <div class="xl:col-span-8 space-y-6">
        <div class="flex items-center justify-between px-2">
          <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{translate('frontend/src/lib/Sidebar.svelte::classes_title')}</h2>
        </div>
        
        <div class="grid gap-6 md:grid-cols-2">
          {#each classes as c}
            <a href={`/classes/${c.id}/overview`} class="group block no-underline text-current">
              <div class="bg-base-100 p-6 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 transition-all h-full flex flex-col relative overflow-hidden">
                <div class="absolute top-0 right-0 w-1/4 h-1/4 bg-primary/5 rounded-bl-full pointer-events-none group-hover:scale-150 transition-transform duration-700"></div>
                
                <div class="flex items-center justify-between mb-5 relative">
                  <h3 class="font-black text-xl tracking-tight truncate group-hover:text-primary transition-colors pr-4">{c.name}</h3>
                  <div class="w-10 h-10 rounded-2xl bg-base-200 flex items-center justify-center group-hover:bg-primary/10 group-hover:text-primary transition-colors shrink-0">
                    <LayoutGrid size={18} />
                  </div>
                </div>

                <div class="space-y-4 flex-1">
                  <div class="flex items-center justify-between text-xs font-bold text-base-content/60">
                    <span>{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_progress')}</span>
                    <span class="tabular-nums font-black text-base-content">{c.completed}/{c.assignments_count}</span>
                  </div>
                  <div class="w-full h-2 rounded-full bg-base-200 overflow-hidden flex">
                    <div class="h-full bg-primary transition-all duration-500 rounded-full" style={`width: ${percent(c.completed, c.assignments_count)}%`}></div>
                  </div>

                  <div class="pt-2 space-y-2">
                    {#each (c.assignmentProgress ?? []).slice(0, 3) as a}
                      <div class="flex items-center gap-3 text-sm group/item">
                        <div class={`w-1.5 h-1.5 rounded-full shrink-0 ${a.done ? 'bg-success' : 'bg-base-300'}`}></div>
                        <span class="truncate opacity-70 group-hover/item:opacity-100 transition-opacity">{a.title}</span>
                      </div>
                    {/each}
                    {#if c.assignments_count === 0}
                      <div class="text-xs italic opacity-40 py-2">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_no_assignments')}</div>
                    {/if}
                  </div>
                </div>

                <div class="mt-6 pt-5 border-t border-base-300/30 flex items-center justify-between">
                  <span class="badge badge-sm badge-ghost font-black text-[9px] uppercase tracking-wider h-6">
                    {c.assignments_count} {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_assignments_label', {count: c.assignments_count})}
                  </span>
                  <div class="text-primary opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all">
                    <ArrowRight size={16} />
                  </div>
                </div>
              </div>
            </a>
          {/each}
        </div>
      </div>

      <!-- Sidebar: Upcoming Deadlines -->
      <div class="xl:col-span-4 space-y-6">
        <div class="flex items-center justify-between px-2">
          <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_upcoming_deadlines')}</h2>
        </div>

        <div class="bg-base-100 rounded-[2rem] border border-base-200 shadow-sm overflow-hidden">
          <div class="p-6 space-y-4">
            {#each upcoming as a}
              <a href={`/assignments/${a.id}`} class="flex items-center gap-4 p-4 rounded-2xl hover:bg-base-200/50 transition-colors group">
                <div class="w-12 h-12 rounded-xl bg-warning/10 text-warning flex flex-col items-center justify-center shrink-0">
                  <span class="text-[9px] font-black uppercase">{new Date(a.deadline).toLocaleString('default', { month: 'short' })}</span>
                  <span class="text-lg font-black leading-none">{new Date(a.deadline).getDate()}</span>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="font-black text-sm truncate group-hover:text-primary transition-colors">{a.title}</div>
                  <div class="text-xs opacity-50 truncate">{a.class}</div>
                </div>
                <ChevronRight size={16} class="opacity-0 group-hover:opacity-30 group-hover:translate-x-1 transition-all shrink-0" />
              </a>
            {:else}
               <div class="py-8 text-center space-y-3">
                 <div class="w-12 h-12 rounded-full bg-base-200 flex items-center justify-center mx-auto opacity-30">
                   <CalendarClock size={20} />
                 </div>
                 <p class="text-xs font-bold opacity-30 uppercase tracking-widest">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_no_upcoming_deadlines')}</p>
               </div>
            {/each}
          </div>
        </div>
      </div>
    </div>
  {:else if role === 'teacher'}
    <!-- Premium Header -->
    <section class="relative overflow-hidden bg-base-100 rounded-3xl border border-base-200 shadow-xl shadow-base-300/30 mb-8 p-6 sm:p-10">
      <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
      <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none"></div>
      <div class="relative flex flex-col md:flex-row items-center gap-8">
        <div class="flex-1 text-center md:text-left">
          <h1 class="text-3xl sm:text-4xl font-black tracking-tight mb-2">
            {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_welcome_back', {name: ($auth as any)?.display_name || ($auth as any)?.name})}
          </h1>
          <p class="text-base-content/60 font-medium max-w-xl mx-auto md:mx-0">
            {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_teacher_subtitle')}
          </p>
        </div>
        
        <!-- Teacher Toolbar integrated into header -->
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 w-full md:w-auto">
          <a href="/forum" class="group flex flex-col items-center gap-2 p-3 px-4 rounded-2xl bg-primary text-primary-content shadow-lg shadow-primary/20 hover:scale-105 transition-all text-center">
            <MessageSquare size={20} />
            <span class="text-[9px] font-black uppercase tracking-widest">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_forum_button')}</span>
          </a>
          <a href="/files" class="group flex flex-col items-center gap-2 p-3 px-4 rounded-2xl bg-base-200 hover:bg-base-300 transition-all text-center">
            <FolderOpen size={20} class="opacity-70 group-hover:opacity-100" />
            <span class="text-[9px] font-black uppercase tracking-widest opacity-60 group-hover:opacity-100">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_files_button')}</span>
          </a>
          <a href="/teachers/assignments" class="group flex flex-col items-center gap-2 p-3 px-4 rounded-2xl bg-base-200 hover:bg-base-300 transition-all text-center">
            <ListChecks size={20} class="opacity-70 group-hover:opacity-100" />
            <span class="text-[9px] font-black uppercase tracking-widest opacity-60 group-hover:opacity-100">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_assignments_button')}</span>
          </a>
          <div class="group flex flex-col items-center gap-2 p-3 px-4 rounded-2xl bg-base-200/50 opacity-50 cursor-not-allowed text-center">
            <Users size={20} />
            <span class="text-[9px] font-black uppercase tracking-widest">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_teachers')}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- Stats Section -->
    <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-8">
      <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-primary/30 transition-all">
        <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_classes')}</div>
        <div class="flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center group-hover:bg-primary group-hover:text-primary-content transition-all duration-300">
            <BookOpen size={20} />
          </div>
          <div class="text-2xl font-black tabular-nums">{totalClasses}</div>
        </div>
      </div>

      <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-success/30 transition-all">
        <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/lib/AdminPanel.svelte::students_stat_title')}</div>
        <div class="flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-success/10 text-success flex items-center justify-center group-hover:bg-success group-hover:text-success-content transition-all duration-300">
            <Users size={20} />
          </div>
          <div class="text-2xl font-black tabular-nums">{teacherStats?.studentsTotal ?? 0}</div>
        </div>
      </div>

      <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-warning/30 transition-all">
        <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_active_assignments')}</div>
        <div class="flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-warning/10 text-warning flex items-center justify-center group-hover:bg-warning group-hover:text-warning-content transition-all duration-300">
            <LayoutGrid size={20} />
          </div>
          <div class="text-2xl font-black tabular-nums">{teacherStats?.activeAssignments ?? 0}</div>
        </div>
      </div>

      <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-info/30 transition-all">
        <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_progress')}</div>
        <div class="flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-info/10 text-info flex items-center justify-center group-hover:bg-info group-hover:text-info-content transition-all duration-300">
             <Sparkles size={20} />
          </div>
          <div class="text-[10px] font-bold opacity-60 leading-tight">
            {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_assignments_not_finished_by_all_students', {n: teacherStats?.unfinishedCount ?? 0})}
          </div>
        </div>
      </div>
    </section>

    <!-- Classes List -->
    <div class="space-y-6">
      <div class="flex items-center justify-between px-2">
        <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{translate('frontend/src/lib/Sidebar.svelte::classes_title')}</h2>
        
        {#if classes.length > 0}
          <button 
            on:click={showInput} 
            class="btn btn-ghost btn-xs gap-2 opacity-50 hover:opacity-100 transition-all font-black text-[10px] uppercase tracking-widest h-8"
          >
            <Plus size={14} />
            {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_create_button')}
          </button>
        {/if}
      </div>

      <div class="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
        {#each classes as c}
          <a href={`/classes/${c.id}`} class="group block no-underline text-current">
            <div class="bg-base-100 p-6 rounded-[2.5rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 transition-all h-full flex flex-col relative overflow-hidden">
               <div class="absolute top-0 right-0 w-1/4 h-1/4 bg-primary/5 rounded-bl-full pointer-events-none group-hover:scale-150 transition-transform duration-700"></div>

               <div class="flex items-center justify-between mb-5 relative">
                 <h3 class="font-black text-xl tracking-tight truncate group-hover:text-primary transition-colors pr-4">{c.name}</h3>
                 <div class="w-10 h-10 rounded-2xl bg-base-200 flex items-center justify-center group-hover:bg-primary/10 group-hover:text-primary transition-colors shrink-0">
                    <Users size={18} />
                 </div>
               </div>

               <div class="space-y-5 flex-1">
                 <div class="flex items-center justify-between">
                    <div class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate('frontend/src/lib/AdminPanel.svelte::students_stat_title')}</div>
                    <div class="text-sm font-black tabular-nums">{c.students_count}</div>
                 </div>

                 <div class="space-y-2">
                    <div class="flex items-center justify-between text-[10px] font-black uppercase tracking-widest opacity-40">
                      <span>{translate('frontend/src/routes/dashboard/+page.svelte::dashboard_progress')}</span>
                      <span class="tabular-nums">{c.average_progress}%</span>
                    </div>
                    <div class="w-full h-2 rounded-full bg-base-200 overflow-hidden">
                       <div class="h-full bg-primary" style={`width: ${c.average_progress}%`}></div>
                    </div>
                 </div>

                 <div class="pt-2 space-y-2">
                    {#each (c.assignments ?? []).slice(0, 3) as a}
                      <div class="flex items-center justify-between text-xs group/item">
                        <span class="truncate opacity-70 group-hover/item:opacity-100 transition-opacity pr-2">{a.title}</span>
                        <div class="flex items-center gap-2 shrink-0">
                           <div class="w-12 h-1 bg-base-200 rounded-full overflow-hidden">
                              <div class="h-full bg-primary/60" style={`width: ${(c.progress.find((x:any)=>x.id===a.id)?.done || 0) / (c.students_count || 1) * 100}%`}></div>
                           </div>
                           <span class="text-[10px] font-black opacity-40 tabular-nums">{(c.progress.find((x:any)=>x.id===a.id)?.done || 0)}/{c.students_count}</span>
                        </div>
                      </div>
                    {/each}
                 </div>
               </div>

               <div class="mt-6 pt-5 border-t border-base-300/30 flex items-center justify-between">
                 <div class="flex gap-2">
                   <div class="badge badge-sm bg-base-200 border-none font-black text-[9px] uppercase tracking-wider h-6">{c.assignments_count} {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_assignments_label')}</div>
                 </div>
                 <div class="text-primary opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all">
                    <ArrowRight size={16} />
                 </div>
               </div>
            </div>
          </a>
        {/each}

        {#if classes.length === 0}
          <!-- Create New Class Card -->
          <div class="bg-base-200/30 border-2 border-dashed border-base-300 rounded-[2.5rem] p-8 flex flex-col items-center justify-center gap-4 hover:border-primary/30 hover:bg-primary/5 transition-all group min-h-[200px]">
              <div class="w-12 h-12 rounded-2xl bg-base-100 flex items-center justify-center shadow-sm group-hover:scale-110 transition-transform">
                 <BookOpen size={24} class="opacity-30 group-hover:opacity-100 group-hover:text-primary transition-all" />
              </div>
              <div class="w-full max-w-[200px] flex flex-col gap-2">
                <input
                  type="text"
                  class="input input-sm bg-base-100 w-full rounded-xl focus:outline-none focus:ring-1 focus:ring-primary h-10 font-bold text-center"
                  placeholder={translate('frontend/src/routes/dashboard/+page.svelte::dashboard_new_class_placeholder')}
                  bind:value={newClassName}
                  on:keydown={(e) => e.key === 'Enter' && createClass()}
                  bind:this={newClassInput}
                />
                <button 
                  class="btn btn-primary btn-sm h-10 w-full rounded-xl font-black uppercase tracking-widest text-[10px]"
                  on:click={() => createClass()}
                  disabled={!newClassName.trim()}
                >
                  {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_create_button')}
                </button>
              </div>
              <p class="text-[10px] font-black uppercase tracking-widest opacity-30 group-hover:opacity-100 group-hover:text-primary transition-all">
                 {translate('frontend/src/routes/dashboard/+page.svelte::dashboard_create_new_class_button')}
              </p>
          </div>
        {/if}
      </div>
    </div>
  {:else if role === 'admin'}
    <AdminPanel />
  {/if}

  {#if err}
    <p class="text-error mt-4">{err}</p>
  {/if}

  <PromptModal bind:this={promptModal} />
{/if}

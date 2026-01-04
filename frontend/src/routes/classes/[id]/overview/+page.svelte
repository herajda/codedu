
<script lang="ts">
import { onMount } from 'svelte';
import { apiJSON } from '$lib/api';
import { page } from '$app/stores';
import { formatDateTime } from "$lib/date";
import { Trophy, CalendarClock, ListChecks, Target, PlayCircle, FolderOpen, MessageSquare, MessageCircle, ChevronRight } from 'lucide-svelte';
import { t, translator } from '$lib/i18n';
import DOMPurify from 'dompurify';
import { marked } from 'marked';

let translate;
$: translate = $translator;

let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(); }

let cls:any = null;
let submissions:any[] = [];
let loading = true;
let err = '';
let safeClassDescription = '';

function sanitizeMarkdown(input: string): string {
  if (!input) return '';
  try {
    return DOMPurify.sanitize((marked.parse(input) as string) || '');
  } catch {
    return '';
  }
}

function percent(done:number,total:number){
  return total ? Math.round((done/total)*100) : 0;
}

async function load(){
  loading = true; err = '';
  try {
    const [classData, submissionData] = await Promise.all([
      apiJSON(`/api/classes/${id}`),
      apiJSON('/api/my-submissions')
    ]);
    const detail = classData ?? null;
    const baseClass = detail?.class ?? detail ?? null;
    const assignments:any[] = Array.isArray(detail?.assignments)
      ? detail.assignments
      : Array.isArray(baseClass?.assignments)
        ? baseClass.assignments
        : [];
    const teacher = detail?.teacher ?? baseClass?.teacher ?? null;
    const students = Array.isArray(detail?.students) ? detail.students : baseClass?.students ?? [];
    cls = baseClass
      ? { ...baseClass, teacher, students, assignments }
      : null;
    submissions = Array.isArray(submissionData) ? submissionData : [];
  } catch(e:any){ err = e.message; cls = null; submissions = []; }
  loading = false;
}

onMount(load);

// Derived helpers for UI
$: assignments = Array.isArray(cls?.assignments) ? cls.assignments : [];
$: normalizedAssignments = assignments.map((a: any) => ({
  ...a,
  max_points: Number(a?.max_points ?? 0)
}));
$: assignmentProgress = normalizedAssignments.map((a: any) => {
  const best = (submissions ?? [])
    .filter((s: any) => s.assignment_id === a.id)
    .reduce((m: number, s: any) => {
      const p = Number(s.override_points ?? s.points ?? 0);
      return p > m ? p : m;
    }, 0);
  return { ...a, best };
});
$: totalAssignments = assignmentProgress.length;
$: completedAssignments = assignmentProgress.filter((p: any) => p.best >= p.max_points).length;
$: pointsTotal = assignmentProgress.reduce((sum: number, a: any) => sum + a.max_points, 0);
$: pointsEarned = assignmentProgress.reduce((sum: number, a: any) => sum + a.best, 0);
$: progressPercent = percent(completedAssignments, totalAssignments);
$: upcomingAssignments = assignmentProgress
  .filter((a: any) => new Date(a.deadline) > new Date())
  .slice()
  .sort((a: any, b: any) => new Date(a.deadline).getTime() - new Date(b.deadline).getTime());
$: upcomingCount = upcomingAssignments.length;
$: nextAssignment = (() => {
  if (!assignmentProgress.length) return null;
  const incomplete = assignmentProgress
    .filter((a: any) => a.best < a.max_points)
    .slice()
    .sort((a: any, b: any) => new Date(a.deadline).getTime() - new Date(b.deadline).getTime());
  return incomplete[0] ?? null;
})();
$: classAssignmentIds = new Set(assignmentProgress.map((a: any) => a.id));
$: recentSubmissions = (submissions ?? [])
  .filter((s: any) => classAssignmentIds.has(s.assignment_id))
  .sort((a: any, b: any) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
  .slice(0, 5);
$: teacherName = cls?.teacher ? (cls.teacher.name ?? cls.teacher.email ?? '—') : '—';
$: teacherEmail = cls?.teacher?.email ?? '';
$: teacherAvatar = cls?.teacher?.avatar ?? null;
$: teacherInitial = teacherName && teacherName !== '—'
  ? teacherName.trim().charAt(0).toUpperCase()
  : '?';
$: teacherId = cls?.teacher?.id ?? null;
$: teacherMessageQuery = (() => {
  if (!cls?.teacher) return '';
  const params = new URLSearchParams();
  if (cls.teacher.name) params.set('name', cls.teacher.name);
  else if (cls.teacher.email) params.set('email', cls.teacher.email);
  const q = params.toString();
  return q ? `?${q}` : '';
})();
$: teacherMessageUrl = teacherId ? `/messages/${teacherId}${teacherMessageQuery}` : '';
$: safeClassDescription = sanitizeMarkdown(cls?.description ?? '');

function badgeFor(a: any) {
  const best = assignmentProgress.find((p: any) => p.id === a.id)?.best ?? 0;
  const complete = best >= a.max_points;
  const late = new Date(a.deadline) < new Date() && !complete;
  if (complete) return { text: t('frontend/src/routes/classes/[id]/overview/+page.svelte::badge_completed'), cls: 'badge-success' };
  if (late) return { text: t('frontend/src/routes/classes/[id]/overview/+page.svelte::badge_late'), cls: 'badge-error' };
  return { text: t('frontend/src/routes/classes/[id]/overview/+page.svelte::badge_upcoming'), cls: 'badge-info' };
}
</script>

{#if loading}
  <div class="flex justify-center mt-12">
    <span class="loading loading-dots loading-lg text-primary"></span>
  </div>
{:else if err}
  <div class="p-8 text-center">
    <p class="text-error font-black uppercase tracking-widest text-xs mb-2">Error</p>
    <p class="text-base-content/60">{err}</p>
  </div>
{:else}
  <!-- Premium Header -->
  <section class="relative overflow-hidden bg-base-100 rounded-3xl border border-base-200 shadow-xl shadow-base-300/30 mb-8 p-6 sm:p-10">
    <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
    <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none"></div>
    <div class="relative flex flex-col md:flex-row items-center gap-6">
      <div class="flex-1 text-center md:text-left">
        <h1 class="text-3xl sm:text-4xl font-black tracking-tight mb-2">
          {cls.name} <span class="text-primary/40">/</span> {t('frontend/src/routes/classes/[id]/overview/+page.svelte::overview_suffix')}
        </h1>
        <p class="text-base-content/60 font-medium max-w-xl mx-auto md:mx-0">
          {t('frontend/src/routes/classes/[id]/overview/+page.svelte::overview_description')}
        </p>
      </div>
      <div class="flex items-center gap-3">
          <a href={`/classes/${id}/files`} class="btn btn-ghost bg-base-200/50 hover:bg-base-200 rounded-2xl gap-2 font-black uppercase tracking-widest text-[10px]">
            <FolderOpen class="w-4 h-4" aria-hidden="true" /> {t('frontend/src/routes/classes/[id]/overview/+page.svelte::files_button')}
          </a>
          <a href={`/classes/${id}/forum`} class="btn btn-ghost bg-base-200/50 hover:bg-base-200 rounded-2xl gap-2 font-black uppercase tracking-widest text-[10px]">
            <MessageSquare class="w-4 h-4" aria-hidden="true" /> {t('frontend/src/routes/classes/[id]/overview/+page.svelte::forum_button')}
          </a>
      </div>
    </div>
  </section>

  <!-- Stats Section -->
  <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-8">
    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-primary/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::progress_card_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center group-hover:bg-primary group-hover:text-primary-content transition-all duration-300">
          <Trophy size={20} />
        </div>
        <div>
          <div class="text-2xl font-black tabular-nums">{progressPercent}%</div>
          <div class="text-[10px] font-black opacity-40 uppercase tracking-tight">{translate('frontend/src/routes/classes/[id]/overview/+page.svelte::x_of_y_assignments', {completed: completedAssignments, total: totalAssignments})}</div>
        </div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-success/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::points_card_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-success/10 text-success flex items-center justify-center group-hover:bg-success group-hover:text-success-content transition-all duration-300">
          <Target size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{pointsEarned} <span class="text-sm opacity-40 font-normal">/ {pointsTotal}</span></div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-warning/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::total_assignments_card_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-warning/10 text-warning flex items-center justify-center group-hover:bg-warning group-hover:text-warning-content transition-all duration-300">
          <ListChecks size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{totalAssignments}</div>
      </div>
    </div>

    <div class="bg-base-100 p-5 rounded-3xl border border-base-200 shadow-sm group hover:border-info/30 transition-all">
      <div class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-3">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::upcoming_card_title')}</div>
      <div class="flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-info/10 text-info flex items-center justify-center group-hover:bg-info group-hover:text-info-content transition-all duration-300">
          <CalendarClock size={20} />
        </div>
        <div class="text-2xl font-black tabular-nums">{upcomingCount}</div>
      </div>
    </div>
  </section>

  {#if safeClassDescription}
    <div class="bg-base-100 p-8 rounded-[2rem] border border-base-200 shadow-sm mb-8 relative overflow-hidden group">
      <div class="absolute top-0 left-0 w-2 h-full bg-primary/20 group-hover:bg-primary transition-colors"></div>
      <div class="prose max-w-none assignment-description text-base-content/90">
        {@html safeClassDescription}
      </div>
    </div>
  {/if}

  {#if teacherId}
    <div class="bg-base-100 flex flex-wrap items-center gap-6 p-6 rounded-[2.5rem] border border-base-200 shadow-sm mb-8 relative overflow-hidden group">
      <div class="absolute top-0 right-0 w-32 h-32 bg-primary/5 rounded-full -mr-16 -mt-16 blur-2xl group-hover:scale-150 transition-transform duration-700"></div>
      
      <div class="relative shrink-0">
        <div class="w-16 h-16 rounded-full overflow-hidden ring-4 ring-base-200 shadow-lg">
          {#if teacherAvatar}
            <img src={teacherAvatar} alt={`Avatar of ${teacherName}`} class="w-full h-full object-cover" loading="lazy" />
          {:else}
            <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-2xl font-black text-primary">
              {teacherInitial}
            </div>
          {/if}
        </div>
        <div class="absolute -bottom-1 -right-1 w-5 h-5 bg-success rounded-full border-4 border-base-100"></div>
      </div>

      <div class="min-w-0 flex-1 relative">
        <div class="text-[10px] font-black uppercase tracking-widest text-base-content/40 mb-1">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::teacher_label')}</div>
        <div class="text-xl font-black tracking-tight truncate">{teacherName}</div>
        {#if teacherEmail && teacherEmail !== teacherName}
          <div class="text-sm font-medium opacity-50 flex items-center gap-2 mt-0.5">
             <div class="w-1 h-1 rounded-full bg-base-content/30 italic"></div>
             {teacherEmail}
          </div>
        {/if}
      </div>

      {#if teacherMessageUrl}
        <a href={teacherMessageUrl} class="btn btn-primary rounded-2xl px-6 gap-2 shadow-lg shadow-primary/20 relative">
          <MessageCircle size={18} />
          <span class="font-black uppercase tracking-widest text-[10px]">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::message_teacher_button')}</span>
        </a>
      {/if}
    </div>
  {/if}

  <div class="grid gap-8 lg:grid-cols-12">
    <section class="lg:col-span-8 space-y-6">
      {#if nextAssignment}
        <div class="bg-primary/5 rounded-[2rem] border border-primary/10 p-6 flex flex-col sm:flex-row items-center justify-between gap-6 group hover:bg-primary/10 transition-colors">
          <div class="flex items-center gap-5 min-w-0">
            <div class="w-14 h-14 rounded-2xl bg-primary/20 text-primary flex items-center justify-center shrink-0">
               <PlayCircle size={28} />
            </div>
            <div class="min-w-0 text-center sm:text-left">
              <div class="text-[10px] font-black uppercase tracking-widest text-primary/60 mb-1">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::continue_where_you_left_off')}</div>
              <div class="text-xl font-black tracking-tight truncate group-hover:text-primary transition-colors">{nextAssignment.title}</div>
              <div class="text-xs font-bold opacity-40 mt-1">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::due_prefix')} {formatDateTime(nextAssignment.deadline)}</div>
            </div>
          </div>
          <a href={`/assignments/${nextAssignment.id}`} class="btn btn-primary rounded-2xl px-8 shadow-lg shadow-primary/20 font-black uppercase tracking-widest text-[10px]">
             {t('frontend/src/routes/classes/[id]/overview/+page.svelte::continue_button')}
          </a>
        </div>
      {/if}

      <div class="space-y-6">
        <div class="flex items-center justify-between px-2">
          <h2 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::your_assignments_title')}</h2>
        </div>
        
        <div class="grid gap-4">
          {#each assignmentProgress as a}
            <a href={`/assignments/${a.id}`} class="group block no-underline text-current">
              <div class="bg-base-100 p-5 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 transition-all flex flex-col sm:flex-row items-center gap-6">
                <div class="min-w-0 flex-1">
                  <div class="font-black text-lg tracking-tight truncate group-hover:text-primary transition-colors mb-1">{a.title}</div>
                  <div class="text-xs font-bold opacity-40 italic">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::due_prefix')} {formatDateTime(a.deadline)}</div>
                </div>

                <div class="flex items-center gap-6 shrink-0 w-full sm:w-auto">
                  <div class="flex-1 sm:w-32 space-y-2">
                     <div class="flex items-center justify-between text-[10px] font-black opacity-40 uppercase">
                        <span>{percent(a.best, a.max_points)}%</span>
                        <span>{a.best}/{a.max_points}</span>
                     </div>
                     <div class="w-full h-1.5 rounded-full bg-base-200 overflow-hidden">
                        <div class="h-full bg-primary transition-all duration-500" style={`width: ${percent(a.best, a.max_points)}%`}></div>
                     </div>
                  </div>
                  
                  {#key a.id}
                    {#if badgeFor(a)}
                      <span class={`badge badge-ghost border-none font-black text-[9px] uppercase tracking-widest px-3 h-6 ${badgeFor(a).cls.replace('badge-', 'text-')}`}>
                        {badgeFor(a).text}
                      </span>
                    {/if}
                  {/key}
                </div>
              </div>
            </a>
          {:else}
            <div class="bg-base-100 p-12 rounded-[2rem] border border-base-200 border-dashed text-center">
               <p class="text-sm font-bold opacity-30 uppercase tracking-[0.2em]">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::no_assignments_message')}</p>
            </div>
          {/each}
        </div>
      </div>
    </section>

    <aside class="lg:col-span-4 space-y-8">
      <div class="space-y-6">
        <div class="flex items-center justify-between px-2">
          <h3 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::upcoming_deadlines_title')}</h3>
        </div>
        
        <div class="bg-base-100 rounded-[2.5rem] border border-base-200 shadow-sm overflow-hidden p-3">
          <div class="space-y-1">
            {#each upcomingAssignments as a}
              <a href={`/assignments/${a.id}`} class="flex items-center gap-4 p-4 rounded-[1.5rem] hover:bg-base-200 transition-colors group">
                <div class="w-12 h-12 rounded-2xl bg-warning/10 text-warning flex flex-col items-center justify-center shrink-0">
                  <span class="text-[9px] font-black uppercase tracking-tighter">{new Date(a.deadline).toLocaleString('default', { month: 'short' })}</span>
                  <span class="text-lg font-black leading-none">{new Date(a.deadline).getDate()}</span>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="font-black text-sm truncate group-hover:text-primary transition-colors">{a.title}</div>
                  <div class={`text-[10px] font-black uppercase tracking-widest mt-0.5 ${badgeFor(a).cls.replace('badge-', 'text-')}`}>{badgeFor(a).text}</div>
                </div>
                <ChevronRight size={16} class="opacity-0 group-hover:opacity-30 group-hover:translate-x-1 transition-all" />
              </a>
            {:else}
               <div class="py-10 text-center space-y-3">
                 <div class="w-12 h-12 rounded-full bg-base-200 flex items-center justify-center mx-auto opacity-30">
                    <CalendarClock size={20} />
                 </div>
                 <p class="text-[10px] font-black opacity-30 uppercase tracking-widest">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::no_upcoming_deadlines_message')}</p>
               </div>
            {/each}
          </div>
        </div>
      </div>

      <div class="space-y-6">
        <div class="flex items-center justify-between px-2">
          <h3 class="text-sm font-black uppercase tracking-[0.2em] opacity-40">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::recent_submissions_title')}</h3>
        </div>
        
        <div class="bg-base-100 rounded-[2.5rem] border border-base-200 shadow-sm overflow-hidden p-3">
          <div class="space-y-1">
            {#each recentSubmissions as s}
              <a href={`/submissions/${s.id}`} class="flex items-center gap-4 p-4 rounded-[1.5rem] hover:bg-base-200 transition-colors group">
                <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center shrink-0">
                   <Target size={18} />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="font-black text-sm truncate group-hover:text-primary transition-colors">{cls.assignments.find((a:any) => a.id === s.assignment_id)?.title}</div>
                  <div class="text-[10px] font-black opacity-40 uppercase tracking-widest mt-0.5">{formatDateTime(s.created_at)}</div>
                </div>
              </a>
            {:else}
              <div class="py-10 text-center space-y-3">
                <div class="w-10 h-10 rounded-full bg-base-200 flex items-center justify-center mx-auto opacity-30">
                   <Target size={18} />
                </div>
                <p class="text-[10px] font-black opacity-30 uppercase tracking-widest">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::no_submissions_yet_message')}</p>
              </div>
            {/each}
          </div>
        </div>
      </div>
    </aside>
  </div>
{/if}

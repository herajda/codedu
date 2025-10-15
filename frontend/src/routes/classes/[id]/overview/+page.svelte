
<script lang="ts">
import { onMount } from 'svelte';
import { apiJSON } from '$lib/api';
import { page } from '$app/stores';
import { formatDateTime } from "$lib/date";
import { Trophy, CalendarClock, ListChecks, Target, PlayCircle, FolderOpen, MessageSquare, MessageCircle } from 'lucide-svelte';
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
  <p>{t('frontend/src/routes/classes/[id]/overview/+page.svelte::loading')}</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <div class="flex flex-wrap items-start justify-between gap-3 mb-4">
    <div class="space-y-1">
      <h1 class="text-2xl font-semibold">{cls.name} · {t('frontend/src/routes/classes/[id]/overview/+page.svelte::overview_suffix')}</h1>
      <p class="text-sm text-base-content/60">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::overview_description')}</p>
    </div>
    <div class="hidden sm:flex gap-2">
        <a href={`/classes/${id}/files`} class="btn btn-outline"><FolderOpen class="w-4 h-4" aria-hidden="true" /> {t('frontend/src/routes/classes/[id]/overview/+page.svelte::files_button')}</a>
        <a href={`/classes/${id}/forum`} class="btn btn-outline"><MessageSquare class="w-4 h-4" aria-hidden="true" /> {t('frontend/src/routes/classes/[id]/overview/+page.svelte::forum_button')}</a>
    </div>
  </div>

  <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-6">
    <div class="card-elevated p-4 flex items-center gap-3">
      <Trophy class="w-5 h-5 opacity-70" aria-hidden="true" />
      <div>
        <div class="text-xs uppercase opacity-70">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::progress_card_title')}</div>
        <div class="text-xl font-semibold">{progressPercent}%</div>
        <div class="text-xs opacity-70">{translate('frontend/src/routes/classes/[id]/overview/+page.svelte::x_of_y_assignments', {completed: completedAssignments, total: totalAssignments})}</div>
      </div>
    </div>
    <div class="card-elevated p-4 flex items-center gap-3">
      <Target class="w-5 h-5 opacity-70" aria-hidden="true" />
      <div>
        <div class="text-xs uppercase opacity-70">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::points_card_title')}</div>
        <div class="text-xl font-semibold">{pointsEarned}/{pointsTotal}</div>
      </div>
    </div>
    <div class="card-elevated p-4 flex items-center gap-3">
      <ListChecks class="w-5 h-5 opacity-70" aria-hidden="true" />
      <div>
        <div class="text-xs uppercase opacity-70">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::total_assignments_card_title')}</div>
        <div class="text-xl font-semibold">{totalAssignments}</div>
      </div>
    </div>
    <div class="card-elevated p-4 flex items-center gap-3">
      <CalendarClock class="w-5 h-5 opacity-70" aria-hidden="true" />
      <div>
        <div class="text-xs uppercase opacity-70">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::upcoming_card_title')}</div>
        <div class="text-xl font-semibold">{upcomingCount}</div>
      </div>
    </div>
  </section>

  {#if safeClassDescription}
    <div class="card-elevated px-5 py-5 mb-6">
      <div class="prose max-w-none assignment-description text-base-content/90">
        {@html safeClassDescription}
      </div>
    </div>
  {/if}

  {#if teacherId}
    <div class="card-elevated flex flex-wrap items-center gap-4 px-5 py-4 mb-6">
      <div class="avatar">
        <div class="w-14 h-14 rounded-full overflow-hidden ring-2 ring-base-300/60">
          {#if teacherAvatar}
            <img src={teacherAvatar} alt={`Avatar of ${teacherName}`} class="w-full h-full object-cover" loading="lazy" />
          {:else}
            <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-xl font-semibold text-primary">
              {teacherInitial}
            </div>
          {/if}
        </div>
      </div>
      <div class="min-w-0 flex-1">
        <div class="text-xs uppercase tracking-wide text-base-content/60">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::teacher_label')}</div>
        <div class="font-semibold leading-tight truncate">{teacherName}</div>
        {#if teacherEmail && teacherEmail !== teacherName}
          <a class="text-sm text-primary truncate hover:underline" href={`mailto:${teacherEmail}`}>{teacherEmail}</a>
        {/if}
      </div>
      {#if teacherMessageUrl}
        <a href={teacherMessageUrl} class="btn btn-primary gap-2">
          <MessageCircle class="w-4 h-4" aria-hidden="true" />
          {t('frontend/src/routes/classes/[id]/overview/+page.svelte::message_teacher_button')}
        </a>
      {/if}
    </div>
  {/if}

  <div class="grid gap-6 lg:grid-cols-3">
    <section class="lg:col-span-2 space-y-6">
      {#if nextAssignment}
        <div class="card-elevated p-5 flex items-center justify-between gap-4">
          <div class="min-w-0">
            <div class="text-sm opacity-70">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::continue_where_you_left_off')}</div>
            <div class="text-lg font-semibold truncate">{nextAssignment.title}</div>
            <div class="text-sm opacity-70">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::due_prefix')} {formatDateTime(nextAssignment.deadline)}</div>
          </div>
          <a href={`/assignments/${nextAssignment.id}`} class="btn"><PlayCircle class="w-4 h-4" aria-hidden="true" /> {t('frontend/src/routes/classes/[id]/overview/+page.svelte::continue_button')}</a>
        </div>
      {/if}

      <div class="card-elevated p-5">
        <div class="flex items-center justify-between mb-3">
          <h2 class="font-semibold">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::your_assignments_title')}</h2>
        </div>
        <ul class="space-y-3">
          {#each assignmentProgress as a}
            <li>
              <a href={`/assignments/${a.id}`} class="block no-underline text-current">
                <div class="card-elevated p-4 hover:shadow-md transition">
                  <div class="flex items-center justify-between gap-4">
                    <div class="min-w-0">
                      <div class="font-medium truncate">{a.title}</div>
                      <div class="text-sm opacity-70 truncate">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::due_prefix')} {formatDateTime(a.deadline)}</div>
                    </div>
                  <div class="flex items-center gap-3 shrink-0">
                      <div class="flex items-center gap-2">
                      <progress class="progress progress-primary w-20 sm:w-24" value={a.best} max={a.max_points}></progress>
                        <span class="text-sm whitespace-nowrap">{a.best}/{a.max_points}</span>
                      </div>
                      {#key a.id}
                        {#if badgeFor(a)}<span class={`badge ${badgeFor(a).cls}`}>{badgeFor(a).text}</span>{/if}
                      {/key}
                    </div>
                  </div>
                </div>
              </a>
            </li>
          {/each}
          {#if !assignmentProgress.length}
            <li class="text-sm opacity-70">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::no_assignments_message')}</li>
          {/if}
        </ul>
      </div>
    </section>

    <aside class="space-y-6">
      <div class="card-elevated p-5">
        <div class="flex items-center justify-between mb-3">
          <h3 class="font-semibold">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::upcoming_deadlines_title')}</h3>
        </div>
        <ul class="divide-y divide-base-300/60">
          {#each upcomingAssignments as a}
            <li>
              <a href={`/assignments/${a.id}`} class="flex items-center justify-between py-3 hover:opacity-90">
                <div class="min-w-0">
                  <div class="font-medium truncate">{a.title}</div>
                  <div class="text-sm opacity-70 truncate">{formatDateTime(a.deadline)}</div>
                </div>
                <span class={`badge ${badgeFor(a).cls}`}>{badgeFor(a).text}</span>
              </a>
            </li>
          {/each}
          {#if !upcomingAssignments.length}
            <li class="py-3 text-sm opacity-70">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::no_upcoming_deadlines_message')}</li>
          {/if}
        </ul>
      </div>

      <div class="card-elevated p-5">
        <h3 class="font-semibold mb-3">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::recent_submissions_title')}</h3>
        <ul class="space-y-2">
          {#each recentSubmissions as s}
            <li>
              <a
                href={`/submissions/${s.id}`}
                class="flex items-center justify-between text-sm hover:opacity-90"
              >
                <span class="truncate"
                  >{cls.assignments.find((a:any) => a.id === s.assignment_id)?.title}</span
                >
                <span class="opacity-70 whitespace-nowrap"
                  >{formatDateTime(s.created_at)}</span
                >
              </a>
            </li>
          {/each}
          {#if !recentSubmissions.length}
            <li class="text-sm opacity-70">{t('frontend/src/routes/classes/[id]/overview/+page.svelte::no_submissions_yet_message')}</li>
          {/if}
        </ul>
      </div>
    </aside>
  </div>
{/if}

<script lang="ts">
import { onMount } from 'svelte';
import { apiJSON } from '$lib/api';
import { page } from '$app/stores';
import { formatDateTime } from "$lib/date";
import { Trophy, CalendarClock, ListChecks, Target, PlayCircle, FolderOpen, MessageSquare } from 'lucide-svelte';

let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(); }

let cls:any = null;
let submissions:any[] = [];
let loading = true;
let err = '';

function percent(done:number,total:number){
  return total ? Math.round((done/total)*100) : 0;
}

async function load(){
  loading = true; err = '';
  try {
    cls = await apiJSON(`/api/classes/${id}`);
    submissions = await apiJSON('/api/my-submissions');
    // Normalize potentially null/undefined payloads
    const assignments:any[] = Array.isArray(cls?.assignments) ? cls.assignments : [];
    submissions = Array.isArray(submissions) ? submissions : [];
    cls.assignments = assignments;
    cls.pointsTotal = assignments.reduce((s:any,a:any)=>s+a.max_points,0);
    cls.assignmentProgress = assignments.map((a:any)=>{
      const best = submissions
        .filter((s:any)=>s.assignment_id===a.id)
        .reduce((m:number,s:any)=>{
          const p = s.override_points ?? s.points ?? 0;
          return p>m ? p : m;
        },0);
      return { ...a, best };
    });
    cls.completed = cls.assignmentProgress.filter((p:any)=>p.best>=p.max_points).length;
    cls.pointsEarned = cls.assignmentProgress.reduce((tot:any,a:any)=>tot+a.best,0);
    const now = new Date();
    cls.upcoming = assignments
      .filter((a:any)=>new Date(a.deadline) > now)
      .sort((a:any,b:any)=>new Date(a.deadline).getTime()-new Date(b.deadline).getTime());
    cls = { ...cls };
  } catch(e:any){ err = e.message; }
  loading = false;
}

onMount(load);

// Derived helpers for UI
$: totalAssignments = cls?.assignments?.length ?? 0;
$: progressPercent = percent(cls?.completed ?? 0, totalAssignments);
$: upcomingCount = cls?.upcoming?.length ?? 0;
$: nextAssignment = (() => {
  if (!cls) return null;
  const incomplete = cls.assignments.filter((a: any) => (cls.assignmentProgress.find((p: any) => p.id === a.id)?.best ?? 0) < a.max_points);
  incomplete.sort((a: any, b: any) => new Date(a.deadline).getTime() - new Date(b.deadline).getTime());
  return incomplete[0] ?? null;
})();
$: classAssignmentIds = new Set((cls?.assignments ?? []).map((a: any) => a.id));
$: recentSubmissions = (submissions ?? [])
  .filter((s: any) => classAssignmentIds.has(s.assignment_id))
  .sort((a: any, b: any) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
  .slice(0, 5);

function badgeFor(a: any) {
  const best = cls.assignmentProgress.find((p: any) => p.id === a.id)?.best ?? 0;
  const complete = best >= a.max_points;
  const late = new Date(a.deadline) < new Date() && !complete;
  if (complete) return { text: 'Completed', cls: 'badge-success' };
  if (late) return { text: 'Late', cls: 'badge-error' };
  return { text: 'Upcoming', cls: 'badge-info' };
}
</script>

{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <div class="flex items-start justify-between gap-3 mb-4 flex-wrap">
    <div>
      <h1 class="text-2xl font-semibold">{cls.name} · Overview</h1>
      <p class="opacity-70 text-sm">Teacher: {cls.teacher?.name ?? cls.teacher?.email ?? '—'}</p>
    </div>
    <div class="hidden sm:flex gap-2">
      <a href={`/classes/${id}/files`} class="btn btn-outline"><FolderOpen class="w-4 h-4" aria-hidden="true" /> Files</a>
      <a href={`/classes/${id}/forum`} class="btn btn-outline"><MessageSquare class="w-4 h-4" aria-hidden="true" /> Forum</a>
      </div>
  </div>

  <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-6">
    <div class="card-elevated p-4 flex items-center gap-3">
      <Trophy class="w-5 h-5 opacity-70" aria-hidden="true" />
      <div>
        <div class="text-xs uppercase opacity-70">Progress</div>
        <div class="text-xl font-semibold">{progressPercent}%</div>
        <div class="text-xs opacity-70">{cls.completed}/{totalAssignments} assignments</div>
      </div>
    </div>
    <div class="card-elevated p-4 flex items-center gap-3">
      <Target class="w-5 h-5 opacity-70" aria-hidden="true" />
      <div>
        <div class="text-xs uppercase opacity-70">Points</div>
        <div class="text-xl font-semibold">{cls.pointsEarned}/{cls.pointsTotal}</div>
      </div>
    </div>
    <div class="card-elevated p-4 flex items-center gap-3">
      <ListChecks class="w-5 h-5 opacity-70" aria-hidden="true" />
      <div>
        <div class="text-xs uppercase opacity-70">Assignments</div>
        <div class="text-xl font-semibold">{totalAssignments}</div>
      </div>
    </div>
    <div class="card-elevated p-4 flex items-center gap-3">
      <CalendarClock class="w-5 h-5 opacity-70" aria-hidden="true" />
      <div>
        <div class="text-xs uppercase opacity-70">Upcoming</div>
        <div class="text-xl font-semibold">{upcomingCount}</div>
      </div>
    </div>
  </section>

  <div class="grid gap-6 lg:grid-cols-3">
    <section class="lg:col-span-2 space-y-6">
      {#if nextAssignment}
        <div class="card-elevated p-5 flex items-center justify-between gap-4">
          <div class="min-w-0">
            <div class="text-sm opacity-70">Continue where you left off</div>
            <div class="text-lg font-semibold truncate">{nextAssignment.title}</div>
            <div class="text-sm opacity-70">Due {formatDateTime(nextAssignment.deadline)}</div>
          </div>
          <a href={`/assignments/${nextAssignment.id}`} class="btn"><PlayCircle class="w-4 h-4" aria-hidden="true" /> Continue</a>
        </div>
      {/if}

      <div class="card-elevated p-5">
        <div class="flex items-center justify-between mb-3">
          <h2 class="font-semibold">Your assignments</h2>
        </div>
        <ul class="space-y-3">
          {#each cls.assignmentProgress as a}
            <li>
              <a href={`/assignments/${a.id}`} class="block no-underline text-current">
                <div class="card-elevated p-4 hover:shadow-md transition">
                  <div class="flex items-center justify-between gap-4">
                    <div class="min-w-0">
                      <div class="font-medium truncate">{a.title}</div>
                      <div class="text-sm opacity-70 truncate">Due {formatDateTime(a.deadline)}</div>
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
          {#if !cls.assignmentProgress.length}
            <li class="text-sm opacity-70">No assignments</li>
          {/if}
        </ul>
      </div>
    </section>

    <aside class="space-y-6">
      <div class="card-elevated p-5">
        <div class="flex items-center justify-between mb-3">
          <h3 class="font-semibold">Upcoming deadlines</h3>
        </div>
        <ul class="divide-y divide-base-300/60">
          {#each cls.upcoming as a}
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
          {#if !cls.upcoming.length}
            <li class="py-3 text-sm opacity-70">No upcoming deadlines</li>
          {/if}
        </ul>
      </div>

      <div class="card-elevated p-5">
        <h3 class="font-semibold mb-3">Recent submissions</h3>
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
            <li class="text-sm opacity-70">No submissions yet</li>
          {/if}
        </ul>
      </div>
    </aside>
  </div>
{/if}

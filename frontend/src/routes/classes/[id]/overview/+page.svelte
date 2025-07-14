<script lang="ts">
import { onMount } from 'svelte';
import { apiJSON } from '$lib/api';
import { page } from '$app/stores';

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
    cls.pointsTotal = cls.assignments.reduce((s:any,a:any)=>s+a.max_points,0);
    cls.assignmentProgress = cls.assignments.map((a:any)=>{
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
    cls.upcoming = cls.assignments
      .filter((a:any)=>new Date(a.deadline) > now)
      .sort((a:any,b:any)=>new Date(a.deadline).getTime()-new Date(b.deadline).getTime());
  } catch(e:any){ err = e.message; }
  loading = false;
}

onMount(load);
</script>

<h1 class="text-2xl font-bold mb-4">Overview</h1>
{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <p class="mb-4"><strong>Teacher:</strong> {cls.teacher.name ?? cls.teacher.email}</p>
  <div class="stats stats-vertical sm:stats-horizontal mb-6">
    <div class="stat">
      <div class="stat-title">Progress</div>
      <div class="stat-value">{percent(cls.completed, cls.assignments.length)}%</div>
      <div class="stat-desc">{cls.completed}/{cls.assignments.length} assignments</div>
    </div>
    <div class="stat">
      <div class="stat-title">Points</div>
      <div class="stat-value">{cls.pointsEarned}/{cls.pointsTotal}</div>
    </div>
  </div>
  <ul class="space-y-3">
    {#each cls.assignmentProgress as a}
      <li class="flex items-center gap-2">
        <span class="flex-1">{a.title}</span>
        <progress class="progress progress-primary flex-1" value={a.best} max={a.max_points}></progress>
        <span class="w-20 text-right">{a.best}/{a.max_points}</span>
      </li>
    {/each}
    {#if !cls.assignmentProgress.length}
      <li><i>No assignments</i></li>
    {/if}
  </ul>
  <h2 class="text-xl font-bold mt-8 mb-4">Upcoming deadlines</h2>
  <ul class="space-y-2">
    {#each cls.upcoming as a}
      <li class="flex justify-between items-center">
        <a href={`/assignments/${a.id}`} class="link">{a.title}</a>
        <span class="badge badge-info">{new Date(a.deadline).toLocaleString()}</span>
      </li>
    {/each}
    {#if !cls.upcoming.length}
      <li><i>No upcoming deadlines</i></li>
    {/if}
  </ul>
{/if}

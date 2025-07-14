<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { page } from '$app/stores';

  let id = $page.params.id;
  $: if ($page.params.id !== id) { id = $page.params.id; load(); }

  let cls:any = null;
  let submissions:any[] = [];
  let upcoming:any[] = [];
  let loading = true;
  let err='';

  function percent(done:number,total:number){
    return total ? Math.round((done/total)*100) : 0;
  }

  async function load(){
    loading = true;
    err='';
    try {
      cls = await apiJSON(`/api/classes/${id}`);
      submissions = await apiJSON('/api/my-submissions');
      cls.assignments = cls.assignments ?? [];
      cls.pointsTotal = cls.assignments.reduce((s:any,a:any)=>s+a.max_points,0);
      cls.completed = 0;
      cls.pointsEarned = 0;
      cls.assignmentProgress = cls.assignments.map((a:any)=>{
        const done = submissions.find((s:any)=>s.assignment_id===a.id && s.status==='completed');
        if(done){
          cls.completed++; cls.pointsEarned += a.max_points;
        }
        return {title:a.title, max:a.max_points, points: done ? a.max_points : 0, id:a.id, deadline:a.deadline};
      });
      const now = new Date();
      const soon = new Date();
      soon.setDate(soon.getDate()+7);
      upcoming = cls.assignments.filter((a:any)=>new Date(a.deadline)>now && new Date(a.deadline)<=soon)
        .sort((a:any,b:any)=>new Date(a.deadline).getTime()-new Date(b.deadline).getTime());
    } catch(e:any){ err=e.message }
    loading = false;
  }

  onMount(load);
</script>

{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <h1 class="text-2xl font-bold mb-4">{cls.name} Overview</h1>
  <p class="mb-4"><strong>Teacher:</strong> {cls.teacher.name ?? cls.teacher.email}</p>
  <div class="stats stats-vertical lg:stats-horizontal mb-6">
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

  <ul class="space-y-2">
    {#each cls.assignmentProgress as a}
      <li class="flex items-center gap-2">
        <span class="flex-1">{a.title}</span>
        <progress class="progress progress-primary flex-1" value={a.points} max={a.max}></progress>
        <span class="text-sm">{a.points}/{a.max}</span>
      </li>
    {/each}
    {#if !cls.assignmentProgress.length}
      <li><i>No assignments</i></li>
    {/if}
  </ul>

  <h2 class="text-xl font-bold mt-8 mb-4">Upcoming deadlines</h2>
  <ul class="space-y-2">
    {#each upcoming as a}
      <li>
        <a href={`/assignments/${a.id}`} class="flex justify-between items-center p-3 bg-base-100 rounded shadow hover:bg-base-200">
          <span>{a.title}</span>
          <span class="badge badge-info">{new Date(a.deadline).toLocaleString()}</span>
        </a>
      </li>
    {/each}
    {#if !upcoming.length}
      <li><i>No upcoming deadlines</i></li>
    {/if}
  </ul>
{/if}

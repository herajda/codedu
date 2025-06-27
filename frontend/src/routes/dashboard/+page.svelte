<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';

  let role = '';
  let classes:any[] = [];
  let submissions:any[] = [];
  let upcoming:any[] = [];
  let loading = true;
  let err = '';

  function percent(done:number,total:number){
    return total ? Math.round((done/total)*100) : 0;
  }

  onMount(async () => {
    try {
      const me = await apiJSON('/api/me');
      role = me.role;
      classes = await apiJSON('/api/classes');

      if (role === 'student') {
        submissions = await apiJSON('/api/my-submissions');
      }

      for (const c of classes) {
        const detail = await apiJSON(`/api/classes/${c.id}`);
        c.assignments = detail.assignments ?? [];
        c.students = detail.students ?? [];
        c.pointsTotal = c.assignments.reduce((s:any,a:any)=>s+a.max_points,0);
        if (role === 'student') {
          c.completed = c.assignments.filter((a:any)=>
            submissions.find((s:any)=>s.assignment_id===a.id && s.status==='completed')
          ).length;
          c.pointsEarned = c.assignments.filter((a:any)=>
            submissions.find((s:any)=>s.assignment_id===a.id && s.status==='completed')
          ).reduce((s:any,a:any)=>s+a.max_points,0);
        } else if (role === 'teacher') {
          c.progress = [];
          for (const a of c.assignments) {
            const data = await apiJSON(`/api/assignments/${a.id}`);
            const subs = Array.isArray(data.submissions) ? data.submissions : [];
            const done = new Set(subs.filter((s:any)=>s.status==='completed').map((s:any)=>s.student_id)).size;
            c.progress.push({ id:a.id, title:a.title, done });
          }
        }
      }

      if (role === 'student') {
        const now = new Date();
        const soon = new Date();
        soon.setDate(soon.getDate()+7);
        upcoming = classes.flatMap(c => (c.assignments ?? []).map((a: any) => ({ class: c.name, ...a })))
          .filter(a=>new Date(a.deadline)>now && new Date(a.deadline)<=soon)
          .sort((a,b)=>new Date(a.deadline).getTime()-new Date(b.deadline).getTime());
      }
    } catch(e:any){
      err = e.message;
    }
    loading = false;
  });
</script>

{#if loading}
  <div class="flex justify-center mt-8"><span class="loading loading-dots loading-lg"></span></div>
{:else}
  <h1 class="text-2xl font-bold mb-6">Dashboard</h1>

  {#if role === 'student'}
    <div class="grid gap-6 md:grid-cols-2">
      {#each classes as c}
        <a href={`/classes/${c.id}`} class="card bg-base-100 shadow hover:shadow-lg block">
          <div class="card-body">
            <h2 class="card-title">{c.name}</h2>
            <div class="stats stats-vertical lg:stats-horizontal mt-3">
              <div class="stat">
                <div class="stat-title">Progress</div>
                <div class="stat-value">{percent(c.completed, c.assignments.length)}%</div>
                <div class="stat-desc">{c.completed}/{c.assignments.length} assignments</div>
              </div>
              <div class="stat">
                <div class="stat-title">Points</div>
                <div class="stat-value">{c.pointsEarned}/{c.pointsTotal}</div>
              </div>
            </div>
          </div>
        </a>
      {/each}
    </div>

    <h2 class="text-xl font-bold mt-8 mb-4">Upcoming deadlines</h2>
    <ul class="space-y-2">
      {#each upcoming as a}
        <li class="flex justify-between items-center p-3 bg-base-100 rounded shadow">
          <span>{a.title} <span class="text-sm text-base-content/60">({a.class})</span></span>
          <span class="badge badge-info">{new Date(a.deadline).toLocaleString()}</span>
        </li>
      {/each}
      {#if !upcoming.length}
        <li><i>No upcoming deadlines</i></li>
      {/if}
    </ul>
  {:else if role === 'teacher'}
    <div class="grid gap-6 md:grid-cols-2">
      {#each classes as c}
        <a href={`/classes/${c.id}`} class="card bg-base-100 shadow hover:shadow-lg block">
          <div class="card-body">
            <h2 class="card-title">{c.name}</h2>
            <p class="text-sm mb-2">{c.students.length} students</p>
            <ul class="space-y-2">
              {#each c.assignments as a}
                <li class="flex items-center justify-between">
                  <span>{a.title}</span>
                  <progress class="progress progress-primary w-24" value={c.progress.find((x:any)=>x.id===a.id)?.done || 0} max={c.students.length}></progress>
                  <span class="text-sm">{c.progress.find((x:any)=>x.id===a.id)?.done || 0}/{c.students.length}</span>
                </li>
              {/each}
              {#if !c.assignments.length}<li><i>No assignments</i></li>{/if}
            </ul>
          </div>
        </a>
      {/each}
    </div>
  {/if}

  {#if err}
    <p class="text-error mt-4">{err}</p>
  {/if}
{/if}

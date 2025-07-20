<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, apiFetch } from '$lib/api';
  import AdminPanel from '$lib/AdminPanel.svelte';

  let role = '';
  let classes:any[] = [];
  let submissions:any[] = [];
  let upcoming:any[] = [];
  let loading = true;
  let err = '';
  let newClassName = '';

  function percent(done:number,total:number){
    return total ? Math.round((done/total)*100) : 0;
  }

  onMount(async () => {
    try {
      const me = await apiJSON('/api/me');
      role = me.role;
      if (role === 'admin') {
        loading = false;
        return;
      }
      const result = await apiJSON('/api/classes');
      classes = Array.isArray(result) ? result : [];

      if (role === 'student') {
        submissions = await apiJSON('/api/my-submissions');
      }

      for (const c of classes) {
        const detail = await apiJSON(`/api/classes/${c.id}`);
        c.assignments = detail.assignments ?? [];
        c.students = detail.students ?? [];
        c.pointsTotal = c.assignments.reduce((s:any,a:any)=>s+a.max_points,0);
        if (role === 'student') {
          c.assignmentProgress = c.assignments.map((a:any)=>{
            const best = submissions
              .filter((s:any)=>s.assignment_id===a.id)
              .reduce((m:number,s:any)=>{
                const p = s.override_points ?? s.points ?? 0;
                return p>m ? p : m;
              },0);
            return { id:a.id, title:a.title, done: best >= a.max_points };
          });
          c.completed = c.assignmentProgress.filter((p:any)=>p.done).length;
          c.pointsEarned = c.assignments.reduce((tot:any,a:any)=>{
            const best = submissions
              .filter((s:any)=>s.assignment_id===a.id)
              .reduce((m:number,s:any)=>{
                const p = s.override_points ?? s.points ?? 0;
                return p>m ? p : m;
              },0);
            return tot + best;
          },0);
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

  async function createClass(){
    try{
      const cl = await apiJSON('/api/classes', {
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({name:newClassName})
      });
      classes = [...classes, { ...cl, assignments: [], students: [] }];
      newClassName='';
    }catch(e:any){ err = e.message; }
  }
</script>

{#if loading}
  <div class="flex justify-center mt-8"><span class="loading loading-dots loading-lg"></span></div>
{:else}

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
            <ul class="mt-4 space-y-2">
              {#each c.assignmentProgress as a}
                <li class="grid grid-cols-[10rem_1fr] items-center gap-2">
                  <span class="truncate w-40">{a.title}</span>
                  <progress class="progress progress-primary w-full" value={a.done ? 100 : 0} max="100"></progress>
                </li>
              {/each}
              {#if !c.assignmentProgress.length}
                <li><i>No assignments</i></li>
              {/if}
            </ul>
          </div>
        </a>
      {/each}
    </div>

    <h2 class="text-xl font-bold mt-8 mb-4">Upcoming deadlines</h2>
    <ul class="space-y-2">
      {#each upcoming as a}
        <li>
          <a
            href={`/assignments/${a.id}`}
            class="flex justify-between items-center p-3 bg-base-100 rounded shadow hover:bg-base-200"
          >
            <span>{a.title} <span class="text-sm text-base-content/60">({a.class})</span></span>
            <span class="badge badge-info">{new Date(a.deadline).toLocaleString()}</span>
          </a>
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
                <li class="grid grid-cols-[10rem_1fr_auto] items-center gap-2">
                  <span class="truncate w-40">{a.title}</span>
                  <progress class="progress progress-primary w-full" value={c.progress.find((x:any)=>x.id===a.id)?.done || 0} max={c.students.length}></progress>
                  <span class="text-sm">{c.progress.find((x:any)=>x.id===a.id)?.done || 0}/{c.students.length}</span>
                </li>
              {/each}
              {#if !c.assignments.length}<li><i>No assignments</i></li>{/if}
            </ul>
          </div>
        </a>
      {/each}
    </div>
    <form class="mt-6 flex gap-2 max-w-sm" on:submit|preventDefault={createClass}>
      <input class="input input-bordered flex-1" placeholder="New class" bind:value={newClassName} required />
      <button class="btn">Create</button>
    </form>
  {:else if role === 'admin'}
    <AdminPanel />
  {/if}

  {#if err}
    <p class="text-error mt-4">{err}</p>
  {/if}
{/if}

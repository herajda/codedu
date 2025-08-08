<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { apiJSON, apiFetch } from '$lib/api';
  import AdminPanel from '$lib/AdminPanel.svelte';
  import { formatDateTime } from "$lib/date";
  import { BookOpen, CalendarClock, Trophy, Inbox, Users, LayoutGrid, MessageSquare, FolderOpen } from 'lucide-svelte';

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
          c.assignments.sort((a:any,b:any)=>new Date(b.created_at).getTime()-new Date(a.created_at).getTime());
          c.notFinished = c.assignments.filter((a:any)=>{
            const done = c.progress.find((p:any)=>p.id===a.id)?.done ?? 0;
            return done < c.students.length;
          }).length;
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
      showNewClassInput = false;
    }catch(e:any){ err = e.message; }
  }

  async function showInput() {
    showNewClassInput = true;
    await tick();
    newClassInput?.focus();
  }

  // Derived small stats for nicer UI
  $: totalClasses = classes.length;
  $: studentStats = role === 'student' ? (() => {
    const totalAssignments = classes.reduce((sum: number, c: any) => sum + c.assignments.length, 0);
    const completedAssignments = classes.reduce((sum: number, c: any) => sum + (c.completed ?? 0), 0);
    const pointsEarned = classes.reduce((sum: number, c: any) => sum + (c.pointsEarned ?? 0), 0);
    const pointsTotal = classes.reduce((sum: number, c: any) => sum + (c.pointsTotal ?? 0), 0);
    return { totalAssignments, completedAssignments, pointsEarned, pointsTotal };
  })() : null;

  $: teacherStats = role === 'teacher' ? (() => {
    const studentsTotal = classes.reduce((sum: number, c: any) => sum + (c.students?.length ?? 0), 0);
    const activeAssignments = classes.reduce((sum: number, c: any) => sum + c.assignments.length, 0);
    const outstanding = classes.reduce((sum: number, c: any) => {
      return sum + c.assignments.reduce((s: number, a: any) => {
        const done = (c.progress?.find((p: any) => p.id === a.id)?.done) ?? 0;
        return s + Math.max(0, (c.students?.length ?? 0) - done);
      }, 0);
    }, 0);
    return { studentsTotal, activeAssignments, outstanding };
  })() : null;
</script>

{#if loading}
  <div class="flex justify-center mt-8"><span class="loading loading-dots loading-lg"></span></div>
{:else}

  {#if role === 'student'}
    <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-6">
      <div class="card-elevated p-4 flex items-center gap-3">
        <BookOpen class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">Classes</div>
          <div class="text-xl font-semibold">{totalClasses}</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <Trophy class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">Progress</div>
          <div class="text-xl font-semibold">{percent(studentStats?.completedAssignments ?? 0, studentStats?.totalAssignments ?? 0)}%</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <CalendarClock class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">Upcoming</div>
          <div class="text-xl font-semibold">{upcoming.length}</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <Inbox class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">Points</div>
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
                {c.assignments.length} assignments
              </span>
            </div>
            <div class="flex items-center gap-3 mb-3">
              <div class="flex-1 h-2 rounded-full bg-base-300/60 overflow-hidden">
                <div class="h-full bg-gradient-to-r from-cyan-400 via-sky-400 to-teal-400" style={`width: ${percent(c.completed, c.assignments.length)}%`}></div>
              </div>
              <span class="text-sm whitespace-nowrap">{c.completed}/{c.assignments.length}</span>
            </div>
            <ul class="space-y-2">
              {#each c.assignmentProgress.slice(0, 3) as a}
                <li class="flex items-center gap-2 text-sm">
                  <span class={`w-2 h-2 rounded-full ${a.done ? 'bg-teal-400' : 'bg-base-300'}`}></span>
                  <span class="truncate">{a.title}</span>
                </li>
              {/each}
              {#if c.assignmentProgress.length === 0}
                <li class="text-sm opacity-70">No assignments</li>
              {/if}
            </ul>
          </div>
        </a>
      {/each}
    </section>

    <section class="mt-8 grid gap-6 lg:grid-cols-2">
      <div class="card-elevated p-5">
        <div class="flex items-center justify-between mb-3">
          <h3 class="font-semibold">Upcoming deadlines</h3>
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
            <li class="py-3 text-sm opacity-70">No upcoming deadlines</li>
          {/if}
        </ul>
      </div>
      <div class="card-elevated p-5">
        <div class="flex items-center justify-between mb-3">
          <h3 class="font-semibold">Quick actions</h3>
        </div>
        <div class="flex flex-wrap gap-3">
          <a href="/my-classes" class="btn btn-primary btn-outline"><LayoutGrid class="w-4 h-4" aria-hidden="true" /> My classes</a>
          <a href="/messages" class="btn btn-ghost"><MessageSquare class="w-4 h-4" aria-hidden="true" /> Messages</a>
          <a href="/files/0" class="btn btn-ghost"><FolderOpen class="w-4 h-4" aria-hidden="true" /> Files</a>
        </div>
      </div>
    </section>
  {:else if role === 'teacher'}
    <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-6">
      <div class="card-elevated p-4 flex items-center gap-3">
        <BookOpen class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">Classes</div>
          <div class="text-xl font-semibold">{totalClasses}</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <Users class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">Students</div>
          <div class="text-xl font-semibold">{teacherStats?.studentsTotal ?? 0}</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <CalendarClock class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">Active assignments</div>
          <div class="text-xl font-semibold">{teacherStats?.activeAssignments ?? 0}</div>
        </div>
      </div>
      <div class="card-elevated p-4 flex items-center gap-3">
        <Inbox class="w-5 h-5 opacity-70" aria-hidden="true" />
        <div>
          <div class="text-xs uppercase opacity-70">Outstanding</div>
          <div class="text-xl font-semibold">{teacherStats?.outstanding ?? 0}</div>
        </div>
      </div>
    </section>

    <section class="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
      {#each classes as c}
        <a href={`/classes/${c.id}`} class="block no-underline text-current">
          <div class="card-elevated p-5 h-full">
            <div class="flex items-center justify-between mb-2">
              <h2 class="font-semibold text-lg truncate">{c.name}</h2>
              <span class="badge badge-ghost">{c.students.length} students</span>
            </div>
            <ul class="space-y-2">
              {#each c.assignments.slice(0,5) as a}
                <li class="flex items-center gap-3">
                  <span class="truncate flex-1">{a.title}</span>
                  <progress class="progress progress-primary flex-1" value={c.progress.find((x:any)=>x.id===a.id)?.done || 0} max={c.students.length}></progress>
                  <span class="text-sm whitespace-nowrap">{c.progress.find((x:any)=>x.id===a.id)?.done || 0}/{c.students.length}</span>
                </li>
              {/each}
              {#if !c.assignments.length}
                <li class="text-sm opacity-70">No assignments</li>
              {/if}
              {#if c.assignments.length > 5}
                <li class="text-xs opacity-70">{c.notFinished} assignments not finished by all students</li>
              {/if}
            </ul>
          </div>
        </a>
      {/each}
    </section>

    <section class="mt-6">
      {#if showNewClassInput}
        <form class="flex gap-2 max-w-sm" on:submit|preventDefault={createClass}>
          <input class="input input-bordered flex-1" placeholder="New class" bind:value={newClassName} bind:this={newClassInput} required />
          <button class="btn" type="submit">Create</button>
        </form>
      {:else}
        <button class="btn" on:click={showInput} type="button">Create New Class</button>
      {/if}
    </section>
  {:else if role === 'admin'}
    <AdminPanel />
  {/if}

  {#if err}
    <p class="text-error mt-4">{err}</p>
  {/if}
{/if}

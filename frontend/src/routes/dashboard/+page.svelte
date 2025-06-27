<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { auth } from '$lib/auth';
  import { apiJSON } from '$lib/api';

  type Class = { id:number; name:string };
  type Assignment = { id:number; title:string; deadline:string; max_points:number; class_id:number };
  type Submission = { assignment_id:number; status:string };

  let role = '';
  let stats: any[] = [];
  let upcoming: Assignment[] = [];
  let err = '';

  onMount(async () => {
    const user = get(auth);
    if (!user) return;
    role = user.role;
    try {
      const classes: Class[] = await apiJSON('/api/classes');
      if (role === 'student') {
        const [assignments, subs]: [Assignment[], Submission[]] = await Promise.all([
          apiJSON('/api/assignments'),
          apiJSON('/api/my-submissions')
        ]);
        stats = classes.map(c => {
          const list = assignments.filter(a => a.class_id === c.id);
          const total = list.reduce((s, a) => s + a.max_points, 0);
          const earned = list.reduce((s, a) => {
            const done = subs.find(sb => sb.assignment_id === a.id && sb.status === 'completed');
            return s + (done ? a.max_points : 0);
          }, 0);
          const upcomingList = list.filter(a => new Date(a.deadline) > new Date() && !subs.find(sb => sb.assignment_id === a.id && sb.status === 'completed'))
            .sort((a, b) => new Date(a.deadline).getTime() - new Date(b.deadline).getTime());
          return { ...c, total, earned, percent: total ? Math.round(earned / total * 100) : 0, upcoming: upcomingList };
        });
        upcoming = assignments.filter(a => new Date(a.deadline) > new Date() && !subs.find(sb => sb.assignment_id === a.id && sb.status === 'completed'))
          .sort((a, b) => new Date(a.deadline).getTime() - new Date(b.deadline).getTime())
          .slice(0, 5);
      } else if (role === 'teacher') {
        stats = [];
        for (const c of classes) {
          const detail = await apiJSON(`/api/classes/${c.id}`);
          const students = detail.students ?? [];
          const assignments: Assignment[] = detail.assignments ?? [];
          const item: any = { id: c.id, name: c.name, students: students.length, assignments: [] };
          for (const a of assignments) {
            const info = await apiJSON(`/api/assignments/${a.id}`);
            const subs = info.submissions ?? [];
            const done = subs.filter((s: any) => s.status === 'completed').length;
            item.assignments.push({ title: a.title, done, total: students.length });
          }
          stats.push(item);
        }
      }
    } catch (e: any) {
      err = e.message;
    }
  });
</script>

{#if role === 'student'}
  <h1 class="text-2xl font-bold mb-4">Dashboard</h1>
  <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
    {#each stats as s}
      <div class="card bg-base-100 shadow">
        <div class="card-body">
          <h2 class="card-title">{s.name}</h2>
          <p class="text-sm">{s.earned} / {s.total} pts</p>
          <progress class="progress progress-primary w-full" value={s.percent} max="100"></progress>
          {#if s.upcoming.length}
            <p class="text-sm mt-2">Next: {s.upcoming[0].title} ({new Date(s.upcoming[0].deadline).toLocaleDateString()})</p>
          {/if}
          <div class="card-actions justify-end">
            <a class="btn btn-sm btn-primary" href={`/classes/${s.id}`}>Open</a>
          </div>
        </div>
      </div>
    {/each}
  </div>
  {#if upcoming.length}
    <h2 class="text-xl font-bold mt-8 mb-4">Upcoming</h2>
    <ul class="space-y-2">
      {#each upcoming as a}
        <li class="flex justify-between items-center">
          <span>{a.title}</span>
          <span class="text-sm">{new Date(a.deadline).toLocaleDateString()}</span>
        </li>
      {/each}
    </ul>
  {/if}
{/if}

{#if role === 'teacher'}
  <h1 class="text-2xl font-bold mb-4">Dashboard</h1>
  <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
    {#each stats as c}
      <div class="card bg-base-100 shadow">
        <div class="card-body">
          <h2 class="card-title">{c.name}</h2>
          <p class="text-sm mb-2">{c.students} students</p>
          <ul class="space-y-1 text-sm">
            {#each c.assignments as a}
              <li class="flex justify-between"><span>{a.title}</span><span class="badge badge-sm">{a.done}/{a.total}</span></li>
            {/each}
            {#if !c.assignments.length}<li><i>No assignments</i></li>{/if}
          </ul>
          <div class="card-actions justify-end mt-2">
            <a class="btn btn-sm btn-primary" href={`/classes/${c.id}`}>Open</a>
          </div>
        </div>
      </div>
    {/each}
  </div>
{/if}

{#if err}
  <p class="text-error mt-4">{err}</p>
{/if}

<script lang="ts">
import { onMount } from 'svelte';
import { apiJSON } from '$lib/api';
import { page } from '$app/stores';

let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(); }

let students:any[]=[];
let assignments:any[]=[];
let scores:any[]=[];
let loading=true;
let err='';

async function load(){
  loading=true; err='';
  try{
    const data = await apiJSON(`/api/classes/${id}/progress`);
    students = data.students ?? [];
    assignments = data.assignments ?? [];
    scores = data.scores ?? [];
  }catch(e:any){ err = e.message }
  loading=false;
}

onMount(load);

function score(sid:number, aid:number){
  const cell = scores.find((c:any)=>c.student_id===sid && c.assignment_id===aid);
  return cell?.points ?? 0;
}

function total(sid:number){
  return assignments.reduce((sum,a)=>sum + (score(sid,a.id) || 0),0);
}
</script>

<h1 class="text-2xl font-bold mb-4">Progress</h1>
{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <div class="overflow-x-auto">
    <table class="table table-zebra">
      <thead>
        <tr>
          <th>Student</th>
          {#each assignments as a}
            <th>{a.title}</th>
          {/each}
          <th>Total</th>
        </tr>
      </thead>
      <tbody>
        {#each students as s}
          <tr>
            <td>{s.name ?? s.email}</td>
            {#each assignments as a}
              <td>{score(s.id, a.id)}</td>
            {/each}
            <td>{total(s.id)}</td>
          </tr>
        {/each}
        {#if !students.length}
          <tr><td colspan={assignments.length + 2}><i>No students</i></td></tr>
        {/if}
      </tbody>
    </table>
  </div>
{/if}

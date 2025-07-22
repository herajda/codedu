<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiJSON } from '$lib/api';

  import { formatDateTime } from "$lib/date";
  interface Submission {
    id: number;
    assignment_id: number;
    status: string;
    created_at: string;
    results?: any[];
  }

  let subs: Submission[] = [];
  let titles: Record<number,string> = {};
  let loading = true;
  let err = '';
  let es: EventSource | null = null;

  function resultColor(s: string){
    if(s === 'passed') return 'badge-success';
    if(s === 'wrong_output') return 'badge-error';
    if(s === 'runtime_error') return 'badge-error';
    if(s === 'time_limit_exceeded' || s==='memory_limit_exceeded') return 'badge-warning';
    return '';
  }

  function statusColor(s: string){
    if(s === 'completed') return 'badge-success';
    if(s === 'running') return 'badge-info';
    if(s === 'failed') return 'badge-error';
    return '';
  }

  async function load(){
    err='';
    try{
      subs = await apiJSON('/api/my-submissions');
      const aids = Array.from(new Set(subs.map(s=>s.assignment_id)));
      for(const id of aids){
        const data = await apiJSON(`/api/assignments/${id}`);
        titles[id] = data.assignment.title;
      }
      for(const s of subs){
        const data = await apiJSON(`/api/submissions/${s.id}`);
        s.results = data.results;
      }
    }catch(e:any){ err = e.message }
    loading = false;
  }

  onMount(() => {
    load();
    es = new EventSource('/api/events');
    es.addEventListener('status', (ev) => {
      const d = JSON.parse((ev as MessageEvent).data);
      const s = subs.find((x) => x.id === d.submission_id);
      if (s) {
        s.status = d.status;
        if (d.status !== 'running') load();
      }
    });
    es.addEventListener('result', (ev) => {
      const d = JSON.parse((ev as MessageEvent).data);
      const s = subs.find((x) => x.id === d.submission_id);
      if (s) {
        s.results = [...(s.results ?? []), d];
      }
    });
  });
  onDestroy(() => { es?.close(); });
</script>

{#if loading}
  <div class="flex justify-center mt-8"><span class="loading loading-dots loading-lg"></span></div>
{:else}
  <h1 class="text-2xl font-bold mb-6">My Submissions</h1>
  <div class="overflow-x-auto">
    <table class="table table-zebra">
      <thead>
        <tr><th>Date</th><th>Assignment</th><th>Status</th><th>Tests</th></tr>
      </thead>
      <tbody>
        {#each subs as s}
          <tr>
            <td>{formatDateTime(s.created_at)}</td>
            <td>{titles[s.assignment_id] ?? s.assignment_id}</td>
            <td><span class={`badge ${statusColor(s.status)}`}>{s.status}</span></td>
            <td>
              <div class="flex flex-wrap gap-1">
                {#each s.results ?? [] as r, i}
                  <span class={`badge badge-sm ${resultColor(r.status)} tooltip`} data-tip={`${r.status} ${r.runtime_ms}ms exit ${r.exit_code}${r.stderr? '\n'+r.stderr:''}`}>{i+1}</span>
                {/each}
                {#if !(s.results && s.results.length)}<i>pending</i>{/if}
              </div>
            </td>
          </tr>
        {/each}
        {#if !subs.length}
          <tr><td colspan="4"><i>No submissions yet</i></td></tr>
        {/if}
      </tbody>
    </table>
  </div>
  {#if err}<p class="text-error mt-4">{err}</p>{/if}
{/if}

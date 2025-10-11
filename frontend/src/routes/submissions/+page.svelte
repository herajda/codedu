
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { createEventSource } from '$lib/sse';
  import { t } from '$lib/i18n';

  import { formatDateTime } from "$lib/date";
  interface Submission {
    id: number;
    assignment_id: number;
    status: string;
    created_at: string;
    manually_accepted?: boolean;
    results?: any[];
  }

  let subs: Submission[] = [];
  let titles: Record<string,string> = {};
  let loading = true;
  let err = '';
  let esCtrl: { close: () => void } | null = null;

  // UI state
  let query = '';
  let statusFilter: 'all' | 'running' | 'completed' | 'failed' = 'all';
  let sortBy: 'newest' | 'oldest' | 'status' = 'newest';
  let layoutMode: 'table' | 'cards' | 'board' = 'table';

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
      // Load assignment titles in parallel
      const titlePairs = await Promise.all(
        aids.map(async (id) => {
          const data = await apiJSON(`/api/assignments/${id}`);
          return [id, data.assignment.title] as const;
        })
      );
      for (const [id, title] of titlePairs) titles[id] = title;
      // Load results in parallel
      const resultsList = await Promise.all(
        subs.map(async (s) => {
          const data = await apiJSON(`/api/submissions/${s.id}`);
          return { id: s.id, results: data.results } as const;
        })
      );
      const idToResults = new Map(resultsList.map(r => [r.id, r.results]));
      subs = subs.map(s => ({ ...s, results: idToResults.get(s.id) ?? [] }));
    }catch(e:any){ err = e.message }
    loading = false;
  }

  onMount(() => {
    try {
      const saved = localStorage.getItem('subs_layout');
      if (saved === 'table' || saved === 'cards') layoutMode = saved;
    } catch {}
    load();
    esCtrl = createEventSource(
      '/api/events',
      (src) => {
    src.addEventListener('status', (ev) => {
      const d = JSON.parse((ev as MessageEvent).data);
      const s = subs.find((x) => x.id === d.submission_id);
      if (s) {
        s.status = d.status;
        if (d.status !== 'running') load();
      }
    });
    src.addEventListener('result', (ev) => {
      const d = JSON.parse((ev as MessageEvent).data);
      const s = subs.find((x) => x.id === d.submission_id);
      if (s) {
        s.results = [...(s.results ?? []), d];
      }
    });
      },
      {
        onError: (m) => { err = m; },
        onOpen: () => { err = ''; }
      }
    );
  });
  onDestroy(() => { esCtrl?.close(); });

  function matchesQuery(s: Submission) {
    const t = (titles[s.assignment_id] ?? String(s.assignment_id)).toLowerCase();
    return !query || t.includes(query.toLowerCase()) || String(s.id).includes(query);
  }

  function matchesStatus(s: Submission){
    if (statusFilter === 'all') return true;
    return s.status === statusFilter;
  }

  function sortSubs(list: Submission[]) {
    if (sortBy === 'newest') return [...list].sort((a,b)=> +new Date(b.created_at) - +new Date(a.created_at));
    if (sortBy === 'oldest') return [...list].sort((a,b)=> +new Date(a.created_at) - +new Date(b.created_at));
    if (sortBy === 'status') return [...list].sort((a,b)=> a.status.localeCompare(b.status));
    return list;
  }

  function countByStatus(status: string){
    return subs.filter(s=>s.status===status).length;
  }

  function passRatio(s: Submission){
    const total = s.results?.length ?? 0;
    const passed = (s.results ?? []).filter(r=>r.status==='passed').length;
    return { passed, total };
  }

  function avgRuntimeMs(s: Submission){
    const rs = (s.results ?? []).map(r=>r.runtime_ms).filter((x)=> typeof x === 'number');
    if (!rs.length) return null;
    const avg = Math.round(rs.reduce((a,b)=>a+b,0)/rs.length);
    return avg;
  }

  function passPct(s: Submission){
    const r = passRatio(s);
    return Math.round((r.passed/Math.max(1, r.total))*100);
  }

  $: try { localStorage.setItem('subs_layout', layoutMode) } catch {}
  $: filtered = sortSubs(subs.filter(matchesQuery).filter(matchesStatus));
  $: grouped = {
    running: filtered.filter(s=>s.status==='running'),
    completed: filtered.filter(s=>s.status==='completed'),
    failed: filtered.filter(s=>s.status==='failed'),
    other: filtered.filter(s=> !['running','completed','failed'].includes(s.status))
  };
</script>

{#if loading}
  <div class="space-y-6">
    <div class="flex items-end justify-between gap-4">
      <div class="skeleton h-10 w-48"></div>
      <div class="skeleton h-10 w-80"></div>
    </div>
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each Array(6) as _}
        <div class="card bg-base-100 shadow animate-pulse">
          <div class="card-body space-y-3">
            <div class="skeleton h-4 w-24"></div>
            <div class="skeleton h-6 w-3/4"></div>
            <div class="skeleton h-4 w-1/2"></div>
            <div class="skeleton h-3 w-full"></div>
          </div>
        </div>
      {/each}
    </div>
  </div>
{:else}
  <div class="space-y-6">
    <div class="flex flex-col md:flex-row md:items-end md:justify-between gap-4">
      <div>
        <h1 class="text-3xl font-semibold tracking-tight">{t('frontend/src/routes/submissions/+page.svelte::my_submissions')}</h1>
        <p class="text-sm opacity-70">{t('frontend/src/routes/submissions/+page.svelte::track_grading_progress')}</p>
      </div>
      <div class="flex flex-wrap gap-2 items-center">
        <div class="join">
          <input class="input input-bordered join-item" placeholder={t('frontend/src/routes/submissions/+page.svelte::search_by_assignment_or_id')} bind:value={query}>
          <select class="select select-bordered join-item" bind:value={statusFilter}>
            <option value="all">{t('frontend/src/routes/submissions/+page.svelte::all_filter')}</option>
            <option value="running">{t('frontend/src/routes/submissions/+page.svelte::running_filter')}</option>
            <option value="completed">{t('frontend/src/routes/submissions/+page.svelte::completed_filter')}</option>
            <option value="failed">{t('frontend/src/routes/submissions/+page.svelte::failed_filter')}</option>
          </select>
          <select class="select select-bordered join-item" bind:value={sortBy}>
            <option value="newest">{t('frontend/src/routes/submissions/+page.svelte::newest')}</option>
            <option value="oldest">{t('frontend/src/routes/submissions/+page.svelte::oldest')}</option>
            <option value="status">{t('frontend/src/routes/submissions/+page.svelte::status_sort')}</option>
          </select>
        </div>
        <div class="join ml-auto">
          <button class={`btn join-item ${layoutMode==='table' ? 'btn-primary' : 'btn-ghost'}`} on:click={() => layoutMode='table'}>{t('frontend/src/routes/submissions/+page.svelte::table_layout')}</button>
          <button class={`btn join-item ${layoutMode==='cards' ? 'btn-primary' : 'btn-ghost'}`} on:click={() => layoutMode='cards'}>{t('frontend/src/routes/submissions/+page.svelte::cards_layout')}</button>
          <button class={`btn join-item ${layoutMode==='board' ? 'btn-primary' : 'btn-ghost'}`} on:click={() => layoutMode='board'}>{t('frontend/src/routes/submissions/+page.svelte::board_layout')}</button>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
      <div class="stat bg-base-100 rounded-box shadow">
        <div class="stat-title">{t('frontend/src/routes/submissions/+page.svelte::total_submissions')}</div>
        <div class="stat-value text-primary">{subs.length}</div>
      </div>
      <div class="stat bg-base-100 rounded-box shadow">
        <div class="stat-title">{t('frontend/src/routes/submissions/+page.svelte::running_submissions')}</div>
        <div class="stat-value text-info">{countByStatus('running')}</div>
      </div>
      <div class="stat bg-base-100 rounded-box shadow">
        <div class="stat-title">{t('frontend/src/routes/submissions/+page.svelte::completed_submissions')}</div>
        <div class="stat-value text-success">{countByStatus('completed')}</div>
      </div>
      <div class="stat bg-base-100 rounded-box shadow">
        <div class="stat-title">{t('frontend/src/routes/submissions/+page.svelte::failed_submissions')}</div>
        <div class="stat-value text-error">{countByStatus('failed')}</div>
      </div>
    </div>

    {#if filtered.length}
      {#if layoutMode === 'table'}
        <div class="overflow-x-auto">
          <table class="table">
            <thead>
              <tr>
                <th>{t('frontend/src/routes/submissions/+page.svelte::assignment_th')}</th>
                <th>{t('frontend/src/routes/submissions/+page.svelte::submitted_th')}</th>
                <th>{t('frontend/src/routes/submissions/+page.svelte::status_th_table')}</th>
                <th>{t('frontend/src/routes/submissions/+page.svelte::pass_th')}</th>
                <th>{t('frontend/src/routes/submissions/+page.svelte::avg_ms_th')}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each filtered as s}
                <tr class="hover">
                  <td class="max-w-xs">
                    <div class="truncate" title={titles[s.assignment_id] ?? String(s.assignment_id)}>
                      {titles[s.assignment_id] ?? s.assignment_id}
                    </div>
                    <div class="text-xs opacity-60">#{s.id}</div>
                  </td>
                  <td class="whitespace-nowrap">{formatDateTime(s.created_at)}</td>
                  <td><span class={`badge ${statusColor(s.status)}`}>{s.status}</span> {#if s.manually_accepted}<span class="badge badge-xs badge-outline badge-success ml-2" title={t('frontend/src/routes/submissions/+page.svelte::accepted_by_teacher')}>{t('frontend/src/routes/submissions/+page.svelte::accepted_badge')}</span>{/if}</td>
                  <td>
                    {#if s.results && s.results.length}
                      {Math.round((passRatio(s).passed / Math.max(1, passRatio(s).total)) * 100)}%
                    {:else}
                      <span class="opacity-60">{t('frontend/src/routes/submissions/+page.svelte::pending_results')}</span>
                    {/if}
                  </td>
                  <td>{avgRuntimeMs(s) ?? '-'}</td>
                  <td class="text-right">
                    <a class="btn btn-sm" href={`/submissions/${s.id}`}>{t('frontend/src/routes/submissions/+page.svelte::open_submission')}</a>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else if layoutMode === 'cards'}
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {#each filtered as s}
            <a href={`/submissions/${s.id}`} class="card bg-base-100/70 backdrop-blur border border-base-300 hover:border-base-200 shadow-sm hover:shadow transition">
              <div class="card-body gap-3">
                <div class="flex items-center justify-between gap-3">
                  <div class="text-xs opacity-70">{formatDateTime(s.created_at)}</div>
                  <span class={`badge ${statusColor(s.status)}`}>{s.status}</span>
                </div>
                <h2 class="card-title text-base md:text-lg truncate">{titles[s.assignment_id] ?? s.assignment_id}</h2>
                <div class="flex items-center gap-3">
                  <svg class="w-10 h-10" viewBox="0 0 40 40" aria-hidden="true">
                    <circle cx="20" cy="20" r="16" class="text-base-300" stroke="currentColor" stroke-width="4" fill="none" />
                    {#if s.results && s.results.length}
                      <circle cx="20" cy="20" r="16" stroke="currentColor" class="text-success"
                        stroke-width="4" fill="none"
                        stroke-dasharray={`${(2*Math.PI*16).toFixed(2)} ${(2*Math.PI*16).toFixed(2)}`}
                        stroke-dashoffset={`${((1 - passPct(s)/100) * 2*Math.PI*16).toFixed(2)}`}
                        pathLength="100"
                        transform="rotate(-90 20 20)" />
                      <text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" class="fill-current text-xs">{passPct(s)}%</text>
                    {:else}
                      <text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" class="fill-current text-xs opacity-60">…</text>
                    {/if}
                  </svg>
                  <div class="text-xs opacity-70">
                    {#if s.results && s.results.length}
                      {passRatio(s).passed}/{passRatio(s).total} {t('frontend/src/routes/submissions/+page.svelte::passed_count')}
                    {:else}
                      {t('frontend/src/routes/submissions/+page.svelte::pending_tests')}
                    {/if}
                  </div>
                  <div class="ml-auto text-xs opacity-70">
                    {#if avgRuntimeMs(s) !== null}
                      {t('frontend/src/routes/submissions/+page.svelte::avg_runtime')} {avgRuntimeMs(s)}ms
                    {/if}
                  </div>
                </div>
                {#if s.manually_accepted}
                  <div class="text-xs text-success">{t('frontend/src/routes/submissions/+page.svelte::manually_accepted')}</div>
                {/if}
              </div>
            </a>
          {/each}
        </div>
      {:else}
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-info"></div>
              <div class="font-medium">{t('frontend/src/routes/submissions/+page.svelte::running_board_title')}</div>
              <span class="badge badge-ghost">{grouped.running.length}</span>
            </div>
            <div class="space-y-2">
              {#each grouped.running as s}
                <a href={`/submissions/${s.id}`} class="block p-3 rounded border border-base-300 hover:border-info/60 bg-base-100/60 hover:bg-base-100 transition">
                  <div class="text-xs opacity-70 flex justify-between">
                    <span>{formatDateTime(s.created_at)}</span>
                    <span class={`badge badge-xs ${statusColor(s.status)}`}>{s.status}</span>
                  </div>
                  <div class="truncate font-medium">{titles[s.assignment_id] ?? s.assignment_id}</div>
                  <div class="text-xs opacity-70">#{s.id}</div>
                </a>
              {/each}
            </div>
          </div>
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-success"></div>
              <div class="font-medium">{t('frontend/src/routes/submissions/+page.svelte::completed_board_title')}</div>
              <span class="badge badge-ghost">{grouped.completed.length}</span>
            </div>
            <div class="space-y-2">
              {#each grouped.completed as s}
                <a href={`/submissions/${s.id}`} class="block p-3 rounded border border-base-300 hover:border-success/60 bg-base-100/60 hover:bg-base-100 transition">
                  <div class="text-xs opacity-70 flex justify-between">
                    <span>{formatDateTime(s.created_at)}</span>
                    <span class={`badge badge-xs ${statusColor(s.status)}`}>{s.status}</span>
                  </div>
                  <div class="truncate font-medium">{titles[s.assignment_id] ?? s.assignment_id}</div>
                  <div class="text-xs opacity-70">#{s.id}</div>
                </a>
              {/each}
            </div>
          </div>
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-error"></div>
              <div class="font-medium">{t('frontend/src/routes/submissions/+page.svelte::failed_board_title')}</div>
              <span class="badge badge-ghost">{grouped.failed.length}</span>
            </div>
            <div class="space-y-2">
              {#each grouped.failed as s}
                <a href={`/submissions/${s.id}`} class="block p-3 rounded border border-base-300 hover:border-error/60 bg-base-100/60 hover:bg-base-100 transition">
                  <div class="text-xs opacity-70 flex justify-between">
                    <span>{formatDateTime(s.created_at)}</span>
                    <span class={`badge badge-xs ${statusColor(s.status)}`}>{s.status}</span>
                  </div>
                  <div class="truncate font-medium">{titles[s.assignment_id] ?? s.assignment_id}</div>
                  <div class="text-xs opacity-70">#{s.id}</div>
                </a>
              {/each}
            </div>
          </div>
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-base-300"></div>
              <div class="font-medium">{t('frontend/src/routes/submissions/+page.svelte::other_board_title')}</div>
              <span class="badge badge-ghost">{grouped.other.length}</span>
            </div>
            <div class="space-y-2">
              {#each grouped.other as s}
                <a href={`/submissions/${s.id}`} class="block p-3 rounded border border-base-300 hover:border-base-200 bg-base-100/60 hover:bg-base-100 transition">
                  <div class="text-xs opacity-70 flex justify-between">
                    <span>{formatDateTime(s.created_at)}</span>
                    <span class={`badge badge-xs ${statusColor(s.status)}`}>{s.status}</span>
                  </div>
                  <div class="truncate font-medium">{titles[s.assignment_id] ?? s.assignment_id}</div>
                  <div class="text-xs opacity-70">#{s.id}</div>
                </a>
              {/each}
            </div>
          </div>
        </div>
      {/if}
    {:else}
      <div class="text-center py-16 opacity-70">
        <p>{t('frontend/src/routes/submissions/+page.svelte::no_submissions_match_filters')}</p>
      </div>
    {/if}

    {#if err}<p class="text-error">{err}</p>{/if}
  </div>
{/if}

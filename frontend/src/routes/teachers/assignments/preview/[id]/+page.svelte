<script lang="ts">
// @ts-nocheck
import { onMount } from 'svelte';
import { page } from '$app/stores';
import { goto } from '$app/navigation';
import { apiJSON } from '$lib/api';
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { formatDateTime } from '$lib/date';
import { t, translator } from '$lib/i18n'; // Added

let translate; // Added
$: translate = $translator; // Added

let id = $page.params.id;
$: id = $page.params.id;

// Data
let assignment:any=null;
let tests:any[]=[];
let testsCount = 0;
let subStats: Record<string, {passed:number, total:number}> = {};
let allSubs:any[]=[];       // all student submissions (teacher view)
let teacherRuns:any[]=[];   // teacher runs
let students:any[]=[];      // class roster
let progress:any[]=[];      // computed per-student
let canViewInstructor = false;
let expanded:number|null=null;
let loading=true;
let err='';
let safeDesc='';

// Teacher run state
let solFiles: File[] = [];
let isSolDragging = false;
let solLoading = false;
let teacherRunDialog: HTMLDialogElement;

// Copy (import) to class state
let copyDialog: HTMLDialogDialogElement;
let myClasses: any[] = [];
let copyClassId: string | null = null;
let copyErr = '';
let copyLoading = false;

async function openCopyToClass(){
  copyErr = '';
  copyClassId = null;
  copyLoading = true;
  try {
    myClasses = await apiJSON('/api/classes');
  } catch (e:any) {
    copyErr = e.message;
  }
  copyLoading = false;
  copyDialog?.showModal();
}

async function doCopyToClass(){
  if (!copyClassId) { copyErr = t('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::choose_class_error'); return; } // Localized
  try{
    const res = await apiJSON(`/api/classes/${copyClassId}/assignments/import`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ source_assignment_id: assignment.id }) });
    copyDialog?.close();
    if (res?.assignment_id) {
      await goto(`/assignments/${res.assignment_id}?new=1`);
    }
  }catch(e:any){
    copyErr = e.message;
  }
}

function openTeacherRunModal(){
  teacherRunDialog?.showModal();
}

async function runTeacherSolution(){
  if (!solFiles.length) return;
  const fd = new FormData();
  for (const f of solFiles) fd.append('files', f);
  try{
    solLoading = true;
    await apiJSON(`/api/assignments/${id}/solution-run`, { method: 'POST', body: fd });
    solFiles = [];
    teacherRunDialog?.close();
    await load();
    // Activate Teacher runs tab
    try{
      const tabs = document.querySelectorAll('.tabs .tab');
      tabs.forEach(el => el.classList.remove('tab-active'));
      // Localized the string used for comparison
      const runsBtn = Array.from(tabs).find(el => el.textContent?.trim().toLowerCase().includes(t('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::teacher_runs_lowercase'))) as HTMLElement | undefined;
      if (runsBtn) runsBtn.classList.add('tab-active');
      const ov = document.getElementById('pv-overview');
      const ins = document.getElementById('pv-instructor');
      const rn = document.getElementById('pv-runs');
      if (ov) ov.hidden = true;
      if (ins) ins.hidden = true;
      if (rn) rn.hidden = false;
    }catch{}
  }catch(e:any){
    err = e.message;
  }finally{
    solLoading = false;
  }
}

function activateTab(el: HTMLElement, tab: 'overview'|'instructor'|'runs'){
  const container = el?.parentElement;
  container?.querySelectorAll('.tab').forEach(t => t.classList.remove('tab-active'));
  el.classList.add('tab-active');
  const ov = document.getElementById('pv-overview');
  const ins = document.getElementById('pv-instructor');
  const rn = document.getElementById('pv-runs');
  if (ov) ov.hidden = tab !== 'overview';
  if (ins) ins.hidden = tab !== 'instructor';
  if (rn) rn.hidden = tab !== 'runs';
}

function statusColor(s:string){
  if(s==='completed') return 'badge-success';
  if(s==='running') return 'badge-info';
  if(s==='failed') return 'badge-error';
  if(s==='passed') return 'badge-success';
  if(s==='wrong_output') return 'badge-error';
  if(s==='runtime_error') return 'badge-error';
  if(s==='illegal_tool_use') return 'badge-error';
  if(s==='time_limit_exceeded' || s==='memory_limit_exceeded') return 'badge-warning';
  return '';
}

function policyLabel(policy:string){
  if(policy==='all_or_nothing') return t('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::all_or_nothing'); // Localized
  if(policy==='weighted') return t('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::weighted'); // Localized
  return policy;
}

async function toggleStudent(sid:number){
  const next = expanded===sid ? null : sid;
  expanded = next;
  if(next !== null){
    const entry = progress.find((p:any)=>p.student.id===next);
    if(entry){
      await loadSubmissionStats(entry.all);
    }
  }
}

async function loadSubmissionStats(list?: any[], reset=false){
  try{
    const source = Array.isArray(list) && list.length ? list : allSubs;
    if(reset) subStats = {};
    if(testsCount>0 && Array.isArray(source) && source.length){
      const targets = reset ? source : source.filter((s:any)=>!subStats[s.id]);
      if(!targets.length) return;
      const pairs = await Promise.all(targets.map(async (s:any)=>{
        try{
          const subData = await apiJSON(`/api/submissions/${s.id}`);
          const res = subData.results ?? [];
          const passed = Array.isArray(res) ? res.filter((r:any)=>r.status==='passed').length : 0;
          return [s.id, {passed, total: res.length}] as const;
        }catch{
          return [s.id, {passed: 0, total: 0}] as const;
        }
      }));
      const nextStats: Record<string, {passed:number, total:number}> = reset ? {} : {...subStats};
      for(const [sid, st] of pairs){ nextStats[sid] = st; }
      subStats = nextStats;
    }
  }catch{}
}

async function load(){
  loading=true; err='';
  try{
    const data = await apiJSON(`/api/assignments/${id}`);
    assignment = data.assignment;
    tests = data.tests ?? [];
    testsCount = Array.isArray(tests) ? tests.length : 0;
    subStats = {};
    allSubs = data.submissions ?? [];
    teacherRuns = data.teacher_runs ?? [];
    try { safeDesc = DOMPurify.sanitize((marked.parse(assignment.description) as string) || ''); } catch { safeDesc=''; }
    // Load class roster and compute progress (optional)
    try {
      const cls = await apiJSON(`/api/classes/${assignment.class_id}`);
      students = cls.students ?? [];
      progress = students.map((s:any)=>{
        const subs = allSubs.filter((x:any)=>x.student_id===s.id);
        const latest = subs[0];
        const hasCompleted = subs.some((x:any)=>x.status==='completed' || x.status==='passed');
        const displayStatus = hasCompleted ? 'completed' : (latest ? latest.status : 'none');
        return {student:s, latest, all:subs, displayStatus};
      });
      canViewInstructor = true;
    } catch {}
  }catch(e:any){ err=e.message }
  loading=false;
}

onMount(load);
</script>

{#if loading}
  <div class="p-6"><span class="loading loading-dots"></span></div>
{:else if err}
  <div class="p-6 text-error">{err}</div>
{:else if assignment}
  <!-- Hero header (mirrors main assignment style, without deadlines/actions) -->
  <section class="relative overflow-hidden mb-6 rounded-2xl border border-base-300/60 bg-gradient-to-br from-primary/10 to-secondary/10 p-0">
    <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-0 sm:gap-6">
      <div class="flex-1 p-6">
        <div class="flex items-center justify-between gap-3">
          <h1 class="text-2xl sm:text-3xl font-semibold tracking-tight">{assignment.title}</h1>
          <div class="flex items-center gap-2">
            <button class="btn btn-sm" on:click={() => history.back()}><i class="fa-solid fa-arrow-left mr-2"></i>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::back')}</button>
            <button class="btn btn-primary btn-sm" on:click={openCopyToClass}><i class="fa-solid fa-copy mr-2"></i>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::add_to_my_class')}</button>
          </div>
        </div>
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <span class="badge badge-ghost">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::max_label')}{assignment.max_points} {translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::points_abbr')}</span>
          <span class="badge badge-ghost">{policyLabel(assignment.grading_policy)}</span>
          {#if assignment.manual_review}
            <span class="badge badge-info">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::manual_review_badge')}</span>
          {/if}
          {#if assignment.published}
            <span class="badge badge-success">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::published')}</span>
          {:else}
            <span class="badge badge-warning">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::draft')}</span>
          {/if}
          <!-- No deadline badges in preview -->
        </div>
        <div class="mt-4 text-xs opacity-70">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::readonly_preview')}</div>
      </div>
    </div>
  </section>

  <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
    <div class="lg:col-span-12">
      <div class="tabs tabs-boxed w-full mb-4">
        <button class="tab tab-active" aria-current="page" on:click={(e)=>activateTab(e.currentTarget as HTMLElement, 'overview')}>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::overview_tab')}</button>
        <button class="tab" on:click={(e)=>activateTab(e.currentTarget as HTMLElement, 'runs')}>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::teacher_runs_tab')}</button>
      </div>

      <section id="pv-overview" class="card-elevated p-6 space-y-4">
        <div class="markdown assignment-description">{@html safeDesc}</div>
        <div class="grid sm:grid-cols-3 gap-3">
          <div class="stat bg-base-100 rounded-xl border border-base-300/60">
            <div class="stat-title">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::max_points_stat_title')}</div>
            <div class="stat-value text-lg">{assignment.max_points}</div>
            <div class="stat-desc">{policyLabel(assignment.grading_policy)}</div>
          </div>
          <div class="stat bg-base-100 rounded-xl border border-base-300/60">
            <div class="stat-title">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::traceback_visible_stat_title')}</div>
            <div class="stat-value text-lg">{assignment.show_traceback ? translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::yes') : translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::no')}</div>
            <div class="stat-desc">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::student_error_output_desc')}</div>
          </div>
          <div class="stat bg-base-100 rounded-xl border border-base-300/60">
            <div class="stat-title">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::manual_review_stat_title')}</div>
            <div class="stat-value text-lg">{assignment.manual_review ? translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::yes') : translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::no')}</div>
            <div class="stat-desc">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::grading_method_desc')}</div>
          </div>
        </div>
      </section>

      <section id="pv-instructor" class="card-elevated p-6 space-y-4" hidden>
        <div class="flex items-center justify-between">
          <h3 class="font-semibold text-lg">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::student_progress_heading')}</h3>
        </div>
        <div class="overflow-x-auto">
          <table class="table table-zebra">
            <thead>
              <tr>
                <th>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::student_table_header')}</th>
                <th>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::status_table_header')}</th>
                <th>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::last_submission_table_header')}</th>
              </tr>
            </thead>
            <tbody>
              {#each progress as p (p.student.id)}
                <tr class="cursor-pointer" on:click={() => toggleStudent(p.student.id)}>
                  <td>{p.student.name ?? p.student.email}</td>
                  <td><span class={`badge ${statusColor(p.displayStatus)}`}>{p.displayStatus}</span></td>
                  <td>{p.latest ? formatDateTime(p.latest.created_at) : '-'}</td>
                </tr>
                {#if expanded === p.student.id}
                  <tr>
                    <td colspan="3">
                      {#if p.all && p.all.length}
                        <ul class="timeline timeline-vertical timeline-compact m-0 p-0">
                          {#each p.all as s, i}
                            <li>
                              {#if i !== 0}<hr />{/if}
                              <div class="timeline-middle">
                                {#if s.status === 'completed' || s.status === 'passed'}
                                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-5 w-5 text-success">
                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clip-rule="evenodd" />
                                  </svg>
                                {:else}
                                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-5 w-5 text-error">
                                    <path fill-rule="evenodd" d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16ZM8.28 7.22a.75.75 0 0 0-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 1 0 1.06 1.06L10 11.06l1.72 1.72a.75.75 0 1 0 1.06-1.06L11.06 10l1.72-1.72a.75.75 0 0 0-1.06-1.06L10 8.94 8.28 7.22Z" clip-rule="evenodd" />
                                  </svg>
                                {/if}
                              </div>
                              <div class="timeline-end timeline-box m-0 space-y-2">
                                <div class="flex flex-wrap items-center gap-2">
                                  <span class="mr-2 text-xs opacity-70">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::attempt_prefix')}#{s.attempt_number ?? '?'}</span>
                                  <a class="link" href={`/submissions/${s.id}`}>{formatDateTime(s.created_at)}</a>
                                  {#if s.manually_accepted}
                                    <span class="badge badge-xs badge-outline badge-success" title="{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::accepted_badge')}">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::accepted_badge')}</span>
                                  {/if}
                                </div>
                                <div class="flex flex-wrap items-center gap-2 text-xs">
                                  {#if testsCount>0}
                                    <span class="badge badge-ghost badge-xs">
                                      {#if subStats[s.id]}
                                        {subStats[s.id].passed} / {subStats[s.id].total || testsCount} {translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::tests_suffix')}
                                      {:else}
                                        - / - {translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::tests_suffix')}
                                      {/if}
                                    </span>
                                  {/if}
                                  <span class="badge badge-outline badge-xs">{(s.override_points ?? s.points ?? 0)} {translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::points_abbr_badge')}</span>
                                </div>
                              </div>
                              {#if i !== p.all.length - 1}<hr />{/if}
                            </li>
                          {/each}
                        </ul>
                      {:else}
                        <i>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::no_submissions')}</i>
                      {/if}
                    </td>
                  </tr>
                {/if}
              {/each}
              {#if !progress.length}
                <tr><td colspan="3"><i>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::no_students')}</i></td></tr>
              {/if}
            </tbody>
          </table>
        </div>
      </section>

      <section id="pv-runs" class="card-elevated p-6 space-y-3" hidden>
        <div class="flex items-center justify-between">
          <h3 class="font-semibold text-lg">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::your_runs_heading')}</h3>
          <button class="btn btn-sm" on:click={openTeacherRunModal}>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::new_run_button')}</button>
        </div>
        <div class="overflow-x-auto">
          <table class="table table-zebra">
            <thead>
              <tr>
                <th>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::date_table_header')}</th>
                <th>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::status_table_header')}</th>
                <th>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::first_failure_table_header')}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each teacherRuns as s}
                <tr>
                  <td>{formatDateTime(s.created_at)}</td>
                  <td><span class={`badge ${statusColor(s.status)}`}>{s.status}</span></td>
                  <td>{s.failure_reason ?? '-'}</td>
                  <td><a class="btn btn-sm btn-outline" href={`/submissions/${s.id}`}>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::view_button')}</a></td>
                </tr>
              {/each}
              {#if !teacherRuns.length}
                <tr><td colspan="4"><i>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::no_runs_yet')}</i></td></tr>
              {/if}
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <!-- Right side: Optional details (kept minimal for preview) -->
    <aside class="lg:col-span-4 space-y-4">
      <div class="card-elevated p-5">
        <h3 class="font-semibold mb-2">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::details_heading')}</h3>
        <ul class="text-sm space-y-1">
          <li><b>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::template_label')}</b> {assignment.template_path ? translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::present_status') : translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::none_status')}</li>
          <li><b>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::created_label')}</b> {formatDateTime(assignment.created_at)}</li>
          <li><b>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::updated_label')}</b> {formatDateTime(assignment.updated_at)}</li>
          <li><b>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::tests_label')}</b> {tests.length}</li>
        </ul>
      </div>
    </aside>
  </div>
{/if}

<!-- Teacher run upload modal -->
<dialog bind:this={teacherRunDialog} class="modal">
  <div class="modal-box w-11/12 max-w-lg space-y-4">
    <h3 class="font-bold text-lg">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::new_teacher_run_modal_title')}</h3>
    <div
      role="region"
      aria-label="Teacher solution dropzone"
      class={`border-2 border-dashed rounded-xl p-6 text-center transition ${isSolDragging ? 'bg-base-200' : 'bg-base-100'}`}
      on:dragover|preventDefault={() => isSolDragging = true}
      on:dragleave={() => isSolDragging = false}
      on:drop|preventDefault={(e)=>{ isSolDragging=false; const dt=(e as DragEvent).dataTransfer; if(dt){ solFiles=[...solFiles, ...Array.from(dt.files)].filter(f=>f.name.endsWith('.py')) } }}
    >
      <div class="text-sm opacity-70 mb-2">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::drag_and_drop_files_instruction')}</div>
      <div class="mb-3">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::or_connector')}</div>
      <input type="file" accept=".py" multiple class="file-input file-input-bordered w-full"
        on:change={e=>solFiles=Array.from((e.target as HTMLInputElement).files||[])}>
    </div>
    {#if solFiles.length}
      <div class="text-sm opacity-70">{solFiles.length} {solFiles.length===1 ? translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::file_singular') : translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::file_plural')} {translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::selected_suffix')}</div>
    {/if}
    <div class="modal-action">
      <button class={`btn btn-primary ${solLoading ? 'loading' : ''}`} on:click={runTeacherSolution} disabled={!solFiles.length || solLoading}>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::run_button')}</button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::close_button')}</button></form>
</dialog>

<!-- Copy to class modal -->
<dialog bind:this={copyDialog} class="modal">
  <div class="modal-box w-11/12 max-w-md">
    <h3 class="font-bold mb-3">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::add_to_my_class_modal_title')}</h3>
    {#if copyLoading}
      <div class="py-4 text-center"><span class="loading loading-dots"></span></div>
    {:else}
      <label class="label"><span class="label-text">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::choose_class_label')}</span></label>
      <select class="select select-bordered w-full" bind:value={copyClassId}>
        <option value="" disabled selected>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::select_option_placeholder')}</option>
        {#each myClasses as c}
          <option value={c.id}>{c.name}</option>
        {/each}
      </select>
      {#if copyErr}<p class="text-error mt-2">{copyErr}</p>{/if}
      <div class="modal-action">
        <form method="dialog"><button class="btn">{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::cancel_button')}</button></form>
        <button class="btn btn-primary" on:click|preventDefault={doCopyToClass}>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::add_button')}</button>
      </div>
    {/if}
  </div>
  <form method="dialog" class="modal-backdrop"><button>{translate('frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::close_button')}</button></form>
</dialog>

<style>
  @import '@fortawesome/fontawesome-free/css/all.min.css';
  .markdown :global(p) { margin: 0.5rem 0; }
  .markdown :global(code) { background: hsl(var(--b2)); padding: 0.1rem 0.25rem; border-radius: 0.25rem; }
</style>

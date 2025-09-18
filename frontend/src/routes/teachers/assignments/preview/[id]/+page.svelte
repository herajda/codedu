<script lang="ts">
// @ts-nocheck
import { onMount } from 'svelte';
import { page } from '$app/stores';
import { goto } from '$app/navigation';
import { apiJSON } from '$lib/api';
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { formatDateTime } from '$lib/date';

let id = $page.params.id;
$: id = $page.params.id;

// Data
let assignment:any=null;
let tests:any[]=[];
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
let copyDialog: HTMLDialogElement;
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
  if (!copyClassId) { copyErr = 'Choose a class'; return; }
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
      const runsBtn = Array.from(tabs).find(el => el.textContent?.trim().toLowerCase().includes('teacher runs')) as HTMLElement | undefined;
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
  if(s==='time_limit_exceeded' || s==='memory_limit_exceeded') return 'badge-warning';
  return '';
}

function policyLabel(policy:string){
  if(policy==='all_or_nothing') return 'All or nothing';
  if(policy==='weighted') return 'Weighted';
  return policy;
}

function toggleStudent(sid:number){
  expanded = expanded===sid ? null : sid;
}

async function load(){
  loading=true; err='';
  try{
    const data = await apiJSON(`/api/assignments/${id}`);
    assignment = data.assignment;
    tests = data.tests ?? [];
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
            <button class="btn btn-sm" on:click={() => history.back()}><i class="fa-solid fa-arrow-left mr-2"></i>Back</button>
            <button class="btn btn-primary btn-sm" on:click={openCopyToClass}><i class="fa-solid fa-copy mr-2"></i>Add to my class</button>
          </div>
        </div>
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <span class="badge badge-ghost">Max {assignment.max_points} pts</span>
          <span class="badge badge-ghost">{policyLabel(assignment.grading_policy)}</span>
          {#if assignment.manual_review}
            <span class="badge badge-info">Manual review</span>
          {/if}
          {#if assignment.published}
            <span class="badge badge-success">Published</span>
          {:else}
            <span class="badge badge-warning">Draft</span>
          {/if}
          <!-- No deadline badges in preview -->
        </div>
        <div class="mt-4 text-xs opacity-70">Read‑only preview</div>
      </div>
    </div>
  </section>

  <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
    <div class="lg:col-span-8">
      <div class="tabs tabs-boxed w-full mb-4">
        <button class="tab tab-active" aria-current="page" on:click={(e)=>activateTab(e.currentTarget as HTMLElement, 'overview')}>Overview</button>
        <button class="tab" on:click={(e)=>activateTab(e.currentTarget as HTMLElement, 'runs')}>Teacher runs</button>
      </div>

      <section id="pv-overview" class="card-elevated p-6 space-y-4">
        <div class="markdown">{@html safeDesc}</div>
        <div class="grid sm:grid-cols-3 gap-3">
          <div class="stat bg-base-100 rounded-xl border border-base-300/60">
            <div class="stat-title">Max points</div>
            <div class="stat-value text-lg">{assignment.max_points}</div>
            <div class="stat-desc">{policyLabel(assignment.grading_policy)}</div>
          </div>
          <div class="stat bg-base-100 rounded-xl border border-base-300/60">
            <div class="stat-title">Traceback visible</div>
            <div class="stat-value text-lg">{assignment.show_traceback ? 'Yes' : 'No'}</div>
            <div class="stat-desc">student error output</div>
          </div>
          <div class="stat bg-base-100 rounded-xl border border-base-300/60">
            <div class="stat-title">Manual review</div>
            <div class="stat-value text-lg">{assignment.manual_review ? 'Yes' : 'No'}</div>
            <div class="stat-desc">grading method</div>
          </div>
        </div>
      </section>

      <section id="pv-instructor" class="card-elevated p-6 space-y-4" hidden>
        <div class="flex items-center justify-between">
          <h3 class="font-semibold text-lg">Student progress</h3>
        </div>
        <div class="overflow-x-auto">
          <table class="table table-zebra">
            <thead>
              <tr><th>Student</th><th>Status</th><th>Last submission</th></tr>
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
                              <div class="timeline-end timeline-box flex items-center m-0">
                                <span class="mr-2 text-xs opacity-70">Attempt #{s.attempt_number ?? '?'}</span>
                                <a class="link" href={`/submissions/${s.id}`}>{formatDateTime(s.created_at)}</a>
                                {#if s.manually_accepted}
                                  <span class="badge badge-xs badge-outline badge-success ml-2" title="Accepted by teacher">accepted</span>
                                {/if}
                              </div>
                              {#if i !== p.all.length - 1}<hr />{/if}
                            </li>
                          {/each}
                        </ul>
                      {:else}
                        <i>No submissions</i>
                      {/if}
                    </td>
                  </tr>
                {/if}
              {/each}
              {#if !progress.length}
                <tr><td colspan="3"><i>No students</i></td></tr>
              {/if}
            </tbody>
          </table>
        </div>
      </section>

      <section id="pv-runs" class="card-elevated p-6 space-y-3" hidden>
        <div class="flex items-center justify-between">
          <h3 class="font-semibold text-lg">Your runs</h3>
          <button class="btn btn-sm" on:click={openTeacherRunModal}>New run</button>
        </div>
        <div class="overflow-x-auto">
          <table class="table table-zebra">
            <thead>
              <tr><th>Date</th><th>Status</th><th>First failure</th><th></th></tr>
            </thead>
            <tbody>
              {#each teacherRuns as s}
                <tr>
                  <td>{formatDateTime(s.created_at)}</td>
                  <td><span class={`badge ${statusColor(s.status)}`}>{s.status}</span></td>
                  <td>{s.failure_reason ?? '-'}</td>
                  <td><a class="btn btn-sm btn-outline" href={`/submissions/${s.id}`}>View</a></td>
                </tr>
              {/each}
              {#if !teacherRuns.length}
                <tr><td colspan="4"><i>No runs yet</i></td></tr>
              {/if}
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <!-- Right side: Optional details (kept minimal for preview) -->
    <aside class="lg:col-span-4 space-y-4">
      <div class="card-elevated p-5">
        <h3 class="font-semibold mb-2">Details</h3>
        <ul class="text-sm space-y-1">
          <li><b>Template:</b> {assignment.template_path ? 'Present' : 'None'}</li>
          <li><b>Created:</b> {formatDateTime(assignment.created_at)}</li>
          <li><b>Updated:</b> {formatDateTime(assignment.updated_at)}</li>
          <li><b>Tests:</b> {tests.length}</li>
        </ul>
      </div>
    </aside>
  </div>
{/if}

<!-- Teacher run upload modal -->
<dialog bind:this={teacherRunDialog} class="modal">
  <div class="modal-box w-11/12 max-w-lg space-y-4">
    <h3 class="font-bold text-lg">New teacher run</h3>
    <div
      role="region"
      aria-label="Teacher solution dropzone"
      class={`border-2 border-dashed rounded-xl p-6 text-center transition ${isSolDragging ? 'bg-base-200' : 'bg-base-100'}`}
      on:dragover|preventDefault={() => isSolDragging = true}
      on:dragleave={() => isSolDragging = false}
      on:drop|preventDefault={(e)=>{ isSolDragging=false; const dt=(e as DragEvent).dataTransfer; if(dt){ solFiles=[...solFiles, ...Array.from(dt.files)].filter(f=>f.name.endsWith('.py')) } }}
    >
      <div class="text-sm opacity-70 mb-2">Drag and drop reference .py files here</div>
      <div class="mb-3">or</div>
      <input type="file" accept=".py" multiple class="file-input file-input-bordered w-full"
        on:change={e=>solFiles=Array.from((e.target as HTMLInputElement).files||[])}>
    </div>
    {#if solFiles.length}
      <div class="text-sm opacity-70">{solFiles.length} file{solFiles.length===1?'':'s'} selected</div>
    {/if}
    <div class="modal-action">
      <button class={`btn btn-primary ${solLoading ? 'loading' : ''}`} on:click={runTeacherSolution} disabled={!solFiles.length || solLoading}>Run</button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

<!-- Copy to class modal -->
<dialog bind:this={copyDialog} class="modal">
  <div class="modal-box w-11/12 max-w-md">
    <h3 class="font-bold mb-3">Add to my class</h3>
    {#if copyLoading}
      <div class="py-4 text-center"><span class="loading loading-dots"></span></div>
    {:else}
      <label class="label"><span class="label-text">Choose class</span></label>
      <select class="select select-bordered w-full" bind:value={copyClassId}>
        <option value="" disabled selected>Select…</option>
        {#each myClasses as c}
          <option value={c.id}>{c.name}</option>
        {/each}
      </select>
      {#if copyErr}<p class="text-error mt-2">{copyErr}</p>{/if}
      <div class="modal-action">
        <form method="dialog"><button class="btn">Cancel</button></form>
        <button class="btn btn-primary" on:click|preventDefault={doCopyToClass}>Add</button>
      </div>
    {/if}
  </div>
  <form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

<style>
  @import '@fortawesome/fontawesome-free/css/all.min.css';
  .markdown :global(p) { margin: 0.5rem 0; }
  .markdown :global(code) { background: hsl(var(--b2)); padding: 0.1rem 0.25rem; border-radius: 0.25rem; }
</style>

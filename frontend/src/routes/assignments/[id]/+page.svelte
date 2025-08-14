<script lang="ts">
import { onMount, onDestroy, tick } from 'svelte'
  import { auth } from '$lib/auth'
import { apiFetch, apiJSON } from '$lib/api'
import { MarkdownEditor } from '$lib'
import { marked } from 'marked'
import { formatDateTime } from "$lib/date";
import DOMPurify from 'dompurify'
import { goto } from '$app/navigation'
import { page } from '$app/stores'



$: id = $page.params.id
let role = '';
$: role = $auth?.role ?? '';

  let assignment:any=null
  // tests moved to standalone page
  let submissions:any[]=[] // student submissions
  let latestSub:any=null
  let results:any[]=[]
  let esCtrl:{close:()=>void}|null=null
  let allSubs:any[]=[]     // teacher view
  let teacherRuns:any[]=[] // persisted teacher submissions
  let students:any[]=[]    // class roster for teacher
  let progress:any[]=[]    // computed progress per student
  let expanded:number|null=null
  let pointsEarned=0
  let done=false
  let percent=0
  let testsPassed=0
  let testsPercent=0
  let err=''
  // removed test creation inputs (moved to tests page)
  let files: File[] = []
  let templateFile:File|null=null
  // removed unittest file input (moved to tests page)
  let submitDialog: HTMLDialogElement;
  // removed tests dialog (moved to tests page)
$: percent = assignment ? Math.round(pointsEarned / assignment.max_points * 100) : 0;
$: testsPassed = results.filter((r:any) => r.status === 'passed').length;
$: testsPercent = results.length ? Math.round(testsPassed / results.length * 100) : 0;
  let editing=false
  let eTitle='', eDesc='', eDeadline='', ePoints=0, ePolicy='all_or_nothing', eShowTraceback=false
  let safeDesc=''
$: safeDesc = assignment ? DOMPurify.sanitize(marked.parse(assignment.description) as string) : ''

  // Enhanced UX state
  type TabKey = 'overview' | 'submissions' | 'results' | 'instructor' | 'teacher-runs'
  let activeTab: TabKey = 'overview'
  let isDragging = false

  // Teacher solution test-run state (modal in Teacher runs tab)
  let solFiles: File[] = []
  let isSolDragging = false
  let solLoading = false
  let teacherRunDialog: HTMLDialogElement

  function policyLabel(policy:string){
    if(policy==='all_or_nothing') return 'All or nothing'
    if(policy==='weighted') return 'Weighted'
    return policy
  }
  function relativeToDeadline(deadline:string){
    const now = new Date()
    const due = new Date(deadline)
    const diffMs = due.getTime() - now.getTime()
    const abs = Math.abs(diffMs)
    const mins = Math.round(abs / 60000)
    const hrs = Math.round(mins / 60)
    const days = Math.round(hrs / 24)
    if(abs < 60) return `${diffMs>=0 ? 'in' : ''} ${mins} min${mins===1?'':'s'}${diffMs<0 ? ' ago' : ''}`
    if(abs < 60*24) return `${diffMs>=0 ? 'in' : ''} ${hrs} hour${hrs===1?'':'s'}${diffMs<0 ? ' ago' : ''}`
    return `${diffMs>=0 ? 'in' : ''} ${days} day${days===1?'':'s'}${diffMs<0 ? ' ago' : ''}`
  }
  $: isOverdue = assignment ? new Date(assignment.deadline) < new Date() : false
  $: timeUntilDeadline = assignment ? new Date(assignment.deadline).getTime() - Date.now() : 0
  $: deadlineSoon = timeUntilDeadline > 0 && timeUntilDeadline <= 24 * 60 * 60 * 1000
  $: deadlineBadgeClass = isOverdue && !(role==='student' && done) ? 'badge-error' : 'badge-ghost'
  $: deadlineLabel = assignment ? (
      isOverdue
        ? (role==='student' && done
            ? `Deadline passed ${relativeToDeadline(assignment.deadline)}`
            : `Due ${relativeToDeadline(assignment.deadline)}`
          )
        : `Due ${relativeToDeadline(assignment.deadline)}`
    ) : ''

  async function publish(){
    try{
      await apiFetch(`/api/assignments/${id}/publish`,{method:'PUT'})
      await load()
    }catch(e:any){ err=e.message }
  }

  async function load(){
    err=''
    try{
      const data = await apiJSON(`/api/assignments/${id}`)
      assignment = data.assignment
      // If this was newly created, switch to edit mode by default
      if (role !== 'student' && typeof location !== 'undefined' && new URLSearchParams(location.search).get('new') === '1') {
        startEdit()
        history.replaceState(null, '', location.pathname)
      }
      if(role==='student') {
        submissions = data.submissions ?? []
        latestSub = submissions[0] ?? null
        results = []
        if(latestSub){
          const subData = await apiJSON(`/api/submissions/${latestSub.id}`)
          results = subData.results ?? []
        }
        const best = submissions.reduce((m:number,s:any)=>{
          const p = s.override_points ?? s.points ?? 0
          return p>m ? p : m
        },0)
        pointsEarned = best
        done = best >= assignment.max_points
      } else {
        allSubs = data.submissions ?? []
        teacherRuns = data.teacher_runs ?? []
        const cls = await apiJSON(`/api/classes/${assignment.class_id}`)
        students = cls.students ?? []
        progress = students.map((s:any)=>{
          const subs = allSubs.filter((x:any)=>x.student_id===s.id)
          const latest = subs[0]
          return {student:s, latest, all: subs}
        })
      }
    }catch(e:any){ err=e.message }
  }


  onMount(async () => {
    await load()
    // restore tab from URL
    try{
      const t = $page?.url?.searchParams?.get('tab') || ''
      if (t && isValidTab(t)) activeTab = t as TabKey
    }catch{}
    if(typeof sessionStorage!=='undefined'){
      const saved = sessionStorage.getItem(`assign-${id}-expanded`)
      if(saved) expanded = parseInt(saved)
      await tick()
      const scroll = sessionStorage.getItem(`assign-${id}-scroll`)
      if(scroll) window.scrollTo(0, parseInt(scroll))
    }
    window.addEventListener('beforeunload', saveState)
    
    const evs = new EventSource('/api/events')
    evs.addEventListener('status', (ev: MessageEvent) => {
      const d = JSON.parse((ev as MessageEvent).data)
      if(latestSub && d.submission_id===latestSub.id){
        latestSub.status = d.status
        if(d.status!== 'running') load()
      }
    })
    evs.addEventListener('result', (ev: MessageEvent) => {
      const d = JSON.parse((ev as MessageEvent).data)
      if(latestSub && d.submission_id===latestSub.id){
        results = [...results, d]
      }
    })
    esCtrl = { close: () => evs.close() }
  })


  onDestroy(()=>{
    esCtrl?.close()
    window.removeEventListener('beforeunload', saveState)
  })

  async function uploadTemplate(){
    if(!templateFile) return
    const fd = new FormData()
    fd.append('file', templateFile)
    try{
      await apiFetch(`/api/assignments/${id}/template`,{method:'POST', body:fd})
      templateFile=null
      await load()
    }catch(e:any){ err=e.message }
  }

  async function downloadTemplate(){
    try{
      const res = await apiFetch(`/api/assignments/${id}/template`)
      if(!res.ok) throw new Error('download failed')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = assignment.template_path.split('/').pop()
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    }catch(e:any){ err=e.message }
  }

  function startEdit(){
    editing=true
    eTitle=assignment.title
    eDesc=assignment.description
    eDeadline=assignment.deadline.slice(0,16)
    ePoints=assignment.max_points
    ePolicy=assignment.grading_policy
    eShowTraceback=assignment.show_traceback
  }

  async function saveEdit(){
    try{
      if(new Date(eDeadline)<new Date() && !confirm('The deadline is in the past. Continue?')) return
      await apiFetch(`/api/assignments/${id}`,{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({
          title:eTitle,
          description:eDesc,
          deadline:new Date(eDeadline).toISOString(),
          max_points:Number(ePoints),
          grading_policy:ePolicy,
          show_traceback:eShowTraceback
        })
      })
      editing=false
      await load()
    }catch(e:any){ err=e.message }
  }

  async function delAssignment(){
    if(!confirm('Delete this assignment?')) return
    try{
      await apiFetch(`/api/assignments/${id}`,{method:'DELETE'})
      goto(`/classes/${assignment.class_id}`)
    }catch(e:any){ err=e.message }
  }

  function saveState(){
    if(typeof sessionStorage==='undefined') return
    sessionStorage.setItem(`assign-${id}-expanded`, expanded===null ? '' : String(expanded))
    sessionStorage.setItem(`assign-${id}-scroll`, String(window.scrollY))
  }

  function toggleStudent(id:number){
    expanded = expanded===id ? null : id
    saveState()
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

  function openTeacherRunModal(){
    teacherRunDialog.showModal()
  }

  async function runTeacherSolution(){
    if (!solFiles.length) return
    const fd = new FormData()
    for (const f of solFiles) fd.append('files', f)
    try{
      solLoading = true
      await apiJSON(`/api/assignments/${id}/solution-run`, { method: 'POST', body: fd })
      solFiles = []
      teacherRunDialog.close()
      await load()
      activeTab = 'teacher-runs'
    }catch(e:any){
      err = e.message
    }finally{
      solLoading = false
    }
  }

  async function submit(){
    if(files.length === 0) return
    const fd = new FormData()
    for(const f of files){
      fd.append('files', f)
    }
    try{
      await apiFetch(`/api/assignments/${id}/submissions`,{method:'POST', body:fd})
      files = []
      submitDialog.close()
      alert('Uploaded!')
      await load()
    }catch(e:any){ err=e.message }
  }

  function openSubmitModal(){
    submitDialog.showModal()
  }

  function openTestsModal(){ goto(`/assignments/${id}/tests`) }

  // removed updateTest (moved to tests page)

  // Persist and restore selected tab via URL so back/forward keeps state
  function isValidTab(key: string): key is TabKey {
    const allowed: TabKey[] = ['overview']
    if (role==='student') {
      allowed.push('submissions','results')
    }
    if (role==='teacher' || role==='admin') {
      allowed.push('instructor','teacher-runs')
    }
    return allowed.includes(key as TabKey)
  }

  // initialize activeTab from URL once on mount (do not keep overwriting on every reactive cycle)

  function saveTabToUrl(){
    try{
      if(typeof location!=='undefined' && typeof history!=='undefined'){
        const url = new URL(location.href)
        url.searchParams.set('tab', activeTab)
        history.replaceState(history.state, '', url)
      }
    }catch{}
  }

  function setTab(tab: TabKey){
    activeTab = tab
    saveTabToUrl()
  }
</script>

{#if !assignment}
  <div class="flex items-center gap-3">
    <span class="loading loading-spinner loading-md"></span>
    <p>Loading assignment…</p>
  </div>
{:else}
  {#if editing}
    <div class="card-elevated mb-6">
      <div class="card-body space-y-4 p-6">
        <div class="flex items-center justify-between">
          <h1 class="card-title text-2xl">Edit assignment</h1>
          <div class="badge badge-outline">ID #{assignment.id}</div>
        </div>
        <input class="input input-bordered w-full" bind:value={eTitle} placeholder="Title" required>
        <MarkdownEditor bind:value={eDesc} placeholder="Description" />
        <div class="grid sm:grid-cols-2 gap-3">
          <input type="number" min="1" class="input input-bordered w-full" bind:value={ePoints} placeholder="Max points" required>
          <select class="select select-bordered w-full" bind:value={ePolicy}>
            <option value="all_or_nothing">All or nothing</option>
            <option value="weighted">Weighted</option>
          </select>
          <input type="datetime-local" class="input input-bordered w-full sm:col-span-2" bind:value={eDeadline} required>
          <label class="flex items-center gap-2 sm:col-span-2">
            <input type="checkbox" class="checkbox" bind:checked={eShowTraceback}>
            <span class="label-text">Show traceback to students</span>
          </label>
        </div>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-2">
            <input type="file" class="file-input file-input-bordered" on:change={e=>templateFile=(e.target as HTMLInputElement).files?.[0] || null}>
            <button class="btn" on:click={uploadTemplate} disabled={!templateFile}>Upload template</button>
            {#if assignment.template_path}
              <button class="btn btn-ghost" on:click|preventDefault={downloadTemplate}>Download current</button>
            {/if}
          </div>
          <div class="card-actions">
            <button class="btn" on:click={()=>editing=false}>Cancel</button>
            <button class="btn btn-primary" on:click={saveEdit}>Save changes</button>
          </div>
        </div>
      </div>
    </div>
  {:else}
    <!-- Hero header -->
    <section class="relative overflow-hidden mb-6 rounded-2xl border border-base-300/60 bg-gradient-to-br from-primary/10 to-secondary/10 p-0">
      <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-0 sm:gap-6">
        <div class="flex-1 p-6">
          <div class="flex items-center justify-between gap-3">
            <h1 class="text-2xl sm:text-3xl font-semibold tracking-tight">{assignment.title}</h1>
            {#if role==='student'}
              <div class="hidden sm:flex items-center gap-3">
                <div class="radial-progress text-primary" style="--value:{testsPercent};" aria-valuenow={testsPercent} role="progressbar">{testsPercent}%</div>
                <span class="font-semibold">{testsPassed} / {results.length} tests</span>
              </div>
            {/if}
          </div>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <span class={`badge ${deadlineBadgeClass}`}>{deadlineLabel}</span>
            <span class="badge badge-ghost">Max {assignment.max_points} pts</span>
            <span class="badge badge-ghost">{policyLabel(assignment.grading_policy)}</span>
            {#if role!=='student'}
              {#if assignment.published}
                <span class="badge badge-success">Published</span>
              {:else}
                <span class="badge badge-warning">Draft</span>
              {/if}
            {/if}
            {#if done}
              <span class="badge badge-success">Completed</span>
            {/if}
          </div>
          <div class="mt-4 flex flex-wrap items-center gap-2">
            {#if assignment.template_path}
              <a class="btn btn-sm btn-ghost" href={`/api/assignments/${id}/template`} on:click|preventDefault={downloadTemplate}>Download template</a>
            {/if}
            {#if role==='teacher' || role==='admin'}
              {#if !assignment.published}
                <button class="btn btn-sm btn-secondary" on:click={publish}>Publish</button>
              {/if}
              <button class="btn btn-sm" on:click={openTestsModal}>Manage tests</button>
              <button class="btn btn-sm" on:click={startEdit}>Edit</button>
              <button class="btn btn-sm btn-error" on:click={delAssignment}>Delete</button>
            {:else}
              <button class="btn btn-sm btn-primary" on:click={openSubmitModal}>Submit solution</button>
            {/if}
          </div>
        </div>
        {#if role==='student'}
          <div class="sm:border-l border-base-300/60 p-6 flex items-center justify-center">
            <div class="flex items-center gap-3">
              <div class="radial-progress text-primary" style="--value:{percent}; --size:6rem; --thickness:10px" aria-valuenow={percent} role="progressbar">{percent}%</div>
              <div>
                <div class="text-xl font-semibold">{pointsEarned} / {assignment.max_points}</div>
                <div class="text-sm opacity-70">points earned</div>
              </div>
            </div>
          </div>
        {/if}
      </div>
    </section>
    {#if deadlineSoon}
      <div class="alert alert-warning mb-4">
        <span>The deadline is near!</span>
      </div>
    {/if}

    <!-- Content with tabs and optional sidebar for students -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
      <div class="lg:col-span-8">
        <div class="tabs tabs-boxed w-full mb-4">
          <button class={`tab ${activeTab==='overview' ? 'tab-active' : ''}`} on:click={() => setTab('overview')}>Overview</button>
          {#if role==='student'}
            <button class={`tab ${activeTab==='submissions' ? 'tab-active' : ''}`} on:click={() => setTab('submissions')}>Submissions</button>
            <button class={`tab ${activeTab==='results' ? 'tab-active' : ''}`} on:click={() => setTab('results')}>Results</button>
          {/if}
          {#if role==='teacher' || role==='admin'}
          <button class={`tab ${activeTab==='instructor' ? 'tab-active' : ''}`} on:click={() => setTab('instructor')}>Instructor</button>
          <button class={`tab ${activeTab==='teacher-runs' ? 'tab-active' : ''}`} on:click={() => setTab('teacher-runs')}>Teacher runs</button>
          {/if}
        </div>

        {#if activeTab==='overview'}
          <article class="card-elevated p-6 space-y-4">
            <div class="markdown">{@html safeDesc}</div>
            <div class="grid sm:grid-cols-3 gap-3">
              <div class="stat bg-base-100 rounded-xl border border-base-300/60">
                <div class="stat-title">Deadline</div>
                <div class="stat-value text-lg">{formatDateTime(assignment.deadline)}</div>
                <div class="stat-desc">{relativeToDeadline(assignment.deadline)}</div>
              </div>
              <div class="stat bg-base-100 rounded-xl border border-base-300/60">
                <div class="stat-title">Max points</div>
                <div class="stat-value text-lg">{assignment.max_points}</div>
                <div class="stat-desc">{policyLabel(assignment.grading_policy)}</div>
              </div>
              {#if role!=='student'}
                <div class="stat bg-base-100 rounded-xl border border-base-300/60">
                  <div class="stat-title">Status</div>
                  <div class="stat-value text-lg">{assignment.published ? 'Published' : 'Draft'}</div>
                  <div class="stat-desc">Assignment visibility</div>
                </div>
              {/if}
            </div>
          </article>
        {/if}

        {#if activeTab==='submissions' && role==='student'}
          <section class="card-elevated p-6 space-y-3">
            <div class="flex items-center justify-between">
              <h3 class="font-semibold text-lg">Your submissions</h3>
              <button class="btn btn-sm" on:click={openSubmitModal}>New submission</button>
            </div>
            <div class="overflow-x-auto">
              <table class="table table-zebra">
                <thead>
                  <tr><th>Date</th><th>Status</th><th></th></tr>
                </thead>
                <tbody>
                  {#each submissions as s}
                    <tr>
                      <td>{formatDateTime(s.created_at)}</td>
                      <td><span class={`badge ${statusColor(s.status)}`}>{s.status}</span></td>
                      <td><a href={`/submissions/${s.id}?fromTab=${activeTab}`} class="btn btn-sm btn-outline" on:click={saveState}>View</a></td>
                    </tr>
                  {/each}
                  {#if !submissions.length}
                    <tr><td colspan="3"><i>No submissions yet</i></td></tr>
                  {/if}
                </tbody>
              </table>
            </div>
          </section>
        {/if}

        {#if activeTab==='results' && role==='student'}
          <section class="card-elevated p-6 space-y-3">
            <h3 class="font-semibold text-lg">Latest results</h3>
            {#if latestSub}
              <div class="flex items-center gap-2">
                <span>Submission:</span>
                <a class="link" href={`/submissions/${latestSub.id}?fromTab=${activeTab}`} on:click={saveState}>{formatDateTime(latestSub.created_at)}</a>
                <span class={`badge ${statusColor(latestSub.status)}`}>{latestSub.status}</span>
              </div>
              <div class="overflow-x-auto mt-2">
                <table class="table table-zebra">
                  <thead>
                    <tr><th>#</th><th>Status</th><th>Runtime (ms)</th><th>Exit</th><th>Traceback</th></tr>
                  </thead>
                  <tbody>
                    {#each results as r, i}
                      <tr>
                        <td>{i+1}</td>
                        <td><span class={`badge ${statusColor(r.status)}`}>{r.status}</span></td>
                        <td>{r.runtime_ms}</td>
                        <td>{r.exit_code}</td>
                        <td><pre class="whitespace-pre-wrap max-w-xs overflow-x-auto">{r.stderr}</pre></td>
                      </tr>
                    {/each}
                    {#if !results.length}
                      <tr><td colspan="5"><i>No results yet</i></td></tr>
                    {/if}
                  </tbody>
                </table>
              </div>
            {:else}
              <div class="alert">
                <span>No submission yet. Submit your solution to see results.</span>
              </div>
            {/if}
          </section>
        {/if}

        {#if activeTab==='instructor' && (role==='teacher' || role==='admin')}
          <section class="space-y-4">
            <div class="card-elevated p-6">
              <div class="flex items-center justify-between">
                <h3 class="font-semibold text-lg">Student progress</h3>
                <button class="btn btn-sm" on:click={openTestsModal}>Manage tests</button>
              </div>
              <div class="overflow-x-auto mt-3">
                <table class="table table-zebra">
                  <thead>
                    <tr><th>Student</th><th>Status</th><th>Last submission</th></tr>
                  </thead>
                  <tbody>
                    {#each progress as p (p.student.id)}
                      <tr class="cursor-pointer" on:click={() => toggleStudent(p.student.id)}>
                        <td>{p.student.name ?? p.student.email}</td>
                        <td><span class={`badge ${statusColor(p.latest ? p.latest.status : 'none')}`}>{p.latest ? p.latest.status : 'none'}</span></td>
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
                                      <a class="link" href={`/submissions/${s.id}?fromTab=${activeTab}`} on:click={saveState}>{formatDateTime(s.created_at)}</a>
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
            </div>

          </section>
        {/if}

        {#if activeTab==='teacher-runs' && (role==='teacher' || role==='admin')}
          <section class="card-elevated p-6 space-y-3">
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
                      <td><a class="btn btn-sm btn-outline" href={`/submissions/${s.id}?fromTab=${activeTab}`} on:click={saveState}>View</a></td>
                    </tr>
                  {/each}
                  {#if !teacherRuns.length}
                    <tr><td colspan="4"><i>No runs yet</i></td></tr>
                  {/if}
                </tbody>
              </table>
            </div>
          </section>
        {/if}
      </div>

      {#if role==='student'}
        <aside class="lg:col-span-4 lg:sticky lg:top-24 h-fit space-y-4">
          <div class="card-elevated p-5 space-y-3">
            <h3 class="font-semibold">Quick actions</h3>
            <button class="btn btn-primary w-full" on:click={openSubmitModal}>Submit solution</button>
            {#if assignment.template_path}
              <div class="divider my-1"></div>
              <div class="text-sm opacity-70">Need a starting point?</div>
              <button class="btn btn-ghost btn-sm" on:click|preventDefault={downloadTemplate}>Download template</button>
            {/if}
          </div>
          {#if latestSub}
            <div class="card-elevated p-5 space-y-2">
              <h3 class="font-semibold">Latest submission</h3>
              <div class="flex items-center gap-2">
                <span class={`badge ${statusColor(latestSub.status)}`}>{latestSub.status}</span>
                <a class="link" href={`/submissions/${latestSub.id}?fromTab=${activeTab}`} on:click={saveState}>{formatDateTime(latestSub.created_at)}</a>
              </div>
            </div>
          {/if}
        </aside>
      {/if}
    </div>
  {/if}

  <!-- tests list moved to modal -->

  <dialog bind:this={submitDialog} class="modal">
    <div class="modal-box w-11/12 max-w-lg space-y-4">
      <h3 class="font-bold text-lg">Submit solution</h3>
      <div
        role="group"
        aria-label="Upload dropzone"
        class={`border-2 border-dashed rounded-xl p-6 text-center transition ${isDragging ? 'bg-base-200' : 'bg-base-100'}`}
        on:dragover|preventDefault={() => isDragging = true}
        on:dragleave={() => isDragging = false}
        on:drop|preventDefault={(e)=>{ isDragging=false; const dt=(e as DragEvent).dataTransfer; if(dt){ files=[...files, ...Array.from(dt.files)].filter(f=>f.name.endsWith('.py')) } }}
      >
        <div class="text-sm opacity-70 mb-2">Drag and drop your .py files here</div>
        <div class="mb-3">or</div>
        <input type="file" accept=".py" multiple class="file-input file-input-bordered w-full"
          on:change={e=>files=Array.from((e.target as HTMLInputElement).files||[])}>
      </div>
      {#if files.length}
        <div class="text-sm opacity-70">{files.length} file{files.length===1?'':'s'} selected</div>
      {/if}
      <div class="modal-action">
        <button class="btn" on:click={submit} disabled={!files.length}>Upload</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>

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

  

  {#if err}
    <div class="alert alert-error mt-4"><span>{err}</span></div>
  {/if}
{/if}

<style>
pre{display:inline;margin:0 0.5rem;padding:0.2rem;background:#eee}
</style>

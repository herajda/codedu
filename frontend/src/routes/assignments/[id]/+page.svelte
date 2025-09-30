<script lang="ts">
  // @ts-nocheck
import { onMount, onDestroy, tick } from 'svelte'
  import { auth } from '$lib/auth'
import { apiFetch, apiJSON } from '$lib/api'
import { MarkdownEditor } from '$lib'
import { marked } from 'marked'
import { formatDateTime } from "$lib/date";
import DOMPurify from 'dompurify'
import { goto } from '$app/navigation'
import { page } from '$app/stores'
import ConfirmModal from '$lib/components/ConfirmModal.svelte'
import { DeadlinePicker } from '$lib'



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
  let overrides:any[]=[]   // per-student deadline overrides (teacher view)
  let overrideMap: Record<string, any> = {}
  let expanded:number|null=null
  let pointsEarned=0
  let done=false
  let percent=0
  let testsPassed=0
  let testsPercent=0
let testsCount=0
let err=''
let subStats: Record<string, {passed:number, total:number}> = {}
// removed test creation inputs (moved to tests page)
let files: File[] = []
let templateFile:File|null=null

let confirmModal: InstanceType<typeof ConfirmModal>
  // removed unittest file input (moved to tests page)
  let submitDialog: HTMLDialogElement;
  // removed tests dialog (moved to tests page)
$: percent = assignment ? Math.round(pointsEarned / assignment.max_points * 100) : 0;
$: testsPassed = results.filter((r:any) => r.status === 'passed').length;
$: testsPercent = results.length ? Math.round(testsPassed / results.length * 100) : 0;
  let editing=false
  let eTitle='', eDesc='', eDeadline='', ePoints=0, ePolicy='all_or_nothing', eShowTraceback=false, eShowTestDetails=false
  let eManualReview=false
  let eLLMInteractive=false
  let eLLMFeedback=false
  let eLLMAutoAward=true
  let eLLMScenarios=''
  let eLLMStrictness:number=50
  let eLLMRubric=''
  let eSecondDeadline=''
  // Enhanced date/time UX state (derived from the above strings)
  let eDeadlineDate=''
  let eDeadlineTime=''
  let eSecondDeadlineDate=''
  let eSecondDeadlineTime=''
  const quickTimes = ['08:00','12:00','17:00','23:59']
  function timeLabel(t:string){
    // Always show 24h format HH:mm in UI labels
    const [hh, mm] = (t || '').split(':')
    const h = String(parseInt(hh||'0', 10)).padStart(2,'0')
    const m = String(parseInt(mm||'0', 10)).padStart(2,'0')
    return `${h}:${m}`
  }
  let eLatePenaltyRatio:number=0.5
  let showAdvancedOptions=false
  let showAiOptions=false
  let showRubric=false
  const exampleScenario = '[{"name":"calc","steps":[{"send":"2 + 2","expect_after":"4"}]}]'
  let safeDesc=''
$: safeDesc = assignment ? DOMPurify.sanitize(marked.parse(assignment.description) as string) : ''

  // Testing model selector (automatic | manual | ai)
  type TestMode = 'automatic' | 'manual' | 'ai'
  let testMode: TestMode = 'automatic'
  $: {
    if (testMode === 'manual') { eManualReview = true; eLLMInteractive = false }
    else if (testMode === 'ai') { eManualReview = false; eLLMInteractive = true }
    else { eManualReview = false; eLLMInteractive = false }
  }
  
  // Auto-switch to automatic testing when weighted policy is selected
  $: {
    if (ePolicy === 'weighted' && testMode !== 'automatic') {
      testMode = 'automatic'
      eManualReview = false
      eLLMInteractive = false
    }
  }

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
  $: isSecondDeadlineActive = assignment?.second_deadline ? new Date(assignment.second_deadline) > new Date() : false
  $: timeUntilDeadline = assignment ? new Date(assignment.deadline).getTime() - Date.now() : 0
  $: timeUntilSecondDeadline = assignment?.second_deadline ? new Date(assignment.second_deadline).getTime() - Date.now() : 0
  $: deadlineSoon = timeUntilDeadline > 0 && timeUntilDeadline <= 24 * 60 * 60 * 1000
  $: secondDeadlineSoon = timeUntilSecondDeadline > 0 && timeUntilSecondDeadline <= 24 * 60 * 60 * 1000
  $: deadlineBadgeClass = isOverdue && !(role==='student' && done) ? 'badge-error' : 'badge-ghost'
  $: deadlineLabel = assignment ? (
      isOverdue
        ? (role==='student' && done
            ? `Deadline passed ${relativeToDeadline(assignment.deadline)}`
            : `Due ${relativeToDeadline(assignment.deadline)}`
          )
        : `Due ${relativeToDeadline(assignment.deadline)}`
    ) : ''
  $: secondDeadlineLabel = assignment?.second_deadline ? (
      new Date(assignment.second_deadline) < new Date()
        ? `Second deadline passed ${relativeToDeadline(assignment.second_deadline)}`
        : `Second deadline: ${relativeToDeadline(assignment.second_deadline)}`
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
        // test count comes from tests_count for students
        testsCount = (typeof data.tests_count === 'number' ? data.tests_count : (Array.isArray((data as any).tests) ? (data as any).tests.length : 0)) || 0
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
        subStats = {}
        await loadSubmissionStats(submissions, true)
      } else {
        allSubs = data.submissions ?? []
        teacherRuns = data.teacher_runs ?? []
        // for non-students, tests array is present
        try { testsCount = Array.isArray((data as any).tests) ? (data as any).tests.length : 0 } catch { testsCount = 0 }
        subStats = {}
        const cls = await apiJSON(`/api/classes/${assignment.class_id}`)
        students = cls.students ?? []
        // Fetch current extensions
        try {
          overrides = await apiJSON(`/api/assignments/${id}/extensions`)
        } catch {}
        overrideMap = {}
        for (const o of overrides) overrideMap[o.student_id] = o
        progress = students.map((s:any)=>{
          const subs = allSubs.filter((x:any)=>x.student_id===s.id)
          const latest = subs[0]
          const hasCompleted = subs.some((x:any)=>x.status==='completed' || x.status==='passed')
          const displayStatus = hasCompleted ? 'completed' : (latest ? latest.status : 'none')
          return {student:s, latest, all: subs, displayStatus}
        })
      }
    }catch(e:any){ err=e.message }
  }

  async function loadSubmissionStats(list?: any[], reset=false){
    try{
      const source = Array.isArray(list) && list.length ? list : submissions
      if(reset) subStats = {}
      if(testsCount>0 && Array.isArray(source) && source.length){
        const targets = reset ? source : source.filter((s:any)=>!subStats[s.id])
        if(!targets.length) return
        const pairs = await Promise.all(targets.map(async (s:any)=>{
          try{
            const subData = await apiJSON(`/api/submissions/${s.id}`)
            const res = subData.results ?? []
            const passed = Array.isArray(res) ? res.filter((r:any)=>r.status==='passed').length : 0
            return [s.id, {passed, total: res.length}] as const
          }catch{
            return [s.id, {passed: 0, total: 0}] as const
          }
        }))
        const next: Record<string, {passed:number, total:number}> = reset ? {} : {...subStats}
        for(const [sid, st] of pairs){ next[sid]=st }
        subStats = next
      }
    }catch{}
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


  onDestroy(() => {
    esCtrl?.close();
    if (typeof window !== 'undefined') {
      window.removeEventListener('beforeunload', saveState);
    }
  });

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
    // split deadline to date/time for nicer UI
    eDeadlineDate = eDeadline ? eDeadline.slice(0,10) : ''
    eDeadlineTime = eDeadline ? eDeadline.slice(11,16) : ''
    ePoints=assignment.max_points
    ePolicy=assignment.grading_policy
    eShowTraceback=assignment.show_traceback
    eShowTestDetails=!!assignment.show_test_details
    eManualReview=assignment.manual_review
    eLLMInteractive=!!assignment.llm_interactive
    eLLMFeedback=!!assignment.llm_feedback
    eLLMAutoAward=assignment.llm_auto_award ?? true
    eLLMScenarios=assignment.llm_scenarios_json ?? ''
    eLLMStrictness = typeof assignment.llm_strictness === 'number' ? assignment.llm_strictness : 50
    eLLMRubric = assignment.llm_rubric ?? ''
    eSecondDeadline = assignment.second_deadline ? assignment.second_deadline.slice(0,16) : ''
    eSecondDeadlineDate = eSecondDeadline ? eSecondDeadline.slice(0,10) : ''
    eSecondDeadlineTime = eSecondDeadline ? eSecondDeadline.slice(11,16) : ''
    eLatePenaltyRatio = assignment.late_penalty_ratio ?? 0.5
    showAdvancedOptions = !!assignment.second_deadline
    if (assignment.manual_review) testMode = 'manual'
    else if (assignment.llm_interactive) testMode = 'ai'
    else testMode = 'automatic'
  }

  // keep combined strings in sync with split date/time inputs
  $: {
    if(eDeadlineDate && eDeadlineTime) eDeadline = `${eDeadlineDate}T${eDeadlineTime}`
    else if (!eDeadlineDate) eDeadline = ''
  }
  $: { if (eDeadlineDate && !eDeadlineTime) eDeadlineTime = '23:59' }
  $: {
    if(showAdvancedOptions){
      if(eSecondDeadlineDate && eSecondDeadlineTime) eSecondDeadline = `${eSecondDeadlineDate}T${eSecondDeadlineTime}`
      else if(!eSecondDeadlineDate) eSecondDeadline = ''
    } else {
      eSecondDeadline = ''
    }
  }
  $: { if (eSecondDeadlineDate && !eSecondDeadlineTime) eSecondDeadlineTime = '23:59' }

  async function saveEdit(){
    try{
      if(new Date(eDeadline)<new Date()){
        const proceed = await confirmModal.open({
          title: 'Deadline is in the past',
          body: 'Students will immediately see this assignment as overdue. Continue anyway?',
          confirmLabel: 'Continue',
          cancelLabel: 'Go back',
          confirmClass: 'btn btn-warning',
          cancelClass: 'btn'
        })
        if(!proceed) return
      }
      if(eSecondDeadline && new Date(eSecondDeadline)<=new Date(eDeadline)){
        const proceed = await confirmModal.open({
          title: 'Second deadline must follow the first',
          body: 'The late deadline should be later than the main deadline. Continue with this timing anyway?',
          confirmLabel: 'Continue',
          cancelLabel: 'Go back',
          confirmClass: 'btn btn-warning',
          cancelClass: 'btn'
        })
        if(!proceed) return
      }
      // For weighted assignments, max_points is calculated from test weights
      const maxPoints = ePolicy === 'weighted' ? (assignment.max_points || 100) : Number(ePoints)
      
      await apiFetch(`/api/assignments/${id}`,{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({
          title:eTitle,
          description:eDesc,
          deadline:new Date(eDeadline).toISOString(),
          max_points:maxPoints,
          grading_policy:ePolicy,
          show_traceback:eShowTraceback,
          show_test_details:eShowTestDetails,
          manual_review:eManualReview,
          llm_interactive:eLLMInteractive,
          llm_feedback:eLLMFeedback,
          llm_auto_award:eLLMAutoAward,
          llm_scenarios_json:eLLMScenarios.trim() ? eLLMScenarios : null,
          llm_strictness: Number.isFinite(eLLMStrictness) ? Math.min(100, Math.max(0, Number(eLLMStrictness))) : 50,
          llm_rubric: eLLMRubric.trim() ? eLLMRubric : null,
          second_deadline: eSecondDeadline.trim() ? new Date(eSecondDeadline).toISOString() : null,
          late_penalty_ratio: Number.isFinite(eLatePenaltyRatio) ? Math.min(1, Math.max(0, Number(eLatePenaltyRatio))) : 0.5
        })
      })
      editing=false
      await load()
    }catch(e:any){ err=e.message }
  }

  async function delAssignment(){
    const confirmed = await confirmModal.open({
      title: 'Delete assignment',
      body: 'This assignment and all related submissions will be permanently removed.',
      confirmLabel: 'Delete assignment',
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    })
    if(!confirmed) return
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

  async function toggleStudent(id:number){
    const next = expanded===id ? null : id
    expanded = next
    saveState()
    if(next!==null){
      const entry = progress.find((p:any)=>p.student.id===next)
      if(entry){
        await loadSubmissionStats(entry.all)
      }
    }
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
      allowed.push('submissions')
      if (!assignment?.manual_review) allowed.push('results')
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

  // Extension dialog state (teacher)
  let extendDialog: HTMLDialogElement
  let extStudent: any = null
  let extDeadline = ''
  let extDeadlineDate = ''
  let extDeadlineTime = ''
  let extNote = ''
  function openExtendDialog(student: any){
    extStudent = student
    const cur = overrideMap[student.id]
    extDeadline = cur ? String(cur.new_deadline).slice(0,16) : (assignment.deadline?.slice(0,16) || '')
    extDeadlineDate = extDeadline ? extDeadline.slice(0,10) : ''
    extDeadlineTime = extDeadline ? extDeadline.slice(11,16) : ''
    extNote = cur?.note || ''
    extendDialog.showModal()
  }
  async function saveExtension(){
    if(!extStudent || !extDeadline) return
    try{
      await apiFetch(`/api/assignments/${id}/extensions/${extStudent.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ new_deadline: new Date(extDeadline).toISOString(), note: extNote.trim() ? extNote : null })
      })
      extendDialog.close()
      await load()
    }catch(e:any){ err = e.message }
  }
  async function clearExtension(){
    if(!extStudent) return
    try{
      await apiFetch(`/api/assignments/${id}/extensions/${extStudent.id}`, { method: 'DELETE' })
      extendDialog.close()
      await load()
    }catch(e:any){ err = e.message }
  }

  // keep combined extension string synced with date/time parts
  $: {
    if(extDeadlineDate && extDeadlineTime) extDeadline = `${extDeadlineDate}T${extDeadlineTime}`
    else if(!extDeadlineDate) extDeadline = ''
  }
  $: { if (extDeadlineDate && !extDeadlineTime) extDeadlineTime = '23:59' }

  // ───────────────────────────
  // Deadline picker modal (reusable)
  // ───────────────────────────
  let deadlinePicker: InstanceType<typeof DeadlinePicker>;
  function euLabelFromParts(d: string, t: string): string {
    if (!d || !t) return '';
    // d: yyyy-mm-dd
    const day = d.slice(8,10); const mon = d.slice(5,7); const y = d.slice(0,4);
    return `${day}/${mon}/${y} ${t}`;
  }
  async function pickMainDeadline(){
    const initial = eDeadlineDate && eDeadlineTime ? `${eDeadlineDate}T${eDeadlineTime}` : (assignment?.deadline ?? null);
    const picked = await deadlinePicker.open({ title: 'Select main deadline', initial });
    if (picked) { eDeadlineDate = picked.slice(0,10); eDeadlineTime = picked.slice(11,16); }
  }
  async function pickSecondDeadline(){
    const initial = eSecondDeadlineDate && eSecondDeadlineTime ? `${eSecondDeadlineDate}T${eSecondDeadlineTime}` : (assignment?.second_deadline ?? null);
    const picked = await deadlinePicker.open({ title: 'Select second deadline', initial });
    if (picked) { eSecondDeadlineDate = picked.slice(0,10); eSecondDeadlineTime = picked.slice(11,16); }
  }
  async function pickExtensionDeadline(){
    const initial = extDeadlineDate && extDeadlineTime ? `${extDeadlineDate}T${extDeadlineTime}` : (assignment?.deadline ?? null);
    const picked = await deadlinePicker.open({ title: 'Select new deadline', initial });
    if (picked) { extDeadlineDate = picked.slice(0,10); extDeadlineTime = picked.slice(11,16); }
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
      <div class="card-body p-6">
        <div class="flex items-center justify-between mb-2">
          <h1 class="card-title text-2xl">Edit assignment</h1>
          <div class="badge badge-outline">ID #{assignment.id}</div>
        </div>

        <div class="grid lg:grid-cols-3 gap-6">
          <div class="lg:col-span-2 space-y-4">
            <!-- Basic info -->
            <section class="rounded-xl border border-base-300/60 bg-base-100 p-5 space-y-3">
              <h3 class="font-semibold">Basic info</h3>
              <input class="input input-bordered w-full" bind:value={eTitle} placeholder="Title" required>
              <MarkdownEditor bind:value={eDesc} placeholder="Description" />
              <div class="grid gap-3" class:sm:grid-cols-2={ePolicy === 'all_or_nothing'}>
                <div class="form-control">
                  <label class="label" for="grading-policy-select"><span class="label-text">Grading policy</span></label>
                  <select id="grading-policy-select" class="select select-bordered w-full" bind:value={ePolicy}>
                    <option value="all_or_nothing">All or nothing</option>
                    <option value="weighted" disabled={testMode === 'manual' || testMode === 'ai'}>Weighted</option>
                  </select>
                  {#if testMode === 'manual' || testMode === 'ai'}
                    <div class="label-text-alt text-warning">Switch to automatic testing to use weighted grading</div>
                  {/if}
                </div>
                {#if ePolicy === 'all_or_nothing'}
                  <div class="form-control">
                    <label class="label" for="max-points-input"><span class="label-text">Max points</span></label>
                    <input id="max-points-input" type="number" min="1" class="input input-bordered w-full" bind:value={ePoints} placeholder="Max points" required>
                  </div>
                {/if}
              </div>
              <div class="text-xs opacity-70 mt-2">
                <strong>All or nothing:</strong> Students get full points only if all tests pass. <strong>Weighted:</strong> Points are calculated from individual test weights set in the Tests section.
              </div>
            </section>

            <!-- Deadlines -->
            <section class="rounded-xl border border-base-300/60 bg-base-100 p-5 space-y-3">
              <div class="flex items-center justify-between">
                <h3 class="font-semibold">Deadlines</h3>
                <label class="flex items-center gap-2">
                  <input type="checkbox" class="toggle" bind:checked={showAdvancedOptions}>
                  <span class="text-sm">Enable second deadline</span>
                </label>
              </div>
              <div class="grid sm:grid-cols-2 gap-3">
                <!-- Main deadline: open picker modal -->
                <div class="form-control">
                  <label class="label"><span class="label-text">Main deadline</span></label>
                  <div class="flex items-center gap-2">
                    <input class="input input-bordered w-full" readonly placeholder="dd/mm/yyyy hh:mm" value={euLabelFromParts(eDeadlineDate, eDeadlineTime)}>
                    <button type="button" class="btn" on:click={pickMainDeadline}>Pick</button>
                    {#if eDeadlineDate}
                      <button type="button" class="btn btn-ghost" title="Clear" on:click={() => { eDeadlineDate=''; eDeadlineTime=''; }}>Clear</button>
                    {/if}
                  </div>
                </div>
                {#if showAdvancedOptions}
                  <div class="form-control">
                    <label class="label"><span class="label-text">Second deadline</span></label>
                    <div class="flex items-center gap-2">
                      <input class="input input-bordered w-full" readonly placeholder="dd/mm/yyyy hh:mm" value={euLabelFromParts(eSecondDeadlineDate, eSecondDeadlineTime)}>
                      <button type="button" class="btn" on:click={pickSecondDeadline}>Pick</button>
                      {#if eSecondDeadlineDate}
                        <button type="button" class="btn btn-ghost" title="Clear" on:click={() => { eSecondDeadlineDate=''; eSecondDeadlineTime=''; }}>Clear</button>
                      {/if}
                    </div>
                  </div>
                {/if}
              </div>
              {#if showAdvancedOptions}
                <div class="form-control">
                  <label class="label" for="late-penalty-range">
                    <span class="label-text">Late penalty ratio</span>
                    <span class="label-text-alt">{Math.round(eLatePenaltyRatio * 100)}%</span>
                  </label>
                  <input id="late-penalty-range" type="range" min="0" max="1" step="0.1" class="range range-primary" bind:value={eLatePenaltyRatio} />
                  <div class="w-full flex justify-between text-xs px-2 mt-1">
                    <span>0%</span>
                    <span>100%</span>
                  </div>
                </div>
              {/if}
            </section>

            <!-- Testing and grading -->
            <section class="rounded-xl border border-base-300/60 bg-base-100 p-5 space-y-3">
              <h3 class="font-semibold">Testing and grading</h3>
              <div class="flex flex-wrap items-center gap-3">
                <label class="form-control w-full max-w-xs">
                  <select class="select select-bordered select-sm" bind:value={testMode} disabled={ePolicy === 'weighted'}>
                    <option value="automatic">Automatic tests</option>
                    <option value="manual" disabled={ePolicy === 'weighted'}>Manual teacher review</option>
                    <option value="ai" disabled={ePolicy === 'weighted'}>AI testing (LLM-Interactive)</option>
                  </select>
                </label>
                {#if testMode === 'automatic'}
                  <div class="flex flex-col gap-2">
                    <label class="flex items-center gap-2">
                      <input type="checkbox" class="checkbox" bind:checked={eShowTraceback}>
                      <span class="label-text">Show traceback to students</span>
                    </label>
                    <label class="flex items-center gap-2">
                      <input type="checkbox" class="checkbox" bind:checked={eShowTestDetails}>
                      <span class="label-text">Reveal test definitions in teacher review</span>
                    </label>
                  </div>
                {/if}
              </div>
              <p class="text-xs opacity-70">
                {#if ePolicy === 'weighted'}
                  Weighted assignments require automatic testing to calculate points from test weights.
                {:else if testMode === 'automatic'}
                  Use IO/unittest tests (including AI-generated tests) to grade automatically.
                {:else if testMode === 'manual'}
                  Teacher reviews submissions and assigns points. No automated tests run.
                {:else}
                  Grade using LLM-driven interactive scenarios.
                {/if}
              </p>

              {#if testMode==='ai'}
                <div class="divider my-2"></div>
                <button type="button" class="btn btn-ghost btn-sm" on:click={() => showAiOptions = !showAiOptions}>
                  {showAiOptions ? 'Hide' : 'Show'} AI options
                </button>
                {#if showAiOptions}
                  <div class="mt-2 space-y-3">
                    <div class="grid sm:grid-cols-2 gap-3">
                      <label class="flex items-center gap-2">
                        <input type="checkbox" class="checkbox checkbox-sm" bind:checked={eLLMFeedback}>
                        <span class="label-text">Give AI feedback to students</span>
                      </label>
                      <label class="flex items-center gap-2">
                        <input type="checkbox" class="checkbox checkbox-sm" bind:checked={eLLMAutoAward}>
                        <span class="label-text">Auto-award points from AI</span>
                      </label>
                    </div>
                    <div class="form-control">
                      <label class="label" for="ai-strictness-range">
                        <span class="label-text">Strictness</span>
                        <span class="label-text-alt">{eLLMStrictness}%</span>
                      </label>
                      <input id="ai-strictness-range" type="range" min="0" max="100" step="5" class="range range-primary" bind:value={eLLMStrictness}>
                    </div>

                    <div class="form-control">
                      <button type="button" class="btn btn-ghost btn-sm w-fit" on:click={() => showRubric = !showRubric}>
                        {showRubric ? 'Hide rubric' : 'Add rubric'}
                      </button>
                      {#if showRubric}
                        <textarea class="textarea textarea-bordered min-h-[6rem]" bind:value={eLLMRubric} placeholder="Optional rubric to guide AI scoring"></textarea>
                      {/if}
                    </div>
                  </div>
                {/if}
              {/if}
            </section>

            <!-- Template (collapsible) -->
            <details class="collapse collapse-arrow bg-base-100 border border-base-300/60 rounded-xl">
              <summary class="collapse-title text-base font-medium">Assignment template</summary>
              <div class="collapse-content">
                <div class="flex flex-wrap items-center gap-2">
                  <input type="file" class="file-input file-input-bordered" on:change={e=>templateFile=(e.target as HTMLInputElement).files?.[0] || null}>
                  <button class="btn" on:click={uploadTemplate} disabled={!templateFile}>Upload template</button>
                  {#if assignment.template_path}
                    <button class="btn btn-ghost" on:click|preventDefault={downloadTemplate}>Download current</button>
                  {/if}
                </div>
              </div>
            </details>
          </div>

          <!-- Sticky actions / summary -->
          <aside class="lg:col-span-1">
            <div class="rounded-xl border border-base-300/60 bg-base-100 p-5 lg:sticky lg:top-24 space-y-4">
              <h3 class="font-semibold">Actions</h3>
              <div class="space-y-2 text-sm opacity-70">
                <div>Policy: <span class="font-semibold">{policyLabel(ePolicy)}</span></div>
                {#if ePolicy === 'all_or_nothing'}
                  <div>Max points: <span class="font-semibold">{ePoints}</span></div>
                {:else}
                  <div>Max points: <span class="font-semibold text-base-content/50">From test weights</span></div>
                {/if}
                <div>Deadline: <span class="font-semibold">{eDeadline || '-'}</span></div>
                {#if showAdvancedOptions}
                  <div>2nd deadline: <span class="font-semibold">{eSecondDeadline || '-'}</span></div>
                  <div>Late penalty: <span class="font-semibold">{Math.round(eLatePenaltyRatio * 100)}%</span></div>
                {/if}
                {#if testMode==='ai'}
                  <div>AI strictness: <span class="font-semibold">{eLLMStrictness}%</span></div>
                {/if}
              </div>
              <div class="card-actions">
                <button class="btn w-full" on:click={()=>editing=false}>Cancel</button>
                <button class="btn btn-primary w-full" on:click={saveEdit}>Save changes</button>
              </div>
            </div>
          </aside>
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
            {#if role==='student' && !assignment.manual_review && testsCount > 0}
              <div class="hidden sm:flex items-center gap-3">
                <div class="radial-progress text-primary" style="--value:{testsPercent};" aria-valuenow={testsPercent} role="progressbar">{testsPercent}%</div>
                <span class="font-semibold">{testsPassed} / {results.length} tests</span>
              </div>
            {/if}
          </div>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <span class={`badge ${deadlineBadgeClass}`}>{deadlineLabel}</span>
            {#if assignment.second_deadline}
              <span class="badge badge-warning">{secondDeadlineLabel}</span>
            {/if}
            <span class="badge badge-ghost">Max {assignment.max_points} pts</span>
            <span class="badge badge-ghost">{policyLabel(assignment.grading_policy)}</span>
            {#if assignment.manual_review}
              <span class="badge badge-info">Manual review</span>
            {/if}
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
              {#if !assignment.manual_review}
                <button class="btn btn-sm" on:click={openTestsModal}>Manage tests</button>
              {/if}
              <button class="btn btn-sm" on:click={startEdit}>Edit</button>
              <button class="btn btn-sm btn-error" on:click={delAssignment}>Delete</button>
            {:else}
              <button class="btn btn-sm btn-primary" on:click={openSubmitModal} disabled={assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}>Submit solution</button>
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
    {#if role==='student' && assignment.manual_review}
      <div class="alert alert-info mb-4">
        <span>This assignment is graded by teacher review. Automatic tests will not run; points will appear after review.</span>
      </div>
    {/if}
    {#if deadlineSoon}
      <div class="alert alert-warning mb-4">
        <span>The deadline is near!</span>
      </div>
    {/if}
    {#if secondDeadlineSoon}
      <div class="alert alert-warning mb-4">
        <span>The second deadline is near! Submissions after the first deadline will receive {Math.round(assignment.late_penalty_ratio * 100)}% of points.</span>
      </div>
    {/if}

    <!-- Content with tabs and optional sidebar for students -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
      <div class="lg:col-span-8">
        <div class="tabs tabs-boxed w-full mb-4">
          <button class={`tab ${activeTab==='overview' ? 'tab-active' : ''}`} on:click={() => setTab('overview')}>Overview</button>
          {#if role==='student'}
            <button class={`tab ${activeTab==='submissions' ? 'tab-active' : ''}`} on:click={() => setTab('submissions')}>Submissions</button>
            {#if !assignment.manual_review}
              <button class={`tab ${activeTab==='results' ? 'tab-active' : ''}`} on:click={() => setTab('results')}>Results</button>
            {/if}
          {/if}
          {#if role==='teacher' || role==='admin'}
          <button class={`tab ${activeTab==='instructor' ? 'tab-active' : ''}`} on:click={() => setTab('instructor')}>Student progress</button>
          <button class={`tab ${activeTab==='teacher-runs' ? 'tab-active' : ''}`} on:click={() => setTab('teacher-runs')}>Teacher runs</button>
          {/if}
        </div>

        {#if activeTab==='overview'}
          <article class="card-elevated p-6 space-y-4">
            <div class="markdown assignment-description">{@html safeDesc}</div>
            {#if role==='student' && assignment.second_deadline && new Date() > assignment.deadline && new Date() <= assignment.second_deadline}
              <div class="alert alert-warning">
                <span>
                  <strong>Second deadline active!</strong> You can still submit your solution, but you will receive {Math.round(assignment.late_penalty_ratio * 100)}% of the maximum points.
                  <br>Second deadline: {formatDateTime(assignment.second_deadline)}
                </span>
              </div>
            {:else if role==='student' && assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}
              <div class="alert alert-error">
                <span>
                  <strong>All deadlines have passed:</strong> No more submissions are accepted for this assignment.
                </span>
              </div>
            {/if}
            <div class="grid sm:grid-cols-3 gap-3">
              <div class="stat bg-base-100 rounded-xl border border-base-300/60">
                <div class="stat-title">Deadline</div>
                <div class="stat-value text-lg whitespace-normal break-anywhere">{formatDateTime(assignment.deadline)}</div>
                <div class="stat-desc">{relativeToDeadline(assignment.deadline)}</div>
              </div>
              {#if assignment.second_deadline}
                <div class="stat bg-base-100 rounded-xl border border-base-300/60">
                  <div class="stat-title">Second deadline</div>
                  <div class="stat-value text-lg whitespace-normal break-anywhere">{formatDateTime(assignment.second_deadline)}</div>
                  <div class="stat-desc">{relativeToDeadline(assignment.second_deadline)} • {Math.round(assignment.late_penalty_ratio * 100)}% points</div>
                </div>
              {/if}
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
              <button class="btn btn-sm" on:click={openSubmitModal} disabled={assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}>New submission</button>
            </div>
            {#if assignment.second_deadline && new Date() > assignment.deadline && new Date() <= assignment.second_deadline}
              <div class="alert alert-info">
                <span>
                  <strong>Second deadline period:</strong> You can still submit, but submissions made after the first deadline will receive {Math.round(assignment.late_penalty_ratio * 100)}% of points.
                </span>
              </div>
            {:else if assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}
              <div class="alert alert-error">
                <span>
                  <strong>Both deadlines have passed:</strong> No more submissions are accepted for this assignment.
                </span>
              </div>
            {/if}
            <div class="overflow-x-auto">
              <table class="table table-zebra">
                <thead>
                  <tr>
                    <th>Attempt</th>
                    <th>Date</th>
                    <th>Deadline</th>
                    <th>Status</th>
                    {#if testsCount>0}
                      <th>Passed</th>
                      <th>Points</th>
                    {/if}
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {#each submissions as s}
                    <tr>
                      <td>#{s.attempt_number ?? '?'}</td>
                      <td>{formatDateTime(s.created_at)}</td>
                      <td>
                        {#if s.created_at > assignment.deadline}
                          {#if assignment.second_deadline && s.created_at <= assignment.second_deadline}
                            <span class="badge badge-warning badge-sm">Second deadline ({Math.round(assignment.late_penalty_ratio * 100)}%)</span>
                          {:else}
                            <span class="badge badge-error badge-sm">Late (no points)</span>
                          {/if}
                        {:else}
                          <span class="badge badge-success badge-sm">On time</span>
                        {/if}
                      </td>
                      <td><span class={`badge ${statusColor(s.status)}`}>{s.status}</span></td>
                      {#if testsCount>0}
                        <td>{#if subStats[s.id]}{subStats[s.id].passed} / {testsCount}{:else}-{/if}</td>
                        <td>{(s.override_points ?? s.points ?? 0)} {#if s.manually_accepted}<span class="badge badge-xs badge-outline badge-success ml-2" title="Accepted by teacher">accepted</span>{/if}</td>
                      {/if}
                      <td><a href={`/submissions/${s.id}?fromTab=${activeTab}`} class="btn btn-sm btn-outline" on:click={saveState}>View</a></td>
                    </tr>
                  {/each}
                  {#if !submissions.length}
                    <tr><td colspan="{testsCount>0?7:5}"><i>No submissions yet</i></td></tr>
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
                <span class="text-xs opacity-70">Attempt #{latestSub.attempt_number ?? '?'}</span>
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
                {#if !assignment.manual_review}
                  <button class="btn btn-sm" on:click={openTestsModal}>Manage tests</button>
                {/if}
              </div>
              <div class="overflow-x-auto mt-3">
                <table class="table table-zebra">
                  <thead>
                    <tr><th>Student</th><th>Status</th><th>Deadline</th><th>Last submission</th><th class="w-40">Extension</th></tr>
                  </thead>
                  <tbody>
                    {#each progress as p (p.student.id)}
                      <tr class="cursor-pointer" on:click={() => toggleStudent(p.student.id)}>
                        <td>{p.student.name ?? p.student.email}</td>
                        <td><span class={`badge ${statusColor(p.displayStatus)}`}>{p.displayStatus}</span></td>
                        <td>
                          {#if p.latest}
                            {#if p.latest.created_at > assignment.deadline}
                              {#if assignment.second_deadline && p.latest.created_at <= assignment.second_deadline}
                                <span class="badge badge-warning flex-wrap whitespace-normal break-words text-center h-auto py-1 leading-tight">Second deadline ({Math.round(assignment.late_penalty_ratio * 100)}%)</span>
                              {:else}
                                <span class="badge badge-error flex-wrap whitespace-normal break-words text-center h-auto py-1 leading-tight">Late (no points)</span>
                              {/if}
                            {:else}
                              <span class="badge badge-success flex-wrap whitespace-normal break-words text-center h-auto py-1 leading-tight">On time</span>
                            {/if}
                          {:else}
                            <span class="badge badge-ghost flex-wrap whitespace-normal break-words text-center h-auto py-1 leading-tight">No submission</span>
                          {/if}
                        </td>
                        <td>{p.latest ? formatDateTime(p.latest.created_at) : '-'}</td>
                        <td>
                          {#if overrideMap[p.student.id]}
                            <div class="flex items-center gap-2 flex-nowrap">
                              <span class="badge badge-info badge-sm whitespace-nowrap" title={overrideMap[p.student.id].note || ''}>Until {formatDateTime(overrideMap[p.student.id].new_deadline)}</span>
                              <button class="btn btn-ghost btn-xs whitespace-nowrap" on:click|stopPropagation={() => openExtendDialog(p.student)}>Edit</button>
                            </div>
                          {:else}
                            <button class="btn btn-ghost btn-xs whitespace-nowrap" on:click|stopPropagation={() => openExtendDialog(p.student)}>Extend…</button>
                          {/if}
                        </td>
                      </tr>
                      {#if expanded === p.student.id}
                        <tr>
                          <td colspan="5">
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
                                        <span class="mr-2 text-xs opacity-70">Attempt #{s.attempt_number ?? '?'}</span>
                                        <a class="link" href={`/submissions/${s.id}?fromTab=${activeTab}`} on:click={saveState}>{formatDateTime(s.created_at)}</a>
                                        {#if s.created_at > assignment.deadline}
                                          {#if assignment.second_deadline && s.created_at <= assignment.second_deadline}
                                            <span class="badge badge-xs badge-warning" title="Second deadline submission">2nd ({Math.round(assignment.late_penalty_ratio * 100)}%)</span>
                                          {:else}
                                            <span class="badge badge-xs badge-error" title="Late submission">Late</span>
                                          {/if}
                                        {/if}
                                        {#if s.manually_accepted}
                                          <span class="badge badge-xs badge-outline badge-success" title="Accepted by teacher">accepted</span>
                                        {/if}
                                      </div>
                                      <div class="flex flex-wrap items-center gap-2 text-xs">
                                        {#if testsCount>0}
                                          <span class="badge badge-ghost badge-xs">
                                            {#if subStats[s.id]}
                                              {subStats[s.id].passed} / {subStats[s.id].total || testsCount} tests
                                            {:else}
                                              - / - tests
                                            {/if}
                                          </span>
                                        {/if}
                                        <span class="badge badge-outline badge-xs">{(s.override_points ?? s.points ?? 0)} pts</span>
                                      </div>
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
                      <tr><td colspan="4"><i>No students</i></td></tr>
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
            <button class="btn btn-primary w-full" on:click={openSubmitModal} disabled={assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}>Submit solution</button>
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
                <span class="text-xs opacity-70">Attempt #{latestSub.attempt_number ?? '?'}</span>
                <a class="link" href={`/submissions/${latestSub.id}?fromTab=${activeTab}`} on:click={saveState}>{formatDateTime(latestSub.created_at)}</a>
              </div>
              {#if assignment.second_deadline && latestSub.created_at > assignment.deadline && latestSub.created_at <= assignment.second_deadline}
                <div class="alert alert-warning alert-sm">
                  <span>This submission was made after the first deadline but before the second deadline. You will receive {Math.round(assignment.late_penalty_ratio * 100)}% of points.</span>
                </div>
              {/if}
            </div>
          {/if}
          {#if assignment.second_deadline && new Date() > assignment.deadline && new Date() <= assignment.second_deadline}
            <div class="card-elevated p-5 space-y-2">
              <h3 class="font-semibold text-warning">Second deadline active</h3>
              <p class="text-sm">You can still submit, but you will receive {Math.round(assignment.late_penalty_ratio * 100)}% of points.</p>
              <div class="text-xs opacity-70">
                Second deadline: {formatDateTime(assignment.second_deadline)}
              </div>
            </div>
          {:else if assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}
            <div class="card-elevated p-5 space-y-2">
              <h3 class="font-semibold text-error">All deadlines passed</h3>
              <p class="text-sm">No more submissions are accepted for this assignment.</p>
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
      {#if assignment.second_deadline && new Date() > assignment.deadline && new Date() <= assignment.second_deadline}
        <div class="alert alert-warning">
          <span>
            <strong>Second deadline period:</strong> This submission will receive {Math.round(assignment.late_penalty_ratio * 100)}% of the maximum points.
          </span>
        </div>
      {:else if assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}
        <div class="alert alert-error">
          <span>
            <strong>All deadlines have passed:</strong> No more submissions are accepted for this assignment.
          </span>
        </div>
      {/if}
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
        <button class="btn" on:click={submit} disabled={!files.length || (assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline)}>Upload</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>

  <!-- Extend deadline dialog (teacher) -->
  <dialog bind:this={extendDialog} class="modal">
    <div class="modal-box w-11/12 max-w-md space-y-4">
      <h3 class="font-bold text-lg">Extend deadline</h3>
      <div class="form-control">
        <label class="label"><span class="label-text">Student</span></label>
        <div class="input input-bordered">{extStudent?.name ?? extStudent?.email}</div>
      </div>
      <div class="form-control">
        <label class="label"><span class="label-text">New deadline</span></label>
        <div class="flex items-center gap-2">
          <input class="input input-bordered w-full" readonly placeholder="dd/mm/yyyy hh:mm" value={euLabelFromParts(extDeadlineDate, extDeadlineTime)}>
          <button class="btn" on:click|preventDefault={pickExtensionDeadline}>Pick</button>
          {#if extDeadlineDate}
            <button class="btn btn-ghost" on:click|preventDefault={() => { extDeadlineDate=''; extDeadlineTime=''; }}>Clear</button>
          {/if}
        </div>
      </div>
      <div class="form-control">
        <label class="label"><span class="label-text">Note (optional)</span></label>
        <input type="text" class="input input-bordered w-full" placeholder="Reason or context" bind:value={extNote} />
      </div>
      <div class="modal-action">
        {#if overrideMap[extStudent?.id]}
          <button class="btn btn-error btn-outline" on:click={clearExtension}>Clear</button>
        {/if}
        <button class="btn" on:click={saveExtension} disabled={!extStudent || !extDeadline}>Save</button>
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
  <ConfirmModal bind:this={confirmModal} />
  <DeadlinePicker bind:this={deadlinePicker} />
{/if}

<style>
</style>

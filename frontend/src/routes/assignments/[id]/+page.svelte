<script lang="ts">
import { onMount, onDestroy } from 'svelte'
  import { get } from 'svelte/store'
  import { auth } from '$lib/auth'
import { apiFetch, apiJSON } from '$lib/api'
import { MarkdownEditor } from '$lib'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { goto } from '$app/navigation'
import { page } from '$app/stores'



$: id = $page.params.id
const role = get(auth)?.role!;

  let assignment:any=null
  let tests:any[]=[] // teacher/admin only
  let submissions:any[]=[] // student submissions
  let latestSub:any=null
  let results:any[]=[]
  let es:EventSource|null=null
  let allSubs:any[]=[]     // teacher view
  let students:any[]=[]    // class roster for teacher
  let progress:any[]=[]    // computed progress per student
  let pointsEarned=0
  let done=false
  let percent=0
  let err=''
  let tStdin='', tStdout='', tLimit=''
  let files: File[] = []
  let templateFile:File|null=null
  let submitDialog: HTMLDialogElement;
  let testsDialog: HTMLDialogElement;
$: percent = assignment ? Math.round(pointsEarned / assignment.max_points * 100) : 0;
  let editing=false
  let eTitle='', eDesc='', eDeadline='', ePoints=0, ePolicy='all_or_nothing', eShowTraceback=false
  let safeDesc=''
$: safeDesc = assignment ? DOMPurify.sanitize(marked.parse(assignment.description) as string) : ''

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
      if(role==='student') {
        submissions = data.submissions ?? []
        latestSub = submissions[0] ?? null
        results = []
        if(latestSub){
          const subData = await apiJSON(`/api/submissions/${latestSub.id}`)
          results = subData.results ?? []
        }
        const completed = submissions.find((s:any)=>s.status==='completed')
        done = !!completed
        pointsEarned = completed ? assignment.max_points : 0
      } else {
        tests = data.tests ?? []
        allSubs = data.submissions ?? []
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

  onMount(() => {
    load()
    es = new EventSource('/api/events')
    es.addEventListener('status', (ev) => {
      const d = JSON.parse((ev as MessageEvent).data)
      if(latestSub && d.submission_id===latestSub.id){
        latestSub.status = d.status
        if(d.status!== 'running') load()
      }
    })
    es.addEventListener('result', (ev) => {
      const d = JSON.parse((ev as MessageEvent).data)
      if(latestSub && d.submission_id===latestSub.id){
        results = [...results, d]
      }
    })
  })

  onDestroy(()=>{es?.close()})

  async function addTest(){
    try{
      await apiFetch(`/api/assignments/${id}/tests`,{
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({stdin:tStdin, expected_stdout:tStdout, time_limit_sec: parseFloat(tLimit) || undefined})
      })
      tStdin=tStdout=tLimit=''
      await load()
    }catch(e:any){ err=e.message }
  }

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

  async function delTest(tid:number){
    if(!confirm('Delete this test?')) return
    try{
      await apiFetch(`/api/tests/${tid}`,{method:'DELETE'})
      await load()
    }catch(e:any){ err=e.message }
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

  function openTestsModal(){
    testsDialog.showModal()
  }

  async function updateTest(t:any){
    try{
      await apiFetch(`/api/tests/${t.id}`,{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({stdin:t.stdin, expected_stdout:t.expected_stdout, time_limit_sec: parseFloat(t.time_limit_sec) || undefined})
      })
      await load()
    }catch(e:any){ err=e.message }
  }
</script>

{#if !assignment}
  <p>Loading…</p>
{:else}
  {#if editing}
    <div class="card bg-base-100 shadow mb-4">
      <div class="card-body space-y-3">
        <h1 class="card-title">Edit assignment</h1>
        <input class="input input-bordered w-full" bind:value={eTitle} placeholder="Title" required>
        <MarkdownEditor bind:value={eDesc} placeholder="Description" />
        <input type="number" min="1" class="input input-bordered w-full" bind:value={ePoints} placeholder="Max points" required>
        <select class="select select-bordered w-full" bind:value={ePolicy}>
          <option value="all_or_nothing">all_or_nothing</option>
          <option value="percentage">percentage</option>
          <option value="weighted">weighted</option>
        </select>
        <input type="datetime-local" class="input input-bordered w-full" bind:value={eDeadline} required>
        <label class="flex items-center gap-2">
          <input type="checkbox" class="checkbox" bind:checked={eShowTraceback}>
          <span class="label-text">Show traceback to students</span>
        </label>
        <div class="card-actions justify-end">
          <button class="btn btn-primary" on:click={saveEdit}>Save</button>
          <button class="btn" on:click={()=>editing=false}>Cancel</button>
        </div>
      </div>
    </div>
  {:else}
    <div class="card bg-base-100 shadow mb-4">
      <div class="card-body space-y-2">
        <div class="flex justify-between items-start">
          <h1 class="card-title text-2xl">{assignment.title}</h1>
          {#if role==='student'}
            <div class="flex items-center gap-2">
              <div class="radial-progress text-primary" style="--value:{percent};" aria-valuenow={percent} role="progressbar">{percent}%</div>
              <span class="font-semibold">{pointsEarned} / {assignment.max_points} pts</span>
            </div>
          {/if}
        </div>
        <div class="markdown">{@html safeDesc}</div>
        <p><strong>Deadline:</strong> {new Date(assignment.deadline).toLocaleString()}</p>
        <p><strong>Max points:</strong> {assignment.max_points}</p>
        <p><strong>Policy:</strong> {assignment.grading_policy}</p>
        {#if assignment.template_path}
          <a class="link" href={`/api/assignments/${id}/template`} on:click|preventDefault={downloadTemplate}>Download template</a>
        {/if}
        {#if role==='teacher' || role==='admin'}
          <div class="mt-2 space-x-2">
            <input type="file" class="file-input file-input-bordered" on:change={e=>templateFile=(e.target as HTMLInputElement).files?.[0] || null}>
            <button class="btn" on:click={uploadTemplate} disabled={!templateFile}>Upload template</button>
          </div>
        {/if}
        {#if done}
          <span class="badge badge-success">Done</span>
        {/if}
        {#if role==='teacher' || role==='admin'}
          <div class="card-actions justify-end">
            <button class="btn" on:click={openTestsModal}>Manage tests</button>
            <button class="btn" on:click={startEdit}>Edit</button>
            <button class="btn btn-error" on:click={delAssignment}>Delete</button>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  <!-- tests list moved to modal -->

  {#if role==='teacher' || role==='admin'}
    <details class="mb-4">
      <summary class="cursor-pointer font-semibold">Student progress</summary>
      <div class="overflow-x-auto mt-2">
        <table class="table table-zebra">
        <thead>
          <tr><th>Student</th><th>Status</th><th>Last submission</th><th>Attempts</th></tr>
        </thead>
        <tbody>
          {#each progress as p}
            <tr>
              <td>{p.student.name ?? p.student.email}</td>
              <td><span class={`badge ${statusColor(p.latest ? p.latest.status : 'none')}`}>{p.latest ? p.latest.status : 'none'}</span></td>
              <td>{p.latest ? new Date(p.latest.created_at).toLocaleString() : '-'}</td>
              <td>
                {#if p.all && p.all.length}
                  <details>
                    <summary class="cursor-pointer">{p.all.length} attempt{p.all.length === 1 ? '' : 's'}</summary>
                    <ul class="ml-4 list-disc">
                      {#each p.all as s}
                        <li>{new Date(s.created_at).toLocaleString()} - <span class={`badge ${statusColor(s.status)}`}>{s.status}</span> <a href={`/submissions/${s.id}`}>view</a></li>
                      {/each}
                    </ul>
                  </details>
                {/if}
              </td>
            </tr>
          {/each}
          {#if !progress.length}
            <tr><td colspan="4"><i>No students</i></td></tr>
          {/if}
        </tbody>
      </table>
      </div>
    </details>
  {/if}

  {#if role==='student'}
    <div class="card bg-base-100 shadow mb-4">
      <div class="card-body space-y-2">
        <h3 class="card-title">Your submissions</h3>
        <details class="mt-2">
          <summary class="cursor-pointer">View table</summary>
          <div class="overflow-x-auto mt-2">
          <table class="table table-zebra">
            <thead>
              <tr><th>Date</th><th>Status</th><th></th></tr>
            </thead>
            <tbody>
              {#each submissions as s}
                <tr>
                  <td>{new Date(s.created_at).toLocaleString()}</td>
                  <td><span class={`badge ${statusColor(s.status)}`}>{s.status}</span></td>
                  <td><a href={`/submissions/${s.id}`} class="btn btn-sm btn-outline">view</a></td>
                </tr>
              {/each}
              {#if !submissions.length}
                <tr><td colspan="3"><i>No submissions yet</i></td></tr>
              {/if}
            </tbody>
          </table>
          </div>
        </details>
        <button class="btn" on:click={openSubmitModal}>Submit new solution</button>
      </div>
    </div>
    {#if latestSub}
    <div class="card bg-base-100 shadow mb-4">
      <div class="card-body space-y-2">
        <h3 class="card-title">Latest submission results</h3>
        <p>Status: <span class={`badge ${statusColor(latestSub.status)}`}>{latestSub.status}</span></p>
        <details class="mt-2">
          <summary class="cursor-pointer">View results</summary>
          <div class="overflow-x-auto mt-2">
          <table class="table table-zebra">
            <thead>
              <tr><th>Test</th><th>Status</th><th>Runtime (ms)</th><th>Exit</th><th>Traceback</th></tr>
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
        </details>
      </div>
    </div>
    {/if}
  {/if}

{#if role==='teacher' || role==='admin'}
    {#if !assignment.published}
      <button class="btn btn-secondary mb-4" on:click={publish}>Publish assignment</button>
    {/if}
{/if}

  <dialog bind:this={submitDialog} class="modal">
    <div class="modal-box w-11/12 max-w-md space-y-2">
      <h3 class="font-bold text-lg">Submit solution</h3>
      <input type="file" accept=".py" multiple class="file-input file-input-bordered w-full" on:change={e=>files=Array.from((e.target as HTMLInputElement).files||[])}>
      <div class="modal-action">
        <button class="btn" on:click={submit} disabled={!files.length}>Upload</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>

  <dialog bind:this={testsDialog} class="modal">
    <div class="modal-box w-11/12 max-w-xl space-y-4">
      <h3 class="font-bold text-lg mb-2">Manage tests</h3>
      <p class="text-sm text-gray-500">Specify the program input, expected output and time limit in seconds.</p>
      <div class="space-y-4 max-h-60 overflow-y-auto">
        {#each tests as t, i}
          <div class="border rounded p-2 space-y-2">
            <div class="font-semibold">Test {i+1}</div>
            <label class="form-control w-full space-y-1">
              <span class="label-text">Input</span>
              <input class="input input-bordered w-full" placeholder="stdin" bind:value={t.stdin}>
            </label>
            <label class="form-control w-full space-y-1">
              <span class="label-text">Expected output</span>
              <input class="input input-bordered w-full" placeholder="expected stdout" bind:value={t.expected_stdout}>
            </label>
            <label class="form-control w-full space-y-1">
              <span class="label-text">Time limit (s)</span>
              <input class="input input-bordered w-full" placeholder="seconds" bind:value={t.time_limit_sec}>
            </label>
            <div class="flex justify-end gap-2">
              <button class="btn btn-sm" on:click={()=>updateTest(t)}>Save</button>
              <button class="btn btn-sm btn-error" on:click={()=>delTest(t.id)}>Delete</button>
            </div>
          </div>
        {/each}
        {#if !(tests && tests.length)}<p><i>No tests</i></p>{/if}
      </div>
      <div class="border-t pt-2 space-y-2">
        <h4 class="font-semibold">Add test</h4>
        <label class="form-control w-full space-y-1">
          <span class="label-text">Input</span>
          <input class="input input-bordered w-full" placeholder="stdin" bind:value={tStdin}>
        </label>
        <label class="form-control w-full space-y-1">
          <span class="label-text">Expected output</span>
          <input class="input input-bordered w-full" placeholder="expected stdout" bind:value={tStdout}>
        </label>
        <label class="form-control w-full space-y-1">
          <span class="label-text">Time limit (s)</span>
          <input class="input input-bordered w-full" placeholder="seconds" bind:value={tLimit}>
        </label>
        <div class="modal-action">
          <button class="btn btn-primary" on:click={addTest} disabled={!tStdin || !tStdout}>Add</button>
        </div>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>

  {#if err}<p style="color:red">{err}</p>{/if}
{/if}

<style>
pre{display:inline;margin:0 0.5rem;padding:0.2rem;background:#eee}
</style>

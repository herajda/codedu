<script lang="ts">
  import { onMount } from 'svelte'
  import { get } from 'svelte/store'
  import { auth } from '$lib/auth'
import { apiFetch, apiJSON } from '$lib/api'
import { goto } from '$app/navigation'
import { page } from '$app/stores'



$: id = $page.params.id
const role = get(auth)?.role!;

  let assignment:any=null
  let tests:any[]=[] // teacher/admin only
  let submissions:any[]=[] // student submissions
  let allSubs:any[]=[]     // teacher view
  let students:any[]=[]    // class roster for teacher
  let progress:any[]=[]    // computed progress per student
  let pointsEarned=0
  let done=false
  let percent=0
  let err=''
  let tStdin='', tStdout='', tLimit=''
  let file:File|null=null
$: percent = assignment ? Math.round(pointsEarned / assignment.max_points * 100) : 0;
  let editing=false
  let eTitle='', eDesc='', eDeadline='', ePoints=0, ePolicy='all_or_nothing'

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
          return {student:s, latest}
        })
      }
    }catch(e:any){ err=e.message }
  }

  onMount(load)

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

  function startEdit(){
    editing=true
    eTitle=assignment.title
    eDesc=assignment.description
    eDeadline=assignment.deadline.slice(0,16)
    ePoints=assignment.max_points
    ePolicy=assignment.grading_policy
  }

  async function saveEdit(){
    try{
      await apiFetch(`/api/assignments/${id}`,{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({
          title:eTitle,
          description:eDesc,
          deadline:new Date(eDeadline).toISOString(),
          max_points:Number(ePoints),
          grading_policy:ePolicy
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
    return '';
  }

  async function submit(){
    if(!file) return
    const fd = new FormData()
    fd.append('file', file)
    try{
      await apiFetch(`/api/assignments/${id}/submissions`,{method:'POST', body:fd})
      file=null
      alert('Uploaded!')
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
        <textarea class="textarea textarea-bordered w-full" bind:value={eDesc} placeholder="Description" required></textarea>
        <input type="number" min="1" class="input input-bordered w-full" bind:value={ePoints} placeholder="Max points" required>
        <select class="select select-bordered w-full" bind:value={ePolicy}>
          <option value="all_or_nothing">all_or_nothing</option>
          <option value="percentage">percentage</option>
          <option value="weighted">weighted</option>
        </select>
        <input type="datetime-local" class="input input-bordered w-full" bind:value={eDeadline} required>
        <div class="card-actions justify-end">
          <button class="btn btn-primary" on:click={saveEdit}>Save</button>
          <button class="btn" on:click={()=>editing=false}>Cancel</button>
        </div>
      </div>
    </div>
  {:else}
    <div class="card bg-base-100 shadow mb-4">
      <div class="card-body">
        <h1 class="card-title text-2xl">{assignment.title}</h1>
        <p>{assignment.description}</p>
        <p><strong>Deadline:</strong> {new Date(assignment.deadline).toLocaleString()}</p>
        <p><strong>Max points:</strong> {assignment.max_points}</p>
        <p><strong>Policy:</strong> {assignment.grading_policy}</p>
        {#if role==='teacher' || role==='admin'}
          <div class="card-actions justify-end">
            <button class="btn" on:click={startEdit}>Edit</button>
            <button class="btn btn-error" on:click={delAssignment}>Delete</button>
          </div>
        {/if}
      </div>
    </div>
  {/if}
  {#if role==='student'}
    <div class="flex items-center gap-4 mb-4">
      <div class="radial-progress text-primary" style="--value:{percent};" aria-valuenow={percent} role="progressbar">{percent}%</div>
      <div>
        <p class="font-semibold">Your points: {pointsEarned} / {assignment.max_points}</p>
        {#if done}
          <p class="text-success font-bold">Assignment done.</p>
        {/if}
      </div>
    </div>
  {/if}

  {#if role !== 'student'}
    <h2>Tests</h2>
    <ul>
      {#each tests ?? [] as t, i}
        <li>
          <pre>Test {i + 1}</pre> <pre>{t.stdin}</pre>→<pre>{t.expected_stdout}</pre> <span>({t.time_limit_sec} s)</span>
          {#if role==='teacher' || role==='admin'}
            <button on:click={()=>delTest(t.id)}>✕</button>
          {/if}
        </li>
      {/each}
      {#if !(tests && tests.length)}<i>No tests</i>{/if}
    </ul>
  {/if}

  {#if role==='teacher' || role==='admin'}
    <h3>Student progress</h3>
    <div class="overflow-x-auto">
      <table class="table table-zebra">
        <thead>
          <tr><th>Student</th><th>Status</th><th>Last submission</th><th></th></tr>
        </thead>
        <tbody>
          {#each progress as p}
            <tr>
              <td>{p.student.name ?? p.student.email}</td>
              <td><span class={`badge ${statusColor(p.latest ? p.latest.status : 'none')}`}>{p.latest ? p.latest.status : 'none'}</span></td>
              <td>{p.latest ? new Date(p.latest.created_at).toLocaleString() : '-'}</td>
              <td>{#if p.latest}<a href={`/submissions/${p.latest.id}`}>view</a>{/if}</td>
            </tr>
          {/each}
          {#if !progress.length}
            <tr><td colspan="4"><i>No students</i></td></tr>
          {/if}
        </tbody>
      </table>
    </div>
  {/if}

  {#if role==='student'}
    <h3>Your submissions</h3>
    <ul>
      {#each submissions as s}
        <li>
          <a href={`/submissions/${s.id}`}>{new Date(s.created_at).toLocaleString()}</a>
          &nbsp;– {s.status}
        </li>
      {/each}
      {#if !submissions.length}<i>No submissions yet</i>{/if}
    </ul>
  {/if}

  {#if role==='teacher' || role==='admin'}
    {#if !assignment.published}
      <button class="btn btn-secondary mb-4" on:click={publish}>Publish assignment</button>
    {/if}
    <div class="card bg-base-100 shadow mb-4">
      <div class="card-body space-y-2">
        <h3 class="card-title">Add test</h3>
        <input class="input input-bordered w-full" placeholder="stdin" bind:value={tStdin}>
        <input class="input input-bordered w-full" placeholder="expected stdout" bind:value={tStdout}>
        <input class="input input-bordered w-full" placeholder="time limit (s)" bind:value={tLimit}>
        <div class="card-actions justify-end">
          <button class="btn btn-primary" on:click={addTest}>Add</button>
        </div>
      </div>
    </div>
  {/if}

  {#if role==='student'}
    <div class="card bg-base-100 shadow mb-4">
      <div class="card-body space-y-2">
        <h3 class="card-title">Submit solution</h3>
        <input type="file" accept=".py" class="file-input file-input-bordered w-full" on:change={e=>file=(e.target as HTMLInputElement).files?.[0] || null}>
        <div class="card-actions justify-end">
          <button class="btn btn-primary" disabled={!file} on:click={submit}>Upload</button>
        </div>
      </div>
    </div>
  {/if}

  {#if err}<p style="color:red">{err}</p>{/if}
{/if}

<style>
pre{display:inline;margin:0 0.5rem;padding:0.2rem;background:#eee}
</style>

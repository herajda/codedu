<script lang="ts">
  import { onMount } from 'svelte'
  import { get } from 'svelte/store'
  import { auth } from '$lib/auth'
  import { apiFetch, apiJSON } from '$lib/api'

  export let params:{id:string}

  const role = get(auth)?.role!

  let assignment:any=null
  let tests:any[]=[] // teacher/admin only
  let submissions:any[]=[] // student submissions
  let allSubs:any[]=[]     // teacher view
  let students:any[]=[]    // class roster for teacher
  let progress:any[]=[]    // computed progress per student
  let pointsEarned=0
  let done=false
  let err=''
  let tStdin='', tStdout='', tLimit=''
  let file:File|null=null
  let editing=false
  let eTitle='', eDesc='', eDeadline='', ePoints=0, ePolicy='all_or_nothing'

  async function publish(){
    try{
      await apiFetch(`/api/assignments/${params.id}/publish`,{method:'PUT'})
      await load()
    }catch(e:any){ err=e.message }
  }

  async function load(){
    err=''
    try{
      const data = await apiJSON(`/api/assignments/${params.id}`)
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
      await apiFetch(`/api/assignments/${params.id}/tests`,{
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
      await apiFetch(`/api/assignments/${params.id}`,{
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
      await apiFetch(`/api/assignments/${params.id}`,{method:'DELETE'})
      window.location.hash=`#/classes/${assignment.class_id}`
    }catch(e:any){ err=e.message }
  }

  async function delTest(tid:number){
    if(!confirm('Delete this test?')) return
    try{
      await apiFetch(`/api/tests/${tid}`,{method:'DELETE'})
      await load()
    }catch(e:any){ err=e.message }
  }

  async function submit(){
    if(!file) return
    const fd = new FormData()
    fd.append('file', file)
    try{
      await apiFetch(`/api/assignments/${params.id}/submissions`,{method:'POST', body:fd})
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
    <h1>Edit assignment</h1>
    <input bind:value={eTitle} placeholder="Title" required>
    <br>
    <textarea bind:value={eDesc} placeholder="Description" required></textarea>
    <br>
    <input type="number" min="1" bind:value={ePoints} placeholder="Max points" required>
    <br>
    <select bind:value={ePolicy}>
      <option value="all_or_nothing">all_or_nothing</option>
      <option value="percentage">percentage</option>
      <option value="weighted">weighted</option>
    </select>
    <br>
    <input type="datetime-local" bind:value={eDeadline} required>
    <br>
    <button on:click={saveEdit}>Save</button>
    <button on:click={()=>editing=false}>Cancel</button>
  {:else}
    <h1>{assignment.title}</h1>
    <p>{assignment.description}</p>
    <p><strong>Deadline:</strong> {new Date(assignment.deadline).toLocaleString()}</p>
    <p><strong>Max points:</strong> {assignment.max_points}</p>
    <p><strong>Policy:</strong> {assignment.grading_policy}</p>
    {#if role==='teacher' || role==='admin'}
      <button on:click={startEdit}>Edit</button>
      <button on:click={delAssignment}>Delete assignment</button>
    {/if}
  {/if}
  {#if role==='student'}
    <p><strong>Your points:</strong> {pointsEarned} / {assignment.max_points}</p>
    {#if done}
      <p style="color:green"><strong>Assignment done.</strong></p>
    {/if}
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
    <table>
      <thead>
        <tr><th>Student</th><th>Status</th><th>Last submission</th><th></th></tr>
      </thead>
      <tbody>
        {#each progress as p}
          <tr>
            <td>{p.student.name ?? p.student.email}</td>
            <td>{p.latest ? p.latest.status : 'none'}</td>
            <td>{p.latest ? new Date(p.latest.created_at).toLocaleString() : '-'}</td>
            <td>{#if p.latest}<a href={`/submissions/${p.latest.id}`}>view</a>{/if}</td>
          </tr>
        {/each}
        {#if !progress.length}
          <tr><td colspan="4"><i>No students</i></td></tr>
        {/if}
      </tbody>
    </table>
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
      <button on:click={publish}>Publish assignment</button>
    {/if}
    <h3>Add test</h3>
    <input placeholder="stdin" bind:value={tStdin}>
    <br>
    <input placeholder="expected stdout" bind:value={tStdout}>
    <br>
    <input placeholder="time limit (s)" bind:value={tLimit}>
    <br>
    <button on:click={addTest}>Add</button>
  {/if}

  {#if role==='student'}
    <h3>Submit solution</h3>
    <input type="file" accept=".py" on:change={e=>file=(e.target as HTMLInputElement).files?.[0] || null}>
    <button disabled={!file} on:click={submit}>Upload</button>
  {/if}

  {#if err}<p style="color:red">{err}</p>{/if}
{/if}

<style>
pre{display:inline;margin:0 0.5rem;padding:0.2rem;background:#eee}
</style>

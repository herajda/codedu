<script lang="ts">
  import { onMount } from 'svelte';
  import { auth } from '$lib/auth';
import { apiFetch, apiJSON } from '$lib/api';
import { formatDateTime } from "$lib/date";
import { page } from '$app/stores';
import { goto } from '$app/navigation';
import { marked } from 'marked';
  import { Filter, Search, Plus, CheckCircle2, AlertTriangle, Clock } from 'lucide-svelte';

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    load();
  }
  let role = '';
  $: role = $auth?.role ?? '';

  let cls:any = null;
  let loading = true;
  let students:any[] = [];
  let assignments:any[] = [];
  let progressCounts:any = {};
  let mySubs:any[] = [];
  let allStudents:any[] = [];
  let selectedIDs:number[] = [];
  let search='';
  let addDialog: HTMLDialogElement;
  $: filtered = allStudents.filter(s => (s.name ?? s.email).toLowerCase().includes(search.toLowerCase()));
  let aTitle='';
  let aShowTraceback=false;
  let aDescription = '';
  let aDeadlineLocal = '';
  let aMaxPoints: number = 100;
  let aGradingPolicy: 'all_or_nothing' | 'weighted' = 'all_or_nothing';
  let aPublished = false;
  let aTemplateFile: File | null = null;
  let aTestsFile: File | null = null;
  let creating = false;
  let err='';
  let now = Date.now();
  let newName = '';

  onMount(() => {
    const t = setInterval(() => now = Date.now(), 60000);
    return () => clearInterval(t);
  });

  function countdown(deadline: string) {
    const diff = new Date(deadline).getTime() - now;
    if (diff <= 0) return 'late';
    const d = Math.floor(diff / 86400000);
    if (d >= 1) return `${d}d`;
    const h = Math.floor(diff / 3600000);
    const m = Math.floor((diff % 3600000) / 60000);
    return `${h}h ${m}m`;
  }

  async function load() {
    loading = true;
    err='';
    cls = null;
    try {
      const data = await apiJSON(`/api/classes/${id}`);
      cls = data;
      newName = data.name;
      students = data.students ?? [];
      assignments = [...(data.assignments ?? [])].sort((a,b)=>new Date(a.deadline).getTime()-new Date(b.deadline).getTime());
      if (role === 'student') {
        mySubs = await apiJSON('/api/my-submissions');
        assignments = assignments.map(a => {
          const subs = mySubs.filter((s:any)=>s.assignment_id===a.id);
          const best = subs.reduce((m:number,s:any)=>{
            const p = s.override_points ?? s.points ?? 0;
            return p>m ? p : m;
          },0);
          return {
            ...a,
            best,
            completed: subs.some((s:any)=>s.status==='completed')
          };
        });
      }
      if (role === 'teacher' || role === 'admin') {
        allStudents = await apiJSON('/api/students');
        const prog = await apiJSON(`/api/classes/${id}/progress`);
        progressCounts = {};
        for (const a of prog.assignments ?? []) {
          const done = (prog.scores ?? []).filter((sc:any)=>sc.assignment_id===a.id && (sc.points ?? 0) >= a.max_points).length;
          progressCounts[a.id] = done;
        }
      }
    } catch(e:any){ err=e.message }
    loading = false;
  }

  onMount(load);

  async function addStudents(){
    try{
      await apiFetch(`/api/classes/${id}/students`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({student_ids:selectedIDs})});
      selectedIDs=[];
      addDialog.close();
      await load();
    }catch(e:any){ err=e.message }
  }

  async function removeStudent(sid:number){
    if(!confirm('Remove this student from class?')) return;
    try{
      await apiFetch(`/api/classes/${id}/students/${sid}`,{method:'DELETE'});
      await load();
    }catch(e:any){ err=e.message }
  }

  async function createAssignment(){
    try{
      creating = true;
      err = '';
      // 1) create base assignment and capture id
      const created = await apiJSON(`/api/classes/${id}/assignments`,{
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({ title: aTitle, description: aDescription, show_traceback: aShowTraceback })
      });
      const newId = created.id;
      // 2) update advanced fields
      const dlISO = aDeadlineLocal ? new Date(aDeadlineLocal).toISOString() : new Date(Date.now()+24*3600*1000).toISOString();
      await apiFetch(`/api/assignments/${newId}`,{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({
          title: aTitle,
          description: aDescription,
          deadline: dlISO,
          max_points: aMaxPoints,
          grading_policy: aGradingPolicy,
          show_traceback: aShowTraceback
        })
      });
      // 3) optional template upload
      if (aTemplateFile) {
        const fd = new FormData();
        fd.append('file', aTemplateFile);
        await apiFetch(`/api/assignments/${newId}/template`, { method: 'POST', body: fd });
      }
      // 4) optional unit tests upload
      if (aTestsFile) {
        const fd2 = new FormData();
        fd2.append('file', aTestsFile);
        await apiFetch(`/api/assignments/${newId}/tests/upload`, { method: 'POST', body: fd2 });
      }
      // 5) optional publish
      if (aPublished) {
        await apiFetch(`/api/assignments/${newId}/publish`, { method: 'PUT' });
      }
      // reset form
      aTitle='';
      aDescription='';
      aShowTraceback=false;
      aDeadlineLocal='';
      aMaxPoints=100;
      aGradingPolicy='all_or_nothing';
      aPublished=false;
      aTemplateFile=null;
      aTestsFile=null;
      await load();
    }catch(e:any){ err=e.message } finally { creating = false; }
  }

  // Quick create: one-click create and jump to assignment editor
  async function quickCreateAssignment() {
    try {
      const created = await apiJSON(`/api/classes/${id}/assignments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: 'Untitled assignment', description: '', show_traceback: false })
      });
      goto(`/assignments/${created.id}?new=1`);
    } catch (e: any) { err = e.message; }
  }


  function openAddModal(){
    addDialog.showModal();
  }

  let bkUser='';
  let bkPass='';
  let bkAtoms:{Id:string;Name:string}[]=[];
  let loadingAtoms=false;

  async function fetchAtoms(){
    err='';
    loadingAtoms=true;
    try{
      bkAtoms = await apiJSON('/api/bakalari/atoms',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:bkUser,password:bkPass})});
    }catch(e:any){ err=e.message }
    loadingAtoms=false;
  }

  async function importAtom(aid:string){
    err='';
    try{
      const res = await apiJSON<{added:number}>(`/api/classes/${id}/import-bakalari`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:bkUser,password:bkPass,atom_id:aid})});
      await load();
      alert(`Imported ${res.added} students`);
    }catch(e:any){ err=e.message }
  }

  async function renameClass(){
    try{
      await apiFetch(`/api/classes/${id}`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:newName})});
      cls.name = newName;
    }catch(e:any){ err=e.message }
  }

  async function deleteClass(){
    if(!confirm('Delete this class?')) return;
    try{
      await apiFetch(`/api/classes/${id}`,{method:'DELETE'});
      goto('/my-classes');
    }catch(e:any){ err=e.message }
  }
</script>

{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <div class="flex items-start justify-between gap-3 mb-4">
    <div>
      <h1 class="text-2xl font-semibold">{cls.name}</h1>
      {#if role === 'student'}
        <p class="opacity-70 text-sm">Teacher: {cls.teacher.name ?? cls.teacher.email}</p>
      {/if}
    </div>
    {#if role === 'teacher' || role === 'admin'}
      <div class="flex gap-2">
        <button class="btn" on:click={openAddModal} type="button"><Plus class="w-4 h-4" aria-hidden="true" /> Add students</button>
        <button class="btn btn-outline" on:click={renameClass} type="button">Rename</button>
      </div>
    {/if}
  </div>

  <div class="grid gap-6 lg:grid-cols-3">
    <section class="lg:col-span-2">
      <div class="card-elevated p-5">
        <div class="flex items-center justify-between mb-3">
          <h2 class="font-semibold">Assignments</h2>
          <div class="flex items-center gap-2">
            <div class="join hidden sm:flex">
              <button class="btn btn-sm join-item btn-ghost" type="button"><Filter class="w-4 h-4" aria-hidden="true" /> All</button>
              <button class="btn btn-sm join-item btn-ghost" type="button"><Clock class="w-4 h-4" aria-hidden="true" /> Upcoming</button>
              <button class="btn btn-sm join-item btn-ghost" type="button"><AlertTriangle class="w-4 h-4" aria-hidden="true" /> Late</button>
            </div>
            <label class="input input-bordered input-sm flex items-center gap-2">
              <Search class="w-4 h-4" aria-hidden="true" />
              <input type="text" class="grow" placeholder="Search" bind:value={search} />
            </label>
            {#if role === 'teacher' || role === 'admin'}
              <button class="btn btn-sm" type="button" on:click={quickCreateAssignment}>New assignment</button>
            {/if}
          </div>
        </div>
        <ul class="space-y-3">
          {#each assignments.filter(a => (a.title ?? '').toLowerCase().includes(search.toLowerCase())) as a}
            <li>
              <a href={`/assignments/${a.id}`} class="block no-underline text-current">
                <div class="card-elevated p-4 hover:shadow-md transition">
                  <div class="flex items-center justify-between gap-4">
                    <div class="min-w-0">
                      <div class="font-medium truncate">{a.title}</div>
                      <div class="text-sm opacity-70 flex items-center gap-2">
                        <span class={new Date(a.deadline)<new Date() && !a.completed ? 'text-error' : ''}>{formatDateTime(a.deadline)}</span>
                        <span>·</span>
                        <span>{countdown(a.deadline)}</span>
                      </div>
                    </div>
                    <div class="flex items-center gap-3 shrink-0">
                      {#if role==='student'}
                        <div class="flex items-center gap-2">
                          <progress class="progress progress-primary w-24" value={a.best || 0} max={a.max_points}></progress>
                          <span class="text-sm whitespace-nowrap">{a.best ?? 0}/{a.max_points}</span>
                        </div>
                      {/if}
                      {#if role==='teacher' || role==='admin'}
                        {#if a.published}
                          <div class="flex items-center gap-2">
                            <progress class="progress progress-primary w-24" value={progressCounts[a.id] || 0} max={students.length}></progress>
                            <span class="text-sm whitespace-nowrap">{progressCounts[a.id] || 0}/{students.length}</span>
                          </div>
                        {/if}
                      {/if}
                      {#if a.completed}
                        <span class="badge badge-success"><CheckCircle2 class="w-3 h-3" aria-hidden="true" /> Done</span>
                      {/if}
                      {#if !a.published}
                        <span class="badge badge-warning">Unpublished</span>
                      {/if}
                    </div>
                  </div>
                </div>
              </a>
            </li>
          {/each}
          {#if !assignments.length}
            <li class="text-sm opacity-70">No assignments yet</li>
          {/if}
        </ul>

      </div>
    </section>

    <aside class="space-y-6">
      <div class="card-elevated p-5">
        <h3 class="font-semibold mb-3">Class info</h3>
        <ul class="text-sm space-y-2">
          <li><span class="opacity-70">Students:</span> {students.length}</li>
          <li><span class="opacity-70">Assignments:</span> {assignments.length}</li>
        </ul>
      </div>
      {#if role === 'teacher' || role === 'admin'}
        <div class="card-elevated p-5">
          <h3 class="font-semibold mb-3">Manage</h3>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-outline" on:click={openAddModal} type="button"><Plus class="w-4 h-4" aria-hidden="true" /> Add students</button>
          </div>
        </div>
      {/if}
    </aside>
  </div>
  
  <dialog bind:this={addDialog} class="modal">
    <div class="modal-box max-w-2xl space-y-3">
      <h3 class="font-bold text-lg">Add students</h3>
      <label class="input input-bordered flex items-center gap-2">
        <Search class="w-4 h-4" aria-hidden="true" />
        <input type="text" class="grow" placeholder="Search students..." bind:value={search} />
      </label>
      <div class="max-h-80 overflow-auto divide-y divide-base-300/60">
        {#each filtered as s}
          <label class="flex items-center gap-2 py-2">
            <input type="checkbox" class="checkbox"
              checked={selectedIDs.includes(s.id)}
              on:change={(e) => {
                const checked = (e.target as HTMLInputElement).checked;
                selectedIDs = checked ? Array.from(new Set([...selectedIDs, s.id])) : selectedIDs.filter((id) => id !== s.id);
              }}
            />
            <span class="font-medium">{s.name ?? s.email}</span>
          </label>
        {/each}
        {#if !filtered.length}
          <p class="py-2 opacity-70 text-sm">No matches</p>
        {/if}
      </div>
      <div class="modal-action">
        <button class="btn" on:click={addStudents} type="button">Add selected</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>
{/if}

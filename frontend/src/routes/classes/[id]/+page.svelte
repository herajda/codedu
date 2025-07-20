<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { auth } from '$lib/auth';
import { apiFetch, apiJSON } from '$lib/api';
import { page } from '$app/stores';
import { goto } from '$app/navigation';
import { marked } from 'marked';

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    load();
  }
  const role: string = get(auth)?.role ?? '';

  let cls:any = null;
  let loading = true;
  let students:any[] = [];
  let assignments:any[] = [];
  let mySubs:any[] = [];
  let allStudents:any[] = [];
  let selectedIDs:number[] = [];
  let search='';
  let addDialog: HTMLDialogElement;
  $: filtered = allStudents.filter(s => (s.name ?? s.email).toLowerCase().includes(search.toLowerCase()));
  let aTitle='';
  let aShowTraceback=false;
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
        assignments = assignments.map(a => ({
          ...a,
          completed: mySubs.some((s:any)=>s.assignment_id===a.id && s.status==='completed')
        }));
      }
      if (role === 'teacher' || role === 'admin') allStudents = await apiJSON('/api/students');
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
      await apiFetch(`/api/classes/${id}/assignments`,{
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({title:aTitle, show_traceback:aShowTraceback})
      });
      aTitle='';
      aShowTraceback=false;
      await load();
    }catch(e:any){ err=e.message }
  }

  async function deleteAssignment(aid:number){
    if(!confirm('Delete this assignment?')) return;
    try{
      await apiFetch(`/api/assignments/${aid}`,{method:'DELETE'});
      await load();
    }catch(e:any){ err=e.message }
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
  <h1 class="text-2xl font-bold mb-4">{cls.name}</h1>
  {#if role === 'student'}
    <p class="mb-4"><strong>Teacher:</strong> {cls.teacher.name ?? cls.teacher.email}</p>
  {/if}

  <div class="space-y-6">


    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <h2 class="card-title">Assignments</h2>
        <ul class="space-y-4">
          {#each assignments as a}
            <li>
              <a href={`/assignments/${a.id}`} class={`block no-underline text-current card shadow transition hover:-translate-y-1 hover:shadow-lg ${a.completed ? 'bg-success/10' : 'bg-base-100'}`}>
                <div class="card-body flex-col sm:flex-row justify-between items-start sm:items-center gap-2 py-3">
                  <span class="text-lg font-semibold text-primary">{a.title}</span>
                  <div class="flex flex-col sm:flex-row items-start sm:items-center gap-2 sm:justify-end w-full sm:w-auto">
                    <span class={`badge ${new Date(a.deadline)<new Date() && !a.completed ? 'badge-error' : 'badge-info'}`}>{new Date(a.deadline).toLocaleString()}</span>
                    <span class="text-sm">{countdown(a.deadline)}</span>
                    {#if !a.published}
                      <span class="badge badge-warning">unpublished</span>
                    {/if}
                    {#if a.completed}
                      <span class="badge badge-success">done</span>
                    {/if}
                    {#if role === 'teacher' || role === 'admin'}
                      <button class="btn btn-xs btn-error" on:click|preventDefault|stopPropagation={() => deleteAssignment(a.id)}>Delete</button>
                    {/if}
                  </div>
                </div>
              </a>
            </li>
          {/each}
          {#if !assignments.length}<li><i>No assignments yet</i></li>{/if}
        </ul>

        {#if role === 'teacher' || role === 'admin'}
          <form class="mt-4 space-y-2" on:submit|preventDefault={createAssignment}>
            <input class="input input-bordered w-full" placeholder="Title" bind:value={aTitle} required>
            <label class="flex items-center gap-2">
              <input type="checkbox" class="checkbox" bind:checked={aShowTraceback}>
              <span class="label-text">Show traceback to students</span>
            </label>
            <button class="btn">Create</button>
          </form>
        {/if}
      </div>
    </div>

  </div>
{/if}

<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { auth } from '$lib/auth';
import { apiFetch, apiJSON } from '$lib/api';
import { page } from '$app/stores';
import { MarkdownEditor } from '$lib';
import { marked } from 'marked';

  $: id = $page.params.id;
  const role: string = get(auth)?.role ?? '';

  let cls:any = null;
  let students:any[] = [];
  let assignments:any[] = [];
  let allStudents:any[] = [];
  let selectedIDs:number[] = [];
  let search='';
  let addDialog: HTMLDialogElement;
  $: filtered = allStudents.filter(s => (s.name ?? s.email).toLowerCase().includes(search.toLowerCase()));
  let aTitle='';
  let aDesc='';
  let err='';

  async function load() {
    err='';
    try {
      const data = await apiJSON(`/api/classes/${id}`);
      cls = data;
      students = data.students;
      assignments = [...(data.assignments ?? [])].sort((a,b)=>new Date(a.deadline).getTime()-new Date(b.deadline).getTime());
      if (role === 'teacher' || role === 'admin') allStudents = await apiJSON('/api/students');
    } catch(e:any){ err=e.message }
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
        body:JSON.stringify({title:aTitle, description:aDesc})
      });
      aTitle='';
      aDesc='';
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
</script>

{#if !cls}
  <p>Loading…</p>
{:else}
  <h1 class="text-2xl font-bold mb-4">{cls.name}</h1>
  {#if role === 'student'}
    <p class="mb-4"><strong>Teacher:</strong> {cls.teacher.name ?? cls.teacher.email}</p>
  {/if}

  <div class="space-y-6">
    {#if role === 'teacher' || role === 'admin'}
      <div class="card bg-base-100 shadow">
        <div class="card-body">
          <h2 class="card-title">Students</h2>
          <ul class="space-y-1">
            {#each students as s}
              <li class="flex justify-between items-center">
                <span>{s.name ?? s.email}</span>
                {#if role === 'teacher' || role === 'admin'}
                  <button class="btn btn-xs btn-error" on:click={()=>removeStudent(s.id)}>Remove</button>
                {/if}
              </li>
            {/each}
            {#if !students.length}<li><i>No students yet</i></li>{/if}
          </ul>

          <div class="mt-4 space-y-2">
            <button class="btn" on:click={openAddModal}>Add students</button>

            <details class="collapse collapse-arrow mt-2">
              <summary class="collapse-title font-medium">Import from Bakaláři</summary>
              <div class="collapse-content space-y-2">
                <input class="input input-bordered w-full" placeholder="Username" bind:value={bkUser}>
                <input class="input input-bordered w-full" type="password" placeholder="Password" bind:value={bkPass}>
                <button class="btn" on:click={fetchAtoms} disabled={loadingAtoms}>Load classes</button>
                {#if bkAtoms.length}
                  <ul class="menu">
                    {#each bkAtoms as a}
                      <li><button class="btn btn-sm btn-outline w-full justify-between" on:click={()=>importAtom(a.Id)}>{a.Name}</button></li>
                    {/each}
                  </ul>
                {:else if loadingAtoms}
                  <span class="loading loading-dots"></span>
                {/if}
              </div>
            </details>
          </div>
      </div>
    </div>
    <dialog bind:this={addDialog} class="modal">
      <div class="modal-box w-11/12 max-w-lg">
        <h3 class="font-bold text-lg mb-3">Add students</h3>
        <input class="input input-bordered w-full mb-3" placeholder="Search" bind:value={search} />
        <div class="max-h-60 overflow-y-auto space-y-2 mb-4">
          {#each filtered as s}
            <label class="flex items-center gap-2">
              <input type="checkbox" class="checkbox" value={s.id} bind:group={selectedIDs} />
              <span>{s.name ?? s.email}</span>
            </label>
          {/each}
          {#if !filtered.length}
            <p><i>No students</i></p>
          {/if}
        </div>
        <div class="modal-action">
          <button class="btn" on:click={addStudents} disabled={!selectedIDs.length}>Add selected</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop"><button>close</button></form>
    </dialog>
  {/if}

    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <h2 class="card-title">Assignments</h2>
        <ul class="space-y-4">
          {#each assignments as a}
            <li class="border-b pb-2 last:border-none">
              <div class="flex justify-between items-center">
                <a href={`/assignments/${a.id}`} class="link link-primary text-lg">{a.title}</a>
                {#if role === 'teacher' || role === 'admin'}
                  <button class="btn btn-xs btn-error" on:click={()=>deleteAssignment(a.id)}>Delete</button>
                {/if}
              </div>
              <div class="text-sm mb-1">
                {#if !a.published}
                  <span class="badge badge-sm mr-2">draft</span>
                {/if}
                <span class={new Date(a.deadline)<new Date() ? 'text-error' : ''}>due {new Date(a.deadline).toLocaleString()}</span>
              </div>
              <p class="text-sm markdown">{@html marked.parse(a.description)} (max {a.max_points} pts, {a.grading_policy})</p>
            </li>
          {/each}
          {#if !assignments.length}<li><i>No assignments yet</i></li>{/if}
        </ul>

        {#if role === 'teacher' || role === 'admin'}
          <form class="mt-4" on:submit|preventDefault={createAssignment}>
            <input class="input input-bordered w-full mb-2" placeholder="Title" bind:value={aTitle} required>
            <MarkdownEditor bind:value={aDesc} placeholder="Description" class="mb-2" />
            <button class="btn">Create</button>
          </form>
        {/if}
      </div>
    </div>

    {#if err}<p class="text-error">{err}</p>{/if}
  </div>
{/if}

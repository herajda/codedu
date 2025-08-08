<script lang="ts">
import { onMount, tick } from 'svelte';
import { auth } from '$lib/auth';
import { apiFetch, apiJSON } from '$lib/api';
import { page } from '$app/stores';
import { goto } from '$app/navigation';

let id = $page.params.id;
$: if ($page.params.id !== id) { id = $page.params.id; load(); }
let role = '';
$: role = $auth?.role ?? '';

let cls:any = null;
let loading = true;
let students:any[] = [];
let allStudents:any[] = [];
let selectedIDs:number[] = [];
let search='';
let addDialog: HTMLDialogElement;
$: filtered = allStudents.filter(s => (s.name ?? s.email).toLowerCase().includes(search.toLowerCase()));
let err='';
let newName = '';
let renaming = false;
let renameInput: HTMLInputElement;

async function load(){
  loading = true; err=''; cls=null;
  try{
    const data = await apiJSON(`/api/classes/${id}`);
    cls = data;
    newName = data.name;
    students = data.students ?? [];
    if(role==='teacher' || role==='admin') allStudents = await apiJSON('/api/students');
  }catch(e:any){ err=e.message }
  loading=false;
}

onMount(load);

function startRename(){
  renaming = true;
  tick().then(()=>renameInput?.focus());
}

async function renameClass(){
  try{
    await apiFetch(`/api/classes/${id}`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:newName})});
    cls.name = newName;
    renaming=false;
  }catch(e:any){ err=e.message }
}

async function deleteClass(){
  if(!confirm('Delete this class?')) return;
  try{
    await apiFetch(`/api/classes/${id}`,{method:'DELETE'});
    goto('/my-classes');
  }catch(e:any){ err=e.message }
}

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

{#if loading}
  <p>Loading…</p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <h1 class="text-2xl font-bold mb-4">{cls.name}</h1>
  {#if role==='teacher' || role==='admin'}
    <form class="mb-4 flex gap-2 max-w-sm items-start" on:submit|preventDefault={renameClass}>
      {#if renaming}
        <input class="input input-bordered flex-1" bind:value={newName} bind:this={renameInput} />
        <button class="btn">Save</button>
        <button type="button" class="btn" on:click={()=>{renaming=false}}>Cancel</button>
      {:else}
        <button type="button" class="btn" on:click={startRename}>Rename class</button>
      {/if}
      <button type="button" class="btn btn-error ml-auto" on:click={deleteClass}>Delete</button>
    </form>
  {/if}

  <div class="space-y-6">
    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <h2 class="card-title">Students</h2>
        <ul class="space-y-1">
          {#each students as s}
            <li class="flex justify-between items-center">
              <span>{s.name ?? s.email}</span>
              {#if role==='teacher' || role==='admin'}
                <button class="btn btn-xs btn-error" on:click={()=>removeStudent(s.id)}>Remove</button>
              {/if}
            </li>
          {/each}
          {#if !students.length}<li><i>No students yet</i></li>{/if}
        </ul>

        <div class="mt-4">
          <button class="btn" on:click={openAddModal}>Add students</button>
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

      <details class="collapse collapse-arrow mt-4">
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
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>
  </div>
{/if}

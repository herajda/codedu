<script lang="ts">
// @ts-nocheck
import { onMount } from 'svelte';
import { auth } from '$lib/auth';
import { apiJSON, apiFetch } from '$lib/api';
import { formatDateTime } from '$lib/date';
import { TEACHER_GROUP_ID } from '$lib/teacherGroup';
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { goto } from '$app/navigation';

type ClassFile = {
  id: string;
  class_id: string;
  parent_id?: string | null;
  name: string;
  path: string;
  is_dir: boolean;
  assignment_id?: string | null;
  size?: number;
  created_at: string;
  updated_at: string;
};

// Fixed Teachers' group ID
const groupId = TEACHER_GROUP_ID;

let role = '';
$: role = $auth?.role ?? '';

let items: ClassFile[] = [];
let displayed: ClassFile[] = [];
let breadcrumbs: { id: string | null; name: string }[] = [{ id: null, name: '🏠' }];
let currentParent: string | null = null;
let loading = false;
let err = '';

let search = '';
let searchOpen = false;
$: if (searchOpen && search.trim() !== '') { fetchSearch(search.trim()); } else { displayed = filterItems(items); }

function filterItems(list: ClassFile[]): ClassFile[] {
  // Only folders and assignment references
  return list.filter(it => it.is_dir || !!it.assignment_id);
}

async function load(parent: string | null) {
  // Normalize root marker from sessionStorage
  if ((parent as any) === '') parent = null;
  loading = true; err = '';
  try {
    const q = parent === null ? '' : `?parent=${parent}`;
    const list = await apiJSON(`/api/classes/${groupId}/files${q}`);
    items = list;
    displayed = filterItems(items);
    currentParent = parent;
  } catch (e: any) { err = e.message; }
  loading = false;
}

async function fetchSearch(q: string) {
  loading = true; err = '';
  try {
    const list = await apiJSON(`/api/classes/${groupId}/files?search=${encodeURIComponent(q)}`);
    displayed = filterItems(list);
  } catch (e: any) { err = e.message; }
  loading = false;
}

async function openDir(item: ClassFile) {
  breadcrumbs = [...breadcrumbs, { id: item.id, name: item.name }];
  if (typeof sessionStorage !== 'undefined') {
    sessionStorage.setItem(`tassign_bc`, JSON.stringify(breadcrumbs));
    sessionStorage.setItem(`tassign_parent`, String(item.id));
  }
  await load(item.id);
}

function crumbTo(i: number) {
  const b = breadcrumbs[i];
  breadcrumbs = breadcrumbs.slice(0, i + 1);
  if (typeof sessionStorage !== 'undefined') {
    sessionStorage.setItem(`tassign_bc`, JSON.stringify(breadcrumbs));
    sessionStorage.setItem(`tassign_parent`, b.id === null ? '' : String(b.id));
  }
  load(b.id);
}

// Create folder
async function createDir(name: string) {
  await apiFetch(`/api/classes/${groupId}/files`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, parent_id: currentParent, is_dir: true }) });
  await load(currentParent);
}
function promptDir() { const nm = prompt('Folder name'); if (nm) createDir(nm); }

// Rename
async function rename(it: ClassFile) {
  const nm = prompt('New name', it.name);
  if (!nm || nm.trim() === '' || nm === it.name) return;
  await apiFetch(`/api/files/${it.id}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: nm }) });
  await load(currentParent);
}

// Delete
async function del(it: ClassFile) {
  if (!confirm(`Delete ${it.name}?`)) return;
  await apiFetch(`/api/files/${it.id}`, { method: 'DELETE' });
  await load(currentParent);
}

// Add assignment ref modal
let addDialog: HTMLDialogElement;
let myAssignments: any[] = [];
let myAssignLoading = false;
let selectedAssignmentId = '';
let addName = '';
let addErr = '';
let addClasses: any[] = [];
let selectedAddClassId = '';
$: filteredAddAssignments = selectedAddClassId
  ? myAssignments.filter(a => a.class_id === selectedAddClassId)
  : [];
$: if (!selectedAddClassId && selectedAssignmentId) {
  selectedAssignmentId = '';
}
$: if (selectedAddClassId && selectedAssignmentId) {
  const matches = myAssignments.some(a => a.id === selectedAssignmentId && a.class_id === selectedAddClassId);
  if (!matches) selectedAssignmentId = '';
}

async function openAdd() {
  addErr = '';
  addName = '';
  selectedAssignmentId = '';
  selectedAddClassId = '';
  addClasses = [];
  myAssignLoading = true;
  try {
    // Teachers get their own assignments; admins get all
    const [assignments, classes] = await Promise.all([
      apiJSON('/api/assignments'),
      apiJSON('/api/classes')
    ]);
    myAssignments = assignments;
    addClasses = classes;
  } catch (e: any) { addErr = e.message; }
  myAssignLoading = false;
  addDialog.showModal();
}

async function doAdd() {
  if (!selectedAssignmentId) { addErr = 'Pick an assignment'; return; }
  const body: any = { parent_id: currentParent, assignment_id: selectedAssignmentId };
  if (addName.trim() !== '') body.name = addName.trim();
  const res = await apiFetch(`/api/classes/${groupId}/files`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!res.ok) { const js = await res.json().catch(() => ({})); addErr = js.error || res.statusText; return; }
  addDialog.close();
  await load(currentParent);
}

// Copy to class modal
let copyDialog: HTMLDialogElement;
let myClasses: any[] = [];
let copyClassId: string | null = null;
let copyErr = '';
let copyItem: ClassFile | null = null;

async function openCopy(it: ClassFile) {
  copyErr = '';
  copyItem = it;
  copyClassId = null;
  try { myClasses = await apiJSON('/api/classes'); }
  catch (e:any) { copyErr = e.message; }
  copyDialog.showModal();
}

async function doCopy() {
  if (!copyItem || !copyItem.assignment_id) { copyErr = 'Invalid item'; return; }
  if (!copyClassId) { copyErr = 'Choose a class'; return; }
  const res = await apiFetch(`/api/classes/${copyClassId}/assignments/import`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ source_assignment_id: copyItem.assignment_id }) });
  let js: any = null;
  try { js = await res.json(); } catch {}
  if (!res.ok) { copyErr = js?.error || res.statusText; return; }
  copyDialog.close();
  if (js?.assignment_id) {
    await goto(`/assignments/${js.assignment_id}?new=1`);
  } else {
    await goto(`/classes/${copyClassId}/assignments`);
  }
}

// Preview logic
let previewDialog: HTMLDialogElement;
let preview: any = null;
let previewLoading = false;
let previewErr = '';
let previewItem: ClassFile | null = null;
let previewClass: any = null;
let safeDesc = '';

async function openPreview(it: ClassFile) {
  if (!it.assignment_id) return;
  previewItem = it; previewErr=''; preview=null; previewClass=null; previewLoading=true;
  previewDialog.showModal();
  try {
    const data = await apiJSON(`/api/assignments/${it.assignment_id}`);
    preview = data;
    try { previewClass = await apiJSON(`/api/classes/${data.assignment.class_id}`); } catch {}
    try { safeDesc = DOMPurify.sanitize((marked.parse(preview.assignment.description) as string) || ''); } catch { safeDesc=''; }
  } catch (e:any) {
    previewErr = e.message;
  }
  previewLoading=false;
}

function copyAssignmentLink() {
  try {
    if (!preview?.assignment?.id) return;
    const url = `${location.origin}/teachers/assignments/preview/${preview.assignment.id}`;
    navigator.clipboard?.writeText(url);
    alert('Link copied to clipboard');
  } catch {}
}

function openFullPreview() {
  try {
    if (!preview?.assignment?.id) return;
    const url = `/teachers/assignments/preview/${preview.assignment.id}`;
    goto(url);
  } catch {}
}

onMount(() => {
  let storedParent: string | null = null;
  if (typeof sessionStorage !== 'undefined') {
    const raw = sessionStorage.getItem('tassign_parent');
    storedParent = raw && raw.trim() !== '' ? raw : null;
    const bc = sessionStorage.getItem('tassign_bc');
    if (bc) { try { breadcrumbs = JSON.parse(bc); } catch {} }
  }
  load(storedParent);
});
</script>

<div>
  <nav class="mb-4 sticky top-16 z-30 bg-base-200 rounded-box shadow px-4 py-2 flex items-center flex-wrap gap-2">
    <ul class="flex flex-wrap gap-1 text-sm items-center flex-grow">
      {#each breadcrumbs as b,i}
        <li class="after:mx-1 after:content-['/'] last:after:hidden">
          <button type="button" class="link px-2 py-1 rounded hover:bg-base-300" on:click={() => crumbTo(i)} aria-label={`Open ${b.name}`}>{b.name}</button>
        </li>
      {/each}
    </ul>
    <div class="flex items-center gap-2 ml-auto">
      <div class="relative">
        {#if searchOpen}
          <input class="input input-bordered input-sm w-48 sm:w-72" placeholder="Search…" bind:value={search} />
        {:else}
          <button class="btn btn-sm" on:click={() => searchOpen = !searchOpen}><i class="fa-solid fa-magnifying-glass mr-2"></i>Search</button>
        {/if}
      </div>
      {#if role === 'teacher' || role === 'admin'}
        <button class="btn btn-sm" on:click={openAdd}><i class="fa-solid fa-plus mr-2"></i>Add Assignment</button>
        <button class="btn btn-sm" on:click={promptDir}><i class="fa-solid fa-folder-plus mr-2"></i>Folder</button>
      {/if}
    </div>
  </nav>

  {#if err}<p class="text-error">{err}</p>{/if}
  {#if loading}<p>Loading…</p>{/if}

  <div class="overflow-x-auto mb-4">
    <table class="table table-zebra w-full">
      <thead>
        <tr>
          <th class="text-left">Name</th>
          <th class="text-left">Type</th>
          <th class="text-right">Modified</th>
          <th class="w-64 text-right"></th>
        </tr>
      </thead>
      <tbody>
        {#each displayed as it (it.id)}
          <tr class="hover:bg-base-200">
            <td>
              {#if it.is_dir}
                <button class="link" on:click={() => openDir(it)}><i class="fa-solid fa-folder text-warning mr-2"></i>{it.name}</button>
              {:else}
                <button class="link" on:click={() => openPreview(it)} title="Preview assignment">
                  <i class="fa-solid fa-file-circle-check text-primary mr-2"></i>{it.name}
                </button>
              {/if}
              <div class="text-xs text-gray-500">{it.path}</div>
            </td>
            <td>{it.is_dir ? 'Folder' : 'Assignment'}</td>
            <td class="text-right">{formatDateTime(it.updated_at)}</td>
            <td class="text-right">
              {#if role === 'teacher' || role === 'admin'}
                {#if it.assignment_id}
                  <button class="btn btn-xs" on:click={() => openPreview(it)}><i class="fa-solid fa-eye mr-2"></i>Preview</button>
                  <button class="btn btn-xs ml-2" on:click={() => openCopy(it)}><i class="fa-solid fa-copy mr-2"></i>Copy to class</button>
                {/if}
                <button class="btn btn-xs ml-2" on:click={() => rename(it)}><i class="fa-solid fa-pen mr-2"></i>Rename</button>
                <button class="btn btn-xs btn-error ml-2" on:click={() => del(it)}><i class="fa-solid fa-trash mr-2"></i>Delete</button>
              {/if}
            </td>
          </tr>
        {/each}
        {#if !displayed.length}
          <tr><td colspan="4"><i>Nothing here yet</i></td></tr>
        {/if}
      </tbody>
    </table>
  </div>

  <!-- Add assignment modal -->
  <dialog bind:this={addDialog} class="modal">
    <div class="modal-box">
      <h3 class="font-bold mb-3">Add assignment</h3>
      {#if myAssignLoading}
        <p>Loading…</p>
      {:else}
        <div class="rounded-box bg-base-200 p-4 space-y-4">
          <div class="form-control">
            <label class="label"><span class="label-text font-semibold">Choose a class</span></label>
            <select class="select select-bordered select-primary w-full"
              bind:value={selectedAddClassId}>
              <option value="" disabled>Select a class…</option>
              {#each addClasses as c}
                <option value={c.id}>{c.name}</option>
              {/each}
            </select>
            {#if !addClasses.length && !addErr}
              <p class="mt-2 text-sm text-gray-500">You don't have any classes yet.</p>
            {/if}
          </div>
          <div class="form-control">
            <label class="label"><span class="label-text font-semibold">Choose an assignment</span></label>
            <select class="select select-bordered w-full"
              bind:value={selectedAssignmentId}
              disabled={!selectedAddClassId || !filteredAddAssignments.length}>
              <option value="" disabled>{!selectedAddClassId ? 'Select a class first…' : (!filteredAddAssignments.length ? 'No assignments in this class' : 'Select an assignment…')}</option>
              {#if selectedAddClassId}
                {#each filteredAddAssignments as a (a.id)}
                  <option value={a.id}>{a.title}</option>
                {/each}
              {/if}
            </select>
            {#if selectedAddClassId && filteredAddAssignments.length}
              <p class="mt-2 text-xs text-gray-500">{filteredAddAssignments.length} assignment{filteredAddAssignments.length === 1 ? '' : 's'} found in this class.</p>
            {/if}
          </div>
          <div class="form-control">
            <label class="label"><span class="label-text">Display name (optional)</span></label>
            <input class="input input-bordered w-full" bind:value={addName} placeholder="Defaults to assignment title" />
          </div>
        </div>
        {#if addErr}<p class="text-error mt-2">{addErr}</p>{/if}
        <div class="modal-action">
          <form method="dialog"><button class="btn">Cancel</button></form>
          <button class="btn btn-primary" on:click|preventDefault={doAdd}>Add</button>
        </div>
      {/if}
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>

  <!-- Copy to class modal -->
  <dialog bind:this={copyDialog} class="modal">
    <div class="modal-box">
      <h3 class="font-bold mb-3">Copy to my class</h3>
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
        <button class="btn btn-primary" on:click|preventDefault={doCopy}>Copy</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>

  <!-- Preview modal -->
  <dialog bind:this={previewDialog} class="modal">
    <div class="modal-box max-w-4xl">
      {#if previewLoading}
        <div class="py-6 text-center"><span class="loading loading-dots"></span></div>
      {:else if previewErr}
        <p class="text-error">{previewErr}</p>
      {:else if preview?.assignment}
        <div class="flex items-start justify-between gap-3">
          <div>
            <h3 class="font-bold text-lg">{preview.assignment.title}</h3>
            <div class="flex flex-wrap items-center gap-2 mt-1 text-xs">
              <span class="badge badge-ghost" title="Class">
                <i class="fa-solid fa-users mr-1"></i>
                {previewClass?.name ?? 'Class'}
              </span>
              <a class="badge badge-outline" href={`/classes/${preview.assignment.class_id}`} title="Open class">
                <i class="fa-solid fa-arrow-up-right-from-square mr-1"></i>Open class
              </a>
              <span class={`badge ${preview.assignment.published ? 'badge-success' : 'badge-ghost'}`} title="Published">
                {preview.assignment.published ? 'Published' : 'Unpublished'}
              </span>
              <!-- No deadlines in preview for teachers -->
            </div>
          </div>
          <div class="shrink-0 flex items-center gap-2">
            <button class="btn btn-outline btn-sm" on:click={openFullPreview}>
              <i class="fa-solid fa-up-right-from-square mr-2"></i>Open full view
            </button>
            <button class="btn btn-primary btn-sm" on:click={() => { if (previewItem) openCopy(previewItem); }}>
              <i class="fa-solid fa-copy mr-2"></i>Copy to class
            </button>
            <button class="btn btn-sm" on:click={copyAssignmentLink} title="Copy link">
              <i class="fa-solid fa-link mr-2"></i>Copy link
            </button>
          </div>
        </div>

        {#if safeDesc}
          <div class="mt-3 text-sm prose max-w-none">{@html safeDesc}</div>
        {/if}

        <div class="grid md:grid-cols-2 gap-4 mt-4">
          <div class="card bg-base-200 p-3">
            <div class="font-semibold mb-2">Settings</div>
            <ul class="text-sm space-y-1">
              <li><b>Max points:</b> {preview.assignment.max_points}</li>
              <li><b>Grading:</b> {preview.assignment.grading_policy}</li>
              <li><b>Manual review:</b> {preview.assignment.manual_review ? 'Yes' : 'No'}</li>
              <li><b>Show traceback:</b> {preview.assignment.show_traceback ? 'Yes' : 'No'}</li>
              <li><b>LLM interactive:</b> {preview.assignment.llm_interactive ? 'Yes' : 'No'}</li>
              <li><b>LLM feedback:</b> {preview.assignment.llm_feedback ? 'Yes' : 'No'}</li>
              <li><b>LLM auto‑award:</b> {preview.assignment.llm_auto_award ? 'Yes' : 'No'}</li>
              <li><b>LLM strictness:</b> {typeof preview.assignment.llm_strictness === 'number' ? preview.assignment.llm_strictness : 50}</li>
              <li><b>Template:</b> {preview.assignment.template_path ? 'Present' : 'None'}</li>
              <li><b>Created:</b> {formatDateTime(preview.assignment.created_at)}</li>
              <li><b>Updated:</b> {formatDateTime(preview.assignment.updated_at)}</li>
            </ul>
          </div>
          <div class="card bg-base-200 p-3">
            <div class="flex items-center justify-between mb-2">
              <div class="font-semibold">Tests</div>
              <a class="btn btn-xs" href={`/assignments/${preview.assignment.id}/tests`} target="_blank" rel="noopener">
                <i class="fa-solid fa-list-check mr-2"></i>Open tests
              </a>
            </div>
            <ul class="text-sm space-y-2 max-h-64 overflow-auto pr-1">
              {#each (preview.tests ?? []) as t}
                <li class="border-b border-base-300/50 pb-2">
                  {#if t.unittest_name}
                    <div><b>Unit:</b> {t.unittest_name}</div>
                  {:else}
                    <div><b>Stdin:</b> <code class="text-xs whitespace-pre-wrap">{t.stdin}</code></div>
                    <div><b>Expected:</b> <code class="text-xs whitespace-pre-wrap">{t.expected_stdout}</code></div>
                  {/if}
                  <div class="opacity-70 text-xs">Weight: {t.weight} · Time limit: {t.time_limit_sec}s</div>
                </li>
              {/each}
              {#if !(preview.tests ?? []).length}
                <li class="opacity-70">No tests</li>
              {/if}
            </ul>
            <div class="mt-2 text-xs opacity-70">
              Total tests: {(preview.tests ?? []).length}
            </div>
          </div>
        </div>
        {#if (preview.submissions ?? []).length || (preview.teacher_runs ?? []).length}
          <div class="grid md:grid-cols-2 gap-4 mt-4">
            <div class="card bg-base-200 p-3">
              <div class="font-semibold mb-2">Activity</div>
              <ul class="text-sm space-y-1">
                <li><b>Student submissions:</b> {(preview.submissions ?? []).length}</li>
                <li><b>Teacher runs:</b> {(preview.teacher_runs ?? []).length}</li>
              </ul>
            </div>
          </div>
        {/if}
      {/if}
      <div class="modal-action">
        <button class="btn btn-outline" on:click|preventDefault={openFullPreview}>
          <i class="fa-solid fa-up-right-from-square mr-2"></i>Open full view
        </button>
        <form method="dialog"><button class="btn">Close</button></form>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop"><button>close</button></form>
  </dialog>
</div>

<style>
  @import '@fortawesome/fontawesome-free/css/all.min.css';
</style>

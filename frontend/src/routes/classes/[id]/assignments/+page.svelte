<script lang="ts">
  import { onMount } from 'svelte';
  import { auth } from '$lib/auth';
  import { apiJSON, apiFetch } from '$lib/api';
  import { formatDateTime } from '$lib/date';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { Filter, Search, AlertTriangle, Clock, CheckCircle2, Copy, GraduationCap, Users, Plus, FileText, Activity, ArrowRight } from 'lucide-svelte';
  import { TEACHER_GROUP_ID } from '$lib/teacherGroup';
  import { t, translator } from '$lib/i18n';
  
  let translate;
  $: translate = $translator;

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    load();
  }
  let role = '';
  $: role = $auth?.role ?? '';

  let cls: any = null;
  let loading = true;
  let assignments: any[] = [];
  let mySubs: any[] = [];
  let students: any[] = [];
  let progressCounts: Record<string, number> = {};
  let err = '';
  let now = Date.now();
  let search = '';
  type FilterMode = 'all' | 'upcoming' | 'late'
  let filterMode: FilterMode = 'all';
  type SortMode = 'deadline_asc' | 'deadline_desc' | 'title_asc'
  let sortMode: SortMode = 'deadline_asc';

  function percent(done: number, total: number) {
    return total ? Math.round((done / total) * 100) : 0;
  }

  onMount(() => {
    const t = setInterval(() => (now = Date.now()), 60000);
    return () => clearInterval(t);
  });

  function countdown(deadline: string, completed?: boolean) {
    const diff = new Date(deadline).getTime() - now;
    if (diff <= 0) return completed ? t('frontend/src/routes/classes/[id]/assignments/+page.svelte::deadline_passed') : t('frontend/src/routes/classes/[id]/assignments/+page.svelte::overdue');
    const d = Math.floor(diff / 86400000);
    if (d >= 1) return `${d}d`;
    const h = Math.floor(diff / 3600000);
    const m = Math.floor((diff % 3600000) / 60000);
    return `${h}h ${m}m`;
  }

  async function load() {
    loading = true;
    err = '';
    cls = null;
    try {
      const data = await apiJSON(`/api/classes/${id}`);
      cls = data;
      students = data.students ?? [];
      assignments = [...(data.assignments ?? [])];
      if (role === 'student') {
        mySubs = await apiJSON('/api/my-submissions');
        assignments = assignments.map((a) => {
          const subs = mySubs.filter((s: any) => s.assignment_id === a.id);
          const best = subs.reduce((m: number, s: any) => {
            const p = s.override_points ?? s.points ?? 0;
            return p > m ? p : m;
          }, 0);
          return {
            ...a,
            best,
            completed: subs.some((s: any) => s.status === 'completed')
          };
        });
      } else if (role === 'teacher' || role === 'admin') {
        // compute progress counts per assignment
        const prog = await apiJSON(`/api/classes/${id}/progress`);
        progressCounts = {};
        for (const a of prog.assignments ?? []) {
          const done = (prog.scores ?? []).filter((sc: any) => sc.assignment_id === a.id && (sc.points ?? 0) >= a.max_points).length;
          progressCounts[a.id] = done;
        }
      }
    } catch (e: any) {
      err = e.message;
    }
    loading = false;
  }

  onMount(load);

  // Derived visible list with search/filter/sort
  $: visibleAssignments = assignments
    .filter((a) => (a.title ?? '').toLowerCase().includes(search.toLowerCase()))
    .filter((a) => {
      if (filterMode === 'all') return true;
      const isLate = new Date(a.deadline) < new Date();
      return filterMode === 'late' ? isLate : !isLate;
    })
    .slice()
    .sort((a, b) => {
      if (sortMode === 'title_asc') return String(a.title).localeCompare(String(b.title));
      const da = new Date(a.deadline).getTime();
      const db = new Date(b.deadline).getTime();
      return sortMode === 'deadline_desc' ? db - da : da - db;
    });

  // Quick create: one-click create and jump to assignment editor (teachers/admin only)
  async function quickCreateAssignment() {
    try {
      const created = await apiJSON(`/api/classes/${id}/assignments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: t('frontend/src/routes/classes/[id]/assignments/+page.svelte::untitled_assignment'), description: '', show_traceback: false, show_test_details: false })
      });
      goto(`/assignments/${created.id}?new=1`);
    } catch (e: any) {
      err = e.message;
    }
  }

  // Copy assignment from Teachers' group
  let copyDialog: HTMLDialogElement;
  let teacherFiles: any[] = [];
  let copyLoading = false;
  let copyErr = '';
  let selectedAssignmentId = '';
  let copyBreadcrumbs: { id: string | null; name: string }[] = [{ id: null, name: '🏠' }];
  let copyCurrentParent: string | null = null;

  async function openCopyFromTeachers() {
    copyErr = '';
    selectedAssignmentId = '';
    copyLoading = true;
    copyBreadcrumbs = [{ id: null, name: '🏠' }];
    copyCurrentParent = null;
    try {
      // Get files from Teachers' group root
      await loadTeacherFiles(null);
    } catch (e: any) {
      copyErr = e.message;
    }
    copyLoading = false;
    copyDialog.showModal();
  }

  async function loadTeacherFiles(parent: string | null) {
    copyLoading = true;
    copyErr = '';
    try {
      const q = parent === null ? '' : `?parent=${parent}`;
      const files = await apiJSON(`/api/classes/${TEACHER_GROUP_ID}/files${q}`);
      teacherFiles = files.filter((f: any) => f.is_dir || f.assignment_id);
      copyCurrentParent = parent;
    } catch (e: any) {
      copyErr = e.message;
    }
    copyLoading = false;
  }

  async function openTeacherFolder(item: any) {
    if (!item.is_dir) return;
    copyBreadcrumbs = [...copyBreadcrumbs, { id: item.id, name: item.name }];
    await loadTeacherFiles(item.id);
  }

  function copyCrumbTo(i: number) {
    const b = copyBreadcrumbs[i];
    copyBreadcrumbs = copyBreadcrumbs.slice(0, i + 1);
    loadTeacherFiles(b.id);
  }

  async function doCopyFromTeachers() {
    if (!selectedAssignmentId) {
      copyErr = t('frontend/src/routes/classes/[id]/assignments/+page.svelte::please_select_assignment');
      return;
    }
    try {
      const res = await apiFetch(`/api/classes/${id}/assignments/import`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ source_assignment_id: selectedAssignmentId })
      });
      const result = await res.json();
      if (!res.ok) {
        copyErr = result.error || t('frontend/src/routes/classes/[id]/assignments/+page.svelte::failed_to_copy_assignment');
        return;
      }
      copyDialog.close();
      // Navigate to the copied assignment
      if (result.assignment_id) {
        goto(`/assignments/${result.assignment_id}?new=1`);
      } else {
        // Refresh the page to show the new assignment
        load();
      }
    } catch (e: any) {
      copyErr = e.message;
    }
  }
</script>

<svelte:head>
  <title>{cls?.name ? `${cls.name} | CodEdu` : 'Assignments | CodEdu'}</title>
</svelte:head>

{#if loading}
  <div class="flex justify-center mt-8"><span class="loading loading-dots loading-lg"></span></div>
{:else if err}
  <div class="alert alert-error">
    <AlertTriangle size={20} />
    <span>{err}</span>
  </div>
{:else}
  <!-- Premium Class Header -->
  <section class="class-assignments-header relative overflow-hidden bg-base-100 rounded-2xl border border-base-200 shadow-lg shadow-base-300/20 mb-6 p-5 sm:p-6">
    <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
    <div class="absolute -top-12 -right-12 w-48 h-48 bg-primary/10 rounded-full blur-3xl pointer-events-none"></div>
    <div class="relative flex flex-col md:flex-row items-center justify-between gap-4">
      <div class="relative flex flex-col sm:flex-row items-start sm:items-center gap-4 sm:gap-8">
        <div class="relative shrink-0">
          <h1 class="text-sm sm:text-base font-black uppercase tracking-[0.5em] leading-none text-base-content/20">
            {translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::assignments_heading')}
          </h1>
        </div>
        
        <div class="flex flex-col gap-1">
          <h2 class="text-xl sm:text-3xl font-black tracking-tight text-base-content leading-tight">{cls.name}</h2>
          {#if role === 'student'}
            <div class="flex items-center gap-2 text-base-content/50 font-bold uppercase tracking-[0.15em] text-[10px] ml-0.5">
              <div class="w-1 h-1 rounded-full bg-primary/30"></div>
              <span>{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::teacher_prefix')} {cls.teacher.name ?? cls.teacher.email}</span>
            </div>
          {/if}
        </div>
      </div>
      
      <div class="flex flex-wrap items-center gap-3">
        {#if role === 'teacher' || role === 'admin'}
          <button class="btn btn-primary btn-sm rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] h-10 px-4" type="button" on:click={quickCreateAssignment}>
            <Plus size={16} />
            {translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::new_assignment_button')}
          </button>
          <button class="btn btn-ghost btn-sm rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] h-10 border border-base-300 hover:bg-base-200" type="button" on:click={openCopyFromTeachers}>
            <Copy size={16} />
            {translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::copy_from_teachers_group_button')}
          </button>
        {/if}
      </div>
    </div>
  </section>

  <!-- Assignments Section -->
  <div>
    <div class="flex flex-col sm:flex-row sm:items-center justify-end gap-4 mb-6 px-2">
      <div class="flex flex-wrap items-center gap-2 justify-end">
        <div class="join hidden lg:flex bg-base-200/50 p-1 rounded-xl">
          <button class={`btn btn-xs join-item border-none ${filterMode==='all' ? 'bg-base-100 shadow-sm text-primary' : 'bg-transparent opacity-60'}`} type="button" on:click={() => filterMode='all'}>
            {translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::filter_all')}
          </button>
          <button class={`btn btn-xs join-item border-none ${filterMode==='upcoming' ? 'bg-base-100 shadow-sm text-primary' : 'bg-transparent opacity-60'}`} type="button" on:click={() => filterMode='upcoming'}>
            {translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::filter_upcoming')}
          </button>
          <button class={`btn btn-xs join-item border-none ${filterMode==='late' ? 'bg-base-100 shadow-sm text-primary' : 'bg-transparent opacity-60'}`} type="button" on:click={() => filterMode='late'}>
            {translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::filter_overdue')}
          </button>
        </div>

        <div class="relative flex items-center">
          <Search size={14} class="absolute left-3 opacity-40" />
          <input 
            type="text" 
            class="input input-sm bg-base-100 border-base-200 focus:border-primary/30 w-full sm:w-48 pl-9 rounded-xl font-medium text-xs h-9" 
            placeholder={translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::search_placeholder')} 
            bind:value={search} 
          />
        </div>

        <select class="select select-sm bg-base-100 border-base-200 focus:border-primary/30 rounded-xl font-medium text-xs h-9" bind:value={sortMode}>
          <option value="deadline_asc">{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::sort_deadline_asc')}</option>
          <option value="deadline_desc">{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::sort_deadline_desc')}</option>
          <option value="title_asc">{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::sort_title_asc')}</option>
        </select>
      </div>
    </div>

    <div class="grid gap-4 sm:grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
      {#each visibleAssignments as a}
        <a href={`/assignments/${a.id}`} class="group block no-underline text-current">
          <div class="bg-base-100 p-5 rounded-[2rem] border border-base-200 shadow-sm hover:shadow-xl hover:shadow-primary/5 transition-all h-full flex flex-col relative overflow-hidden">
            <div class="absolute top-0 right-0 w-1/4 h-1/4 bg-primary/5 rounded-bl-full pointer-events-none group-hover:scale-150 transition-transform duration-700"></div>
            
            <div class="flex items-start justify-between mb-4 relative">
              <div class="flex items-center gap-3 min-w-0">
                <div class={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${a.completed ? 'bg-success/10 text-success' : 'bg-primary/10 text-primary'} group-hover:bg-primary group-hover:text-primary-content transition-all`}>
                  <FileText size={20} />
                </div>
                <div class="min-w-0">
                  <h3 class="font-black text-base tracking-tight truncate group-hover:text-primary transition-colors">{a.title}</h3>
                  <div class="flex items-center gap-2 text-[10px] font-bold uppercase tracking-wider opacity-40">
                    <Clock size={10} />
                    <span class={new Date(a.deadline) < new Date() && !a.completed ? 'text-error opacity-100' : ''}>
                      {formatDateTime(a.deadline)}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <div class="space-y-4 flex-1">
              {#if role === 'student'}
                <div class="space-y-2">
                  <div class="flex items-center justify-between text-[10px] font-black uppercase tracking-widest opacity-40">
                    <span>{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::dashboard_progress')}</span>
                    <span class="tabular-nums font-black text-base-content">{a.best ?? 0}/{a.max_points}</span>
                  </div>
                  <div class="w-full h-1.5 rounded-full bg-base-200 overflow-hidden">
                    <div class={`h-full transition-all duration-500 rounded-full ${a.completed ? 'bg-success' : 'bg-primary'}`} style={`width: ${percent(a.best || 0, a.max_points)}%`}></div>
                  </div>
                </div>
              {:else if role === 'teacher' || role === 'admin'}
                {#if a.published}
                  <div class="space-y-2">
                    <div class="flex items-center justify-between text-[10px] font-black uppercase tracking-widest opacity-40">
                      <span>{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::dashboard_progress')}</span>
                      <span class="tabular-nums font-black text-base-content">{progressCounts[a.id] || 0}/{students.length}</span>
                    </div>
                    <div class="w-full h-1.5 rounded-full bg-base-200 overflow-hidden">
                      <div class="h-full bg-primary transition-all duration-500 rounded-full" style={`width: ${percent(progressCounts[a.id] || 0, students.length)}%`}></div>
                    </div>
                  </div>
                {:else}
                  <div class="flex items-center gap-2 py-1">
                    <div class="w-1.5 h-1.5 rounded-full bg-warning animate-pulse"></div>
                    <span class="text-[10px] font-bold text-warning uppercase tracking-widest">{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::status_unpublished_badge')}</span>
                  </div>
                {/if}
              {/if}

              <div class="flex flex-wrap gap-1.5 mt-2">
                 <div class="badge badge-sm bg-base-200 border-none font-black text-[9px] uppercase tracking-wider h-6 gap-1 opacity-70">
                   <Activity size={10} />
                   {countdown(a.deadline, a.completed)}
                 </div>
                 {#if a.completed}
                   <div class="badge badge-sm badge-success border-none font-black text-[9px] uppercase tracking-wider h-6 gap-1 text-success-content">
                     <CheckCircle2 size={10} />
                     {translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::status_done_badge')}
                   </div>
                 {/if}
                 {#if a.second_deadline}
                    <div class="badge badge-sm bg-warning/20 text-warning-content border-none font-black text-[9px] uppercase tracking-wider h-6 gap-1">
                      <Clock size={10} />
                      {formatDateTime(a.second_deadline)} ({Math.round(a.late_penalty_ratio * 100)}%)
                    </div>
                 {/if}
              </div>
            </div>

            <div class="mt-4 pt-4 border-t border-base-300/30 flex items-center justify-between">
              <span class="text-[9px] font-black uppercase tracking-widest opacity-30">
                ID: {a.id.slice(0, 8)}
              </span>
              <div class="text-primary opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all">
                <ArrowRight size={16} />
              </div>
            </div>
          </div>
        </a>
      {:else}
        <div class="col-span-full py-12 text-center bg-base-100/50 rounded-[2.5rem] border-2 border-dashed border-base-200">
           <div class="w-12 h-12 rounded-full bg-base-200 flex items-center justify-center mx-auto mb-4 opacity-30">
              <FileText size={20} />
           </div>
           <p class="text-sm font-bold opacity-30 uppercase tracking-widest">
             {translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::no_assignments_yet')}
           </p>
        </div>
      {/each}
    </div>
  </div>


  <!-- Copy from Teachers' group modal -->
  <dialog bind:this={copyDialog} class="modal">
    <div class="modal-box max-w-4xl">
      <h3 class="font-bold mb-3">{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::copy_from_teachers_group_button')}</h3>
      
      <!-- Breadcrumb navigation -->
      <div class="mb-4">
        <nav class="text-sm">
          <ul class="flex flex-wrap gap-1 items-center">
            {#each copyBreadcrumbs as b, i}
              <li class="after:mx-1 after:content-['/'] last:after:hidden">
                <button 
                  type="button" 
                  class="link px-2 py-1 rounded hover:bg-base-300" 
                  on:click={() => copyCrumbTo(i)} 
                  aria-label={`${translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::open_crumb_label')} ${b.name}`}
                >
                  {b.name}
                </button>
              </li>
            {/each}
          </ul>
        </nav>
      </div>

      {#if copyLoading}
        <p>{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::loading')}</p>
      {:else if copyErr}
        <p class="text-error">{copyErr}</p>
      {:else}
        <!-- File structure -->
        <div class="max-h-96 overflow-y-auto border border-base-300 rounded-lg">
          <table class="table table-zebra w-full">
            <thead>
              <tr>
                <th class="text-left">{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::table_header_name')}</th>
                <th class="text-left">{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::table_header_type')}</th>
                <th class="text-right">{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::table_header_modified')}</th>
                <th class="w-32 text-right">{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::table_header_action')}</th>
              </tr>
            </thead>
            <tbody>
              {#each teacherFiles as item}
                <tr class="hover:bg-base-200">
                  <td>
                    {#if item.is_dir}
                      <button 
                        class="link flex items-center" 
                        on:click={() => openTeacherFolder(item)}
                      >
                        <i class="fa-solid fa-folder text-warning mr-2"></i>
                        {item.name}
                      </button>
                    {:else}
                      <div class="flex items-center">
                        <i class="fa-solid fa-file-circle-check text-primary mr-2"></i>
                        {item.name}
                      </div>
                    {/if}
                    <div class="text-xs text-gray-500">{item.path}</div>
                  </td>
                  <td>{item.is_dir ? translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::type_folder') : translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::type_assignment')}</td>
                  <td class="text-right">{formatDateTime(item.updated_at)}</td>
                  <td class="text-right">
                    {#if !item.is_dir && item.assignment_id}
                      <button 
                        class="btn btn-xs btn-primary" 
                        on:click={() => selectedAssignmentId = item.assignment_id}
                        class:btn-active={selectedAssignmentId === item.assignment_id}
                      >
                        {selectedAssignmentId === item.assignment_id ? translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::button_selected') : translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::button_select')}
                      </button>
                    {/if}
                  </td>
                </tr>
              {/each}
              {#if !teacherFiles.length}
                <tr><td colspan="4"><i>{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::no_items_in_folder')}</i></td></tr>
              {/if}
            </tbody>
          </table>
        </div>

        {#if selectedAssignmentId}
          <div class="mt-4 p-3 bg-primary/10 rounded-lg">
            <p class="text-sm">
              <i class="fa-solid fa-check-circle text-primary mr-2"></i>
              {translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::assignment_selected_for_copying')}
            </p>
          </div>
        {/if}

        <div class="modal-action">
          <form method="dialog"><button class="btn">{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::button_cancel')}</button></form>
          <button 
            class="btn btn-primary" 
            on:click|preventDefault={doCopyFromTeachers} 
            disabled={!selectedAssignmentId}
          >
            {translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::button_copy_assignment')}
          </button>
        </div>
      {/if}
    </div>
    <form method="dialog" class="modal-backdrop"><button>{translate('frontend/src/routes/classes/[id]/assignments/+page.svelte::button_close')}</button></form>
  </dialog>
{/if}
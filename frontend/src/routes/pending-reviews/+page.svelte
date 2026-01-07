<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, apiFetch } from '$lib/api';
  import { translator } from '$lib/i18n';
  import type { Translator } from '$lib/i18n';
  import { submissionStatusLabel } from '$lib/status';
  import { loadPendingReviewCount } from '$lib/stores/pendingReviews';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import {
    ClipboardList,
    User,
    Calendar,
    BookOpen,
    School,
    ArrowRight,
    Clock,
    CheckCircle2,
    Filter,
    SortAsc,
    Layers,
    History,
    ChevronRight,
    Search,
    XCircle
  } from 'lucide-svelte';

  interface PendingReview {
    id: string;
    assignment_id: string;
    assignment_title: string;
    class_id: string;
    class_name: string;
    student_id: string;
    student_email: string;
    student_name: string | null;
    student_avatar: string | null;
    status: string;
    points: number | null;
    created_at: string;
    attempt_number: number;
  }

  let reviews: PendingReview[] = [];
  let loading = true;
  let err = '';

  // Filter and sort state
  let sortBy: 'newest' | 'oldest' | 'student' | 'assignment' = 'newest';
  let groupBy: 'none' | 'student' | 'assignment' | 'class' = 'none';
  let filterAssignment = '';
  let searchQuery = '';

  let translate: Translator;
  $: translate = $translator;
  let confirmModal: InstanceType<typeof ConfirmModal>;

  async function load() {
    loading = true;
    err = '';
    try {
      reviews = await apiJSON('/api/pending-reviews');
    } catch (e: any) {
      err = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    load();
    loadPendingReviewCount();
  });

  async function ignoreReview(id: string) {
    const confirmed = await confirmModal.open({
      title: translate('frontend/src/routes/pending-reviews/+page.svelte::skip_submission_title'),
      body: translate('frontend/src/routes/pending-reviews/+page.svelte::skip_submission_body'),
      confirmLabel: translate('frontend/src/routes/pending-reviews/+page.svelte::skip_submission_button'),
      confirmClass: 'btn btn-warning',
      icon: XCircle
    });
    if (!confirmed) return;
    try {
      await apiFetch(`/api/submissions/${id}/skip`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
      });
      await load();
      loadPendingReviewCount();
    } catch (e: any) {
      err = e.message;
    }
  }

  function statusColor(s: string) {
    if (s === 'completed') return 'text-success bg-success/10 border-success/20';
    if (s === 'provisional') return 'text-amber-500 bg-amber-500/10 border-amber-500/20';
    if (s === 'partially_completed') return 'text-orange-500 bg-orange-500/10 border-orange-500/20';
    if (s === 'failed') return 'text-error bg-error/10 border-error/20';
    if (s === 'skipped') return 'text-base-content/60 bg-base-200 border-base-300';
    return 'text-base-content/60 bg-base-200 border-base-300';
  }

  function relativeTime(dateStr: string): string {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHrs = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return translate('frontend/src/routes/pending-reviews/+page.svelte::just_now');
    if (diffMins < 60) return translate('frontend/src/routes/pending-reviews/+page.svelte::minutes_ago', { count: diffMins });
    if (diffHrs < 24) return translate('frontend/src/routes/pending-reviews/+page.svelte::hours_ago', { count: diffHrs });
    return translate('frontend/src/routes/pending-reviews/+page.svelte::days_ago', { count: diffDays });
  }

  function sortReviews(list: PendingReview[], order: typeof sortBy) {
    const sorted = [...list];
    switch (order) {
      case 'newest':
        sorted.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
        break;
      case 'oldest':
        sorted.sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
        break;
      case 'student':
        sorted.sort((a, b) => (a.student_name || a.student_email).localeCompare(b.student_name || b.student_email));
        break;
      case 'assignment':
        sorted.sort((a, b) => a.assignment_title.localeCompare(b.assignment_title));
        break;
    }
    return sorted;
  }

  function filterReviews(list: PendingReview[], assignmentId: string, query: string) {
    return list.filter(r => {
      if (assignmentId && r.assignment_id !== assignmentId) return false;
      if (query) {
        const q = query.toLowerCase();
        const match = 
          (r.student_name || '').toLowerCase().includes(q) ||
          r.student_email.toLowerCase().includes(q) ||
          r.assignment_title.toLowerCase().includes(q) ||
          r.class_name.toLowerCase().includes(q);
        if (!match) return false;
      }
      return true;
    });
  }

  // Get unique classes and assignments for filters
  $: uniqueAssignments = [...new Map(reviews.map(r => [r.assignment_id, { id: r.assignment_id, title: r.assignment_title }])).values()];

  // Apply filters and sorting
  $: filtered = sortReviews(filterReviews(reviews, filterAssignment, searchQuery), sortBy);

  // Group by logic
  type GroupedReviews = { key: string; label: string; items: PendingReview[] }[];
  $: grouped = (() => {
    if (groupBy === 'none') return null;
    const groups = new Map<string, { label: string; items: PendingReview[] }>();
    for (const r of filtered) {
      let key: string;
      let label: string;
      if (groupBy === 'student') {
        key = r.student_id;
        label = r.student_name || r.student_email;
      } else if (groupBy === 'assignment') {
        key = r.assignment_id;
        label = r.assignment_title;
      } else {
        key = r.class_id;
        label = r.class_name;
      }
      if (!groups.has(key)) {
        groups.set(key, { label, items: [] });
      }
      groups.get(key)!.items.push(r);
    }
    return Array.from(groups.entries()).map(([key, val]) => ({ key, ...val }));
  })();
</script>

<svelte:head>
  <title>{translate('frontend/src/routes/pending-reviews/+page.svelte::page_title')} | CodEdu</title>
</svelte:head>

<div class="max-w-6xl mx-auto space-y-8 pb-12 px-4 lg:px-0">
  <!-- Header Section -->
  <div class="flex flex-col md:flex-row md:items-center justify-between gap-6 pt-4">
    <div class="flex items-center gap-4">
      <div class="p-4 rounded-2xl bg-gradient-to-br from-amber-500 to-orange-600 shadow-lg shadow-amber-500/20 text-white">
        <ClipboardList class="w-8 h-8" />
      </div>
      <div>
        <h1 class="text-4xl font-extrabold tracking-tight bg-clip-text text-transparent bg-gradient-to-r from-base-content to-base-content/70">
          {translate('frontend/src/routes/pending-reviews/+page.svelte::page_title')}
        </h1>
        <p class="text-base-content/60 font-medium">{translate('frontend/src/routes/pending-reviews/+page.svelte::page_subtitle')}</p>
      </div>
    </div>

    {#if !loading && reviews.length > 0}
      <div class="flex items-center gap-2 bg-amber-500/10 text-amber-600 px-4 py-2 rounded-full border border-amber-500/20 font-bold shadow-sm backdrop-blur-sm self-start md:self-center">
        <Clock class="w-5 h-5 animate-pulse" />
        <span>{reviews.length} {translate('frontend/src/routes/pending-reviews/+page.svelte::pending_count')}</span>
      </div>
    {/if}
  </div>

  <!-- Filters and Search -->
  {#if !loading && reviews.length > 0}
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-4">
      <!-- Search Bar -->
      <div class="lg:col-span-4 relative group">
        <div class="absolute inset-y-0 left-4 flex items-center pointer-events-none text-base-content/40 group-focus-within:text-primary transition-colors">
          <Search class="w-5 h-5" />
        </div>
        <input 
          type="text" 
          bind:value={searchQuery}
          placeholder="Search students, assignments..."
          class="input input-lg w-full pl-12 bg-base-100/50 backdrop-blur-md border border-base-300 focus:border-primary/50 focus:ring-4 focus:ring-primary/10 rounded-2xl transition-all shadow-sm"
        />
      </div>

      <!-- Quick Action Controls -->
      <div class="lg:col-span-8 flex flex-wrap items-center gap-3">
        <div class="flex items-center gap-2 px-4 py-2 bg-base-100/50 border border-base-300 rounded-2xl shadow-sm">
          <SortAsc class="w-4 h-4 text-primary" />
          <span class="text-sm font-semibold text-base-content/60">{translate('frontend/src/routes/pending-reviews/+page.svelte::sort_by')}:</span>
          <select class="select select-ghost select-sm font-bold focus:bg-transparent" bind:value={sortBy}>
            <option value="newest">{translate('frontend/src/routes/pending-reviews/+page.svelte::sort_newest')}</option>
            <option value="oldest">{translate('frontend/src/routes/pending-reviews/+page.svelte::sort_oldest')}</option>
            <option value="student">{translate('frontend/src/routes/pending-reviews/+page.svelte::sort_student')}</option>
            <option value="assignment">{translate('frontend/src/routes/pending-reviews/+page.svelte::sort_assignment')}</option>
          </select>
        </div>

        <div class="flex items-center gap-2 px-4 py-2 bg-base-100/50 border border-base-300 rounded-2xl shadow-sm">
          <Layers class="w-4 h-4 text-primary" />
          <span class="text-sm font-semibold text-base-content/60">{translate('frontend/src/routes/pending-reviews/+page.svelte::group_by')}:</span>
          <select class="select select-ghost select-sm font-bold focus:bg-transparent" bind:value={groupBy}>
            <option value="none">{translate('frontend/src/routes/pending-reviews/+page.svelte::group_none')}</option>
            <option value="student">{translate('frontend/src/routes/pending-reviews/+page.svelte::group_student')}</option>
            <option value="assignment">{translate('frontend/src/routes/pending-reviews/+page.svelte::group_assignment')}</option>
            <option value="class">{translate('frontend/src/routes/pending-reviews/+page.svelte::group_class')}</option>
          </select>
        </div>

        {#if uniqueAssignments.length > 1}
          <div class="flex items-center gap-2 px-4 py-2 bg-base-100/50 border border-base-300 rounded-2xl shadow-sm">
            <BookOpen class="w-4 h-4 text-primary" />
            <select class="select select-ghost select-sm font-bold focus:bg-transparent" bind:value={filterAssignment}>
              <option value="">{translate('frontend/src/routes/pending-reviews/+page.svelte::filter_all')} Assignments</option>
              {#each uniqueAssignments as a}
                <option value={a.id}>{a.title}</option>
              {/each}
            </select>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Content States -->
  <div class="space-y-4">
    {#if loading}
      <div class="space-y-4">
        {#each Array(5) as _}
          <div class="h-28 bg-base-100/50 border border-base-300 rounded-[2rem] animate-pulse flex items-center px-6 gap-6">
            <div class="w-16 h-16 rounded-full bg-base-300"></div>
            <div class="flex-1 space-y-3">
              <div class="h-5 w-1/3 bg-base-300 rounded-full"></div>
              <div class="h-4 w-1/4 bg-base-200 rounded-full"></div>
            </div>
            <div class="w-32 h-10 bg-base-200 rounded-2xl"></div>
          </div>
        {/each}
      </div>

    {:else if reviews.length === 0}
      <div class="flex flex-col items-center justify-center py-20 px-4 bg-base-100/30 backdrop-blur rounded-[3rem] border-2 border-dashed border-base-300 shadow-inner group transition-all duration-500">
        <div class="relative mb-8">
            <div class="absolute inset-0 bg-success/20 blur-3xl rounded-full group-hover:bg-success/30 transition-all duration-700"></div>
            <div class="relative p-8 rounded-full bg-base-100 border border-base-200 shadow-xl group-hover:scale-110 transition-transform duration-500">
                <CheckCircle2 class="w-20 h-20 text-success" />
            </div>
        </div>
        <h2 class="text-3xl font-black mb-3">{translate('frontend/src/routes/pending-reviews/+page.svelte::empty_title')}</h2>
        <p class="text-base-content/60 font-medium max-w-sm text-center leading-relaxed">
            {translate('frontend/src/routes/pending-reviews/+page.svelte::empty_description')}
        </p>
      </div>

    {:else if grouped}
      {#each grouped as group}
        <div class="space-y-4 pt-4 first:pt-0">
          <div class="flex items-center gap-3 px-2 group/header">
            <div class="p-2 rounded-xl bg-primary/10 text-primary group-hover/header:rotate-12 transition-transform">
              {#if groupBy === 'student'}
                <User class="w-5 h-5" />
              {:else if groupBy === 'assignment'}
                <BookOpen class="w-5 h-5" />
              {:else}
                <School class="w-5 h-5" />
              {/if}
            </div>
            <h3 class="text-xl font-bold">{group.label}</h3>
            <span class="px-2 py-0.5 rounded-lg bg-base-200 text-xs font-bold opacity-60 uppercase tracking-widest">{group.items.length} items</span>
          </div>
          
          <div class="grid grid-cols-1 gap-3">
            {#each group.items as review}
              <a
                href="/submissions/{review.id}"
                class="flex flex-col lg:flex-row lg:items-center gap-4 p-5 md:p-6 bg-base-100/60 backdrop-blur-md border border-base-300 hover:border-primary/40 hover:bg-base-100/80 hover:scale-[1.01] transition-all duration-300 rounded-[2rem] shadow-sm hover:shadow-xl group no-underline"
              >
                <!-- Avatar Section -->
                <div class="flex items-center gap-4">
                  <div class="relative">
                    {#if review.student_avatar}
                      <div class="w-16 h-16 rounded-full overflow-hidden shadow-md ring-2 ring-base-200 group-hover:ring-primary/20 transition-all">
                        <img src={review.student_avatar} alt={review.student_name || review.student_email} class="w-full h-full object-cover" />
                      </div>
                    {:else}
                      <div class="w-16 h-16 rounded-full bg-gradient-to-br from-primary/20 via-primary/10 to-secondary/20 flex items-center justify-center shadow-inner text-2xl font-black text-primary border border-primary/20 group-hover:border-primary/40 transition-all">
                        {(review.student_name || review.student_email).charAt(0).toUpperCase()}
                      </div>
                    {/if}
                    <div class="absolute -bottom-1 -right-1 w-6 h-6 rounded-full bg-base-100 border-2 border-base-100 shadow-sm flex items-center justify-center text-primary">
                        <User class="w-3.5 h-3.5" />
                    </div>
                  </div>

                  <div class="flex-1 min-w-0 lg:hidden">
                    <div class="font-bold text-lg leading-tight truncate">{review.student_name || review.student_email}</div>
                    <div class="flex items-center gap-2 mt-1">
                      <span class={`px-2 py-0.5 rounded-lg border text-[10px] font-black uppercase tracking-tighter ${statusColor(review.status)}`}>
                        {submissionStatusLabel(review.status)}
                      </span>
                      {#if review.attempt_number > 1}
                        <span class="inline-flex items-center gap-1 text-[10px] font-bold opacity-40 px-2 py-0.5 bg-base-200 rounded-lg uppercase tracking-widest">
                            <History class="w-3 h-3" />
                            #{review.attempt_number}
                        </span>
                      {/if}
                    </div>
                  </div>
                </div>

                <!-- Info Content (Desktop) -->
                <div class="hidden lg:flex flex-1 flex-col min-w-0">
                  <div class="flex items-center gap-3">
                    <span class="text-xl font-bold truncate group-hover:text-primary transition-colors">{review.student_name || review.student_email}</span>
                    <span class={`px-3 py-0.5 rounded-lg border text-[10px] font-black uppercase tracking-tighter shadow-sm ${statusColor(review.status)}`}>
                      {submissionStatusLabel(review.status)}
                    </span>
                    {#if review.attempt_number > 1}
                      <span class="inline-flex items-center gap-1 text-[10px] font-bold opacity-40 px-2 py-0.5 bg-base-200 rounded-lg uppercase tracking-widest">
                          <History class="w-3 h-3" />
                          Attempt #{review.attempt_number}
                      </span>
                    {/if}
                  </div>
                  <div class="flex items-center gap-4 mt-2 text-sm font-semibold text-base-content/50">
                    <span class="flex items-center gap-1.5 truncate bg-base-200/50 px-3 py-1 rounded-xl group-hover:bg-primary/5 transition-colors">
                      <BookOpen class="w-4 h-4 text-primary/60" />
                      {review.assignment_title}
                    </span>
                    <span class="flex items-center gap-1.5 bg-base-200/50 px-3 py-1 rounded-xl group-hover:bg-primary/5 transition-colors">
                      <School class="w-4 h-4 text-primary/60" />
                      {review.class_name}
                    </span>
                  </div>
                </div>

                <!-- Desktop Relative Time -->
                <div class="hidden lg:flex items-center gap-1.5 text-xs font-bold opacity-40 group-hover:opacity-100 transition-opacity whitespace-nowrap px-4">
                  <Calendar class="w-3.5 h-3.5" />
                  {relativeTime(review.created_at)}
                </div>

                <!-- Mobile view info -->
                <div class="lg:hidden space-y-3 mt-1">
                    <div class="flex flex-col gap-2 p-3 bg-base-200/40 rounded-2xl border border-base-200">
                        <div class="flex items-center gap-2 text-sm font-bold">
                            <BookOpen class="w-4 h-4 text-primary/60" />
                            <span class="truncate">{review.assignment_title}</span>
                        </div>
                        <div class="flex items-center gap-2 text-xs font-semibold opacity-60">
                            <School class="w-4 h-4" />
                            <span>{review.class_name}</span>
                        </div>
                    </div>
                    <div class="flex items-center justify-between gap-2">
                        <div class="flex items-center gap-1.5 text-xs opacity-60 font-bold">
                            <Calendar class="w-3.5 h-3.5" />
                            {relativeTime(review.created_at)}
                        </div>
                        <div class="flex items-center gap-2">
                          <button
                            type="button"
                            class="btn btn-sm btn-ghost rounded-xl gap-2 font-bold px-4"
                            on:click|preventDefault|stopPropagation={() => ignoreReview(review.id)}
                          >
                            <XCircle class="w-4 h-4" />
                            {translate('frontend/src/routes/pending-reviews/+page.svelte::skip_submission_button')}
                          </button>
                          <div class="btn btn-sm btn-primary rounded-xl gap-2 font-bold px-4">
                              {translate('frontend/src/routes/pending-reviews/+page.svelte::review_button')}
                              <ArrowRight class="w-4 h-4" />
                          </div>
                        </div>
                    </div>
                </div>

                <!-- Desktop Action -->
                <div class="hidden lg:flex items-center px-2 gap-3">
                  <button
                    type="button"
                    class="btn btn-ghost btn-sm rounded-xl gap-2 font-bold uppercase text-[10px] tracking-[0.2em] opacity-0 translate-x-4 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-300"
                    on:click|preventDefault|stopPropagation={() => ignoreReview(review.id)}
                  >
                    <XCircle class="w-4 h-4" />
                    {translate('frontend/src/routes/pending-reviews/+page.svelte::skip_submission_button')}
                  </button>
                  <div class="flex items-center gap-4 text-primary font-black uppercase text-[10px] tracking-[0.2em] opacity-0 translate-x-4 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-300">
                    {translate('frontend/src/routes/pending-reviews/+page.svelte::review_button')}
                    <div class="p-3 rounded-2xl bg-primary text-white shadow-lg shadow-primary/30 group-hover:scale-110 transition-transform">
                        <ChevronRight class="w-5 h-5" strokeWidth={3} />
                    </div>
                  </div>
                </div>
              </a>
            {/each}
          </div>
        </div>
      {/each}

    {:else}
      <div class="grid grid-cols-1 gap-3">
        {#each filtered as review}
          <a
            href="/submissions/{review.id}"
            class="flex flex-col lg:flex-row lg:items-center gap-4 p-5 md:p-6 bg-base-100/60 backdrop-blur-md border border-base-300 hover:border-primary/40 hover:bg-base-100/80 hover:scale-[1.01] transition-all duration-300 rounded-[2rem] shadow-sm hover:shadow-xl group no-underline"
          >
            <!-- Avatar Section -->
            <div class="flex items-center gap-4">
              <div class="relative">
                {#if review.student_avatar}
                  <div class="w-16 h-16 rounded-full overflow-hidden shadow-md ring-2 ring-base-200 group-hover:ring-primary/20 transition-all">
                    <img src={review.student_avatar} alt={review.student_name || review.student_email} class="w-full h-full object-cover" />
                  </div>
                {:else}
                  <div class="w-16 h-16 rounded-full bg-gradient-to-br from-primary/20 via-primary/10 to-secondary/20 flex items-center justify-center shadow-inner text-2xl font-black text-primary border border-primary/20 group-hover:border-primary/40 transition-all">
                    {(review.student_name || review.student_email).charAt(0).toUpperCase()}
                  </div>
                {/if}
                <div class="absolute -bottom-1 -right-1 w-6 h-6 rounded-full bg-base-100 border-2 border-base-100 shadow-sm flex items-center justify-center text-primary">
                    <User class="w-3.5 h-3.5" />
                </div>
              </div>

              <div class="flex-1 min-w-0 lg:hidden">
                <div class="font-bold text-lg leading-tight truncate">{review.student_name || review.student_email}</div>
                <div class="flex items-center gap-2 mt-1">
                  <span class={`px-2 py-0.5 rounded-lg border text-[10px] font-black uppercase tracking-tighter ${statusColor(review.status)}`}>
                    {submissionStatusLabel(review.status)}
                  </span>
                  {#if review.attempt_number > 1}
                    <span class="inline-flex items-center gap-1 text-[10px] font-bold opacity-40 px-2 py-0.5 bg-base-200 rounded-lg uppercase tracking-widest">
                        <History class="w-3 h-3" />
                        #{review.attempt_number}
                    </span>
                  {/if}
                </div>
              </div>
            </div>

            <!-- Info Content (Desktop) -->
            <div class="hidden lg:flex flex-1 flex-col min-w-0">
              <div class="flex items-center gap-3">
                <span class="text-xl font-bold truncate group-hover:text-primary transition-colors">{review.student_name || review.student_email}</span>
                <span class={`px-3 py-0.5 rounded-lg border text-[10px] font-black uppercase tracking-tighter shadow-sm ${statusColor(review.status)}`}>
                  {submissionStatusLabel(review.status)}
                </span>
                {#if review.attempt_number > 1}
                  <span class="inline-flex items-center gap-1 text-[10px] font-bold opacity-40 px-2 py-0.5 bg-base-200 rounded-lg uppercase tracking-widest">
                      <History class="w-3 h-3" />
                      Attempt #{review.attempt_number}
                  </span>
                {/if}
              </div>
              <div class="flex items-center gap-4 mt-2 text-sm font-semibold text-base-content/50">
                <span class="flex items-center gap-1.5 truncate bg-base-200/50 px-3 py-1 rounded-xl group-hover:bg-primary/5 transition-colors">
                  <BookOpen class="w-4 h-4 text-primary/60" />
                  {review.assignment_title}
                </span>
                <span class="flex items-center gap-1.5 bg-base-200/50 px-3 py-1 rounded-xl group-hover:bg-primary/5 transition-colors">
                  <School class="w-4 h-4 text-primary/60" />
                  {review.class_name}
                </span>
              </div>
            </div>

            <!-- Desktop Relative Time -->
            <div class="hidden lg:flex items-center gap-1.5 text-xs font-bold opacity-40 group-hover:opacity-100 transition-opacity whitespace-nowrap px-4">
              <Calendar class="w-3.5 h-3.5" />
              {relativeTime(review.created_at)}
            </div>

            <!-- Mobile view info -->
            <div class="lg:hidden space-y-3 mt-1">
                <div class="flex flex-col gap-2 p-3 bg-base-200/40 rounded-2xl border border-base-200">
                    <div class="flex items-center gap-2 text-sm font-bold">
                        <BookOpen class="w-4 h-4 text-primary/60" />
                        <span class="truncate">{review.assignment_title}</span>
                    </div>
                    <div class="flex items-center gap-2 text-xs font-semibold opacity-60">
                        <School class="w-4 h-4" />
                        <span>{review.class_name}</span>
                    </div>
                </div>
                <div class="flex items-center justify-between gap-2">
                    <div class="flex items-center gap-1.5 text-xs opacity-60 font-bold">
                        <Calendar class="w-3.5 h-3.5" />
                        {relativeTime(review.created_at)}
                    </div>
                    <div class="flex items-center gap-2">
                      <button
                        type="button"
                        class="btn btn-sm btn-ghost rounded-xl gap-2 font-bold px-4"
                        on:click|preventDefault|stopPropagation={() => ignoreReview(review.id)}
                      >
                        <XCircle class="w-4 h-4" />
                        {translate('frontend/src/routes/pending-reviews/+page.svelte::skip_submission_button')}
                      </button>
                      <div class="btn btn-sm btn-primary rounded-xl gap-2 font-bold px-4">
                          {translate('frontend/src/routes/pending-reviews/+page.svelte::review_button')}
                          <ArrowRight class="w-4 h-4" />
                      </div>
                    </div>
                </div>
            </div>

            <!-- Desktop Action -->
            <div class="hidden lg:flex items-center px-2 gap-3">
              <button
                type="button"
                class="btn btn-ghost btn-sm rounded-xl gap-2 font-bold uppercase text-[10px] tracking-[0.2em] opacity-0 translate-x-4 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-300"
                on:click|preventDefault|stopPropagation={() => ignoreReview(review.id)}
              >
                <XCircle class="w-4 h-4" />
                {translate('frontend/src/routes/pending-reviews/+page.svelte::skip_submission_button')}
              </button>
              <div class="flex items-center gap-4 text-primary font-black uppercase text-[10px] tracking-[0.2em] opacity-0 translate-x-4 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-300">
                {translate('frontend/src/routes/pending-reviews/+page.svelte::review_button')}
                <div class="p-3 rounded-2xl bg-primary text-white shadow-lg shadow-primary/30 group-hover:scale-110 transition-transform">
                    <ChevronRight class="w-5 h-5" strokeWidth={3} />
                </div>
              </div>
            </div>
          </a>
        {/each}
      </div>
    {/if}
  </div>

  {#if err}
    <div class="alert alert-error rounded-2xl shadow-lg border-2 border-error/20">
      <div class="flex items-center gap-3">
        <div class="p-2 bg-white/20 rounded-lg">
          <Clock class="w-5 h-5 rotate-45" />
        </div>
        <span class="font-bold">{err}</span>
      </div>
    </div>
  {/if}
</div>

<ConfirmModal bind:this={confirmModal} />

<style>
  :global(.select:focus) {
    outline: none;
  }
</style>

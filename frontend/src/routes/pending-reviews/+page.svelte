<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { translator } from '$lib/i18n';
  import type { Translator } from '$lib/i18n';
  import { formatDateTime } from '$lib/date';
  import { submissionStatusLabel } from '$lib/status';
  import { loadPendingReviewCount } from '$lib/stores/pendingReviews';
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
    SortDesc
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
  let filterClass = '';
  let filterAssignment = '';

  let translate: Translator;
  $: translate = $translator;

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
    // Refresh count for sidebar
    loadPendingReviewCount();
  });

  function statusColor(s: string) {
    if (s === 'completed') return 'badge-success';
    if (s === 'provisional') return 'badge-warning';
    if (s === 'partially_completed') return 'badge-warning';
    if (s === 'failed') return 'badge-error';
    return '';
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

  function sortReviews(list: PendingReview[]) {
    const sorted = [...list];
    switch (sortBy) {
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

  function filterReviews(list: PendingReview[]) {
    return list.filter(r => {
      if (filterClass && r.class_id !== filterClass) return false;
      if (filterAssignment && r.assignment_id !== filterAssignment) return false;
      return true;
    });
  }

  // Get unique classes and assignments for filters
  $: uniqueClasses = [...new Map(reviews.map(r => [r.class_id, { id: r.class_id, name: r.class_name }])).values()];
  $: uniqueAssignments = [...new Map(reviews.map(r => [r.assignment_id, { id: r.assignment_id, title: r.assignment_title }])).values()];

  // Apply filters and sorting
  $: filtered = sortReviews(filterReviews(reviews));

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

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col md:flex-row md:items-end md:justify-between gap-4">
    <div>
      <div class="flex items-center gap-3">
        <div class="p-3 rounded-2xl bg-gradient-to-br from-amber-500/20 to-orange-500/20">
          <ClipboardList class="w-8 h-8 text-amber-500" />
        </div>
        <div>
          <h1 class="text-3xl font-bold tracking-tight">{translate('frontend/src/routes/pending-reviews/+page.svelte::page_title')}</h1>
          <p class="text-sm opacity-70">{translate('frontend/src/routes/pending-reviews/+page.svelte::page_subtitle')}</p>
        </div>
      </div>
    </div>

    {#if !loading && reviews.length > 0}
      <div class="badge badge-lg badge-warning gap-2 shadow-lg">
        <Clock class="w-4 h-4" />
        {reviews.length} {translate('frontend/src/routes/pending-reviews/+page.svelte::pending_count')}
      </div>
    {/if}
  </div>

  <!-- Filters and Sort -->
  {#if !loading && reviews.length > 0}
    <div class="card bg-base-100/70 backdrop-blur border border-base-300 shadow-sm">
      <div class="card-body py-4">
        <div class="flex flex-col lg:flex-row gap-4 items-start lg:items-center">
          <!-- Sort -->
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium opacity-70 flex items-center gap-1">
              <SortAsc class="w-4 h-4" />
              {translate('frontend/src/routes/pending-reviews/+page.svelte::sort_by')}:
            </span>
            <select class="select select-sm select-bordered" bind:value={sortBy}>
              <option value="newest">{translate('frontend/src/routes/pending-reviews/+page.svelte::sort_newest')}</option>
              <option value="oldest">{translate('frontend/src/routes/pending-reviews/+page.svelte::sort_oldest')}</option>
              <option value="student">{translate('frontend/src/routes/pending-reviews/+page.svelte::sort_student')}</option>
              <option value="assignment">{translate('frontend/src/routes/pending-reviews/+page.svelte::sort_assignment')}</option>
            </select>
          </div>

          <!-- Group -->
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium opacity-70">{translate('frontend/src/routes/pending-reviews/+page.svelte::group_by')}:</span>
            <select class="select select-sm select-bordered" bind:value={groupBy}>
              <option value="none">{translate('frontend/src/routes/pending-reviews/+page.svelte::group_none')}</option>
              <option value="student">{translate('frontend/src/routes/pending-reviews/+page.svelte::group_student')}</option>
              <option value="assignment">{translate('frontend/src/routes/pending-reviews/+page.svelte::group_assignment')}</option>
              <option value="class">{translate('frontend/src/routes/pending-reviews/+page.svelte::group_class')}</option>
            </select>
          </div>

          <div class="divider divider-horizontal hidden lg:flex"></div>

          <!-- Filter by Class -->
          {#if uniqueClasses.length > 1}
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium opacity-70 flex items-center gap-1">
                <Filter class="w-4 h-4" />
                {translate('frontend/src/routes/pending-reviews/+page.svelte::filter_class')}:
              </span>
              <select class="select select-sm select-bordered" bind:value={filterClass}>
                <option value="">{translate('frontend/src/routes/pending-reviews/+page.svelte::filter_all')}</option>
                {#each uniqueClasses as cls}
                  <option value={cls.id}>{cls.name}</option>
                {/each}
              </select>
            </div>
          {/if}

          <!-- Filter by Assignment -->
          {#if uniqueAssignments.length > 1}
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium opacity-70">{translate('frontend/src/routes/pending-reviews/+page.svelte::filter_assignment')}:</span>
              <select class="select select-sm select-bordered" bind:value={filterAssignment}>
                <option value="">{translate('frontend/src/routes/pending-reviews/+page.svelte::filter_all')}</option>
                {#each uniqueAssignments as a}
                  <option value={a.id}>{a.title}</option>
                {/each}
              </select>
            </div>
          {/if}
        </div>
      </div>
    </div>
  {/if}

  <!-- Loading State -->
  {#if loading}
    <div class="space-y-3">
      {#each Array(5) as _}
        <div class="card bg-base-100 shadow animate-pulse">
          <div class="card-body py-4">
            <div class="flex items-center gap-4">
              <div class="skeleton w-12 h-12 rounded-full"></div>
              <div class="flex-1 space-y-2">
                <div class="skeleton h-4 w-48"></div>
                <div class="skeleton h-3 w-32"></div>
              </div>
              <div class="skeleton h-8 w-24 rounded-lg"></div>
            </div>
          </div>
        </div>
      {/each}
    </div>

  <!-- Empty State -->
  {:else if reviews.length === 0}
    <div class="card bg-base-100/70 backdrop-blur border border-base-300 shadow-lg">
      <div class="card-body items-center text-center py-16">
        <div class="p-4 rounded-full bg-success/10 mb-4">
          <CheckCircle2 class="w-16 h-16 text-success" />
        </div>
        <h2 class="card-title text-2xl">{translate('frontend/src/routes/pending-reviews/+page.svelte::empty_title')}</h2>
        <p class="opacity-70 max-w-md">{translate('frontend/src/routes/pending-reviews/+page.svelte::empty_description')}</p>
      </div>
    </div>

  <!-- Grouped View -->
  {:else if grouped}
    <div class="space-y-6">
      {#each grouped as group}
        <div class="space-y-2">
          <h3 class="font-bold text-lg flex items-center gap-2 px-2">
            {#if groupBy === 'student'}
              <User class="w-5 h-5 text-primary" />
            {:else if groupBy === 'assignment'}
              <BookOpen class="w-5 h-5 text-primary" />
            {:else}
              <School class="w-5 h-5 text-primary" />
            {/if}
            {group.label}
            <span class="badge badge-ghost badge-sm">{group.items.length}</span>
          </h3>
          <div class="space-y-2">
            {#each group.items as review}
              <a
                href="/submissions/{review.id}"
                class="card bg-base-100/80 backdrop-blur border border-base-300 hover:border-primary/40 shadow-sm hover:shadow-md transition-all duration-200 group"
              >
                <div class="card-body py-4 px-5">
                  <div class="flex items-center gap-4">
                    <!-- Avatar -->
                    <div class="avatar placeholder">
                      <div class="w-12 h-12 rounded-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center">
                        <span class="text-lg font-bold text-primary">
                          {(review.student_name || review.student_email).charAt(0).toUpperCase()}
                        </span>
                      </div>
                    </div>

                    <!-- Info -->
                    <div class="flex-1 min-w-0">
                      <div class="flex items-center gap-2 flex-wrap">
                        <span class="font-semibold truncate">{review.student_name || review.student_email}</span>
                        <span class={`badge badge-sm ${statusColor(review.status)}`}>{submissionStatusLabel(review.status)}</span>
                        {#if review.attempt_number > 1}
                          <span class="badge badge-ghost badge-xs">#{review.attempt_number}</span>
                        {/if}
                      </div>
                      <div class="flex items-center gap-3 text-sm opacity-70 mt-1">
                        <span class="flex items-center gap-1 truncate">
                          <BookOpen class="w-3.5 h-3.5" />
                          {review.assignment_title}
                        </span>
                        <span class="flex items-center gap-1">
                          <Calendar class="w-3.5 h-3.5" />
                          {relativeTime(review.created_at)}
                        </span>
                      </div>
                    </div>

                    <!-- Action -->
                    <div class="flex items-center gap-2">
                      <span class="btn btn-sm btn-primary gap-1 group-hover:gap-2 transition-all">
                        {translate('frontend/src/routes/pending-reviews/+page.svelte::review_button')}
                        <ArrowRight class="w-4 h-4" />
                      </span>
                    </div>
                  </div>
                </div>
              </a>
            {/each}
          </div>
        </div>
      {/each}
    </div>

  <!-- Flat List View -->
  {:else}
    <div class="space-y-2">
      {#each filtered as review}
        <a
          href="/submissions/{review.id}"
          class="card bg-base-100/80 backdrop-blur border border-base-300 hover:border-primary/40 shadow-sm hover:shadow-md transition-all duration-200 group"
        >
          <div class="card-body py-4 px-5">
            <div class="flex items-center gap-4">
              <!-- Avatar -->
              <div class="avatar placeholder">
                <div class="w-12 h-12 rounded-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center">
                  <span class="text-lg font-bold text-primary">
                    {(review.student_name || review.student_email).charAt(0).toUpperCase()}
                  </span>
                </div>
              </div>

              <!-- Info -->
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="font-semibold truncate">{review.student_name || review.student_email}</span>
                  <span class={`badge badge-sm ${statusColor(review.status)}`}>{submissionStatusLabel(review.status)}</span>
                  {#if review.attempt_number > 1}
                    <span class="badge badge-ghost badge-xs">#{review.attempt_number}</span>
                  {/if}
                </div>
                <div class="flex items-center gap-3 text-sm opacity-70 mt-1 flex-wrap">
                  <span class="flex items-center gap-1 truncate">
                    <BookOpen class="w-3.5 h-3.5" />
                    {review.assignment_title}
                  </span>
                  <span class="flex items-center gap-1">
                    <School class="w-3.5 h-3.5" />
                    {review.class_name}
                  </span>
                  <span class="flex items-center gap-1">
                    <Calendar class="w-3.5 h-3.5" />
                    {relativeTime(review.created_at)}
                  </span>
                </div>
              </div>

              <!-- Action -->
              <div class="flex items-center gap-2">
                <span class="btn btn-sm btn-primary gap-1 group-hover:gap-2 transition-all">
                  {translate('frontend/src/routes/pending-reviews/+page.svelte::review_button')}
                  <ArrowRight class="w-4 h-4" />
                </span>
              </div>
            </div>
          </div>
        </a>
      {/each}
    </div>
  {/if}

  {#if err}
    <div class="alert alert-error">
      <span>{err}</span>
    </div>
  {/if}
</div>

<script lang="ts">
  import { onMount } from "svelte";
  import { page } from "$app/stores";
  import { apiJSON } from "$lib/api";
  import { formatDateTime } from "$lib/date";
  import { t } from "$lib/i18n";
  import {
    ChevronLeft,
    User,
    BookOpen,
    Award,
    Activity,
    Calendar,
    ArrowUpRight,
    CheckCircle2,
    Clock,
    AlertCircle,
    BarChart3,
    FlaskConical,
  } from "lucide-svelte";

  let classId = $page.params.id;
  let studentId = $page.params.sid;
  $: if ($page.params.id !== classId || $page.params.sid !== studentId) {
    classId = $page.params.id;
    studentId = $page.params.sid;
    load();
  }

  let cls: any = null;
  let student: any = null;
  let progress: any = null;
  let loading = true;
  let err = "";
  let recent: {
    id: string;
    assignment_id: string;
    title: string;
    created_at: string;
    status?: string;
    points?: number;
    passed_tests?: number;
    total_tests?: number;
  }[] = [];
  let testCounts: Record<string, number> = {};

  function bestFor(assignmentId: string) {
    const cell = (progress?.scores ?? []).find(
      (c: any) =>
        c.student_id === studentId && c.assignment_id === assignmentId,
    );
    return cell?.points ?? 0;
  }
  
  function bestPassedCount(assignmentId: string) {
    const cell = (progress?.scores ?? []).find(
      (c: any) =>
        c.student_id === studentId && c.assignment_id === assignmentId,
    );
    return cell?.passed_tests ?? 0;
  }

  function completedCount() {
    return (cls?.assignments ?? []).filter(
      (a: any) => bestFor(a.id) >= a.max_points,
    ).length;
  }

  function pointsEarned() {
    return (cls?.assignments ?? []).reduce(
      (sum: number, a: any) => sum + bestFor(a.id),
      0,
    );
  }

  function pointsTotal() {
    return (cls?.assignments ?? []).reduce(
      (sum: number, a: any) => sum + (a.max_points ?? 0),
      0,
    );
  }

  function badgeFor(a: any) {
    const best = bestFor(a.id);
    const complete = best >= a.max_points;
    const late = new Date(a.deadline) < new Date() && !complete;
    if (complete)
      return {
        text: t(
          "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::badge_completed",
        ),
        cls: "badge-success",
      };
    if (late)
      return {
        text: t(
          "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::badge_late",
        ),
        cls: "badge-error",
      };
    return {
      text: t(
        "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::badge_upcoming",
      ),
      cls: "badge-info",
    };
  }

  async function load() {
    loading = true;
    err = "";
    testCounts = {};
    try {
      const [clsData, userData, prog] = await Promise.all([
        apiJSON(`/api/classes/${classId}`),
        apiJSON(`/api/users/${studentId}`),
        apiJSON(`/api/classes/${classId}/progress`),
      ]);
      cls = clsData?.class ? { ...clsData.class, assignments: clsData.assignments || [] } : clsData;
      student = userData;
      progress = prog;

      // Build recent submissions list by querying each assignment and extracting this student's latest sub
      const assignments: any[] = cls?.assignments ?? [];
      const details = await Promise.all(
        assignments.map((a) =>
          apiJSON(`/api/assignments/${a.id}`).catch(() => null),
        ),
      );
      const rec: any[] = [];
      for (let i = 0; i < assignments.length; i++) {
        const a = assignments[i];
        const d = details[i];
        
        // Determine total tests for assignment (teacher view returns 'tests' array, student view 'tests_count')
        const tCount = d?.tests?.length ?? d?.tests_count ?? 0;
        testCounts[a.id] = tCount;

        const subs = (d?.submissions ?? []).filter(
          (s: any) => s.student_id === studentId,
        );
        if (subs.length) {
          const latest = [...subs].sort(
            (x: any, y: any) =>
              new Date(y.created_at).getTime() -
              new Date(x.created_at).getTime(),
          )[0];
          rec.push({
            id: latest.id,
            assignment_id: a.id,
            title: a.title,
            created_at: latest.created_at,
            status: latest.status,
            points: latest.override_points ?? latest.points,
            passed_tests: latest.passed_tests,
            total_tests: latest.total_tests ?? tCount, 
          });
        }
      }
      recent = rec
        .sort(
          (a, b) =>
            new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
        )
        .slice(0, 8);
    } catch (e: any) {
      err = e.message;
    }
    loading = false;
  }

  onMount(load);
</script>

<svelte:head>
  <title>{student?.name ? `${student.name} · ${cls?.name} | CodEdu` : 'Progress | CodEdu'}</title>
</svelte:head>

{#if loading}
  <div class="flex flex-col items-center justify-center min-h-[400px] gap-4">
    <span class="loading loading-spinner loading-lg text-primary"></span>
    <p class="text-base-content/60 animate-pulse">
      {t(
        "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::loading_message",
      )}
    </p>
  </div>
{:else if err}
  <div class="alert alert-error shadow-lg rounded-xl">
    <AlertCircle class="w-6 h-6" />
    <span>{err}</span>
  </div>
{:else}
  <div class="flex flex-col gap-8 pb-12">
    <!-- Header & Navigation -->
    <div class="flex flex-col gap-6">
      <div class="flex items-center gap-4">
        <a
          href={`/classes/${classId}/progress`}
          class="btn btn-ghost btn-sm rounded-lg gap-2"
        >
          <ChevronLeft class="w-4 h-4" />
          {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::back_to_list")}
        </a>
      </div>

      <div class="relative overflow-hidden rounded-3xl p-8 lg:p-10 border border-primary/10 shadow-xl bg-gradient-to-br from-primary/10 via-base-100 to-secondary/10">
        <div class="absolute top-0 right-0 -m-8 w-64 h-64 bg-primary/10 rounded-full blur-3xl opacity-50"></div>
        <div class="absolute bottom-0 left-0 -m-8 w-64 h-64 bg-secondary/10 rounded-full blur-3xl opacity-50"></div>

        <div class="relative flex flex-col md:flex-row items-center gap-8">
          <div class="avatar placeholder">
            <div class="bg-primary text-primary-content rounded-full w-24 h-24 sm:w-32 sm:h-32 shadow-2xl ring-4 ring-base-100">
              {#if student?.avatar}
                <img src={student.avatar} alt={student.name} />
              {:else}
                <span class="text-4xl font-bold">{(student?.name ?? student?.email ?? "?")[0].toUpperCase()}</span>
              {/if}
            </div>
          </div>

          <div class="flex-1 text-center md:text-left space-y-2">
            <h1 class="text-3xl sm:text-4xl font-bold tracking-tight text-base-content flex flex-wrap items-center justify-center md:justify-start gap-x-3">
              <span>{student?.name ?? student?.email}</span>
              <span class="text-primary/40 font-light">/</span>
              <span class="opacity-50 text-2xl sm:text-3xl">{cls?.name}</span>
            </h1>
            <div class="flex flex-wrap items-center justify-center md:justify-start gap-4 text-base-content/70">
              <div class="flex items-center gap-2 px-3 py-1 bg-base-200/50 rounded-full text-sm backdrop-blur-sm border border-base-content/5">
                <BookOpen class="w-4 h-4 text-primary" />
                <span>{cls?.name}</span>
              </div>
              <div class="flex items-center gap-2 px-3 py-1 bg-base-200/50 rounded-full text-sm backdrop-blur-sm border border-base-content/5">
                <User class="w-4 h-4 text-secondary" />
                <span>{student?.email}</span>
              </div>
            </div>
          </div>

          <div class="flex gap-3">
             <a href={`/messages/${studentId}`} class="btn btn-primary rounded-xl shadow-lg shadow-primary/20">
               {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::message_student")}
             </a>
          </div>
        </div>
      </div>
    </div>

    <!-- Quick Stats -->
    <section class="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-4">
      <div class="card card-elevated p-6 relative overflow-hidden group">
        <div class="absolute top-0 right-0 p-4 opacity-10 transition-transform group-hover:scale-110">
          <Award class="w-12 h-12" />
        </div>
        <div class="flex flex-col gap-1">
          <span class="text-xs font-bold uppercase tracking-wider text-base-content/50">
            {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::assignments_card_title")}
          </span>
          <div class="flex items-baseline gap-2">
            <span class="text-3xl font-bold text-primary">{completedCount()}</span>
            <span class="text-base-content/40 font-medium">/ {cls?.assignments?.length ?? 0}</span>
          </div>
          <progress
            class="progress progress-primary w-full h-2 mt-2"
            value={completedCount()}
            max={cls?.assignments?.length ?? 0}
          ></progress>
        </div>
      </div>

      <div class="card card-elevated p-6 relative overflow-hidden group">
        <div class="absolute top-0 right-0 p-4 opacity-10 transition-transform group-hover:scale-110">
          <BarChart3 class="w-12 h-12" />
        </div>
        <div class="flex flex-col gap-1">
          <span class="text-xs font-bold uppercase tracking-wider text-base-content/50">
            {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::points_card_title")}
          </span>
          <div class="flex items-baseline gap-2">
            <span class="text-3xl font-bold text-secondary">{pointsEarned()}</span>
            <span class="text-base-content/40 font-medium">/ {pointsTotal()}</span>
          </div>
          <progress
            class="progress progress-secondary w-full h-2 mt-2"
            value={pointsEarned()}
            max={pointsTotal()}
          ></progress>
        </div>
      </div>

      <div class="card card-elevated p-6 relative overflow-hidden group">
        <div class="absolute top-0 right-0 p-4 opacity-10 transition-transform group-hover:scale-110">
          <Activity class="w-12 h-12" />
        </div>
        <div class="flex flex-col gap-1">
          <span class="text-xs font-bold uppercase tracking-wider text-base-content/50">
             {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::recent_card_title")}
          </span>
          <div class="flex items-baseline gap-2">
            <span class="text-3xl font-bold text-accent">{recent.length}</span>
            <span class="text-base-content/40 font-medium">{t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::submissions_label")}</span>
          </div>
          <div class="text-xs text-base-content/60 mt-2 flex items-center gap-1">
            <Clock class="w-3 h-3" />
            {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::last_8_submissions")}
          </div>
        </div>
      </div>

      <div class="card card-elevated p-6 relative overflow-hidden group bg-gradient-to-br from-primary/5 to-secondary/5">
        <div class="absolute top-0 right-0 p-4 opacity-10 transition-transform group-hover:scale-110">
           <Calendar class="w-12 h-12" />
        </div>
        <div class="flex flex-col gap-1">
          <span class="text-xs font-bold uppercase tracking-wider text-base-content/50">
            {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::completion_rate")}
          </span>
          <div class="flex items-baseline gap-2">
            <span class="text-3xl font-bold">
              {cls?.assignments?.length ? Math.round((completedCount() / cls.assignments.length) * 100) : 0}%
            </span>
          </div>
          <div class="text-xs text-base-content/60 mt-2">
            {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::overall_performance")}
          </div>
        </div>
      </div>
    </section>

    <!-- Content Grid -->
    <div class="grid gap-8 lg:grid-cols-3">
      <!-- Main Content: Assignments -->
      <section class="lg:col-span-2 space-y-6">
        <div class="flex items-center justify-between mb-2">
          <h2 class="text-xl font-bold flex items-center gap-2">
            <BookOpen class="w-5 h-5 text-primary" />
            {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::assignments_section_title")}
          </h2>
        </div>

        <div class="space-y-4">
          {#each cls?.assignments ?? [] as a}
            <a
              href={`/assignments/${a.id}`}
              class="group block no-underline"
            >
              <div class="card card-elevated p-5 transition-all duration-300 hover:scale-[1.01] hover:shadow-xl hover:border-primary/20 bg-base-100/40 backdrop-blur-sm">
                <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-6">
                  <div class="flex items-center gap-4 min-w-0">
                    <div class={`w-12 h-12 rounded-xl flex items-center justify-center shrink-0 shadow-inner ${bestFor(a.id) >= a.max_points ? 'bg-success/10 text-success' : 'bg-base-200 text-base-content/40'}`}>
                      {#if bestFor(a.id) >= a.max_points}
                        <CheckCircle2 class="w-6 h-6" />
                      {:else}
                        <BookOpen class="w-6 h-6" />
                      {/if}
                    </div>
                    <div class="min-w-0">
                      <div class="font-bold text-lg group-hover:text-primary transition-colors truncate">
                        {a.title}
                      </div>
                      <div class="flex items-center gap-3 text-sm text-base-content/60 mt-1">
                        <span class="flex items-center gap-1">
                          <Clock class="w-3.5 h-3.5" />
                          {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::due_date_prefix")} {formatDateTime(a.deadline)}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div class="flex flex-wrap items-center gap-6 shrink-0 ml-16 sm:ml-0">
                     {#if (testCounts[a.id] ?? 0) > 0}
                       <div class="flex flex-col items-end gap-1.5 min-w-[80px]">
                            <div class="flex items-center gap-1.5 text-xs font-semibold text-base-content/70" title="Tests Passed">
                                <FlaskConical class="w-3.5 h-3.5" />
                                <span>{bestPassedCount(a.id)}<span class="opacity-40">/</span>{testCounts[a.id]}</span>
                            </div>
                            <progress class="progress progress-accent w-20 h-1" value={bestPassedCount(a.id)} max={testCounts[a.id]}></progress>
                       </div>
                     {/if}

                    <div class="flex flex-col items-end gap-1.5 min-w-[120px]">
                       <div class="flex items-center gap-2 text-sm font-medium">
                        <span class={bestFor(a.id) >= a.max_points ? 'text-success' : 'text-base-content'}>
                          {bestFor(a.id)}
                        </span>
                        <span class="text-base-content/30">/</span>
                        <span>{a.max_points} pts</span>
                      </div>
                      <progress
                        class={`progress w-24 h-1.5 ${bestFor(a.id) >= a.max_points ? 'progress-success' : 'progress-primary'}`}
                        value={Math.min(bestFor(a.id), a.max_points)}
                        max={a.max_points}
                      ></progress>
                    </div>

                    <div class="flex items-center gap-3">
                      <span class={`badge badge-lg rounded-lg border-none px-4 py-3 font-semibold ${badgeFor(a).cls === 'badge-success' ? 'bg-success/10 text-success' : badgeFor(a).cls === 'badge-error' ? 'bg-error/10 text-error' : 'bg-info/10 text-info'}`}>
                        {badgeFor(a).text}
                      </span>
                      <ArrowUpRight class="w-5 h-5 opacity-0 group-hover:opacity-40 transition-all -translate-x-2 group-hover:translate-x-0" />
                    </div>
                  </div>
                </div>
              </div>
            </a>
          {:else}
            <div class="card card-elevated p-12 text-center opacity-60 italic">
               <BookOpen class="w-12 h-12 mx-auto mb-4 opacity-20" />
               {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::no_assignments_message")}
            </div>
          {/each}
        </div>
      </section>

      <!-- Sidebar: Recent Submissions -->
      <aside class="space-y-6">
        <div class="card card-elevated p-6 space-y-6 bg-base-100/60 backdrop-blur-md sticky top-6 border-primary/5">
          <div class="flex items-center justify-between">
            <h3 class="text-lg font-bold flex items-center gap-2">
              <Activity class="w-5 h-5 text-secondary" />
              {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::recent_submissions_title")}
            </h3>
          </div>

          <div class="relative">
            <!-- Timeline Line -->
            <div class="absolute left-3.5 top-2 bottom-2 w-0.5 bg-base-300/50"></div>

            <ul class="space-y-6 relative">
              {#each recent as r}
                <li class="relative pl-10 group">
                  <div class="absolute left-0 top-1.5 w-7 h-7 rounded-lg bg-base-100 border-2 border-primary/30 flex items-center justify-center transition-transform group-hover:scale-110 z-10 shadow-sm">
                    <div class="w-2 h-2 rounded-full bg-primary animate-pulse"></div>
                  </div>

                  <a
                    href={`/submissions/${r.id}`}
                    class="block space-y-1 hover:no-underline"
                  >
                    <div class="text-sm font-bold text-base-content group-hover:text-primary transition-colors line-clamp-1">
                      {r.title}
                    </div>
                    <div class="flex items-center gap-2 text-[11px] uppercase tracking-wider text-base-content/40 font-semibold">
                      <Clock class="w-3 h-3" />
                      {formatDateTime(r.created_at)}
                    </div>
                    <div class="flex items-center gap-2 mt-1">
                        {#if r.points !== undefined}
                            <span class="badge badge-sm rounded-md bg-secondary/10 text-secondary border-none font-bold">
                            {r.points} pts
                            </span>
                        {/if}
                        {#if (r.total_tests ?? 0) > 0}
                            <span class="badge badge-sm rounded-md bg-accent/10 text-accent border-none font-bold flex items-center gap-1">
                                <FlaskConical class="w-3 h-3" />
                                {r.passed_tests ?? 0}/{r.total_tests}
                            </span>
                        {/if}
                    </div>
                  </a>
                </li>
              {:else}
                <li class="text-center py-8 text-sm opacity-50 italic">
                  <Activity class="w-8 h-8 mx-auto mb-2 opacity-20" />
                  {t("frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::no_submissions_message")}
                </li>
              {/each}
            </ul>
          </div>
        </div>
      </aside>
    </div>
  </div>
{/if}

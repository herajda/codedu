<script lang="ts">
  import { onMount } from "svelte";
  import { page } from "$app/stores";
  import { apiJSON } from "$lib/api";
  import { formatDateTime } from "$lib/date";
  import { t } from "$lib/i18n";

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
    assignment_id: string;
    title: string;
    created_at: string;
    status?: string;
    points?: number;
  }[] = [];

  function bestFor(assignmentId: string) {
    const cell = (progress?.scores ?? []).find(
      (c: any) =>
        c.student_id === studentId && c.assignment_id === assignmentId,
    );
    return cell?.points ?? 0;
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
    try {
      const [clsData, userData, prog] = await Promise.all([
        apiJSON(`/api/classes/${classId}`),
        apiJSON(`/api/users/${studentId}`),
        apiJSON(`/api/classes/${classId}/progress`),
      ]);
      cls = clsData;
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
            assignment_id: a.id,
            title: a.title,
            created_at: latest.created_at,
            status: latest.status,
            points: latest.override_points ?? latest.points,
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

{#if loading}
  <p>
    {t(
      "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::loading_message",
    )}
  </p>
{:else if err}
  <p class="text-error">{err}</p>
{:else}
  <div class="flex items-start justify-between gap-3 mb-4 flex-wrap">
    <div>
      <h1 class="text-2xl font-semibold">
        {student?.name ?? student?.email}
        {t(
          "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::student_progress_title",
        )}
      </h1>
      <p class="opacity-70 text-sm">
        {t(
          "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::class_name_label",
        )}
        {cls?.name}
      </p>
    </div>
  </div>

  <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-6">
    <div class="card-elevated p-4">
      <div class="text-xs uppercase opacity-70">
        {t(
          "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::assignments_card_title",
        )}
      </div>
      <div class="text-xl font-semibold">
        {completedCount()}/{cls?.assignments?.length ?? 0}
      </div>
    </div>
    <div class="card-elevated p-4">
      <div class="text-xs uppercase opacity-70">
        {t(
          "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::points_card_title",
        )}
      </div>
      <div class="text-xl font-semibold">{pointsEarned()}/{pointsTotal()}</div>
    </div>
    <div class="card-elevated p-4">
      <div class="text-xs uppercase opacity-70">
        {t(
          "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::recent_card_title",
        )}
      </div>
      <div class="text-xl font-semibold">{recent.length}</div>
      <div class="text-xs opacity-70">
        {t(
          "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::submissions_label",
        )}
      </div>
    </div>
  </section>

  <div class="grid gap-6 lg:col-span-3">
    <section class="lg:col-span-2">
      <div class="card-elevated p-5">
        <h2 class="font-semibold mb-3">
          {t(
            "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::assignments_section_title",
          )}
        </h2>
        <ul class="space-y-3">
          {#each cls?.assignments ?? [] as a}
            <li>
              <a
                href={`/assignments/${a.id}`}
                class="block no-underline text-current"
              >
                <div class="card-elevated p-4 hover:shadow-md transition">
                  <div class="flex items-center justify-between gap-4">
                    <div class="min-w-0">
                      <div class="font-medium truncate">{a.title}</div>
                      <div class="text-sm opacity-70 truncate">
                        {t(
                          "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::due_date_prefix",
                        )}
                        {formatDateTime(a.deadline)}
                      </div>
                    </div>
                    <div class="flex items-center gap-3 shrink-0">
                      <div class="flex items-center gap-2">
                        <progress
                          class="progress progress-primary w-20 sm:w-24"
                          value={Math.min(bestFor(a.id), a.max_points)}
                          max={a.max_points}
                        ></progress>
                        <span class="text-sm whitespace-nowrap"
                          >{bestFor(a.id)}/{a.max_points}</span
                        >
                      </div>
                      <span class={`badge ${badgeFor(a).cls}`}
                        >{badgeFor(a).text}</span
                      >
                    </div>
                  </div>
                </div>
              </a>
            </li>
          {/each}
          {#if !cls?.assignments?.length}
            <li class="text-sm opacity-70">
              {t(
                "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::no_assignments_message",
              )}
            </li>
          {/if}
        </ul>
      </div>
    </section>

    <aside class="space-y-6">
      <div class="card-elevated p-5">
        <h3 class="font-semibold mb-3">
          {t(
            "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::recent_submissions_title",
          )}
        </h3>
        <ul class="space-y-2">
          {#each recent as r}
            <li class="flex items-center justify-between text-sm">
              <a href={`/assignments/${r.assignment_id}`} class="truncate"
                >{r.title}</a
              >
              <span class="opacity-70 whitespace-nowrap"
                >{formatDateTime(r.created_at)}</span
              >
            </li>
          {/each}
          {#if !recent.length}
            <li class="text-sm opacity-70">
              {t(
                "frontend/src/routes/classes/[id]/progress/[sid]/+page.svelte::no_submissions_message",
              )}
            </li>
          {/if}
        </ul>
      </div>
    </aside>
  </div>
{/if}

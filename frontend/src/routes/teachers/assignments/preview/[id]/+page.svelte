<script lang="ts">
  // @ts-nocheck
  import { onMount } from "svelte";
  import { page } from "$app/stores";
  import { goto } from "$app/navigation";
  import { apiJSON } from "$lib/api";
  import { marked } from "marked";
  import DOMPurify from "dompurify";
  import { formatDateTime } from "$lib/date";
  import CustomSelect from "$lib/components/CustomSelect.svelte";
  import { t, translator } from "$lib/i18n";
  import { FlaskConical, Code, Clock, Scale, Eye, Users } from "lucide-svelte";
  import { extractMethodFromUnittest } from "$lib/unittests";

  let translate;
  $: translate = $translator;

  let id = $page.params.id;
  $: id = $page.params.id;

  // Data
  let assignment: any = null;
  let tests: any[] = [];
  let testsCount = 0;
  let loading = true;
  let err = "";

  let safeDesc = "";
  let activeTab: "overview" | "tests" = "overview";

  // Copy (import) to class state
  let copyDialog: HTMLDialogDialogElement;
  let myClasses: any[] = [];
  $: copyClassOptions = myClasses.map((c) => ({ value: String(c.id), label: c.name }));
  let copyClassId: string | null = null;
  let copyErr = "";
  let copyLoading = false;

  async function openCopyToClass() {
    copyErr = "";
    copyClassId = null;
    copyLoading = true;
    try {
      myClasses = await apiJSON("/api/classes");
    } catch (e: any) {
      copyErr = e.message;
    }
    copyLoading = false;
    copyDialog?.showModal();
  }

  async function doCopyToClass() {
    if (!copyClassId) {
      copyErr = t(
        "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::choose_class_error",
      );
      return;
    } // Localized
    try {
      const res = await apiJSON(
        `/api/classes/${copyClassId}/assignments/import`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ source_assignment_id: assignment.id }),
        },
      );
      copyDialog?.close();
      if (res?.assignment_id) {
        await goto(`/assignments/${res.assignment_id}?new=1`);
      }
    } catch (e: any) {
      copyErr = e.message;
    }
  }

  function statusColor(s: string) {
    if (s === "completed") return "badge-success";
    if (s === "running") return "badge-info";
    if (s === "failed") return "badge-error";
    if (s === "passed") return "badge-success";
    if (s === "wrong_output") return "badge-error";
    if (s === "runtime_error") return "badge-error";
    if (s === "illegal_tool_use") return "badge-error";
    if (s === "time_limit_exceeded" || s === "memory_limit_exceeded")
      return "badge-warning";
    return "";
  }

  function policyLabel(policy: string) {
    if (policy === "all_or_nothing")
      return t(
        "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::all_or_nothing",
      ); // Localized
    if (policy === "weighted")
      return t(
        "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::weighted",
      ); // Localized
    return policy;
  }

  async function load() {
    loading = true;
    err = "";
    try {
      const data = await apiJSON(`/api/assignments/${id}`);
      assignment = data.assignment;
      tests = data.tests ?? [];
      testsCount = Array.isArray(tests) ? tests.length : 0;
      try {
        safeDesc = DOMPurify.sanitize(
          (marked.parse(assignment.description) as string) || "",
        );
      } catch {
        safeDesc = "";
      }
    } catch (e: any) {
      err = e.message;
    }
    loading = false;
    // Check for tab param
    if ($page.url.searchParams.get("tab") === "tests") {
      activeTab = "tests";
    }
  }

  onMount(load);
</script>

{#if loading}
  <div class="p-6"><span class="loading loading-dots"></span></div>
{:else if err}
  <div class="p-6 text-error">{err}</div>
{:else if assignment}
  <!-- Hero header (mirrors main assignment style, without deadlines/actions) -->
  <section
    class="relative overflow-hidden mb-6 rounded-2xl border border-base-300/60 bg-gradient-to-br from-primary/10 to-secondary/10 p-0"
  >
    <div
      class="flex flex-col sm:flex-row items-stretch sm:items-center gap-0 sm:gap-6"
    >
      <div class="flex-1 p-6">
        <div class="flex items-center justify-between gap-3">
          <h1 class="text-2xl sm:text-3xl font-semibold tracking-tight">
            {assignment.title}
          </h1>
          <div class="flex items-center gap-2">
            <button class="btn btn-sm" on:click={() => history.back()}
              ><i class="fa-solid fa-arrow-left mr-2"></i>{translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::back",
              )}</button
            >
            <button class="btn btn-primary btn-sm" on:click={openCopyToClass}
              ><i class="fa-solid fa-copy mr-2"></i>{translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::add_to_my_class",
              )}</button
            >
          </div>
        </div>
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <span class="badge badge-ghost"
            >{translate(
              "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::max_label",
            )}{assignment.max_points}
            {translate(
              "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::points_abbr",
            )}</span
          >
          <span class="badge badge-ghost"
            >{policyLabel(assignment.grading_policy)}</span
          >
          {#if assignment.manual_review}
            <span class="badge badge-info"
              >{translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::manual_review_badge",
              )}</span
            >
          {/if}
          {#if assignment.published}
            <span class="badge badge-success"
              >{translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::published",
              )}</span
            >
          {:else}
            <span class="badge badge-warning"
              >{translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::draft",
              )}</span
            >
          {/if}
        </div>
        <div class="mt-4 text-xs opacity-70">
          {translate(
            "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::readonly_preview",
          )}
        </div>
      </div>
    </div>
  </section>

  <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
    <div class="lg:col-span-12">
      <div class="tabs tabs-boxed w-full mb-4">
        <button
          class="tab {activeTab === 'overview' ? 'tab-active' : ''}"
          on:click={() => (activeTab = "overview")}
          >{translate(
            "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::overview_tab",
          )}</button
        >
        <button
          class="tab {activeTab === 'tests' ? 'tab-active' : ''}"
          on:click={() => (activeTab = "tests")}
          >{translate(
            "frontend/src/routes/teachers/assignments/+page.svelte::tests_button_label",
          )} ( {testsCount} )</button
        >
      </div>

      <section
        id="pv-overview"
        class="card-elevated p-6 space-y-4"
        hidden={activeTab !== "overview"}
      >
        <div class="markdown assignment-description">{@html safeDesc}</div>
        <div class="grid sm:grid-cols-3 gap-3">
          <div class="stat bg-base-100 rounded-xl border border-base-300/60">
            <div class="stat-title">
              {translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::max_points_stat_title",
              )}
            </div>
            <div class="stat-value text-lg">{assignment.max_points}</div>
            <div class="stat-desc">
              {policyLabel(assignment.grading_policy)}
            </div>
          </div>
          <div class="stat bg-base-100 rounded-xl border border-base-300/60">
            <div class="stat-title">
              {translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::traceback_visible_stat_title",
              )}
            </div>
            <div class="stat-value text-lg">
              {assignment.show_traceback
                ? translate(
                    "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::yes",
                  )
                : translate(
                    "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::no",
                  )}
            </div>
            <div class="stat-desc">
              {translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::student_error_output_desc",
              )}
            </div>
          </div>
          <div class="stat bg-base-100 rounded-xl border border-base-300/60">
            <div class="stat-title">
              {translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::manual_review_stat_title",
              )}
            </div>
            <div class="stat-value text-lg">
              {assignment.manual_review
                ? translate(
                    "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::yes",
                  )
                : translate(
                    "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::no",
                  )}
            </div>
            <div class="stat-desc">
              {translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::grading_method_desc",
              )}
            </div>
          </div>
        </div>
      </section>

      <section
        id="pv-tests"
        class="card-elevated p-6 space-y-4"
        hidden={activeTab !== "tests"}
      >
        <div class="flex items-center justify-between">
          <h3 class="font-semibold text-lg">
            {translate(
              "frontend/src/routes/teachers/assignments/+page.svelte::tests_button_label",
            )}
          </h3>
          <span class="badge badge-ghost">{tests.length}</span>
        </div>
        <div class="grid gap-3">
          {#each tests as t, i}
            {@const mode =
              t.execution_mode ??
              (t.unittest_name
                ? "unittest"
                : t.function_name
                  ? "function"
                  : "stdin_stdout")}
            {@const utName = (t.unittest_name ?? "").trim()}
            {@const hasUnittestCode =
              typeof t.unittest_code === "string" &&
              t.unittest_code.trim().length > 0}
            <div class="rounded-xl border border-base-300/60 p-3 space-y-2">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2 font-semibold">
                  <span class="opacity-70">#{i + 1}</span>
                  {#if mode === "function"}
                    <span class="badge badge-info gap-1"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::function",
                      )}</span
                    >
                    {#if t.function_name}
                      <span class="badge badge-outline ml-1"
                        >{t.function_name}</span
                      >
                    {/if}
                  {:else if mode === "unittest"}
                    <span class="badge badge-primary gap-1"
                      ><FlaskConical size={14} />
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest",
                      )}</span
                    >
                    {#if utName}
                      <span class="badge badge-outline ml-1">{utName}</span>
                    {:else}
                      <span class="badge badge-outline ml-1 opacity-70"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::unnamed_test",
                        )}</span
                      >
                    {/if}
                  {:else}
                    <span class="badge badge-secondary gap-1"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::io",
                      )}</span
                    >
                  {/if}
                </div>
              </div>
              {#if mode === "function"}
                <div class="grid gap-2 md:grid-cols-2">
                  <label class="form-control w-full space-y-1">
                    <span class="label-text"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_name",
                      )}</span
                    >
                    <input
                      class="input input-bordered w-full"
                      disabled
                      value={t.function_name}
                    />
                  </label>
                  <label class="form-control w-full space-y-1">
                    <span class="label-text"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_return_json",
                      )}</span
                    >
                    <textarea
                      class="textarea textarea-bordered w-full"
                      rows="2"
                      disabled
                      value={t.expected_return}
                    ></textarea>
                  </label>
                  <label class="form-control w-full space-y-1 md:col-span-1">
                    <span class="label-text"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::arguments_json_array",
                      )}</span
                    >
                    <textarea
                      class="textarea textarea-bordered w-full"
                      rows="2"
                      disabled
                      value={t.function_args}
                    ></textarea>
                  </label>
                  <label class="form-control w-full space-y-1 md:col-span-1">
                    <span class="label-text"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::keyword_args_json_object",
                      )}</span
                    >
                    <textarea
                      class="textarea textarea-bordered w-full"
                      rows="2"
                      disabled
                      value={t.function_kwargs}
                    ></textarea>
                  </label>
                </div>
              {:else if t.unittest_name}
                {#if hasUnittestCode}
                  <details class="mt-1" open>
                    <summary
                      class="cursor-pointer text-sm opacity-70 flex items-center gap-1"
                      ><Eye size={14} />
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::view_test_method_code",
                      )}</summary
                    >
                    <pre
                      class="mt-2 whitespace-pre-wrap text-xs opacity-80 p-2 rounded-lg bg-base-200">{extractMethodFromUnittest(
                        t.unittest_code,
                        t.unittest_name,
                      )}</pre>
                  </details>
                {/if}
              {:else}
                <div class="grid sm:grid-cols-2 gap-2">
                  <label class="form-control w-full space-y-1">
                    <span class="label-text"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::input",
                      )}</span
                    >
                    <textarea
                      class="textarea textarea-bordered w-full"
                      rows="3"
                      disabled
                      value={t.stdin}
                    ></textarea>
                  </label>
                  <label class="form-control w-full space-y-1">
                    <span class="label-text"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_output",
                      )}</span
                    >
                    <textarea
                      class="textarea textarea-bordered w-full"
                      rows="3"
                      disabled
                      value={t.expected_stdout}
                    ></textarea>
                  </label>
                </div>
              {/if}
              <div
                class="grid gap-2"
                class:sm:grid-cols-2={assignment?.grading_policy === "weighted"}
              >
                <label class="form-control w-full space-y-1">
                  <span class="label-text flex items-center gap-1"
                    ><Clock size={14} />
                    <span
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::time_limit_s",
                      )}</span
                    ></span
                  >
                  <input
                    class="input input-bordered w-full"
                    disabled
                    value={t.time_limit_sec}
                  />
                </label>
                {#if assignment?.grading_policy === "weighted"}
                  <label class="form-control w-full space-y-1">
                    <span class="label-text flex items-center gap-1"
                      ><Scale size={14} />
                      <span
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::points",
                        )}</span
                      ></span
                    >
                    <input
                      class="input input-bordered w-full"
                      disabled
                      value={t.weight}
                    />
                  </label>
                {/if}
              </div>
            </div>
          {/each}
          {#if !(tests && tests.length)}<p>
              <i
                >{translate(
                  "frontend/src/routes/teachers/assignments/+page.svelte::no_tests_label",
                )}</i
              >
            </p>{/if}
        </div>
      </section>
    </div>

    <!-- Right side: Optional details (kept minimal for preview) -->
    <aside class="lg:col-span-4 space-y-4">
      <div class="card-elevated p-5">
        <h3 class="font-semibold mb-2">
          {translate(
            "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::details_heading",
          )}
        </h3>
        <ul class="text-sm space-y-1">
          <li>
            <b
              >{translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::template_label",
              )}</b
            >
            {assignment.template_path
              ? translate(
                  "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::present_status",
                )
              : translate(
                  "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::none_status",
                )}
          </li>
          <li>
            <b
              >{translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::created_label",
              )}</b
            >
            {formatDateTime(assignment.created_at)}
          </li>
          <li>
            <b
              >{translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::updated_label",
              )}</b
            >
            {formatDateTime(assignment.updated_at)}
          </li>
          <li>
            <b
              >{translate(
                "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::tests_label",
              )}</b
            >
            {tests.length}
          </li>
        </ul>
      </div>
    </aside>
  </div>
{/if}

<!-- Copy to class modal -->
<dialog bind:this={copyDialog} class="modal">
  <div class="modal-box w-11/12 max-w-md">
    <h3 class="font-bold mb-3">
      {translate(
        "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::add_to_my_class_modal_title",
      )}
    </h3>
    {#if copyLoading}
      <div class="py-4 text-center">
        <span class="loading loading-dots"></span>
      </div>
    {:else}
      <label class="label"
        ><span class="label-text"
          >{translate(
            "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::choose_class_label",
          )}</span
        ></label
      >
      <CustomSelect
        options={copyClassOptions}
        bind:value={copyClassId}
        placeholder={translate(
          "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::select_option_placeholder",
        )}
        icon={Users}
      />
      {#if copyErr}<p class="text-error mt-2">{copyErr}</p>{/if}
      <div class="modal-action">
        <form method="dialog">
          <button class="btn"
            >{translate(
              "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::cancel_button",
            )}</button
          >
        </form>
        <button class="btn btn-primary" on:click|preventDefault={doCopyToClass}
          >{translate(
            "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::add_button",
          )}</button
        >
      </div>
    {/if}
  </div>
  <form method="dialog" class="modal-backdrop">
    <button
      >{translate(
        "frontend/src/routes/teachers/assignments/preview/[id]/+page.svelte::close_button",
      )}</button
    >
  </form>
</dialog>

<style>
  @import "@fortawesome/fontawesome-free/css/all.min.css";
  .markdown :global(p) {
    margin: 0.5rem 0;
  }
  .markdown :global(code) {
    background: hsl(var(--b2));
    padding: 0.1rem 0.25rem;
    border-radius: 0.25rem;
  }
</style>

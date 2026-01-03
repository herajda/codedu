<script lang="ts">
  // @ts-nocheck
  import { onMount, onDestroy, tick } from "svelte";
  import { auth } from "$lib/auth";
  import { apiFetch, apiJSON } from "$lib/api";
  import { MarkdownEditor } from "$lib";
  import { marked } from "marked";
  import { formatDateTime } from "$lib/date";
  import DOMPurify from "dompurify";
  import { goto } from "$app/navigation";
  import { page } from "$app/stores";
  import ConfirmModal from "$lib/components/ConfirmModal.svelte";
  import CustomSelect from "$lib/components/CustomSelect.svelte";
  import StylishInput from "$lib/components/StylishInput.svelte";
  import { DeadlinePicker } from "$lib";
  import { strictnessGuidance } from "$lib/llmStrictness";
  import { t, translator } from "$lib/i18n";
  import { submissionStatusLabel } from "$lib/status";
  import {
    Calendar,
    Clock,
    Trophy,
    GraduationCap,
    FileCode,
    Users,
    Activity,
    CheckCircle2,
    AlertTriangle,
    Info,
    ChevronDown,
    ChevronUp,
    ChevronRight,
    LayoutDashboard,
    ListTodo,
    List,
    LayoutList,
    Lightbulb,
    History,
    Settings2,
    FlaskConical,
    Send,
    Eye,
    Trash2,
    Edit3,
    ArrowRight,
    Search,
    Filter,
    Table,
    Download,
    Upload,
    Sparkles,
    ShieldCheck,
    Briefcase,
    Globe,
    ExternalLink,
    Save,
    Plus,
    X,
    Cpu,
    FileUp,
    Check,
    FileText,
    AlertCircle
  } from "lucide-svelte";


  $: id = $page.params.id;
  let role = "";
  $: role = $auth?.role ?? "";

  let assignment: any = null;
  // tests moved to standalone page
  let submissions: any[] = []; // student submissions
  let latestSub: any = null;
  let results: any[] = [];
  let esCtrl: { close: () => void } | null = null;
  let allSubs: any[] = []; // teacher view
  let teacherRuns: any[] = []; // persisted teacher submissions
  let students: any[] = []; // class roster for teacher
  let progress: any[] = []; // computed progress per student
  let overrides: any[] = []; // per-student deadline overrides (teacher view)
  let overrideMap: Record<string, any> = {};
  let teacherGroupSync: { has_clone?: boolean; needs_update?: boolean; clone_ids?: string[] } = {
    has_clone: false,
    needs_update: false,
    clone_ids: [],
  };
  let syncTGLoading = false;
  let expanded: number | null = null;
  let pointsEarned = 0;
  let done = false;
  let percent = 0;
  let testsPassed = 0;
  let testsPercent = 0;
  let testsCount = 0;
  let err = "";
  let subStats: Record<string, { passed: number; total: number }> = {};
  // removed test creation inputs (moved to tests page)
  let files: File[] = [];
  let isUploading = false;
  let uploadProgress = 0;
  let templateFile: File | null = null;
  type ProgrammingLanguage = "python" | "scratch";
  const pythonFileExt = ".py";
  const scratchFileExt = ".sb3";
  const defaultMaxSubmissionSizeMB = 10;
  let eProgrammingLanguage: ProgrammingLanguage = "python";

  let confirmModal: InstanceType<typeof ConfirmModal>;
  // removed unittest file input (moved to tests page)
  let submitDialog: HTMLDialogElement;
  let fileInput: HTMLInputElement | null = null;
  $: isScratchAssignment =
    (editing ? eProgrammingLanguage : assignment?.programming_language) ===
    "scratch";
  $: scratchModeView = normalizeScratchEvaluationMode(
    assignment?.scratch_evaluation_mode,
  );
  $: submissionExtension = isScratchAssignment ? scratchFileExt : pythonFileExt;
  $: submissionExtLabel = submissionExtension;

  function formatMB(bytes: number) {
    return Math.round((bytes / (1024 * 1024)) * 10) / 10;
  }

  function getSubmissionLimitMB() {
    return assignment?.max_submission_size_mb ?? defaultMaxSubmissionSizeMB;
  }

  function addSubmissionFiles(incoming: File[]) {
    const ext = submissionExtension.toLowerCase();
    const allowed = incoming.filter((f) =>
      f.name.toLowerCase().endsWith(ext),
    );
    if (!allowed.length) return;
    err = "";
    const maxMB = getSubmissionLimitMB();
    const maxBytes = maxMB * 1024 * 1024;
    if (isScratchAssignment) {
      const candidate = allowed[allowed.length - 1];
      if (candidate.size > maxBytes) {
        err = t(
          "frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_size_error",
          { size: formatMB(candidate.size), max: maxMB },
        );
        return;
      }
      files = [candidate];
      if (fileInput) fileInput.value = "";
      return;
    }
    const totalBytes = files.reduce((sum, f) => sum + f.size, 0);
    const newTotal = totalBytes + allowed.reduce((sum, f) => sum + f.size, 0);
    if (newTotal > maxBytes) {
      err = t(
        "frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_size_error",
        { size: formatMB(newTotal), max: maxMB },
      );
      return;
    }
    files = [...files, ...allowed];
  }
  // removed tests dialog (moved to tests page)
  $: percent = assignment
    ? Math.round((pointsEarned / assignment.max_points) * 100)
    : 0;
  $: testsPassed = results.filter((r: any) => r.status === "passed").length;
  $: testsPercent = results.length
    ? Math.round((testsPassed / results.length) * 100)
    : 0;
  $: allTestsFailed =
    Array.isArray(results) &&
    results.length > 0 &&
    results.every((r: any) => r.status !== "passed" && r.status !== "running");
  let editing = false;
  let eTitle = "",
    eDesc = "",
    eDeadline = "",
    ePoints = 0,
    ePolicy = "all_or_nothing",
    eMaxSubmissionSizeMB = defaultMaxSubmissionSizeMB,
    eShowTraceback = false,
    eShowTestDetails = false;
  let eManualReview = false;
  let eLLMInteractive = false;
  let eLLMFeedback = false;
  let eLLMHelpWhyFailed = false;
  let eLLMAutoAward = true;
  let eLLMScenarios = "";
  let eLLMStrictness: number = 50;
  let eLLMRubric = "";
  let eLLMTeacherBaseline = "";
  type ScratchEvaluationMode = "manual" | "semi_automatic" | "automatic";
  type ScratchCriterionInput = { id: string; text: string; points: string };
  let scratchEvaluationMode: ScratchEvaluationMode = "manual";
  let scratchCriteria: ScratchCriterionInput[] = [];
  $: eLLMStrictnessMessage = strictnessGuidance(eLLMStrictness);
  let eSecondDeadline = "";
  // Enhanced date/time UX state (derived from the above strings)
  let eDeadlineDate = "";
  let eDeadlineTime = "";
  let eSecondDeadlineDate = "";
  let eSecondDeadlineTime = "";
  const quickTimes = ["08:00", "12:00", "17:00", "23:59"];
  function timeLabel(t: string) {
    // Always show 24h format HH:mm in UI labels
    const [hh, mm] = (t || "").split(":");
    const h = String(parseInt(hh || "0", 10)).padStart(2, "0");
    const m = String(parseInt(mm || "0", 10)).padStart(2, "0");
    return `${h}:${m}`;
  }
  let eLatePenaltyRatio: number = 0.5;
  let showAdvancedOptions = false;
  let showAiOptions = false;
  let showRubric = false;
  const exampleScenario =
    '[{"name":"calc","steps":[{"send":"2 + 2","expect_after":"4"}]}]';
  let safeDesc = "";
  $: safeDesc = assignment
    ? DOMPurify.sanitize(marked.parse(assignment.description) as string)
    : "";

  function newScratchCriterion(text = "", points = ""): ScratchCriterionInput {
    return {
      id: `crit-${Date.now()}-${Math.random().toString(16).slice(2)}`,
      text,
      points,
    };
  }

  function parseScratchCriteria(
    raw: string | null | undefined,
  ): ScratchCriterionInput[] {
    const trimmed = (raw ?? "").trim();
    if (!trimmed) return [newScratchCriterion()];
    try {
      const parsed = JSON.parse(trimmed);
      if (Array.isArray(parsed)) {
        if (parsed.every((item) => typeof item === "string")) {
          const items = parsed
            .map((text) => String(text).trim())
            .filter(Boolean)
            .map((text) => newScratchCriterion(text));
          return items.length ? items : [newScratchCriterion()];
        }
        const items = parsed
          .map((item) => {
            if (!item || typeof item !== "object") return null;
            const text =
              typeof item.text === "string"
                ? item.text
                : typeof item.item === "string"
                  ? item.item
                  : "";
            const points =
              typeof item.points === "number"
                ? String(item.points)
                : typeof item.points === "string"
                  ? item.points
                  : "";
            if (!text.trim()) return null;
            return newScratchCriterion(text, points);
          })
          .filter(Boolean) as ScratchCriterionInput[];
        return items.length ? items : [newScratchCriterion()];
      }
    } catch {}

    const lines = trimmed
      .split("\n")
      .map((line) => line.trim())
      .map((line) => line.replace(/^[-*]\s*/, "").trim())
      .filter(Boolean);
    if (!lines.length) return [newScratchCriterion()];
    return lines.map((text) => newScratchCriterion(text));
  }

  function normalizeScratchEvaluationMode(
    raw: string | null | undefined,
  ): ScratchEvaluationMode {
    if (raw === "automatic") return "automatic";
    if (raw === "semi_automatic") return "semi_automatic";
    return "manual";
  }

  function normalizeScratchPoints(points: string): number | null {
    const value = Number(points);
    if (!Number.isFinite(value) || value <= 0) return null;
    return value;
  }

  function serializeScratchCriteria(
    list: ScratchCriterionInput[],
  ): string | null {
    const payload = list
      .map((item) => {
        const text = item.text.trim();
        if (!text) return null;
        const points = normalizeScratchPoints(item.points);
        return points === null ? { text } : { text, points };
      })
      .filter(Boolean);
    return payload.length ? JSON.stringify(payload) : null;
  }

  function addScratchCriterion() {
    scratchCriteria = [...scratchCriteria, newScratchCriterion()];
  }

  function removeScratchCriterion(id: string) {
    scratchCriteria = scratchCriteria.filter((item) => item.id !== id);
    if (!scratchCriteria.length) {
      scratchCriteria = [newScratchCriterion()];
    }
  }

  // Testing model selector (automatic | manual | ai)
  type TestMode = "automatic" | "manual" | "ai";
  let testMode: TestMode = "automatic";
  $: {
    if (eProgrammingLanguage === "scratch") {
      eManualReview = false;
      eLLMInteractive = false;
    } else if (testMode === "manual") {
      eManualReview = true;
      eLLMInteractive = false;
    } else if (testMode === "ai") {
      eManualReview = false;
      eLLMInteractive = true;
    } else {
      eManualReview = false;
      eLLMInteractive = false;
    }
  }

  // Auto-switch to automatic testing when weighted policy is selected
  $: {
    if (eProgrammingLanguage !== "scratch" && ePolicy === "weighted" && testMode !== "automatic") {
      testMode = "automatic";
      eManualReview = false;
      eLLMInteractive = false;
    }
  }
  let translate;
  $: translate = $translator;

  $: gradingPolicyOptions = [
    {
      value: "all_or_nothing",
      label: t("frontend/src/routes/assignments/[id]/+page.svelte::policyLabel_allOrNothing")
    },
    {
      value: "weighted",
      label: t("frontend/src/routes/assignments/[id]/+page.svelte::policyLabel_weighted"),
      disabled: !isScratchAssignment && (testMode === "manual" || testMode === "ai")
    }
  ];

  $: programmingLanguageOptions = [
    { value: "python", label: t("frontend/src/routes/assignments/[id]/+page.svelte::programming_language_python"), icon: "/python_logo.webp" },
    { value: "scratch", label: t("frontend/src/routes/assignments/[id]/+page.svelte::programming_language_scratch"), icon: "/scratch_logo.webp" }
  ];

  $: scratchEvaluationModeOptions = [
    { value: "manual", label: t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_manual") },
    { value: "semi_automatic", label: t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_semi") },
    { value: "automatic", label: t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_auto") }
  ];

  $: testModeOptions = [
    { value: "automatic", label: t("frontend/src/routes/assignments/[id]/+page.svelte::automatic_tests_option") },
    { 
      value: "manual", 
      label: t("frontend/src/routes/assignments/[id]/+page.svelte::manual_teacher_review_option"),
      disabled: ePolicy === "weighted"
    },
    { 
      value: "ai", 
      label: t("frontend/src/routes/assignments/[id]/+page.svelte::ai_testing_option"),
      disabled: ePolicy === "weighted"
    }
  ];

  // Enhanced UX state
  type TabKey =
    | "overview"
    | "submissions"
    | "results"
    | "instructor"
    | "teacher-runs";
  let activeTab: TabKey = "overview";
  let isDragging = false;

  // Teacher solution test-run state (modal in Teacher runs tab)
  let solFiles: File[] = [];
  let isSolDragging = false;
  let solLoading = false;
  let teacherRunDialog: HTMLDialogElement;
  let teacherFileInput: HTMLInputElement;

  function policyLabel(policy: string) {
    if (policy === "all_or_nothing")
      return t(
        "frontend/src/routes/assignments/[id]/+page.svelte::policyLabel_allOrNothing",
      );
    if (policy === "weighted")
      return t(
        "frontend/src/routes/assignments/[id]/+page.svelte::policyLabel_weighted",
      );
    return policy;
  }
  function scratchModeLabel(mode: ScratchEvaluationMode) {
    if (mode === "automatic")
      return t(
        "frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_auto",
      );
    if (mode === "semi_automatic")
      return t(
        "frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_semi",
      );
    return t(
      "frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_manual",
    );
  }
  function relativeToDeadline(deadline: string) {
    const now = new Date();
    const due = new Date(deadline);
    const diffMs = due.getTime() - now.getTime();
    const abs = Math.abs(diffMs);
    const mins = Math.round(abs / 60000);
    const hrs = Math.round(mins / 60);
    const days = Math.round(hrs / 24);

    if (abs < 60) {
      if (diffMs >= 0)
        return translate(
          mins === 1
            ? "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_future_minutes_singular"
            : "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_future_minutes_plural",
          { count: mins },
        );
      return translate(
        mins === 1
          ? "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_past_minutes_singular"
          : "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_past_minutes_plural",
        { count: mins },
      );
    }
    if (abs < 60 * 24) {
      if (diffMs >= 0)
        return translate(
          hrs === 1
            ? "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_future_hours_singular"
            : "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_future_hours_plural",
          { count: hrs },
        );
      return translate(
        hrs === 1
          ? "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_past_hours_singular"
          : "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_past_hours_plural",
        { count: hrs },
      );
    }
    if (diffMs >= 0)
      return translate(
        days === 1
          ? "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_future_days_singular"
          : "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_future_days_plural",
        { count: days },
      );
    return translate(
      days === 1
        ? "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_past_days_singular"
        : "frontend/src/routes/assignments/[id]/+page.svelte::relativeToDeadline_past_days_plural",
      { count: days },
    );
  }
  $: isOverdue = assignment
    ? new Date(assignment.deadline) < new Date()
    : false;
  $: isSecondDeadlineActive = assignment?.second_deadline
    ? new Date(assignment.second_deadline) > new Date()
    : false;
  $: timeUntilDeadline = assignment
    ? new Date(assignment.deadline).getTime() - Date.now()
    : 0;
  $: timeUntilSecondDeadline = assignment?.second_deadline
    ? new Date(assignment.second_deadline).getTime() - Date.now()
    : 0;
  $: deadlineSoon =
    timeUntilDeadline > 0 && timeUntilDeadline <= 24 * 60 * 60 * 1000;
  $: secondDeadlineSoon =
    timeUntilSecondDeadline > 0 &&
    timeUntilSecondDeadline <= 24 * 60 * 60 * 1000;
  $: deadlineBadgeClass =
    isOverdue && !(role === "student" && done) ? "badge-error" : "badge-ghost";
  $: deadlineLabel = assignment
    ? isOverdue
      ? role === "student" && done
        ? translate(
            "frontend/src/routes/assignments/[id]/+page.svelte::deadlineLabel_student_done",
            { relativeTime: relativeToDeadline(assignment.deadline) },
          )
        : translate(
            "frontend/src/routes/assignments/[id]/+page.svelte::deadlineLabel_overdue",
            { relativeTime: relativeToDeadline(assignment.deadline) },
          )
      : translate(
          "frontend/src/routes/assignments/[id]/+page.svelte::deadlineLabel_due",
          { relativeTime: relativeToDeadline(assignment.deadline) },
        )
    : "";
  $: secondDeadlineLabel = assignment?.second_deadline
    ? new Date(assignment.second_deadline) < new Date()
      ? translate(
          "frontend/src/routes/assignments/[id]/+page.svelte::secondDeadlineLabel_passed",
          { relativeTime: relativeToDeadline(assignment.second_deadline) },
        )
      : translate(
          "frontend/src/routes/assignments/[id]/+page.svelte::secondDeadlineLabel_active",
          { relativeTime: relativeToDeadline(assignment.second_deadline) },
        )
    : "";

  async function publish() {
    try {
      await apiFetch(`/api/assignments/${id}/publish`, { method: "PUT" });
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }

  async function syncTeachersGroup() {
    try {
      syncTGLoading = true;
      await apiFetch(`/api/assignments/${id}/sync-teachers-group`, {
        method: "POST",
      });
      await load();
    } catch (e: any) {
      err = e.message;
    } finally {
      syncTGLoading = false;
    }
  }

  async function load() {
    err = "";
    try {
      const data = await apiJSON(`/api/assignments/${id}`);
      assignment = data.assignment;
      teacherGroupSync = data.teacher_group_sync ?? {
        has_clone: false,
        needs_update: false,
        clone_ids: [],
      };
      // If this was newly created, switch to edit mode by default
      if (
        role !== "student" &&
        typeof location !== "undefined" &&
        new URLSearchParams(location.search).get("new") === "1"
      ) {
        startEdit();
        history.replaceState(null, "", location.pathname);
      }
      if (role === "student") {
        submissions = data.submissions ?? [];
        latestSub = submissions[0] ?? null;
        results = [];
        // test count comes from tests_count for students
        testsCount =
          (typeof data.tests_count === "number"
            ? data.tests_count
            : Array.isArray((data as any).tests)
              ? (data as any).tests.length
              : 0) || 0;
        if (latestSub) {
          const subData = await apiJSON(`/api/submissions/${latestSub.id}`);
          results = subData.results ?? [];
          results.forEach((r: any) => {
            if (r.failure_explanation) {
              explanations[r.test_case_id] = { loading: false, text: r.failure_explanation };
            }
          });
          explanations = { ...explanations };
        }
        const best = submissions.reduce((m: number, s: any) => {
          const p = s.override_points ?? s.points ?? 0;
          return p > m ? p : m;
        }, 0);
        pointsEarned = best;
        done = best >= assignment.max_points;
        subStats = {};
        await loadSubmissionStats(submissions, true);
      } else {
        allSubs = data.submissions ?? [];
        teacherRuns = data.teacher_runs ?? [];
        // for non-students, tests array is present
        try {
          testsCount = Array.isArray((data as any).tests)
            ? (data as any).tests.length
            : 0;
        } catch {
          testsCount = 0;
        }
        subStats = {};
        const cls = await apiJSON(`/api/classes/${assignment.class_id}`);
        students = cls.students ?? [];
        // Fetch current extensions
        try {
          overrides = await apiJSON(`/api/assignments/${id}/extensions`);
        } catch {}
        overrideMap = {};
        for (const o of overrides) overrideMap[o.student_id] = o;
        progress = students.map((s: any) => {
          const subs = allSubs.filter((x: any) => x.student_id === s.id);
          const latest = subs[0];
          const hasCompleted = subs.some(
            (x: any) => x.status === "completed" || x.status === "passed",
          );
          const displayStatus = hasCompleted
            ? "completed"
            : latest
              ? latest.status
              : "none";
          return { student: s, latest, all: subs, displayStatus };
        });
      }
    } catch (e: any) {
      err = e.message;
    }
  }

  async function loadSubmissionStats(list?: any[], reset = false) {
    try {
      const source = Array.isArray(list) && list.length ? list : submissions;
      if (reset) subStats = {};
      if (testsCount > 0 && Array.isArray(source) && source.length) {
        const targets = reset
          ? source
          : source.filter((s: any) => !subStats[s.id]);
        if (!targets.length) return;
        const pairs = await Promise.all(
          targets.map(async (s: any) => {
            try {
              const subData = await apiJSON(`/api/submissions/${s.id}`);
              const res = subData.results ?? [];
              const passed = Array.isArray(res)
                ? res.filter((r: any) => r.status === "passed").length
                : 0;
              return [s.id, { passed, total: res.length }] as const;
            } catch {
              return [s.id, { passed: 0, total: 0 }] as const;
            }
          }),
        );
        const next: Record<string, { passed: number; total: number }> = reset
          ? {}
          : { ...subStats };
        for (const [sid, st] of pairs) {
          next[sid] = st;
        }
        subStats = next;
      }
    } catch {}
  }

  onMount(async () => {
    await load();
    // restore tab from URL
    try {
      const t = $page?.url?.searchParams?.get("tab") || "";
      if (t && isValidTab(t)) activeTab = t as TabKey;
    } catch {}
    if (typeof sessionStorage !== "undefined") {
      const saved = sessionStorage.getItem(`assign-${id}-expanded`);
      if (saved) expanded = parseInt(saved);
      await tick();
      const scroll = sessionStorage.getItem(`assign-${id}-scroll`);
      if (scroll) window.scrollTo(0, parseInt(scroll));
    }
    window.addEventListener("beforeunload", saveState);

    const evs = new EventSource("/api/events");
    evs.addEventListener("status", (ev: MessageEvent) => {
      const d = JSON.parse((ev as MessageEvent).data);
      if (latestSub && d.submission_id === latestSub.id) {
        latestSub.status = d.status;
        if (d.status !== "running") load();
      }
    });
    evs.addEventListener("result", (ev: MessageEvent) => {
      const d = JSON.parse((ev as MessageEvent).data);
      if (latestSub && d.submission_id === latestSub.id) {
        results = [...results, d];
      }
    });
    esCtrl = { close: () => evs.close() };
  });

  onDestroy(() => {
    esCtrl?.close();
    if (typeof window !== "undefined") {
      window.removeEventListener("beforeunload", saveState);
    }
  });

  async function uploadTemplate() {
    if (!templateFile) return;
    const fd = new FormData();
    fd.append("file", templateFile);
    try {
      await apiFetch(`/api/assignments/${id}/template`, {
        method: "POST",
        body: fd,
      });
      templateFile = null;
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }

  async function downloadTemplate() {
    try {
      const res = await apiFetch(`/api/assignments/${id}/template`);
      if (!res.ok) throw new Error("download failed");
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = assignment.template_path.split("/").pop();
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (e: any) {
      err = e.message;
    }
  }

  function startEdit() {
    eTitle = assignment.title;
    eDesc = assignment.description;
    ePoints = assignment.max_points;
    ePolicy = assignment.grading_policy;
    eMaxSubmissionSizeMB =
      assignment.max_submission_size_mb ?? defaultMaxSubmissionSizeMB;
    // Fix: convert UTC deadline to local time string for input[type="datetime-local"]
    // The input expects "YYYY-MM-DDTHH:mm", but simply slicing toISOString() gives UTC time.
    // We need to shift the time by the timezone offset before slicing.
    const toLocalISO = (dateStr: string) => {
      if (!dateStr) return "";
      const d = new Date(dateStr);
      const local = new Date(d.getTime() - d.getTimezoneOffset() * 60000);
      return local.toISOString().slice(0, 16);
    };

    eDeadline = toLocalISO(assignment.deadline);
    if (eDeadline) {
      eDeadlineDate = eDeadline.slice(0, 10);
      eDeadlineTime = eDeadline.slice(11, 16);
    }
    if (assignment.second_deadline) {
      eSecondDeadline = toLocalISO(assignment.second_deadline);
      eSecondDeadlineDate = eSecondDeadline.slice(0, 10);
      eSecondDeadlineTime = eSecondDeadline.slice(11, 16);
    } else {
      eSecondDeadline = "";
      eSecondDeadlineDate = "";
      eSecondDeadlineTime = "";
    }
    eLatePenaltyRatio = assignment.late_penalty_ratio ?? 0.5;
    eShowTraceback = assignment.show_traceback;
    eShowTestDetails = !!assignment.show_test_details;
    eProgrammingLanguage = assignment.programming_language ?? "python";
    eManualReview = assignment.manual_review;
    eLLMInteractive = !!assignment.llm_interactive;
    eLLMFeedback = !!assignment.llm_feedback;
    eLLMHelpWhyFailed = !!assignment.llm_help_why_failed;
    eLLMAutoAward = assignment.llm_auto_award ?? true;
    eLLMScenarios = assignment.llm_scenarios_json ?? "";
    eLLMStrictness =
      typeof assignment.llm_strictness === "number"
        ? assignment.llm_strictness
        : 50;
    eLLMRubric = assignment.llm_rubric ?? "";
    eLLMTeacherBaseline = assignment.llm_teacher_baseline_json ?? "";
    scratchCriteria = parseScratchCriteria(
      assignment.scratch_semantic_criteria ?? "",
    );
    scratchEvaluationMode = normalizeScratchEvaluationMode(
      assignment.scratch_evaluation_mode,
    );
    showAdvancedOptions = !!assignment.second_deadline;
    if (assignment.manual_review) testMode = "manual";
    else if (assignment.llm_interactive) testMode = "ai";
    else testMode = "automatic";
    editing = true;
  }

  // keep combined strings in sync with split date/time inputs
  $: {
    if (eDeadlineDate && eDeadlineTime)
      eDeadline = `${eDeadlineDate}T${eDeadlineTime}`;
    else if (!eDeadlineDate) eDeadline = "";
  }
  $: {
    if (eDeadlineDate && !eDeadlineTime) eDeadlineTime = "23:59";
  }
  $: {
    if (showAdvancedOptions) {
      if (eSecondDeadlineDate && eSecondDeadlineTime)
        eSecondDeadline = `${eSecondDeadlineDate}T${eSecondDeadlineTime}`;
      else if (!eSecondDeadlineDate) eSecondDeadline = "";
    } else {
      eSecondDeadline = "";
    }
  }
  $: {
    if (eSecondDeadlineDate && !eSecondDeadlineTime)
      eSecondDeadlineTime = "23:59";
  }

  async function saveEdit() {
    try {
      if (new Date(eDeadline) < new Date()) {
        const proceed = await confirmModal.open({
          title: t(
            "frontend/src/routes/assignments/[id]/+page.svelte::deadline_past_title",
          ),
          body: t(
            "frontend/src/routes/assignments/[id]/+page.svelte::deadline_past_body",
          ),
          confirmLabel: t(
            "frontend/src/routes/assignments/[id]/+page.svelte::confirm",
          ),
          cancelLabel: t(
            "frontend/src/routes/assignments/[id]/+page.svelte::cancel",
          ),
          confirmClass: "btn btn-warning",
          cancelClass: "btn",
        });
        if (!proceed) return;
      }
      if (eSecondDeadline && new Date(eSecondDeadline) <= new Date(eDeadline)) {
        const proceed = await confirmModal.open({
          title: t(
            "frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_first_title",
          ),
          body: t(
            "frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_first_body",
          ),
          confirmLabel: t(
            "frontend/src/routes/assignments/[id]/+page.svelte::confirm",
          ),
          cancelLabel: t(
            "frontend/src/routes/assignments/[id]/+page.svelte::cancel",
          ),
          confirmClass: "btn btn-warning",
          cancelClass: "btn",
        });
        if (!proceed) return;
      }
      // For weighted assignments, max_points is calculated from test weights
      const isScratch = eProgrammingLanguage === "scratch";
      const maxPoints =
        ePolicy === "weighted" && !isScratch
          ? assignment.max_points || 100
          : Number(ePoints);
      const scratchCriteriaPayload = isScratch
        ? serializeScratchCriteria(scratchCriteria)
        : null;

      await apiFetch(`/api/assignments/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          title: eTitle,
          description: eDesc,
          deadline: new Date(eDeadline).toISOString(),
          max_points: maxPoints,
          max_submission_size_mb: Number(eMaxSubmissionSizeMB),
          grading_policy: ePolicy,
          show_traceback: eShowTraceback,
          show_test_details: eShowTestDetails,
          manual_review: eManualReview,
          programming_language: eProgrammingLanguage,
          llm_interactive: eLLMInteractive,
          llm_feedback: eLLMFeedback,
          llm_help_why_failed: eLLMHelpWhyFailed,
          llm_auto_award: eLLMAutoAward,
          llm_scenarios_json: eLLMScenarios.trim() ? eLLMScenarios : null,
          llm_strictness: Number.isFinite(eLLMStrictness)
            ? Math.min(100, Math.max(0, Number(eLLMStrictness)))
            : 50,
          llm_rubric: eLLMRubric.trim() ? eLLMRubric : null,
          scratch_evaluation_mode: isScratch ? scratchEvaluationMode : "manual",
          scratch_semantic_criteria: scratchCriteriaPayload,
          second_deadline: eSecondDeadline.trim()
            ? new Date(eSecondDeadline).toISOString()
            : null,
          late_penalty_ratio: Number.isFinite(eLatePenaltyRatio)
            ? Math.min(1, Math.max(0, Number(eLatePenaltyRatio)))
            : 0.5,
        }),
      });
      editing = false;
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }

  async function delAssignment() {
    const confirmed = await confirmModal.open({
      title: t(
        "frontend/src/routes/assignments/[id]/+page.svelte::delete_assignment_title",
      ),
      body: t(
        "frontend/src/routes/assignments/[id]/+page.svelte::delete_assignment_body",
      ),
      confirmLabel: t(
        "frontend/src/routes/assignments/[id]/+page.svelte::delete_assignment_confirm",
      ),
      confirmClass: "btn btn-error",
      cancelClass: "btn",
    });
    if (!confirmed) return;
    try {
      await apiFetch(`/api/assignments/${id}`, { method: "DELETE" });
      goto(`/classes/${assignment.class_id}`);
    } catch (e: any) {
      err = e.message;
    }
  }

  function saveState() {
    if (typeof sessionStorage === "undefined") return;
    sessionStorage.setItem(
      `assign-${id}-expanded`,
      expanded === null ? "" : String(expanded),
    );
    sessionStorage.setItem(`assign-${id}-scroll`, String(window.scrollY));
  }

  async function toggleStudent(id: number) {
    const next = expanded === id ? null : id;
    expanded = next;
    saveState();
    if (next !== null) {
      const entry = progress.find((p: any) => p.student.id === next);
      if (entry) {
        await loadSubmissionStats(entry.all);
      }
    }
  }

  function statusColor(s: string) {
    if (s === "completed") return "badge-success";
    if (s === "running") return "badge-info";
    if (s === "provisional") return "badge-warning";
    if (s === "partially_completed") return "badge-warning";
    if (s === "failed") return "badge-error";
    if (s === "passed") return "badge-success";
    if (s === "wrong_output") return "badge-error";
    if (s === "runtime_error") return "badge-error";
    if (s === "illegal_tool_use") return "badge-error";
    if (s === "time_limit_exceeded" || s === "memory_limit_exceeded")
      return "badge-warning";
    return "";
  }

  function statusLabel(s: string) {
    return submissionStatusLabel(s);
  }

  let explanations: Record<string, { loading: boolean; text?: string; error?: string }> = {};
  let explainInFlight = false;
  let explainQueue: string[] = [];
  let summaryExplanation: { loading: boolean; text?: string; error?: string } = {
    loading: false,
  };

  async function fetchExplanation(sid: string, tcid: string) {
    return apiJSON(`/api/submissions/${sid}/explain-test-failure`, {
      method: "POST",
      body: JSON.stringify({ test_case_id: tcid }),
    });
  }

  async function fetchSummaryExplanation(sid: string) {
    return apiJSON(`/api/submissions/${sid}/explain-all-test-failures`, {
      method: "POST",
    });
  }

  async function askWhyFailed(sid: string, tcid: string) {
    if (explanations[tcid]?.loading || explanations[tcid]?.text) return;
    if (explainInFlight) {
      explanations[tcid] = { loading: true };
      explanations = { ...explanations };
      if (!explainQueue.includes(tcid)) explainQueue = [...explainQueue, tcid];
      return;
    }

    explainInFlight = true;
    explanations[tcid] = { loading: true };
    explanations = { ...explanations };
    try {
      const res = await fetchExplanation(sid, tcid);
      let firstApplied = false;
      while (explainQueue.length) {
        const queued = explainQueue;
        explainQueue = [];
        const results = await Promise.all(
          queued.map(async (id) => {
            try {
              const cached = await fetchExplanation(sid, id);
              return { id, text: cached.explanation as string };
            } catch (e: any) {
              return { id, error: e.message as string };
            }
          }),
        );
        if (!firstApplied) {
          explanations[tcid] = { loading: false, text: res.explanation };
          firstApplied = true;
        }
        for (const r of results) {
          if (r.error) {
            explanations[r.id] = { loading: false, error: r.error };
          } else {
            explanations[r.id] = { loading: false, text: r.text };
          }
        }
        explanations = { ...explanations };
      }
      if (!firstApplied) {
        explanations[tcid] = { loading: false, text: res.explanation };
        explanations = { ...explanations };
      }
    } catch (e: any) {
      const errMsg = e.message as string;
      explanations[tcid] = { loading: false, error: errMsg };
      if (explainQueue.length) {
        for (const id of explainQueue) {
          explanations[id] = { loading: false, error: errMsg };
        }
        explainQueue = [];
      }
      explanations = { ...explanations };
    } finally {
      explainInFlight = false;
    }
  }

  async function askWhyAllFailed(sid: string) {
    if (summaryExplanation.loading || summaryExplanation.text) return;
    summaryExplanation = { loading: true };
    try {
      const res = await fetchSummaryExplanation(sid);
      summaryExplanation = { loading: false, text: res.explanation };
    } catch (e: any) {
      summaryExplanation = { loading: false, error: e.message as string };
    }
  }

  function openTeacherRunModal() {
    teacherRunDialog.showModal();
  }

  async function runTeacherSolution() {
    if (!solFiles.length) return;
    const fd = new FormData();
    for (const f of solFiles) fd.append("files", f);
    try {
      solLoading = true;
      await apiJSON(`/api/assignments/${id}/solution-run`, {
        method: "POST",
        body: fd,
      });
      solFiles = [];
      teacherRunDialog.close();
      await load();
      activeTab = "teacher-runs";
    } catch (e: any) {
      err = e.message;
    } finally {
      solLoading = false;
    }
  }

  async function submit() {
    if (files.length === 0) return;
    
    isUploading = true;
    uploadProgress = 0;
    
    const fd = new FormData();
    for (const f of files) {
      fd.append("files", f);
    }
    
    try {
      // Use XMLHttpRequest to track upload progress
      await new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        const url = assignment.id ? `/api/assignments/${id}/submissions` : `/api/assignments/${id}/submissions`; 
        // Note: id is derived from $page.params.id at line 60
        
        xhr.open("POST", `/api/assignments/${id}/submissions`);
        xhr.withCredentials = true;
        
        xhr.upload.onprogress = (event) => {
          if (event.lengthComputable) {
            uploadProgress = Math.round((event.loaded / event.total) * 100);
          }
        };
        
        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            resolve(xhr.response);
          } else {
            try {
              const errorData = JSON.parse(xhr.responseText);
              if (errorData?.error === "submission_too_large") {
                const size =
                  typeof errorData.size_mb === "number"
                    ? Math.round(errorData.size_mb * 10) / 10
                    : 0;
                const max =
                  typeof errorData.max_mb === "number"
                    ? errorData.max_mb
                    : defaultMaxSubmissionSizeMB;
                reject(
                  new Error(
                    t(
                      "frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_size_error",
                      { size, max },
                    ),
                  ),
                );
              } else {
                reject(new Error(errorData.error || xhr.statusText));
              }
            } catch {
              reject(new Error(xhr.statusText));
            }
          }
        };
        
        xhr.onerror = () => reject(new Error("Network error"));
        xhr.send(fd);
      });

      files = [];
      if (fileInput) fileInput.value = "";
      submitDialog.close();
      await load();
    } catch (e: any) {
      err = e.message;
    } finally {
      isUploading = false;
      uploadProgress = 0;
    }
  }

  function openSubmitModal() {
    files = [];
    if (fileInput) fileInput.value = "";
    submitDialog.showModal();
  }

  function openTestsModal() {
    goto(`/assignments/${id}/tests`);
  }

  // removed updateTest (moved to tests page)

  // Persist and restore selected tab via URL so back/forward keeps state
  function isValidTab(key: string): key is TabKey {
    const allowed: TabKey[] = ["overview"];
    if (role === "student") {
      allowed.push("submissions");
      if (!assignment?.manual_review && !isScratchAssignment) {
        allowed.push("results");
      }
    }
    if (role === "teacher" || role === "admin") {
      allowed.push("instructor");
      if (!isScratchAssignment) allowed.push("teacher-runs");
    }
    return allowed.includes(key as TabKey);
  }

  // initialize activeTab from URL once on mount (do not keep overwriting on every reactive cycle)

  function saveTabToUrl() {
    try {
      if (typeof location !== "undefined" && typeof history !== "undefined") {
        const url = new URL(location.href);
        url.searchParams.set("tab", activeTab);
        history.replaceState(history.state, "", url);
      }
    } catch {}
  }

  function setTab(tab: TabKey) {
    activeTab = tab;
    saveTabToUrl();
  }

  // Extension dialog state (teacher)
  let extendDialog: HTMLDialogElement;
  let extStudent: any = null;
  let extDeadline = "";
  let extDeadlineDate = "";
  let extDeadlineTime = "";
  let extNote = "";
  function openExtendDialog(student: any) {
    extStudent = student;
    const cur = overrideMap[student.id];
    extDeadline = cur
      ? String(cur.new_deadline).slice(0, 16)
      : assignment.deadline?.slice(0, 16) || "";
    extDeadlineDate = extDeadline ? extDeadline.slice(0, 10) : "";
    extDeadlineTime = extDeadline ? extDeadline.slice(11, 16) : "";
    extNote = cur?.note || "";
    extendDialog.showModal();
  }
  async function saveExtension() {
    if (!extStudent || !extDeadline) return;
    try {
      await apiFetch(`/api/assignments/${id}/extensions/${extStudent.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          new_deadline: new Date(extDeadline).toISOString(),
          note: extNote.trim() ? extNote : null,
        }),
      });
      extendDialog.close();
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }
  async function clearExtension() {
    if (!extStudent) return;
    try {
      await apiFetch(`/api/assignments/${id}/extensions/${extStudent.id}`, {
        method: "DELETE",
      });
      extendDialog.close();
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }

  // keep combined extension string synced with date/time parts
  $: {
    if (extDeadlineDate && extDeadlineTime)
      extDeadline = `${extDeadlineDate}T${extDeadlineTime}`;
    else if (!extDeadlineDate) extDeadline = "";
  }
  $: {
    if (extDeadlineDate && !extDeadlineTime) extDeadlineTime = "23:59";
  }

  // ───────────────────────────
  // Deadline picker modal (reusable)
  // ───────────────────────────
  let deadlinePicker: InstanceType<typeof DeadlinePicker>;
  function euLabelFromParts(d: string, t: string): string {
    if (!d || !t) return "";
    // d: yyyy-mm-dd
    const day = d.slice(8, 10);
    const mon = d.slice(5, 7);
    const y = d.slice(0, 4);
    return `${day}. ${mon}. ${y} ${t}`;
  }

  function formatDeadlineDisplay(isoStr: string): string {
    if (!isoStr) return "-";
    // Expects YYYY-MM-DDTHH:mm
    const [d, t] = isoStr.split("T");
    if (!d || !t) return isoStr;
    const day = d.slice(8, 10);
    const mon = d.slice(5, 7);
    const y = d.slice(0, 4);
    return `${day}. ${mon}. ${y} ${t}`;
  }
  async function pickMainDeadline() {
    const initial =
      eDeadlineDate && eDeadlineTime
        ? `${eDeadlineDate}T${eDeadlineTime}`
        : (assignment?.deadline ?? null);
    const picked = await deadlinePicker.open({
      title: t(
        "frontend/src/routes/assignments/[id]/+page.svelte::select_main_deadline",
      ),
      initial,
    });
    if (picked) {
      eDeadlineDate = picked.slice(0, 10);
      eDeadlineTime = picked.slice(11, 16);
    }
  }
  async function pickSecondDeadline() {
    const initial =
      eSecondDeadlineDate && eSecondDeadlineTime
        ? `${eSecondDeadlineDate}T${eSecondDeadlineTime}`
        : (assignment?.second_deadline ?? null);
    const picked = await deadlinePicker.open({
      title: t(
        "frontend/src/routes/assignments/[id]/+page.svelte::select_second_deadline",
      ),
      initial,
    });
    if (picked) {
      eSecondDeadlineDate = picked.slice(0, 10);
      eSecondDeadlineTime = picked.slice(11, 16);
    }
  }
  async function pickExtensionDeadline() {
    const initial =
      extDeadlineDate && extDeadlineTime
        ? `${extDeadlineDate}T${extDeadlineTime}`
        : (assignment?.deadline ?? null);
    const picked = await deadlinePicker.open({
      title: t(
        "frontend/src/routes/assignments/[id]/+page.svelte::select_new_deadline",
      ),
      initial,
    });
    if (picked) {
      extDeadlineDate = picked.slice(0, 10);
      extDeadlineTime = picked.slice(11, 16);
    }
  }
</script>

{#if !assignment}
  <div class="flex items-center gap-3">
    <span class="loading loading-spinner loading-md"></span>
    <p>
      {t(
        "frontend/src/routes/assignments/[id]/+page.svelte::loading_assignment",
      )}
    </p>
  </div>
{:else}
  {#if editing}
    <div class="card-elevated mb-8 overflow-hidden border-none shadow-2xl">
      <div class="card-body p-0">
        <!-- Modern Header with Gradient -->
        <div class="bg-gradient-to-r from-primary/10 via-base-200 to-secondary/10 p-6 border-b border-base-300/50">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-4">
              <div class="p-3 rounded-2xl bg-primary text-primary-content shadow-lg shadow-primary/20">
                <Edit3 size={24} />
              </div>
              <div>
                <h1 class="text-2xl font-black tracking-tight">
                  {t("frontend/src/routes/assignments/[id]/+page.svelte::edit_assignment_title")}
                </h1>
                <div class="flex items-center gap-2 mt-1">
                  {#if !assignment.published}
                    <span class="badge badge-sm badge-warning font-black text-[9px] uppercase tracking-wider">Draft</span>
                  {:else}
                    <span class="badge badge-sm badge-success font-black text-[9px] uppercase tracking-wider">Published</span>
                  {/if}
                </div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <button class="btn btn-sm btn-ghost hover:bg-base-300/50 gap-2" on:click={() => (editing = false)}>
                <X size={16} />
                <span class="hidden sm:inline">{t("frontend/src/routes/assignments/[id]/+page.svelte::cancel_button")}</span>
              </button>
              <button class="btn btn-sm btn-primary shadow-lg shadow-primary/20 gap-2" on:click={saveEdit}>
                <Save size={16} />
                <span>{t("frontend/src/routes/assignments/[id]/+page.svelte::save_changes_button")}</span>
              </button>
            </div>
          </div>
        </div>

        <div class="grid lg:grid-cols-12 gap-0">
          <!-- Main Content Area -->
          <div class="lg:col-span-8 p-6 space-y-6 border-r border-base-300/30">
            
            <!-- Basic info -->
            <section class="space-y-4">
              <div class="flex items-center gap-2 px-1">
                <div class="p-1.5 rounded-lg bg-primary/10 text-primary">
                  <Info size={16} />
                </div>
                <h3 class="font-black text-[11px] uppercase tracking-widest opacity-60">
                  {t("frontend/src/routes/assignments/[id]/+page.svelte::basic_info_heading")}
                </h3>
              </div>
              
              <div class="space-y-3 p-1">
                <StylishInput
                  bind:value={eTitle}
                  placeholder={t("frontend/src/routes/assignments/[id]/+page.svelte::title_placeholder")}
                  required
                  icon={FileText}
                  small
                />
                
                <div class="form-control">
                  <MarkdownEditor
                    bind:value={eDesc}
                    placeholder={t("frontend/src/routes/assignments/[id]/+page.svelte::description_placeholder")}
                  />
                </div>
              </div>
            </section>

            <!-- Scoring & Policy -->
            <section class="space-y-4">
              <div class="flex items-center gap-2 px-1">
                <div class="p-1.5 rounded-lg bg-secondary/10 text-secondary">
                  <Trophy size={16} />
                </div>
                <h3 class="font-black text-[11px] uppercase tracking-widest opacity-60">
                  Grading & Scoring
                </h3>
              </div>

              <div class="bg-base-200/40 rounded-2xl p-4 border border-base-300/30 space-y-4">
                <div class="form-control">
                  <label class="label pt-0" for="grading-policy-select">
                    <span class="label-text font-bold text-xs">{t("frontend/src/routes/assignments/[id]/+page.svelte::grading_policy_label")}</span>
                  </label>
                  <CustomSelect
                    options={gradingPolicyOptions}
                    bind:value={ePolicy}
                    small
                  />
                  {#if !isScratchAssignment && (testMode === "manual" || testMode === "ai")}
                    <div class="label-text-alt text-warning mt-1.5 flex gap-1.5 items-start">
                      <AlertTriangle size={12} class="shrink-0 mt-0.5" />
                      <span>{t("frontend/src/routes/assignments/[id]/+page.svelte::weighted_grading_warning")}</span>
                    </div>
                  {/if}
                </div>

                {#if ePolicy === "all_or_nothing" || isScratchAssignment}
                  <StylishInput
                    type="number"
                    bind:value={ePoints}
                    label={t("frontend/src/routes/assignments/[id]/+page.svelte::max_points_label")}
                    placeholder={t("frontend/src/routes/assignments/[id]/+page.svelte::max_points_placeholder")}
                    required
                    icon={Trophy}
                    small
                  />
                {:else}
                  <div class="p-3 bg-primary/5 rounded-xl border border-primary/10">
                    <div class="text-[10px] font-black uppercase tracking-wider text-primary mb-1">Max Points</div>
                    <div class="text-sm font-medium italic opacity-60">
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::from_test_weights_label")}
                    </div>
                  </div>
                {/if}
                
                <div class="divider opacity-10 my-1"></div>
                
                <div class="text-[10px] space-y-2 leading-relaxed">
                  <div class="opacity-70">
                    <span class="font-black text-primary">{t("frontend/src/routes/assignments/[id]/+page.svelte::policyLabel_allOrNothing")}:</span>
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::all_or_nothing_desc")}
                  </div>
                  <div class="opacity-70">
                    <span class="font-black text-secondary">{t("frontend/src/routes/assignments/[id]/+page.svelte::policyLabel_weighted")}:</span>
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::weighted_desc")}
                  </div>
                </div>
              </div>
            </section>

            <!-- Deadlines -->
            <section class="space-y-4">
              <div class="flex items-center justify-between px-1">
                <div class="flex items-center gap-2">
                  <div class="p-1.5 rounded-lg bg-accent/10 text-accent">
                    <Calendar size={16} />
                  </div>
                  <h3 class="font-black text-[11px] uppercase tracking-widest opacity-60">
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::deadlines_heading")}
                  </h3>
                </div>
                
                <label class="flex items-center gap-2 cursor-pointer group">
                  <input
                    type="checkbox"
                    class="checkbox checkbox-xs checkbox-primary"
                    bind:checked={showAdvancedOptions}
                  />
                  <span class="text-[11px] font-bold opacity-60 group-hover:opacity-100 transition-opacity">
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::enable_second_deadline")}
                  </span>
                </label>
              </div>

              <div class="grid sm:grid-cols-2 gap-4">
                <div class="bg-base-200/40 rounded-2xl p-4 border border-base-300/30">
                  <div class="label pt-0">
                    <span class="label-text font-bold text-xs">{t("frontend/src/routes/assignments/[id]/+page.svelte::main_deadline_label")}</span>
                  </div>
                  <div class="flex items-center gap-2">
                    <div class="relative flex-1">
                      <input
                        class="input input-bordered input-sm w-full bg-base-100 pl-3 font-mono text-xs text-left"
                        readonly
                        placeholder="dd/mm/yyyy hh:mm"
                        value={euLabelFromParts(eDeadlineDate, eDeadlineTime)}
                      />
                    </div>
                    <button type="button" class="btn btn-sm" on:click={pickMainDeadline}>
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::pick_button")}
                    </button>
                    {#if eDeadlineDate}
                      <button type="button" class="btn btn-sm btn-ghost btn-square" on:click={() => { eDeadlineDate = ""; eDeadlineTime = ""; }}>
                        <X size={14} />
                      </button>
                    {/if}
                  </div>
                </div>

                {#if showAdvancedOptions}
                  <div class="bg-base-200/40 rounded-2xl p-4 border border-base-300/30 animate-in fade-in slide-in-from-left-2 duration-300">
                    <div class="label pt-0">
                      <span class="label-text font-bold text-xs">{t("frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_label")}</span>
                    </div>
                    <div class="flex items-center gap-2">
                      <div class="relative flex-1">
                        <input
                          class="input input-bordered input-sm w-full bg-base-100 pl-3 font-mono text-xs text-left"
                          readonly
                          placeholder="dd/mm/yyyy hh:mm"
                          value={euLabelFromParts(eSecondDeadlineDate, eSecondDeadlineTime)}
                        />
                      </div>
                      <button type="button" class="btn btn-sm" on:click={pickSecondDeadline}>
                        {t("frontend/src/routes/assignments/[id]/+page.svelte::pick_button")}
                      </button>
                      {#if eSecondDeadlineDate}
                        <button type="button" class="btn btn-sm btn-ghost btn-square" on:click={() => { eSecondDeadlineDate = ""; eSecondDeadlineTime = ""; }}>
                          <X size={14} />
                        </button>
                      {/if}
                    </div>
                  </div>
                {/if}
              </div>

              {#if showAdvancedOptions}
                <div class="bg-warning/5 rounded-2xl p-4 border border-warning/10 space-y-3 animate-in fade-in duration-500">
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-bold opacity-70">
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::late_penalty_ratio_label")}
                    </span>
                    <span class="badge badge-sm badge-warning font-black font-mono">
                      {Math.round(eLatePenaltyRatio * 100)}%
                    </span>
                  </div>
                  <div class="flex flex-col gap-1.5">
                    <input
                      id="late-penalty-range"
                      type="range"
                      min="0"
                      max="100"
                      step="1"
                      class="range range-warning range-xs w-full"
                      value={Math.round(eLatePenaltyRatio * 100)}
                      on:input={(e) => (eLatePenaltyRatio = parseInt(e.currentTarget.value) / 100)}
                    />
                    <div class="grid grid-cols-3 text-[10px] font-black opacity-40 uppercase tracking-tighter px-0.5">
                      <span class="text-left">0%</span>
                      <span class="text-center text-warning/80">50%</span>
                      <span class="text-right">100%</span>
                    </div>
                  </div>
                </div>
              {/if}
            </section>

            <!-- Testing and grading -->
            <section class="space-y-4">
              <div class="flex items-center gap-2 px-1">
                <div class="p-1.5 rounded-lg bg-info/10 text-info">
                  <FlaskConical size={16} />
                </div>
                <h3 class="font-black text-[11px] uppercase tracking-widest opacity-60">
                  {t("frontend/src/routes/assignments/[id]/+page.svelte::testing_grading_heading")}
                </h3>
              </div>

              <div class="bg-base-200/40 rounded-2xl p-5 border border-base-300/30 space-y-5">
                <div class="flex flex-wrap items-center gap-4">
                  <div class="form-control min-w-[220px]">
                    <span class="label-text font-bold text-[10px] uppercase opacity-40 mb-1.5 ml-1">
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::programming_language_label")}
                    </span>
                    <CustomSelect
                      options={programmingLanguageOptions}
                      bind:value={eProgrammingLanguage}
                      small
                    />
                  </div>
                  <div class="flex-1 min-w-[200px]">
                    <StylishInput
                      type="number"
                      bind:value={eMaxSubmissionSizeMB}
                      label={t("frontend/src/routes/assignments/[id]/+page.svelte::max_submission_size_label")}
                      icon={FileUp}
                      small
                    />
                  </div>
                  {#if eProgrammingLanguage === "scratch"}
                    <div class="flex-1 text-[11px] text-info/80 font-medium">
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_manual_review_note")}
                    </div>
                  {/if}
                </div>

                <div class="flex flex-wrap items-center gap-4">
                  {#if eProgrammingLanguage === "scratch"}
                    <div class="form-control min-w-[220px]">
                      <span class="label-text font-bold text-[10px] uppercase opacity-40 mb-1.5 ml-1">
                        {t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_label")}
                      </span>
                      <CustomSelect
                        options={scratchEvaluationModeOptions}
                        bind:value={scratchEvaluationMode}
                        small
                      />
                    </div>
                  {:else}
                    <div class="form-control min-w-[200px]">
                      <span class="label-text font-bold text-[10px] uppercase opacity-40 mb-1.5 ml-1">Mode</span>
                      <CustomSelect
                        options={testModeOptions}
                        bind:value={testMode}
                        small
                        disabled={ePolicy === "weighted"}
                      />
                    </div>
                  {/if}

                  {#if eProgrammingLanguage !== "scratch" && testMode === "automatic"}
                    <div class="flex flex-wrap items-center gap-x-6 gap-y-2 pt-4 sm:pt-0">
                      <label class="flex items-center gap-2 cursor-pointer group">
                        <input type="checkbox" class="checkbox checkbox-xs" bind:checked={eShowTraceback} />
                        <span class="text-xs font-medium opacity-70 group-hover:opacity-100 transition-opacity">
                          {t("frontend/src/routes/assignments/[id]/+page.svelte::show_traceback_students")}
                        </span>
                      </label>
                      <label class="flex items-center gap-2 cursor-pointer group">
                        <input type="checkbox" class="checkbox checkbox-xs" bind:checked={eShowTestDetails} />
                        <span class="text-xs font-medium opacity-70 group-hover:opacity-100 transition-opacity">
                          {t("frontend/src/routes/assignments/[id]/+page.svelte::reveal_test_details_teacher_review")}
                        </span>
                      </label>
                      <label class="flex items-center gap-2 cursor-pointer group">
                        <input type="checkbox" class="checkbox checkbox-xs" bind:checked={eLLMHelpWhyFailed} />
                        <div class="flex flex-col">
                          <span class="text-xs font-medium opacity-70 group-hover:opacity-100 transition-opacity">
                            {t("frontend/src/routes/assignments/[id]/+page.svelte::llm_help_why_failed")}
                          </span>
                        </div>
                      </label>
                    </div>
                  {/if}
                </div>

                <div class="p-3 bg-base-100 rounded-xl border border-base-300/40 text-[11px] leading-relaxed opacity-70 italic">
                  {#if eProgrammingLanguage === "scratch"}
                    {#if scratchEvaluationMode === "automatic"}
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_auto_desc")}
                    {:else if scratchEvaluationMode === "semi_automatic"}
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_semi_desc")}
                    {:else}
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_manual_desc")}
                    {/if}
                  {:else if ePolicy === "weighted"}
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::weighted_assignments_desc")}
                  {:else if testMode === "automatic"}
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::automatic_tests_desc")}
                  {:else if testMode === "manual"}
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::manual_review_desc")}
                  {:else}
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::ai_grading_desc")}
                  {/if}
                </div>

                {#if eProgrammingLanguage === "scratch"}
                  <div class="space-y-6 animate-in fade-in duration-500">
                    <div class="flex items-center justify-between gap-3 px-1">
                      <div class="flex items-center gap-2">
                        <div class="p-1.5 rounded-lg bg-primary/10 text-primary">
                          <LayoutList size={16} />
                        </div>
                        <h4 class="text-[11px] font-black uppercase tracking-widest opacity-60">
                          {t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_semantic_criteria_label")}
                        </h4>
                      </div>
                      <button
                        type="button"
                        class="btn btn-sm btn-primary btn-outline gap-2 shadow-sm hover:shadow-md transition-all rounded-xl"
                        on:click={addScratchCriterion}
                      >
                        <Plus size={16} />
                        <span class="text-[10px] font-black uppercase tracking-widest">
                          {t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_semantic_add")}
                        </span>
                      </button>
                    </div>

                    <div class="space-y-4">
                      {#each scratchCriteria as item, index (item.id)}
                        <div class="group relative bg-base-100/50 hover:bg-base-100 transition-all duration-300 rounded-2xl border border-base-300/40 p-5 shadow-sm hover:shadow-md">
                          <div class="flex flex-col sm:flex-row gap-6 items-start">
                            <div class="flex-1 w-full space-y-3">
                              <div class="flex items-center gap-3">
                                <span class="flex items-center justify-center w-6 h-6 rounded-lg bg-primary/10 text-primary text-[10px] font-black shrink-0">
                                  {index + 1}
                                </span>
                                <label class="text-[10px] font-black uppercase tracking-widest opacity-40">
                                  {t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_semantic_criterion_label")}
                                </label>
                              </div>
                              <textarea
                                class="textarea textarea-bordered w-full bg-base-100/30 focus:bg-base-100 min-h-[80px] transition-all rounded-xl text-sm leading-relaxed"
                                bind:value={item.text}
                                placeholder={t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_semantic_criteria_placeholder")}
                              ></textarea>
                            </div>
                            
                            <div class="w-full sm:w-36">
                              <StylishInput
                                type="number"
                                bind:value={item.points}
                                label={t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_semantic_points_label")}
                                placeholder="0"
                                disabled={ePolicy !== "weighted"}
                                icon={Trophy}
                                small
                              />
                            </div>

                            <div class="sm:self-end sm:pb-1">
                                <button
                                  type="button"
                                  class="btn btn-ghost btn-circle btn-sm text-error/40 hover:text-error hover:bg-error/10 transition-all"
                                  on:click={() => removeScratchCriterion(item.id)}
                                  aria-label={t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_semantic_remove")}
                                >
                                  <Trash2 size={16} />
                                </button>
                            </div>
                          </div>
                        </div>
                      {/each}
                      
                      {#if scratchCriteria.length === 0}
                        <div class="flex flex-col items-center justify-center py-12 px-4 rounded-2xl border border-dashed border-base-300 opacity-60 bg-base-200/20">
                          <LayoutList size={32} class="mb-3 opacity-20" />
                          <p class="text-xs font-medium italic">No criteria added yet. Add your first logic check for the Scratch project.</p>
                        </div>
                      {/if}
                    </div>

                    <div class="flex flex-col gap-4 pt-2">
                      <div class="p-4 bg-base-200/50 rounded-xl border border-base-300/40 text-[11px] leading-relaxed opacity-70 italic flex gap-3 items-start">
                        <Info size={14} class="shrink-0 mt-0.5 text-primary" />
                        <div>{t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_semantic_criteria_help")}</div>
                      </div>
                      
                      <div class="p-5 bg-info/5 rounded-2xl border border-info/10 relative overflow-hidden group/examples">
                        <div class="absolute -top-6 -right-6 text-info opacity-5 pointer-events-none group-hover/examples:scale-110 transition-transform duration-700">
                          <Lightbulb size={120} />
                        </div>
                        
                        <div class="flex items-center gap-2 mb-4">
                          <div class="p-1.5 rounded-lg bg-info/20 text-info">
                            <Lightbulb size={14} />
                          </div>
                          <h4 class="text-[11px] font-black uppercase tracking-widest text-info/70">
                            {t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_semantic_examples_title")}
                          </h4>
                        </div>
                        
                        <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
                          {#each [1, 2, 3] as i}
                            <div class="p-3 bg-base-100/50 rounded-xl border border-info/5 text-[11px] leading-relaxed italic opacity-80 hover:opacity-100 hover:bg-base-100 transition-all cursor-default relative z-10">
                              {t(`frontend/src/routes/assignments/[id]/+page.svelte::scratch_semantic_example_${i}`)}
                            </div>
                          {/each}
                        </div>
                      </div>
                    </div>
                  </div>
                {/if}

                {#if eProgrammingLanguage !== "scratch" && testMode === "ai"}
                  <div class="pt-2 animate-in slide-in-from-top-2 duration-300">
                    <button
                      type="button"
                      class="btn btn-xs btn-outline gap-2 mb-4"
                      on:click={() => (showAiOptions = !showAiOptions)}
                    >
                      <Sparkles size={12} />
                      {showAiOptions
                        ? t("frontend/src/routes/assignments/[id]/+page.svelte::hide_ai_options_button")
                        : t("frontend/src/routes/assignments/[id]/+page.svelte::show_ai_options_button")}
                    </button>
                    
                    {#if showAiOptions}
                      <div class="p-4 bg-primary/5 rounded-2xl border border-primary/10 space-y-5">
                        <div class="grid sm:grid-cols-2 gap-4">
                          <label class="flex items-center gap-3 cursor-pointer group">
                            <input type="checkbox" class="checkbox checkbox-sm checkbox-primary" bind:checked={eLLMFeedback} />
                            <span class="text-xs font-bold opacity-70 group-hover:opacity-100 transition-opacity">
                              {t("frontend/src/routes/assignments/[id]/+page.svelte::give_ai_feedback_students")}
                            </span>
                          </label>
                          <label class="flex items-center gap-3 cursor-pointer group">
                            <input type="checkbox" class="checkbox checkbox-sm checkbox-primary" bind:checked={eLLMAutoAward} />
                            <span class="text-xs font-bold opacity-70 group-hover:opacity-100 transition-opacity">
                              {t("frontend/src/routes/assignments/[id]/+page.svelte::auto_award_points_ai")}
                            </span>
                          </label>
                        </div>
                        
                        <div class="form-control">
                          <div class="flex items-center justify-between mb-2">
                            <span class="text-xs font-bold opacity-70">
                              {t("frontend/src/routes/assignments/[id]/+page.svelte::strictness_label")}
                            </span>
                            <span class="badge badge-sm badge-primary font-black font-mono">{eLLMStrictness}%</span>
                          </div>
                          <input
                            id="ai-strictness-range"
                            type="range"
                            min="0"
                            max="100"
                            step="5"
                            class="range range-primary range-xs"
                            bind:value={eLLMStrictness}
                          />
                          <p class="text-[10px] italic opacity-60 mt-2 px-1">
                            {eLLMStrictnessMessage}
                          </p>
                        </div>

                        <div class="form-control">
                          <button
                            type="button"
                            class="btn btn-sm btn-ghost gap-2 w-fit px-2 -ml-2 hover:bg-primary/10"
                            on:click={() => (showRubric = !showRubric)}
                          >
                            <List size={14} />
                            <span class="text-xs font-bold">
                              {showRubric
                                ? t("frontend/src/routes/assignments/[id]/+page.svelte::hide_rubric_button")
                                : t("frontend/src/routes/assignments/[id]/+page.svelte::add_rubric_button")}
                            </span>
                          </button>
                          {#if showRubric}
                            <textarea
                              class="textarea textarea-bordered min-h-[5rem] text-xs bg-base-100 mt-2"
                              bind:value={eLLMRubric}
                              placeholder={t("frontend/src/routes/assignments/[id]/+page.svelte::rubric_placeholder")}
                            ></textarea>
                          {/if}
                        </div>
                      </div>
                    {/if}
                  </div>
                {/if}
              </div>
            </section>

            <!-- Template (collapsible) -->
            <section class="animate-in fade-in duration-700">
              <details class="group bg-base-200/30 border border-base-300/40 rounded-2xl overflow-hidden transition-all duration-300">
                <summary class="flex items-center justify-between p-4 cursor-pointer hover:bg-base-300/20 list-none">
                  <div class="flex items-center gap-3">
                    <div class="p-1.5 rounded-lg bg-base-300/50 text-base-content/70">
                      <FileCode size={16} />
                    </div>
                    <span class="text-xs font-black uppercase tracking-widest opacity-60">
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::assignment_template_heading")}
                    </span>
                  </div>
                  <ChevronDown size={16} class="opacity-40 group-open:rotate-180 transition-transform duration-300" />
                </summary>
                <div class="p-5 pt-1 space-y-4">
                  <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
                    <div class="relative flex-1 group/file">
                      <input
                        type="file"
                        class="file-input file-input-bordered file-input-sm w-full bg-base-100"
                        on:change={(e) =>
                          (templateFile =
                            (e.target as HTMLInputElement).files?.[0] || null)}
                      />
                    </div>
                    <button
                      class="btn btn-sm btn-outline gap-2"
                      on:click={uploadTemplate}
                      disabled={!templateFile}
                    >
                      <Upload size={14} />
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::upload_template_button")}
                    </button>
                  </div>
                  
                  {#if assignment.template_path}
                    <div class="flex items-center justify-between p-3 bg-base-100 rounded-xl border border-base-300/40">
                      <div class="flex items-center gap-2 truncate">
                        <Check size={14} class="text-success shrink-0" />
                        <span class="text-xs font-mono opacity-60 truncate">{assignment.template_path.split("/").pop()}</span>
                      </div>
                      <button
                        class="btn btn-ghost btn-xs gap-1.5"
                        on:click|preventDefault={downloadTemplate}
                      >
                        <Download size={14} />
                        {t("frontend/src/routes/assignments/[id]/+page.svelte::download_current_template_button")}
                      </button>
                    </div>
                  {/if}
                </div>
              </details>
            </section>
          </div>

          <!-- Sticky actions / summary -->
          <aside class="lg:col-span-4 bg-base-200/20 p-6 space-y-6">
            <div class="lg:sticky lg:top-8 space-y-6">
              <div class="flex items-center gap-2 px-1">
                <div class="p-1.5 rounded-lg bg-primary/10 text-primary">
                  <Activity size={16} />
                </div>
                <h3 class="font-black text-[11px] uppercase tracking-widest opacity-60">
                  {t("frontend/src/routes/assignments/[id]/+page.svelte::actions_heading")}
                </h3>
              </div>

              <div class="bg-base-100 rounded-2xl p-5 border border-base-300/50 shadow-sm space-y-4">
                <div class="space-y-4">
                  <div class="flex justify-between items-start">
                    <span class="text-[10px] font-black uppercase tracking-wider opacity-40 mt-1">{t("frontend/src/routes/assignments/[id]/+page.svelte::policy_label")}</span>
                    <span class="badge badge-sm badge-ghost font-bold py-2 px-3">{policyLabel(ePolicy)}</span>
                  </div>
                  
                  <div class="flex justify-between items-start">
                    <span class="text-[10px] font-black uppercase tracking-wider opacity-40 mt-1">{t("frontend/src/routes/assignments/[id]/+page.svelte::max_points_sidebar_label")}</span>
                    {#if ePolicy === "all_or_nothing"}
                      <span class="text-lg font-black text-primary">{ePoints}<span class="text-[10px] opacity-40 ml-1 uppercase">pts</span></span>
                    {:else}
                      <span class="text-[10px] font-bold opacity-60 italic text-right max-w-[120px]">{t("frontend/src/routes/assignments/[id]/+page.svelte::from_test_weights_label")}</span>
                    {/if}
                  </div>

                  <div class="divider opacity-5 my-0"></div>

                  <div class="space-y-3 pt-1">
                    <div class="flex items-center gap-3">
                      <div class="p-1.5 rounded-lg bg-base-200 text-base-content/50">
                        <Calendar size={14} />
                      </div>
                      <div class="flex flex-col min-w-0">
                        <span class="text-[9px] font-black uppercase tracking-wider opacity-40 leading-none mb-1">Main Deadline</span>
                        <span class="text-xs font-mono font-medium truncate">{formatDeadlineDisplay(eDeadline)}</span>
                      </div>
                    </div>

                    {#if showAdvancedOptions}
                      <div class="flex items-center gap-3">
                        <div class="p-1.5 rounded-lg bg-warning/10 text-warning">
                          <Calendar size={14} />
                        </div>
                        <div class="flex flex-col min-w-0">
                          <span class="text-[9px] font-black uppercase tracking-wider text-warning leading-none mb-1">Late Deadline</span>
                          <span class="text-xs font-mono font-medium truncate">{formatDeadlineDisplay(eSecondDeadline)}</span>
                        </div>
                      </div>
                    {/if}
                  </div>
                </div>

                <div class="pt-4 space-y-2">
                  <button class="btn btn-primary btn-md w-full shadow-lg shadow-primary/20 gap-2 h-12" on:click={saveEdit}>
                    <Check size={18} />
                    <span class="font-black uppercase tracking-widest text-[11px]">{t("frontend/src/routes/assignments/[id]/+page.svelte::save_changes_button")}</span>
                  </button>
                  <button class="btn btn-ghost btn-sm w-full opacity-60 hover:opacity-100 h-10" on:click={() => (editing = false)}>
                    <X size={16} />
                    <span class="font-bold uppercase tracking-wider text-[10px]">{t("frontend/src/routes/assignments/[id]/+page.svelte::cancel_button")}</span>
                  </button>
                </div>
              </div>

              <!-- Danger Zone Section -->
              <div class="pt-6 border-t border-base-300/30">
                <button
                  class="btn btn-sm btn-ghost text-error hover:bg-error/10 w-full gap-2 group transition-all"
                  on:click={delAssignment}
                >
                  <Trash2 size={16} class="opacity-60 group-hover:opacity-100" />
                  <span class="font-bold uppercase tracking-wider text-[10px]">{t("frontend/src/routes/assignments/[id]/+page.svelte::delete_button")}</span>
                </button>
              </div>
            </div>
          </aside>
        </div>
      </div>
    </div>
  {:else}
    <!-- Hero header -->
    <section class="relative overflow-hidden mb-8 rounded-3xl border border-base-300 bg-gradient-to-br from-primary/15 via-base-100 to-secondary/15 shadow-xl shadow-primary/5">
      <!-- Decorative background elements -->
      <div class="absolute top-0 right-0 -mt-20 -mr-20 w-64 h-64 bg-primary/10 rounded-full blur-3xl"></div>
      <div class="absolute bottom-0 left-0 -mb-20 -ml-20 w-64 h-64 bg-secondary/10 rounded-full blur-3xl"></div>

      <div class="flex flex-col md:flex-row items-stretch md:items-center relative z-10">
        <div class="flex-1 p-8 sm:p-10">
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-6">
            <div class="space-y-4">

              <h1 class="text-2xl sm:text-3xl lg:text-4xl font-black tracking-tight text-base-content leading-tight">
                {assignment.title}
              </h1>
              
              <div class="flex flex-wrap items-center gap-2">
                <div class={`badge h-7 gap-2 px-2.5 font-black text-[9px] uppercase tracking-wider border-none shadow-sm ${isOverdue ? 'bg-error text-error-content' : 'bg-primary text-primary-content'}`}>
                  <Clock size={12} />
                  {deadlineLabel}
                </div>
                
                {#if assignment.second_deadline}
                  <div class="badge h-7 gap-2 px-2.5 font-black text-[9px] uppercase tracking-wider bg-warning text-warning-content border-none shadow-sm">
                    <AlertTriangle size={12} />
                    {secondDeadlineLabel}
                  </div>
                {/if}

                <div class="badge h-7 gap-2 px-2.5 font-black text-[9px] uppercase tracking-wider bg-base-200 text-base-content border-none shadow-sm">
                  <Trophy size={12} class="text-warning" />
                  {t("frontend/src/routes/assignments/[id]/+page.svelte::max_points_badge", { maxPoints: assignment.max_points })}
                </div>

                {#if assignment.programming_language === "scratch"}
                  <div class="badge h-7 gap-2 px-2.5 font-black text-[9px] uppercase tracking-wider bg-secondary text-secondary-content border-none shadow-sm">
                    <span class="font-black">S</span>
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_badge")}
                  </div>
                {/if}

                {#if isScratchAssignment}
                  <div
                    class={`badge h-7 gap-2 px-2.5 font-black text-[9px] uppercase tracking-wider border-none shadow-sm ${
                      scratchModeView === "automatic"
                        ? "bg-success text-success-content"
                        : scratchModeView === "semi_automatic"
                          ? "bg-warning text-warning-content"
                          : "bg-info text-info-content"
                    }`}
                  >
                    <GraduationCap size={12} />
                    {scratchModeLabel(scratchModeView)}
                  </div>
                {:else if assignment.manual_review}
                  <div class="badge h-7 gap-2 px-2.5 font-black text-[9px] uppercase tracking-wider bg-info text-info-content border-none shadow-sm">
                    <GraduationCap size={12} />
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::manual_review_badge")}
                  </div>
                {/if}

                {#if role !== "student"}
                   <div class={`badge h-7 gap-2 px-2.5 font-black text-[9px] uppercase tracking-wider border-none shadow-sm ${assignment.published ? 'bg-success text-success-content' : 'bg-warning text-warning-content'}`}>
                      {#if assignment.published}
                        <Globe size={12} />
                        {t("frontend/src/routes/assignments/[id]/+page.svelte::published_badge")}
                      {:else}
                        <Edit3 size={12} />
                        {t("frontend/src/routes/assignments/[id]/+page.svelte::draft_badge")}
                      {/if}
                   </div>
                {/if}

                {#if done}
                  <div class="badge h-7 gap-2 px-2.5 font-black text-[9px] uppercase tracking-wider bg-success text-success-content border-none shadow-sm">
                    <CheckCircle2 size={12} />
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::completed_badge")}
                  </div>
                {/if}
              </div>
            </div>

             {#if role === "student" && !assignment.manual_review && !isScratchAssignment && testsCount > 0}
               <div class="flex flex-col items-center gap-1.5 bg-base-100/60 backdrop-blur-md p-4 rounded-3xl border border-primary/20 shadow-xl shadow-primary/10 min-w-[110px] animate-in zoom-in-95 duration-500">
                  <div class="radial-progress text-primary font-black" style="--value:{testsPercent}; --size:3.5rem; --thickness: 6px;" aria-valuenow={testsPercent} role="progressbar">
                    <span class="text-xs">{testsPercent}%</span>
                  </div>
                  <div class="text-center">
                    <div class="text-[8px] font-black uppercase tracking-widest opacity-40">{t("frontend/src/routes/assignments/[id]/+page.svelte::tests_label")}</div>
                    <div class="font-black text-lg">{testsPassed}/{results.length || testsCount}</div>
                  </div>
               </div>
            {/if}
          </div>

          <div class="mt-8 flex flex-wrap items-center gap-3">
             {#if role === "teacher" || role === "admin"}
               {#if !assignment.published}
                 <button class="btn btn-secondary shadow-lg shadow-secondary/20 font-black uppercase tracking-widest text-[10px] gap-2 h-10 px-4" on:click={publish}>
                   <Send size={14} />
                   {t("frontend/src/routes/assignments/[id]/+page.svelte::publish_button")}
                 </button>
               {/if}
               {#if !isScratchAssignment}
                 <button class="btn btn-primary shadow-lg shadow-primary/20 font-black uppercase tracking-widest text-[10px] gap-2 h-10 px-4" on:click={openTestsModal}>
                   <FlaskConical size={14} />
                   {t("frontend/src/routes/assignments/[id]/+page.svelte::manage_tests_button")}
                 </button>
               {/if}
               <button class="btn bg-base-100 hover:bg-base-200 border-base-300 font-black uppercase tracking-widest text-[10px] gap-2 h-10 px-4 shadow-sm" on:click={startEdit}>
                 <Settings2 size={14} />
                 {t("frontend/src/routes/assignments/[id]/+page.svelte::edit_button")}
               </button>
               {#if teacherGroupSync?.needs_update}
                 <button class="btn btn-warning shadow-lg shadow-warning/20 font-black uppercase tracking-widest text-[10px] gap-2 h-10 px-4" on:click={syncTeachersGroup} disabled={syncTGLoading}>
                   <Activity size={14} class={syncTGLoading ? 'animate-spin' : ''} />
                   {syncTGLoading ? t("frontend/src/routes/assignments/[id]/+page.svelte::syncing_teachers_group_button") : t("frontend/src/routes/assignments/[id]/+page.svelte::update_teachers_group_button")}
                 </button>
               {/if}
               <button class="btn btn-ghost text-error hover:bg-error/10 font-black uppercase tracking-widest text-[10px] gap-2 h-10 px-4 ml-auto" on:click={delAssignment}>
                 <Trash2 size={14} />
                 {t("frontend/src/routes/assignments/[id]/+page.svelte::delete_button")}
               </button>
            {:else}
              <button class="btn btn-primary shadow-2xl shadow-primary/30 font-black uppercase tracking-[0.1em] h-12 px-6 gap-3 rounded-xl animate-in fade-in slide-in-from-bottom-2 duration-700 text-xs" on:click={openSubmitModal} disabled={assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}>
                <Send size={18} />
                {t("frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_button")}
              </button>
              {#if assignment.template_path}
                 <button class="btn btn-ghost border-base-300 h-12 px-5 font-black uppercase tracking-widest text-[10px] gap-3 rounded-xl" on:click|preventDefault={downloadTemplate}>
                   <Download size={18} />
                   {t("frontend/src/routes/assignments/[id]/+page.svelte::download_template_button")}
                 </button>
              {/if}
            {/if}
          </div>
        </div>

        {#if role === "student"}
          <div class="md:w-64 bg-primary/5 backdrop-blur-md p-6 md:border-l border-primary/10 flex flex-col items-center justify-center gap-4 relative overflow-hidden">
             <div class="absolute inset-0 opacity-10 pointer-events-none">
                <Trophy size={160} class="absolute -bottom-10 -right-10 rotate-12" />
             </div>
             <div class="relative">
                <div class="radial-progress text-primary shadow-2xl shadow-primary/20 bg-base-100/50" style="--value:{percent}; --size:7rem; --thickness:12px" aria-valuenow={percent} role="progressbar">
                  <span class="text-2xl font-black">{percent}%</span>
                </div>
                <div class="absolute -top-1 -right-1 bg-warning text-warning-content rounded-full p-2 shadow-xl scale-110 border-4 border-base-100">
                   <Trophy size={14} />
                </div>
             </div>
             <div class="text-center space-y-1 relative z-10">
                <div class="text-[9px] font-black uppercase tracking-[0.2em] opacity-40">{t("frontend/src/routes/assignments/[id]/+page.svelte::points_earned_label")}</div>
                <div class="text-3xl font-black text-primary tabular-nums">
                  {pointsEarned}<span class="text-base-content/20 text-lg ml-1">/ {assignment.max_points}</span>
                </div>
             </div>
          </div>
        {/if}
      </div>
    </section>

    {#if role === "student" && isScratchAssignment}
      <div class="alert bg-info/10 border-info/20 text-info-content mb-6 rounded-2xl shadow-sm">
        <Info size={20} />
        <span class="font-medium text-sm">
          {scratchModeView === "automatic"
            ? t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_auto_student_alert")
            : scratchModeView === "semi_automatic"
              ? t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_semi_student_alert")
              : t("frontend/src/routes/assignments/[id]/+page.svelte::scratch_mode_manual_student_alert")}
        </span>
      </div>
    {:else if role === "student" && assignment.manual_review}
      <div class="alert bg-info/10 border-info/20 text-info-content mb-6 rounded-2xl shadow-sm">
        <Info size={20} />
        <span class="font-medium text-sm">{t("frontend/src/routes/assignments/[id]/+page.svelte::teacher_review_alert_body")}</span>
      </div>
    {/if}
    {#if deadlineSoon}
      <div class="alert bg-warning/10 border-warning/20 text-warning-content mb-6 rounded-2xl shadow-sm">
        <Clock size={20} />
        <span class="font-bold text-sm">{t("frontend/src/routes/assignments/[id]/+page.svelte::deadline_near_alert")}</span>
      </div>
    {/if}
    {#if secondDeadlineSoon}
      <div class="alert bg-warning/10 border-warning/20 text-warning-content mb-6 rounded-2xl shadow-sm">
        <AlertTriangle size={20} />
        <span class="font-bold text-sm">{t("frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_near_alert", { penalty: Math.round(assignment.late_penalty_ratio * 100) })}</span>
      </div>
    {/if}

    <!-- Content with tabs and optional sidebar for students -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-8">
      <div
        class={`col-span-full ${role === "student" ? "lg:col-span-8" : "lg:col-span-12"}`}
      >
        <!-- Custom styled tabs -->
        <div class="flex flex-wrap items-center gap-1.5 p-1 bg-base-200/50 backdrop-blur-sm rounded-[1rem] border border-base-300/50 mb-6 max-w-fit shadow-inner">
          <button
            class={`flex items-center gap-2 px-4 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-widest transition-all duration-300 ${activeTab === "overview" ? "bg-base-100 text-primary shadow-lg shadow-base-300 scale-[1.02] border border-base-300" : "hover:bg-base-300/50 opacity-50 hover:opacity-100"}`}
            on:click={() => setTab("overview")}
          >
            <LayoutDashboard size={12} />
            {t("frontend/src/routes/assignments/[id]/+page.svelte::tab_overview")}
          </button>
          {#if role === "student"}
            <button
              class={`flex items-center gap-2 px-4 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-widest transition-all duration-300 ${activeTab === "submissions" ? "bg-base-100 text-primary shadow-lg shadow-base-300 scale-[1.02] border border-base-300" : "hover:bg-base-300/50 opacity-50 hover:opacity-100"}`}
              on:click={() => setTab("submissions")}
            >
              <History size={12} />
              {t("frontend/src/routes/assignments/[id]/+page.svelte::tab_submissions")}
            </button>
            {#if !assignment.manual_review && !isScratchAssignment}
              <button
                class={`flex items-center gap-2 px-4 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-widest transition-all duration-300 ${activeTab === "results" ? "bg-base-100 text-primary shadow-lg shadow-base-300 scale-[1.02] border border-base-300" : "hover:bg-base-300/50 opacity-50 hover:opacity-100"}`}
                on:click={() => setTab("results")}
              >
                <Activity size={12} />
                {t("frontend/src/routes/assignments/[id]/+page.svelte::tab_results")}
              </button>
            {/if}
          {/if}
          {#if role === "teacher" || role === "admin"}
            <button
              class={`flex items-center gap-2 px-4 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-widest transition-all duration-300 ${activeTab === "instructor" ? "bg-base-100 text-primary shadow-lg shadow-base-300 scale-[1.02] border border-base-300" : "hover:bg-base-300/50 opacity-50 hover:opacity-100"}`}
              on:click={() => setTab("instructor")}
            >
              <GraduationCap size={12} />
              {t("frontend/src/routes/assignments/[id]/+page.svelte::tab_instructor")}
            </button>
            {#if !isScratchAssignment}
              <button
                class={`flex items-center gap-2 px-4 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-widest transition-all duration-300 ${activeTab === "teacher-runs" ? "bg-base-100 text-primary shadow-lg shadow-base-300 scale-[1.02] border border-base-300" : "hover:bg-base-300/50 opacity-50 hover:opacity-100"}`}
                on:click={() => setTab("teacher-runs")}
              >
                <FlaskConical size={12} />
                {t("frontend/src/routes/assignments/[id]/+page.svelte::tab_teacher_runs")}
              </button>
            {/if}
          {/if}
        </div>

        {#if activeTab === "overview"}
          <article class="space-y-4">
            <div class="card-elevated p-5 sm:p-6 space-y-5 bg-base-100 rounded-[1.25rem] border border-base-200">
               <div class="flex items-center gap-3 border-b border-base-200 pb-4 mb-0.5">
                  <div class="p-2 bg-primary/10 rounded-lg text-primary">
                    <ListTodo size={18} />
                  </div>
                  <div>
                    <h3 class="text-base font-black">{t("frontend/src/routes/assignments/[id]/+page.svelte::basic_info_heading")}</h3>
                    <p class="text-[9px] font-bold opacity-40 uppercase tracking-widest">
                      {isScratchAssignment
                        ? scratchModeLabel(scratchModeView)
                        : assignment.manual_review
                          ? t("frontend/src/routes/assignments/[id]/+page.svelte::manual_teacher_review_option")
                          : t("frontend/src/routes/assignments/[id]/+page.svelte::automatic_tests_option")}
                    </p>
                  </div>
               </div>

              <div class="markdown assignment-description text-base-content/90 leading-relaxed font-medium">{@html safeDesc}</div>
              
              {#if role === "student" && assignment.second_deadline && new Date() > assignment.deadline && new Date() <= assignment.second_deadline}
                <div class="alert bg-warning/10 border-warning/20 text-warning-content rounded-2xl flex items-start gap-3">
                  <AlertTriangle size={20} class="mt-0.5 shrink-0" />
                  <div class="text-sm">
                    <strong class="font-black uppercase tracking-wider text-[10px] block mb-1">{t("frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_active_alert_strong")}</strong>
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_active_alert_body", { penalty: Math.round(assignment.late_penalty_ratio * 100) })}
                    <div class="mt-2 font-black tabular-nums">{t("frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_label_with_date")}: {formatDateTime(assignment.second_deadline)}</div>
                  </div>
                </div>
              {:else if role === "student" && assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}
                <div class="alert bg-error/10 border-error/20 text-error-content rounded-2xl flex items-start gap-3">
                  <AlertTriangle size={20} class="mt-0.5 shrink-0" />
                  <div class="text-sm">
                    <strong class="font-black uppercase tracking-wider text-[10px] block mb-1">{t("frontend/src/routes/assignments/[id]/+page.svelte::all_deadlines_passed_alert_strong")}</strong>
                    {t("frontend/src/routes/assignments/[id]/+page.svelte::all_deadlines_passed_alert_body")}
                  </div>
                </div>
              {/if}

              <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-4 border-t border-base-200 pt-8">
                <div class="bg-base-200/30 p-4 rounded-xl border border-base-200 space-y-2.5 hover:border-primary/20 transition-all group">
                  <div class="flex items-center gap-2 opacity-40 group-hover:opacity-100 transition-opacity">
                    <Calendar size={14} class="text-primary" />
                    <span class="text-[10px] font-black uppercase tracking-widest">{t("frontend/src/routes/assignments/[id]/+page.svelte::stat_deadline_title")}</span>
                  </div>
                  <div class="space-y-1">
                    <div class="font-black text-base truncate" title={formatDateTime(assignment.deadline)}>{formatDateTime(assignment.deadline)}</div>
                    <div class="text-[10px] font-bold text-primary uppercase tracking-wider bg-primary/10 px-2 py-0.5 rounded-full w-fit">{relativeToDeadline(assignment.deadline)}</div>
                  </div>
                </div>

                {#if assignment.second_deadline}
                  <div class="bg-base-200/30 p-5 rounded-2xl border border-base-200 space-y-3 hover:border-warning/50 transition-all group">
                    <div class="flex items-center gap-2 opacity-40 group-hover:opacity-100 transition-opacity">
                      <Clock size={14} class="text-warning" />
                      <span class="text-[10px] font-black uppercase tracking-widest">{t("frontend/src/routes/assignments/[id]/+page.svelte::stat_second_deadline_title")}</span>
                    </div>
                    <div class="space-y-1">
                      <div class="font-black text-base truncate" title={formatDateTime(assignment.second_deadline)}>{formatDateTime(assignment.second_deadline)}</div>
                      <div class="text-[10px] font-bold text-warning uppercase tracking-wider bg-warning/10 px-2 py-0.5 rounded-full w-fit">{Math.round(assignment.late_penalty_ratio * 100)}% {t("frontend/src/routes/assignments/[id]/+page.svelte::points_label")}</div>
                    </div>
                  </div>
                {/if}

                <div class="bg-base-200/30 p-4 rounded-xl border border-base-200 space-y-2.5 hover:border-primary/20 transition-all group">
                  <div class="flex items-center gap-2 opacity-40 group-hover:opacity-100 transition-opacity">
                    <Trophy size={14} class="text-warning" />
                    <span class="text-[10px] font-black uppercase tracking-widest">{t("frontend/src/routes/assignments/[id]/+page.svelte::stat_max_points_title")}</span>
                  </div>
                  <div class="space-y-1">
                    <div class="font-black text-2xl tabular-nums">{assignment.max_points}</div>
                    <div class="text-[10px] font-bold text-base-content/40 uppercase tracking-widest">{policyLabel(assignment.grading_policy)}</div>
                  </div>
                </div>

                {#if role !== "student"}
                  <div class="bg-base-200/30 p-5 rounded-2xl border border-base-200 space-y-3 transition-all group">
                    <div class="flex items-center gap-2 opacity-40 group-hover:opacity-100 transition-opacity">
                      <Activity size={14} class="text-secondary" />
                      <span class="text-[10px] font-black uppercase tracking-widest">{t("frontend/src/routes/assignments/[id]/+page.svelte::stat_status_title")}</span>
                    </div>
                    <div class="space-y-1">
                      <div class="font-black text-base uppercase tracking-wider">
                        {#if assignment.published}
                           <span class="text-success">{t("frontend/src/routes/assignments/[id]/+page.svelte::stat_published_value")}</span>
                        {:else}
                           <span class="text-warning">{t("frontend/src/routes/assignments/[id]/+page.svelte::stat_draft_value")}</span>
                        {/if}
                      </div>
                      <div class="text-[10px] font-bold text-base-content/40 uppercase tracking-widest">{t("frontend/src/routes/assignments/[id]/+page.svelte::stat_assignment_visibility_desc")}</div>
                    </div>
                  </div>
                {/if}
              </div>
            </div>
            
            {#if role === "student" && submissions.length > 0}
               <div class="bg-primary/5 rounded-[2rem] p-8 border border-primary/10 flex flex-col md:flex-row items-center gap-8 animate-in fade-in slide-in-from-top-4 duration-700">
                  <div class="relative group">
                    <div class="absolute inset-0 bg-primary/20 rounded-full blur-2xl group-hover:blur-3xl transition-all duration-500 opacity-50"></div>
                    <div class="radial-progress text-primary bg-base-100 shadow-2xl shadow-primary/20 relative z-10" style="--value:{percent}; --size:6rem; --thickness:10px">
                       <span class="text-xl font-black">{percent}%</span>
                    </div>
                  </div>
                  <div class="flex-1 space-y-2.5 text-center md:text-left">
                     <h4 class="font-black text-xl tracking-tight">
                       {#if latestSub && assignment.deadline && new Date(latestSub.created_at) > new Date(assignment.deadline)}
                         {t("frontend/src/routes/assignments/[id]/+page.svelte::late_submission_message", { name: ($auth as any)?.display_name ?? ($auth as any)?.name ?? 'there' })}
                       {:else if percent >= 90}
                         {t("frontend/src/routes/assignments/[id]/+page.svelte::excellent_work_message", { name: ($auth as any)?.display_name ?? ($auth as any)?.name ?? 'there' })}
                       {:else if percent >= 70}
                         {t("frontend/src/routes/assignments/[id]/+page.svelte::good_job_message", { name: ($auth as any)?.display_name ?? ($auth as any)?.name ?? 'there' })}
                       {:else if percent >= 50}
                         {t("frontend/src/routes/assignments/[id]/+page.svelte::keep_going_message", { name: ($auth as any)?.display_name ?? ($auth as any)?.name ?? 'there' })}
                       {:else if percent > 0}
                         {t("frontend/src/routes/assignments/[id]/+page.svelte::keep_trying_message", { name: ($auth as any)?.display_name ?? ($auth as any)?.name ?? 'there' })}
                       {:else}
                         {t("frontend/src/routes/assignments/[id]/+page.svelte::get_started_message", { name: ($auth as any)?.display_name ?? ($auth as any)?.name ?? 'there' })}
                       {/if}
                     </h4>
                     <p class="text-sm text-base-content/60 leading-relaxed font-medium max-w-xl">
                        {t("frontend/src/routes/assignments/[id]/+page.svelte::overview_progress_desc", { points: pointsEarned, max: assignment.max_points })}
                     </p>
                     <div class="pt-3 flex flex-wrap justify-center md:justify-start gap-2.5">
                        <button class="btn bg-base-100 hover:bg-base-200 border-base-300 rounded-xl px-6 h-10 font-black uppercase tracking-widest text-[10px] shadow-sm transform transition-all active:scale-95" on:click={() => setTab('submissions')}>
                           <History size={14} class="text-primary" />
                           {t("frontend/src/routes/assignments/[id]/+page.svelte::view_my_submissions")}
                        </button>
                        <button class="btn btn-primary rounded-xl px-6 h-10 font-black uppercase tracking-widest text-[10px] shadow-lg shadow-primary/20 transform transition-all active:scale-95" on:click={openSubmitModal}>
                           <Send size={14} />
                           {t("frontend/src/routes/assignments/[id]/+page.svelte::submit_new_attempt")}
                        </button>
                     </div>
                  </div>
               </div>
            {/if}
          </article>
        {/if}

        {#if activeTab === "submissions" && role === "student"}
          <section class="card-elevated p-8 sm:p-10 bg-base-100 rounded-[2rem] border border-base-200">
            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-6 mb-10">
              <div class="flex items-center gap-4">
                 <div class="p-4 bg-primary/10 rounded-[1.25rem] text-primary shadow-inner">
                    <History size={24} />
                 </div>
                 <div>
                    <h3 class="text-2xl font-black tracking-tight">{t("frontend/src/routes/assignments/[id]/+page.svelte::your_submissions_heading")}</h3>
                    <p class="text-[10px] font-black opacity-40 uppercase tracking-[0.2em]">{submissions.length} {t("frontend/src/routes/assignments/[id]/+page.svelte::total_attempts_label")}</p>
                 </div>
              </div>
              <button
                class="btn btn-primary rounded-2xl px-8 h-12 font-black uppercase tracking-widest text-[11px] shadow-lg shadow-primary/20"
                on:click={openSubmitModal}
                disabled={assignment.second_deadline &&
                  new Date() > assignment.deadline &&
                  new Date() > assignment.second_deadline}
              >
                <Upload size={16} />
                {t("frontend/src/routes/assignments/[id]/+page.svelte::new_submission_button")}
              </button>
            </div>
            
            {#if assignment.second_deadline && new Date() > assignment.deadline && new Date() <= assignment.second_deadline}
              <div class="alert bg-warning/10 border-warning/20 text-warning-content rounded-2xl flex items-start gap-4 mb-8">
                <Clock size={20} class="mt-0.5 shrink-0" />
                <div class="text-sm">
                  <strong class="font-black uppercase tracking-widest text-[10px] block mb-1">{t("frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_period_info_strong")}</strong>
                  {t("frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_period_info_body", { penalty: Math.round(assignment.late_penalty_ratio * 100) })}
                </div>
              </div>
            {:else if assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}
              <div class="alert bg-error/10 border-error/20 text-error-content rounded-2xl flex items-start gap-4 mb-8">
                <AlertTriangle size={20} class="mt-0.5 shrink-0" />
                <div class="text-sm">
                  <strong class="font-black uppercase tracking-widest text-[10px] block mb-1">{t("frontend/src/routes/assignments/[id]/+page.svelte::all_deadlines_passed_alert_strong")}</strong>
                  {t("frontend/src/routes/assignments/[id]/+page.svelte::all_deadlines_passed_alert_body")}
                </div>
              </div>
            {/if}

            <div class="overflow-x-auto -mx-2">
              <table class="table w-full">
                <thead>
                  <tr>
                    <th class="pl-6">{t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_header_attempt")}</th>
                    <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_header_date")}</th>
                    <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_header_deadline")}</th>
                    <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_header_status")}</th>
                    {#if testsCount > 0}
                      <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_header_passed")}</th>
                      <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_header_points")}</th>
                    {/if}
                    <th class="pr-6"></th>
                  </tr>
                </thead>
                <tbody class="text-sm font-medium">
                  {#each submissions as s}
                    <tr class="hover:bg-base-200/50 transition-colors group border-b border-base-200/50 last:border-none">
                      <td class="pl-6">
                        <div class="font-black text-base tabular-nums">
                          #{s.attempt_number ?? "?"}
                        </div>
                      </td>
                      <td>
                        <div class="flex flex-col">
                           <span>{formatDateTime(s.created_at)}</span>
                           <span class="text-[10px] opacity-40 font-bold">{relativeToDeadline(s.created_at)}</span>
                        </div>
                      </td>
                      <td>
                        {#if s.created_at > assignment.deadline}
                          {#if assignment.second_deadline && s.created_at <= assignment.second_deadline}
                            <div class="badge bg-warning/10 text-warning border-none font-black text-[10px] uppercase tracking-wider py-3">
                              {t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_badge_second_deadline", { penalty: Math.round(assignment.late_penalty_ratio * 100) })}
                            </div>
                          {:else}
                            <div class="badge bg-error/10 text-error border-none font-black text-[10px] uppercase tracking-wider py-3">
                              {t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_badge_late")}
                            </div>
                          {/if}
                        {:else}
                          <div class="badge bg-success/10 text-success border-none font-black text-[10px] uppercase tracking-wider py-3">
                            {t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_badge_on_time")}
                          </div>
                        {/if}
                      </td>
                      <td>
                        <div class={`badge border-none font-black text-[10px] uppercase tracking-wider py-3 ${statusColor(s.status).replace('badge-', 'bg-')}/20 ${statusColor(s.status).replace('badge-', 'text-')}`}>
                          {statusLabel(s.status)}
                        </div>
                      </td>
                      {#if testsCount > 0}
                        <td>
                          {#if s.tests_passed !== undefined}
                            <div class="flex items-center gap-3">
                               <div class="w-16 bg-base-300 rounded-full h-2 overflow-hidden shadow-inner">
                                  <div class="bg-primary h-full rounded-full" style="width: {(s.tests_passed / (s.tests_total || testsCount)) * 100}%"></div>
                               </div>
                               <span class="font-black tabular-nums">{s.tests_passed}/{s.tests_total || testsCount}</span>
                            </div>
                          {:else}
                            <span class="opacity-20 font-black">—</span>
                          {/if}
                        </td>
                        <td>
                          {#if s.points_earned !== undefined}
                            <span class="font-black text-primary tabular-nums text-lg">{s.points_earned}</span>
                            <span class="text-[10px] opacity-40 ml-0.5 font-bold">/ {assignment.max_points}</span>
                          {:else}
                            <span class="opacity-20 font-black">—</span>
                          {/if}
                        </td>
                      {/if}
                      <td class="pr-6 text-right">
                         <a
                           class="btn btn-ghost btn-circle btn-sm opacity-0 group-hover:opacity-100 transition-all hover:bg-primary hover:text-primary-content"
                           href={`/submissions/${s.id}?fromTab=${activeTab}`}
                           on:click={saveState}
                         >
                           <ArrowRight size={18} />
                         </a>
                      </td>
                    </tr>
                  {/each}
                  {#if !submissions.length}
                    <tr>
                      <td colspan={testsCount > 0 ? 7 : 5} class="py-24 text-center">
                         <div class="flex flex-col items-center gap-4 opacity-20">
                            <History size={64} strokeWidth={1} />
                            <p class="font-black uppercase tracking-[0.3em] text-xs">{t("frontend/src/routes/assignments/[id]/+page.svelte::no_submissions_yet_table")}</p>
                         </div>
                      </td>
                    </tr>
                  {/if}
                </tbody>
              </table>
            </div>
          </section>
        {/if}


        {#if activeTab === "results" && role === "student"}
          <section class="card-elevated p-8 sm:p-10 bg-base-100 rounded-[2rem] border border-base-200">
            <div class="flex items-center gap-4 mb-10">
               <div class="p-4 bg-primary/10 rounded-[1.25rem] text-primary shadow-inner">
                  <Activity size={24} />
               </div>
               <div>
                  <h3 class="text-2xl font-black tracking-tight">{t("frontend/src/routes/assignments/[id]/+page.svelte::latest_results_heading")}</h3>
                  {#if latestSub}
                    <p class="text-[10px] font-black opacity-40 uppercase tracking-[0.2em]">{t("frontend/src/routes/assignments/[id]/+page.svelte::attempt_label")}{latestSub.attempt_number ?? "?"}</p>
                  {/if}
               </div>
               {#if latestSub}
                  <div class="ml-auto">
                     <a
                       class="btn bg-base-100 hover:bg-base-200 border-base-300 rounded-2xl px-6 h-12 font-black uppercase tracking-widest text-[11px] shadow-sm transform transition-all active:scale-95"
                       href={`/submissions/${latestSub.id}?fromTab=${activeTab}`}
                       on:click={saveState}
                     >
                       <Eye size={16} class="mr-2" />
                       {t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_view_button")}
                     </a>
                  </div>
               {/if}
            </div>

            {#if latestSub}
              {#if assignment.llm_help_why_failed && allTestsFailed}
                <div class="mb-6">
                  {#if summaryExplanation.text}
                    <div class="p-3 bg-base-200 rounded-lg text-xs border border-base-300 shadow-sm max-w-xl text-left">
                      <div class="flex gap-2 items-start">
                        <Sparkles
                          size={14}
                          class="text-primary mt-0.5 shrink-0"
                        />
                        <span class="leading-relaxed"
                          >{summaryExplanation.text}</span
                        >
                      </div>
                    </div>
                  {:else}
                    <button
                      class="btn btn-xs btn-ghost gap-1 text-[10px] font-bold text-primary opacity-60 hover:opacity-100"
                      on:click={() => askWhyAllFailed(latestSub.id)}
                      disabled={summaryExplanation.loading}
                    >
                      {#if summaryExplanation.loading}
                        <span class="loading loading-spinner loading-xs"></span>
                        {t(
                          "frontend/src/routes/assignments/[id]/+page.svelte::explain_failure_loading",
                        )}
                      {:else}
                        <Sparkles size={12} />
                        {t(
                          "frontend/src/routes/submissions/[id]/+page.svelte::explain_all_failures_btn",
                        )}
                      {/if}
                    </button>
                    {#if summaryExplanation.error}
                      <div class="text-[10px] text-error mt-1">
                        {t(
                          "frontend/src/routes/assignments/[id]/+page.svelte::explain_failure_error",
                        )}
                      </div>
                    {/if}
                  {/if}
                </div>
              {/if}
              <div class="overflow-x-auto -mx-2">
                <table class="table w-full">
                  <thead>
                    <tr>
                      <th class="pl-6">{t("frontend/src/routes/assignments/[id]/+page.svelte::results_table_header_num")}</th>
                      <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::results_table_header_status")}</th>
                      <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::results_table_header_runtime")}</th>
                      <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::results_table_header_exit")}</th>
                      <th class="pr-6">{t("frontend/src/routes/assignments/[id]/+page.svelte::results_table_header_traceback")}</th>
                    </tr>
                  </thead>
                  <tbody class="text-sm font-medium">
                    {#each results as r, i}
                      <tr class="hover:bg-base-200/50 transition-colors border-b border-base-200/50 last:border-none">
                        <td class="pl-6 font-black tabular-nums">#{i + 1}</td>
                        <td>
                          <div class={`badge border-none font-black text-[10px] uppercase tracking-wider py-3 ${statusColor(r.status).replace('badge-', 'bg-')}/20 ${statusColor(r.status).replace('badge-', 'text-')}`}>
                            {r.status}
                          </div>
                          {#if assignment.llm_help_why_failed && !allTestsFailed && r.status !== 'passed' && r.status !== 'running' && r.test_case_id}
                             <div class="mt-2">
                                {#if explanations[r.test_case_id]?.text}
                                   <div class="p-2 bg-base-200 rounded-lg text-xs border border-base-300 shadow-sm max-w-xs text-left">
                                      <div class="flex gap-2 items-start">
                                         <Sparkles size={14} class="text-primary mt-0.5 shrink-0" />
                                         <span class="leading-relaxed">{explanations[r.test_case_id].text}</span>
                                      </div>
                                   </div>
                                {:else}
                                   <button class="btn btn-xs btn-ghost gap-1 text-[10px] font-bold text-primary opacity-60 hover:opacity-100" on:click={() => askWhyFailed(latestSub.id, r.test_case_id)} disabled={explanations[r.test_case_id]?.loading}>
                                       {#if explanations[r.test_case_id]?.loading}
                                           <span class="loading loading-spinner loading-xs"></span>
                                           {t("frontend/src/routes/assignments/[id]/+page.svelte::explain_failure_loading")}
                                       {:else}
                                           <Sparkles size={12}/>
                                           {t("frontend/src/routes/assignments/[id]/+page.svelte::explain_failure_btn")}
                                       {/if}
                                   </button>
                                   {#if explanations[r.test_case_id]?.error}
                                       <div class="text-[10px] text-error mt-1">{t("frontend/src/routes/assignments/[id]/+page.svelte::explain_failure_error")}</div>
                                   {/if}
                                {/if}
                             </div>
                          {/if}
                        </td>
                        <td class="tabular-nums font-bold opacity-60">{r.runtime_ms}ms</td>
                        <td>
                           <span class={`badge font-mono text-[10px] font-black ${r.exit_code === 0 ? 'badge-ghost opacity-40' : 'badge-error'}`}>{r.exit_code}</span>
                        </td>
                        <td class="pr-6">
                           {#if r.stderr}
                             <div class="group relative">
                                <pre class="max-w-md max-h-32 text-[11px] overflow-hidden group-hover:overflow-auto transition-all bg-[var(--assignment-code-bg)] p-3 rounded-xl border border-[var(--assignment-code-border)] font-mono leading-relaxed">{r.stderr}</pre>
                                <div class="absolute bottom-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity">
                                   <button class="btn btn-circle btn-xs btn-ghost" on:click={() => {navigator.clipboard.writeText(r.stderr); alert(t("frontend/src/routes/assignments/[id]/tests/+page.svelte::copied"))}}>
                                      <Search size={12} />
                                   </button>
                                </div>
                             </div>
                           {:else}
                             <span class="opacity-20 font-black tracking-widest text-[10px]">EMPTY</span>
                           {/if}
                        </td>
                      </tr>
                    {/each}
                    {#if !results.length}
                      <tr>
                        <td colspan="5" class="py-20 text-center">
                           <div class="flex flex-col items-center gap-4 opacity-20">
                              <FlaskConical size={48} strokeWidth={1} />
                              <p class="font-black uppercase tracking-[0.2em] text-xs">{t("frontend/src/routes/assignments/[id]/+page.svelte::no_results_yet_table")}</p>
                           </div>
                        </td>
                      </tr>
                    {/if}
                  </tbody>
                </table>
              </div>
            {:else}
              <div class="bg-base-200/30 rounded-[2rem] p-20 flex flex-col items-center justify-center gap-6 border border-dashed border-base-300">
                <div class="p-6 bg-base-100 rounded-full shadow-lg opacity-20 transform -rotate-12">
                   <Send size={48} strokeWidth={1} />
                </div>
                <div class="text-center space-y-2">
                   <h4 class="font-black text-xl opacity-40">{t("frontend/src/routes/assignments/[id]/+page.svelte::no_submission_yet_alert")}</h4>
                   <button class="btn btn-primary btn-sm rounded-full px-8 font-black uppercase tracking-widest text-[10px]" on:click={openSubmitModal}>
                      {t("frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_button")}
                   </button>
                </div>
              </div>
            {/if}
          </section>
        {/if}

        {#if activeTab === "instructor" && (role === "teacher" || role === "admin")}
          <section class="space-y-6">
            <div class="card-elevated p-8 sm:p-10 bg-base-100 rounded-[2rem] border border-base-200">
               <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-6 mb-10">
                  <div class="flex items-center gap-4">
                     <div class="p-4 bg-secondary/10 rounded-[1.25rem] text-secondary shadow-inner">
                        <Users size={24} />
                     </div>
                     <div>
                        <h3 class="text-2xl font-black tracking-tight">{t("frontend/src/routes/assignments/[id]/+page.svelte::student_progress_heading")}</h3>
                        <p class="text-[10px] font-black opacity-40 uppercase tracking-[0.2em]">{progress.length} {t("frontend/src/routes/assignments/[id]/+page.svelte::students_enrolled_label")}</p>
                     </div>
                  </div>
                  <div class="flex items-center gap-3">
                     <div class="join shadow-sm border border-base-300 rounded-2xl overflow-hidden bg-base-100">
                        <div class="join-item flex items-center px-4 bg-base-200/50 border-r border-base-300">
                           <Search size={14} class="opacity-40" />
                        </div>
                        <input type="text" placeholder={t("frontend/src/routes/assignments/[id]/+page.svelte::search_students_placeholder")} class="join-item input input-ghost input-sm focus:bg-base-100 transition-all font-medium py-5 px-4 w-48 sm:w-64" />
                     </div>
                  </div>
               </div>

               <div class="overflow-x-auto -mx-2">
                 <table class="table w-full">
                   <thead>
                     <tr>
                       <th class="pl-6">{t("frontend/src/routes/assignments/[id]/+page.svelte::progress_table_header_student")}</th>
                       <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::progress_table_header_status")}</th>
                       <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::progress_table_header_deadline")}</th>
                       <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::progress_table_header_latest")}</th>
                       <th class="pr-6">{t("frontend/src/routes/assignments/[id]/+page.svelte::progress_table_header_extension")}</th>
                     </tr>
                   </thead>
                   <tbody class="text-sm font-medium">
                     {#each progress as p (p.student.id)}
                       <tr
                         class="hover:bg-base-200/50 transition-colors cursor-pointer group border-b border-base-200/50 last:border-none"
                         on:click={() => toggleStudent(p.student.id)}
                       >
                         <td class="pl-6">
                            <div class="flex items-center gap-3">
                               <div class="avatar placeholder">
                                  <div class="bg-base-300 text-base-content/40 rounded-xl w-10 font-black text-xs uppercase overflow-hidden">
                                     {#if p.student.avatar}
                                       <img src={p.student.avatar} alt={p.student.name ?? p.student.email ?? ""} class="w-full h-full object-cover" loading="lazy" />
                                     {:else}
                                       {p.student.name?.substring(0, 2) || p.student.email?.substring(0, 2)}
                                     {/if}
                                  </div>
                               </div>
                               <div class="flex flex-col">
                                  <span class="font-black text-base">{p.student.name ?? p.student.email}</span>
                                  {#if p.student.name}
                                    <span class="text-[10px] opacity-40 font-bold uppercase tracking-wider">{p.student.email}</span>
                                  {/if}
                               </div>
                            </div>
                         </td>
                         <td>
                           <div class={`badge border-none font-black text-[10px] uppercase tracking-wider py-3 ${statusColor(p.displayStatus).replace('badge-', 'bg-')}/20 ${statusColor(p.displayStatus).replace('badge-', 'text-')}`}>
                             {statusLabel(p.displayStatus)}
                           </div>
                         </td>
                         <td>
                           {#if p.latest}
                             {#if p.latest.created_at > assignment.deadline}
                               {#if assignment.second_deadline && p.latest.created_at <= assignment.second_deadline}
                                 <div class="badge bg-warning/10 text-warning border-none font-black text-[10px] uppercase tracking-wider py-3">
                                   {t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_badge_second_deadline", { penalty: Math.round(assignment.late_penalty_ratio * 100) })}
                                 </div>
                               {:else}
                                 <div class="badge bg-error/10 text-error border-none font-black text-[10px] uppercase tracking-wider py-3">
                                   {t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_badge_late")}
                                 </div>
                               {/if}
                             {:else}
                               <div class="badge bg-success/10 text-success border-none font-black text-[10px] uppercase tracking-wider py-3">
                                 {t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_badge_on_time")}
                               </div>
                             {/if}
                           {:else}
                             <div class="badge bg-base-200 text-base-content/40 border-none font-black text-[10px] uppercase tracking-wider py-3">
                               {t("frontend/src/routes/assignments/[id]/+page.svelte::progress_table_badge_no_submission")}
                             </div>
                           {/if}
                         </td>
                         <td>
                            {#if p.latest}
                               <div class="flex flex-col">
                                  <span class="font-bold">{formatDateTime(p.latest.created_at)}</span>
                                  <span class="text-[10px] opacity-40 font-bold uppercase tracking-wider">Attempt #{p.latest.attempt_number ?? "?"}</span>
                               </div>
                            {:else}
                               <span class="opacity-20 font-black">—</span>
                            {/if}
                         </td>
                         <td class="pr-6">
                           {#if overrideMap[p.student.id]}
                             <div class="flex items-center gap-2">
                               <div class="badge bg-info/10 text-info border-none font-black text-[10px] uppercase tracking-wider py-3" title={overrideMap[p.student.id].note || ""}>
                                 {t("frontend/src/routes/assignments/[id]/+page.svelte::progress_table_extension_until")} {formatDateTime(overrideMap[p.student.id].new_deadline)}
                               </div>
                               <button
                                 class="btn btn-ghost btn-xs opacity-0 group-hover:opacity-100 transition-opacity"
                                 on:click|stopPropagation={() => openExtendDialog(p.student)}
                               >
                                 <Edit3 size={12} />
                               </button>
                             </div>
                           {:else}
                             <button
                               class="btn btn-ghost btn-xs opacity-0 group-hover:opacity-100 transition-opacity font-black uppercase tracking-widest text-[9px] text-primary"
                               on:click|stopPropagation={() => openExtendDialog(p.student)}
                             >
                               {t("frontend/src/routes/assignments/[id]/+page.svelte::progress_table_extension_extend")}
                             </button>
                           {/if}
                         </td>
                       </tr>

                       {#if expanded === p.student.id}
                         <tr class="bg-base-200/20 border-b border-base-200/50">
                           <td colspan="5" class="p-8">
                             {#if p.all && p.all.length}
                               <div class="space-y-4">
                                  <h4 class="text-[10px] font-black uppercase tracking-[0.2em] opacity-40 mb-6">{t("frontend/src/routes/assignments/[id]/+page.svelte::submission_history_label")}</h4>
                                  <div class="grid gap-3">
                                    {#each p.all as s}
                                      <div class="flex items-center gap-6 p-4 bg-base-100 rounded-2xl border border-base-200/60 shadow-sm hover:border-primary/20 transition-all">
                                         <div class="w-10 h-10 rounded-xl flex items-center justify-center font-black tabular-nums bg-base-200 text-base-content/40">
                                            #{s.attempt_number ?? "?"}
                                         </div>
                                         <div class="flex-1 flex items-center gap-8">
                                            <div class="flex flex-col">
                                               <span class="font-bold text-sm">{formatDateTime(s.created_at)}</span>
                                               <span class="text-[10px] opacity-40 font-bold uppercase tracking-wider">{relativeToDeadline(s.created_at)}</span>
                                            </div>
                                            <div class={`badge border-none font-black text-[9px] uppercase tracking-wider ${statusColor(s.status).replace('badge-', 'bg-')}/15 ${statusColor(s.status).replace('badge-', 'text-')}`}>
                                               {statusLabel(s.status)}
                                            </div>
                                            {#if testsCount > 0}
                                              <div class="flex items-center gap-3">
                                                 <div class="w-12 bg-base-200 rounded-full h-1.5 overflow-hidden">
                                                    <div class="bg-primary h-full rounded-full" style="width: {(s.tests_passed / (s.tests_total || testsCount)) * 100}%"></div>
                                                 </div>
                                                 <span class="font-black text-xs tabular-nums text-base-content/60">{s.tests_passed}/{s.tests_total || testsCount}</span>
                                              </div>
                                            {/if}
                                         </div>
                                         <a
                                           class="btn btn-ghost btn-sm rounded-xl font-black text-[9px] uppercase tracking-widest text-primary"
                                           href={`/submissions/${s.id}?fromTab=${activeTab}`}
                                           on:click={saveState}
                                         >
                                            {t("frontend/src/routes/assignments/[id]/+page.svelte::submission_table_view_button")}
                                            <ArrowRight size={14} class="ml-1" />
                                         </a>
                                      </div>
                                    {/each}
                                  </div>
                               </div>
                             {:else}
                               <div class="py-10 text-center opacity-20">
                                  <History size={32} class="mx-auto mb-3" />
                                  <p class="font-black uppercase tracking-widest text-[10px]">{t("frontend/src/routes/assignments/[id]/+page.svelte::no_submissions_yet_table")}</p>
                               </div>
                             {/if}
                           </td>
                         </tr>
                       {/if}
                     {/each}
                     {#if !progress.length}
                        <tr>
                           <td colspan="5" class="py-20 text-center">
                              <div class="flex flex-col items-center gap-4 opacity-20">
                                 <Users size={48} strokeWidth={1} />
                                 <p class="font-black uppercase tracking-[0.2em] text-xs">{t("frontend/src/routes/assignments/[id]/+page.svelte::no_students_table")}</p>
                              </div>
                           </td>
                        </tr>
                     {/if}
                   </tbody>
                 </table>
               </div>
            </div>
          </section>
        {/if}

        {#if activeTab === "teacher-runs" && (role === "teacher" || role === "admin") && !isScratchAssignment}
          <section class="card-elevated p-8 sm:p-10 bg-base-100 rounded-[2rem] border border-base-200">
            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-6 mb-10">
              <div class="flex items-center gap-4">
                 <div class="p-4 bg-secondary/10 rounded-[1.25rem] text-secondary shadow-inner">
                    <FlaskConical size={24} />
                 </div>
                 <div>
                    <h3 class="text-2xl font-black tracking-tight">{t("frontend/src/routes/assignments/[id]/+page.svelte::your_runs_heading")}</h3>
                    <p class="text-[10px] font-black opacity-40 uppercase tracking-[0.2em]">{teacherRuns.length} {t("frontend/src/routes/assignments/[id]/+page.svelte::total_runs_label")}</p>
                 </div>
              </div>
              <button
                class="btn btn-primary rounded-2xl px-8 h-12 font-black uppercase tracking-widest text-[11px] shadow-lg shadow-primary/20"
                on:click={openTeacherRunModal}
              >
                <FlaskConical size={16} />
                {t("frontend/src/routes/assignments/[id]/+page.svelte::new_run_button")}
              </button>
            </div>

            <div class="overflow-x-auto -mx-2">
              <table class="table w-full">
                <thead>
                  <tr>
                    <th class="pl-6">{t("frontend/src/routes/assignments/[id]/+page.svelte::teacher_runs_table_header_date")}</th>
                    <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::teacher_runs_table_header_status")}</th>
                    <th>{t("frontend/src/routes/assignments/[id]/+page.svelte::teacher_runs_table_header_first_failure")}</th>
                    <th class="pr-6"></th>
                  </tr>
                </thead>
                <tbody class="text-sm font-medium">
                  {#each teacherRuns as s}
                    <tr class="hover:bg-base-200/50 transition-colors group border-b border-base-200/50 last:border-none">
                      <td class="pl-6">
                        <div class="flex flex-col">
                           <span class="font-bold">{formatDateTime(s.created_at)}</span>
                           <span class="text-[10px] opacity-40 font-bold uppercase tracking-wider">{relativeToDeadline(s.created_at)}</span>
                        </div>
                      </td>
                      <td>
                        <div class={`badge border-none font-black text-[10px] uppercase tracking-wider py-3 ${statusColor(s.status).replace('badge-', 'bg-')}/20 ${statusColor(s.status).replace('badge-', 'text-')}`}>
                          {statusLabel(s.status)}
                        </div>
                      </td>
                      <td>
                        {#if s.failure_reason}
                           <div class="flex items-center gap-2 text-error">
                              <AlertTriangle size={14} />
                              <span class="font-bold text-xs truncate max-w-xs">{s.failure_reason}</span>
                           </div>
                        {:else}
                           <span class="opacity-20 font-black tracking-widest text-[10px]">NONE</span>
                        {/if}
                      </td>
                      <td class="pr-6 text-right">
                         <a
                           class="btn btn-ghost btn-circle btn-sm opacity-0 group-hover:opacity-100 transition-all hover:bg-primary hover:text-primary-content"
                           href={`/submissions/${s.id}?fromTab=${activeTab}`}
                           on:click={saveState}
                         >
                           <ArrowRight size={18} />
                         </a>
                      </td>
                    </tr>
                  {/each}
                  {#if !teacherRuns.length}
                    <tr>
                      <td colspan="4" class="py-24 text-center">
                         <div class="flex flex-col items-center gap-4 opacity-20">
                            <FlaskConical size={64} strokeWidth={1} />
                            <p class="font-black uppercase tracking-[0.3em] text-xs">{t("frontend/src/routes/assignments/[id]/+page.svelte::no_runs_yet_table")}</p>
                         </div>
                      </td>
                    </tr>
                  {/if}
                </tbody>
              </table>
            </div>
          </section>
        {/if}
      </div>

      {#if role === "student"}
        <aside class="lg:col-span-4 lg:sticky lg:top-24 h-fit space-y-4">
          <div class="card-elevated p-5 space-y-3">
            <h3 class="font-semibold">
              {t(
                "frontend/src/routes/assignments/[id]/+page.svelte::quick_actions_heading",
              )}
            </h3>
            <button
              class="btn btn-primary w-full shadow-lg shadow-primary/20 hover:shadow-primary/30 group transition-all duration-300 h-14 rounded-2xl gap-3 border-none"
              on:click={openSubmitModal}
              disabled={assignment.second_deadline &&
                new Date() > assignment.deadline &&
                new Date() > assignment.second_deadline}
            >
              <div class="p-2 bg-white/20 rounded-xl group-hover:rotate-12 transition-transform">
                <Send size={18} />
              </div>
              <span class="font-black uppercase tracking-widest text-[11px]">
                {t("frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_button")}
              </span>
            </button>
            {#if assignment.template_path}
              <div class="divider my-1"></div>
              <div class="text-sm opacity-70">
                {t(
                  "frontend/src/routes/assignments/[id]/+page.svelte::need_starting_point_text",
                )}
              </div>
              <button
                class="btn btn-ghost btn-sm"
                on:click|preventDefault={downloadTemplate}
                >{t(
                  "frontend/src/routes/assignments/[id]/+page.svelte::download_template_button",
                )}</button
              >
            {/if}
          </div>
          {#if latestSub}
            <div class="card-elevated p-5 space-y-2">
              <h3 class="font-semibold">
                {t(
                  "frontend/src/routes/assignments/[id]/+page.svelte::latest_submission_heading",
                )}
              </h3>
              <div class="flex items-center gap-2">
                <span class={`badge ${statusColor(latestSub.status)}`}
                  >{statusLabel(latestSub.status)}</span
                >
                <span class="text-xs opacity-70"
                  >{t(
                    "frontend/src/routes/assignments/[id]/+page.svelte::attempt_num_label",
                    { num: latestSub.attempt_number ?? "?" },
                  )}</span
                >
                <a
                  class="link"
                  href={`/submissions/${latestSub.id}?fromTab=${activeTab}`}
                  on:click={saveState}>{formatDateTime(latestSub.created_at)}</a
                >
              </div>
              {#if assignment.second_deadline && latestSub.created_at > assignment.deadline && latestSub.created_at <= assignment.second_deadline}
                <div class="alert alert-warning alert-sm">
                  <span
                    >{t(
                      "frontend/src/routes/assignments/[id]/+page.svelte::latest_submission_alert_body",
                      {
                        penalty: Math.round(
                          assignment.late_penalty_ratio * 100,
                        ),
                      },
                    )}</span
                  >
                </div>
              {/if}
            </div>
          {/if}
          {#if assignment.second_deadline && new Date() > assignment.deadline && new Date() <= assignment.second_deadline}
            <div class="card-elevated p-5 space-y-2">
              <h3 class="font-semibold text-warning">
                {t(
                  "frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_active_sidebar_heading",
                )}
              </h3>
              <p class="text-sm">
                {t(
                  "frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_active_sidebar_body",
                  { penalty: Math.round(assignment.late_penalty_ratio * 100) },
                )}
              </p>
              <div class="text-xs opacity-70">
                {t(
                  "frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_label_with_date",
                )}: {formatDateTime(assignment.second_deadline)}
              </div>
            </div>
          {:else if assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}
            <div class="card-elevated p-5 space-y-2">
              <h3 class="font-semibold text-error">
                {t(
                  "frontend/src/routes/assignments/[id]/+page.svelte::all_deadlines_passed_sidebar_heading",
                )}
              </h3>
              <p class="text-sm">
                {t(
                  "frontend/src/routes/assignments/[id]/+page.svelte::all_deadlines_passed_sidebar_body",
                )}
              </p>
            </div>
          {/if}
        </aside>
      {/if}
    </div>
  {/if}

  <!-- tests list moved to modal -->

  <dialog bind:this={submitDialog} class="modal modal-bottom sm:modal-middle">
    <div class="modal-box sm:w-11/12 sm:max-w-lg p-0 overflow-hidden bg-base-100 rounded-[2rem] shadow-2xl border border-base-300">
      <!-- Modal Header -->
      <div class="p-6 pb-4 flex items-center justify-between border-b border-base-200 bg-base-200/30">
        <div class="flex items-center gap-3">
          <div class="p-2.5 bg-primary/10 rounded-xl text-primary">
            <FileUp size={20} />
          </div>
          <h3 class="font-black text-xl tracking-tight">
            {t("frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_heading")}
          </h3>
        </div>
        <form method="dialog">
          <button class="btn btn-circle btn-ghost btn-sm opacity-40 hover:opacity-100">
            <X size={20} />
          </button>
        </form>
      </div>

      <div class="p-6 space-y-6">
        {#if assignment.second_deadline && new Date() > assignment.deadline && new Date() <= assignment.second_deadline}
          <div class="alert bg-warning/10 border-warning/20 text-warning-content rounded-2xl flex items-start gap-3 py-3">
            <AlertTriangle size={18} class="mt-0.5 shrink-0" />
            <div class="text-xs">
              <strong class="font-black uppercase tracking-widest text-[9px] block mb-0.5">
                {t("frontend/src/routes/assignments/[id]/+page.svelte::second_deadline_period_info_strong")}
              </strong>
              {t("frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_alert_body", { penalty: Math.round(assignment.late_penalty_ratio * 100) })}
            </div>
          </div>
        {:else if assignment.second_deadline && new Date() > assignment.deadline && new Date() > assignment.second_deadline}
          <div class="alert bg-error/10 border-error/20 text-error-content rounded-2xl flex items-start gap-3 py-3">
            <AlertCircle size={18} class="mt-0.5 shrink-0" />
            <div class="text-xs">
              <strong class="font-black uppercase tracking-widest text-[9px] block mb-0.5">
                {t("frontend/src/routes/assignments/[id]/+page.svelte::all_deadlines_passed_alert_strong")}
              </strong>
              {t("frontend/src/routes/assignments/[id]/+page.svelte::all_deadlines_passed_alert_body")}
            </div>
          </div>
        {/if}

        <div
          role="button"
          tabindex="0"
          aria-label="Upload dropzone"
          class={`relative group/drop border-2 border-dashed rounded-3xl p-10 text-center transition-all duration-300 cursor-pointer ${isDragging ? "border-primary bg-primary/5 scale-[0.99]" : "border-base-300 bg-base-200/30 hover:bg-base-200/50 hover:border-base-400"}`}
          on:dragover|preventDefault={() => (isDragging = true)}
          on:dragleave={() => (isDragging = false)}
          on:click={() => fileInput?.click()}
          on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); fileInput?.click(); } }}
          on:drop|preventDefault={(e) => {
            isDragging = false;
            const dt = (e as DragEvent).dataTransfer;
            if (dt) {
              addSubmissionFiles(Array.from(dt.files));
            }
          }}
        >
          <div class="flex flex-col items-center gap-4">
            <div class={`p-5 rounded-2xl transition-all duration-300 ${isDragging ? "bg-primary text-primary-content scale-110 shadow-xl shadow-primary/20" : "bg-base-100 text-base-content/30 group-hover/drop:text-primary group-hover/drop:scale-110 shadow-sm"}`}>
              <Upload size={32} strokeWidth={1.5} />
            </div>
            <div>
              <div class="font-black text-sm mb-1">
                {t("frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_dropzone_text", { ext: submissionExtLabel })}
              </div>
              <div class="text-[10px] font-bold opacity-40 uppercase tracking-widest">
                {t("frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_or")}
              </div>
            </div>
            <button 
              class="btn btn-sm bg-base-100 hover:bg-base-300 border-base-300 rounded-xl px-6 font-black uppercase tracking-widest text-[9px]"
              tabindex="-1"
            >
              {t("frontend/src/routes/+layout.2svelte::select_image_button").replace("image", "files")}
            </button>
          </div>
          
          <input
            bind:this={fileInput}
            type="file"
            accept={submissionExtension}
            multiple={!isScratchAssignment}
            class="hidden"
            on:change={(e) =>
              addSubmissionFiles(Array.from((e.target as HTMLInputElement).files || []))}
          />
        </div>

        <div class="text-[10px] font-bold uppercase tracking-widest opacity-40 text-center">
          {t(
            "frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_max_size",
            { max: getSubmissionLimitMB() },
          )}
        </div>

        {#if files.length > 0}
          <div class="space-y-2 animate-in fade-in slide-in-from-top-2">
            <div class="flex items-center justify-between px-1">
              <span class="text-[10px] font-black uppercase tracking-widest opacity-40">
                {translate(
                  files.length === 1
                    ? "frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_files_selected_singular"
                    : "frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_files_selected_plural",
                  { count: files.length },
                )}
              </span>
              <button 
                class="text-[10px] font-black uppercase tracking-widest text-error hover:underline"
                on:click={() => { files = []; if(fileInput) fileInput.value = ""; }}
              >
                {t("frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_clear_button")}
              </button>
            </div>
            <div class="max-h-32 overflow-y-auto space-y-1.5 pr-1 custom-scrollbar">
              {#each files as f, i}
                <div class="flex items-center justify-between p-2.5 bg-base-200/50 rounded-xl border border-base-300/50">
                  <div class="flex items-center gap-2.5 truncate">
                    <div class="p-1.5 bg-base-100 rounded-lg text-primary/70">
                      <FileCode size={14} />
                    </div>
                    <span class="text-xs font-mono font-medium truncate">{f.name}</span>
                  </div>
                  <button 
                    class="btn btn-ghost btn-circle btn-xs text-error/40 hover:text-error hover:bg-error/10"
                    on:click={() => files = files.filter((_, idx) => idx !== i)}
                  >
                    <Trash2 size={12} />
                  </button>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        {#if isUploading}
          <div class="space-y-3 animate-in fade-in zoom-in-95">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <div class="loading loading-spinner loading-xs text-primary"></div>
                <span class="text-xs font-black uppercase tracking-widest text-primary">
                  {t("frontend/src/routes/assignments/[id]/+page.svelte::uploading_status")}
                </span>
              </div>
              <span class="font-mono text-xs font-black text-primary">{uploadProgress}%</span>
            </div>
            <div class="w-full bg-base-200 rounded-full h-2.5 overflow-hidden border border-base-300 shadow-inner">
              <div 
                class="bg-primary h-full transition-all duration-300 relative overflow-hidden" 
                style="width: {uploadProgress}%"
              >
                <div class="absolute inset-0 bg-white/20 animate-pulse"></div>
              </div>
            </div>
          </div>
        {/if}
      </div>

      <div class="p-6 pt-0 flex gap-3">
        <form method="dialog" class="flex-1">
          <button class="btn btn-ghost w-full rounded-2xl font-black uppercase tracking-widest text-[10px] h-12">
            {t("frontend/src/routes/assignments/[id]/+page.svelte::cancel_button")}
          </button>
        </form>
        <button
          class="btn btn-primary flex-[2] rounded-2xl h-12 shadow-lg shadow-primary/20 gap-2 font-black uppercase tracking-widest text-[11px]"
          on:click={submit}
          disabled={!files.length || isUploading ||
            (assignment.second_deadline &&
              new Date() > assignment.deadline &&
              new Date() > assignment.second_deadline)}
        >
          {#if isUploading}
            <div class="loading loading-spinner loading-xs"></div>
          {:else}
            <div class="p-1.5 bg-white/20 rounded-lg">
              <Check size={14} />
            </div>
          {/if}
          {t("frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_upload_button")}
        </button>
      </div>
    </div>
  </dialog>

  <!-- Extend deadline dialog (teacher) -->
  <dialog bind:this={extendDialog} class="modal">
    <div class="modal-box w-11/12 max-w-md space-y-4">
      <h3 class="font-bold text-lg">
        {t(
          "frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_heading",
        )}
      </h3>
      <div class="form-control">
        <div class="label"
          ><span class="label-text"
            >{t(
              "frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_student_label",
            )}</span
          ></div
        >
        <div class="input input-bordered">
          {extStudent?.name ?? extStudent?.email}
        </div>
      </div>
      <div class="form-control">
        <div class="label"
          ><span class="label-text"
            >{t(
              "frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_new_deadline_label",
            )}</span
          ></div
        >
        <div class="flex items-center gap-2">
          <input
            class="input input-bordered w-full"
            readonly
            placeholder="dd/mm/yyyy hh:mm"
            value={euLabelFromParts(extDeadlineDate, extDeadlineTime)}
          />
          <button class="btn" on:click|preventDefault={pickExtensionDeadline}
            >{t(
              "frontend/src/routes/assignments/[id]/+page.svelte::pick_button",
            )}</button
          >
          {#if extDeadlineDate}
            <button
              class="btn btn-ghost"
              on:click|preventDefault={() => {
                extDeadlineDate = "";
                extDeadlineTime = "";
              }}
              >{t(
                "frontend/src/routes/assignments/[id]/+page.svelte::clear_button_label",
              )}</button
            >
          {/if}
        </div>
      </div>
      <div class="form-control">
        <div class="label"
          ><span class="label-text"
            >{t(
              "frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_note_label",
            )}</span
          ></div
        >
        <input
          type="text"
          class="input input-bordered w-full"
          placeholder={t(
            "frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_note_placeholder",
          )}
          bind:value={extNote}
        />
      </div>
      <div class="modal-action">
        {#if overrideMap[extStudent?.id]}
          <button class="btn btn-error btn-outline" on:click={clearExtension}
            >{t(
              "frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_clear_button",
            )}</button
          >
        {/if}
        <button
          class="btn"
          on:click={saveExtension}
          disabled={!extStudent || !extDeadline}
          >{t(
            "frontend/src/routes/assignments/[id]/+page.svelte::extend_deadline_modal_save_button",
          )}</button
        >
      </div>
    </div>
    <form method="dialog" class="modal-backdrop">
      <button
        >{t(
          "frontend/src/routes/assignments/[id]/+page.svelte::modal_close_button",
        )}</button
      >
    </form>
  </dialog>

  <!-- Teacher run upload modal -->
  <dialog bind:this={teacherRunDialog} class="modal">
    <div class="modal-box w-11/12 max-w-lg space-y-4">
      <h3 class="font-bold text-lg">
        {t(
          "frontend/src/routes/assignments/[id]/+page.svelte::new_teacher_run_modal_heading",
        )}
      </h3>
      <div
        role="button"
        tabindex="0"
        aria-label="Teacher solution dropzone"
        class={`border-2 border-dashed rounded-xl p-6 text-center transition cursor-pointer ${isSolDragging ? "bg-base-200" : "bg-base-100 hover:bg-base-200/50"}`}
        on:dragover|preventDefault={() => (isSolDragging = true)}
        on:dragleave={() => (isSolDragging = false)}
        on:click={() => teacherFileInput?.click()}
        on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); teacherFileInput?.click(); } }}
        on:drop|preventDefault={(e) => {
          isSolDragging = false;
          const dt = (e as DragEvent).dataTransfer;
          if (dt) {
            solFiles = [...solFiles, ...Array.from(dt.files)].filter((f) =>
              f.name.endsWith(".py"),
            );
          }
        }}
      >
        <div class="text-sm opacity-70 mb-2">
          {t(
            "frontend/src/routes/assignments/[id]/+page.svelte::new_teacher_run_modal_dropzone_text",
          )}
        </div>
        <div class="mb-3">
          {t(
            "frontend/src/routes/assignments/[id]/+page.svelte::submit_solution_modal_or",
          )}
        </div>
        <div class="btn btn-sm btn-ghost border-base-300">
          {t("frontend/src/routes/+layout.2svelte::select_image_button").replace("image", "files")}
        </div>
        <input
          bind:this={teacherFileInput}
          type="file"
          accept=".py"
          multiple
          class="hidden"
          on:change={(e) =>
            (solFiles = Array.from((e.target as HTMLInputElement).files || []))}
        />
      </div>
      {#if solFiles.length}
        <div class="text-sm opacity-70">
          {translate(
            solFiles.length === 1
              ? "frontend/src/routes/assignments/[id]/+page.svelte::new_teacher_run_modal_files_selected_singular"
              : "frontend/src/routes/assignments/[id]/+page.page.svelte::new_teacher_run_modal_files_selected_plural",
            { count: solFiles.length },
          )}
        </div>
      {/if}
      <div class="modal-action">
        <button
          class={`btn btn-primary ${solLoading ? "loading" : ""}`}
          on:click={runTeacherSolution}
          disabled={!solFiles.length || solLoading}
          >{t(
            "frontend/src/routes/assignments/[id]/+page.svelte::new_teacher_run_modal_run_button",
          )}</button
        >
      </div>
    </div>
    <form method="dialog" class="modal-backdrop">
      <button
        >{t(
          "frontend/src/routes/assignments/[id]/+page.svelte::modal_close_button",
        )}</button
      >
    </form>
  </dialog>

  {#if err}
    <div class="alert alert-error mt-4"><span>{err}</span></div>
  {/if}
  <ConfirmModal bind:this={confirmModal} />
  <DeadlinePicker bind:this={deadlinePicker} />
{/if}

<style>
  .card-elevated {
    background-color: var(--fallback-b1,oklch(var(--b1)/1));
    border-radius: 2rem;
    border-width: 1px;
    border-color: rgba(var(--fallback-b2,oklch(var(--b2)/1)), 0.6);
    box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
    transition-property: all;
    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
    transition-duration: 300ms;
  }

  .card-elevated:hover {
    border-color: rgba(var(--fallback-p,oklch(var(--p)/1)), 0.2);
    box-shadow: 0 20px 25px -5px rgba(var(--fallback-p,oklch(var(--p)/1)), 0.05);
  }

  .markdown :global(p) {
    margin-bottom: 1rem;
  }

  .markdown :global(h1), .markdown :global(h2), .markdown :global(h3) {
    font-weight: 900;
    letter-spacing: -0.025em;
    margin-bottom: 1rem;
    margin-top: 1.5rem;
    color: var(--fallback-bc,oklch(var(--bc)/0.9));
  }

  .markdown :global(ul) {
    list-style-type: disc;
    list-style-position: inside;
    margin-bottom: 1rem;
  }
  
  .markdown :global(ul > li) {
    margin-top: 0.5rem;
  }

  .markdown :global(ol) {
    list-style-type: decimal;
    list-style-position: inside;
    margin-bottom: 1rem;
  }
  
  .markdown :global(ol > li) {
    margin-top: 0.5rem;
  }

  .table thead th {
    font-size: 0.625rem;
    font-weight: 900;
    text-transform: uppercase;
    letter-spacing: 0.15em;
    opacity: 0.4;
    padding-top: 1rem;
    padding-bottom: 1rem;
  }

  .table tbody td {
    padding-top: 1rem;
    padding-bottom: 1rem;
  }

  /* Smooth scroll for the entire page */
  :global(html) {
    scroll-behavior: smooth;
  }
</style>

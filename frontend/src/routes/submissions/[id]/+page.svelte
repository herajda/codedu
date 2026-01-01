<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { apiJSON, apiFetch } from "$lib/api";
  import { createEventSource } from "$lib/sse";
  import { page } from "$app/stores";
  import JSZip from "jszip";
  import { FileTree, RunConsole } from "$lib";
  import ScratchPlayer from "$lib/components/ScratchPlayer.svelte";
  import ScratchBlocksViewer from "$lib/components/ScratchBlocksViewer.svelte";
  import { formatDateTime } from "$lib/date";
  import { goto } from "$app/navigation";
  import { auth } from "$lib/auth";
  import {
    extractMethodFromUnittest,
    stripUnittestMainBlock,
  } from "$lib/unittests";
  import { t, translator } from "$lib/i18n";

  $: id = $page.params.id;

  let submission: any = null;
  let results: any[] = [];
  let err = "";
  let files: { name: string; content: string }[] = [];
  let tree: FileNode[] = [];
  let selected: { name: string; content: string } | null = null;
  let highlighted = "";
  import {
    Sparkles,
    Trophy,
    History,
    User,
    Calendar,
    ArrowLeft,
    ExternalLink,
    FileCode,
    CheckCircle2,
    AlertCircle,
    AlertTriangle,
    Clock,
    ArrowRight,
    GraduationCap,
    FlaskConical,
    Cpu,
    Search,
    Shield,
    Save,
    Gamepad2,
    Maximize2,
    Minimize2
  } from "lucide-svelte";
  let manualConsoleVisible = false;
  let esCtrl: { close: () => void } | null = null;
  let assignmentTitle: string = "";
  let assignmentManual: boolean = false;
  let assignmentTestsCount: number = 0;
  let assignmentLLMInteractive: boolean = false;
  let assignmentLLMFeedback: boolean = false;
  let assignmentShowTestDetails = false;
  let assignmentShowTraceback = false;
  let assignmentLLMHelpWhyFailed = false;
  let assignmentLanguage: string = "python";
  let assignmentLoaded = false;
  let scratchProject: Uint8Array | null = null;
  let scratchProjectName = "";
  let scratchProjectError = "";
  let scratchLoading = false;
  let lastCodeContent = "";
  let lastCodeBytes: Uint8Array | null = null;
  let filesLoading = false;
  let filesLoaded = false;
  let pendingZip: JSZip | null = null;
  let allTestsFailed = false;
  let sid: number = 0;
  let role = "";
  $: role = $auth?.role ?? "";
  let translate;
  $: translate = $translator;
  let scratchFullscreenMode: "none" | "player" | "both" = "none";
  let scratchFullscreenHost: HTMLDivElement | null = null;
  let removeScratchFullscreenListener: (() => void) | null = null;

  import hljs from "highlight.js";
  import "highlight.js/styles/github.css";
  let fileDialog: HTMLDialogElement;

  let llm: any = null;
  let activeTab: "results" | "files" | "review" | "scratch" = "results";
  // Derived visibility flags

  // Inline teacher points override component
  // This is a tiny Svelte component defined in-file using a function that returns markup via a slot approach
  // Svelte does not support runtime component definitions; instead use a block here:
  let overrideValue: string | number | null = "";
  let savingOverride = false;
  async function saveOverride() {
    try {
      savingOverride = true;
      const raw: any = overrideValue;
      const str =
        raw == null ? "" : typeof raw === "string" ? raw : String(raw);
      const v = str.trim() === "" ? null : parseInt(str, 10);
      await apiFetch(`/api/submissions/${submission.id}/points`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ points: v }),
      });
      await load();
    } catch (e: any) {
      err = e.message;
    } finally {
      savingOverride = false;
    }
  }

  interface FileNode {
    name: string;
    content?: string;
    children?: FileNode[];
  }

  function decodeBase64(b64: string): Uint8Array | null {
    try {
      const bin = atob(b64);
      const bytes = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
      return bytes;
    } catch {
      return null;
    }
  }

  async function parseFilesFromZip(zip: JSZip) {
    const list: { name: string; content: string }[] = [];
    for (const file of Object.values(zip.files)) {
      if (file.dir) continue;
      if (file.name.toLowerCase().endsWith(".sb3")) {
        list.push({
          name: file.name,
          content: t(
            "frontend/src/routes/submissions/[id]/+page.svelte::scratch_project_file_placeholder",
          ),
        });
        continue;
      }
      const content = await file.async("string");
      list.push({ name: file.name, content });
    }
    return list;
  }

  async function parseFiles(b64: string, bytes?: Uint8Array | null) {
    const payload = bytes !== undefined ? bytes : decodeBase64(b64);
    if (!payload) {
      return [{ name: "code", content: b64 }];
    }

    try {
      const zip = await JSZip.loadAsync(payload);
      return await parseFilesFromZip(zip);
    } catch {
      const text = new TextDecoder().decode(payload);
      return [{ name: "code", content: text }];
    }
  }

  async function loadFilesFromZip(zip: JSZip) {
    if (filesLoading || filesLoaded) return;
    filesLoading = true;
    try {
      files = await parseFilesFromZip(zip);
      tree = buildTree(files);
      selected = files[0] ?? null;
      filesLoaded = true;
      pendingZip = null;
    } finally {
      filesLoading = false;
    }
  }

  async function loadFilesFromBytes(b64: string, bytes?: Uint8Array | null) {
    if (filesLoading || filesLoaded) return;
    filesLoading = true;
    try {
      files = await parseFiles(b64, bytes);
      tree = buildTree(files);
      selected = files[0] ?? null;
      filesLoaded = true;
    } finally {
      filesLoading = false;
    }
  }

  async function extractScratchProject(
    b64: string,
    bytes?: Uint8Array | null,
  ): Promise<JSZip | null> {
    scratchProject = null;
    scratchProjectName = "";
    scratchProjectError = "";
    const payload = bytes !== undefined ? bytes : decodeBase64(b64);
    if (!payload) {
      scratchProjectError = t(
        "frontend/src/routes/submissions/[id]/+page.svelte::scratch_project_decode_error",
      );
      return null;
    }
    try {
      scratchLoading = true;
      const zip = await JSZip.loadAsync(payload);
      const candidates = Object.values(zip.files).filter(
        (file) =>
          !file.dir && file.name.toLowerCase().endsWith(".sb3"),
      );
      if (!candidates.length) {
        scratchProjectError = t(
          "frontend/src/routes/submissions/[id]/+page.svelte::scratch_project_missing",
        );
        return zip;
      }
      const file = candidates[0];
      scratchProject = await file.async("uint8array");
      scratchProjectName = file.name;
      return zip;
    } catch (e: any) {
      scratchProjectError =
        e?.message ||
        t(
          "frontend/src/routes/submissions/[id]/+page.svelte::scratch_project_load_error",
        );
      return null;
    } finally {
      scratchLoading = false;
    }
  }

  async function load() {
    err = "";
    assignmentLoaded = false;
    try {
      const data = await apiJSON(`/api/submissions/${id}`);
      submission = data.submission;
      results = data.results;
      llm = data.llm ?? null;

      // Prefill override input with the currently assigned points (teacher sees what's set)
      try {
        const cur = submission?.override_points ?? submission?.points;
        overrideValue = (cur ?? "") as any;
      } catch {}

      if (submission?.assignment_id) {
        try {
          const ad = await apiJSON(
            `/api/assignments/${submission.assignment_id}`,
          );
          assignmentTitle = ad.assignment?.title ?? "";
          assignmentManual = !!ad.assignment?.manual_review;
          assignmentLLMInteractive = !!ad.assignment?.llm_interactive;
          assignmentLLMFeedback = !!ad.assignment?.llm_feedback;
          assignmentShowTestDetails = !!ad.assignment?.show_test_details;
          assignmentShowTraceback = !!ad.assignment?.show_traceback;
          assignmentLLMHelpWhyFailed = !!ad.assignment?.llm_help_why_failed;
          assignmentLanguage = ad.assignment?.programming_language ?? "python";
          // Prefer aggregate tests_count when present (student view), fallback to tests array (teacher/admin)
          try {
            assignmentTestsCount =
              typeof ad.tests_count === "number"
                ? ad.tests_count
                : Array.isArray(ad.tests)
                  ? ad.tests.length
                  : 0;
          } catch {
            assignmentTestsCount = 0;
          }
        if (assignmentLanguage === "scratch") {
          if (activeTab === "results") activeTab = "scratch";
        } else {
          scratchProject = null;
          scratchProjectName = "";
          scratchProjectError = "";
        }
      } catch {}
      assignmentLoaded = true;
    } else {
      assignmentLoaded = true;
      }

      const codeContent = submission?.code_content ?? "";
      const codeChanged = codeContent !== lastCodeContent;
      if (codeChanged) {
        lastCodeContent = codeContent;
        lastCodeBytes = codeContent ? decodeBase64(codeContent) : null;
        files = [];
        tree = [];
        selected = null;
        highlighted = "";
        filesLoaded = false;
        filesLoading = false;
        pendingZip = null;
      }

      if (assignmentLanguage === "scratch") {
        if (codeChanged) {
          scratchProject = null;
          scratchProjectName = "";
          scratchProjectError = "";
        }
        const shouldLoadScratch =
          codeChanged || (!scratchProject && !scratchProjectError && !scratchLoading);
        if (shouldLoadScratch && codeContent) {
          pendingZip = await extractScratchProject(codeContent, lastCodeBytes);
        }
      } else if (codeChanged || (!filesLoaded && !filesLoading)) {
        await loadFilesFromBytes(codeContent, lastCodeBytes);
      }
    } catch (e: any) {
      err = e.message;
    } finally {
      if (!assignmentLoaded) assignmentLoaded = true;
    }
  }

  function buildTree(list: { name: string; content: string }[]): FileNode[] {
    const root: FileNode = { name: "", children: [] };
    for (const f of list) {
      const parts = f.name.split("/");
      let node = root;
      for (let i = 0; i < parts.length; i++) {
        const part = parts[i];
        if (!node.children) node.children = [];
        let child = node.children.find((c) => c.name === part);
        if (!child) {
          child = { name: part };
          node.children.push(child);
        }
        node = child;
        if (i === parts.length - 1) {
          node.content = f.content;
          node.children = undefined;
        }
      }
    }
    return root.children ?? [];
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

  function resultColor(s: string) {
    if (s === "passed") return "badge-success";
    if (s === "wrong_output") return "badge-error";
    if (s === "runtime_error") return "badge-error";
    if (s === "illegal_tool_use") return "badge-error";
    if (s === "time_limit_exceeded" || s === "memory_limit_exceeded")
      return "badge-warning";
    return "";
  }

  // Show LLM block when assignment uses LLM-interactive
  $: showLLM = assignmentLLMInteractive;
  // Allow detailed LLM artifacts for students only if teacher enabled feedback
  $: allowLLMDetails = role !== "student" || assignmentLLMFeedback;
  $: allowTestDetails = role !== "student" || assignmentShowTestDetails;
  $: allowTraceback = role !== "student" || assignmentShowTraceback;
  $: isScratchSubmission = assignmentLanguage === "scratch";
  // Show Auto-tests only when NOT LLM mode and there are tests configured
  $: showAutoUI =
    !isScratchSubmission &&
    !assignmentLLMInteractive &&
    assignmentTestsCount > 0;
  // Keep legacy meaning of hideAutoUI: specifically, when no auto tests exist
  $: hideAutoUI = assignmentTestsCount === 0 || isScratchSubmission;
  $: forceManualConsole =
    (assignmentManual || hideAutoUI) && !isScratchSubmission;
  $: if (forceManualConsole) manualConsoleVisible = true;
  $: if (isScratchSubmission) manualConsoleVisible = false;
  $: if (activeTab !== "scratch" && scratchFullscreenMode !== "none") {
    void exitScratchFullscreen();
  }
  $: if (typeof document !== "undefined") {
    if (scratchFullscreenMode !== "none") {
      document.body.classList.add("scratch-fullscreen-active");
    } else {
      document.body.classList.remove("scratch-fullscreen-active");
    }
  }

  function bgFromBadge(badgeClass: string) {
    return badgeClass.replace("badge", "bg");
  }

  $: totalTests = results?.length ?? 0;
  $: passedCount = results.filter((r) => r.status === "passed").length;
  $: failedCount = results.filter((r) =>
    ["wrong_output", "runtime_error", "failed", "illegal_tool_use"].includes(
      r.status,
    ),
  ).length;
  $: warnedCount = results.filter((r) =>
    ["time_limit_exceeded", "memory_limit_exceeded"].includes(r.status),
  ).length;

  // ----- LLM UI helpers -----
  function safeParseJSON(raw: any): any {
    try {
      if (!raw || typeof raw !== "string") return null;
      return JSON.parse(raw);
    } catch {
      return null;
    }
  }

  function viewableUnitTestSnippet(
    code: string | null | undefined,
    name: string | null | undefined,
  ): string {
    if (code == null) return "";
    const sanitized = stripUnittestMainBlock(String(code));
    if (!name) return sanitized;
    const snippet = extractMethodFromUnittest(String(code), String(name));
    return snippet.trim().length ? snippet : sanitized;
  }

  // Parsed review JSON (typed in backend as Review)
  $: review = safeParseJSON(llm?.review_json);

  // Transcript lines styled as chat bubbles
  type TranscriptMsg = { role: "AI" | "Program" | "Other"; text: string };
  $: transcriptMsgs = (() => {
    const t_llm = llm?.transcript; // Renamed t to avoid conflict with i18n t function
    if (!t_llm || typeof t_llm !== "string") return [] as TranscriptMsg[];
    return t_llm
      .split("\n")
      .map((s: string) => s.trim())
      .filter((s: string) => s.length > 0)
      .map((line: string): TranscriptMsg => {
        if (line.startsWith("AI> ")) return { role: "AI", text: line.slice(4) };
        if (line.startsWith("PROGRAM> "))
          return { role: "Program", text: line.slice(9) };
        return { role: "Other", text: line };
      });
  })();

  function openFiles() {
    if (files.length) {
      selected = files[0];
      fileDialog.showModal();
    }
  }

  async function downloadFiles() {
    try {
      if (!filesLoaded && !filesLoading) {
        if (pendingZip) {
          await loadFilesFromZip(pendingZip);
        } else if (submission?.code_content) {
          await loadFilesFromBytes(submission.code_content, lastCodeBytes);
        }
      }
      if (Array.isArray(files) && files.length) {
        const zip = new JSZip();
        for (const f of files) {
          zip.file(f.name, f.content ?? "");
        }
        const blob = await zip.generateAsync({ type: "blob" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        const safeTitle = (assignmentTitle || "submission")
          .replace(/[^a-z0-9_\-]+/gi, "_")
          .slice(0, 60);
        a.href = url;
        a.download = `${safeTitle}_${submission?.id ?? id}.zip`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
      } else {
        const textContent = submission?.code_content ?? "";
        const blob = new Blob([textContent], {
          type: "text/plain;charset=utf-8",
        });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `submission_${submission?.id ?? id}.txt`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
      }
    } catch (e: any) {
      err = e?.message ?? String(e);
    }
  }

  function goBack() {
    try {
      if (typeof window !== "undefined" && window.history.length > 1) {
        window.history.back();
        return;
      }
    } catch {}
    const fromTab = $page?.url?.searchParams?.get("fromTab");
    if (submission?.assignment_id) {
      const tabPart = fromTab ? `?tab=${fromTab}` : "";
      goto(`/assignments/${submission.assignment_id}${tabPart}`);
    } else {
      goto("/submissions");
    }
  }

  function chooseFile(n: FileNode) {
    if (n.content) {
      selected = { name: n.name, content: n.content };
    }
  }

  async function enterScratchFullscreen(mode: "player" | "both") {
    scratchFullscreenMode = mode;
    if (typeof document === "undefined") return;
    if (!scratchFullscreenHost || !scratchFullscreenHost.requestFullscreen) return;
    if (document.fullscreenElement === scratchFullscreenHost) return;
    try {
      await scratchFullscreenHost.requestFullscreen();
    } catch {}
  }

  async function exitScratchFullscreen() {
    scratchFullscreenMode = "none";
    if (typeof document === "undefined") return;
    if (!document.fullscreenElement) return;
    try {
      await document.exitFullscreen();
    } catch {}
  }

  function toggleScratchFullscreen(mode: "player" | "both") {
    if (scratchFullscreenMode === mode) {
      void exitScratchFullscreen();
      return;
    }
    void enterScratchFullscreen(mode);
  }

  $: if (selected) {
    highlighted = hljs.highlightAuto(selected.content).value;
  }

  $: if (!selected && submission && assignmentLoaded && !isScratchSubmission) {
    highlighted = hljs.highlightAuto(submission.code_content).value;
  }

  $: if (!selected && isScratchSubmission) {
    highlighted = "";
  }

  $: if (activeTab === "files" && isScratchSubmission && !filesLoaded && !filesLoading) {
    if (pendingZip) {
      void loadFilesFromZip(pendingZip);
    } else if (submission?.code_content) {
      void loadFilesFromBytes(submission.code_content, lastCodeBytes);
    }
  }

  onMount(() => {
    load();
    esCtrl = createEventSource(
      "/api/events",
      (src) => {
        src.addEventListener("status", (ev) => {
          const d = JSON.parse((ev as MessageEvent).data);
          if (submission && d.submission_id === submission.id) {
            submission.status = d.status;
            if (d.status !== "running") load();
          }
        });
        src.addEventListener("result", (ev) => {
          const d = JSON.parse((ev as MessageEvent).data);
          if (submission && d.submission_id === submission.id) {
            results = [...results, d];
          }
        });
      },
      {
        onError: (m) => {
          err = m;
        },
        onOpen: () => {
          err = "";
        },
      },
    );
    if (typeof document !== "undefined") {
      const handleFullscreenChange = () => {
        if (!document.fullscreenElement && scratchFullscreenMode !== "none") {
          scratchFullscreenMode = "none";
        }
      };
      document.addEventListener("fullscreenchange", handleFullscreenChange);
      removeScratchFullscreenListener = () => {
        document.removeEventListener("fullscreenchange", handleFullscreenChange);
      };
    }
  });
  onDestroy(() => {
    esCtrl?.close();
    removeScratchFullscreenListener?.();
  });
  $: sid = submission?.id ?? id;
  $: allTestsFailed =
    Array.isArray(results) &&
    results.length > 0 &&
    results.every((r) => r.status !== "passed" && r.status !== "running");

  let explanations: Record<string, { loading: boolean; text?: string; error?: string }> = {};
  let explainInFlight = false;
  let explainQueue: string[] = [];
  let summaryExplanation: { loading: boolean; text?: string; error?: string } = {
    loading: false,
  };

  async function fetchExplanation(sidStr: any, tcid: string) {
    return apiJSON(`/api/submissions/${sidStr}/explain-test-failure`, {
      method: "POST",
      body: JSON.stringify({ test_case_id: tcid }),
    });
  }

  async function fetchSummaryExplanation(sidStr: any) {
    return apiJSON(`/api/submissions/${sidStr}/explain-all-test-failures`, {
      method: "POST",
    });
  }

  async function askWhyFailed(sidStr: any, tcid: string) {
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
      const res = await fetchExplanation(sidStr, tcid);
      let firstApplied = false;
      while (explainQueue.length) {
        const queued = explainQueue;
        explainQueue = [];
        const results = await Promise.all(
          queued.map(async (id) => {
            try {
              const cached = await fetchExplanation(sidStr, id);
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

  async function askWhyAllFailed(sidStr: any) {
    if (summaryExplanation.loading || summaryExplanation.text) return;
    summaryExplanation = { loading: true };
    try {
      const res = await fetchSummaryExplanation(sidStr);
      summaryExplanation = { loading: false, text: res.explanation };
    } catch (e: any) {
      summaryExplanation = { loading: false, error: e.message as string };
    }
  }
</script>

{#if !submission}
  <div class="flex flex-col items-center justify-center py-20 gap-4">
    <span class="loading loading-ring loading-lg text-primary"></span>
    <p class="text-sm font-black uppercase tracking-[0.2em] opacity-40 animate-pulse">{t("frontend/src/routes/assignments/[id]/+page.svelte::loading_assignment")}</p>
  </div>
{:else}
  <div class="space-y-6">
    <!-- Premium Header Section -->
    <section class="relative overflow-hidden bg-base-100 rounded-3xl border border-base-200 shadow-xl shadow-base-300/30">
      <!-- Decorative background elements -->
      <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
      <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none"></div>
      
      <div class="relative p-6 sm:p-7 flex flex-col lg:flex-row gap-6">
        <div class="flex-1 space-y-8">
          <!-- Breadcrumbs / Back button -->
          <button on:click={goBack} class="group flex items-center gap-2 text-[10px] font-black uppercase tracking-[0.2em] opacity-40 hover:opacity-100 transition-all hover:text-primary">
            <ArrowLeft size={14} class="group-hover:-translate-x-1 transition-transform" />
            {t("frontend/src/routes/submissions/[id]/+page.svelte::back_button")}
          </button>

          <div class="space-y-4">
            <div class="flex flex-wrap items-center gap-3">
              <h1 class="text-2xl sm:text-3xl font-black tracking-tight text-base-content break-words max-w-2xl">
                {assignmentTitle || t("frontend/src/routes/submissions/[id]/+page.svelte::assignment_fallback_title")}
              </h1>
              <div class="flex items-center gap-2">
                <div class={`badge h-7 gap-2 px-2.5 font-black text-[9px] uppercase tracking-wider border-none shadow-sm ${statusColor(submission.status).replace('badge-', 'bg-')}/20 ${statusColor(submission.status).replace('badge-', 'text-')}`}>
                  {submission.status}
                </div>
                {#if submission.manually_accepted}
                  <div class="badge h-7 gap-2 px-2.5 font-black text-[9px] uppercase tracking-wider bg-success/20 text-success border-none shadow-sm">
                    <CheckCircle2 size={12} />
                    {t("frontend/src/routes/submissions/[id]/+page.svelte::manually_accepted_badge")}
                  </div>
                {/if}
              </div>
            </div>

            <div class="flex flex-wrap items-center gap-y-4 gap-x-8">
              {#if role === "teacher" || role === "admin"}
                <div class="flex items-center gap-3 group">
                  <div class="p-2 bg-base-200 rounded-lg group-hover:bg-primary/10 group-hover:text-primary transition-colors">
                    <User size={16} />
                  </div>
                  <div>
                    <div class="text-[9px] font-black uppercase tracking-widest opacity-40">{t("frontend/src/routes/assignments/[id]/+page.svelte::progress_table_header_student")}</div>
                    <div class="font-black text-sm">{submission.student_name || "Unknown Student"}</div>
                  </div>
                </div>
              {/if}

              <div class="flex items-center gap-3 group">
                <div class="p-2 bg-base-200 rounded-lg group-hover:bg-primary/10 group-hover:text-primary transition-colors">
                  <Calendar size={16} />
                </div>
                <div>
                  <div class="text-[9px] font-black uppercase tracking-widest opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::submitted_prefix")}</div>
                  <div class="font-black text-sm tabular-nums">{formatDateTime(submission.created_at)}</div>
                </div>
              </div>

              <div class="flex items-center gap-3 group">
                <div class="p-2 bg-base-200 rounded-lg group-hover:bg-primary/10 group-hover:text-primary transition-colors">
                  <Trophy size={16} />
                </div>
                <div>
                  <div class="text-[9px] font-black uppercase tracking-widest opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::attempt_prefix")}</div>
                  <div class="font-black text-sm tabular-nums">#{submission.attempt_number ?? submission.id}</div>
                </div>
              </div>

              {#if assignmentManual}
                <div class="flex items-center gap-3 group">
                  <div class="p-2 bg-info/10 text-info rounded-lg">
                    <FileCode size={16} />
                  </div>
                  <div>
                    <div class="text-[9px] font-black uppercase tracking-widest opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::manual_review_badge")}</div>
                    <div class="font-black text-sm text-info uppercase tracking-wider">{t("frontend/src/routes/assignments/[id]/+page.svelte::manual_teacher_review_option")}</div>
                  </div>
                </div>
              {/if}
            </div>
          </div>


        </div>

        {#if role === "teacher" || role === "admin" || (submission.points !== null || submission.override_points !== null)}
          <div class="lg:w-72 space-y-6">
            <!-- Points Card -->
            <div class="relative group">
              <div class="absolute inset-0 bg-primary/10 rounded-3xl blur-2xl opacity-50 group-hover:opacity-100 transition-opacity"></div>
              <div class="relative bg-base-200/50 backdrop-blur-md p-6 rounded-3xl border border-white/10 flex flex-col items-center justify-center gap-4 text-center overflow-hidden">
                <div class="absolute -top-10 -right-10 opacity-5 group-hover:scale-110 group-hover:rotate-12 transition-transform duration-700">
                  <Trophy size={160} />
                </div>
                
                <div class="text-[10px] font-black uppercase tracking-[0.3em] opacity-40">{t("frontend/src/routes/assignments/[id]/+page.svelte::points_earned_label")}</div>
                <div class="flex items-baseline gap-1">
                  <span class="text-5xl font-black text-primary tabular-nums">
                    {submission.override_points ?? submission.points ?? "—"}
                  </span>
                </div>

                {#if role === "teacher" || role === "admin"}
                  <div class="w-full pt-4 space-y-3">
                    <div class="join w-full shadow-md rounded-xl overflow-hidden border border-base-300">
                      <input
                        type="number"
                        step="1"
                        min="0"
                        class="join-item input input-bordered input-sm w-full font-black text-center focus:outline-none"
                        bind:value={overrideValue}
                        placeholder="Pts"
                      />
                      <button
                        class={`join-item btn btn-primary btn-sm px-4 ${savingOverride ? "loading" : ""}`}
                        on:click={saveOverride}
                        disabled={savingOverride}
                      >
                        <Save size={16} />
                      </button>
                    </div>

                    {#if submission.manually_accepted}
                      <button
                        class="btn btn-warning btn-sm w-full rounded-xl font-black uppercase tracking-widest text-[9px]"
                        on:click={async () => {
                          try {
                            await apiFetch(`/api/submissions/${submission.id}/undo-accept`, {
                              method: "PUT",
                              headers: { "Content-Type": "application/json" },
                              body: JSON.stringify({}),
                            });
                            await load();
                          } catch (e: any) { err = e.message; }
                        }}
                      >
                        {t("frontend/src/routes/submissions/[id]/+page.svelte::undo_manual_acceptance_button")}
                      </button>
                    {:else}
                      <button
                        class="btn btn-success btn-sm w-full rounded-xl font-black uppercase tracking-widest text-[9px] shadow-lg shadow-success/20"
                        on:click={async () => {
                          try {
                            const raw: any = overrideValue;
                            const v = raw === "" ? null : parseInt(raw, 10);
                            await apiFetch(`/api/submissions/${submission.id}/accept`, {
                              method: "PUT",
                              headers: { "Content-Type": "application/json" },
                              body: JSON.stringify({ points: v }),
                            });
                            await load();
                          } catch (e: any) { err = e.message; }
                        }}
                      >
                        <CheckCircle2 size={14} />
                        {t("frontend/src/routes/submissions/[id]/+page.svelte::accept_submission_button")}
                      </button>
                    {/if}
                  </div>
                {/if}
              </div>
            </div>
          </div>
        {/if}
      </div>
    </section>

    <!-- Tab Navigation -->
    <div class="flex flex-wrap items-center gap-1 p-1 bg-base-200/50 backdrop-blur-sm rounded-xl border border-base-300/50 mb-0 max-w-fit shadow-inner">
      {#if !isScratchSubmission}
        <button 
          class={`px-3 py-1.5 rounded-[0.6rem] text-[9px] font-black uppercase tracking-widest transition-all ${activeTab === 'results' ? 'bg-base-100 text-primary shadow-md' : 'hover:bg-base-300/50 opacity-50 hover:opacity-100'}`}
          on:click={() => activeTab = 'results'}
        >
          <div class="flex items-center gap-2 text-[11px]">
            <FlaskConical size={12} />
            {t("frontend/src/routes/submissions/[id]/+page.svelte::results_title")}
          </div>
        </button>
      {/if}

      {#if isScratchSubmission}
        <button 
          class={`px-3 py-1.5 rounded-[0.6rem] text-[9px] font-black uppercase tracking-widest transition-all ${activeTab === 'scratch' ? 'bg-base-100 text-primary shadow-md' : 'hover:bg-base-300/50 opacity-50 hover:opacity-100'}`}
          on:click={() => activeTab = 'scratch'}
        >
          <div class="flex items-center gap-2 text-[11px]">
            <Gamepad2 size={12} />
            {t("frontend/src/routes/submissions/[id]/+page.svelte::scratch_tab")}
          </div>
        </button>
      {/if}
      
      <button 
        class={`px-3 py-1.5 rounded-[0.6rem] text-[9px] font-black uppercase tracking-widest transition-all ${activeTab === 'files' ? 'bg-base-100 text-primary shadow-md' : 'hover:bg-base-300/50 opacity-50 hover:opacity-100'}`}
        on:click={() => activeTab = 'files'}
      >
        <div class="flex items-center gap-2 text-[11px]">
          <FileCode size={12} />
          {t("frontend/src/routes/submissions/[id]/+page.svelte::files_dialog_title")}
        </div>
      </button>

      {#if showLLM}
        <button 
          class={`px-3 py-1.5 rounded-[0.6rem] text-[9px] font-black uppercase tracking-widest transition-all ${activeTab === 'review' ? 'bg-base-100 text-primary shadow-md' : 'hover:bg-base-300/50 opacity-50 hover:opacity-100'}`}
          on:click={() => activeTab = 'review'}
        >
          <div class="flex items-center gap-2 text-[11px]">
            <Search size={12} />
            {t("frontend/src/routes/submissions/[id]/+page.svelte::llm_review_tab")}
          </div>
        </button>
      {/if}
    </div>

    <div class="space-y-6">
      {#if activeTab === 'results'}
        {#if showAutoUI}
          <!-- Stats Summary Card -->
          <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div class="bg-base-100 p-4 rounded-2xl border border-base-200 shadow-sm group hover:border-primary/30 transition-all">
              <div class="text-[9px] font-black uppercase tracking-widest opacity-40 mb-1">{t("frontend/src/routes/submissions/[id]/+page.svelte::stat_tests")}</div>
              <div class="flex items-center gap-3">
                <div class="w-8 h-8 rounded-lg bg-base-200 flex items-center justify-center text-base-content group-hover:bg-primary/10 group-hover:text-primary transition-colors">
                  <Cpu size={16} />
                </div>
                <div class="text-xl font-black tabular-nums">{totalTests}</div>
              </div>
            </div>
            
            <div class="bg-base-100 p-4 rounded-2xl border border-base-200 shadow-sm group hover:border-success/30 transition-all">
              <div class="text-[9px] font-black uppercase tracking-widest opacity-40 mb-1">{t("frontend/src/routes/submissions/[id]/+page.svelte::stat_passed")}</div>
              <div class="flex items-center gap-3">
                <div class="w-8 h-8 rounded-lg bg-success/10 text-success flex items-center justify-center group-hover:scale-110 transition-transform">
                  <CheckCircle2 size={16} />
                </div>
                <div class="text-xl font-black text-success tabular-nums">{passedCount}</div>
              </div>
            </div>

            <div class="bg-base-100 p-4 rounded-2xl border border-base-200 shadow-sm group hover:border-warning/30 transition-all">
              <div class="text-[9px] font-black uppercase tracking-widest opacity-40 mb-1">{t("frontend/src/routes/submissions/[id]/+page.svelte::stat_limited")}</div>
              <div class="flex items-center gap-3">
                <div class="w-8 h-8 rounded-lg bg-warning/10 text-warning flex items-center justify-center group-hover:scale-110 transition-transform">
                  <Clock size={16} />
                </div>
                <div class="text-xl font-black text-warning tabular-nums">{warnedCount}</div>
              </div>
            </div>

            <div class="bg-base-100 p-4 rounded-2xl border border-base-200 shadow-sm group hover:border-error/30 transition-all">
              <div class="text-[9px] font-black uppercase tracking-widest opacity-40 mb-1">{t("frontend/src/routes/submissions/[id]/+page.svelte::stat_failed")}</div>
              <div class="flex items-center gap-3">
                <div class="w-8 h-8 rounded-lg bg-error/10 text-error flex items-center justify-center group-hover:scale-110 transition-transform">
                  <AlertCircle size={16} />
                </div>
                <div class="text-xl font-black text-error tabular-nums">{failedCount}</div>
              </div>
            </div>
          </div>

          <!-- Progress Visualization -->
          <div class="h-3 w-full rounded-full bg-base-200 overflow-hidden flex shadow-inner border border-base-300/30">
            {#each results as r}
              <div
                class={`h-full flex-1 transition-all duration-500 hover:opacity-80 cursor-help ${bgFromBadge(resultColor(r.status))}`}
                title={`${r.status}`}
              ></div>
            {/each}
            {#if !results.length}
              <div class="h-full w-full bg-gradient-to-r from-base-200 via-primary/20 to-base-200 animate-shimmer" style="background-size: 200% 100%"></div>
            {/if}
          </div>
          <div class="space-y-6">
            <!-- AI Summary (Why did everything fail?) -->
            {#if assignmentLLMHelpWhyFailed && allTestsFailed}
              <div class="bg-primary/5 border border-primary/20 rounded-3xl p-6 relative overflow-hidden group">
                <div class="absolute -right-12 -bottom-12 text-primary/10 group-hover:scale-110 transition-transform duration-700">
                  <Sparkles size={160} />
                </div>
                <div class="relative space-y-4 max-w-2xl">
                  <div class="flex items-center gap-2 text-primary font-black uppercase tracking-[0.2em] text-[9px]">
                    <Sparkles size={14} />
                    {t("frontend/src/routes/submissions/[id]/+page.svelte::explain_all_failures_btn")}
                  </div>
                  {#if summaryExplanation.text}
                    <p class="text-sm leading-relaxed font-medium opacity-80">{summaryExplanation.text}</p>
                  {:else}
                    <button
                      class="btn btn-primary btn-sm rounded-xl px-5 h-8 font-black uppercase tracking-widest text-[9px] shadow-lg shadow-primary/20"
                      on:click|preventDefault|stopPropagation={() => askWhyAllFailed(sid)}
                      disabled={summaryExplanation.loading}
                    >
                      {#if summaryExplanation.loading}
                        <span class="loading loading-spinner loading-xs"></span>
                        {t("frontend/src/routes/assignments/[id]/+page.svelte::explain_failure_loading")}
                      {:else}
                        <Sparkles size={14} />
                        {t("frontend/src/routes/submissions/[id]/+page.svelte::explain_all_failures_btn")}
                      {/if}
                    </button>
                    {#if summaryExplanation.error}
                      <p class="text-xs text-error font-black uppercase tracking-widest opacity-60">
                        {t("frontend/src/routes/assignments/[id]/+page.svelte::explain_failure_error")}
                      </p>
                    {/if}
                  {/if}
                </div>
              </div>
            {/if}

            <div class="bg-base-200/40 rounded-3xl border border-base-200 shadow-lg shadow-base-300/20 overflow-hidden">
              <div class="px-6 py-4 border-b border-base-200 flex items-center justify-between bg-base-100/50 backdrop-blur-sm">
                <div class="flex items-center gap-3">
                  <div class="p-2 bg-primary/10 text-primary rounded-lg">
                    <FlaskConical size={18} />
                  </div>
                  <h2 class="text-lg font-black tracking-tight">{t("frontend/src/routes/submissions/[id]/+page.svelte::results_title")}</h2>
                </div>
              </div>
              
              <div class="p-4 space-y-3">
                {#if Array.isArray(results) && results.length}
                  {#each results as r, i}
                    {@const mode = r.execution_mode ?? (r.unittest_name ? "unittest" : r.function_name ? "function" : "stdin_stdout")}
                    {@const allowLog = allowTraceback || r.status === "illegal_tool_use"}
                    <div class="group bg-base-100 rounded-2xl border border-base-200 shadow-sm hover:shadow-md hover:border-primary/30 transition-all overflow-hidden">
                      <details class="collapse collapse-arrow">
                        <summary class="collapse-title !p-0">
                          <div class="px-5 pr-12 py-4 flex flex-col md:flex-row md:items-center justify-between gap-6">
                            <div class="flex items-center gap-4">
                              <div class={`w-10 h-10 rounded-xl flex items-center justify-center font-black text-sm shadow-sm transition-transform group-hover:scale-110 ${statusColor(r.status).includes('success') ? 'bg-success/10 text-success' : statusColor(r.status).includes('error') ? 'bg-error/10 text-error' : 'bg-warning/10 text-warning'}`}>
                                {r.test_number ?? i + 1}
                              </div>
                              <div class="space-y-1">
                                <div class="flex items-center gap-2">
                                  <span class="font-black text-sm tracking-tight">
                                    {t("frontend/src/routes/submissions/[id]/+page.svelte::test_prefix")}{r.test_number ?? i + 1}
                                  </span>
                                </div>
                                <div class="flex items-center gap-4 text-[10px] font-black uppercase tracking-[0.1em] opacity-40">
                                  <span class="flex items-center gap-1.5"><Clock size={12} /> {r.runtime_ms}{t("frontend/src/routes/submissions/[id]/+page.svelte::milliseconds_unit")}</span>
                                  <span class="flex items-center gap-1.5"><Shield size={12} /> {t("frontend/src/routes/submissions/[id]/+page.svelte::exit_code_prefix")}{r.exit_code}</span>
                                </div>
                              </div>
                            </div>

                            <div class="flex items-center gap-3">
                              {#if assignmentLLMHelpWhyFailed && !allTestsFailed && r.status !== "passed" && r.status !== "running" && r.test_case_id}
                                <div class="relative">
                                  {#if explanations[r.test_case_id]?.text}
                                    <div class="p-2.5 bg-base-200 rounded-xl border border-base-300 shadow-sm max-w-sm text-xs font-medium leading-relaxed group/explain relative pr-8">
                                      <div class="flex gap-2">
                                        <Sparkles size={12} class="text-primary mt-0.5 shrink-0" />
                                        <span>{explanations[r.test_case_id].text}</span>
                                      </div>
                                    </div>
                                  {:else}
                                    <button 
                                      class="btn btn-ghost btn-xs h-8 rounded-lg px-3 gap-2 text-primary font-black uppercase tracking-widest text-[8px] hover:bg-primary/10 transition-all border border-primary/20" 
                                      on:click|preventDefault|stopPropagation={() => askWhyFailed(sid, r.test_case_id)} 
                                      disabled={explanations[r.test_case_id]?.loading}
                                    >
                                      {#if explanations[r.test_case_id]?.loading}
                                        <span class="loading loading-spinner loading-xs"></span>
                                      {:else}
                                        <Sparkles size={14} />
                                        {t("frontend/src/routes/assignments/[id]/+page.svelte::explain_failure_btn")}
                                      {/if}
                                    </button>
                                  {/if}
                                </div>
                              {/if}
                              <div class={`badge badge-md h-6 px-3 font-black text-[8px] uppercase tracking-widest border-none shadow-sm ${statusColor(r.status).replace('badge-', 'bg-')}/20 ${statusColor(r.status).replace('badge-', 'text-')}`}>
                                {r.status}
                              </div>
                            </div>
                          </div>
                        </summary>
                        <div class="collapse-content px-5 pb-5 pt-0">
                          <div class="border-t border-base-200/50 mb-5"></div>
                          <div class="grid lg:grid-cols-2 gap-4">
                            {#if allowTestDetails}
                              <div class="space-y-4">
                                <div class="text-[10px] font-black uppercase tracking-widest opacity-40 flex items-center gap-2">
                                  <FileCode size={14} />
                                  {t("frontend/src/routes/submissions/[id]/+page.svelte::test_definition_title")}
                                </div>
                                <div class="bg-base-200/50 rounded-2xl border border-base-300/50 p-5 space-y-4 overflow-hidden">
                                  {#if mode === "function"}
                                    <div class="grid grid-cols-2 gap-4">
                                      <div class="space-y-1">
                                        <div class="text-[9px] font-black uppercase opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::function_label")}</div>
                                        <code class="text-xs font-black">{r.function_name}</code>
                                      </div>
                                      <div class="space-y-1">
                                        <div class="text-[9px] font-black uppercase opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::expected_return_label")}</div>
                                        <code class="text-xs font-black text-primary">{r.expected_return ?? "None"}</code>
                                      </div>
                                      <div class="col-span-2 space-y-1">
                                        <div class="text-[9px] font-black uppercase opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::arguments_label")}</div>
                                        <code class="text-xs opacity-80 break-words">{r.function_args ?? "[]"}</code>
                                      </div>
                                    </div>
                                  {:else if r.unittest_code}
                                    <pre class="text-xs font-mono bg-base-300/50 p-4 rounded-xl overflow-auto border border-base-300 max-h-60"><code>{viewableUnitTestSnippet(r.unittest_code, r.unittest_name)}</code></pre>
                                  {:else if typeof r.stdin !== "undefined" || typeof r.expected_stdout !== "undefined"}
                                    <div class="space-y-4">
                                      {#if r.stdin}
                                        <div class="space-y-1">
                                          <div class="text-[9px] font-black uppercase opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::input_label")}</div>
                                          <pre class="text-xs font-mono bg-base-300/50 p-3 rounded-xl overflow-auto border border-base-300 max-h-32"><code>{r.stdin}</code></pre>
                                        </div>
                                      {/if}
                                      <div class="space-y-1">
                                        <div class="text-[9px] font-black uppercase opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::expected_output_label")}</div>
                                        <pre class="text-xs font-mono bg-base-300/50 p-3 rounded-xl overflow-auto border border-base-300 max-h-32"><code>{r.expected_stdout}</code></pre>
                                      </div>
                                    </div>
                                  {/if}
                                </div>
                              </div>
                            {/if}

                            {#if allowLog}
                              <div class="space-y-4">
                                <div class="text-[10px] font-black uppercase tracking-widest opacity-40 flex items-center gap-2">
                                  <History size={14} />
                                  {t("frontend/src/routes/submissions/[id]/+page.svelte::execution_log_title")}
                                </div>
                                <div class="bg-base-300/30 rounded-2xl border border-base-300/50 p-5 overflow-hidden">
                                  {#if r.stderr}
                                    <pre class="text-xs font-mono text-error/80 whitespace-pre-wrap max-h-60 overflow-auto"><code>{r.stderr}</code></pre>
                                  {:else}
                                    <div class="text-xs font-black uppercase opacity-20 tracking-widest py-8 text-center">{t("frontend/src/routes/submissions/[id]/+page.svelte::no_stderr_output_message")}</div>
                                  {/if}
                                </div>
                              </div>
                            {/if}
                          </div>
                        </div>
                      </details>
                    </div>
                  {/each}
                {:else}
                  <div class="bg-base-100/40 rounded-2xl border border-dashed border-base-300/60 p-20 flex flex-col items-center justify-center text-center gap-4">
                    <div class="p-4 bg-base-200/50 rounded-full text-base-content/20">
                      <FlaskConical size={48} />
                    </div>
                    <div>
                      <p class="font-black uppercase tracking-[0.2em] text-[10px] opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::no_results_yet_message")}</p>
                    </div>
                  </div>
                {/if}
              </div>
            </div>

            <!-- Manual Console Section -->
            {#if (role === "teacher" || role === "admin") && !isScratchSubmission}
              <div class="bg-base-200/40 rounded-3xl border border-base-200 shadow-lg shadow-base-300/20 overflow-hidden">
                <div class="px-6 py-4 border-b border-base-200 flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-base-100/50 backdrop-blur-sm">
                  <div class="flex items-center gap-3">
                    <div class="p-2 bg-warning/10 text-warning rounded-lg">
                      <Cpu size={18} />
                    </div>
                    <div>
                      <h2 class="text-xl font-black tracking-tight">{t("frontend/src/routes/submissions/[id]/+page.svelte::manual_testing_title")}</h2>
                      <p class="text-[10px] font-black uppercase tracking-widest opacity-40">
                        {t("frontend/src/routes/submissions/[id]/+page.svelte::manual_testing_description")}
                      </p>
                    </div>
                  </div>
                  {#if !forceManualConsole}
                    <button
                      class="btn btn-ghost btn-sm rounded-lg px-4 h-8 font-black uppercase tracking-widest text-[9px] border border-base-300"
                      on:click={() => (manualConsoleVisible = !manualConsoleVisible)}
                    >
                      {manualConsoleVisible ? t("frontend/src/routes/submissions/[id]/+page.svelte::hide_console_button") : t("frontend/src/routes/submissions/[id]/+page.svelte::show_console_button")}
                    </button>
                  {/if}
                </div>

                {#if forceManualConsole || manualConsoleVisible}
                  <div class="p-6 bg-base-200/50 backdrop-blur-sm">
                    <RunConsole submissionId={sid} />
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        {/if}
      {/if}

      {#if activeTab === 'scratch'}
        <div
          class={scratchFullscreenMode !== "none" ? "scratch-fullscreen-shell" : "space-y-6"}
          bind:this={scratchFullscreenHost}
        >
          <div class={`flex flex-wrap items-center justify-between gap-3 ${scratchFullscreenMode !== "none" ? "scratch-fullscreen-toolbar" : ""}`}>
            <div class="flex flex-wrap items-center gap-2">
              <button
                class={`btn btn-sm rounded-lg px-3 h-8 font-black uppercase tracking-widest text-[9px] ${scratchFullscreenMode === "player" ? "btn-primary" : "btn-ghost border border-base-300"} ${scratchFullscreenMode !== "none" ? "text-white" : ""}`}
                on:click={() => toggleScratchFullscreen("player")}
              >
                <Maximize2 size={14} />
                {t("frontend/src/routes/submissions/[id]/+page.svelte::scratch_fullscreen_game")}
              </button>
              <button
                class={`btn btn-sm rounded-lg px-3 h-8 font-black uppercase tracking-widest text-[9px] ${scratchFullscreenMode === "both" ? "btn-primary" : "btn-ghost border border-base-300"} ${scratchFullscreenMode !== "none" ? "text-white" : ""}`}
                on:click={() => toggleScratchFullscreen("both")}
              >
                <Maximize2 size={14} />
                {t("frontend/src/routes/submissions/[id]/+page.svelte::scratch_fullscreen_game_blocks")}
              </button>
            </div>
            {#if scratchFullscreenMode !== "none"}
              <button
                class="btn btn-ghost btn-sm rounded-lg px-3 h-8 font-black uppercase tracking-widest text-[9px] border border-base-300 text-white"
                on:click={exitScratchFullscreen}
              >
                <Minimize2 size={14} />
                {t("frontend/src/routes/submissions/[id]/+page.svelte::scratch_fullscreen_exit")}
              </button>
            {/if}
          </div>

          <div
            class={scratchFullscreenMode !== "none"
              ? `scratch-fullscreen-grid ${scratchFullscreenMode === "both" ? "scratch-fullscreen-grid-both" : "scratch-fullscreen-grid-player"}`
              : "space-y-6"}
          >
            <div class={`bg-base-100 rounded-3xl border border-base-200 shadow-lg shadow-base-300/30 overflow-hidden ${scratchFullscreenMode !== "none" ? "scratch-fullscreen-card" : ""}`}>
              <div class="px-6 py-4 border-b border-base-200 flex flex-wrap items-center justify-between gap-3 bg-base-100/50 backdrop-blur-sm">
                <div class="flex items-center gap-3">
                  <div class="p-2 bg-secondary/10 text-secondary rounded-lg">
                    <Gamepad2 size={18} />
                  </div>
                  <h2 class="text-lg font-black tracking-tight">{t("frontend/src/routes/submissions/[id]/+page.svelte::scratch_project_title")}</h2>
                </div>
                {#if scratchProjectName}
                  <span class="text-xs font-mono font-bold opacity-60">{scratchProjectName}</span>
                {/if}
              </div>
              <div class={`p-6 ${scratchFullscreenMode !== "none" ? "scratch-fullscreen-body" : ""}`}>
                {#if scratchLoading}
                  <div class="flex items-center gap-2 text-sm opacity-70">
                    <span class="loading loading-spinner loading-sm"></span>
                    {t("frontend/src/routes/submissions/[id]/+page.svelte::scratch_project_loading")}
                  </div>
                {:else if scratchProjectError}
                  <div class="alert bg-error/10 border-error/20 text-error-content rounded-2xl">
                    <AlertCircle size={18} />
                    <span class="font-medium text-sm">{scratchProjectError}</span>
                  </div>
                {:else if scratchProject}
                  <ScratchPlayer
                    projectData={scratchProject}
                    projectName={scratchProjectName}
                    fullScreen={scratchFullscreenMode !== "none"}
                  />
                {:else}
                  <div class="alert bg-warning/10 border-warning/20 text-warning-content rounded-2xl">
                    <AlertTriangle size={18} />
                    <span class="font-medium text-sm">
                      {t("frontend/src/routes/submissions/[id]/+page.svelte::scratch_project_empty")}
                    </span>
                  </div>
                {/if}
              </div>
            </div>

            {#if scratchFullscreenMode !== "player"}
              <div class={`bg-base-100 rounded-3xl border border-base-200 shadow-lg shadow-base-300/30 overflow-hidden ${scratchFullscreenMode !== "none" ? "scratch-fullscreen-card" : ""}`}>
                <div class="px-6 py-4 border-b border-base-200 flex flex-wrap items-center justify-between gap-3 bg-base-100/50 backdrop-blur-sm">
                  <div class="flex items-center gap-3">
                    <div class="p-2 bg-base-200 text-base-content/70 rounded-lg">
                      <FileCode size={18} />
                    </div>
                    <h2 class="text-lg font-black tracking-tight">
                      {t("frontend/src/routes/submissions/[id]/+page.svelte::scratch_blocks_title")}
                    </h2>
                  </div>
                </div>
                <div class={`p-6 ${scratchFullscreenMode !== "none" ? "scratch-fullscreen-body scratch-fullscreen-body--blocks" : ""}`}>
                  {#if scratchLoading}
                    <div class="flex items-center gap-2 text-sm opacity-70">
                      <span class="loading loading-spinner loading-sm"></span>
                      {t("frontend/src/routes/submissions/[id]/+page.svelte::scratch_blocks_loading")}
                    </div>
                  {:else if scratchProjectError}
                    <div class="alert bg-error/10 border-error/20 text-error-content rounded-2xl">
                      <AlertCircle size={18} />
                      <span class="font-medium text-sm">{scratchProjectError}</span>
                    </div>
                  {:else if scratchProject}
                    <ScratchBlocksViewer projectData={scratchProject} fullHeight={scratchFullscreenMode === "both"} />
                  {:else}
                    <div class="alert bg-warning/10 border-warning/20 text-warning-content rounded-2xl">
                      <AlertTriangle size={18} />
                      <span class="font-medium text-sm">
                        {t("frontend/src/routes/submissions/[id]/+page.svelte::scratch_project_empty")}
                      </span>
                    </div>
                  {/if}
                </div>
              </div>
            {/if}
          </div>
        </div>
      {/if}

      {#if activeTab === 'files'}
        <div class="bg-base-100 rounded-3xl border border-base-200 shadow-lg shadow-base-300/30 overflow-hidden min-h-[500px]">
          <div class="px-6 py-4 border-b border-base-200 flex items-center justify-between bg-base-100/50 backdrop-blur-sm">
            <div class="flex items-center gap-3">
              <div class="p-2 bg-secondary/10 text-secondary rounded-lg">
                <FileCode size={18} />
              </div>
              <h2 class="text-lg font-black tracking-tight">{t("frontend/src/routes/submissions/[id]/+page.svelte::files_dialog_title")}</h2>
            </div>
            <button class="btn btn-secondary btn-sm rounded-lg px-4 h-8 font-black uppercase tracking-widest text-[9px] shadow-lg shadow-secondary/20" on:click={downloadFiles}>
              <ExternalLink size={14} />
              {t("frontend/src/routes/submissions/[id]/+page.svelte::download_button")}
            </button>
          </div>
          
          <div class="flex flex-col md:flex-row h-[600px]">
            <div class="md:w-60 border-r border-base-200 bg-base-50 overflow-auto p-3">
              <FileTree nodes={tree} select={chooseFile} />
            </div>
            <div class="flex-1 overflow-hidden flex flex-col bg-base-200/20">
              <div class="px-4 py-2 border-b border-base-200 bg-base-100/80 backdrop-blur-md flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <div class="w-1.5 h-1.5 rounded-full bg-secondary"></div>
                  <span class="font-black text-[10px] tracking-tight opacity-70">{selected?.name || "No file selected"}</span>
                </div>
              </div>
              <div class="flex-1 overflow-auto p-0">
                <pre class="text-xs font-mono p-4 min-h-full bg-transparent"><code class="hljs">{@html highlighted || "Empty file"}</code></pre>
              </div>
            </div>
          </div>
        </div>
      {/if}

      {#if activeTab === 'review'}
        <div class="space-y-6">
          {#if llm}
            <div class="grid md:grid-cols-3 gap-4">
              <div class="bg-base-100 p-6 rounded-3xl border border-base-200 shadow-sm relative overflow-hidden group">
                <div class="text-[9px] font-black uppercase tracking-widest opacity-40 mb-3">{t("frontend/src/routes/submissions/[id]/+page.svelte::smoke_test_label")}</div>
                <div class="flex items-center gap-4">
                  <div class={`w-12 h-12 rounded-xl flex items-center justify-center transition-transform group-hover:scale-110 ${llm.smoke_ok ? 'bg-success/10 text-success' : 'bg-error/10 text-error'}`}>
                    {#if llm.smoke_ok}<CheckCircle2 size={24} />{:else}<AlertCircle size={24} />{/if}
                  </div>
                  <div class="text-lg font-black tracking-tight">
                    {llm.smoke_ok ? t("frontend/src/routes/submissions/[id]/+page.svelte::smoke_test_ok") : t("frontend/src/routes/submissions/[id]/+page.svelte::smoke_test_failed")}
                  </div>
                </div>
              </div>
 
              <div class="bg-base-100 p-6 rounded-3xl border border-base-200 shadow-sm relative overflow-hidden group">
                <div class="text-[9px] font-black uppercase tracking-widest opacity-40 mb-3">{t("frontend/src/routes/submissions/[id]/+page.svelte::verdict_label")}</div>
                <div class="flex items-center gap-4">
                  <div class="w-12 h-12 rounded-xl bg-primary/10 text-primary flex items-center justify-center transition-transform group-hover:scale-110">
                    <GraduationCap size={24} />
                  </div>
                  <div class="text-lg font-black tracking-tight uppercase">
                    {llm.verdict ?? t("frontend/src/routes/submissions/[id]/+page.svelte::dash_placeholder")}
                  </div>
                </div>
              </div>
 
              <div class="md:col-span-1 bg-base-100 p-6 rounded-3xl border border-base-200 shadow-sm group">
                <div class="text-[9px] font-black uppercase tracking-widest opacity-40 mb-3">{t("frontend/src/routes/submissions/[id]/+page.svelte::reason_label")}</div>
                <p class="text-xs font-medium leading-relaxed opacity-70">
                  {llm.reason ?? t("frontend/src/routes/submissions/[id]/+page.svelte::dash_placeholder")}
                </p>
              </div>
            </div>
 
            {#if review && allowLLMDetails}
              <div class="bg-base-100 rounded-3xl border border-base-200 shadow-lg shadow-base-300/30 overflow-hidden">
                <div class="px-6 py-4 border-b border-base-200 bg-base-100/50 backdrop-blur-sm flex items-center gap-3">
                  <div class="p-2 bg-primary/10 text-primary rounded-lg">
                    <Search size={18} />
                  </div>
                  <h2 class="text-lg font-black tracking-tight">{t("frontend/src/routes/submissions/[id]/+page.svelte::llm_review_title")}</h2>
                </div>
                
                <div class="p-6 space-y-6">
                  {#if review.summary}
                    <div class="space-y-2">
                      <div class="text-[9px] font-black uppercase tracking-widest opacity-40">{t("frontend/src/routes/assignments/[id]/+page.svelte::assignment_description_label")}</div>
                      <p class="text-base leading-relaxed font-medium">{review.summary}</p>
                    </div>
                  {/if}
 
                  {#if Array.isArray(review.issues) && review.issues.length}
                    <div class="space-y-4">
                      <div class="text-[9px] font-black uppercase tracking-widest opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::issues_title")}</div>
                      <div class="grid gap-3">
                        {#each review.issues as issue}
                          <div class="bg-base-200/50 rounded-2xl p-5 border border-base-300/50 space-y-3 group">
                            <div class="flex flex-wrap items-center justify-between gap-4">
                              <div class="flex items-center gap-3">
                                <div class={`w-1.5 h-1.5 rounded-full ${issue.severity === 'critical' ? 'bg-error animate-pulse' : issue.severity === 'high' ? 'bg-warning' : 'bg-info'}`}></div>
                                <div class="text-base font-black tracking-tight">{issue.title}</div>
                              </div>
                              <span class={`badge font-black text-[8px] uppercase tracking-widest px-2 h-5 border-none ${issue.severity === 'critical' ? 'bg-error/20 text-error' : issue.severity === 'high' ? 'bg-warning/20 text-warning' : 'bg-info/20 text-info'}`}>
                                {issue.severity}
                              </span>
                            </div>
                            
                            {#if issue.rationale}
                              <p class="text-xs font-medium opacity-70 leading-relaxed">{issue.rationale}</p>
                            {/if}
 
                            {#if issue.reproduction}
                              <div class="bg-base-300/50 rounded-xl p-3.5 space-y-2 border border-base-300/30">
                                <div class="text-[8px] font-black uppercase opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::reproduction_label")}</div>
                                {#if Array.isArray(issue.reproduction.inputs) && issue.reproduction.inputs.length}
                                  <div class="flex flex-wrap gap-1.5">
                                    {#each issue.reproduction.inputs as inp}
                                      <code class="bg-base-100 px-2 py-1 rounded-md text-[11px] font-black shadow-sm">{inp}</code>
                                    {/each}
                                  </div>
                                {/if}
                                {#if issue.reproduction.expect_regex}
                                  <div class="flex items-center gap-2">
                                    <span class="text-[9px] font-black uppercase tracking-widest opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::expect_label")}:</span>
                                    <code class="text-[11px] font-black text-primary bg-primary/10 px-2 py-1 rounded-md">/{issue.reproduction.expect_regex}/</code>
                                  </div>
                                {/if}
                              </div>
                            {/if}
                          </div>
                        {/each}
                      </div>
                    </div>
                  {/if}
 
                  {#if Array.isArray(review.suggestions) && review.suggestions.length}
                    <div class="space-y-3">
                      <div class="text-[9px] font-black uppercase tracking-widest opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::suggestions_title")}</div>
                      <div class="flex flex-wrap gap-2">
                        {#each review.suggestions as s}
                          <div class="bg-secondary/5 text-secondary border border-secondary/20 px-4 py-2 rounded-xl text-xs font-black tracking-tight flex items-center gap-2">
                            <Sparkles size={14} />
                            {s}
                          </div>
                        {/each}
                      </div>
                    </div>
                  {/if}
 
                  {#if review.acceptance}
                    <div class="pt-5 border-t border-base-200">
                      <div class="bg-base-200/50 rounded-2xl p-6 flex flex-col md:flex-row md:items-center justify-between gap-6">
                        <div class="space-y-1.5">
                          <div class="text-[9px] font-black uppercase tracking-widest opacity-40">{t("frontend/src/routes/submissions/[id]/+page.svelte::acceptance_title")}</div>
                          <div class="text-xl font-black tracking-tight">{review.acceptance.ok ? t("frontend/src/routes/submissions/[id]/+page.svelte::accepted_status") : t("frontend/src/routes/submissions/[id]/+page.svelte::rejected_status")}</div>
                          {#if review.acceptance.reason}
                            <p class="text-xs font-medium opacity-70">{review.acceptance.reason}</p>
                          {/if}
                        </div>
                        <div class={`w-16 h-16 rounded-2xl flex items-center justify-center shadow-xl ${review.acceptance.ok ? 'bg-success text-success-content shadow-success/40' : 'bg-error text-error-content shadow-error/40'}`}>
                          {#if review.acceptance.ok}<CheckCircle2 size={32} />{:else}<AlertTriangle size={32} />{/if}
                        </div>
                      </div>
                    </div>
                  {/if}
                </div>
              </div>
            {/if}
 
            {#if transcriptMsgs.length && allowLLMDetails}
              <div class="bg-base-100 rounded-3xl border border-base-200 shadow-lg shadow-base-300/30 overflow-hidden">
                <div class="px-6 py-4 border-b border-base-200 bg-base-100/50 backdrop-blur-sm flex items-center gap-3">
                  <div class="p-2 bg-base-300 text-base-content/70 rounded-lg">
                    <History size={18} />
                  </div>
                  <h2 class="text-lg font-black tracking-tight">{t("frontend/src/routes/submissions/[id]/+page.svelte::interactive_transcript_title")}</h2>
                </div>
                <div class="p-6 space-y-4 bg-base-200/20">
                  {#each transcriptMsgs as m}
                    <div class={`flex ${m.role === 'AI' ? 'justify-end' : 'justify-start'}`}>
                      <div class={`max-w-[85%] p-4.5 rounded-2xl shadow-sm relative group ${m.role === 'AI' ? 'bg-primary text-primary-content rounded-tr-none' : 'bg-base-100 text-base-content rounded-tl-none border border-base-200'}`}>
                        <div class={`text-[8px] font-black uppercase tracking-widest mb-1.5 opacity-50 ${m.role === 'AI' ? 'text-primary-content' : 'text-base-content/60'}`}>{m.role}</div>
                        <p class="text-xs font-medium leading-relaxed">{m.text}</p>
                      </div>
                    </div>
                  {/each}
                </div>
              </div>
            {/if}
          {:else}
            <div class="bg-base-100 p-12 rounded-3xl border border-base-200 text-center opacity-40">
              <Search size={48} class="mx-auto mb-4" />
              <p class="text-xs font-black uppercase tracking-widest">{t("frontend/src/routes/submissions/[id]/+page.svelte::no_llm_data_yet_message")}</p>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}

<dialog bind:this={fileDialog} class="modal">
  <div class="modal-box w-11/12 max-w-5xl">
    <div class="flex items-center justify-between mb-2">
      <div class="font-medium">
        {t(
          "frontend/src/routes/submissions/[id]/+page.svelte::files_dialog_title",
        )}
      </div>
      <button class="btn btn-sm btn-primary" on:click={downloadFiles}
        >{t(
          "frontend/src/routes/submissions/[id]/+page.svelte::download_button",
        )}</button
      >
    </div>
    {#if files.length}
      <div class="flex flex-col md:flex-row gap-4">
        <div class="md:w-60">
          <FileTree nodes={tree} select={chooseFile} />
        </div>
        <div class="flex-1">
          <div class="font-mono text-sm mb-2">{selected?.name}</div>
          <pre class="whitespace-pre bg-base-200 p-2 rounded"><code class="hljs"
              >{@html highlighted}</code
            ></pre>
        </div>
      </div>
    {:else}
      <pre class="whitespace-pre bg-base-200 p-2 rounded"><code class="hljs"
          >{@html highlighted}</code
        ></pre>
    {/if}
  </div>
  <form method="dialog" class="modal-backdrop">
    <button
      >{t(
        "frontend/src/routes/submissions/[id]/+page.svelte::close_button",
      )}</button
    >
  </form>
</dialog>

{#if err}<p style="color:red">{err}</p>{/if}

<style>
  pre {
    background: #eee;
    padding: 0.5rem;
    overflow: auto;
  }
  .hljs {
    background: transparent;
  }
  :global(body.scratch-fullscreen-active) {
    overflow: hidden;
  }
  .scratch-fullscreen-shell {
    position: fixed;
    inset: 0;
    z-index: 60;
    background: hsl(var(--b2));
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    overflow: hidden;
  }
  .scratch-fullscreen-toolbar {
    flex: 0 0 auto;
  }
  .scratch-fullscreen-grid {
    display: grid;
    gap: 1rem;
    flex: 1 1 auto;
    min-height: 0;
  }
  .scratch-fullscreen-grid-player {
    grid-template-columns: minmax(0, 1fr);
  }
  .scratch-fullscreen-grid-both {
    grid-template-columns: minmax(0, 1fr);
  }
  .scratch-fullscreen-card {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .scratch-fullscreen-body {
    flex: 1 1 auto;
    min-height: 0;
  }
  .scratch-fullscreen-body--blocks {
    overflow: hidden;
  }
  @media (min-width: 768px) {
    .scratch-fullscreen-shell {
      padding: 1.5rem;
    }
  }
  @media (min-width: 1024px) {
    .scratch-fullscreen-grid-both {
      grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.2fr);
    }
  }
</style>

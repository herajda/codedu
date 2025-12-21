<script lang="ts">
  // @ts-nocheck
  import { onMount } from "svelte";
  import { slide } from "svelte/transition";

  import { page } from "$app/stores";
  import { apiFetch, apiJSON } from "$lib/api";
  import { auth } from "$lib/auth";
  import {
    extractMethodFromUnittest,
    leadingIndent,
    parseUnitTestQualifiedName,
  } from "$lib/unittests";
  import CodeMirror from "$lib/components/ui/CodeMirror.svelte";
  import { python } from "@codemirror/lang-python";
  import { bannedCatalog, type CatalogFunction } from "$lib/bannedCatalog";
  import TestFileManager from "$lib/components/TestFileManager.svelte";
  import {
    Plus,
    Save,
    Trash2,
    Eye,
    FlaskConical,
    FileUp,
    Code,
    Copy,
    Clock,
    Scale,
    Shield,
    FileCode2,
    Upload as UploadIcon,
    ChevronRight,
    Terminal,
    ArrowRight,
    Cpu,
    Check,
    RotateCw,
    ShieldAlert,
    Sparkles,
    Wand2,
  } from "lucide-svelte";
  import {
    readFileBase64,
    readFileText,
    textToBase64,
  } from "$lib/utils/testFiles";
  import ConfirmModal from "$lib/components/ConfirmModal.svelte";
  import { strictnessGuidance } from "$lib/llmStrictness";
  import { t, translator } from "$lib/i18n";

  $: id = $page.params.id;
  $: role = ($auth as any)?.role ?? "";
  let translate = t;
  $: translate = $translator;

  let assignment: any = null;
  let tests: any[] = [];
  let err = "";
  let confirmModal: InstanceType<typeof ConfirmModal>;

  // LLM testing settings (moved from Edit page)
  let llmFeedback = false;
  let llmAutoAward = true;
  let llmScenarios = "";
  let llmStrictness = 50;
  let llmRubric = "";
  $: llmStrictnessMessage = strictnessGuidance(llmStrictness);
  const exampleScenario =
    '[{"name":"calc","steps":[{"send":"2 + 2","expect_after":"4"}]}]';

  // local inputs for creating/uploading tests
  let tStdin = "";
  let tStdout = "";
  let tLimit = "";
  let tWeight = "1";
  let unittestFile: File | null = null;
  let ioFileName = "";
  let ioFileText = "";
  let showIOFile = false;
  
  type AttachedFile = { name: string; content: string };
  let ioFiles: AttachedFile[] = [];
  let ioSelectedIndex = -1;


  type ToolMode = "structured" | "advanced";
  type StructuredToolRule = { library: string; function: string; note: string };

  let toolMode: ToolMode = "structured";
  let structuredRules: StructuredToolRule[] = [];
  let draftLibrary = bannedCatalog[0]?.library ?? "builtins";
  let draftFunction = "";
  let draftNote = "";
  let advancedPatternsText = "";
  let bannedSaving = false;
  let bannedSaved = false;

  // function-test inputs removed

  // ──────────────────────────────────────────────────────
  // AI generator state
  // ──────────────────────────────────────────────────────
  let aiNumTests = "5";
  let aiInstructions = "";
  let aiGenerating = false;
  let aiCode = "";
  let hasAIBuilder = false;
  let hasFunctionBuilder = false; // deprecated
  let aiCallMode: "stdin" | "function" = "stdin";
  let aiDifficulty: "simple" | "hard" = "simple";
  let aiSolutionFile: File | null = null;
  let aiSolutionText = "";
  let aiSolutionError = "";
  let aiAuto = true;
  // Teacher solution testing
  let teacherSolutionFile: File | null = null;
  let teacherRun: any = null;
  let teacherRunLoading = false;
  type OutputMode = "manual" | "teacher";
  let ioOutputMode: OutputMode = "manual";
  let utOutputMode: OutputMode = "manual";
  let fnOutputMode: OutputMode = "manual";
  let ioOutputLoading = false;
  let utOutputLoading = false;
  let fnOutputLoading = false;

  // ──────────────────────────────────────────────────────
  // Unittest Builder state
  // ──────────────────────────────────────────────────────
  type UTAssertKind =
    | "equals"
    | "notEquals"
    | "contains"
    | "notContains"
    | "regex"
    | "raises"
    | "custom";
  type UTAssertion =
    | {
        kind: "equals" | "notEquals" | "contains" | "notContains";
        args: string[];
        expected: string;
      }
    | { kind: "regex"; args: string[]; pattern: string }
    | { kind: "raises"; args: string[]; exception: string }
    | { kind: "custom"; code: string };

  type UTTest = {
    name: string;
    description?: string;
    weight: string;
    timeLimit: string;
    callMode: "stdin" | "function";
    functionName: string;
    assertions: UTAssertion[];
    fileName: string;
    fileText: string;
    fileBase64: string;
    files: AttachedFile[];
    selectedFileIndex: number;
    showFile?: boolean;
  };

  let utClassName = "TestAssignment";
  let utSetup = "";
  let utTeardown = "";
  let utTests: UTTest[] = [];
  let utShowPreview = false;
  let utPreviewCode = "";
  let showAdvanced = false;
  let copiedPreview = false;

  type FnParameter = { name: string; type?: string };
  type FnReturn = { name: string; type?: string };
  type FnKwarg = { key: string; value: string };
  type FnCase = {
    name: string;
    args: string[];
    kwargs: FnKwarg[];
    returns: string[];
    weight: string;
    timeLimit: string;
    fileName: string;
    fileText: string;
    fileBase64: string;
    files: AttachedFile[];
    selectedFileIndex: number;
    showFile?: boolean;
  };
  type FnMeta = {
    name: string;
    params: FnParameter[];
    returns: FnReturn[];
    kwargs?: FnParameter | null;
  };

  let builderMode: "unittest" | "function" = "unittest";
  let fnSignature = "";
  let fnSignatureError = "";
  let fnMeta: FnMeta | null = null;
  let fnCases: FnCase[] = [];

  type GeneratedStateSnapshot = {
    aiCode: string;
    utClassName: string;
    utSetup: string;
    utTeardown: string;
    utTests: UTTest[];
    hasAIBuilder: boolean;
    builderMode: typeof builderMode;
    utPreviewCode: string;
    utShowPreview: boolean;
    showAdvanced: boolean;
  };

  // ──────────────────────────────────────────────────────
  // Unittest per-method view/edit helpers
  // ──────────────────────────────────────────────────────
  let editDialog: HTMLDialogElement;
  let editingTest: any = null;
  let editingMethodCode = "";
  let saveModal: HTMLDialogElement;
  let undoPending: {
    testIds: string[];
    snapshot: GeneratedStateSnapshot | null;
  } | null = null;
  let undoLoading = false;

  function normalizeIndent(block: string): { lines: string[]; base: number } {
    const raw = String(block || "").replace(/\r\n?/g, "\n");
    const lines = raw.split("\n");
    let base = Infinity;
    for (const l of lines) {
      if (l.trim() === "") continue;
      const ind = leadingIndent(l).length;
      base = Math.min(base, ind);
    }
    if (!isFinite(base)) base = 0;
    const out = lines.map((l) => (l.length >= base ? l.slice(base) : l));
    return { lines: out, base };
  }

  function buildFilePayload(
    fileName: string,
    fileText: string,
    fileBase64: string,
  ): { file_name: string; file_base64: string } | null {
    const name = String(fileName ?? "").trim();
    const hasBase64 = typeof fileBase64 === "string" && fileBase64.trim() !== "";
    if (hasBase64) {
      if (!name) {
        throw new Error(
          translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::file_name_required",
          ),
        );
      }
      return { file_name: name, file_base64: fileBase64 };
    }
    const text = String(fileText ?? "");
    if (!name && !text) return null;
    if (!name) {
      throw new Error(
        translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::file_name_required",
        ),
      );
    }
    return { file_name: name, file_base64: textToBase64(text) };
  }

  function buildExistingTestFilePayload(
    t: any,
  ): { file_name: string; file_base64: string } | null {
    if (t?.file_create_dirty) {
      return buildFilePayload(
        String(t?.file_create_name ?? ""),
        String(t?.file_create_text ?? ""),
        "",
      );
    }
    return buildFilePayload(
      String(t?.file_name ?? ""),
      "",
      typeof t?.file_base64 === "string" ? t.file_base64 : "",
    );
  }

  function clearTestFile(t: any) {
    t.file_name = "";
    t.file_base64 = "";
    t.file_create_name = "";
    t.file_create_text = "";
    t.file_create_dirty = false;
    tests = [...tests];
  }

  function replaceMethodInUnittest(
    src: string,
    qn: string,
    newMethod: string,
  ): string {
    const { cls, method } = parseUnitTestQualifiedName(qn);
    const lines = String(src || "").split("\n");
    const escapedClass = cls.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const escapedMethod = method.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const classRE = new RegExp(
      `^([\\t ]*)class\\s+${escapedClass}\\s*\\(.*unittest\\.TestCase.*\\):`,
    );
    const methodRE = new RegExp(`^([\\t ]*)def\\s+${escapedMethod}\\s*\\(`);
    let classIdx = -1;
    let classIndent = "";
    for (let i = 0; i < lines.length; i++) {
      const m = lines[i].match(classRE);
      if (m) {
        classIdx = i;
        classIndent = m[1] || "";
        break;
      }
    }
    if (classIdx === -1) return src;
    let start = -1;
    let startIndent = "";
    for (let i = classIdx + 1; i < lines.length; i++) {
      const l = lines[i];
      if (l.trim() === "") continue;
      if (
        !l.startsWith(classIndent) &&
        leadingIndent(l).length <= classIndent.length
      )
        break;
      const m = l.match(methodRE);
      if (m) {
        start = i;
        startIndent = m[1] || "";
        while (
          start - 1 > classIdx &&
          lines[start - 1].trim().startsWith("@") &&
          leadingIndent(lines[start - 1]) === startIndent
        ) {
          start--;
        }
        break;
      }
    }
    if (start === -1) return src;
    let end = lines.length;
    for (let i = start + 1; i < lines.length; i++) {
      const l = lines[i];
      if (l.trim() === "") continue;
      const ind = leadingIndent(l);
      const t = l.trimStart();
      if (
        ind.length <= startIndent.length &&
        (t.startsWith("def ") || t.startsWith("class "))
      ) {
        end = i;
        break;
      }
    }
    const { lines: newLines } = normalizeIndent(newMethod);
    const pad = startIndent;
    const adjusted = newLines.map((l) =>
      l.trim() === "" ? l.trimEnd() : pad + l,
    );
    const replaced = [
      ...lines.slice(0, start),
      ...adjusted,
      ...lines.slice(end),
    ];
    return replaced.join("\n");
  }

  function openEditUnitTest(t: any) {
    editingTest = t;
    editingMethodCode = extractMethodFromUnittest(
      String(t.unittest_code || ""),
      String(t.unittest_name || ""),
    );
    editDialog?.showModal();
  }

  function closeEditUnitTest() {
    editDialog?.close();
    editingTest = null;
    editingMethodCode = "";
  }

  async function saveEditUnitTest() {
    if (!editingTest) return;
    try {
      const before = String(editingTest.unittest_code || "");
      const qn = String(editingTest.unittest_name || "");
      const updated = replaceMethodInUnittest(before, qn, editingMethodCode);
      let newQN = qn;
      const m = editingMethodCode.match(/def\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(/);
      if (m) {
        const { cls } = parseUnitTestQualifiedName(qn);
        newQN = `${cls}.${m[1]}`;
      }
      const testData: any = {
        stdin: editingTest.stdin ?? "",
        expected_stdout: editingTest.expected_stdout ?? "",
        time_limit_sec: parseFloat(editingTest.time_limit_sec) || 1,
        unittest_code: updated,
        unittest_name: newQN,
      };

      // Only include weight for weighted assignments
      if (assignment?.grading_policy === "weighted") {
        testData.weight = parseFloat(editingTest.weight) || 1;
      }

      await apiFetch(`/api/tests/${editingTest.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(testData),
      });
      closeEditUnitTest();
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }

  function getInputs(a: UTAssertion): string {
    return (a as any).args ? ((a as any).args as string[]).join("\n") : "";
  }
  function setInputs(a: UTAssertion, v: string) {
    if ((a as any).args) (a as any).args = v.split("\n");
    utTests = [...utTests];
  }
  function getExpected(a: UTAssertion): string {
    return (a as any).expected ?? "";
  }
  function setExpected(a: UTAssertion, v: string) {
    (a as any).expected = v;
    utTests = [...utTests];
  }
  function getPattern(a: UTAssertion): string {
    return (a as any).pattern ?? "";
  }
  function setPattern(a: UTAssertion, v: string) {
    (a as any).pattern = v;
    utTests = [...utTests];
  }
  function getException(a: UTAssertion): string {
    return (a as any).exception ?? "Exception";
  }
  function setException(a: UTAssertion, v: string) {
    (a as any).exception = v;
    utTests = [...utTests];
  }
  function getCustom(a: UTAssertion): string {
    return (a as any).code ?? "";
  }
  function setCustom(a: UTAssertion, v: string) {
    (a as any).code = v;
    utTests = [...utTests];
  }

  const libraryLabelMap = new Map(
    bannedCatalog.map((entry) => [entry.library, entry.label]),
  );
  let functionOptions: CatalogFunction[] = bannedCatalog[0]?.functions ?? [];
  $: functionOptions =
    bannedCatalog.find((entry) => entry.library === draftLibrary)?.functions ??
    [];

  function libraryLabel(id: string): string {
    return libraryLabelMap.get(id) ?? id;
  }

  function resetDraftRule() {
    draftFunction = "";
    draftNote = "";
  }

  function addStructuredRule() {
    const library = (draftLibrary || "").trim().toLowerCase();
    if (!library) return;
    let fn = (draftFunction || "").trim().toLowerCase();
    if (fn === "" || fn === "*") {
      fn = "*";
    }
    const note = draftNote.trim();
    const existingIndex = structuredRules.findIndex(
      (rule) => rule.library === library && rule.function === fn,
    );
    if (existingIndex >= 0) {
      structuredRules = structuredRules.map((rule, idx) =>
        idx === existingIndex ? { ...rule, note } : rule,
      );
    } else {
      structuredRules = [...structuredRules, { library, function: fn, note }];
    }
    bannedSaved = false;
    resetDraftRule();
  }

  function removeStructuredRule(index: number) {
    structuredRules = structuredRules.filter((_, i) => i !== index);
    bannedSaved = false;
  }

  function updateStructuredNote(index: number, note: string) {
    structuredRules = structuredRules.map((rule, idx) =>
      idx === index ? { ...rule, note } : rule,
    );
    bannedSaved = false;
  }

  function structuredRuleDisplay(rule: StructuredToolRule): string {
    return `${rule.library}.${rule.function === "*" ? "*" : rule.function}`;
  }

  function parseAdvancedPatterns(raw: string): string[] {
    return raw
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line.length > 0);
  }

  function applyBannedConfig(source: any) {
    let mode: ToolMode = "structured";
    let nextStructured: StructuredToolRule[] = [];
    let nextAdvanced = "";

    const raw =
      typeof source?.banned_tool_rules === "string"
        ? source.banned_tool_rules
        : "";
    if (raw.trim()) {
      try {
        const parsed = JSON.parse(raw);
        if (parsed && typeof parsed === "object") {
          if (Array.isArray(parsed.structured)) {
            nextStructured = parsed.structured.map((rule: any) => ({
              library:
                String(rule?.library || "")
                  .trim()
                  .toLowerCase() || "builtins",
              function:
                String(rule?.function || "")
                  .trim()
                  .toLowerCase() || "*",
              note: String(rule?.note || "").trim(),
            }));
          }
          if (Array.isArray(parsed.advanced)) {
            nextAdvanced = parsed.advanced.join("\n");
          }
          if (parsed.mode === "advanced") {
            mode = "advanced";
          } else if (parsed.mode === "structured") {
            mode = "structured";
          }
        }
      } catch {
        // ignore malformed JSON and fall back to legacy arrays
      }
    }

    if (!nextStructured.length && !nextAdvanced) {
      const fallbackFunctions = Array.isArray(source?.banned_functions)
        ? source.banned_functions
        : [];
      const fallbackModules = Array.isArray(source?.banned_modules)
        ? source.banned_modules
        : [];
      const combined: string[] = [];
      for (const fn of fallbackFunctions) {
        const trimmed = String(fn || "").trim();
        if (trimmed) combined.push(trimmed.toLowerCase());
      }
      for (const mod of fallbackModules) {
        const trimmed = String(mod || "").trim();
        if (!trimmed) continue;
        combined.push(
          trimmed.includes("*")
            ? trimmed.toLowerCase()
            : `${trimmed.toLowerCase()}.*`,
        );
      }
      if (combined.length) {
        nextAdvanced = combined.join("\n");
        mode = "advanced";
      }
    }

    structuredRules = nextStructured;
    advancedPatternsText = nextAdvanced;
    toolMode = nextStructured.length ? mode : nextAdvanced ? "advanced" : mode;
    draftLibrary = structuredRules.length
      ? structuredRules[structuredRules.length - 1].library
      : (bannedCatalog[0]?.library ?? "builtins");
    resetDraftRule();
  }

  // Auto-generate preview code reactively when inputs change
  $: {
    utClassName;
    utSetup;
    utTeardown;
    utTests;
    const nextCode = generateUnittestCode();
    if (nextCode !== utPreviewCode) {
      utPreviewCode = nextCode;
    }
  }

  async function load() {
    err = "";
    try {
      const data = await apiJSON(`/api/assignments/${id}`);
      assignment = data.assignment;
      applyBannedConfig(assignment);
      bannedSaved = false;
      tests = (data.tests ?? []).map((t: any) => ({
        ...t,
        files: t.files_json
          ? JSON.parse(t.files_json)
          : t.file_name
          ? [{ name: t.file_name, content: t.file_base64 }]
          : [],
        file_create_name: "",
        file_create_text: "",
        file_create_dirty: false,
      }));
      // init llm state
      llmFeedback = !!assignment.llm_feedback;
      llmAutoAward = assignment.llm_auto_award ?? true;
      llmScenarios = assignment.llm_scenarios_json ?? "";
      llmStrictness =
        typeof assignment.llm_strictness === "number"
          ? assignment.llm_strictness
          : 50;
      llmRubric = assignment.llm_rubric ?? "";
    } catch (e: any) {
      err = e.message;
    }
  }

  async function saveBannedTools() {
    bannedSaving = true;
    bannedSaved = false;
    err = "";
    try {
      const payload = {
        mode: toolMode,
        structured:
          toolMode === "advanced"
            ? []
            : structuredRules.map((rule) => ({
                ...rule,
                note: rule.note.trim(),
              })),
        advanced:
          toolMode === "structured"
            ? []
            : parseAdvancedPatterns(advancedPatternsText),
      };
      const res = await apiJSON(`/api/assignments/${id}/testing-constraints`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (res?.assignment) {
        assignment = res.assignment;
        applyBannedConfig(assignment);
      }
      bannedSaved = true;
    } catch (e: any) {
      err = e.message;
    } finally {
      bannedSaving = false;
    }
  }

  onMount(async () => {
    await load();
  });

  async function addTest() {
    try {
      const filePayload =
        ioFiles.length === 0
          ? buildFilePayload(ioFileName, ioFileText, "")
          : null;
      let expectedStdout = tStdout;
      if (ioOutputMode === "teacher") {
        ioOutputLoading = true;
        const timeLimit = parseFloat(tLimit);
        const previewPayload: any = {
          execution_mode: "stdin_stdout",
          stdin: tStdin,
          expected_stdout: "",
          time_limit_sec: Number.isFinite(timeLimit) ? timeLimit : undefined,
          ...(filePayload ?? {}),
        };
        if (ioFiles.length > 0) {
          previewPayload.files = ioFiles;
        }
        const { previews } = await runTeacherPreview([previewPayload]);
        const previewResult = previews[0];
        if (!previewResult) {
          throw new Error(
            translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_solution_no_preview",
            ),
          );
        }
        expectedStdout = String(previewResult.actual_stdout ?? "");
        tStdout = expectedStdout;
      }
      const testData: any = {
        execution_mode: "stdin_stdout",
        stdin: tStdin,
        expected_stdout: expectedStdout,
        time_limit_sec: parseFloat(tLimit) || undefined,
      };
      if (filePayload) {
        testData.file_name = filePayload.file_name;
        testData.file_base64 = filePayload.file_base64;
      }
      if (ioFiles.length > 0) {
        testData.files = ioFiles;
      }

      // Only include weight for weighted assignments
      if (assignment?.grading_policy === "weighted") {
        testData.weight = parseFloat(tWeight) || 1;
      }

      await apiFetch(`/api/assignments/${id}/tests`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(testData),
      });
      tStdin = tStdout = tLimit = "";
      tWeight = "1";
      ioFileName = "";
      ioFileText = "";
      ioFiles = [];
      await load();
    } catch (e: any) {
      err = e.message;
    } finally {
      ioOutputLoading = false;
    }
  }

  // addFunctionTest removed (deprecated)

  async function uploadUnitTests() {
    if (!unittestFile) return;
    const fd = new FormData();
    fd.append("file", unittestFile);
    try {
      await apiFetch(`/api/assignments/${id}/tests/upload`, {
        method: "POST",
        body: fd,
      });
      unittestFile = null;
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }

  // ──────────────────────────────────────────────────────
  // Enhanced: Upload generated unittest code + adjust weights/limits
  // ──────────────────────────────────────────────────────
  function addUTTest() {
    utTests = [
      ...utTests,
      {
        name: `test_${utTests.length + 1}`,
        description: "",
        weight: assignment?.grading_policy === "weighted" ? "1" : "0",
        timeLimit: "1",
        callMode: "stdin",
        functionName: "",
        assertions: [],
        fileName: "",
        fileText: "",

        fileBase64: "",
        files: [],
        selectedFileIndex: -1,
        showFile: false,
      },
    ];
  }

  function removeUTTest(idx: number) {
    utTests = utTests.filter((_, i) => i !== idx);
  }

  function addUTAssertion(idx: number, kind: UTAssertKind) {
    const t = utTests[idx];
    let a: UTAssertion;
    switch (kind) {
      case "equals":
      case "notEquals":
      case "contains":
      case "notContains":
        a = { kind, args: [""], expected: "" };
        break;
      case "regex":
        a = { kind, args: [""], pattern: "" };
        break;
      case "raises":
        a = { kind, args: [""], exception: "Exception" };
        break;
      default:
        a = { kind: "custom", code: "" };
    }
    t.assertions = [...t.assertions, a];
    utTests = [...utTests];
  }

  function removeUTAssertion(ti: number, ai: number) {
    utTests[ti].assertions = utTests[ti].assertions.filter((_, i) => i !== ai);
    utTests = [...utTests];
  }

  const headerColonRegex = /:\s*(?:->|$)/;

  function hasHeaderColon(text: string): boolean {
    return headerColonRegex.test(text);
  }

  function splitTopLevel(input: string, separator = ","): string[] {
    const parts: string[] = [];
    let current = "";
    let depthParen = 0;
    let depthBracket = 0;
    let depthBrace = 0;
    let depthAngle = 0;
    let inString: string | null = null;
    for (let i = 0; i < input.length; i++) {
      const ch = input[i];
      if (inString) {
        current += ch;
        if (ch === inString && input[i - 1] !== "\\") {
          inString = null;
        }
        continue;
      }
      if (ch === '"' || ch === "'") {
        inString = ch;
        current += ch;
        continue;
      }
      switch (ch) {
        case "(":
          depthParen++;
          break;
        case ")":
          if (depthParen > 0) depthParen--;
          break;
        case "[":
          depthBracket++;
          break;
        case "]":
          if (depthBracket > 0) depthBracket--;
          break;
        case "{":
          depthBrace++;
          break;
        case "}":
          if (depthBrace > 0) depthBrace--;
          break;
        case "<":
          depthAngle++;
          break;
        case ">":
          if (depthAngle > 0) depthAngle--;
          break;
        default:
          break;
      }
      if (
        ch === separator &&
        depthParen === 0 &&
        depthBracket === 0 &&
        depthBrace === 0 &&
        depthAngle === 0
      ) {
        const trimmed = current.trim();
        if (trimmed) parts.push(trimmed);
        current = "";
        continue;
      }
      current += ch;
    }
    const last = current.trim();
    if (last) parts.push(last);
    return parts;
  }

  function stripInlineComment(line: string): string {
    let inString: string | null = null;
    let result = "";
    for (let i = 0; i < line.length; i++) {
      const ch = line[i];
      if (inString) {
        result += ch;
        if (ch === inString && line[i - 1] !== "\\") {
          inString = null;
        }
        continue;
      }
      if (ch === '"' || ch === "'") {
        inString = ch;
        result += ch;
        continue;
      }
      if (ch === "#") {
        break;
      }
      result += ch;
    }
    return result.trimEnd();
  }

  function parseFunctionSignatureBlock(block: string): {
    meta: FnMeta | null;
    error: string;
  } {
    const raw = String(block || "").replace(/\r\n?/g, "\n");
    const lines = raw
      .split("\n")
      .map((l) => l.trim())
      .filter((l) => l);
    if (lines.length === 0) {
      return { meta: null, error: "" };
    }
    const defIdx = lines.findIndex(
      (l) => l.startsWith("def ") || l.startsWith("async def "),
    );
    if (defIdx === -1) {
      return {
        meta: null,
        error: translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_python_function_definition",
        ),
      };
    }
    let signature = lines[defIdx];
    let depth =
      (signature.match(/\(/g) || []).length -
      (signature.match(/\)/g) || []).length;
    let cursor = defIdx + 1;
    while (cursor < lines.length && (depth > 0 || !hasHeaderColon(signature))) {
      const part = lines[cursor];
      signature += " " + part;
      depth +=
        (part.match(/\(/g) || []).length - (part.match(/\)/g) || []).length;
      cursor++;
    }
    signature = stripInlineComment(signature);
    const colonIndex = signature.lastIndexOf(":");
    if (colonIndex === -1) {
      if (typeof window !== "undefined") {
        console.warn("Function builder: missing colon in signature", {
          signature,
        });
      }
      return {
        meta: null,
        error: translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::could_not_parse_function_definition",
        ),
      };
    }
    let header = signature.slice(0, colonIndex).trim();
    if (header.startsWith("async ")) {
      header = header.slice("async ".length).trimStart();
    }
    if (!header.startsWith("def ")) {
      if (typeof window !== "undefined") {
        console.warn("Function builder: header does not start with def", {
          header,
        });
      }
      return {
        meta: null,
        error: translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::could_not_parse_function_definition",
        ),
      };
    }
    const nameStart = header.indexOf("def ") + 4;
    const openIdx = header.indexOf("(", nameStart);
    const name = header.slice(nameStart, openIdx).trim();
    if (!name || !/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(name)) {
      if (typeof window !== "undefined") {
        console.warn("Function builder: failed to extract function name", {
          header,
          name,
        });
      }
      return {
        meta: null,
        error: translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::could_not_parse_function_definition",
        ),
      };
    }
    const closeIdx = header.lastIndexOf(")");
    if (openIdx === -1 || closeIdx === -1 || closeIdx < openIdx) {
      if (typeof window !== "undefined") {
        console.warn("Function builder: could not find parentheses", {
          header,
        });
      }
      return {
        meta: null,
        error: translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::could_not_parse_function_definition",
        ),
      };
    }
    const paramsRaw = header.slice(openIdx + 1, closeIdx);
    const afterParams = header.slice(closeIdx + 1).trim();
    let returnRaw: string | undefined;
    if (!afterParams) {
      returnRaw = "";
    } else if (afterParams.startsWith("->")) {
      returnRaw = afterParams.slice(2).trim();
    } else {
      if (typeof window !== "undefined") {
        console.warn(
          "Function builder: trailing text after params did not start with arrow",
          { afterParams },
        );
      }
      return {
        meta: null,
        error: translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::could_not_parse_function_definition",
        ),
      };
    }
    const params: FnParameter[] = [];
    let kwargs: FnParameter | null = null;
    if (paramsRaw.trim()) {
      const pieces = splitTopLevel(paramsRaw);
      for (const piece of pieces) {
        const part = piece.trim();
        if (!part) continue;
        if (part.startsWith("*")) {
          if (part.startsWith("**")) {
            const kwargRaw = part.slice(2).trim();
            if (!kwargRaw || kwargs) {
              return {
                meta: null,
                error: translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::could_not_parse_function_definition",
                ),
              };
            }
            const [namePartRaw, typePartRaw] = kwargRaw
              .split(":")
              .map((p) => p.trim());
            const namePart = namePartRaw.split("=")[0].trim();
            if (!namePart || !/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(namePart)) {
              return {
                meta: null,
                error: translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::argument_is_not_valid_positional_parameter",
                  { namePartRaw },
                ),
              };
            }
            const typePart = typePartRaw ? typePartRaw.trim() : undefined;
            kwargs = { name: namePart, type: typePart };
            continue;
          }
          return {
            meta: null,
            error: translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::only_positional_arguments_supported",
            ),
          };
        }
        const [namePartRaw, typePartRaw] = part.split(":").map((p) => p.trim());
        const namePart = namePartRaw.split("=")[0].trim();
        if (!namePart || !/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(namePart)) {
          return {
            meta: null,
            error: translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::argument_is_not_valid_positional_parameter",
              { namePartRaw },
            ),
          };
        }
        const typePart = typePartRaw ? typePartRaw.trim() : undefined;
        params.push({ name: namePart, type: typePart });
      }
    }
    let returns: FnReturn[] = [];
    const ret = (returnRaw ?? "").trim();
    if (!ret) {
      returns = [{ name: "return", type: undefined }];
    } else if (/^none$/i.test(ret)) {
      returns = [];
    } else if (ret.startsWith("(") && ret.endsWith(")")) {
      const inner = ret.slice(1, -1);
      const parts = splitTopLevel(inner);
      if (parts.length === 0) {
        returns = [];
      } else {
        returns = parts.map((p, idx) => ({ name: `value${idx + 1}`, type: p }));
      }
    } else {
      returns = [{ name: "return", type: ret }];
    }
    return { meta: { name, params, returns, kwargs }, error: "" };
  }

  function describeTypeControl(type?: string): {
    control: "text" | "textarea" | "number" | "integer" | "boolean";
    placeholder?: string;
  } {
    const t = String(type || "").toLowerCase();
    if (!t) return { control: "text" };
    if (/(bool|boolean)/.test(t)) return { control: "boolean" };
    if (/(int|integer|long)/.test(t)) return { control: "integer" };
    if (/(float|double|decimal)/.test(t)) return { control: "number" };
    if (/(list|tuple|set|dict|map|array|json|sequence)/.test(t))
      return { control: "textarea" };
    if (/(str|string|char)/.test(t)) return { control: "text" };
    return { control: "text" };
  }

  function ensureCaseShape(c: FnCase, meta: FnMeta | null): FnCase {
    const argCount = meta?.params.length ?? 0;
    const returnCount = meta ? meta.returns.length : 1;
    const adjustedArgs = [...(c.args ?? [])];
    const adjustedReturns = [...(c.returns ?? [])];
    const adjustedKwargs = Array.isArray(c.kwargs) ? [...c.kwargs] : [];
    while (adjustedArgs.length < argCount) adjustedArgs.push("");
    if (adjustedArgs.length > argCount) adjustedArgs.length = argCount;
    const expectedReturnCount =
      returnCount === 0 ? 0 : Math.max(returnCount, 1);
    while (adjustedReturns.length < expectedReturnCount)
      adjustedReturns.push("");
    if (adjustedReturns.length > expectedReturnCount)
      adjustedReturns.length = expectedReturnCount;
    return {
      name: c.name,
      args: adjustedArgs,
      kwargs: meta?.kwargs ? adjustedKwargs : [],
      returns: adjustedReturns,
      weight: c.weight,
      timeLimit: c.timeLimit,
      fileName: c.fileName ?? "",
      fileText: c.fileText ?? "",
      fileBase64: c.fileBase64 ?? "",
      files: Array.isArray(c.files) ? c.files : [],
      selectedFileIndex:
        typeof c.selectedFileIndex === "number" ? c.selectedFileIndex : -1,
      showFile: c.showFile ?? false,
    };
  }

  function arraysEqual(a: string[], b: string[]): boolean {
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      if (a[i] !== b[i]) return false;
    }
    return true;
  }

  function kwargsEqual(a: FnKwarg[], b: FnKwarg[]): boolean {
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      if (a[i].key !== b[i].key || a[i].value !== b[i].value) return false;
    }
    return true;
  }

  function casesEqual(a: FnCase, b: FnCase): boolean {
    return (
      a.name === b.name &&
      arraysEqual(a.args, b.args) &&
      kwargsEqual(a.kwargs ?? [], b.kwargs ?? []) &&
      arraysEqual(a.returns, b.returns) &&
      a.weight === b.weight &&
      a.timeLimit === b.timeLimit &&
      a.fileName === b.fileName &&
      a.fileText === b.fileText &&
      a.fileBase64 === b.fileBase64
    );
  }

  function createEmptyCase(
    meta: FnMeta | null,
    idx: number,
    defaultWeight: string,
  ): FnCase {
    const argCount = meta?.params.length ?? 0;
    const returnCount = meta ? meta.returns.length : 1;
    const args = Array.from({ length: argCount }, () => "");
    const returns = Array.from(
      { length: returnCount === 0 ? 0 : Math.max(returnCount, 1) },
      () => "",
    );
    return {
      name: translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::case_n_placeholder",
        { fi: idx + 1 },
      ),
      args,
      kwargs: [],
      returns,
      weight: defaultWeight,
      timeLimit: "1",
      fileName: "",
      fileText: "",
      fileBase64: "",
      files: [],
      selectedFileIndex: -1,
      showFile: false,
    };
  }

  function parseJSONish(value: string): any {
    const text = value.trim();
    if (!text) return null;
    const normalized = text
      .replace(/'/g, '"')
      .replace(/\bTrue\b/g, "true")
      .replace(/\bFalse\b/g, "false")
      .replace(/\bNone\b/g, "null");
    try {
      return JSON.parse(normalized);
    } catch (err) {
      return undefined;
    }
  }

  function coerceValueForType(raw: string, typeHint?: string): any {
    const value = raw.trim();
    if (!value) return null;
    const hint = String(typeHint || "").toLowerCase();
    if (/(bool|boolean)/.test(hint)) {
      if (/^(true|false)$/i.test(value)) return /^true$/i.test(value);
      if (/^[01]$/.test(value)) return value === "1";
    }
    if (/(int|integer|long)/.test(hint)) {
      const parsed = parseInt(value, 10);
      if (!Number.isNaN(parsed)) return parsed;
    }
    if (/(float|double|decimal)/.test(hint)) {
      const parsed = parseFloat(value);
      if (!Number.isNaN(parsed)) return parsed;
    }
    if (/(str|string|char)/.test(hint)) {
      return value;
    }
    if (/(list|tuple|set|dict|map|array|json|sequence)/.test(hint)) {
      const parsed = parseJSONish(value);
      if (parsed !== undefined) return parsed;
    }
    if (hint && hint.includes("none")) {
      return null;
    }
    if (/^(true|false)$/i.test(value)) return /^true$/i.test(value);
    if (/^-?\d+$/.test(value)) {
      const parsed = parseInt(value, 10);
      if (!Number.isNaN(parsed)) return parsed;
    }
    if (/^-?\d*\.\d+$/.test(value)) {
      const parsed = parseFloat(value);
      if (!Number.isNaN(parsed)) return parsed;
    }
    const parsedJSON = parseJSONish(value);
    if (parsedJSON !== undefined) return parsedJSON;
    return value;
  }

  $: {
    const { meta, error } = parseFunctionSignatureBlock(fnSignature);
    fnMeta = meta;
    fnSignatureError = error;
    if (meta && fnCases.length) {
      const nextCases = fnCases.map((c, idx) =>
        ensureCaseShape(
          {
            ...c,
            name:
              c.name ||
              translate(
                "frontend/src/routes/assignments/[id]/tests/+page.svelte::case_n_placeholder",
                { fi: idx + 1 },
              ),
          },
          meta,
        ),
      );
      const changed = nextCases.some(
        (caseItem, idx) => !casesEqual(caseItem, fnCases[idx]),
      );
      if (changed) {
        fnCases = nextCases;
      }
    }
  }

  function addFnCase() {
    if (!fnMeta) {
      err = translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::define_function_signature_first",
      );
      return;
    }
    const defaultWeight = assignment?.grading_policy === "weighted" ? "1" : "0";
    fnCases = [
      ...fnCases,
      createEmptyCase(fnMeta, fnCases.length, defaultWeight),
    ];
    hasFunctionBuilder = true;
  }

  function removeFnCase(idx: number) {
    fnCases = fnCases.filter((_, i) => i !== idx);
    if (fnCases.length === 0) {
      hasFunctionBuilder = false;
    }
  }

  async function createFunctionTestsFromBuilder() {
    if (!fnMeta) {
      err = translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::define_function_signature_before_creating_tests",
      );
      return;
    }
    if (fnCases.length === 0) {
      err = translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_at_least_one_case",
      );
      return;
    }
    try {
      const filePayloads = fnCases.map((c) =>
        c.files && c.files.length > 0
          ? null
          : buildFilePayload(c.fileName, c.fileText, c.fileBase64),
      );
      let teacherExpected: string[] | null = null;
      if (fnOutputMode === "teacher") {
        fnOutputLoading = true;
        const previewPayloads = fnCases.map((c, idx) => {
          const argsValues = fnMeta.params.map((p, idx) =>
            coerceValueForType(c.args[idx] ?? "", p.type),
          );
          const kwargsObject = fnMeta.kwargs
            ? buildKwargsObject(c.kwargs)
            : {};
          const timeLimit = parseFloat(c.timeLimit);
          const previewPayload: any = {
            execution_mode: "function",
            function_name: fnMeta.name,
            function_args: JSON.stringify(argsValues),
            function_kwargs: JSON.stringify(kwargsObject),
            expected_return: "",
            time_limit_sec: Number.isFinite(timeLimit) ? timeLimit : undefined,
            ...(filePayloads[idx] ?? {}),
          };
          if (c.files && c.files.length > 0) {
            previewPayload.files = c.files;
          }
          return previewPayload;
        });
        const { previews } = await runTeacherPreview(previewPayloads);
        teacherExpected = [];
        fnCases = fnCases.map((c, idx) => {
          const preview = previews[idx];
          const normalizedReturn = normalizeTeacherReturn(
            preview?.actual_return ?? "",
          );
          teacherExpected?.push(normalizedReturn);
          const updatedReturns = convertReturnJSONToInputs(
            fnMeta,
            normalizedReturn,
          );
          return { ...c, returns: updatedReturns };
        });
      }
      for (const [idx, c] of fnCases.entries()) {
        const argsValues = fnMeta.params.map((p, idx) =>
          coerceValueForType(c.args[idx] ?? "", p.type),
        );
        const kwargsObject = fnMeta.kwargs ? buildKwargsObject(c.kwargs) : {};
        const returnValues = fnMeta.returns.length
          ? fnMeta.returns.map((r, idx) =>
              coerceValueForType(c.returns[idx] ?? "", r.type),
            )
          : [];
        let expectedPayload: any = null;
        let expectedJSON = "";
        if (teacherExpected) {
          expectedJSON = teacherExpected[idx] ?? "null";
        } else {
          if (fnMeta.returns.length === 0) {
            expectedPayload = null;
          } else if (fnMeta.returns.length === 1) {
            expectedPayload = returnValues[0];
          } else {
            expectedPayload = returnValues;
          }
          expectedJSON =
            expectedPayload === undefined
              ? ""
              : JSON.stringify(expectedPayload);
        }
        const payload: any = {
          execution_mode: "function",
          function_name: fnMeta.name,
          function_args: JSON.stringify(argsValues),
          function_kwargs: JSON.stringify(kwargsObject),
          function_arg_names: JSON.stringify(fnMeta.params.map((p) => p.name)),
          expected_return: expectedJSON,
          stdin: "",
          expected_stdout: "",
          time_limit_sec: parseFloat(c.timeLimit) || undefined,
        };
        if (c.files && c.files.length > 0) {
          payload.files = c.files;
        } else if (filePayloads[idx]) {
          payload.file_name = filePayloads[idx]?.file_name;
          payload.file_base64 = filePayloads[idx]?.file_base64;
        }
        if (assignment?.grading_policy === "weighted") {
          payload.weight = parseFloat(c.weight) || 1;
        }
        await apiFetch(`/api/assignments/${id}/tests`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });
      }
      fnCases = [];
      hasFunctionBuilder = false;
      fnSignature = "";
      fnMeta = null;
      await load();
    } catch (e: any) {
      err = e.message;
    } finally {
      fnOutputLoading = false;
    }
  }

  function pythonExpressionOrDefault(value: string, fallback = "None"): string {
    const trimmed = String(value ?? "").trim();
    if (!trimmed) return fallback;
    return trimmed;
  }

  function buildCallExpression(test: UTTest, args: string[]): string {
    if (test.callMode === "function") {
      const fn = test.functionName?.trim() || "function_name";
      const formattedArgs = args.map((s) =>
        pythonExpressionOrDefault(s, "None"),
      );
      const joined = formattedArgs.length
        ? `, ${formattedArgs.join(", ")}`
        : "";
      return `student_function(${JSON.stringify(fn)}${joined})`;
    }
    const fmtArgs = args.map((s) => JSON.stringify(s));
    return `student_code(${fmtArgs.join(", ")})`;
  }

  function formatExpected(
    test: UTTest,
    value: string,
    fallback = "None",
  ): string {
    if (test.callMode === "function") {
      return pythonExpressionOrDefault(value, fallback);
    }
    return JSON.stringify(value ?? "");
  }

  function generateUnittestCode(): string {
    const lines: string[] = [];
    lines.push("import unittest");
    lines.push("");
    lines.push(`class ${utClassName}(unittest.TestCase):`);
    if (utSetup && utSetup.trim() !== "") {
      lines.push("    def setUp(self):");
      utSetup.split("\n").forEach((l) => lines.push("        " + l));
      if (utSetup[utSetup.length - 1] !== "\n") lines.push("");
    }
    if (utTeardown && utTeardown.trim() !== "") {
      lines.push("    def tearDown(self):");
      utTeardown.split("\n").forEach((l) => lines.push("        " + l));
      if (utTeardown[utTeardown.length - 1] !== "\n") lines.push("");
    }
    for (const t of utTests) {
      const methodName = t.name.startsWith("test_") ? t.name : `test_${t.name}`;
      lines.push(`    def ${methodName}(self):`);
      if (t.description && t.description.trim() !== "") {
        lines.push(`        """${t.description}"""`);
      }
      if (t.assertions.length === 0) {
        lines.push("        self.assertTrue(True)");
        continue;
      }
      for (const a of t.assertions) {
        if (a.kind === "custom") {
          const code = (a as any).code || "";
          const cs = code.split("\n").map((l: string) => "        " + l);
          lines.push(...cs);
          continue;
        }
        const rawArgs = Array.isArray((a as any).args)
          ? (a as any).args
          : (a as any).args != null
            ? [(a as any).args]
            : [];
        const call = buildCallExpression(t, rawArgs);
        switch (a.kind) {
          case "equals":
            lines.push(
              `        self.assertEqual(${call}, ${formatExpected(t, (a as any).expected ?? "")})`,
            );
            break;
          case "notEquals":
            lines.push(
              `        self.assertNotEqual(${call}, ${formatExpected(t, (a as any).expected ?? "")})`,
            );
            break;
          case "contains":
            lines.push(
              `        self.assertIn(${formatExpected(t, (a as any).expected ?? "")}, ${call})`,
            );
            break;
          case "notContains":
            lines.push(
              `        self.assertNotIn(${formatExpected(t, (a as any).expected ?? "")}, ${call})`,
            );
            break;
          case "regex":
            lines.push(
              `        self.assertRegex(${call}, r${JSON.stringify((a as any).pattern ?? "").replace(/^"|"$/g, '"')})`,
            );
            break;
          case "raises":
            lines.push(
              `        with self.assertRaises(${(a as any).exception || "Exception"}):`,
            );
            lines.push(`            ${call}`);
            break;
        }
      }
    }
    lines.push("");
    return lines.join("\n");
  }

  function refreshPreview() {
    utPreviewCode = generateUnittestCode();
  }

  async function copyPreview() {
    try {
      await navigator.clipboard.writeText(utPreviewCode);
      copiedPreview = true;
      setTimeout(() => (copiedPreview = false), 1500);
    } catch {}
  }

  function cloneUTAssertion(a: UTAssertion): UTAssertion {
    switch (a.kind) {
      case "custom":
        return { kind: "custom", code: a.code };
      case "regex":
        return { kind: "regex", args: [...a.args], pattern: a.pattern };
      case "raises":
        return { kind: "raises", args: [...a.args], exception: a.exception };
      default:
        return { kind: a.kind, args: [...a.args], expected: a.expected };
    }
  }

  function cloneUTTests(list: UTTest[]): UTTest[] {
    return list.map((t) => ({
      ...t,
      ...t,
      assertions: t.assertions.map((a) => cloneUTAssertion(a)),
      fileName: t.fileName,
      fileText: t.fileText,
      fileBase64: t.fileBase64,
      files: Array.isArray(t.files) ? [...t.files] : [],
      selectedFileIndex:
        typeof t.selectedFileIndex === "number" ? t.selectedFileIndex : -1,
      showFile: t.showFile,
    }));
  }

  function captureGeneratedState(): GeneratedStateSnapshot | null {
    if (!aiCode.trim() && utTests.length === 0) {
      return null;
    }
    return {
      aiCode,
      utClassName,
      utSetup,
      utTeardown,
      utTests: cloneUTTests(utTests),
      hasAIBuilder,
      builderMode,
      utPreviewCode,
      utShowPreview,
      showAdvanced,
    };
  }

  function restoreGeneratedState(snapshot: GeneratedStateSnapshot | null) {
    if (!snapshot) return;
    aiCode = snapshot.aiCode;
    utClassName = snapshot.utClassName;
    utSetup = snapshot.utSetup;
    utTeardown = snapshot.utTeardown;
    utTests = cloneUTTests(snapshot.utTests);
    hasAIBuilder = snapshot.hasAIBuilder;
    builderMode = snapshot.builderMode;
    utPreviewCode = snapshot.utPreviewCode;
    utShowPreview = snapshot.utShowPreview;
    showAdvanced = snapshot.showAdvanced;
  }

  function clearGeneratedState() {
    aiCode = "";
    hasAIBuilder = false;
    utShowPreview = false;
    utClassName = "TestAssignment";
    utSetup = "";
    utTeardown = "";
    utTests = [];
    utPreviewCode = "";
    showAdvanced = false;
    copiedPreview = false;
    builderMode = "unittest";
    teacherRun = null;
    fnCases = [];
    fnMeta = null;
    fnSignature = "";
    fnSignatureError = "";
    hasFunctionBuilder = false;
  }

  function parseUnittestMethodsFromCode(src: string): string[] {
    const lines = src.split("\n");
    const classRE = /^class\s+(\w+)\s*\(.*unittest\.TestCase.*\):/;
    const methodRE = /^\s*def\s+(test_[A-Za-z0-9_]+)\s*\(/;
    const methods: string[] = [];
    let current = "";
    let indent = 0;
    for (const line of lines) {
      const classMatch = line.match(classRE);
      if (classMatch) {
        current = classMatch[1];
        indent = line.length - line.trimStart().length;
        continue;
      }
      if (!current) continue;
      if (
        line.trim() !== "" &&
        line.length - line.trimStart().length <= indent
      ) {
        current = "";
        continue;
      }
      const methodMatch = line.match(methodRE);
      if (methodMatch && current) {
        methods.push(`${current}.${methodMatch[1]}`);
      }
    }
    return methods;
  }

  function collectPreviewTestsForRun(): any[] {
    if (builderMode !== "unittest") return [];
    const builderHasTests = utTests.length > 0;
    const code = builderHasTests ? generateUnittestCode() : aiCode;
    const trimmed = String(code || "").trim();
    if (!trimmed) return [];

    if (builderHasTests) {
      const className =
        (utClassName || "TestAssignment").trim() || "TestAssignment";
      return utTests.map((t) => {
        const methodName = t.name.startsWith("test_")
          ? t.name
          : `test_${t.name}`;
        const parsedTime = Number.parseFloat(String(t.timeLimit ?? ""));
        const parsedWeight = Number.parseFloat(String(t.weight ?? ""));
        const timeLimit =
          Number.isFinite(parsedTime) && parsedTime > 0 ? parsedTime : 1;
        const useWeight =
          assignment?.grading_policy === "weighted"
            ? Number.isFinite(parsedWeight) && parsedWeight >= 0
              ? parsedWeight
              : 1
            : 1;
        return {
          execution_mode: "unittest",
          unittest_code: trimmed,
          unittest_name: `${className}.${methodName}`,
          time_limit_sec: timeLimit,
          weight: useWeight,
        };
      });
    }

    const methods = parseUnittestMethodsFromCode(trimmed);
    if (methods.length === 0) return [];
    return methods.map((name) => ({
      execution_mode: "unittest",
      unittest_code: trimmed,
      unittest_name: name,
      time_limit_sec: 1,
      weight: 1,
    }));
  }

  function clickExistingTestsTab() {
    const label = translate(
      "frontend/src/routes/assignments/[id]/tests/+page.svelte::existing_tests",
    );
    const existingTab = document.querySelector(
      `input[name="tests-tab"][aria-label="${label}"]`,
    ) as HTMLInputElement | null;
    existingTab?.click();
  }

  function clickAITestsTab() {
    const label = translate(
      "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_generate",
    );
    const aiTab = document.querySelector(
      `input[name="tests-tab"][aria-label="${label}"]`,
    ) as HTMLInputElement | null;
    aiTab?.click();
  }

  // ──────────────────────────────────────────────────────
  // AI generator actions
  // ──────────────────────────────────────────────────────
  function stripOuterQuotes(s: string): string {
    const t = s.trim();
    if (
      (t.startsWith("'") && t.endsWith("'")) ||
      (t.startsWith('"') && t.endsWith('"'))
    ) {
      return t.slice(1, -1);
    }
    return t;
  }
  function splitArgsPreserveQuoted(inner: string): string[] {
    const out: string[] = [];
    let buf = "";
    let q: string | null = null;
    for (let i = 0; i < inner.length; i++) {
      const ch = inner[i];
      if (q) {
        if (ch === q) {
          q = null;
        }
        buf += ch;
        continue;
      }
      if (ch === '"' || ch === "'") {
        q = ch;
        buf += ch;
        continue;
      }
      if (ch === ",") {
        out.push(buf.trim());
        buf = "";
        continue;
      }
      buf += ch;
    }
    if (buf.trim() !== "") out.push(buf.trim());
    return out;
  }
  function parseArgsFromMaybeStudentCode(v: any): string[] {
    const raw = String(v ?? "").trim();
    const m = raw.match(/^student_code\\s*\\(([\\s\\S]*)\\)$/);
    if (m) {
      const inner = m[1];
      return splitArgsPreserveQuoted(inner).map((p) => stripOuterQuotes(p));
    }
    return [stripOuterQuotes(raw)];
  }
  function coerceUTAssertion(a: any): UTAssertion {
    const kind = (a?.kind as UTAssertKind) || "custom";
    if (kind === "custom")
      return { kind: "custom", code: String(a?.code ?? "") };
    const rawArgs: any[] = Array.isArray(a?.args)
      ? a.args
      : a?.args != null
        ? [a.args]
        : [];
    const normalized: string[] = rawArgs.flatMap((v: any) =>
      parseArgsFromMaybeStudentCode(v),
    );
    if (kind === "regex")
      return {
        kind: "regex",
        args: normalized,
        pattern: String(a?.pattern ?? ""),
      };
    if (kind === "raises")
      return {
        kind: "raises",
        args: normalized,
        exception: String(a?.exception ?? "Exception"),
      };
    return { kind, args: normalized, expected: String(a?.expected ?? "") };
  }
  function coerceUTTest(t: any): UTTest {
    const modeRaw = String(t?.callMode ?? t?.mode ?? "").toLowerCase();
    const callMode: "stdin" | "function" =
      modeRaw === "function" ? "function" : "stdin";
    const fnName = String(t?.functionName ?? t?.function_name ?? "").trim();
    return {
      name: String(t?.name ?? "test_case"),
      description: t?.description ? String(t?.description) : "",
      weight: String(t?.weight ?? "1"),
      timeLimit: String(t?.timeLimit ?? "1"),
      callMode,
      functionName: callMode === "function" ? fnName || "function_name" : "",
      assertions: Array.isArray(t?.assertions)
        ? t.assertions.map(coerceUTAssertion)
        : [],
      fileName: String(t?.fileName ?? t?.file_name ?? ""),
      fileText: String(t?.fileText ?? t?.file_text ?? ""),
      fileBase64: String(t?.fileBase64 ?? t?.file_base64 ?? ""),
      files: Array.isArray(t?.files) ? t.files : [],
      selectedFileIndex:
        typeof t?.selectedFileIndex === "number" ? t.selectedFileIndex : -1,
      showFile: !!t?.showFile,
    };
  }

  function formatValueForInput(value: any): string {
    if (value === undefined || value === null) return "";
    if (typeof value === "string") return value;
    if (typeof value === "number" || typeof value === "boolean")
      return String(value);
    try {
      return JSON.stringify(value);
    } catch (err) {
      return String(value);
    }
  }

  function guessTypeFromValue(value: any): string {
    if (value === null || value === undefined) return "Any";
    if (Array.isArray(value)) return "list";
    if (typeof value === "object") return "dict";
    if (typeof value === "boolean") return "bool";
    if (typeof value === "number")
      return Number.isInteger(value) ? "int" : "float";
    if (typeof value === "string") return "str";
    return "Any";
  }

  function buildSignatureFromAI(name: string, sampleCase: any): string {
    const argsSample: any[] = Array.isArray(sampleCase?.args)
      ? sampleCase.args
      : sampleCase?.args != null
        ? [sampleCase.args]
        : [];
    const hasKwargs =
      sampleCase?.kwargs && typeof sampleCase.kwargs === "object";
    const expectedSample = sampleCase?.expected;
    const argNames = argsSample.map(
      (_, idx) => `arg${idx + 1}: ${guessTypeFromValue(argsSample[idx])}`,
    );
    const kwargsPart = hasKwargs ? "**kwargs: dict" : "";
    const argSection = [argNames.join(", "), kwargsPart]
      .filter((part) => part)
      .join(", ");
    let returnType: string | null = null;
    if (Array.isArray(expectedSample)) {
      if (expectedSample.length > 1) {
        const parts = expectedSample
          .map((v: any) => guessTypeFromValue(v))
          .join(", ");
        returnType = `(${parts})`;
      } else if (expectedSample.length === 1) {
        returnType = guessTypeFromValue(expectedSample[0]);
      }
    } else if (expectedSample === null || expectedSample === undefined) {
      returnType = "None";
    } else {
      returnType = guessTypeFromValue(expectedSample);
    }
    let header = `def ${name}(${argSection})`;
    if (returnType && returnType.trim() !== "") {
      header += ` -> ${returnType}`;
    }
    header += ":";
    return header;
  }

  function convertAICaseToBuilder(
    rc: any,
    idx: number,
    meta: FnMeta | null,
    defaultWeight: string,
  ): FnCase {
    const name = rc?.name
      ? String(rc.name)
      : translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::case_n_placeholder",
          { fi: idx + 1 },
        );
    const weightSource = rc?.weight ?? rc?.points ?? rc?.score;
    const timeSource =
      rc?.time_limit ?? rc?.timeLimit ?? rc?.timeout ?? rc?.duration;
    let weight = defaultWeight;
    if (
      weightSource !== undefined &&
      weightSource !== null &&
      weightSource !== ""
    ) {
      const wnum = Number(weightSource);
      if (!Number.isNaN(wnum)) {
        weight = String(wnum);
      }
    }
    let timeLimit = "1";
    if (timeSource !== undefined && timeSource !== null && timeSource !== "") {
      const tnum = Number(timeSource);
      if (!Number.isNaN(tnum)) {
        timeLimit = String(tnum);
      }
    }
    const rawArgs: any[] = Array.isArray(rc?.args)
      ? rc.args
      : rc?.args != null
        ? [rc.args]
        : [];
    const rawKwargs = rc?.kwargs ?? rc?.keyword_args ?? rc?.kwargs_map ?? null;
    const expectedRaw = rc?.expected;
    let args: string[] = rawArgs.map((v) => formatValueForInput(v));
    let kwargs: FnKwarg[] = [];
    if (rawKwargs && typeof rawKwargs === "object") {
      if (Array.isArray(rawKwargs)) {
        kwargs = rawKwargs.map((entry: any) => ({
          key: String(entry?.key ?? ""),
          value: formatValueForInput(entry?.value ?? ""),
        }));
      } else {
        kwargs = Object.entries(rawKwargs).map(([key, value]) => ({
          key: String(key),
          value: formatValueForInput(value),
        }));
      }
    }
    let returns: string[] = [];
    if (Array.isArray(expectedRaw)) {
      returns = expectedRaw.map((v: any) => formatValueForInput(v));
    } else if (expectedRaw !== undefined) {
      returns = [formatValueForInput(expectedRaw)];
    } else {
      returns = [];
    }
    const base: FnCase = {
      name,
      args,
      kwargs,
      returns,
      weight,
      timeLimit,
      fileName: "",
      fileText: "",
      fileBase64: "",
      files: [],
      selectedFileIndex: -1,
    };
    return meta ? ensureCaseShape(base, meta) : base;
  }

  function convertReturnJSONToInputs(
    meta: FnMeta | null,
    raw: string,
  ): string[] {
    if (!meta) return raw ? [raw] : [];
    const count = meta.returns.length;
    if (count === 0) return [];
    if (!raw) return Array.from({ length: count }, () => "");
    try {
      const parsed = JSON.parse(raw);
      if (count > 1) {
        const arr = Array.isArray(parsed) ? parsed : [parsed];
        return meta.returns.map((_, idx) =>
          formatValueForInput(arr[idx] ?? null),
        );
      }
      return [formatValueForInput(parsed)];
    } catch (err) {
      if (count === 1) return [raw];
      return [
        raw,
        ...Array.from({ length: Math.max(count - 1, 0) }, () => ""),
      ];
    }
  }

  function updateFnArg(caseIndex: number, argIndex: number, value: string) {
    fnCases = fnCases.map((c, idx) => {
      if (idx !== caseIndex) return c;
      const nextArgs = [...c.args];
      nextArgs[argIndex] = value;
      return { ...c, args: nextArgs };
    });
  }

  function updateFnReturn(
    caseIndex: number,
    returnIndex: number,
    value: string,
  ) {
    fnCases = fnCases.map((c, idx) => {
      if (idx !== caseIndex) return c;
      const nextReturns = [...c.returns];
      nextReturns[returnIndex] = value;
      return { ...c, returns: nextReturns };
    });
  }

  function updateFnKwargKey(
    caseIndex: number,
    kwargIndex: number,
    value: string,
  ) {
    fnCases = fnCases.map((c, idx) => {
      if (idx !== caseIndex) return c;
      const nextKwargs = [...(c.kwargs ?? [])];
      nextKwargs[kwargIndex] = {
        key: value,
        value: nextKwargs[kwargIndex]?.value ?? "",
      };
      return { ...c, kwargs: nextKwargs };
    });
  }

  function updateFnKwargValue(
    caseIndex: number,
    kwargIndex: number,
    value: string,
  ) {
    fnCases = fnCases.map((c, idx) => {
      if (idx !== caseIndex) return c;
      const nextKwargs = [...(c.kwargs ?? [])];
      nextKwargs[kwargIndex] = {
        key: nextKwargs[kwargIndex]?.key ?? "",
        value,
      };
      return { ...c, kwargs: nextKwargs };
    });
  }

  function addFnKwarg(caseIndex: number) {
    fnCases = fnCases.map((c, idx) => {
      if (idx !== caseIndex) return c;
      const nextKwargs = [...(c.kwargs ?? [])];
      nextKwargs.push({ key: "", value: "" });
      return { ...c, kwargs: nextKwargs };
    });
  }

  function removeFnKwarg(caseIndex: number, kwargIndex: number) {
    fnCases = fnCases.map((c, idx) => {
      if (idx !== caseIndex) return c;
      const nextKwargs = (c.kwargs ?? []).filter((_, i) => i !== kwargIndex);
      return { ...c, kwargs: nextKwargs };
    });
  }

  function buildKwargsObject(kwargs: FnKwarg[]): Record<string, any> {
    const output: Record<string, any> = {};
    for (const pair of kwargs ?? []) {
      const key = String(pair.key ?? "").trim();
      if (!key) continue;
      output[key] = coerceValueForType(pair.value ?? "", undefined);
    }
    return output;
  }

  function stringToBool(value: string): boolean {
    const normalized = String(value ?? "")
      .trim()
      .toLowerCase();
    return (
      normalized === "true" ||
      normalized === "1" ||
      normalized === "yes" ||
      normalized === "on"
    );
  }

  function resetAISolutionInput(clearError = true) {
    aiSolutionFile = null;
    aiSolutionText = "";
    if (clearError) {
      aiSolutionError = "";
    }
    const input = document.getElementById(
      "ai-solution-upload",
    ) as HTMLInputElement | null;
    if (input) {
      input.value = "";
    }
  }

  async function handleAISolutionChange(event: Event) {
    const file = (event.target as HTMLInputElement).files?.[0] ?? null;
    aiSolutionFile = file;
    aiSolutionText = "";
    aiSolutionError = "";
    if (!file) return;
    try {
      aiSolutionText = await file.text();
    } catch (err) {
      aiSolutionError = translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_error",
      );
      resetAISolutionInput(false);
    }
  }

  async function generateWithAI() {
    aiGenerating = true;
    err = "";
    hasAIBuilder = false;
    hasFunctionBuilder = false;
    teacherRun = null;
    try {
      const payload: any = {
        instructions: aiInstructions,
        mode: "unittest",
        call_mode: aiCallMode,
        difficulty: aiDifficulty,
      };
      const trimmedSolution = aiSolutionText.trim();
      if (trimmedSolution) {
        payload.teacher_solution = trimmedSolution;
      }
      if (aiAuto) {
        payload.auto_tests = true;
      } else {
        payload.num_tests = parseInt(aiNumTests) || 5;
      }
      const res = await apiJSON(`/api/assignments/${id}/tests/ai-generate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      const responseMode: "unittest" | "function" =
        res?.mode === "function" ? "function" : "unittest";
      if (typeof res?.call_mode === "string") {
        const normalizedCall = String(res.call_mode).toLowerCase();
        aiCallMode = normalizedCall === "function" ? "function" : "stdin";
      }
      if (typeof res?.difficulty === "string") {
        const normalizedDifficulty = String(res.difficulty).toLowerCase();
        aiDifficulty = normalizedDifficulty === "hard" ? "hard" : "simple";
      }
      if (res?.teacher_solution) {
        const receivedSolution = String(res.teacher_solution);
        if (receivedSolution.trim()) {
          aiSolutionText = receivedSolution;
          aiSolutionFile = null;
          aiSolutionError = "";
          const input = document.getElementById(
            "ai-solution-upload",
          ) as HTMLInputElement | null;
          if (input) input.value = "";
        }
      }
      if (responseMode === "function") {
        builderMode = "function";
        aiCode = typeof res?.python === "string" ? res.python : "";
        const functionName =
          String(res?.function_builder?.function_name ?? "").trim() ||
          "function_name";
        let rawCases: any = res?.function_builder?.cases ?? [];
        if (typeof rawCases === "string") {
          try {
            rawCases = JSON.parse(rawCases || "[]");
          } catch (err) {
            rawCases = [];
          }
        }
        if (!Array.isArray(rawCases)) {
          rawCases = [];
        }
        let signature = String(res?.function_builder?.signature ?? "").trim();
        if (!signature) {
          const sample = rawCases[0] ?? {};
          signature = buildSignatureFromAI(functionName, sample);
        }
        fnSignature = signature;
        const { meta } = parseFunctionSignatureBlock(signature);
        fnMeta = meta;
        const defaultWeight =
          assignment?.grading_policy === "weighted" ? "1" : "0";
        fnCases = rawCases.map((rc: any, idx: number) =>
          convertAICaseToBuilder(rc, idx, meta ?? null, defaultWeight),
        );
        if (!fnCases.length) {
          fnCases = [];
          hasFunctionBuilder = false;
        } else {
          hasFunctionBuilder = true;
        }
      } else {
        builderMode = "unittest";
        aiCode = res.python || "";
        if (res.builder && res.builder.class_name) {
          try {
            utClassName = String(res.builder.class_name);
            const testsRaw = Array.isArray(res.builder.tests)
              ? res.builder.tests
              : JSON.parse(res.builder.tests || "[]");
            utTests = (testsRaw || []).map(coerceUTTest);
            hasAIBuilder = utTests.length > 0;
          } catch (e) {
            hasAIBuilder = false;
          }
          if (hasAIBuilder) {
            refreshPreview();
          }
        }
      }
    } catch (e: any) {
      err = e.message;
    } finally {
      aiGenerating = false;
    }
  }
  async function uploadAIUnitTestsCode() {
    if (builderMode !== "unittest") {
      err = translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_python_upload_only_unittest_mode",
      );
      return;
    }
    if (!aiCode.trim()) return;
    try {
      const snapshot = captureGeneratedState();
      const beforeIds = new Set((tests ?? []).map((t: any) => String(t.id)));
      const blob = new Blob([aiCode], { type: "text/x-python" });
      const file = new File([blob], "ai_tests.py", { type: "text/x-python" });
      const fd = new FormData();
      fd.append("file", file);
      await apiFetch(`/api/assignments/${id}/tests/upload`, {
        method: "POST",
        body: fd,
      });
      await load();
      const newIds = (tests ?? [])
        .map((t: any) => String(t.id))
        .filter((tid) => !beforeIds.has(tid));
      undoPending = { testIds: newIds, snapshot };
      clearGeneratedState();
      clickExistingTestsTab();
      saveModal?.showModal();
    } catch (e: any) {
      err = e.message;
    }
  }

  function normalizeTeacherReturn(raw: any): string {
    if (raw === null || raw === undefined) return "null";
    const text = typeof raw === "string" ? raw : JSON.stringify(raw);
    if (!text.trim()) return "null";
    try {
      JSON.parse(text);
      return text;
    } catch (err) {
      return JSON.stringify(text);
    }
  }

  function pythonLiteralFromValue(value: any): string {
    if (value === null || value === undefined) return "None";
    if (typeof value === "string") return JSON.stringify(value);
    if (typeof value === "number" || typeof value === "boolean")
      return String(value);
    if (Array.isArray(value))
      return `[${value.map((v) => pythonLiteralFromValue(v)).join(", ")}]`;
    if (typeof value === "object") {
      const entries = Object.entries(value).map(
        ([k, v]) => `${JSON.stringify(k)}: ${pythonLiteralFromValue(v)}`,
      );
      return `{${entries.join(", ")}}`;
    }
    return String(value);
  }

  function pythonLiteralFromJSON(raw: string): string {
    const txt = String(raw ?? "").trim();
    if (!txt) return "None";
    try {
      const parsed = JSON.parse(txt);
      return pythonLiteralFromValue(parsed);
    } catch (err) {
      return txt;
    }
  }

  async function runTeacherPreview(previewTests: any[]) {
    if (!teacherSolutionFile) {
      throw new Error(
        translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::upload_teacher_solution_first",
        ),
      );
    }
    const fd = new FormData();
    fd.append("file", teacherSolutionFile);
    if (previewTests.length) {
      fd.append("preview_tests", JSON.stringify(previewTests));
    }
    const res = await apiJSON(`/api/assignments/${id}/solution-run`, {
      method: "POST",
      body: fd,
    });
    const previews = Array.isArray(res?.results)
      ? res.results
          .filter((r: any) => r?.preview)
          .sort((a: any, b: any) =>
            String(a?.test_case_id || "").localeCompare(
              String(b?.test_case_id || ""),
            ),
          )
      : [];
    return { res, previews };
  }
  async function runTeacherSolution() {
    if (!teacherSolutionFile) return;
    teacherRun = null;
    teacherRunLoading = true;
    err = "";
    try {
      const fd = new FormData();
      fd.append("file", teacherSolutionFile);
      const previewTests = collectPreviewTestsForRun();
      if (previewTests.length) {
        fd.append("preview_tests", JSON.stringify(previewTests));
      }
      const res = await apiJSON(`/api/assignments/${id}/solution-run`, {
        method: "POST",
        body: fd,
      });
      teacherRun = res;
    } catch (e: any) {
      err = e.message;
    } finally {
      teacherRunLoading = false;
    }
  }

  async function uploadGeneratedUnitTests() {
    try {
      if (utOutputMode === "teacher") {
        const previewPayloads: any[] = [];
        const indexMap: { ti: number; ai: number; mode: "stdin" | "function" }[] =
          [];
        utTests.forEach((t, ti) => {
          t.assertions.forEach((a, ai) => {
            if (a.kind !== "equals") return;
            const timeLimit = parseFloat(t.timeLimit);
            if (t.callMode === "function") {
              const fn = t.functionName?.trim() || "function_name";
              const argsJSON = JSON.stringify(
                (a as any).args?.map((arg: string) =>
                  coerceValueForType(arg ?? "", undefined),
                ) ?? [],
              );
              const previewPayload: any = {
                execution_mode: "function",
                function_name: fn,
                function_args: argsJSON,
                function_kwargs: "{}",
                expected_return: "",
                time_limit_sec: Number.isFinite(timeLimit)
                  ? timeLimit
                  : undefined,
              };
              if (t.files && t.files.length > 0) {
                previewPayload.files = t.files;
              }
              previewPayloads.push(previewPayload);
              indexMap.push({ ti, ai, mode: "function" });
            } else {
              const stdinVal = ((a as any).args ?? []).join("\n");
              const previewPayload: any = {
                execution_mode: "stdin_stdout",
                stdin: stdinVal,
                expected_stdout: "",
                time_limit_sec: Number.isFinite(timeLimit)
                  ? timeLimit
                  : undefined,
              };
              if (t.files && t.files.length > 0) {
                previewPayload.files = t.files;
              }
              previewPayloads.push(previewPayload);
              indexMap.push({ ti, ai, mode: "stdin" });
            }
          });
        });
        if (previewPayloads.length) {
          utOutputLoading = true;
          const { previews } = await runTeacherPreview(previewPayloads);
          previews.forEach((p, idx) => {
            const mapping = indexMap[idx];
            if (!mapping) return;
            const target = utTests[mapping.ti]?.assertions?.[mapping.ai];
            if (!target) return;
            if (mapping.mode === "function") {
              const lit = pythonLiteralFromJSON(
                normalizeTeacherReturn(
                  p?.actual_return ?? p?.expected_return ?? "",
                ),
              );
              setExpected(target, lit);
            } else {
              setExpected(target, String(p?.actual_stdout ?? ""));
            }
          });
        }
      }
      const snapshot = captureGeneratedState();
      const beforeIds = new Set((tests ?? []).map((t: any) => String(t.id)));
      const code = generateUnittestCode();
      const normalizedCode = code.replace(/\r\n/g, "\n");
      const blob = new Blob([code], { type: "text/x-python" });
      const file = new File([blob], "generated_tests.py", {
        type: "text/x-python",
      });
      const fd = new FormData();
      fd.append("file", file);
      await apiFetch(`/api/assignments/${id}/tests/upload`, {
        method: "POST",
        body: fd,
      });
      await load();
      // After upload, adjust weights/time limits per created unittest method
      const methodConfigs = utTests.map((t) => {
        const methodName = t.name.startsWith("test_")
          ? t.name
          : `test_${t.name}`;
        const qualified = `${utClassName}.${methodName}`;
        const w = parseFloat(t.weight);
        const s = parseFloat(t.timeLimit);
        return {
          qualified,
          weight: Number.isNaN(w) ? 1 : w,
          time: Number.isNaN(s) ? 1 : s,
          fileName: t.fileName,
          fileText: t.fileText,
          fileBase64: t.fileBase64,
          files: t.files,
        };
      });
      const configsByName = new Map(
        methodConfigs.map((cfg) => [cfg.qualified, cfg]),
      );
      const unusedNames = new Set(configsByName.keys());
      for (const t of tests) {
        const mode = (t.execution_mode ??
          (t.unittest_name
            ? "unittest"
            : t.function_name
              ? "function"
              : "stdin_stdout")) as string;
        if (mode !== "unittest") continue;

        let qualifiedName = (t.unittest_name ?? "").trim();
        let cfg = qualifiedName ? configsByName.get(qualifiedName) : undefined;
        const rawCode =
          typeof t.unittest_code === "string" ? t.unittest_code : "";
        const normalizedExisting = rawCode.replace(/\r\n/g, "\n");

        if (!cfg) {
          let candidate =
            normalizedExisting === normalizedCode
              ? [...unusedNames][0]
              : undefined;
          if (!candidate && !rawCode && unusedNames.size === 1) {
            candidate = [...unusedNames][0];
          }
          if (candidate) {
            qualifiedName = candidate;
            cfg = configsByName.get(candidate);
          }
        }
        if (!cfg) continue;

        unusedNames.delete(cfg.qualified);
        const testData: any = {
          execution_mode: "unittest",
          unittest_name: qualifiedName || cfg.qualified,
          unittest_code: rawCode && rawCode.trim() ? rawCode : code,
          stdin: "",
          expected_stdout: "",
          time_limit_sec: cfg.time,
        };

        if (cfg.files && cfg.files.length > 0) {
          testData.files = cfg.files;
          testData.file_name = "";
          testData.file_base64 = "";
        } else {
          const filePayload = buildFilePayload(
            cfg.fileName,
            cfg.fileText,
            cfg.fileBase64,
          );
          if (filePayload) {
            testData.file_name = filePayload.file_name;
            testData.file_base64 = filePayload.file_base64;
          } else {
            testData.file_name = "";
            testData.file_base64 = "";
          }
        }

        // Only include weight for weighted assignments
        if (assignment?.grading_policy === "weighted") {
          testData.weight = cfg.weight;
        }

        await apiFetch(`/api/tests/${t.id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(testData),
        });
      }
      await load();
      const newIds = (tests ?? [])
        .map((t: any) => String(t.id))
        .filter((tid) => !beforeIds.has(tid));
      undoPending = { testIds: newIds, snapshot };
      clearGeneratedState();
      clickExistingTestsTab();
      saveModal?.showModal();
      err = "";
    } catch (e: any) {
      err = e.message;
    } finally {
      utOutputLoading = false;
    }
  }

  async function undoLastSave() {
    if (!undoPending) {
      saveModal?.close();
      return;
    }
    undoLoading = true;
    err = "";
    try {
      for (const tid of undoPending.testIds) {
        if (!tid) continue;
        await apiFetch(`/api/tests/${tid}`, { method: "DELETE" });
      }
      await load();
      restoreGeneratedState(undoPending.snapshot ?? null);
      teacherRun = null;
      undoPending = null;
      clickAITestsTab();
      saveModal?.close();
    } catch (e: any) {
      err = e.message;
    } finally {
      undoLoading = false;
    }
  }

  async function delTest(tid: number) {
    const confirmed = await confirmModal.open({
      title: translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::delete_test",
      ),
      body: translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::this_test_will_be_removed_for_all_students",
      ),
      confirmLabel: translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::delete_label",
      ),
      confirmClass: "btn btn-error",
      cancelClass: "btn",
    });
    if (!confirmed) return;
    try {
      await apiFetch(`/api/tests/${tid}`, { method: "DELETE" });
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }

  async function deleteAllTests() {
    const confirmed = await confirmModal.open({
      title: translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::delete_all_tests",
      ),
      body: translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::all_tests_permanently_deleted",
      ),
      confirmLabel: translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::delete_all_label",
      ),
      confirmClass: "btn btn-error",
      cancelClass: "btn",
    });
    if (!confirmed) return;
    try {
      await apiFetch(`/api/assignments/${id}/tests`, { method: "DELETE" });
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }

  function safeParseJSONArray(json: string): any[] {
    try {
      const parsed = JSON.parse(json || "[]");
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }

  function safeParseJSONObject(json: string): Record<string, any> {
    try {
      const parsed = JSON.parse(json || "{}");
      return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)
        ? parsed
        : {};
    } catch {
      return {};
    }
  }

  function safeParseArgNames(json: string): string[] {
    return safeParseJSONArray(json)
      .map((val) => String(val).trim())
      .filter((val) => val.length > 0);
  }

  function isGenericArgKey(key: string): boolean {
    const trimmed = String(key || "").trim();
    return (
      !trimmed ||
      trimmed.startsWith("#") ||
      (trimmed.startsWith("arg") && /^\d+$/.test(trimmed.substring(3)))
    );
  }

  function coerceArg(val: any): any {
    if (typeof val !== "string") return val;
    const trimmed = val.trim();
    if (!trimmed) return null;
    if (trimmed.toLowerCase() === "true") return true;
    if (trimmed.toLowerCase() === "false") return false;
    if (trimmed.toLowerCase() === "none" || trimmed.toLowerCase() === "null")
      return null;
    if (/^-?\d+$/.test(trimmed)) return parseInt(trimmed, 10);
    if (/^-?\d*\.\d+$/.test(trimmed)) return parseFloat(trimmed);
    try {
      if (trimmed.startsWith("[") || trimmed.startsWith("{")) {
        return JSON.parse(trimmed);
      }
    } catch {
      // ignore
    }
    return trimmed;
  }

  function ensurePrepared(t: any) {
    if (t._prepared_all === undefined) {
      const argsArray = safeParseJSONArray(t.function_args)
        .map(val => typeof val === 'object' ? JSON.stringify(val) : String(val));
      const kwargsObj = safeParseJSONObject(t.function_kwargs);
      const savedArgNames = safeParseArgNames(t.function_arg_names ?? "");
      const metaArgNames =
        fnMeta && fnMeta.name === t.function_name
          ? fnMeta.params.map((p) => p.name)
          : [];
      const argNames = metaArgNames.length ? metaArgNames : savedArgNames;
      
      const all = [];
      // Positional args
      argsArray.forEach((val, i) => {
        const name = argNames[i] || `arg${i + 1}`;
        all.push({ key: name, value: val, is_pos: true, original_idx: i });
      });
      // Keyword args
      Object.entries(kwargsObj).forEach(([k, v]) => {
        all.push({
          key: k,
          value: typeof v === 'object' ? JSON.stringify(v) : String(v),
          is_pos: false
        });
      });
      t._prepared_all = all;
    }
    return t._prepared_all;
  }

  function addPreparedArg(t: any) {
    ensurePrepared(t);
    let nextName = `arg${t._prepared_all.length + 1}`;
    const posCount = t._prepared_all.filter((a: any) => a.is_pos).length;
    if (fnMeta && fnMeta.name === t.function_name && fnMeta.params[posCount]) {
      nextName = fnMeta.params[posCount].name;
    }
    t._prepared_all = [...t._prepared_all, { key: nextName, value: "", is_pos: true, original_idx: posCount }];
    tests = tests;
  }

  function addPreparedKwarg(t: any) {
    ensurePrepared(t);
    t._prepared_all = [...t._prepared_all, { key: "", value: "", is_pos: false }];
    tests = tests;
  }

  function removePreparedArg(t: any, idx: number) {
    // This now receives index in _prepared_all
    ensurePrepared(t);
    t._prepared_all = t._prepared_all.filter((_: any, i: number) => i !== idx);
    tests = tests;
  }

  function removePreparedKwarg(t: any, idx: number) {
    // Legacy support or same as removePreparedArg
    removePreparedArg(t, idx);
  }

  async function updateTest(t: any) {
    try {
      const mode = (t.execution_mode ??
        (t.unittest_name
          ? "unittest"
          : t.function_name
            ? "function"
            : "stdin_stdout")) as string;
      const testData: any = {
        execution_mode: mode,
        stdin: t.stdin ?? "",
        expected_stdout: t.expected_stdout ?? "",
        time_limit_sec: parseFloat(t.time_limit_sec) || undefined,
        files: t.files || [],
      };

      const filePayload = buildExistingTestFilePayload(t);
      if (filePayload) {
        testData.file_name = filePayload.file_name;
        testData.file_base64 = filePayload.file_base64;
      } else {
        testData.file_name = "";
        testData.file_base64 = "";
      }

      if (assignment?.grading_policy === "weighted") {
        testData.weight = parseFloat(t.weight) || 1;
      }

      if (mode === "unittest") {
        testData.unittest_code = t.unittest_code ?? "";
        testData.unittest_name = t.unittest_name ?? "";
      } else if (mode === "function") {
        const fn = String(t.function_name ?? "").trim();
        if (!fn) {
          err = translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_name_is_required",
          );
          return;
        }
        testData.function_name = fn;

        if (t._prepared_all) {
          const args: any[] = [];
          const kwargs: any = {};
          let argNames: string[] = [];
          t._prepared_all.forEach((item: any) => {
            const key = String(item.key || "").trim();
            // If the key is a generic placeholder (e.g. arg1, #1) or empty, keep it positional.
            // Otherwise, express it as a keyword argument as requested.
            const isGeneric = isGenericArgKey(key);
            
            if (isGeneric) {
              args.push(coerceArg(item.value));
            } else {
              kwargs[key] = coerceArg(item.value);
            }
            if (item.is_pos && !isGeneric) {
              argNames.push(key);
            }
          });
          testData.function_args = JSON.stringify(args);
          testData.function_kwargs = JSON.stringify(kwargs);
          if (!argNames.length) {
            const metaArgNames =
              fnMeta && fnMeta.name === fn
                ? fnMeta.params.map((p) => p.name)
                : [];
            const savedArgNames = safeParseArgNames(t.function_arg_names ?? "");
            argNames = metaArgNames.length ? metaArgNames : savedArgNames;
          }
          if (argNames.length) {
            testData.function_arg_names = JSON.stringify(argNames);
          }
        } else {
          testData.function_args =
            typeof t.function_args === "string" ? t.function_args : "[]";
          testData.function_kwargs =
            typeof t.function_kwargs === "string" ? t.function_kwargs : "{}";
          if (typeof t.function_arg_names === "string") {
            testData.function_arg_names = t.function_arg_names;
          }
        }

        testData.expected_return =
          typeof t.expected_return === "string" ? t.expected_return : "";
        testData.stdin = "";
        testData.expected_stdout = "";
      }

      await apiFetch(`/api/tests/${t.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(testData),
      });
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }

  async function saveLLMSettings() {
    try {
      await apiFetch(`/api/assignments/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          // required base fields (preserve current values)
          title: assignment.title,
          description: assignment.description,
          deadline: new Date(assignment.deadline).toISOString(),
          max_points: assignment.max_points,
          grading_policy: assignment.grading_policy,
          show_traceback: assignment.show_traceback,
          show_test_details: !!assignment.show_test_details,
          manual_review: assignment.manual_review,
          llm_interactive: assignment.llm_interactive,
          // llm fields
          llm_feedback: llmFeedback,
          llm_auto_award: llmAutoAward,
          llm_scenarios_json: llmScenarios.trim() ? llmScenarios : null,
          llm_strictness: Number.isFinite(llmStrictness as any)
            ? Math.min(100, Math.max(0, Number(llmStrictness)))
            : 50,
          llm_rubric: llmRubric.trim() ? llmRubric : null,
        }),
      });
      await load();
    } catch (e: any) {
      err = e.message;
    }
  }
</script>

{#if role !== "teacher" && role !== "admin"}
  <div class="alert alert-error">
    <span
      >{translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::no_permission_manage_tests",
      )}</span
    >
  </div>
{:else if !assignment}
  <div class="flex items-center gap-3">
    <span class="loading loading-spinner loading-md"></span>
    <p>
      {translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::loading",
      )}
    </p>
  </div>
{:else}
  <div class="mb-4 flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-semibold">
        {translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::manage_tests",
        )} — {assignment.title}
      </h1>
      {#if assignment.llm_interactive}
        <p class="text-sm opacity-70">
          {translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::configure_ai_testing_llm_interactive",
          )}
        </p>
      {:else}
        <p class="text-sm opacity-70">
          {translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::create_io_tests_build_python_unittest_based_tests_or_use_ai_to_generate_them",
          )}
        </p>
      {/if}
      {#if assignment?.manual_review}
        <div class="alert alert-info mt-2">
          <span
            >{translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::manual_review_enabled_tests_optional",
            )}</span
          >
        </div>
      {/if}
    </div>
    <a class="btn" href={`/assignments/${id}`}
      >{translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::back_to_assignment",
      )}</a
    >
  </div>
  <div class="card-elevated p-6 space-y-6">
    {#if assignment.llm_interactive}
      <div class="grid gap-4">
        <div class="space-y-3">
          <div class="divider">
            {translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_testing_settings",
            )}
          </div>
          <div class="grid sm:grid-cols-2 gap-3">
            <label class="flex items-center gap-2 sm:col-span-2">
              <input
                type="checkbox"
                class="checkbox"
                bind:checked={llmFeedback}
              />
              <span class="label-text"
                >{translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::llm_feedback_visible_to_students",
                )}</span
              >
            </label>
            <label class="flex items-center gap-2">
              <input
                type="checkbox"
                class="checkbox"
                bind:checked={llmAutoAward}
              />
              <span class="label-text"
                >{translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::auto_award_full_points_if_all_scenarios_pass",
                )}</span
              >
            </label>
            <label class="form-control w-full sm:col-span-2">
              <span class="label-text"
                >{translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::scenarios_json_optional",
                )}</span
              >
              <textarea
                class="textarea textarea-bordered h-40"
                bind:value={llmScenarios}
                placeholder={exampleScenario}
              ></textarea>
            </label>
            <div class="sm:col-span-2 grid gap-3">
              <label class="form-control w-full">
                <div class="label">
                  <span class="label-text"
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::strictness",
                    )}</span
                  >
                  <span class="label-text-alt">{llmStrictness}%</span>
                </div>
                <input
                  type="range"
                  min="0"
                  max="100"
                  step="5"
                  class="range range-primary"
                  bind:value={llmStrictness}
                />
                <p class="text-xs opacity-70 mt-2">{llmStrictnessMessage}</p>
              </label>
              <label class="form-control w-full">
                <span class="label-text"
                  >{translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_rubric_what_is_ok_vs_wrong",
                  )}</span
                >
                <textarea
                  class="textarea textarea-bordered h-32"
                  bind:value={llmRubric}
                  placeholder={translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::describe_what_is_acceptable_and_what_should_be_considered_wrong",
                  )}
                ></textarea>
              </label>
            </div>
            <div class="sm:col-span-2 flex justify-end">
              <button class="btn btn-primary" on:click={saveLLMSettings}
                ><Save size={16} />
                {translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::save_settings",
                )}</button
              >
            </div>
          </div>
        </div>
      </div>
    {:else}


      <div class="flex justify-end gap-2 mb-4">
        <button
          class="btn btn-outline"
          on:click={() => document.getElementById("banned_tools_modal").showModal()}
        >
          <Shield size={16} />
          {translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_tools_tab",
          )}
        </button>
        <button
          class="btn btn-neutral"
          on:click={() => document.getElementById("existing_tests_modal").showModal()}
        >
          <Eye size={16} />
          {translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::manage_existing_tests_button",
          )}
        </button>
      </div>

      <dialog id="banned_tools_modal" class="modal">
        <div class="modal-box w-11/12 max-w-4xl bg-base-200/50 backdrop-blur-xl border border-white/10 shadow-2xl p-0 overflow-hidden text-base-content">
          <!-- Premium Header -->
          <div class="p-8 bg-gradient-to-br from-error/10 via-transparent to-transparent border-b border-white/5">
            <form method="dialog">
              <button class="btn btn-sm btn-circle btn-ghost absolute right-4 top-4 z-10 transition-transform hover:rotate-90">✕</button>
            </form>
            <div class="flex items-center gap-6">
              <div class="w-16 h-16 rounded-2xl bg-error/20 text-error flex items-center justify-center shadow-lg shadow-error/10 border border-error/20">
                <ShieldAlert size={32} />
              </div>
              <div>
                <h4 class="text-3xl font-black tracking-tight">
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_tools_heading",
                  )}
                </h4>
                <p class="text-sm opacity-60 font-medium max-w-xl mt-1">
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_tools_hint",
                  )}
                </p>
              </div>
            </div>

            <!-- Stylized Mode Switcher -->
            <div class="flex bg-base-300/50 p-1 rounded-xl mt-8 w-fit border border-white/5">
              <button 
                type="button"
                class="px-6 py-2 rounded-lg text-sm font-bold transition-all duration-200 {toolMode === 'structured' ? 'bg-base-100 shadow-md scale-[1.02] text-primary' : 'hover:bg-base-100/30 opacity-60'}"
                on:click={() => { toolMode = "structured"; bannedSaved = false; }}
              >
                {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_mode_structured")}
              </button>
              <button 
                type="button"
                class="px-6 py-2 rounded-lg text-sm font-bold transition-all duration-200 {toolMode === 'advanced' ? 'bg-base-100 shadow-md scale-[1.02] text-primary' : 'hover:bg-base-100/30 opacity-60'}"
                on:click={() => { toolMode = "advanced"; bannedSaved = false; }}
              >
                {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_mode_advanced")}
              </button>
            </div>
          </div>

          <div class="p-8 space-y-8 max-h-[60vh] overflow-y-auto custom-scrollbar">
            {#if toolMode === "structured"}
              <!-- Add New Rule Section -->
              <div class="card bg-base-100/40 border border-base-300/30 shadow-sm p-6 space-y-6">
                <h5 class="text-sm font-black uppercase tracking-widest opacity-60 flex items-center gap-2">
                  <Plus size={16} class="text-primary" />
                  {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::add_rule")}
                </h5>

                <div class="grid gap-6 md:grid-cols-2">
                  <label class="form-control w-full group">
                    <span class="label-text font-bold text-xs uppercase opacity-40 mb-2 transition-opacity group-focus-within:opacity-100">
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_library_label")}
                    </span>
                    <select
                      class="select select-bordered w-full bg-base-200/50 focus:bg-base-100 border-base-300/50 transition-all font-medium"
                      bind:value={draftLibrary}
                      on:change={() => (bannedSaved = false)}
                    >
                      {#each bannedCatalog as entry}
                        <option value={entry.library}>{entry.label}</option>
                      {/each}
                    </select>
                  </label>

                  <label class="form-control w-full group">
                    <span class="label-text font-bold text-xs uppercase opacity-40 mb-2 transition-opacity group-focus-within:opacity-100">
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_function_label")}
                    </span>
                    <div class="flex gap-2">
                      <div class="relative flex-1">
                        <input
                          class="input input-bordered w-full bg-base-200/50 focus:bg-base-100 border-base-300/50 transition-all font-mono"
                          list="banned-function-options"
                          bind:value={draftFunction}
                          placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_function_placeholder")}
                          on:input={() => (bannedSaved = false)}
                        />
                        <datalist id="banned-function-options">
                          {#each functionOptions as fn}
                            <option value={fn.name}>{fn.label ?? fn.name}</option>
                          {/each}
                        </datalist>
                      </div>
                      <button
                        type="button"
                        class="btn btn-outline border-base-300/50 hover:border-primary/50 transition-colors font-bold"
                        on:click={() => {
                          draftFunction = "*";
                          bannedSaved = false;
                        }}
                      >
                        {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_any_button")}
                      </button>
                    </div>
                  </label>

                  <label class="form-control md:col-span-2 group">
                    <span class="label-text font-bold text-xs uppercase opacity-40 mb-2 transition-opacity group-focus-within:opacity-100">
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_note_label")}
                    </span>
                    <input
                      class="input input-bordered w-full bg-base-200/50 focus:bg-base-100 border-base-300/50 transition-all"
                      bind:value={draftNote}
                      placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_note_placeholder")}
                      on:input={() => (bannedSaved = false)}
                    />
                  </label>

                  <div class="md:col-span-2 flex justify-end">
                    <button
                      type="button"
                      class="btn btn-primary px-8 shadow-lg shadow-primary/20 hover:scale-[1.02] transition-all font-bold"
                      on:click={addStructuredRule}
                    >
                      <Plus size={18} />
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_add_button")}
                    </button>
                  </div>
                </div>
              </div>

              <!-- Configured Rules List -->
              <div class="space-y-4">
                <div class="flex items-center justify-between">
                  <h5 class="text-sm font-black uppercase tracking-widest opacity-60 flex items-center gap-2">
                    <Shield size={16} class="text-error" />
                    {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_rules_heading")}
                  </h5>
                </div>

                {#if structuredRules.length}
                  <div class="grid gap-3">
                    {#each structuredRules as rule, idx}
                      <div class="flex items-center gap-4 p-4 rounded-xl bg-base-100 border border-base-300/40 hover:border-error/30 transition-all group/rule">
                        <div class="w-12 h-12 rounded-lg bg-base-300/30 flex items-center justify-center font-mono text-xs font-black opacity-40 group-hover/rule:bg-error/10 group-hover/rule:text-error group-hover/rule:opacity-100 transition-all">
                          {rule.library.slice(0, 2).toUpperCase()}
                        </div>
                        
                        <div class="flex-1 min-w-0">
                          <div class="flex items-center gap-2 mb-1">
                            <span class="font-bold text-sm tracking-tight">{libraryLabel(rule.library)}</span>
                            <span class="text-xs opacity-30">/</span>
                            <code class="text-xs px-2 py-0.5 rounded bg-base-300/50 font-bold text-error">
                              {rule.function === "*" ? "*" : rule.function}()
                            </code>
                          </div>
                          
                          <input
                            class="w-full bg-transparent border-none text-sm opacity-60 focus:opacity-100 focus:outline-none placeholder:italic"
                            value={rule.note}
                            on:input={(event) => updateStructuredNote(idx, (event.target as HTMLInputElement).value)}
                            placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_note_placeholder")}
                          />
                        </div>

                        <button
                          type="button"
                          class="btn btn-sm btn-ghost hover:bg-error/10 hover:text-error transition-all opacity-20 group-hover/rule:opacity-100"
                          on:click={() => removeStructuredRule(idx)}
                        >
                          <Trash2 size={16} />
                        </button>
                      </div>
                    {/each}
                  </div>
                {:else}
                  <div class="flex flex-col items-center justify-center py-12 px-6 rounded-2xl border-2 border-dashed border-base-300/50 bg-base-200/20 text-center space-y-4">
                    <p class="text-sm opacity-50 font-medium">
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_empty_state")}
                    </p>
                  </div>
                {/if}
              </div>
            {:else}
              <!-- Advanced Mode -->
              <div class="space-y-4">
                <div class="alert bg-info/10 border-info/20 text-info p-4 rounded-xl flex gap-4">
                  <ShieldAlert size={20} />
                  <p class="text-xs font-medium leading-relaxed">
                    {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::advanced_tip")}
                  </p>
                </div>
                
                <label class="form-control w-full group">
                  <span class="label-text font-bold text-xs uppercase opacity-40 mb-3 transition-opacity group-focus-within:opacity-100">
                    {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::advanced_label")}
                  </span>
                  <textarea
                    class="textarea textarea-bordered w-full h-64 bg-base-200/50 focus:bg-base-100 border-base-300/50 transition-all font-mono text-sm p-6"
                    bind:value={advancedPatternsText}
                    placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::advanced_placeholder")}
                    on:input={() => (bannedSaved = false)}
                  ></textarea>
                </label>
              </div>
            {/if}
          </div>

          <!-- Premium Footer Section -->
          <div class="p-8 bg-base-300/30 border-t border-white/5 flex items-center justify-between">
            <div class="flex items-center gap-3">
              {#if bannedSaved}
                <div class="flex items-center gap-2 text-success font-bold text-sm">
                  <Check size={16} />
                  {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_tools_saved")}
                </div>
              {/if}
            </div>

            <div class="flex items-center gap-4">
              <form method="dialog">
                <button class="btn btn-ghost font-bold px-6">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::close")}</button>
              </form>
              <button
                class="btn btn-primary px-8 shadow-lg shadow-primary/20 hover:scale-[1.02] active:scale-95 transition-all font-black"
                on:click={saveBannedTools}
                disabled={bannedSaving}
              >
                {#if bannedSaving}
                  <span class="loading loading-spinner loading-sm"></span>
                {:else}
                  <Save size={18} />
                {/if}
                {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::save_banned_tools")}
              </button>
            </div>
          </div>
        </div>
        <form method="dialog" class="modal-backdrop backdrop-blur-sm bg-base-900/40">
          <button>close</button>
        </form>
      </dialog>


      <dialog id="existing_tests_modal" class="modal">
        <div class="modal-box w-11/12 max-w-5xl bg-base-200/50 backdrop-blur-xl border border-white/10 shadow-2xl p-0 overflow-hidden">
          <div class="p-6 pb-0">
            <form method="dialog">
              <button class="btn btn-sm btn-circle btn-ghost absolute right-4 top-4 z-10">✕</button>
            </form>
            <div class="flex items-center justify-between mb-6">
              <div>
                <h3 class="font-bold text-2xl tracking-tight">
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::existing_tests",
                  )}
                </h3>
                <p class="text-sm opacity-50 font-medium">
                  {tests?.length || 0}
                  {tests?.length === 1
                    ? translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_singular",
                      )
                    : translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_plural",
                      )}
                </p>
              </div>
            </div>
          </div>

          <div class="grid gap-6 max-h-[70vh] overflow-y-auto px-6 pb-6">
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
              
              <div class="group relative bg-base-100 rounded-2xl border border-base-300/50 shadow-sm hover:shadow-xl transition-all duration-300 overflow-hidden
                {mode === 'unittest' ? 'border-l-[6px] border-l-primary' : 
                 mode === 'function' ? 'border-l-[6px] border-l-info' : 
                 'border-l-[6px] border-l-secondary'}"
              >
                <!-- Card Header -->
                <div class="flex items-center justify-between p-5 bg-base-200/30 border-b border-base-200">
                  <div class="flex items-center gap-4">
                    <div class="flex flex-col">
                      <span class="text-[10px] uppercase tracking-widest font-bold opacity-40 mb-1">Test Case</span>
                      <div class="flex items-center gap-2">
                        <span class="text-lg font-black opacity-20">#{i + 1}</span>
                        <div class="flex items-center gap-2">
                          {#if mode === "function"}
                            <span class="badge badge-info badge-sm font-bold gap-1 py-3 px-3">
                              {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::function")}
                            </span>
                            {#if t.function_name}
                              <span class="font-mono text-sm font-semibold text-info/80">{t.function_name}()</span>
                            {/if}
                          {:else if mode === "unittest"}
                            <span class="badge badge-primary badge-sm font-bold gap-1 py-3 px-3">
                              <FlaskConical size={12} />
                              {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest")}
                            </span>
                            {#if utName}
                              <code class="text-xs bg-primary/10 text-primary px-2 py-0.5 rounded font-bold">{utName}</code>
                            {:else}
                              <span class="text-sm opacity-50 italic">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::unnamed_test")}</span>
                            {/if}
                          {:else}
                            <span class="badge badge-secondary badge-sm font-bold gap-1 py-3 px-3">
                              {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::io")}
                            </span>
                          {/if}
                        </div>
                      </div>
                    </div>
                  </div>

                  <div class="flex gap-2">
                    {#if mode === "unittest" && hasUnittestCode}
                      <button
                        class="btn btn-sm btn-ghost hover:bg-base-300 transition-colors tooltip tooltip-bottom"
                        data-tip={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::edit")}
                        on:click={() => openEditUnitTest(t)}
                      >
                        <Code size={18} />
                      </button>
                    {/if}
                    <button 
                      class="btn btn-sm btn-ghost hover:bg-success/20 hover:text-success transition-colors tooltip tooltip-bottom" 
                      data-tip={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::save")}
                      on:click={() => updateTest(t)}
                    >
                      <Save size={18} />
                    </button>
                    <button
                      class="btn btn-sm btn-ghost hover:bg-error/20 hover:text-error transition-colors tooltip tooltip-left"
                      data-tip={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::delete")}
                      on:click={() => delTest(t.id)}
                    >
                      <Trash2 size={18} />
                    </button>
                  </div>
                </div>

                <div class="p-6 space-y-6">
                  {#if mode === "function"}
                    {@const prep = ensurePrepared(t)}
                    <div class="space-y-6">
                      <div class="grid gap-4 md:grid-cols-2">
                        <div class="form-control w-full">
                          <label class="label pb-1">
                            <span class="label-text font-bold text-xs uppercase opacity-60 tracking-wider">
                              {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::function_name")}
                            </span>
                          </label>
                          <input
                            class="input input-bordered w-full bg-base-200/50 focus:bg-base-100 transition-all font-mono"
                            placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::function_name_example_multiply")}
                            bind:value={t.function_name}
                          />
                        </div>
                        <div class="form-control w-full">
                          <label class="label pb-1">
                            <span class="label-text font-bold text-xs uppercase opacity-60 tracking-wider">
                              {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_return_json")}
                            </span>
                          </label>
                          <textarea
                            class="textarea textarea-bordered w-full bg-base-200/50 focus:bg-base-100 transition-all font-mono min-h-[3rem]"
                            rows="1"
                            placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::example_6")}
                            bind:value={t.expected_return}
                          ></textarea>
                        </div>
                      </div>

                      <div class="space-y-3 p-4 rounded-2xl bg-base-200/30 border border-base-200">
                        <div class="flex items-center justify-between mb-2">
                          <h5 class="text-[10px] font-black uppercase tracking-widest opacity-60 flex items-center gap-2">
                            <ChevronRight size={14} class="text-info" />
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::arguments")}
                          </h5>
                          <button
                            class="btn btn-xs btn-outline btn-info border-2 font-bold px-3 hover:scale-105 transition-transform"
                            on:click={() => addPreparedKwarg(t)}
                          >
                            <Plus size={14} />
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::add_arg")}
                          </button>
                        </div>
                        
                        <div class="grid gap-3 md:grid-cols-2">
                          {#each prep as arg, ai}
                            <div class="flex items-center gap-2 group/arg">
                              <input
                                class="input input-bordered input-sm w-32 bg-base-100 font-mono text-xs font-bold transition-all focus:ring-1 focus:ring-info"
                                class:text-info={!arg.is_pos}
                                placeholder={arg.is_pos ? "Arg name" : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::keyword_name")}
                                bind:value={arg.key}
                              />
                              <span class="opacity-30 font-bold">=</span>
                              <div class="flex-1 flex items-center gap-1">
                                <input
                                  class="input input-bordered input-sm flex-1 bg-base-100 transition-all focus:ring-1 focus:ring-info"
                                  placeholder="Value"
                                  bind:value={arg.value}
                                />
                                <button
                                  class="btn btn-circle btn-ghost btn-xs text-error/40 hover:text-error hover:bg-error/10 transition-all opacity-0 group-hover/arg:opacity-100"
                                  on:click={() => removePreparedArg(t, ai)}
                                >
                                  <Trash2 size={12} />
                                </button>
                              </div>
                            </div>
                          {/each}
                          {#if prep.length === 0}
                            <div class="flex flex-col items-center justify-center py-4 opacity-40 md:col-span-2">
                              <p class="text-xs italic">
                                {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::no_arguments")}
                              </p>
                            </div>
                          {/if}
                        </div>
                      </div>
                    </div>
                  {:else if t.unittest_name}
                    {#if t.unittest_code}
                      <details class="group/details">
                        <summary class="cursor-pointer text-sm font-bold opacity-60 hover:opacity-100 flex items-center gap-2 transition-all p-2 rounded-lg hover:bg-base-200 w-fit">
                          <Eye size={16} class="text-primary" />
                          {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::view_test_method_code")}
                          <ChevronRight size={14} class="group-open/details:rotate-90 transition-transform" />
                        </summary>
                        <div class="mt-4 p-4 rounded-xl bg-base-900 text-base-content border border-base-200 overflow-x-auto">
                          <pre class="text-xs leading-relaxed font-mono">{extractMethodFromUnittest(t.unittest_code, t.unittest_name)}</pre>
                        </div>
                      </details>
                    {/if}
                  {:else}
                    <div class="grid sm:grid-cols-2 gap-6">
                      <div class="form-control w-full">
                        <label class="label pb-1">
                          <span class="label-text font-bold text-xs uppercase opacity-60 tracking-wider">
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::input")}
                          </span>
                        </label>
                        <textarea
                          class="textarea textarea-bordered w-full bg-base-200/50 focus:bg-base-100 transition-all font-mono min-h-[6rem]"
                          rows="4"
                          placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::stdin")}
                          bind:value={t.stdin}
                        ></textarea>
                      </div>
                      <div class="form-control w-full">
                        <label class="label pb-1">
                          <span class="label-text font-bold text-xs uppercase opacity-60 tracking-wider">
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_output")}
                          </span>
                        </label>
                        <textarea
                          class="textarea textarea-bordered w-full bg-base-200/50 focus:bg-base-100 transition-all font-mono min-h-[6rem]"
                          rows="4"
                          placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_stdout")}
                          bind:value={t.expected_stdout}
                        ></textarea>
                      </div>
                    </div>
                  {/if}

                  <div class="flex flex-wrap items-end justify-between gap-6 pt-4 border-t border-base-200/50">
                    <div class="flex flex-wrap gap-6">
                      <div class="form-control">
                        <label class="label pb-1.5 px-0">
                          <span class="label-text flex items-center gap-2 font-bold text-[10px] uppercase tracking-widest opacity-60">
                            <Clock size={14} class="text-warning" />
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::time_limit_s_small")}
                          </span>
                        </label>
                        <div class="relative flex items-center">
                          <input
                            class="input input-bordered input-sm w-32 font-bold bg-base-200/30 transition-all focus:w-40"
                            placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::seconds")}
                            bind:value={t.time_limit_sec}
                          />
                          <span class="absolute right-3 text-[10px] font-black opacity-30 pointer-events-none">SEC</span>
                        </div>
                      </div>
                      
                      {#if assignment?.grading_policy === "weighted"}
                        <div class="form-control">
                          <label class="label pb-1.5 px-0">
                            <span class="label-text flex items-center gap-2 font-bold text-[10px] uppercase tracking-widest opacity-60">
                              <Scale size={14} class="text-error" />
                              {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::points")}
                            </span>
                          </label>
                          <div class="relative flex items-center">
                            <input
                              class="input input-bordered input-sm w-32 font-bold bg-base-200/30 transition-all focus:w-40"
                              placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::points_placeholder")}
                              bind:value={t.weight}
                            />
                            <span class="absolute right-3 text-[10px] font-black opacity-30 pointer-events-none">PTS</span>
                          </div>
                        </div>
                      {/if}
                    </div>

                    <!-- Files Section -->
                    <div class="flex-1 flex justify-end">
                      {#if mode === "function" || mode === "stdin_stdout" || mode === "unittest"}
                        {@const hasFiles = t.files && t.files.length > 0}
                        <div class="flex items-center gap-2">
                          {#if hasFiles}
                            <div class="flex -space-x-2 mr-2">
                              {#each t.files.slice(0, 3) as _, fi}
                                <div class="w-6 h-6 rounded-full bg-base-300 border-2 border-base-100 flex items-center justify-center shadow-sm">
                                  <FileCode2 size={10} class="opacity-70" />
                                </div>
                              {/each}
                              {#if t.files.length > 3}
                                <div class="w-6 h-6 rounded-full bg-base-200 border-2 border-base-100 flex items-center justify-center text-[8px] font-bold">
                                  +{t.files.length - 3}
                                </div>
                              {/if}
                            </div>
                          {/if}
                          <button
                            class="btn btn-sm {hasFiles ? 'btn-ghost' : 'btn-outline'} gap-2 px-4 shadow-sm hover:scale-105 transition-all"
                            on:click={() => {
                              t.showFileEditor = !t.showFileEditor;
                              if (!t.showFileEditor) t.selectedFileIndex = undefined;
                              tests = tests;
                            }}
                          >
                            {#if hasFiles}
                              <FileCode2 size={16} class="text-primary" />
                              <span class="font-bold">{t.files.length} {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::attached_files")}</span>
                            {:else}
                              <FileUp size={16} />
                              <span>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::attach_files")}</span>
                            {/if}
                          </button>
                        </div>
                      {/if}
                    </div>
                  </div>

                  <!-- File Editor Expansion -->
                  {#if t.showFileEditor}
                    <div class="mt-4 p-5 rounded-2xl bg-base-200/50 border border-dashed border-base-300 space-y-4 animate-in slide-in-from-top-2 duration-300">
                      <div class="flex items-center justify-between mb-2">
                        <span class="text-xs font-black uppercase tracking-widest opacity-60">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::manage_files")}</span>
                        <button class="btn btn-xs btn-ghost" on:click={() => { t.showFileEditor = false; tests = tests; }}>✕</button>
                      </div>

                      {#if t.files && t.files.length > 0}
                        <div class="flex flex-wrap gap-2">
                          {#each t.files as f, fi}
                            <button
                              class="badge badge-lg gap-2 py-4 px-3 cursor-pointer border-2 transition-all hover:scale-105"
                              class:badge-primary={t.selectedFileIndex === fi}
                              class:badge-ghost={t.selectedFileIndex !== fi}
                              on:click={() => {
                                t.selectedFileIndex = fi;
                                t.file_create_name = f.name;
                                try { t.file_create_text = atob(f.content); } catch { t.file_create_text = ""; }
                                t.file_create_dirty = false;
                                tests = tests;
                              }}
                            >
                              <span class="truncate max-w-[120px] font-bold text-xs">{f.name}</span>
                              <span
                                class="btn btn-ghost btn-xs btn-circle text-error/60 hover:text-error hover:bg-error/10 min-h-0 h-5 w-5"
                                on:click|stopPropagation={() => {
                                  t.files = t.files.filter((_, idx) => idx !== fi);
                                  if (t.selectedFileIndex === fi) {
                                    t.selectedFileIndex = -1;
                                    t.file_create_name = "";
                                    t.file_create_text = "";
                                  } else if (t.selectedFileIndex > fi) {
                                    t.selectedFileIndex--;
                                  }
                                  tests = tests;
                                }}
                              ><Trash2 size={10} /></span>
                            </button>
                          {/each}
                          <button
                            class="badge badge-lg badge-outline border-dashed gap-2 py-4 px-3 cursor-pointer hover:border-solid hover:bg-base-200 transition-all font-bold text-xs"
                            on:click={() => {
                              t.selectedFileIndex = -1;
                              t.file_create_name = "";
                              t.file_create_text = "";
                              tests = tests;
                            }}
                          >
                            <Plus size={14} /> New File
                          </button>
                        </div>
                      {/if}

                      <div class="grid gap-4 md:grid-cols-2">
                        <div class="form-control w-full">
                          <label class="label pt-0"><span class="label-text font-bold text-[10px] uppercase opacity-50">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_upload")}</span></label>
                          <input
                            type="file"
                            multiple
                            class="file-input file-input-bordered file-input-sm w-full bg-base-100"
                            on:click={(e) => ((e.target as HTMLInputElement).value = "")}
                            on:change={async (e) => {
                              const files = (e.target as HTMLInputElement).files || [];
                              if (!files.length) return;
                              if (!t.files) t.files = [];
                              for (let i = 0; i < files.length; i++) {
                                const file = files[i];
                                try {
                                  const b64 = await readFileBase64(file);
                                  t.files.push({ name: file.name, content: b64 });
                                  if (i === files.length - 1) {
                                    t.selectedFileIndex = t.files.length - 1;
                                    t.file_create_name = file.name;
                                    try { t.file_create_text = await readFileText(file); } catch { t.file_create_text = ""; }
                                  }
                                } catch (e: any) { console.error(e); }
                              }
                              tests = tests;
                            }}
                          />
                        </div>
                        <div class="form-control w-full">
                          <label class="label pt-0"><span class="label-text font-bold text-[10px] uppercase opacity-50">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_name")}</span></label>
                          <input
                            class="input input-bordered input-sm w-full bg-base-100 font-mono"
                            placeholder="data.txt"
                            value={t.file_create_name ?? ""}
                            on:input={(e) => {
                              t.file_create_name = (e.target as HTMLInputElement).value;
                              if (t.selectedFileIndex >= 0 && t.files) {
                                t.files[t.selectedFileIndex].name = t.file_create_name;
                                tests = tests;
                              }
                            }}
                          />
                        </div>
                      </div>
                      <div class="form-control w-full">
                        <label class="label pt-0"><span class="label-text font-bold text-[10px] uppercase opacity-50">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents")}</span></label>
                        <textarea
                          class="textarea textarea-bordered w-full font-mono text-xs leading-relaxed bg-base-100 min-h-[5rem]"
                          placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents_hint")}
                          value={t.file_create_text ?? ""}
                          on:input={(e) => {
                            t.file_create_text = (e.target as HTMLTextAreaElement).value;
                            if (t.selectedFileIndex >= 0 && t.files) {
                              t.files[t.selectedFileIndex].content = textToBase64(t.file_create_text);
                              tests = tests;
                            }
                          }}
                        ></textarea>
                      </div>
                    </div>
                  {/if}
                </div>
              </div>
            {/each}
            
            {#if !(tests && tests.length)}
              <div class="flex flex-col items-center justify-center py-20 opacity-30">
                <FlaskConical size={48} class="mb-4" />
                <p class="text-lg font-medium">
                  {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::no_tests")}
                </p>
              </div>
            {/if}
          </div>

          <div class="p-6 bg-base-200/50 border-t border-base-300/50 flex items-center justify-between">
            <div class="flex items-center gap-2 opacity-40 hover:opacity-100 transition-opacity">
              <Shield size={16} />
              <span class="text-[10px] font-black uppercase tracking-widest leading-none">Safe Test Mode Active</span>
            </div>
            <button
              class="btn btn-error btn-outline btn-sm font-black tracking-tighter hover:scale-105 transition-transform"
              on:click={deleteAllTests}
              disabled={!tests || tests.length === 0}
            >
              <Trash2 size={14} />
              {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::delete_all")}
            </button>
          </div>
        </div>
        <form method="dialog" class="modal-backdrop backdrop-blur-sm bg-base-900/40">
          <button>close</button>
        </form>
      </dialog>

      <div role="tablist" class="tabs tabs-lifted">

        <input
          type="radio"
          name="tests-tab"
          role="tab"
          class="tab"
          aria-label={translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_io_test",
          )}
          checked
        />
        <div
          role="tabpanel"
          class="tab-content bg-base-100 border-base-300 rounded-box p-4"
        >
          <div class="group relative bg-base-100 rounded-2xl border border-base-300/50 shadow-sm hover:shadow-xl transition-all duration-300 overflow-hidden border-l-[6px] border-l-secondary">
            <!-- Header -->
            <div class="flex flex-wrap items-center justify-between p-5 bg-base-200/30 border-b border-base-200 gap-4">
              <div class="flex items-center gap-4">
                <div class="flex flex-col">
                  <span class="text-[10px] uppercase tracking-widest font-bold opacity-40 mb-1">New Test Case</span>
                  <div class="flex items-center gap-3">
                    <div class="w-10 h-10 rounded-xl bg-secondary/10 text-secondary flex items-center justify-center">
                      <Code size={20} />
                    </div>
                    <div>
                      <h4 class="font-bold text-lg leading-none">
                        {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::add_io_test")}
                      </h4>
                      <p class="text-xs opacity-50 font-medium mt-1">
                        {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::io_expected_source_hint")}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
              
              <div class="flex flex-wrap gap-2">
                <button
                  type="button"
                  class={`option-pill ${ioOutputMode === "manual" ? "selected" : ""}`}
                  aria-pressed={ioOutputMode === "manual"}
                  on:click={() => (ioOutputMode = "manual")}
                >
                  <span class="option-pill__indicator" aria-hidden="true" />
                  <span
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::output_source_manual",
                    )}</span
                  >
                </button>
                <button
                  type="button"
                  class={`option-pill ${ioOutputMode === "teacher" ? "selected" : ""}`}
                  aria-pressed={ioOutputMode === "teacher"}
                  on:click={() => (ioOutputMode = "teacher")}
                >
                  <span class="option-pill__indicator" aria-hidden="true" />
                  <span
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::output_source_teacher",
                    )}</span
                  >
                </button>
              </div>
            </div>

            <div class="p-6 space-y-6">
              <!-- Teacher Solution Upload (if applicable) -->
              {#if ioOutputMode === "teacher"}
                <div class="p-5 rounded-2xl bg-primary/5 border border-dashed border-primary/30 space-y-3 animate-in fade-in slide-in-from-top-2 duration-300">
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-black uppercase tracking-widest text-primary/70">
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_solution_file")}
                    </span>
                    {#if teacherSolutionFile}
                      <div class="badge badge-primary gap-2 py-3">
                        <Check size={12} />
                        {teacherSolutionFile.name}
                      </div>
                    {/if}
                  </div>
                  <input
                    type="file"
                    accept=".py,.zip,.txt"
                    class="file-input file-input-bordered file-input-primary w-full bg-base-100"
                    on:change={(e) =>
                      (teacherSolutionFile =
                        (e.target as HTMLInputElement).files?.[0] || null)}
                  />
                  <p class="text-[10px] opacity-60 italic">
                    {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_solution_vm_hint")}
                  </p>
                </div>
              {/if}

              <!-- IO Inputs -->
              <div class="grid sm:grid-cols-2 gap-6">
                <div class="form-control w-full">
                  <label class="label pb-1.5 px-0">
                    <span class="label-text flex items-center gap-2 font-bold text-xs uppercase tracking-wider opacity-60 text-secondary">
                      <Terminal size={14} />
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::input")}
                    </span>
                  </label>
                  <textarea
                    class="textarea textarea-bordered w-full bg-base-200/50 focus:bg-base-100 transition-all font-mono min-h-[8rem] shadow-inner"
                    placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::stdin")}
                    bind:value={tStdin}
                  ></textarea>
                </div>

                <div class="form-control w-full">
                  <label class="label pb-1.5 px-0">
                    <span class="label-text flex items-center gap-2 font-bold text-xs uppercase tracking-wider opacity-60 text-secondary">
                      <ArrowRight size={14} />
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_output")}
                      {#if ioOutputMode === "teacher"}
                        <span class="badge badge-secondary badge-xs font-black">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::auto_from_teacher")}</span>
                      {/if}
                    </span>
                  </label>
                  {#if ioOutputMode === "manual"}
                    <textarea
                      class="textarea textarea-bordered w-full bg-base-200/50 focus:bg-base-100 transition-all font-mono min-h-[8rem] shadow-inner"
                      placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_stdout")}
                      bind:value={tStdout}
                    ></textarea>
                  {:else}
                    <div class="flex flex-col items-center justify-center min-h-[8rem] rounded-xl border border-dashed border-base-300 bg-base-200/30 text-center p-4">
                      {#if ioOutputLoading}
                        <span class="loading loading-spinner text-secondary mb-2"></span>
                        <p class="text-xs font-medium opacity-60">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_solution_running")}</p>
                      {:else}
                        <Cpu size={24} class="opacity-20 mb-2" />
                        <p class="text-xs font-medium opacity-60 leading-relaxed">
                          {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_output_from_teacher")}
                        </p>
                      {/if}
                    </div>
                  {/if}
                </div>
              </div>

              <!-- Settings & Files -->
              <div class="flex flex-wrap items-end justify-between gap-6 pt-6 border-t border-base-200/50">
                <div class="flex flex-wrap gap-6">
                  <div class="form-control">
                    <label class="label pb-1.5 px-0">
                      <span class="label-text flex items-center gap-2 font-bold text-[10px] uppercase tracking-widest opacity-60">
                        <Clock size={14} class="text-warning" />
                        {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::time_limit_s")}
                      </span>
                    </label>
                    <div class="relative flex items-center">
                      <input
                        class="input input-bordered input-sm w-32 font-bold bg-base-200/30 transition-all focus:w-40"
                        placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::seconds")}
                        bind:value={tLimit}
                      />
                      <span class="absolute right-3 text-[10px] font-black opacity-30 pointer-events-none">SEC</span>
                    </div>
                  </div>

                  {#if assignment?.grading_policy === "weighted"}
                    <div class="form-control">
                      <label class="label pb-1.5 px-0">
                        <span class="label-text flex items-center gap-2 font-bold text-[10px] uppercase tracking-widest opacity-60">
                          <Scale size={14} class="text-error" />
                          {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::points")}
                        </span>
                      </label>
                      <div class="relative flex items-center">
                        <input
                          class="input input-bordered input-sm w-32 font-bold bg-base-200/30 transition-all focus:w-40"
                          placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::points_placeholder")}
                          bind:value={tWeight}
                        />
                        <span class="absolute right-3 text-[10px] font-black opacity-30 pointer-events-none">PTS</span>
                      </div>
                    </div>
                  {/if}
                </div>

                <div class="flex-1 flex justify-end">
                  <TestFileManager
                    bind:files={ioFiles}
                    bind:selectedIndex={ioSelectedIndex}
                    bind:fileName={ioFileName}
                    bind:fileText={ioFileText}
                    bind:open={showIOFile}
                    onError={(message) => (err = message)}
                  />
                </div>
              </div>

              <!-- Footer / Action -->
              <div class="flex flex-wrap items-center justify-between gap-4 pt-4">
                <p class="text-[10px] opacity-40 font-medium max-w-md italic">
                  {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::stdin_stdout_multiline_hint")}
                </p>
                <button
                  class="btn btn-secondary px-8 font-bold shadow-lg shadow-secondary/20 hover:scale-105 transition-all"
                  on:click={addTest}
                  disabled={!tStdin ||
                    ioOutputLoading ||
                    (ioOutputMode === "manual"
                      ? !tStdout
                      : !teacherSolutionFile)}
                >
                  {#if ioOutputLoading}
                    <span class="loading loading-spinner loading-sm"></span>
                  {:else}
                    <Plus size={18} />
                  {/if}
                  {translate(
                    ioOutputMode === "teacher"
                      ? "frontend/src/routes/assignments/[id]/tests/+page.svelte::generate_and_add"
                      : "frontend/src/routes/assignments/[id]/tests/+page.svelte::add",
                  )}
                </button>
              </div>
            </div>
          </div>
        </div>

        <input
          type="radio"
          name="tests-tab"
          role="tab"
          class="tab"
          aria-label={translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_builder",
          )}
        />
        <div
          role="tabpanel"
          class="tab-content bg-base-100 border-base-300 rounded-box p-4 space-y-4"
        >
          <div class="group relative bg-base-100 rounded-2xl border border-base-300/50 shadow-sm hover:shadow-xl transition-all duration-300 overflow-hidden border-l-[6px] border-l-primary">
            <!-- Header -->
            <div class="flex flex-wrap items-center justify-between p-5 bg-base-200/30 border-b border-base-200 gap-4">
              <div class="flex items-center gap-4">
                <div class="flex flex-col">
                  <span class="text-[10px] uppercase tracking-widest font-bold opacity-40 mb-1">Advanced Testing</span>
                  <div class="flex items-center gap-3">
                    <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center">
                      <FlaskConical size={20} />
                    </div>
                    <div>
                      <h4 class="font-bold text-lg leading-none">
                        {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_builder")}
                      </h4>
                      <p class="text-xs opacity-50 font-medium mt-1">
                        {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_expected_hint")}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
              
              <div class="flex flex-wrap gap-2">
                <button
                  type="button"
                  class={`option-pill ${utOutputMode === "manual" ? "selected" : ""}`}
                  aria-pressed={utOutputMode === "manual"}
                  on:click={() => (utOutputMode = "manual")}
                >
                  <span class="option-pill__indicator" aria-hidden="true" />
                  <span>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::output_source_manual")}</span>
                </button>
                <button
                  type="button"
                  class={`option-pill ${utOutputMode === "teacher" ? "selected" : ""}`}
                  aria-pressed={utOutputMode === "teacher"}
                  on:click={() => (utOutputMode = "teacher")}
                >
                  <span class="option-pill__indicator" aria-hidden="true" />
                  <span>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::output_source_teacher")}</span>
                </button>
              </div>
            </div>

            <div class="p-6 space-y-6">
                <!-- Teacher Solution (if teacher mode) -->
                {#if utOutputMode === "teacher"}
                  <div class="p-5 rounded-2xl bg-primary/5 border border-dashed border-primary/30 space-y-3 animate-in fade-in slide-in-from-top-2 duration-300">
                    <div class="flex items-center justify-between">
                      <span class="text-xs font-black uppercase tracking-widest text-primary/70">
                        {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_solution_file")}
                      </span>
                      {#if teacherSolutionFile}
                        <div class="badge badge-primary gap-2 py-3">
                          <Check size={12} />
                          {teacherSolutionFile.name}
                        </div>
                      {/if}
                    </div>
                    <input
                      type="file"
                      accept=".py,.zip,.txt"
                      class="file-input file-input-bordered file-input-primary w-full bg-base-100"
                      on:change={(e) => (teacherSolutionFile = (e.target as HTMLInputElement).files?.[0] || null)}
                    />
                    <p class="text-[10px] opacity-60 italic leading-relaxed">
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_teacher_autofill_hint")}
                    </p>
                  </div>
                {/if}

                <!-- Class Settings -->
                <div class="grid sm:grid-cols-[1fr_auto] gap-4 items-end bg-base-200/30 p-4 rounded-xl border border-base-200">
                  <label class="form-control w-full space-y-1">
                    <span class="text-[10px] uppercase font-black tracking-widest opacity-40 px-1">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::class_name")}</span>
                    <input
                      class="input input-bordered w-full font-bold"
                      bind:value={utClassName}
                      placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::test_assignment_placeholder")}
                    />
                  </label>
                  <div class="flex gap-2">
                    <button type="button" class="btn btn-outline border-2 font-bold px-6" on:click={addUTTest}>
                      <Plus size={18} />
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::add_test_method")}
                    </button>
                    <button
                      type="button"
                      class="btn btn-ghost border-2 border-transparent hover:border-base-300 font-bold"
                      on:click={() => { utShowPreview = !utShowPreview; refreshPreview(); }}
                    >
                      <Eye size={18} />
                      {utShowPreview ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::hide_code") : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::preview_code")}
                    </button>
                  </div>
                </div>

                <!-- Advanced Setup/Teardown -->
                <div>
                  <button
                    type="button"
                    class="btn btn-ghost btn-xs font-black uppercase tracking-widest opacity-60 hover:opacity-100"
                    on:click={() => (showAdvanced = !showAdvanced)}
                  >
                    <ChevronRight size={14} class={showAdvanced ? 'rotate-90' : ''} />
                    {showAdvanced ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::hide_code") : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::advanced_setup_teardown")}
                  </button>
                  {#if showAdvanced}
                    <div class="grid sm:grid-cols-2 gap-4 mt-4 animate-in fade-in slide-in-from-top-2">
                      <div class="space-y-2">
                        <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::setup_optional")}</span>
                        <div class="rounded-xl overflow-hidden border border-base-300"><CodeMirror bind:value={utSetup} lang={python()} readOnly={false} /></div>
                      </div>
                      <div class="space-y-2">
                        <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::teardown_optional")}</span>
                        <div class="rounded-xl overflow-hidden border border-base-300"><CodeMirror bind:value={utTeardown} lang={python()} readOnly={false} /></div>
                      </div>
                    </div>
                  {/if}
                </div>

                <!-- Test Methods -->
                <div class="space-y-4 pt-4">
                  {#each utTests as ut, ti}
                    <div class="rounded-2xl border border-base-300/50 bg-base-100/50 p-6 space-y-6 relative group/method hover:border-primary/30 transition-all shadow-sm">
                      <button
                        type="button"
                        class="btn btn-circle btn-ghost btn-xs absolute right-4 top-4 text-error/30 hover:text-error hover:bg-error/10"
                        on:click={() => removeUTTest(ti)}
                      ><Trash2 size={14} /></button>

                      <div class="grid gap-6 sm:grid-cols-2">
                        <div class="form-control space-y-1">
                          <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::method_name")}</span>
                          <input
                            class="input input-bordered w-full font-mono font-bold text-sm"
                            bind:value={ut.name}
                            placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::test_something_placeholder")}
                          />
                        </div>
                        <div class="form-control space-y-1">
                          <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::description")}</span>
                          <input
                            class="input input-bordered w-full text-sm"
                            bind:value={ut.description}
                            placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::what_this_test_checks_placeholder")}
                          />
                        </div>
                      </div>

                      <div class="grid sm:grid-cols-2 gap-6">
                        <div class="form-control space-y-1">
                          <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::call_mode")}</span>
                          <select class="select select-bordered w-full font-semibold" bind:value={ut.callMode}>
                            <option value="stdin">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::student_code_stdin_stdout")}</option>
                            <option value="function">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::student_function_return_value")}</option>
                          </select>
                        </div>
                        {#if ut.callMode === "function"}
                          <div class="form-control space-y-1 animate-in fade-in slide-in-from-left-2">
                            <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::function_name")}</span>
                            <input
                              class="input input-bordered w-full font-mono font-bold text-sm"
                              bind:value={ut.functionName}
                              placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::solve_placeholder")}
                            />
                          </div>
                        {/if}
                      </div>

                      <div class="flex flex-wrap gap-6 pt-4 border-t border-base-200/50">
                        <div class="form-control">
                          <span class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-1.5 flex items-center gap-2"><Clock size={12} class="text-warning" /> {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::time_limit_s")}</span>
                          <div class="relative flex items-center">
                            <input class="input input-bordered input-sm w-28 font-bold" bind:value={ut.timeLimit} />
                            <span class="absolute right-2 text-[8px] font-black opacity-30">SEC</span>
                          </div>
                        </div>
                        {#if assignment?.grading_policy === "weighted"}
                          <div class="form-control">
                            <span class="text-[10px] font-black uppercase tracking-widest opacity-40 mb-1.5 flex items-center gap-2"><Scale size={12} class="text-error" /> {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::points")}</span>
                            <div class="relative flex items-center">
                              <input class="input input-bordered input-sm w-28 font-bold" bind:value={ut.weight} />
                              <span class="absolute right-2 text-[8px] font-black opacity-30">PTS</span>
                            </div>
                          </div>
                        {/if}
                        <div class="flex-1 flex justify-end items-end">
                            <TestFileManager
                                bind:files={ut.files}
                                bind:selectedIndex={ut.selectedFileIndex}
                                bind:fileName={ut.fileName}
                                bind:fileText={ut.fileText}
                                bind:open={ut.showFile}
                                onError={(message) => (err = message)}
                            />
                        </div>
                      </div>

                      <!-- Assertions -->
                      <div class="space-y-4 bg-base-200/40 p-5 rounded-2xl border border-base-200">
                         <div class="flex items-center justify-between">
                            <div class="flex items-center gap-2 font-bold text-xs uppercase tracking-wider opacity-60">
                                <Shield size={14} class="text-primary" />
                                {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::assertions")}
                                {#if utOutputMode === "teacher"}
                                    <span class="badge badge-primary badge-xs font-black">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_equals_only_badge")}</span>
                                {/if}
                            </div>
                            <div class="dropdown dropdown-end">
                              <label tabindex="0" class="btn btn-primary btn-xs font-bold gap-1 px-3">
                                <Plus size={12} /> Add Assertion
                              </label>
                              <ul tabindex="0" class="dropdown-content z-[20] menu p-2 shadow-xl bg-base-100 rounded-box w-52 border border-base-200 mt-2">
                                <li><button type="button" on:click={() => addUTAssertion(ti, "equals")}>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::equals")}</button></li>
                                <li><button type="button" disabled={utOutputMode === "teacher"} on:click={() => addUTAssertion(ti, "notEquals")}>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::not_equals")}</button></li>
                                <li><button type="button" disabled={utOutputMode === "teacher"} on:click={() => addUTAssertion(ti, "contains")}>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::contains")}</button></li>
                                <li><button type="button" disabled={utOutputMode === "teacher"} on:click={() => addUTAssertion(ti, "notContains")}>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::not_contains")}</button></li>
                                <li><button type="button" disabled={utOutputMode === "teacher"} on:click={() => addUTAssertion(ti, "regex")}>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::regex")}</button></li>
                                <li><button type="button" disabled={utOutputMode === "teacher"} on:click={() => addUTAssertion(ti, "raises")}>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::raises")}</button></li>
                                <li><button type="button" disabled={utOutputMode === "teacher"} on:click={() => addUTAssertion(ti, "custom")}>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::custom")}</button></li>
                              </ul>
                            </div>
                         </div>

                         <div class="grid gap-3">
                            {#each ut.assertions as a, ai}
                                <div class="bg-base-100 rounded-xl border border-base-300 p-4 space-y-4 shadow-sm animate-in zoom-in-95 duration-200">
                                     <div class="flex items-center justify-between">
                                        <span class="text-[10px] font-black uppercase tracking-widest bg-primary/10 text-primary px-2 py-1 rounded">{a.kind}</span>
                                        <button type="button" class="btn btn-ghost btn-circle btn-xs text-error/40 hover:text-error" on:click={() => removeUTAssertion(ti, ai)}><Trash2 size={12} /></button>
                                     </div>

                                     {#if a.kind === "custom"}
                                        <div class="form-control space-y-1">
                                            <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::custom_python_inside_test_method")}</span>
                                            <textarea class="textarea textarea-bordered h-28 font-mono text-xs" value={getCustom(a)} on:input={(e) => setCustom(a, e.target.value)} placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::self_assert_true_placeholder")}></textarea>
                                        </div>
                                     {:else if a.kind === "regex"}
                                         <div class="grid sm:grid-cols-2 gap-4">
                                             <div class="form-control space-y-1">
                                                <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{ut.callMode === "function" ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::arguments_python_expressions_one_per_line") : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::inputs_one_per_line")}</span>
                                                <textarea class="textarea textarea-bordered h-24 font-mono text-xs" value={getInputs(a)} on:input={(e) => setInputs(a, e.target.value)} placeholder={ut.callMode === "function" ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::example_args_1_2_3") : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::example_inputs_2_3")}></textarea>
                                             </div>
                                             <div class="form-control space-y-1">
                                                <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::regex_pattern")}</span>
                                                <input class="input input-bordered w-full font-mono text-xs" value={getPattern(a)} on:input={(e) => setPattern(a, e.target.value)} />
                                             </div>
                                         </div>
                                     {:else if a.kind === "raises"}
                                         <div class="grid sm:grid-cols-2 gap-4">
                                               <div class="form-control space-y-1">
                                                <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{ut.callMode === "function" ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::arguments_python_expressions_one_per_line") : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::inputs_one_per_line")}</span>
                                                <textarea class="textarea textarea-bordered h-24 font-mono text-xs" value={getInputs(a)} on:input={(e) => setInputs(a, e.target.value)} placeholder={ut.callMode === "function" ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::example_args_1_2_3") : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::example_inputs_2_3")}></textarea>
                                             </div>
                                             <div class="form-control space-y-1">
                                                <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::exception_type_e_g_valueerror")}</span>
                                                <input class="input input-bordered w-full font-mono text-xs" value={getException(a)} on:input={(e) => setException(a, e.target.value)} placeholder="ValueError" />
                                             </div>
                                         </div>
                                     {:else}
                                         <div class="grid sm:grid-cols-2 gap-4">
                                              <div class="form-control space-y-1">
                                                <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{ut.callMode === "function" ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::arguments_python_expressions_one_per_line") : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::inputs_one_per_line")}</span>
                                                <textarea class="textarea textarea-bordered h-24 font-mono text-xs" value={getInputs(a)} on:input={(e) => setInputs(a, e.target.value)} placeholder={ut.callMode === "function" ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::example_args_1_2_3") : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::example_inputs_2_3")}></textarea>
                                             </div>
                                             <div class="form-control space-y-1">
                                                <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_value_python_expression")}</span>
                                                <textarea class="textarea textarea-bordered h-24 font-mono text-xs" value={getExpected(a)} on:input={(e) => setExpected(a, e.target.value)} placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::example_expect_7_or_hello")}></textarea>
                                             </div>
                                         </div>
                                     {/if}
                                </div>
                            {/each}
                         </div>
                         {#if ut.assertions.length === 0}
                            <div class="py-12 border-2 border-dashed border-base-300 rounded-xl flex flex-col items-center justify-center text-center px-4">
                                 <ShieldAlert size={32} class="opacity-20 mb-3" />
                                 <p class="text-xs font-semibold opacity-40 tracking-wide">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::no_assertions_added")}</p>
                            </div>
                         {/if}
                      </div>
                    </div>
                  {/each}
                  {#if utTests.length === 0}
                    <div class="py-16 border-2 border-dashed border-base-300 rounded-2xl flex flex-col items-center justify-center text-center px-4">
                         <div class="w-16 h-16 rounded-full bg-base-200 flex items-center justify-center mb-4"><FlaskConical size={32} class="opacity-20" /></div>
                         <p class="text-sm font-bold opacity-30">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::no_test_methods_yet")}</p>
                    </div>
                  {/if}
                </div>

                <!-- Preview Area -->
                {#if utShowPreview}
                  <div class="space-y-4 animate-in slide-in-from-bottom-4 duration-500">
                    <div class="flex items-center justify-between">
                      <div class="flex items-center gap-2">
                         <div class="w-2 h-2 rounded-full bg-success animate-pulse"></div>
                         <h4 class="font-bold text-xs uppercase tracking-widest opacity-60">
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::generated_python")}
                         </h4>
                      </div>
                      <div class="flex gap-2">
                        <button type="button" class="btn btn-ghost btn-xs font-bold gap-1" on:click={refreshPreview}>
                          <RotateCw size={12} /> {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::refresh")}
                        </button>
                        <button type="button" class="btn btn-ghost btn-xs font-bold gap-1" on:click={copyPreview}>
                          <Copy size={12} /> {copiedPreview ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::copied") : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::copy")}
                        </button>
                      </div>
                    </div>
                    <div class="rounded-xl overflow-hidden border border-base-300 shadow-inner">
                      <CodeMirror bind:value={utPreviewCode} lang={python()} readOnly={true} />
                    </div>
                  </div>
                {/if}

                <!-- Create Button -->
                <div class="flex justify-center pt-8">
                  <button
                    type="button"
                    class="btn btn-primary btn-lg px-12 font-bold shadow-xl shadow-primary/20 hover:scale-105 transition-all"
                    disabled={utTests.length === 0 || utOutputLoading || (utOutputMode === "teacher" && !teacherSolutionFile)}
                    on:click={uploadGeneratedUnitTests}
                  >
                    {#if utOutputLoading}
                      <span class="loading loading-spinner"></span>
                    {:else}
                      <FlaskConical size={20} />
                    {/if}
                    {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::create_tests")}
                  </button>
                </div>
            </div>
          </div>
        </div>

        <input
          type="radio"
          name="tests-tab"
          role="tab"
          class="tab"
          aria-label={translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_tests_builder",
          )}
        />
        <div
          role="tabpanel"
          class="tab-content bg-base-100 border-base-300 rounded-box p-4 space-y-4"
        >
          <div class="space-y-4">
            <div
              class="rounded-xl border border-base-300/70 bg-base-200/60 p-3 flex flex-wrap items-start justify-between gap-3"
            >
              <div class="space-y-1">
                <span class="section-label"
                  >{translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_output_source",
                  )}</span
                >
                <p class="hint">
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_expected_hint",
                  )}
                </p>
              </div>
              <div class="flex flex-wrap gap-2">
                <button
                  type="button"
                  class={`option-pill ${fnOutputMode === "manual" ? "selected" : ""}`}
                  aria-pressed={fnOutputMode === "manual"}
                  on:click={() => (fnOutputMode = "manual")}
                >
                  <span class="option-pill__indicator" aria-hidden="true" />
                  <span
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::output_source_manual",
                    )}</span
                  >
                </button>
                <button
                  type="button"
                  class={`option-pill ${fnOutputMode === "teacher" ? "selected" : ""}`}
                  aria-pressed={fnOutputMode === "teacher"}
                  on:click={() => (fnOutputMode = "teacher")}
                >
                  <span class="option-pill__indicator" aria-hidden="true" />
                  <span
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::output_source_teacher",
                    )}</span
                  >
                </button>
              </div>
            </div>
            {#if fnOutputMode === "teacher"}
              <div class="panel-soft space-y-2">
                <div class="flex items-center justify-between gap-2 flex-wrap">
                  <h5 class="font-semibold">
                    {translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_solution_file",
                    )}
                  </h5>
                  <span class="badge badge-outline badge-primary badge-sm"
                    >{teacherSolutionFile
                      ? translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::using_teacher_solution",
                          { name: teacherSolutionFile.name },
                        )
                      : translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::upload_teacher_solution_prompt",
                        )}</span
                  >
                </div>
                <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
                  <input
                    type="file"
                    accept=".py,.zip,.txt"
                    class="file-input file-input-bordered w-full"
                    on:change={(e) =>
                      (teacherSolutionFile =
                        (e.target as HTMLInputElement).files?.[0] || null)}
                  />
                  <span class="text-xs opacity-70">
                    {translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_solution_vm_hint",
                    )}
                  </span>
                </div>
                <p class="hint">
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_teacher_autofill_hint",
                  )}
                </p>
              </div>
            {/if}
            <div
              class="rounded-2xl border border-base-300/70 bg-base-200/40 p-4 space-y-3"
            >
              <div>
                <h4 class="font-semibold flex items-center gap-2">
                  <Code size={18} />
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::define_function_signature",
                  )}
                </h4>
                <p class="text-sm opacity-70">
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::write_python_function_header_to_describe_arguments",
                  )}
                </p>
              </div>
              <CodeMirror
                bind:value={fnSignature}
                lang={python()}
                readOnly={false}
                placeholder={translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_signature_placeholder",
                )}
              />
              {#if fnSignatureError}
                <div class="alert alert-error text-sm">{fnSignatureError}</div>
              {:else if fnMeta}
                <div class="flex flex-wrap gap-2 text-xs">
                  <span class="badge badge-outline badge-sm"
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_label",
                    )}
                    {fnMeta.name}</span
                  >
                  {#if fnMeta.params.length}
                    {#each fnMeta.params as p}
                      <span class="badge badge-outline badge-sm"
                        >{p.name}{p.type ? `: ${p.type}` : ""}</span
                      >
                    {/each}
                  {:else if !fnMeta.kwargs}
                    <span class="badge badge-outline badge-sm"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::no_arguments",
                      )}</span
                    >
                  {/if}
                  {#if fnMeta.kwargs}
                    <span class="badge badge-outline badge-sm"
                      >**{fnMeta.kwargs.name}{fnMeta.kwargs.type
                        ? `: ${fnMeta.kwargs.type}`
                        : ""}</span
                    >
                  {/if}
                  {#if fnMeta.returns.length === 0}
                    <span class="badge badge-outline badge-sm"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::returns_none",
                      )}</span
                    >
                  {:else}
                    {#each fnMeta.returns as r, ri}
                      <span class="badge badge-outline badge-sm"
                        >{fnMeta.returns.length > 1
                          ? translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::return_n",
                              { ri: ri + 1 },
                            )
                          : translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::return_single",
                            )}{r.type ? `: ${r.type}` : ""}</span
                      >
                    {/each}
                  {/if}
                </div>
              {/if}
            </div>
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div class="text-sm opacity-70">
                {fnCases.length}
                {fnCases.length === 1
                  ? translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::case_singular",
                    )
                  : translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::case_plural",
                    )}
              </div>
              <div class="flex items-center gap-2">
                <button
                  class="btn btn-outline"
                  on:click={addFnCase}
                  disabled={!fnMeta}
                  ><Plus size={16} />
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_case",
                  )}</button
                >
                <button
                  class="btn btn-primary"
                  disabled={!fnMeta || fnCases.length === 0 || fnOutputLoading ||
                    (fnOutputMode === "teacher" && !teacherSolutionFile)}
                  on:click={createFunctionTestsFromBuilder}
                  >
                  {#if fnOutputLoading}
                    <span class="loading loading-spinner loading-sm"></span>
                  {:else}
                    <FlaskConical size={16} />
                  {/if}
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::create_function_tests",
                  )}</button
                >
              </div>
            </div>
            {#if !fnMeta}
              <div
                class="rounded-xl border border-dashed border-base-300/80 p-6 text-center text-sm opacity-70"
              >
                {translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::define_function_signature_to_start_adding_test_cases",
                )}
              </div>
            {:else}
              <div class="space-y-3">
                {#each fnCases as fc, fi}
                  <div
                    class="rounded-2xl border border-base-300/70 bg-base-100 p-4 space-y-4 shadow-sm"
                  >
                    <div
                      class="flex flex-wrap items-center justify-between gap-3"
                    >
                      <input
                        class="input input-bordered w-full flex-1"
                        bind:value={fc.name}
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::case_n_placeholder",
                          { fi: fi + 1 },
                        )}
                      />
                      <div class="flex items-center gap-2 flex-wrap">
                        <label class="form-control w-32">
                          <span class="label-text text-xs"
                            >{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::time_limit_s_small",
                            )}</span
                          >
                          <input
                            class="input input-bordered w-full"
                            bind:value={fc.timeLimit}
                          />
                        </label>
                        {#if assignment?.grading_policy === "weighted"}
                          <label class="form-control w-28">
                            <span class="label-text text-xs"
                              >{translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::points_small",
                              )}</span
                            >
                            <input
                              class="input input-bordered w-full"
                              bind:value={fc.weight}
                            />
                          </label>
                        {/if}
                        <button
                          class="btn btn-ghost btn-xs"
                          on:click={() => removeFnCase(fi)}
                          ><Trash2 size={14} />
                          {translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::remove",
                          )}</button
                        >
                      </div>
                    </div>
                    {#if fnMeta.params.length}
                      <div class="space-y-2">
                        <h5 class="text-sm font-semibold">
                          {translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::arguments",
                          )}
                        </h5>
                        <div class="grid gap-3 md:grid-cols-2">
                          {#each fnMeta.params as param, pi}
                            {@const control = describeTypeControl(param.type)}
                            <div class="form-control space-y-1">
                              <span
                                class="label-text text-xs font-semibold uppercase tracking-wide flex items-center gap-2"
                              >
                                {param.name}
                                {#if param.type}
                                  <span class="badge badge-outline badge-sm"
                                    >{param.type}</span
                                  >
                                {/if}
                              </span>
                              {#if control.control === "boolean"}
                                <label
                                  class="label cursor-pointer justify-start gap-2 rounded-lg border border-base-300/60 px-3 py-2 bg-base-200/60"
                                >
                                  <input
                                    type="checkbox"
                                    class="toggle toggle-sm"
                                    checked={stringToBool(fc.args[pi])}
                                    on:change={(e) =>
                                      updateFnArg(
                                        fi,
                                        pi,
                                        (e.target as HTMLInputElement).checked
                                          ? "true"
                                          : "false",
                                      )}
                                  />
                                  <span class="label-text text-sm"
                                    >{stringToBool(fc.args[pi])
                                      ? translate(
                                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::true",
                                        )
                                      : translate(
                                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::false",
                                        )}</span
                                  >
                                </label>
                              {:else if control.control === "textarea"}
                                <textarea
                                  class="textarea textarea-bordered h-24"
                                  value={fc.args[pi]}
                                  on:input={(e) =>
                                    updateFnArg(
                                      fi,
                                      pi,
                                      (e.target as HTMLTextAreaElement).value,
                                    )}
                                  placeholder={param.type ??
                                    translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::value_placeholder",
                                    )}
                                ></textarea>
                              {:else}
                                <input
                                  class="input input-bordered w-full"
                                  type={control.control === "integer" ||
                                  control.control === "number"
                                    ? "number"
                                    : "text"}
                                  step={control.control === "integer"
                                    ? "1"
                                    : control.control === "number"
                                      ? "any"
                                      : undefined}
                                  value={fc.args[pi]}
                                  on:input={(e) =>
                                    updateFnArg(
                                      fi,
                                      pi,
                                      (e.target as HTMLInputElement).value,
                                    )}
                                  placeholder={param.type ??
                                    translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::value_placeholder",
                                    )}
                                />
                              {/if}
                            </div>
                          {/each}
                        </div>
                      </div>
                    {/if}
                    {#if fnMeta.kwargs}
                      <div class="space-y-2">
                        <div class="flex items-center justify-between gap-2">
                          <h5 class="text-sm font-semibold">
                            {translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::keyword_arguments",
                            )}
                          </h5>
                          <button
                            type="button"
                            class="btn btn-xs btn-outline"
                            on:click={() => addFnKwarg(fi)}
                          >
                            <Plus size={12} />
                            {translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_keyword_argument",
                            )}
                          </button>
                        </div>
                        <p class="hint">
                          {translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::keyword_args_builder_hint",
                          )}
                        </p>
                        {#if fc.kwargs.length === 0}
                          <div class="kwarg-empty">
                            {translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::no_keyword_arguments",
                            )}
                          </div>
                        {:else}
                          <div class="grid gap-2">
                            {#each fc.kwargs as kw, ki}
                              <div class="kwarg-card">
                                <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_auto] items-center">
                                  <label class="form-control w-full">
                                    <span class="label-text text-xs"
                                      >{translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::keyword_name",
                                      )}</span
                                    >
                                    <input
                                      class="input input-bordered w-full"
                                      value={kw.key}
                                      placeholder={translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::keyword_name_placeholder",
                                      )}
                                      on:input={(e) =>
                                        updateFnKwargKey(
                                          fi,
                                          ki,
                                          (e.target as HTMLInputElement).value,
                                        )}
                                    />
                                  </label>
                                  <label class="form-control w-full">
                                    <span class="label-text text-xs"
                                      >{translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::keyword_value",
                                      )}</span
                                    >
                                    <input
                                      class="input input-bordered w-full"
                                      value={kw.value}
                                      placeholder={translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::keyword_value_placeholder",
                                      )}
                                      on:input={(e) =>
                                        updateFnKwargValue(
                                          fi,
                                          ki,
                                          (e.target as HTMLInputElement).value,
                                        )}
                                    />
                                  </label>
                                  <button
                                    type="button"
                                    class="btn btn-ghost btn-xs"
                                    on:click={() => removeFnKwarg(fi, ki)}
                                  >
                                    <Trash2 size={14} />
                                    {translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::remove",
                                    )}
                                  </button>
                                </div>
                              </div>
                            {/each}
                          </div>
                        {/if}
                      </div>
                    {/if}
                    {#if fnMeta.returns.length > 0}
                      <div class="space-y-2">
                        <h5 class="text-sm font-semibold flex items-center gap-2">
                          {fnMeta.returns.length > 1
                            ? translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_return_values",
                              )
                            : translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_return_value",
                              )}
                          {#if fnOutputMode === "teacher"}
                            <span class="badge badge-ghost badge-sm"
                              >{translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::auto_from_teacher",
                              )}</span
                            >
                          {/if}
                        </h5>
                        {#if fnOutputMode === "teacher"}
                          <div
                            class="rounded-lg border border-dashed border-base-300/70 bg-base-200/50 px-4 py-3 text-xs opacity-80"
                          >
                            {translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_return_from_teacher",
                            )}
                          </div>
                        {:else}
                          <div class="grid gap-3 md:grid-cols-2">
                            {#each fnMeta.returns as ret, ri}
                              {@const control = describeTypeControl(ret.type)}
                              <div class="form-control space-y-1">
                                <span
                                  class="label-text text-xs font-semibold uppercase tracking-wide flex items-center gap-2"
                                >
                                  {fnMeta.returns.length > 1
                                    ? translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::return_n_capitalized",
                                        { ri: ri + 1 },
                                      )
                                    : translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::return_single_capitalized",
                                      )}
                                  {#if ret.type}
                                    <span class="badge badge-outline badge-sm"
                                      >{ret.type}</span
                                    >
                                  {/if}
                                </span>
                                {#if control.control === "boolean"}
                                  <label
                                    class="label cursor-pointer justify-start gap-2 rounded-lg border border-base-300/60 px-3 py-2 bg-base-200/60"
                                  >
                                    <input
                                      type="checkbox"
                                      class="toggle toggle-sm"
                                      checked={stringToBool(fc.returns[ri])}
                                      on:change={(e) =>
                                        updateFnReturn(
                                          fi,
                                          ri,
                                          (e.target as HTMLInputElement).checked
                                            ? "true"
                                            : "false",
                                        )}
                                    />
                                    <span class="label-text text-sm"
                                      >{stringToBool(fc.returns[ri])
                                        ? translate(
                                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::true",
                                          )
                                        : translate(
                                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::false",
                                          )}</span
                                    >
                                  </label>
                                {:else if control.control === "textarea"}
                                  <textarea
                                    class="textarea textarea-bordered h-24"
                                    value={fc.returns[ri]}
                                    on:input={(e) =>
                                      updateFnReturn(
                                        fi,
                                        ri,
                                        (e.target as HTMLTextAreaElement).value,
                                      )}
                                    placeholder={ret.type ??
                                      translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::value_placeholder",
                                      )}
                                  ></textarea>
                                {:else}
                                  <input
                                    class="input input-bordered w-full"
                                    type={control.control === "integer" ||
                                    control.control === "number"
                                      ? "number"
                                      : "text"}
                                    step={control.control === "integer"
                                      ? "1"
                                      : control.control === "number"
                                        ? "any"
                                        : undefined}
                                    value={fc.returns[ri]}
                                    on:input={(e) =>
                                      updateFnReturn(
                                        fi,
                                        ri,
                                        (e.target as HTMLInputElement).value,
                                      )}
                                    placeholder={ret.type ??
                                      translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::value_placeholder",
                                      )}
                                  />
                                {/if}
                              </div>
                            {/each}
                          </div>
                        {/if}
                      </div>
                    {:else}
                      <div
                        class="rounded-lg border border-dashed border-base-300/70 bg-base-200/50 px-4 py-3 text-xs opacity-70"
                      >
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::this_function_returns_none",
                        )}
                      </div>
                    {/if}
                  <div class="space-y-2">
                    <TestFileManager
                      bind:files={fc.files}
                      bind:selectedIndex={fc.selectedFileIndex}
                      bind:fileName={fc.fileName}
                      bind:fileText={fc.fileText}
                      bind:open={fc.showFile}
                      onError={(message) => (err = message)}
                    />
                  </div>
                  </div>
                {/each}
                {#if fnCases.length === 0}
                  <div
                    class="rounded-xl border border-dashed border-base-300/80 p-6 text-center text-sm opacity-70"
                  >
                    {translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::no_function_cases_yet",
                    )}
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        </div>

        <input
          type="radio"
          name="tests-tab"
          role="tab"
          class="tab"
          aria-label={translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_generate",
          )}
        />
        <div
          role="tabpanel"
          class="tab-content bg-base-100 border-base-300 rounded-box p-4 space-y-4"
        >
          <div class="group relative bg-base-100 rounded-2xl border border-base-300/50 shadow-sm hover:shadow-xl transition-all duration-300 overflow-hidden border-l-[6px] border-l-secondary">
            <!-- Premium Header -->
            <div class="p-8 bg-gradient-to-br from-secondary/10 via-transparent to-transparent border-b border-base-200">
              <div class="flex flex-col gap-6 md:flex-row md:items-center md:justify-between">
                <div class="flex items-center gap-6">
                  <div class="w-16 h-16 rounded-2xl bg-secondary/20 text-secondary flex items-center justify-center shadow-lg shadow-secondary/10 border border-secondary/20 transition-transform group-hover:scale-110 duration-500">
                    <Sparkles size={32} />
                  </div>
                  <div class="space-y-1">
                    <h3 class="text-3xl font-black tracking-tight">
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::generate_with_ai",
                      )}
                    </h3>
                    <p class="text-sm opacity-60 font-medium max-w-lg">
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_call_mode_hint",
                      )}
                    </p>
                  </div>
                </div>

                <div class="flex flex-wrap gap-2">
                  <span class="badge badge-lg bg-base-200 border-none font-bold gap-2 px-4 py-6 shadow-inner">
                    <Terminal size={14} class="text-secondary" />
                    <span class="opacity-50 text-[10px] uppercase tracking-wider">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::call_mode")}</span>
                    <span class="text-secondary">
                      {aiCallMode === "stdin"
                        ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::stdin_stdout_button")
                        : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::function_return_button")}
                    </span>
                  </span>
                  <span class="badge badge-lg bg-base-200 border-none font-bold gap-2 px-4 py-6 shadow-inner">
                    <Scale size={14} class="text-secondary" />
                    <span class="opacity-50 text-[10px] uppercase tracking-wider">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::task_difficulty")}</span>
                    <span class="text-secondary">
                      {aiDifficulty === "simple"
                        ? translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::simple_task_button")
                        : translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::hard_task_button")}
                    </span>
                  </span>
                </div>
              </div>
            </div>

            <div class="p-8 space-y-8">
              <div class="grid gap-8 xl:grid-cols-[1fr_0.8fr]">
                <!-- Left Column: Settings -->
                <div class="space-y-8">
                  <!-- Mode Selectors Card -->
                  <div class="card bg-base-200/30 border border-base-300/40 p-6 space-y-8">
                    <div class="grid gap-8 md:grid-cols-2">
                      <!-- Call Mode -->
                      <div class="space-y-3">
                        <h5 class="text-[10px] font-black uppercase tracking-widest opacity-60 flex items-center gap-2">
                          <Terminal size={14} class="text-secondary" />
                          {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::call_mode")}
                        </h5>
                        <div class="flex bg-base-300/50 p-1 rounded-xl w-full border border-white/5">
                          <button 
                            type="button"
                            class="flex-1 py-2 rounded-lg text-sm font-bold transition-all duration-200 {aiCallMode === 'stdin' ? 'bg-base-100 shadow-md scale-[1.02] text-secondary' : 'hover:bg-base-100/30 opacity-60'}"
                            on:click={() => (aiCallMode = "stdin")}
                          >
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::stdin_stdout_button")}
                          </button>
                          <button 
                            type="button"
                            class="flex-1 py-2 rounded-lg text-sm font-bold transition-all duration-200 {aiCallMode === 'function' ? 'bg-base-100 shadow-md scale-[1.02] text-secondary' : 'hover:bg-base-100/30 opacity-60'}"
                            on:click={() => (aiCallMode = "function")}
                          >
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::function_return_button")}
                          </button>
                        </div>
                      </div>

                      <!-- Difficulty -->
                      <div class="space-y-3">
                        <h5 class="text-[10px] font-black uppercase tracking-widest opacity-60 flex items-center gap-2">
                          <Scale size={14} class="text-secondary" />
                          {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::task_difficulty")}
                        </h5>
                        <div class="flex bg-base-300/50 p-1 rounded-xl w-full border border-white/5">
                          <button 
                            type="button"
                            class="flex-1 py-2 rounded-lg text-sm font-bold transition-all duration-200 {aiDifficulty === 'simple' ? 'bg-base-100 shadow-md scale-[1.02] text-secondary' : 'hover:bg-base-100/30 opacity-60'}"
                            on:click={() => (aiDifficulty = "simple")}
                          >
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::simple_task_button")}
                          </button>
                          <button 
                            type="button"
                            class="flex-1 py-2 rounded-lg text-sm font-bold transition-all duration-200 {aiDifficulty === 'hard' ? 'bg-base-100 shadow-md scale-[1.02] text-secondary' : 'hover:bg-base-100/30 opacity-60'}"
                            on:click={() => (aiDifficulty = "hard")}
                          >
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::hard_task_button")}
                          </button>
                        </div>
                      </div>
                    </div>

                    <!-- Test Count Mode -->
                    <div class="space-y-4 pt-4 border-t border-base-300/50">
                      <h5 class="text-[10px] font-black uppercase tracking-widest opacity-60 flex items-center gap-2">
                        <Cpu size={14} class="text-secondary" />
                        {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::test_count_mode")}
                      </h5>
                      <div class="flex flex-wrap gap-4 items-center">
                        <div class="flex bg-base-300/50 p-1 rounded-xl w-fit border border-white/5">
                          <button 
                            type="button"
                            class="px-6 py-2 rounded-lg text-sm font-bold transition-all duration-200 {aiAuto ? 'bg-base-100 shadow-md scale-[1.02] text-secondary' : 'hover:bg-base-100/30 opacity-60'}"
                            on:click={() => (aiAuto = true)}
                          >
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::auto")}
                          </button>
                          <button 
                            type="button"
                            class="px-6 py-2 rounded-lg text-sm font-bold transition-all duration-200 {!aiAuto ? 'bg-base-100 shadow-md scale-[1.02] text-secondary' : 'hover:bg-base-100/30 opacity-60'}"
                            on:click={() => (aiAuto = false)}
                          >
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::manual")}
                          </button>
                        </div>

                        {#if !aiAuto}
                          <div class="flex items-center gap-3 animate-in fade-in slide-in-from-left-2 duration-300">
                            <ArrowRight size={14} class="opacity-40" />
                            <input
                              type="number"
                              min="1"
                              max="50"
                              class="input input-bordered w-24 font-bold text-center bg-base-100"
                              bind:value={aiNumTests}
                            />
                            <span class="text-xs font-bold opacity-40 uppercase tracking-widest">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::test_plural")}</span>
                          </div>
                        {/if}
                      </div>
                    </div>
                  </div>

                  <!-- Instructions Card -->
                  <div class="card bg-base-100 border border-base-300/50 shadow-sm p-6 space-y-4">
                    <h5 class="text-[10px] font-black uppercase tracking-widest opacity-60 flex items-center gap-2">
                      <Code size={14} class="text-secondary" />
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::additional_instructions_optional")}
                    </h5>
                    <textarea
                      class="textarea textarea-bordered w-full min-h-[140px] bg-base-200/30 focus:bg-base-100 transition-all leading-relaxed"
                      bind:value={aiInstructions}
                      placeholder={translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::edge_cases_to_cover_placeholder")}
                    ></textarea>
                    <p class="text-[10px] opacity-50 italic flex items-center gap-2">
                       <Shield size={12} />
                       {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_instructions_hint")}
                    </p>
                  </div>
                </div>

                <!-- Right Column: Teacher Solution -->
                <div class="space-y-6">
                  <div class="card bg-secondary/5 border border-dashed border-secondary/30 p-8 space-y-6 flex flex-col items-center text-center">
                    <div class="w-16 h-16 rounded-full bg-secondary/10 text-secondary flex items-center justify-center shadow-inner">
                      <FileCode2 size={24} />
                    </div>
                    <div class="space-y-2">
                      <h4 class="font-bold text-lg">
                        {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_label")}
                      </h4>
                      <p class="text-xs opacity-60 leading-relaxed max-w-xs mx-auto">
                        {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_hint")}
                      </p>
                    </div>

                    <label class="w-full cursor-pointer group/file">
                      <div class="flex flex-col items-center justify-center p-6 border-2 border-dashed border-secondary/20 rounded-2xl bg-base-100/50 group-hover/file:border-secondary/50 group-hover/file:bg-secondary/5 transition-all duration-300">
                        <UploadIcon size={24} class="mb-2 text-secondary/40 group-hover/file:text-secondary group-hover/file:scale-110 transition-all" />
                        <span class="text-sm font-bold opacity-60">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::upload_teacher_solution")}</span>
                        <input
                          id="ai-solution-upload"
                          type="file"
                          accept=".py,.txt"
                          class="hidden"
                          on:change={handleAISolutionChange}
                        />
                      </div>
                    </label>

                    {#if aiSolutionError}
                      <div class="badge badge-error gap-2 py-3">{aiSolutionError}</div>
                    {/if}

                    {#if aiSolutionText.trim().length}
                      <div class="w-full space-y-4 animate-in fade-in slide-in-from-top-4 duration-500">
                        <div class="flex items-center justify-between px-2">
                          <div class="flex items-center gap-2 text-xs font-bold text-secondary">
                             <Check size={14} />
                             {aiSolutionFile?.name || translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_manual")}
                             <span class="opacity-40 text-[10px] ml-2 font-mono">{aiSolutionText.length}B</span>
                          </div>
                          <button
                            type="button"
                            class="btn btn-ghost btn-xs text-error hover:bg-error/10"
                            on:click={resetAISolutionInput}
                          >
                             {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_clear")}
                          </button>
                        </div>
                        <div class="rounded-xl overflow-hidden border border-secondary/20 shadow-lg">
                          <CodeMirror
                            bind:value={aiSolutionText}
                            lang={python()}
                            readOnly={false}
                          />
                        </div>
                      </div>
                    {/if}
                  </div>

                  <!-- Run Tests on Solution Button (only if generated something) -->
                  {#if (aiCode && aiCode.trim().length) || hasAIBuilder}
                    <div class="card bg-base-100 border border-base-300/50 shadow-sm p-6 space-y-4 animate-in zoom-in-95 duration-300">
                      <div class="flex items-center justify-between">
                         <h5 class="text-[10px] font-black uppercase tracking-widest opacity-60">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::optional_test_on_teacher_solution")}</h5>
                         <span class="badge badge-sm badge-outline opacity-40">Teacher Sim</span>
                      </div>
                      <div class="grid gap-3">
                        <input
                          type="file"
                          accept=".py,.zip"
                          class="file-input file-input-bordered file-input-sm w-full bg-base-200/50"
                          on:change={(e) => (teacherSolutionFile = (e.target as HTMLInputElement).files?.[0] || null)}
                        />
                        <button
                          class="btn btn-secondary btn-sm font-black shadow-lg shadow-secondary/10"
                          disabled={!teacherSolutionFile || teacherRunLoading}
                          on:click={runTeacherSolution}
                        >
                          {#if teacherRunLoading}
                            <RotateCw size={14} class="animate-spin" />
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::running")}
                          {:else}
                            <FlaskConical size={14} />
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::run_tests_on_solution")}
                          {/if}
                        </button>
                      </div>

                      {#if teacherRun}
                         <div class="mt-4 space-y-2 max-h-48 overflow-y-auto pr-2 custom-scrollbar">
                           {#each teacherRun.results as r}
                             <div class="flex items-center justify-between p-2 rounded-lg bg-base-200/50 border border-base-300/30 text-[10px]">
                               <div class="flex items-center gap-2 truncate">
                                  <span class="badge {r.status === 'passed' ? 'badge-success' : 'badge-error'} badge-xs"></span>
                                  <span class="font-mono opacity-70 truncate">{r.unittest_name || 'Test Case'}</span>
                               </div>
                               <span class="font-bold {r.status === 'passed' ? 'text-success' : 'text-error'} uppercase tracking-tight">{r.status}</span>
                             </div>
                           {/each}
                         </div>
                      {/if}
                    </div>
                  {/if}
                </div>
              </div>

              <!-- Action Bar -->
              <div class="flex flex-col gap-4 border-t border-base-300/50 pt-8 mt-4">
                <div class="flex flex-col sm:flex-row gap-4">
                  <button
                    class="btn btn-secondary flex-1 h-14 text-lg font-black shadow-xl shadow-secondary/20 hover:scale-[1.02] transition-all group"
                    on:click={generateWithAI}
                    disabled={aiGenerating}
                  >
                    {#if aiGenerating}
                      <RotateCw size={20} class="animate-spin" />
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::generating")}
                    {:else}
                      <Wand2 size={20} class="group-hover:rotate-12 transition-transform" />
                      {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::generate_with_ai")}
                    {/if}
                  </button>
                  
                  <button
                    class="btn btn-outline border-2 flex-1 h-14 text-lg font-black hover:bg-primary hover:text-primary-content hover:border-primary transition-all duration-300"
                    on:click={uploadAIUnitTestsCode}
                    disabled={builderMode !== "unittest" || !aiCode}
                  >
                    <Save size={20} />
                    {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::save_as_tests")}
                  </button>
                </div>

                {#if aiGenerating}
                   <div class="flex items-center justify-center gap-3 text-secondary font-bold animate-pulse">
                      <div class="loading loading-dots loading-md"></div>
                      <span class="text-sm uppercase tracking-[0.2em]">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::generating")}</span>
                   </div>
                {/if}

                {#if hasAIBuilder}
                  <div class="flex items-center gap-3 p-4 rounded-xl bg-primary/10 border border-primary/20 text-primary animate-in slide-in-from-bottom-2 duration-500">
                    <Check size={18} />
                    <span class="text-sm font-bold">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_prepared_builder_structure_below")}</span>
                  </div>
                {/if}
              </div>

              <!-- Generated Preview Section -->
              {#if aiCode && builderMode === "unittest"}
                <div class="space-y-6 pt-8 border-t border-base-300/50 animate-in fade-in slide-in-from-bottom-4 duration-500">
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-3">
                      <div class="w-10 h-10 rounded-xl bg-success/10 text-success flex items-center justify-center">
                        <Code size={20} />
                      </div>
                      <div>
                        <h4 class="font-bold text-lg">
                          {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_python_editable")}
                        </h4>
                        <p class="text-[10px] opacity-40 font-black uppercase tracking-widest">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::preview_badge")}</p>
                      </div>
                    </div>
                    <div class="badge badge-success font-bold px-4 py-3">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::save_as_tests")}</div>
                  </div>
                  <div class="rounded-2xl overflow-hidden border border-base-300/50 shadow-2xl">
                    <CodeMirror
                      bind:value={aiCode}
                      lang={python()}
                      readOnly={false}
                    />
                  </div>
                </div>
              {/if}
            </div>
          </div>
        </div>

        <input
          type="radio"
          name="tests-tab"
          role="tab"
          class="tab"
          aria-label={translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::upload_py",
          )}
        />
        <div
          role="tabpanel"
          class="tab-content bg-base-100 border-base-300 rounded-box p-4"
        >
          <h4 class="font-semibold mb-2 flex items-center gap-2">
            <FileUp size={18} />
            {translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::upload_unittest_file",
            )}
          </h4>
          <input
            type="file"
            accept=".py"
            class="file-input file-input-bordered w-full"
            on:change={(e) =>
              (unittestFile =
                (e.target as HTMLInputElement).files?.[0] || null)}
          />
          <div class="mt-2">
            <button
              class="btn"
              on:click={uploadUnitTests}
              disabled={!unittestFile}
              ><UploadIcon size={16} />
              {translate(
                "frontend/src/routes/assignments/[id]/tests/+page.svelte::upload",
              )}</button
            >
          </div>
          <div class="mt-4 space-y-3 text-sm opacity-90 border-t border-base-300 pt-4">
            <p class="font-bold">{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_upload_comprehensive_intro")}</p>
            <ul class="list-disc list-inside space-y-1 opacity-80">
               <li>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_upload_comprehensive_item_1")}</li>
               <li>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_upload_comprehensive_item_2")}</li>
               <li>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_upload_comprehensive_item_3")}</li>
               <li>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_upload_comprehensive_item_4")}</li>
            </ul>
            <p class="font-bold text-xs mt-4 uppercase opacity-50 tracking-wider">
               {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::example_label")}:
            </p>
            <pre class="bg-base-300/30 p-6 rounded-2xl text-[13px] leading-relaxed font-mono overflow-x-auto whitespace-pre border border-white/5 shadow-inner">
<span class="text-purple-400">import</span> unittest

<span class="text-purple-400">class</span> <span class="text-blue-400">MyTests</span>(unittest.TestCase):
    <span class="text-purple-400">def</span> <span class="text-blue-400">test_hello</span>(self):
        <span class="opacity-40"># Run student's program with multiple inputs</span>
        output = student_code(<span class="text-emerald-400">"John"</span>, <span class="text-emerald-400">"Doe"</span>)
        self.assertIn(<span class="text-emerald-400">"John"</span>, output)
        self.assertIn(<span class="text-emerald-400">"Doe"</span>, output)

    <span class="text-purple-400">def</span> <span class="text-blue-400">test_add</span>(self):
        <span class="opacity-40"># Import and call specific function</span>
        add = student_function(<span class="text-emerald-400">"add"</span>)
        self.assertEqual(add(<span class="text-orange-300">2</span>, <span class="text-orange-300">3</span>), <span class="text-orange-300">5</span>)</pre>
          </div>

        </div>
      </div>
    {/if}
  </div>
{/if}

{#if err}
  <div class="alert alert-error mt-4"><span>{err}</span></div>
{/if}

<dialog bind:this={editDialog} class="modal">
  <div class="modal-box w-11/12 max-w-4xl space-y-3">
    <h3 class="font-semibold">
      {translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::edit_unittest_method",
      )}
    </h3>
    <p class="text-xs opacity-70">{editingTest?.unittest_name}</p>
    <CodeMirror
      bind:value={editingMethodCode}
      lang={python()}
      readOnly={false}
    />
    <div class="modal-action">
      <button class="btn" on:click={saveEditUnitTest}
        ><Save size={16} />
        {translate(
          "frontend/src/routes/assignments/[id]/tests/+page.svelte::save",
        )}</button
      >
      <form method="dialog">
        <button class="btn btn-ghost" on:click={closeEditUnitTest}
          >{translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::close",
          )}</button
        >
      </form>
    </div>
  </div>
</dialog>

<dialog
  bind:this={saveModal}
  class="modal"
  on:close={() => {
    if (!undoLoading) undoPending = null;
  }}
>
  <div class="modal-box space-y-3">
    <h3 class="font-semibold">
      {translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::tests_saved_title",
      )}
    </h3>
    <p class="text-sm opacity-80">
      {translate(
        "frontend/src/routes/assignments/[id]/tests/+page.svelte::tests_saved_body",
      )}
    </p>
    <div class="modal-action">
      {#if undoPending && (undoPending.testIds.length > 0 || undoPending.snapshot)}
        <button
          class="btn btn-outline"
          on:click={undoLastSave}
          disabled={undoLoading}
        >
          {translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::undo_save_button",
          )}
        </button>
      {/if}
      <form method="dialog">
        <button class="btn" disabled={undoLoading}
          >{translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::close",
          )}</button
        >
      </form>
    </div>
  </div>
</dialog>

<ConfirmModal bind:this={confirmModal} />

<style>
  :global(.panel) {
    position: relative;
    border-radius: 1.5rem;
    border: 1px solid color-mix(in oklab, oklch(var(--bc)) 18%, transparent);
    background: color-mix(
      in oklab,
      var(--fallback-b1, oklch(var(--b1))) 88%,
      transparent
    );
    padding: 1.5rem;
    box-shadow: 0 24px 40px rgba(15, 23, 42, 0.08);
    backdrop-filter: blur(12px);
  }
  :global(.ai-hero) {
    overflow: hidden;
  }
  :global(.ai-hero::before) {
    content: "";
    position: absolute;
    inset: 0;
    background: radial-gradient(
        120% 150% at 10% 10%,
        color-mix(in oklab, oklch(var(--p)) 28%, transparent) 0%,
        transparent 60%
      ),
      radial-gradient(
        110% 140% at 90% 20%,
        color-mix(in oklab, oklch(var(--s)) 20%, transparent) 0%,
        transparent 55%
      );
    opacity: 0.65;
    pointer-events: none;
  }
  :global(.ai-hero > *) {
    position: relative;
  }
  :global(.ai-hero__icon) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 2.75rem;
    height: 2.75rem;
    border-radius: 999px;
    background: color-mix(in oklab, oklch(var(--p)) 22%, oklch(var(--b1)) 78%);
    color: color-mix(in oklab, oklch(var(--nc)) 90%, oklch(var(--p)) 10%);
    box-shadow: 0 12px 24px rgba(15, 23, 42, 0.12);
  }
  :global(.stat-chip) {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.4rem 0.75rem;
    border-radius: 999px;
    border: 1px solid color-mix(in oklab, oklch(var(--bc)) 14%, transparent);
    background: color-mix(
      in oklab,
      var(--fallback-b1, oklch(var(--b1))) 75%,
      transparent
    );
  }
  :global(.stat-chip strong) {
    font-weight: 600;
  }
  :global(.section-label) {
    display: inline-flex;
    font-size: 0.75rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: color-mix(in oklab, oklch(var(--bc)) 60%, transparent);
  }
  :global(.hint) {
    font-size: 0.75rem;
    color: color-mix(in oklab, oklch(var(--bc)) 72%, transparent);
  }
  :global(.option-pill) {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.55rem 0.95rem;
    border-radius: 999px;
    border: 1.5px solid color-mix(in oklab, oklch(var(--bc)) 22%, transparent);
    background: color-mix(
      in oklab,
      var(--fallback-b2, oklch(var(--b2))) 78%,
      transparent
    );
    color: color-mix(in oklab, oklch(var(--bc)) 80%, oklch(var(--nc)) 20%);
    position: relative;
    isolation: isolate;
    font-size: 0.85rem;
    font-weight: 600;
    transition:
      transform 0.18s ease,
      box-shadow 0.18s ease,
      border-color 0.18s ease,
      background 0.18s ease,
      color 0.18s ease;
  }
  :global(.option-pill:not(.selected)) {
    opacity: 0.68;
  }
  :global(.option-pill__indicator) {
    width: 0.8rem;
    height: 0.8rem;
    border-radius: 999px;
    background: color-mix(in oklab, oklch(var(--bc)) 60%, transparent);
    box-shadow: inset 0 0 0 2px
      color-mix(in oklab, oklch(var(--b1)) 82%, transparent);
    transition:
      transform 0.18s ease,
      background 0.18s ease,
      box-shadow 0.18s ease;
  }
  :global(.option-pill:hover) {
    transform: translateY(-2px);
    box-shadow: 0 12px 26px rgba(15, 23, 42, 0.08);
  }
  :global(.kwarg-card) {
    border-radius: 0.9rem;
    border: 1px solid color-mix(in oklab, oklch(var(--bc)) 18%, transparent);
    background: color-mix(
      in oklab,
      var(--fallback-b1, oklch(var(--b1))) 85%,
      transparent
    );
    padding: 0.75rem;
    box-shadow: 0 10px 18px rgba(15, 23, 42, 0.06);
  }
  :global(.kwarg-empty) {
    border-radius: 0.9rem;
    border: 1px dashed
      color-mix(in oklab, oklch(var(--bc)) 30%, transparent);
    background: color-mix(
      in oklab,
      var(--fallback-b2, oklch(var(--b2))) 70%,
      transparent
    );
    padding: 0.75rem;
    font-size: 0.75rem;
    color: color-mix(in oklab, oklch(var(--bc)) 68%, transparent);
  }
  :global(.option-pill:focus-visible) {
    outline: 2px solid color-mix(in oklab, oklch(var(--p)) 35%, transparent);
    outline-offset: 2px;
  }
  :global(.option-pill.selected) {
    transform: translateY(-1px) scale(1.05);
    background: linear-gradient(
      135deg,
      color-mix(
          in oklab,
          oklch(var(--p)) 86%,
          var(--fallback-b1, oklch(var(--b1))) 14%
        )
        0%,
      color-mix(
          in oklab,
          oklch(var(--s)) 68%,
          var(--fallback-b1, oklch(var(--b1))) 32%
        )
        52%,
      color-mix(
          in oklab,
          oklch(var(--p)) 80%,
          var(--fallback-b1, oklch(var(--b1))) 20%
        )
        100%
    );
    border-color: color-mix(in oklab, oklch(var(--p)) 92%, transparent);
    color: color-mix(in oklab, oklch(var(--nc)) 98%, oklch(var(--p)) 2%);
    box-shadow:
      0 0 0 3px color-mix(in oklab, oklch(var(--p)) 65%, transparent),
      0 18px 34px rgba(15, 23, 42, 0.25),
      0 0 52px color-mix(in oklab, oklch(var(--p)) 40%, transparent);
    opacity: 1;
    animation: option-pill-glow 2.4s ease-in-out infinite;
  }
  :global(.option-pill.selected::before) {
    content: "";
    position: absolute;
    inset: -6px;
    border-radius: inherit;
    background: radial-gradient(
      70% 70% at 50% 50%,
      color-mix(in oklab, oklch(var(--p)) 58%, transparent) 0%,
      transparent 100%
    );
    opacity: 0.95;
    z-index: -1;
    filter: blur(12px);
  }
  :global(.option-pill.selected::after) {
    content: "";
    position: absolute;
    inset: 0;
    border-radius: inherit;
    background: linear-gradient(
      135deg,
      color-mix(in oklab, oklch(var(--p)) 85%, transparent) 0%,
      color-mix(in oklab, oklch(var(--s)) 65%, transparent) 100%
    );
    opacity: 0.38;
    z-index: -1;
  }
  :global(.option-pill.selected .option-pill__indicator) {
    transform: scale(1.2);
    background: color-mix(in oklab, oklch(var(--n)) 10%, oklch(var(--p)) 80%);
    box-shadow:
      0 0 0 2px color-mix(in oklab, oklch(var(--p)) 65%, transparent),
      inset 0 0 0 2px color-mix(in oklab, oklch(var(--nc)) 92%, transparent);
  }
  @keyframes option-pill-glow {
    0%,
    100% {
      box-shadow:
        0 0 0 3px color-mix(in oklab, oklch(var(--p)) 65%, transparent),
        0 18px 34px rgba(15, 23, 42, 0.25),
        0 0 52px color-mix(in oklab, oklch(var(--p)) 40%, transparent);
    }
    50% {
      box-shadow:
        0 0 0 5px color-mix(in oklab, oklch(var(--p)) 75%, transparent),
        0 22px 40px rgba(15, 23, 42, 0.28),
        0 0 68px color-mix(in oklab, oklch(var(--p)) 48%, transparent);
    }
  }
  :global(.panel-soft) {
    border-radius: 1.1rem;
    border: 1px solid color-mix(in oklab, oklch(var(--bc)) 16%, transparent);
    background: color-mix(
      in oklab,
      var(--fallback-b1, oklch(var(--b1))) 70%,
      color-mix(in oklab, oklch(var(--p)) 12%, transparent) 30%
    );
    padding: 1rem 1.1rem;
  }
  :global(.result-item) {
    border-radius: 0.85rem;
    border: 1px solid color-mix(in oklab, oklch(var(--bc)) 18%, transparent);
    background: color-mix(
      in oklab,
      var(--fallback-b1, oklch(var(--b1))) 82%,
      transparent
    );
    padding: 0.75rem 0.9rem;
  }
  :global(.result-log) {
    margin-top: 0.45rem;
    font-size: 0.75rem;
    line-height: 1.4;
    white-space: pre-wrap;
    opacity: 0.8;
  }
  :global(.card-elevated) {
    border-radius: 1rem;
    border: 1px solid color-mix(in oklab, currentColor 20%, transparent);
    background: var(--fallback-b1, oklch(var(--b1)));
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.06);
  }
  :global(.ut-method) {
    border-color: color-mix(
      in oklab,
      oklch(var(--p)) 28%,
      oklch(var(--bc)) 72%
    );
    background: color-mix(
      in oklab,
      var(--fallback-b1, oklch(var(--b1))) 94%,
      oklch(var(--p)) 6%
    );
  }
  :global(.ut-method-header) {
    padding: 0.25rem 0.25rem;
    border-radius: 0.5rem;
    background: color-mix(in oklab, oklch(var(--p)) 10%, transparent);
  }
  :global(.ut-assertions) {
    position: relative;
    margin-left: 0.25rem;
    padding-left: 0.75rem;
    border-left: 3px solid oklch(var(--p));
    background: color-mix(in oklab, transparent 92%, oklch(var(--p)) 8%);
    border-radius: 0.5rem;
    padding-top: 0.5rem;
    padding-bottom: 0.5rem;
  }
  :global(.ut-assertion-item) {
    background: color-mix(
      in oklab,
      var(--fallback-b1, oklch(var(--b1))) 90%,
      oklch(var(--p)) 10%
    );
    border-color: color-mix(
      in oklab,
      oklch(var(--p)) 40%,
      oklch(var(--bc)) 60%
    );
  }
</style>

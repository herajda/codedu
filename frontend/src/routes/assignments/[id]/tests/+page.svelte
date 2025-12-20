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
    Upload as UploadIcon,
  } from "lucide-svelte";
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
  let ioFileBase64 = "";
  let ioFileUpload: File | null = null;
  let showIOFile = false;


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
  type FnCase = {
    name: string;
    args: string[];
    returns: string[];
    weight: string;
    timeLimit: string;
    fileName: string;
    fileText: string;
    fileBase64: string;
    showFile?: boolean;
  };
  type FnMeta = { name: string; params: FnParameter[]; returns: FnReturn[] };

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

  function textToBase64(text: string): string {
    const enc = new TextEncoder();
    const bytes = enc.encode(text ?? "");
    let binary = "";
    bytes.forEach((b) => {
      binary += String.fromCharCode(b);
    });
    return btoa(binary);
  }

  function readFileBase64(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        const res = String(reader.result || "");
        const parts = res.split(",");
        resolve(parts.length > 1 ? parts[1] : res);
      };
      reader.onerror = () => reject(reader.error ?? new Error("read failed"));
      reader.readAsDataURL(file);
    });
  }

  function readFileText(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result || ""));
      reader.onerror = () => reject(reader.error ?? new Error("read failed"));
      reader.readAsText(file);
    });
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
      const filePayload = buildFilePayload(
        ioFileName,
        ioFileText,
        ioFileBase64,
      );
      let expectedStdout = tStdout;
      if (ioOutputMode === "teacher") {
        ioOutputLoading = true;
        const timeLimit = parseFloat(tLimit);
        const { previews } = await runTeacherPreview([
          {
            execution_mode: "stdin_stdout",
            stdin: tStdin,
            expected_stdout: "",
            time_limit_sec: Number.isFinite(timeLimit) ? timeLimit : undefined,
            ...(filePayload ?? {}),
          },
        ]);
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
      ioFileBase64 = "";
      ioFileUpload = null;
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
    if (paramsRaw.trim()) {
      const pieces = splitTopLevel(paramsRaw);
      for (const piece of pieces) {
        const part = piece.trim();
        if (!part) continue;
        if (part.startsWith("*")) {
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
    return { meta: { name, params, returns }, error: "" };
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
      returns: adjustedReturns,
      weight: c.weight,
      timeLimit: c.timeLimit,
      fileName: c.fileName ?? "",
      fileText: c.fileText ?? "",
      fileBase64: c.fileBase64 ?? "",
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

  function casesEqual(a: FnCase, b: FnCase): boolean {
    return (
      a.name === b.name &&
      arraysEqual(a.args, b.args) &&
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
      returns,
      weight: defaultWeight,
      timeLimit: "1",
      fileName: "",
      fileText: "",
      fileBase64: "",
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
        buildFilePayload(c.fileName, c.fileText, c.fileBase64),
      );
      let teacherExpected: string[] | null = null;
      if (fnOutputMode === "teacher") {
        fnOutputLoading = true;
        const previewPayloads = fnCases.map((c, idx) => {
          const argsValues = fnMeta.params.map((p, idx) =>
            coerceValueForType(c.args[idx] ?? "", p.type),
          );
          const timeLimit = parseFloat(c.timeLimit);
          return {
            execution_mode: "function",
            function_name: fnMeta.name,
            function_args: JSON.stringify(argsValues),
            function_kwargs: "{}",
            expected_return: "",
            time_limit_sec: Number.isFinite(timeLimit) ? timeLimit : undefined,
            ...(filePayloads[idx] ?? {}),
          };
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
          function_kwargs: "{}",
          expected_return: expectedJSON,
          stdin: "",
          expected_stdout: "",
          time_limit_sec: parseFloat(c.timeLimit) || undefined,
        };
        if (filePayloads[idx]) {
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
    const expectedSample = sampleCase?.expected;
    const argNames = argsSample.map(
      (_, idx) => `arg${idx + 1}: ${guessTypeFromValue(argsSample[idx])}`,
    );
    const argSection = argNames.join(", ");
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
    const expectedRaw = rc?.expected;
    let args: string[] = rawArgs.map((v) => formatValueForInput(v));
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
      returns,
      weight,
      timeLimit,
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
              previewPayloads.push({
                execution_mode: "function",
                function_name: fn,
                function_args: argsJSON,
                function_kwargs: "{}",
                expected_return: "",
                time_limit_sec: Number.isFinite(timeLimit)
                  ? timeLimit
                  : undefined,
              });
              indexMap.push({ ti, ai, mode: "function" });
            } else {
              const stdinVal = ((a as any).args ?? []).join("\n");
              previewPayloads.push({
                execution_mode: "stdin_stdout",
                stdin: stdinVal,
                expected_stdout: "",
                time_limit_sec: Number.isFinite(timeLimit)
                  ? timeLimit
                  : undefined,
              });
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
        testData.function_args =
          typeof t.function_args === "string" ? t.function_args : "";
        testData.function_kwargs =
          typeof t.function_kwargs === "string" ? t.function_kwargs : "";
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
      <div class="alert">
        <div>
          <span class="font-medium"
            >{translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::tip",
            )}</span
          >
          {translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::assertion_helper_text",
          )}
        </div>
      </div>

      <!-- Tabs -->
      <div role="tablist" class="tabs tabs-lifted">
        <input
          type="radio"
          name="tests-tab"
          role="tab"
          class="tab"
          aria-label={translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::existing_tests",
          )}
          checked
        />
        <div
          role="tabpanel"
          class="tab-content bg-base-100 border-base-300 rounded-box p-4"
        >
          <div class="flex items-center justify-between mb-2">
            <div class="text-sm opacity-70">
              {tests?.length || 0}
              {tests?.length === 1
                ? translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_singular",
                  )
                : translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_plural",
                  )}
            </div>
            <button
              class="btn btn-error btn-sm"
              on:click={deleteAllTests}
              disabled={!tests || tests.length === 0}
              ><Trash2 size={14} />
              {translate(
                "frontend/src/routes/assignments/[id]/tests/+page.svelte::delete_all",
              )}</button
            >
          </div>
          <div class="grid gap-3 max-h-[32rem] overflow-y-auto">
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
                  <div class="flex gap-2">
                    {#if mode === "unittest" && hasUnittestCode}
                      <button
                        class="btn btn-xs"
                        on:click={() => openEditUnitTest(t)}
                        ><Code size={14} />
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::edit",
                        )}</button
                      >
                    {/if}
                    <button class="btn btn-xs" on:click={() => updateTest(t)}
                      ><Save size={14} />
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::save",
                      )}</button
                    >
                    <button
                      class="btn btn-xs btn-error"
                      on:click={() => delTest(t.id)}
                      ><Trash2 size={14} />
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::delete",
                      )}</button
                    >
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
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_name_example_multiply",
                        )}
                        bind:value={t.function_name}
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
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_6",
                        )}
                        bind:value={t.expected_return}
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
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_2_3",
                        )}
                        bind:value={t.function_args}
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
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_round_2",
                        )}
                        bind:value={t.function_kwargs}
                      ></textarea>
                    </label>
                  </div>
                {:else if t.unittest_name}
                  {#if t.unittest_code}
                    <details class="mt-1">
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
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::stdin",
                        )}
                        bind:value={t.stdin}
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
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_stdout",
                        )}
                        bind:value={t.expected_stdout}
                      ></textarea>
                    </label>
                  </div>
                {/if}
                {#if mode === "function" || mode === "stdin_stdout"}
                  <div
                    class="rounded-xl border border-dashed border-base-300/70 bg-base-200/40 p-3 space-y-2"
                  >
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-sm font-semibold"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_optional",
                        )}</span
                      >
                      {#if t.file_name}
                        <span class="badge badge-outline badge-sm"
                          >{t.file_name}</span
                        >
                      {/if}
                    </div>
                    <div class="grid gap-2 sm:grid-cols-2">
                      <label class="form-control w-full space-y-1">
                        <span class="label-text"
                          >{translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_upload",
                          )}</span
                        >
                        <input
                          type="file"
                          class="file-input file-input-bordered w-full"
                          on:change={async (e) => {
                            const file =
                              (e.target as HTMLInputElement).files?.[0] || null;
                            if (!file) return;
                            try {
                              t.file_base64 = await readFileBase64(file);
                              t.file_name = file.name;
                              t.file_create_name = "";
                              t.file_create_text = "";
                              t.file_create_dirty = false;
                              tests = [...tests];
                            } catch (e: any) {
                              err =
                                e?.message ||
                                translate(
                                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::file_read_error",
                                );
                            }
                          }}
                        />
                      </label>
                      <label class="form-control w-full space-y-1">
                        <span class="label-text"
                          >{translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_name",
                          )}</span
                        >
                        <input
                          class="input input-bordered w-full"
                          placeholder="data.txt"
                          value={t.file_create_name ?? ""}
                          on:input={(e) => {
                            t.file_create_name = (
                              e.target as HTMLInputElement
                            ).value;
                            t.file_create_dirty = true;
                          }}
                        />
                      </label>
                    </div>
                    <label class="form-control w-full space-y-1">
                      <span class="label-text"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents",
                        )}</span
                      >
                      <textarea
                        class="textarea textarea-bordered w-full"
                        rows="3"
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents_hint",
                        )}
                        value={t.file_create_text ?? ""}
                        on:input={(e) => {
                          t.file_create_text = (
                            e.target as HTMLTextAreaElement
                          ).value;
                          t.file_create_dirty = true;
                        }}
                      ></textarea>
                    </label>
                    <div class="flex items-center justify-between text-xs">
                      <span class="opacity-70"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_hint",
                        )}</span
                      >
                      <button
                        type="button"
                        class="btn btn-ghost btn-xs"
                        on:click={() => clearTestFile(t)}
                      >
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_clear",
                        )}
                      </button>
                    </div>
                  </div>
                {/if}
                <div
                  class="grid gap-2"
                  class:sm:grid-cols-2={assignment?.grading_policy ===
                    "weighted"}
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
                      placeholder={translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::seconds",
                      )}
                      bind:value={t.time_limit_sec}
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
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::points_placeholder",
                        )}
                        bind:value={t.weight}
                      />
                    </label>
                  {/if}
                </div>
              </div>
            {/each}
            {#if !(tests && tests.length)}<p>
                <i
                  >{translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::no_tests",
                  )}</i
                >
              </p>{/if}
          </div>
        </div>

        <input
          type="radio"
          name="tests-tab"
          role="tab"
          class="tab"
          aria-label={translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_tools_tab",
          )}
        />
        <div
          role="tabpanel"
          class="tab-content bg-base-100 border-base-300 rounded-box p-4 space-y-4"
        >
          <div class="space-y-2">
            <h4 class="text-lg font-semibold flex items-center gap-2">
              <Shield size={18} />
              {translate(
                "frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_tools_heading",
              )}
            </h4>
            <p class="text-sm opacity-70">
              {translate(
                "frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_tools_hint",
              )}
            </p>
          </div>
          <div class="flex flex-wrap gap-4">
            <label class="label cursor-pointer gap-2">
              <input
                type="radio"
                class="radio"
                value="structured"
                bind:group={toolMode}
                on:change={() => (bannedSaved = false)}
              />
              <span class="label-text"
                >{translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_mode_structured",
                )}</span
              >
            </label>
            <label class="label cursor-pointer gap-2">
              <input
                type="radio"
                class="radio"
                value="advanced"
                bind:group={toolMode}
                on:change={() => (bannedSaved = false)}
              />
              <span class="label-text"
                >{translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_mode_advanced",
                )}</span
              >
            </label>
          </div>

          {#if toolMode === "structured"}
            <div class="space-y-4">
              <div class="grid gap-4 md:grid-cols-2">
                <label class="form-control w-full">
                  <span class="label-text"
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_library_label",
                    )}</span
                  >
                  <select
                    class="select select-bordered w-full"
                    bind:value={draftLibrary}
                    on:change={() => (bannedSaved = false)}
                  >
                    {#each bannedCatalog as entry}
                      <option value={entry.library}>{entry.label}</option>
                    {/each}
                  </select>
                </label>
                <label class="form-control w-full">
                  <span class="label-text"
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_function_label",
                    )}</span
                  >
                  <div class="flex gap-2">
                    <input
                      class="input input-bordered flex-1"
                      list="banned-function-options"
                      bind:value={draftFunction}
                      placeholder={translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_function_placeholder",
                      )}
                      on:input={() => (bannedSaved = false)}
                    />
                    <datalist id="banned-function-options">
                      {#each functionOptions as fn}
                        <option value={fn.name}>{fn.label ?? fn.name}</option>
                      {/each}
                    </datalist>
                    <button
                      type="button"
                      class="btn btn-outline"
                      on:click={() => {
                        draftFunction = "*";
                        bannedSaved = false;
                      }}
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_any_button",
                      )}</button
                    >
                  </div>
                  <span class="label-text-alt text-xs"
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_function_hint",
                    )}</span
                  >
                </label>
                <label class="form-control md:col-span-2">
                  <span class="label-text"
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_note_label",
                    )}</span
                  >
                  <input
                    class="input input-bordered w-full"
                    bind:value={draftNote}
                    placeholder={translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_note_placeholder",
                    )}
                    on:input={() => (bannedSaved = false)}
                  />
                  <span class="label-text-alt text-xs"
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_note_hint",
                    )}</span
                  >
                </label>
                <div class="md:col-span-2 flex justify-end">
                  <button
                    type="button"
                    class="btn"
                    on:click={addStructuredRule}
                  >
                    <Plus size={16} />
                    {translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_add_button",
                    )}
                  </button>
                </div>
              </div>

              {#if structuredRules.length}
                <div class="space-y-2">
                  <h5 class="font-semibold">
                    {translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_rules_heading",
                    )}
                  </h5>
                  <p class="text-xs opacity-70">
                    {translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_rules_hint",
                    )}
                  </p>
                  <div class="space-y-2">
                    {#each structuredRules as rule, idx}
                      <div
                        class="rounded-lg border border-base-300/60 p-3 space-y-2 md:space-y-0 md:flex md:items-center md:gap-3"
                      >
                        <div class="font-mono text-sm md:w-48">
                          {structuredRuleDisplay(rule)}
                        </div>
                        <div class="flex-1 w-full">
                          <label class="form-control w-full">
                            <span class="label-text text-xs"
                              >{translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_rule_note_label",
                              )}</span
                            >
                            <input
                              class="input input-bordered w-full"
                              value={rule.note}
                              on:input={(event) =>
                                updateStructuredNote(
                                  idx,
                                  (event.target as HTMLInputElement).value,
                                )}
                              placeholder={translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_note_placeholder",
                              )}
                            />
                          </label>
                        </div>
                        <button
                          type="button"
                          class="btn btn-ghost btn-sm"
                          on:click={() => removeStructuredRule(idx)}
                        >
                          <Trash2 size={14} />
                          {translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_remove_button",
                          )}
                        </button>
                      </div>
                    {/each}
                  </div>
                </div>
              {:else}
                <p class="text-sm opacity-70">
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::structured_empty_state",
                  )}
                </p>
              {/if}
            </div>
          {:else}
            <div class="space-y-2">
              <label class="form-control w-full">
                <span class="label-text"
                  >{translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::advanced_label",
                  )}</span
                >
                <textarea
                  class="textarea textarea-bordered h-48"
                  bind:value={advancedPatternsText}
                  placeholder={translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::advanced_placeholder",
                  )}
                  on:input={() => (bannedSaved = false)}
                ></textarea>
                <span class="label-text-alt text-xs"
                  >{translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::advanced_tip",
                  )}</span
                >
              </label>
            </div>
          {/if}

          <div class="flex items-center gap-3">
            <button
              class="btn btn-primary"
              on:click={saveBannedTools}
              disabled={bannedSaving}
            >
              {#if bannedSaving}
                <span class="loading loading-spinner loading-sm"></span>
              {:else}
                <Save size={16} />
              {/if}
              <span
                >{translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::save_banned_tools",
                )}</span
              >
            </button>
            {#if bannedSaved}
              <span class="text-sm text-success"
                >{translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::banned_tools_saved",
                )}</span
              >
            {/if}
          </div>
        </div>

        <input
          type="radio"
          name="tests-tab"
          role="tab"
          class="tab"
          aria-label={translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_io_test",
          )}
        />
        <div
          role="tabpanel"
          class="tab-content bg-base-100 border-base-300 rounded-box p-4"
        >
          <div class="border-base-300/60 space-y-4">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h4 class="font-semibold flex items-center gap-2">
                  <Code size={18} />
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_io_test",
                  )}
                </h4>
                <p class="hint">
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::io_expected_source_hint",
                  )}
                </p>
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
            {#if ioOutputMode === "teacher"}
              <div
                class="rounded-xl border border-dashed border-primary/40 bg-primary/5 p-3 space-y-2"
              >
                <label class="form-control w-full">
                  <span class="label-text"
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_solution_file",
                    )}</span
                  >
                  <input
                    type="file"
                    accept=".py,.zip,.txt"
                    class="file-input file-input-bordered w-full"
                    on:change={(e) =>
                      (teacherSolutionFile =
                        (e.target as HTMLInputElement).files?.[0] || null)}
                  />
                </label>
                <div class="flex flex-wrap items-center gap-2 text-xs">
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
                  <span class="label-text-alt"
                    >{translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_solution_vm_hint",
                    )}</span
                  >
                </div>
              </div>
            {/if}
            <div
              class="grid gap-2"
              class:sm:grid-cols-2={assignment?.grading_policy === "weighted"}
            >
              <label class="form-control w-full space-y-1">
                <span class="label-text"
                  >{translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::input",
                  )}</span
                >
                <textarea
                  class="textarea textarea-bordered w-full"
                  rows="3"
                  placeholder={translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::stdin",
                  )}
                  bind:value={tStdin}
                ></textarea>
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text flex items-center gap-2">
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_output",
                  )}
                  {#if ioOutputMode === "teacher"}
                    <span class="badge badge-ghost badge-sm"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::auto_from_teacher",
                      )}</span
                    >
                  {/if}
                </span>
                {#if ioOutputMode === "manual"}
                  <textarea
                    class="textarea textarea-bordered w-full"
                    rows="3"
                    placeholder={translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_stdout",
                    )}
                    bind:value={tStdout}
                  ></textarea>
                {:else}
                  <div class="rounded-lg border border-dashed border-base-300/70 bg-base-200/40 px-3 py-2 text-xs">
                    {ioOutputLoading
                      ? translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_solution_running",
                        )
                      : translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_output_from_teacher",
                        )}
                  </div>
                {/if}
              </label>
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
                  placeholder={translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::seconds",
                  )}
                  bind:value={tLimit}
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
                    placeholder={translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::points_placeholder",
                    )}
                    bind:value={tWeight}
                  />
                </label>
              {/if}
              <div class="sm:col-span-2 space-y-2">
                <div class="flex items-center gap-3">
                  <button
                    class="btn btn-sm btn-outline gap-2"
                    on:click={() => (showIOFile = !showIOFile)}
                  >
                    <FileUp size={16} />
                    {ioFileName
                      ? translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::edit_file",
                        )
                      : translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_file",
                        )}
                  </button>
                  {#if ioFileName}
                    <span class="badge badge-neutral gap-2 p-3">
                      {ioFileName}
                      <button
                        class="btn btn-ghost btn-xs btn-circle text-error min-h-0 h-6 w-6"
                        on:click={() => {
                          ioFileName = "";
                          ioFileText = "";
                          ioFileBase64 = "";
                          ioFileUpload = null;
                        }}
                      >
                        <Trash2 size={12} />
                      </button>
                    </span>
                  {/if}
                </div>

                {#if showIOFile}
                  <div
                    transition:slide
                    class="rounded-xl border border-dashed border-base-300/70 bg-base-200/40 p-4 space-y-4 shadow-inner"
                  >
                    <div class="grid gap-4 sm:grid-cols-2">
                      <div class="form-control w-full">
                        <div class="label">
                          <span class="label-text"
                            >{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_upload",
                            )}</span
                          >
                        </div>
                        <div
                          class="relative flex min-h-[120px] flex-col items-center justify-center rounded-lg border-2 border-dashed border-base-300 bg-base-100 hover:bg-base-200 hover:border-primary/50 transition-all cursor-pointer group"
                        >
                          <input
                            type="file"
                            class="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-10"
                            on:change={async (e) => {
                              const file =
                                (e.target as HTMLInputElement).files?.[0] ||
                                null;
                              ioFileUpload = file;
                              if (!file) {
                                ioFileBase64 = "";
                                ioFileName = "";
                                return;
                              }
                              try {
                                ioFileBase64 = await readFileBase64(file);
                                ioFileName = file.name;
                                try {
                                  ioFileText = await readFileText(file);
                                } catch {
                                  ioFileText = "";
                                }
                              } catch (e: any) {
                                err =
                                  e?.message ||
                                  translate(
                                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::file_read_error",
                                  );
                              }
                            }}
                          />
                          <div
                            class="flex flex-col items-center gap-2 text-xs opacity-60 group-hover:opacity-100 transition-opacity pointer-events-none"
                          >
                            <UploadIcon size={24} class="text-primary" />
                            <span class="font-medium">{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_drag_drop_hint",
                            )}</span>
                          </div>
                        </div>
                      </div>

                      <div class="flex flex-col gap-2">
                        <label class="form-control w-full">
                          <div class="label">
                            <span class="label-text"
                              >{translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_name",
                              )}</span
                            >
                          </div>
                          <input
                            class="input input-bordered w-full"
                            placeholder="data.txt"
                            bind:value={ioFileName}
                          />
                        </label>
                         <p class="text-xs opacity-60 mt-auto">
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_hint")}
                         </p>
                      </div>
                    </div>

                    <label class="form-control w-full space-y-1">
                      <span class="label-text"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents",
                        )}</span
                      >
                      <textarea
                        class="textarea textarea-bordered w-full font-mono text-xs leading-relaxed"
                        rows="5"
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents_hint",
                        )}
                        bind:value={ioFileText}
                      ></textarea>
                    </label>

                    <div class="flex items-center justify-end gap-2">
                        <button class="btn btn-sm btn-ghost" on:click={() => {
                          ioFileName = "";
                          ioFileText = "";
                          ioFileBase64 = "";
                          ioFileUpload = null;
                        }}>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::remove_file")}</button>
                        <button class="btn btn-sm btn-primary" on:click={() => showIOFile = false}>
                            {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::done")}
                        </button>
                    </div>
                  </div>
                {/if}
              </div>
              <p class="hint sm:col-span-2">
                {translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::stdin_stdout_multiline_hint",
                )}
              </p>
            </div>
            <div>
              <button
                class="btn btn-primary"
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
                  <Plus size={16} />
                {/if}
                {translate(
                  ioOutputMode === "teacher"
                    ? "frontend/src/routes/assignments/[id]/tests/+page.svelte::generate_and_add"
                    : "frontend/src/routes/assignments/[id]/tests/+page.svelte::add",
                )}</button
              >
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
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_expected_hint",
                )}
              </p>
            </div>
            <div class="flex flex-wrap gap-2">
              <button
                type="button"
                class={`option-pill ${utOutputMode === "manual" ? "selected" : ""}`}
                aria-pressed={utOutputMode === "manual"}
                on:click={() => (utOutputMode = "manual")}
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
                class={`option-pill ${utOutputMode === "teacher" ? "selected" : ""}`}
                aria-pressed={utOutputMode === "teacher"}
                on:click={() => (utOutputMode = "teacher")}
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
          {#if utOutputMode === "teacher"}
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
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_teacher_autofill_hint",
                )}
              </p>
            </div>
          {/if}
          <div class="grid sm:grid-cols-2 gap-3">
            <label class="form-control w-full space-y-1">
              <span class="label-text"
                >{translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::class_name",
                )}</span
              >
              <input
                class="input input-bordered w-full"
                bind:value={utClassName}
                placeholder={translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_assignment_placeholder",
                )}
              />
            </label>
            <div class="flex items-end gap-2">
              <button class="btn btn-outline" on:click={addUTTest}
                ><Plus size={16} />
                {translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_test_method",
                )}</button
              >
              <button
                class="btn"
                on:click={() => {
                  utShowPreview = !utShowPreview;
                  refreshPreview();
                }}
                ><Eye size={16} />
                {utShowPreview
                  ? translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::hide_code",
                    )
                  : translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::preview_code",
                    )} code</button
              >
              <button
                class="btn btn-primary"
                disabled={utTests.length === 0 || utOutputLoading ||
                  (utOutputMode === "teacher" && !teacherSolutionFile)}
                on:click={uploadGeneratedUnitTests}
                >
                {#if utOutputLoading}
                  <span class="loading loading-spinner loading-sm"></span>
                {:else}
                  <FlaskConical size={16} />
                {/if}
                {translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::create_tests",
                )}</button
              >
            </div>
          </div>

          <div>
            <button
              class="btn btn-outline btn-sm"
              on:click={() => (showAdvanced = !showAdvanced)}
              >{showAdvanced
                ? translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::hide_code",
                  )
                : translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::advanced_setup_teardown",
                  )}</button
            >
            {#if showAdvanced}
              <div class="grid sm:grid-cols-2 gap-3 mt-3">
                <div>
                  <div class="label">
                    <span class="label-text"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::setup_optional",
                      )}</span
                    >
                  </div>
                  <CodeMirror
                    bind:value={utSetup}
                    lang={python()}
                    readOnly={false}
                  />
                </div>
                <div>
                  <div class="label">
                    <span class="label-text"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::teardown_optional",
                      )}</span
                    >
                  </div>
                  <CodeMirror
                    bind:value={utTeardown}
                    lang={python()}
                    readOnly={false}
                  />
                </div>
              </div>
            {/if}
          </div>

          <div class="space-y-3">
            {#each utTests as ut, ti}
              <div
                class="rounded-xl border border-base-300/60 p-3 space-y-2 ut-method"
              >
                <div
                  class="flex items-center justify-between gap-3 ut-method-header"
                >
                  <div
                    class="grid gap-2 flex-1"
                    class:sm:grid-cols-2={assignment?.grading_policy ===
                      "weighted"}
                  >
                    <label class="form-control w-full space-y-1">
                      <span class="label-text"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::method_name",
                        )}</span
                      >
                      <input
                        class="input input-bordered w-full"
                        bind:value={ut.name}
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_something_placeholder",
                        )}
                      />
                    </label>
                    <label class="form-control w-full space-y-1">
                      <span class="label-text"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::description",
                        )}</span
                      >
                      <input
                        class="input input-bordered w-full"
                        bind:value={ut.description}
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::what_this_test_checks_placeholder",
                        )}
                      />
                    </label>
                  </div>
                  <div class="flex items-end gap-2">
                    <button
                      class="btn btn-ghost btn-xs"
                      on:click={() => removeUTTest(ti)}
                      ><Trash2 size={14} />
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::remove",
                      )}</button
                    >
                  </div>
                </div>
                <div class="grid sm:grid-cols-2 gap-2">
                  <label class="form-control w-full space-y-1">
                    <span class="label-text"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::call_mode",
                      )}</span
                    >
                    <select
                      class="select select-bordered w-full"
                      bind:value={ut.callMode}
                    >
                      <option value="stdin"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::student_code_stdin_stdout",
                        )}</option
                      >
                      <option value="function"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::student_function_return_value",
                        )}</option
                      >
                    </select>
                  </label>
                  {#if ut.callMode === "function"}
                    <label class="form-control w-full space-y-1">
                      <span class="label-text"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_name",
                        )}</span
                      >
                      <input
                        class="input input-bordered w-full"
                        bind:value={ut.functionName}
                        placeholder={translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::solve_placeholder",
                        )}
                      />
                    </label>
                  {/if}
                </div>
                <div
                  class="grid gap-2"
                  class:sm:grid-cols-2={assignment?.grading_policy ===
                    "weighted"}
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
                      bind:value={ut.timeLimit}
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
                        bind:value={ut.weight}
                      />
                    </label>
                  {/if}
                </div>

                <!-- File Upload for Unittest -->
                <div class="sm:col-span-2 space-y-2">
                  <div class="flex items-center gap-3">
                    <button
                      class="btn btn-sm btn-outline gap-2"
                      on:click={() => (ut.showFile = !ut.showFile)}
                    >
                      <FileUp size={16} />
                      {ut.fileName
                        ? translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::edit_file",
                          )
                        : translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_file",
                          )}
                    </button>
                    {#if ut.fileName}
                      <span class="badge badge-neutral gap-2 p-3">
                        {ut.fileName}
                        <button
                          class="btn btn-ghost btn-xs btn-circle text-error min-h-0 h-6 w-6"
                          on:click={() => {
                            ut.fileName = "";
                            ut.fileText = "";
                            ut.fileBase64 = "";
                          }}
                        >
                          <Trash2 size={12} />
                        </button>
                      </span>
                    {/if}
                  </div>

                  {#if ut.showFile}
                    <div
                      transition:slide
                      class="rounded-xl border border-dashed border-base-300/70 bg-base-200/40 p-4 space-y-4 shadow-inner"
                    >
                      <div class="grid gap-4 sm:grid-cols-2">
                        <div class="form-control w-full">
                          <div class="label">
                            <span class="label-text"
                              >{translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_upload",
                              )}</span
                            >
                          </div>
                          <div
                            class="relative flex min-h-[120px] flex-col items-center justify-center rounded-lg border-2 border-dashed border-base-300 bg-base-100 hover:bg-base-200 hover:border-primary/50 transition-all cursor-pointer group"
                          >
                            <input
                              type="file"
                              class="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-10"
                              on:change={async (e) => {
                                const file =
                                  (e.target as HTMLInputElement).files?.[0] ||
                                  null;
                                if (!file) {
                                  ut.fileBase64 = "";
                                  ut.fileName = "";
                                  return;
                                }
                                try {
                                  ut.fileBase64 = await readFileBase64(file);
                                  ut.fileName = file.name;
                                  try {
                                    ut.fileText = await readFileText(file);
                                  } catch {
                                    ut.fileText = "";
                                  }
                                } catch (e: any) {
                                  err =
                                    e?.message ||
                                    translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::file_read_error",
                                    );
                                }
                              }}
                            />
                            <div
                              class="flex flex-col items-center gap-2 text-xs opacity-60 group-hover:opacity-100 transition-opacity pointer-events-none"
                            >
                              <UploadIcon size={24} class="text-primary" />
                              <span class="font-medium">{translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_drag_drop_hint",
                              )}</span>
                            </div>
                          </div>
                        </div>

                        <div class="flex flex-col gap-2">
                          <label class="form-control w-full">
                            <div class="label">
                              <span class="label-text"
                                >{translate(
                                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_name",
                                )}</span
                              >
                            </div>
                            <input
                              class="input input-bordered w-full"
                              placeholder="data.txt"
                              bind:value={ut.fileName}
                            />
                          </label>
                          <p class="text-xs opacity-60 mt-auto">
                            {translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_hint",
                            )}
                          </p>
                        </div>
                      </div>

                      <label class="form-control w-full space-y-1">
                        <span class="label-text"
                          >{translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents",
                          )}</span
                        >
                        <textarea
                          class="textarea textarea-bordered w-full font-mono text-xs leading-relaxed"
                          rows="5"
                          placeholder={translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents_hint",
                          )}
                          bind:value={ut.fileText}
                        ></textarea>
                      </label>

                      <div class="flex items-center justify-end gap-2">
                        <button
                          class="btn btn-sm btn-ghost"
                          on:click={() => {
                            ut.fileName = "";
                            ut.fileText = "";
                            ut.fileBase64 = "";
                          }}
                          >{translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::remove_file",
                          )}</button
                        >
                        <button
                          class="btn btn-sm btn-primary"
                          on:click={() => (ut.showFile = false)}
                        >
                          {translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::done",
                          )}
                        </button>
                      </div>
                    </div>
                  {/if}
                </div>
                <div class="space-y-2 ut-assertions">
                  <div class="flex items-center justify-between">
                    <div class="font-medium text-primary flex items-center gap-2">
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::assertions",
                      )}
                      {#if utOutputMode === "teacher"}
                        <span class="badge badge-outline badge-sm"
                          >{translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_equals_only_badge",
                          )}</span
                        >
                      {/if}
                    </div>
                    <div class="join">
                      <button
                        class="btn btn-xs join-item"
                        on:click={() => addUTAssertion(ti, "equals")}
                        ><Plus size={12} />
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::equals",
                        )}</button
                      >
                      <button
                        class="btn btn-xs join-item"
                        disabled={utOutputMode === "teacher"}
                        title={utOutputMode === "teacher"
                          ? translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_equals_only",
                            )
                          : ""}
                        on:click={() => addUTAssertion(ti, "notEquals")}
                        ><Plus size={12} />
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::not_equals",
                        )}</button
                      >
                      <button
                        class="btn btn-xs join-item"
                        disabled={utOutputMode === "teacher"}
                        title={utOutputMode === "teacher"
                          ? translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_equals_only",
                            )
                          : ""}
                        on:click={() => addUTAssertion(ti, "contains")}
                        ><Plus size={12} />
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::contains",
                        )}</button
                      >
                      <button
                        class="btn btn-xs join-item"
                        disabled={utOutputMode === "teacher"}
                        title={utOutputMode === "teacher"
                          ? translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_equals_only",
                            )
                          : ""}
                        on:click={() => addUTAssertion(ti, "notContains")}
                        ><Plus size={12} />
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::not_contains",
                        )}</button
                      >
                      <button
                        class="btn btn-xs join-item"
                        disabled={utOutputMode === "teacher"}
                        title={utOutputMode === "teacher"
                          ? translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_equals_only",
                            )
                          : ""}
                        on:click={() => addUTAssertion(ti, "regex")}
                        ><Plus size={12} />
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::regex",
                        )}</button
                      >
                      <button
                        class="btn btn-xs join-item"
                        disabled={utOutputMode === "teacher"}
                        title={utOutputMode === "teacher"
                          ? translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_equals_only",
                            )
                          : ""}
                        on:click={() => addUTAssertion(ti, "raises")}
                        ><Plus size={12} />
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::raises",
                        )}</button
                      >
                      <button
                        class="btn btn-xs join-item"
                        disabled={utOutputMode === "teacher"}
                        title={utOutputMode === "teacher"
                          ? translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::teacher_equals_only",
                            )
                          : ""}
                        on:click={() => addUTAssertion(ti, "custom")}
                        ><Plus size={12} />
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::custom",
                        )}</button
                      >
                    </div>
                  </div>
                  <div class="space-y-2">
                    {#each ut.assertions as a, ai}
                      <div
                        class="rounded-lg border border-base-300/60 p-2 space-y-2 ut-assertion-item"
                      >
                        <div class="flex items-center justify-between">
                          <span class="badge badge-primary">{a.kind}</span>
                          <button
                            class="btn btn-ghost btn-xs"
                            on:click={() => removeUTAssertion(ti, ai)}
                            ><Trash2 size={14} />
                            {translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::remove",
                            )}</button
                          >
                        </div>
                        {#if a.kind === "custom"}
                          <label class="form-control w-full space-y-1">
                            <span class="label-text"
                              >{translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::custom_python_inside_test_method",
                              )}</span
                            >
                            <textarea
                              class="textarea textarea-bordered h-24"
                              value={getCustom(a)}
                              on:input={(e) =>
                                setCustom(
                                  a,
                                  (e.target as HTMLTextAreaElement).value,
                                )}
                              placeholder={translate(
                                "frontend/src/routes/assignments/[id]/tests/+page.svelte::self_assert_true_placeholder",
                              )}
                            ></textarea>
                          </label>
                        {:else if a.kind === "regex"}
                          <div class="grid sm:grid-cols-2 gap-2">
                            <label class="form-control w-full space-y-1">
                              <span class="label-text"
                                >{ut.callMode === "function"
                                  ? translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::arguments_python_expressions_one_per_line",
                                    )
                                  : translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::inputs_one_per_line",
                                    )}</span
                              >
                              <textarea
                                class="textarea textarea-bordered h-24"
                                value={getInputs(a)}
                                on:input={(e) =>
                                  setInputs(
                                    a,
                                    (e.target as HTMLTextAreaElement).value,
                                  )}
                                placeholder={ut.callMode === "function"
                                  ? translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_args_1_2_3",
                                    )
                                  : translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_inputs_2_3",
                                    )}
                              ></textarea>
                            </label>
                            <label class="form-control w-full space-y-1">
                              <span class="label-text"
                                >{translate(
                                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::regex_pattern",
                                )}</span
                              >
                              <input
                                class="input input-bordered w-full"
                                value={getPattern(a)}
                                on:input={(e) =>
                                  setPattern(
                                    a,
                                    (e.target as HTMLInputElement).value,
                                  )}
                                placeholder={translate(
                                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_regex_5",
                                )}
                              />
                            </label>
                          </div>
                        {:else if a.kind === "raises"}
                          <div class="grid sm:grid-cols-2 gap-2">
                            <label class="form-control w-full space-y-1">
                              <span class="label-text"
                                >{ut.callMode === "function"
                                  ? translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::arguments_python_expressions_one_per_line",
                                    )
                                  : translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::inputs_one_per_line",
                                    )}</span
                              >
                              <textarea
                                class="textarea textarea-bordered h-24"
                                value={getInputs(a)}
                                on:input={(e) =>
                                  setInputs(
                                    a,
                                    (e.target as HTMLTextAreaElement).value,
                                  )}
                                placeholder={ut.callMode === "function"
                                  ? translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_invalid_value",
                                    )
                                  : translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_bad_input",
                                    )}
                              ></textarea>
                            </label>
                            <label class="form-control w-full space-y-1">
                              <span class="label-text"
                                >{translate(
                                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::exception_type",
                                )}</span
                              >
                              <input
                                class="input input-bordered w-full"
                                value={getException(a)}
                                on:input={(e) =>
                                  setException(
                                    a,
                                    (e.target as HTMLInputElement).value,
                                  )}
                                placeholder={translate(
                                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::value_error_placeholder",
                                )}
                              />
                            </label>
                          </div>
                        {:else}
                          <div class="grid sm:grid-cols-2 gap-2">
                            <label class="form-control w-full space-y-1">
                              <span class="label-text"
                                >{ut.callMode === "function"
                                  ? translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::arguments_python_expressions_one_per_line",
                                    )
                                  : translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::inputs_one_per_line",
                                    )}</span
                              >
                              <textarea
                                class="textarea textarea-bordered h-24"
                                value={getInputs(a)}
                                on:input={(e) =>
                                  setInputs(
                                    a,
                                    (e.target as HTMLTextAreaElement).value,
                                  )}
                                placeholder={ut.callMode === "function"
                                  ? translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_args_1_2_3",
                                    )
                                  : translate(
                                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_inputs_2_3",
                                    )}
                              ></textarea>
                            </label>
                            {#if utOutputMode === "manual"}
                              <label class="form-control w-full space-y-1">
                                <span class="label-text"
                                  >{ut.callMode === "function"
                                    ? translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_return_python_expression",
                                      )
                                    : translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_output",
                                      )}</span
                                >
                                <textarea
                                  class="textarea textarea-bordered h-24"
                                  value={getExpected(a)}
                                  on:input={(e) =>
                                    setExpected(
                                      a,
                                      (e.target as HTMLTextAreaElement).value,
                                    )}
                                  placeholder={translate(
                                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::example_expected_5",
                                  )}
                                ></textarea>
                              </label>
                            {:else}
                              <div class="form-control w-full">
                                <span class="label-text">
                                  {ut.callMode === "function"
                                    ? translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_return_python_expression",
                                      )
                                    : translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_output",
                                      )}
                                </span>
                                <div
                                  class="rounded-lg border border-dashed border-base-300/70 bg-base-200/40 px-3 py-2 text-xs"
                                >
                                  {translate(
                                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::expected_output_from_teacher",
                                  )}
                                </div>
                              </div>
                            {/if}
                          </div>
                        {/if}
                      </div>
                    {/each}
                    {#if ut.assertions.length === 0}
                      <p class="text-sm opacity-70">
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_assertions_to_this_test",
                        )}
                      </p>
                    {/if}
                  </div>
                </div>
              </div>
            {/each}
          </div>

          {#if utShowPreview}
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <h4 class="font-semibold">
                  {translate(
                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::generated_python",
                  )}
                </h4>
                <div class="join">
                  <button class="btn btn-sm join-item" on:click={refreshPreview}
                    ><Code size={14} />
                    {translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::refresh",
                    )}</button
                  >
                  <button class="btn btn-sm join-item" on:click={copyPreview}
                    ><Copy size={14} />
                    {copiedPreview
                      ? translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::copied",
                        )
                      : translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::copy",
                        )}</button
                  >
                </div>
              </div>
              <CodeMirror
                bind:value={utPreviewCode}
                lang={python()}
                readOnly={true}
              />
            </div>
          {/if}
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
                  {:else}
                    <span class="badge badge-outline badge-sm"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::no_arguments",
                      )}</span
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
                    <div class="flex items-center gap-3">
                      <button
                        class="btn btn-xs btn-outline gap-2"
                        on:click={() =>
                          (fnCases = fnCases.map((c, idx) =>
                            idx === fi ? { ...c, showFile: !c.showFile } : c,
                          ))}
                      >
                        <FileUp size={14} />
                        {fc.fileName
                          ? translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::edit_file",
                            )
                          : translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_file",
                            )}
                      </button>
                      {#if fc.fileName}
                        <span class="badge badge-neutral gap-2">
                          {fc.fileName}
                          <button
                            class="btn btn-ghost btn-xs btn-circle text-error min-h-0 h-4 w-4"
                            on:click={() =>
                              (fnCases = fnCases.map((c, idx) =>
                                idx === fi
                                  ? {
                                      ...c,
                                      fileName: "",
                                      fileText: "",
                                      fileBase64: "",
                                    }
                                  : c,
                              ))}
                          >
                            <Trash2 size={10} />
                          </button>
                        </span>
                      {/if}
                    </div>

                    {#if fc.showFile}
                      <div
                        transition:slide
                        class="rounded-xl border border-dashed border-base-300/70 bg-base-200/40 p-4 space-y-4 shadow-inner"
                      >
                        <div class="grid gap-4 sm:grid-cols-2">
                          <div class="form-control w-full">
                            <div class="label">
                              <span class="label-text"
                                >{translate(
                                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_upload",
                                )}</span
                              >
                            </div>
                            <div
                              class="relative flex min-h-[100px] flex-col items-center justify-center rounded-lg border-2 border-dashed border-base-300 bg-base-100 hover:bg-base-200 hover:border-primary/50 transition-all cursor-pointer group"
                            >
                              <input
                                type="file"
                                class="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-10"
                                on:change={async (e) => {
                                  const file =
                                    (e.target as HTMLInputElement).files?.[0] ||
                                    null;
                                  if (!file) return;
                                  try {
                                    const base64 = await readFileBase64(file);
                                    let text = "";
                                    try {
                                      text = await readFileText(file);
                                    } catch {}
                                    fnCases = fnCases.map((c, idx) =>
                                      idx === fi
                                        ? {
                                            ...c,
                                            fileBase64: base64,
                                            fileName: file.name,
                                            fileText: text,
                                          }
                                        : c,
                                    );
                                  } catch (e: any) {
                                    err =
                                      e?.message ||
                                      translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::file_read_error",
                                      );
                                  }
                                }}
                              />
                              <div
                                class="flex flex-col items-center gap-2 text-xs opacity-60 group-hover:opacity-100 transition-opacity pointer-events-none"
                              >
                                <UploadIcon size={24} class="text-primary" />
                                <span class="font-medium">{translate(
                                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_drag_drop_hint",
                                )}</span>
                              </div>
                            </div>
                          </div>

                          <div class="flex flex-col gap-2">
                            <label class="form-control w-full">
                              <div class="label">
                                <span class="label-text"
                                  >{translate(
                                    "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_name",
                                  )}</span
                                >
                              </div>
                              <input
                                class="input input-bordered w-full"
                                placeholder="data.txt"
                                value={fc.fileName}
                                on:input={(e) =>
                                  (fnCases = fnCases.map((c, idx) =>
                                    idx === fi
                                      ? {
                                          ...c,
                                          fileName: (
                                            e.target as HTMLInputElement
                                          ).value,
                                        }
                                      : c,
                                  ))}
                              />
                            </label>
                             <p class="text-xs opacity-60 mt-auto">
                                {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_hint")}
                             </p>
                          </div>
                        </div>

                        <label class="form-control w-full space-y-1">
                          <span class="label-text"
                            >{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents",
                            )}</span
                          >
                          <textarea
                            class="textarea textarea-bordered w-full font-mono text-xs leading-relaxed"
                            rows="5"
                            placeholder={translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents_hint",
                            )}
                            value={fc.fileText}
                            on:input={(e) =>
                              (fnCases = fnCases.map((c, idx) =>
                                idx === fi
                                  ? {
                                      ...c,
                                      fileText: (e.target as HTMLTextAreaElement)
                                        .value,
                                      fileBase64: "", // Clear base64 if text edited manually? Or logic is to prefer base64 if present?
                                      // Actually existing logic in buildFilePayload prefers base64.
                                      // If I edit text, I should probably clear base64 so it uses text.
                                      // The previous implementation did exactly this:
                                      // fileText: ..., fileBase64: ""
                                    }
                                  : c,
                              ))}
                          ></textarea>
                        </label>

                        <div class="flex items-center justify-end gap-2">
                            <button class="btn btn-sm btn-ghost" on:click={() =>
                              (fnCases = fnCases.map((c, idx) =>
                                idx === fi
                                  ? {
                                      ...c,
                                      fileName: "",
                                      fileText: "",
                                      fileBase64: "",
                                    }
                                  : c,
                              ))}>{translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::remove_file")}</button>
                            <button class="btn btn-sm btn-primary" on:click={() =>
                              (fnCases = fnCases.map((c, idx) =>
                                idx === fi ? { ...c, showFile: false } : c,
                              ))}>
                                {translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::done")}
                            </button>
                        </div>
                      </div>
                    {/if}
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
          <div class="space-y-6">
            <section class="panel ai-hero">
              <div
                class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between"
              >
                <div class="flex items-start gap-3">
                  <div class="ai-hero__icon">
                    <FlaskConical size={18} />
                  </div>
                  <div class="space-y-1">
                    <h3 class="text-lg font-semibold">
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::generate_with_ai",
                      )}
                    </h3>
                    <p class="text-sm opacity-75">
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_call_mode_hint",
                      )}
                    </p>
                  </div>
                </div>
                <div class="flex flex-wrap gap-2 text-xs font-medium">
                  <span class="stat-chip">
                    {translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::call_mode",
                    )}:
                    <strong
                      >{aiCallMode === "stdin"
                        ? translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::stdin_stdout_button",
                          )
                        : translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_return_button",
                          )}</strong
                    >
                  </span>
                  <span class="stat-chip">
                    {translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::task_difficulty",
                    )}:
                    <strong
                      >{aiDifficulty === "simple"
                        ? translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::simple_task_button",
                          )
                        : translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::hard_task_button",
                          )}</strong
                    >
                  </span>
                  <span class="stat-chip">
                    {translate(
                      "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_count_mode",
                    )}:
                    <strong
                      >{aiAuto
                        ? translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::auto",
                          )
                        : `${translate("frontend/src/routes/assignments/[id]/tests/+page.svelte::manual")} · ${aiNumTests}`}</strong
                    >
                  </span>
                </div>
              </div>
            </section>

            <div class="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
              <div class="space-y-6">
                <section class="panel space-y-6">
                  <div class="space-y-4">
                    <div class="space-y-2">
                      <span class="section-label"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::call_mode",
                        )}</span
                      >
                      <div class="flex flex-wrap gap-2">
                        <button
                          type="button"
                          class={`option-pill ${aiCallMode === "stdin" ? "selected" : ""}`}
                          aria-pressed={aiCallMode === "stdin"}
                          on:click={() => (aiCallMode = "stdin")}
                        >
                          <span
                            class="option-pill__indicator"
                            aria-hidden="true"
                          />
                          <span
                            >{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::stdin_stdout_button",
                            )}</span
                          >
                        </button>
                        <button
                          type="button"
                          class={`option-pill ${aiCallMode === "function" ? "selected" : ""}`}
                          aria-pressed={aiCallMode === "function"}
                          on:click={() => (aiCallMode = "function")}
                        >
                          <span
                            class="option-pill__indicator"
                            aria-hidden="true"
                          />
                          <span
                            >{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::function_return_button",
                            )}</span
                          >
                        </button>
                      </div>
                      <p class="hint">
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_call_mode_hint",
                        )}
                      </p>
                    </div>

                    <div class="space-y-2">
                      <span class="section-label"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::task_difficulty",
                        )}</span
                      >
                      <div class="flex flex-wrap gap-2">
                        <button
                          type="button"
                          class={`option-pill ${aiDifficulty === "simple" ? "selected" : ""}`}
                          aria-pressed={aiDifficulty === "simple"}
                          on:click={() => (aiDifficulty = "simple")}
                        >
                          <span
                            class="option-pill__indicator"
                            aria-hidden="true"
                          />
                          <span
                            >{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::simple_task_button",
                            )}</span
                          >
                        </button>
                        <button
                          type="button"
                          class={`option-pill ${aiDifficulty === "hard" ? "selected" : ""}`}
                          aria-pressed={aiDifficulty === "hard"}
                          on:click={() => (aiDifficulty = "hard")}
                        >
                          <span
                            class="option-pill__indicator"
                            aria-hidden="true"
                          />
                          <span
                            >{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::hard_task_button",
                            )}</span
                          >
                        </button>
                      </div>
                      <p class="hint">
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_difficulty_hint",
                        )}
                      </p>
                    </div>

                    <div class="space-y-2">
                      <span class="section-label"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_count_mode",
                        )}</span
                      >
                      <div class="flex flex-wrap gap-2">
                        <button
                          type="button"
                          class={`option-pill ${aiAuto ? "selected" : ""}`}
                          aria-pressed={aiAuto}
                          on:click={() => (aiAuto = true)}
                        >
                          <span
                            class="option-pill__indicator"
                            aria-hidden="true"
                          />
                          <span
                            >{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::auto",
                            )}</span
                          >
                        </button>
                        <button
                          type="button"
                          class={`option-pill ${!aiAuto ? "selected" : ""}`}
                          aria-pressed={!aiAuto}
                          on:click={() => (aiAuto = false)}
                        >
                          <span
                            class="option-pill__indicator"
                            aria-hidden="true"
                          />
                          <span
                            >{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::manual",
                            )}</span
                          >
                        </button>
                      </div>
                      {#if !aiAuto}
                        <div
                          class="mt-2 grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
                        >
                          <input
                            type="number"
                            min="1"
                            class="input input-bordered w-full"
                            bind:value={aiNumTests}
                            placeholder={translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::how_many_tests_placeholder",
                            )}
                          />
                        </div>
                      {/if}
                      <p class="hint">
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::auto_lets_model_decide_number_of_tests",
                        )}
                      </p>
                    </div>
                  </div>

                  <div class="space-y-2">
                    <span class="section-label"
                      >{translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::additional_instructions_optional",
                      )}</span
                    >
                    <textarea
                      class="textarea textarea-bordered min-h-[110px]"
                      bind:value={aiInstructions}
                      placeholder={translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::edge_cases_to_cover_placeholder",
                      )}
                    ></textarea>
                    <p class="hint">
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_instructions_hint",
                      )}
                    </p>
                  </div>
                </section>

                <section class="panel space-y-4">
                  <div class="space-y-1">
                    <h4 class="text-base font-semibold">
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_label",
                      )}
                    </h4>
                    <p class="hint">
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_hint",
                      )}
                    </p>
                  </div>
                  <label class="form-control w-full">
                    <input
                      id="ai-solution-upload"
                      type="file"
                      accept=".py,.txt"
                      class="file-input file-input-bordered w-full"
                      on:change={handleAISolutionChange}
                    />
                  </label>
                  {#if aiSolutionError}
                    <div class="text-xs text-error">{aiSolutionError}</div>
                  {/if}
                  {#if aiSolutionText.trim().length}
                    <div
                      class="flex flex-wrap items-center justify-between gap-2 text-xs"
                    >
                      <span
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_loaded",
                          {
                            name: aiSolutionFile
                              ? aiSolutionFile.name
                              : translate(
                                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_manual",
                                ),
                            bytes: aiSolutionText.length,
                          },
                        )}</span
                      >
                      <button
                        type="button"
                        class="btn btn-ghost btn-xs"
                        on:click={resetAISolutionInput}
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_clear",
                        )}</button
                      >
                    </div>
                    <CodeMirror
                      bind:value={aiSolutionText}
                      lang={python()}
                      readOnly={false}
                      placeholder={translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_teacher_solution_hint",
                      )}
                    />
                  {/if}
                </section>

                {#if (aiCode && aiCode.trim().length) || hasAIBuilder}
                  <section class="panel space-y-4">
                    <div class="flex items-center justify-between gap-2">
                      <h4 class="text-base font-semibold">
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::optional_test_on_teacher_solution",
                        )}
                      </h4>
                      <span class="badge badge-outline badge-sm"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::upload_teacher_solution",
                        )}</span
                      >
                    </div>
                    <div
                      class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
                    >
                      <input
                        type="file"
                        accept=".py,.zip"
                        class="file-input file-input-bordered w-full"
                        on:change={(e) =>
                          (teacherSolutionFile =
                            (e.target as HTMLInputElement).files?.[0] || null)}
                      />
                      <button
                        class="btn btn-outline sm:w-max"
                        disabled={!teacherSolutionFile || teacherRunLoading}
                        on:click={runTeacherSolution}
                      >
                        <FlaskConical size={16} />
                        {teacherRunLoading
                          ? translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::running",
                            )
                          : translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::run_tests_on_solution",
                            )}
                      </button>
                    </div>
                    {#if teacherRun}
                      <div class="panel-soft space-y-3">
                        <div
                          class="flex items-center justify-between text-sm font-medium"
                        >
                          <span
                            >{translate(
                              "frontend/src/routes/assignments/[id]/tests/+page.svelte::results_passed",
                              {
                                passed: teacherRun.passed,
                                total: teacherRun.total,
                              },
                            )}</span
                          >
                          <span class="badge badge-primary badge-sm"
                            >{teacherRun.passed}/{teacherRun.total}</span
                          >
                        </div>
                        <div class="grid gap-2 max-h-56 overflow-y-auto pr-1">
                          {#each teacherRun.results as r}
                            <div class="result-item">
                              <div
                                class="flex items-center justify-between text-xs font-medium"
                              >
                                <div class="flex items-center gap-2">
                                  {#if r.preview}
                                    <span class="badge badge-secondary badge-sm"
                                      >{translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::preview_badge",
                                      )}</span
                                    >
                                  {:else if r.test_case_id}
                                    <span class="badge badge-outline badge-sm"
                                      >#{r.test_case_id}</span
                                    >
                                  {/if}
                                  {#if r.unittest_name}<span
                                      class="badge badge-primary badge-sm"
                                      >{r.unittest_name}</span
                                    >{/if}
                                </div>
                                <span
                                  class="badge {r.status === 'passed'
                                    ? 'badge-success'
                                    : 'badge-error'} badge-sm"
                                >
                                  {r.status === "passed"
                                    ? translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::passed",
                                      )
                                    : translate(
                                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::failed",
                                      )}
                                </span>
                              </div>
                              {#if r.stderr}
                                <pre class="result-log">{r.stderr}</pre>
                              {/if}
                            </div>
                          {/each}
                        </div>
                      </div>
                    {/if}
                  </section>
                {/if}
              </div>

              <div class="space-y-6">
                <section class="panel space-y-4 lg:sticky lg:top-28">
                  <div class="space-y-1">
                    <h4 class="text-base font-semibold">
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::generate_with_ai",
                      )}
                    </h4>
                    <p class="hint">
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::you_can_edit_this_before_saving",
                      )}
                    </p>
                  </div>
                  <div class="flex flex-col gap-2 sm:flex-row">
                    <button
                      class="btn btn-primary flex-1"
                      on:click={generateWithAI}
                      disabled={aiGenerating}
                    >
                      <FlaskConical size={16} />
                      {aiGenerating
                        ? translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::generating",
                          )
                        : translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::generate_with_ai",
                          )}
                    </button>
                    <button
                      class="btn btn-outline flex-1"
                      on:click={uploadAIUnitTestsCode}
                      disabled={builderMode !== "unittest" || !aiCode}
                      title={builderMode !== "unittest"
                        ? translate(
                            "frontend/src/routes/assignments/[id]/tests/+page.svelte::available_only_for_unittest_generation",
                          )
                        : ""}
                    >
                      <UploadIcon size={16} />
                      {translate(
                        "frontend/src/routes/assignments/[id]/tests/+page.svelte::save_as_tests",
                      )}
                    </button>
                  </div>
                  {#if aiGenerating}
                    <div
                      class="inline-flex items-center gap-2 text-xs font-medium text-warning"
                    >
                      <Clock size={14} />
                      <span
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::generating",
                        )}</span
                      >
                    </div>
                  {/if}
                  {#if hasAIBuilder}
                    <div class="panel-soft border border-primary/30 text-sm">
                      <span
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_prepared_builder_structure_below",
                        )}</span
                      >
                    </div>
                  {/if}
                </section>

                {#if aiCode && builderMode === "unittest"}
                  <section class="panel space-y-3">
                    <div class="flex items-center justify-between">
                      <h4 class="text-base font-semibold">
                        {translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::ai_python_editable",
                        )}
                      </h4>
                      <span class="badge badge-outline badge-sm"
                        >{translate(
                          "frontend/src/routes/assignments/[id]/tests/+page.svelte::save_as_tests",
                        )}</span
                      >
                    </div>
                    <CodeMirror
                      bind:value={aiCode}
                      lang={python()}
                      readOnly={false}
                    />
                  </section>
                {/if}
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
          <p class="text-xs opacity-70 mt-2">
            {translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::unittest_upload_guidance",
            )}
          </p>
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

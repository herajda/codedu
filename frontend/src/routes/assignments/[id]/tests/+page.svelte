<script lang="ts">
  // @ts-nocheck
  import { onMount } from 'svelte'
  import { page } from '$app/stores'
  import { apiFetch, apiJSON } from '$lib/api'
  import { auth } from '$lib/auth'
  import { extractMethodFromUnittest, leadingIndent, parseUnitTestQualifiedName } from '$lib/unittests'
  import CodeMirror from '$lib/components/ui/CodeMirror.svelte'
  import { python } from '@codemirror/lang-python'
  import { Plus, Save, Trash2, Eye, FlaskConical, FileUp, Code, Copy, Clock, Scale, Upload as UploadIcon } from 'lucide-svelte'
  import ConfirmModal from '$lib/components/ConfirmModal.svelte'

  $: id = $page.params.id
  $: role = ($auth as any)?.role ?? ''

  let assignment: any = null
  let tests: any[] = []
  let err = ''
  let confirmModal: InstanceType<typeof ConfirmModal>

  // LLM testing settings (moved from Edit page)
  let llmFeedback = false
  let llmAutoAward = true
  let llmScenarios = ''
  let llmStrictness = 50
  let llmRubric = ''
  const exampleScenario = '[{"name":"calc","steps":[{"send":"2 + 2","expect_after":"4"}]}]'

  // local inputs for creating/uploading tests
  let tStdin = ''
  let tStdout = ''
  let tLimit = ''
  let tWeight = '1'
  let unittestFile: File | null = null

  // ──────────────────────────────────────────────────────
  // AI generator state
  // ──────────────────────────────────────────────────────
  let aiNumTests = '5'
  let aiInstructions = ''
  let aiGenerating = false
  let aiCode = ''
  let hasAIBuilder = false
  let aiAuto = true
  // Teacher solution testing
  let teacherSolutionFile: File | null = null
  let teacherRun: any = null
  let teacherRunLoading = false

  // ──────────────────────────────────────────────────────
  // Unittest Builder state
  // ──────────────────────────────────────────────────────
  type UTAssertKind = 'equals' | 'notEquals' | 'contains' | 'notContains' | 'regex' | 'raises' | 'custom'
  type UTAssertion =
    | { kind: 'equals' | 'notEquals' | 'contains' | 'notContains'; args: string[]; expected: string }
    | { kind: 'regex'; args: string[]; pattern: string }
    | { kind: 'raises'; args: string[]; exception: string }
    | { kind: 'custom'; code: string }

  type UTTest = {
    name: string
    description?: string
    weight: string
    timeLimit: string
    assertions: UTAssertion[]
  }

  let utClassName = 'TestAssignment'
  let utSetup = ''
  let utTeardown = ''
  let utTests: UTTest[] = []
  let utShowPreview = false
  let utPreviewCode = ''
  let showAdvanced = false
  let copiedPreview = false

  // ──────────────────────────────────────────────────────
  // Unittest per-method view/edit helpers
  // ──────────────────────────────────────────────────────
  let editDialog: HTMLDialogElement
  let editingTest: any = null
  let editingMethodCode = ''

  function normalizeIndent(block: string): { lines: string[]; base: number } {
    const raw = String(block || '').replace(/\r\n?/g, '\n')
    const lines = raw.split('\n')
    let base = Infinity
    for (const l of lines) {
      if (l.trim() === '') continue
      const ind = leadingIndent(l).length
      base = Math.min(base, ind)
    }
    if (!isFinite(base)) base = 0
    const out = lines.map((l) => (l.length >= base ? l.slice(base) : l))
    return { lines: out, base }
  }

  function replaceMethodInUnittest(src: string, qn: string, newMethod: string): string {
    const { cls, method } = parseUnitTestQualifiedName(qn)
    const lines = String(src || '').split('\n')
    const escapedClass = cls.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const escapedMethod = method.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const classRE = new RegExp(`^([\\t ]*)class\\s+${escapedClass}\\s*\\(.*unittest\\.TestCase.*\\):`)
    const methodRE = new RegExp(`^([\\t ]*)def\\s+${escapedMethod}\\s*\\(`)
    let classIdx = -1
    let classIndent = ''
    for (let i = 0; i < lines.length; i++) {
      const m = lines[i].match(classRE)
      if (m) {
        classIdx = i
        classIndent = m[1] || ''
        break
      }
    }
    if (classIdx === -1) return src
    let start = -1
    let startIndent = ''
    for (let i = classIdx + 1; i < lines.length; i++) {
      const l = lines[i]
      if (l.trim() === '') continue
      if (!l.startsWith(classIndent) && leadingIndent(l).length <= classIndent.length) break
      const m = l.match(methodRE)
      if (m) {
        start = i
        startIndent = m[1] || ''
        while (start - 1 > classIdx && lines[start - 1].trim().startsWith('@') && leadingIndent(lines[start - 1]) === startIndent) {
          start--
        }
        break
      }
    }
    if (start === -1) return src
    let end = lines.length
    for (let i = start + 1; i < lines.length; i++) {
      const l = lines[i]
      if (l.trim() === '') continue
      const ind = leadingIndent(l)
      const t = l.trimStart()
      if (ind.length <= startIndent.length && (t.startsWith('def ') || t.startsWith('class '))) {
        end = i
        break
      }
    }
    const { lines: newLines } = normalizeIndent(newMethod)
    const pad = startIndent
    const adjusted = newLines.map((l) => (l.trim() === '' ? l.trimEnd() : pad + l))
    const replaced = [...lines.slice(0, start), ...adjusted, ...lines.slice(end)]
    return replaced.join('\n')
  }

  function openEditUnitTest(t: any) {
    editingTest = t
    editingMethodCode = extractMethodFromUnittest(String(t.unittest_code || ''), String(t.unittest_name || ''))
    editDialog?.showModal()
  }

  function closeEditUnitTest() {
    editDialog?.close()
    editingTest = null
    editingMethodCode = ''
  }

  async function saveEditUnitTest() {
    if (!editingTest) return
    try {
      const before = String(editingTest.unittest_code || '')
      const qn = String(editingTest.unittest_name || '')
      const updated = replaceMethodInUnittest(before, qn, editingMethodCode)
      let newQN = qn
      const m = editingMethodCode.match(/def\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(/)
      if (m) {
        const { cls } = parseUnitTestQualifiedName(qn)
        newQN = `${cls}.${m[1]}`
      }
      const testData: any = {
        stdin: editingTest.stdin ?? '',
        expected_stdout: editingTest.expected_stdout ?? '',
        time_limit_sec: parseFloat(editingTest.time_limit_sec) || 1,
        unittest_code: updated,
        unittest_name: newQN
      }
      
      // Only include weight for weighted assignments
      if (assignment?.grading_policy === 'weighted') {
        testData.weight = parseFloat(editingTest.weight) || 1
      }
      
      await apiFetch(`/api/tests/${editingTest.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(testData)
      })
      closeEditUnitTest()
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  function getInputs(a: UTAssertion): string {
    return (a as any).args ? ((a as any).args as string[]).join('\n') : ''
  }
  function setInputs(a: UTAssertion, v: string) {
    if ((a as any).args) (a as any).args = v.split('\n')
    utTests = [...utTests]
  }
  function getExpected(a: UTAssertion): string {
    return (a as any).expected ?? ''
  }
  function setExpected(a: UTAssertion, v: string) {
    (a as any).expected = v
    utTests = [...utTests]
  }
  function getPattern(a: UTAssertion): string {
    return (a as any).pattern ?? ''
  }
  function setPattern(a: UTAssertion, v: string) {
    (a as any).pattern = v
    utTests = [...utTests]
  }
  function getException(a: UTAssertion): string {
    return (a as any).exception ?? 'Exception'
  }
  function setException(a: UTAssertion, v: string) {
    (a as any).exception = v
    utTests = [...utTests]
  }
  function getCustom(a: UTAssertion): string {
    return (a as any).code ?? ''
  }
  function setCustom(a: UTAssertion, v: string) {
    (a as any).code = v
    utTests = [...utTests]
  }

  // Auto-generate preview code reactively when inputs change
  $: {
    utClassName; utSetup; utTeardown; utTests;
    const nextCode = generateUnittestCode()
    if (nextCode !== utPreviewCode) {
      utPreviewCode = nextCode
    }
  }

  async function load() {
    err = ''
    try {
      const data = await apiJSON(`/api/assignments/${id}`)
      assignment = data.assignment
      tests = data.tests ?? []
      // init llm state
      llmFeedback = !!assignment.llm_feedback
      llmAutoAward = assignment.llm_auto_award ?? true
      llmScenarios = assignment.llm_scenarios_json ?? ''
      llmStrictness = typeof assignment.llm_strictness === 'number' ? assignment.llm_strictness : 50
      llmRubric = assignment.llm_rubric ?? ''
    } catch (e: any) {
      err = e.message
    }
  }

  onMount(async () => {
    await load()
  })

  async function addTest() {
    try {
      const testData: any = {
        stdin: tStdin,
        expected_stdout: tStdout,
        time_limit_sec: parseFloat(tLimit) || undefined
      }
      
      // Only include weight for weighted assignments
      if (assignment?.grading_policy === 'weighted') {
        testData.weight = parseFloat(tWeight) || 1
      }
      
      await apiFetch(`/api/assignments/${id}/tests`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(testData)
      })
      tStdin = tStdout = tLimit = ''
      tWeight = '1'
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  async function uploadUnitTests() {
    if (!unittestFile) return
    const fd = new FormData()
    fd.append('file', unittestFile)
    try {
      await apiFetch(`/api/assignments/${id}/tests/upload`, { method: 'POST', body: fd })
      unittestFile = null
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  // ──────────────────────────────────────────────────────
  // Enhanced: Upload generated unittest code + adjust weights/limits
  // ──────────────────────────────────────────────────────
  function addUTTest() {
    utTests = [
      ...utTests,
      { name: `test_${utTests.length + 1}`, description: '', weight: assignment?.grading_policy === 'weighted' ? '1' : '0', timeLimit: '1', assertions: [] }
    ]
  }

  function removeUTTest(idx: number) {
    utTests = utTests.filter((_, i) => i !== idx)
  }

  function addUTAssertion(idx: number, kind: UTAssertKind) {
    const t = utTests[idx]
    let a: UTAssertion
    switch (kind) {
      case 'equals':
      case 'notEquals':
      case 'contains':
      case 'notContains':
        a = { kind, args: [''], expected: '' }
        break
      case 'regex':
        a = { kind, args: [''], pattern: '' }
        break
      case 'raises':
        a = { kind, args: [''], exception: 'Exception' }
        break
      default:
        a = { kind: 'custom', code: '' }
    }
    t.assertions = [...t.assertions, a]
    utTests = [...utTests]
  }

  function removeUTAssertion(ti: number, ai: number) {
    utTests[ti].assertions = utTests[ti].assertions.filter((_, i) => i !== ai)
    utTests = [...utTests]
  }

  function generateUnittestCode(): string {
    const lines: string[] = []
    lines.push('import unittest')
    lines.push('')
    lines.push(`class ${utClassName}(unittest.TestCase):`)
    if (utSetup && utSetup.trim() !== '') {
      lines.push('    def setUp(self):')
      utSetup.split('\n').forEach((l) => lines.push('        ' + l))
      if (utSetup[utSetup.length - 1] !== '\n') lines.push('')
    }
    if (utTeardown && utTeardown.trim() !== '') {
      lines.push('    def tearDown(self):')
      utTeardown.split('\n').forEach((l) => lines.push('        ' + l))
      if (utTeardown[utTeardown.length - 1] !== '\n') lines.push('')
    }
    for (const t of utTests) {
      const methodName = t.name.startsWith('test_') ? t.name : `test_${t.name}`
      lines.push(`    def ${methodName}(self):`)
      if (t.description && t.description.trim() !== '') {
        lines.push(`        """${t.description}"""`)
      }
      if (t.assertions.length === 0) {
        lines.push('        self.assertTrue(True)')
        continue
      }
      for (const a of t.assertions) {
        if (a.kind === 'custom') {
          const code = (a as any).code || ''
          const cs = code.split('\n').map((l: string) => '        ' + l)
          lines.push(...cs)
          continue
        }
        const fmtArgs = (a as any).args?.map((s: string) => JSON.stringify(s)) ?? []
        const call = `student_code(${fmtArgs.join(', ')})`
        switch (a.kind) {
          case 'equals':
            lines.push(`        self.assertEqual(${call}, ${JSON.stringify((a as any).expected ?? '')})`)
            break
          case 'notEquals':
            lines.push(`        self.assertNotEqual(${call}, ${JSON.stringify((a as any).expected ?? '')})`)
            break
          case 'contains':
            lines.push(`        self.assertIn(${JSON.stringify((a as any).expected ?? '')}, ${call})`)
            break
          case 'notContains':
            lines.push(`        self.assertNotIn(${JSON.stringify((a as any).expected ?? '')}, ${call})`)
            break
          case 'regex':
            lines.push(`        self.assertRegex(${call}, r${JSON.stringify((a as any).pattern ?? '').replace(/^"|"$/g,'"')})`)
            break
          case 'raises':
            lines.push(`        with self.assertRaises(${(a as any).exception || 'Exception'}):`)
            lines.push(`            ${call}`)
            break
        }
      }
    }
    lines.push('')
    return lines.join('\n')
  }

  function refreshPreview() {
    utPreviewCode = generateUnittestCode()
  }

  async function copyPreview() {
    try {
      await navigator.clipboard.writeText(utPreviewCode)
      copiedPreview = true
      setTimeout(() => (copiedPreview = false), 1500)
    } catch {}
  }

  // ──────────────────────────────────────────────────────
  // AI generator actions
  // ──────────────────────────────────────────────────────
  function stripOuterQuotes(s: string): string {
    const t = s.trim()
    if ((t.startsWith("'") && t.endsWith("'")) || (t.startsWith('"') && t.endsWith('"'))) {
      return t.slice(1, -1)
    }
    return t
  }
  function splitArgsPreserveQuoted(inner: string): string[] {
    const out: string[] = []
    let buf = ''
    let q: string | null = null
    for (let i = 0; i < inner.length; i++) {
      const ch = inner[i]
      if (q) {
        if (ch === q) {
          q = null
        }
        buf += ch
        continue
      }
      if (ch === '"' || ch === "'") {
        q = ch
        buf += ch
        continue
      }
      if (ch === ',') {
        out.push(buf.trim())
        buf = ''
        continue
      }
      buf += ch
    }
    if (buf.trim() !== '') out.push(buf.trim())
    return out
  }
  function parseArgsFromMaybeStudentCode(v: any): string[] {
    const raw = String(v ?? '').trim()
    const m = raw.match(/^student_code\s*\(([\s\S]*)\)$/)
    if (m) {
      const inner = m[1]
      return splitArgsPreserveQuoted(inner).map((p) => stripOuterQuotes(p))
    }
    return [stripOuterQuotes(raw)]
  }
  function coerceUTAssertion(a: any): UTAssertion {
    const kind = (a?.kind as UTAssertKind) || 'custom'
    if (kind === 'custom') return { kind: 'custom', code: String(a?.code ?? '') }
    const rawArgs: any[] = Array.isArray(a?.args) ? a.args : (a?.args != null ? [a.args] : [])
    const normalized: string[] = rawArgs.flatMap((v: any) => parseArgsFromMaybeStudentCode(v))
    if (kind === 'regex') return { kind: 'regex', args: normalized, pattern: String(a?.pattern ?? '') }
    if (kind === 'raises') return { kind: 'raises', args: normalized, exception: String(a?.exception ?? 'Exception') }
    return { kind, args: normalized, expected: String(a?.expected ?? '') }
  }
  function coerceUTTest(t: any): UTTest {
    return {
      name: String(t?.name ?? 'test_case'),
      description: t?.description ? String(t?.description) : '',
      weight: String(t?.weight ?? '1'),
      timeLimit: String(t?.timeLimit ?? '1'),
      assertions: Array.isArray(t?.assertions) ? t.assertions.map(coerceUTAssertion) : []
    }
  }
  async function generateWithAI() {
    aiGenerating = true
    err = ''
    hasAIBuilder = false
    teacherRun = null
    try {
      const payload: any = { instructions: aiInstructions }
      if (aiAuto) {
        payload.auto_tests = true
      } else {
        payload.num_tests = parseInt(aiNumTests) || 5
      }
      const res = await apiJSON(`/api/assignments/${id}/tests/ai-generate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })
      aiCode = res.python || ''
      if (res.builder && res.builder.class_name) {
        try {
          utClassName = String(res.builder.class_name)
          const testsRaw = Array.isArray(res.builder.tests) ? res.builder.tests : JSON.parse(res.builder.tests || '[]')
          utTests = (testsRaw || []).map(coerceUTTest)
          hasAIBuilder = utTests.length > 0
        } catch (e) {
          hasAIBuilder = false
        }
        refreshPreview()
      }
    } catch (e: any) {
      err = e.message
    } finally {
      aiGenerating = false
    }
  }
  async function uploadAIUnitTestsCode() {
    if (!aiCode.trim()) return
    try {
      const blob = new Blob([aiCode], { type: 'text/x-python' })
      const file = new File([blob], 'ai_tests.py', { type: 'text/x-python' })
      const fd = new FormData()
      fd.append('file', file)
      await apiFetch(`/api/assignments/${id}/tests/upload`, { method: 'POST', body: fd })
      await load()
    } catch (e: any) {
      err = e.message
    }
  }
  async function runTeacherSolution() {
    if (!teacherSolutionFile) return
    teacherRunLoading = true
    err = ''
    try {
      const fd = new FormData()
      fd.append('file', teacherSolutionFile)
      const res = await apiJSON(`/api/assignments/${id}/solution-run`, { method: 'POST', body: fd })
      teacherRun = res
    } catch (e: any) {
      err = e.message
    } finally {
      teacherRunLoading = false
    }
  }

  async function uploadGeneratedUnitTests() {
    try {
      const code = generateUnittestCode()
      const blob = new Blob([code], { type: 'text/x-python' })
      const file = new File([blob], 'generated_tests.py', { type: 'text/x-python' })
      const fd = new FormData()
      fd.append('file', file)
      await apiFetch(`/api/assignments/${id}/tests/upload`, { method: 'POST', body: fd })
      await load()
      // After upload, adjust weights/time limits per created unittest method
      const nameToCfg = new Map<string, { weight: number; time: number }>()
      for (const t of utTests) {
        const methodName = t.name.startsWith('test_') ? t.name : `test_${t.name}`
        const fullName = `${utClassName}.${methodName}`
        const w = parseFloat(t.weight)
        const s = parseFloat(t.timeLimit)
        nameToCfg.set(fullName, { weight: isNaN(w) ? 1 : w, time: isNaN(s) ? 1 : s })
      }
      for (const t of tests) {
        if (t.unittest_name && nameToCfg.has(t.unittest_name)) {
          const cfg = nameToCfg.get(t.unittest_name)!
          const testData: any = {
            stdin: t.stdin ?? '',
            expected_stdout: t.expected_stdout ?? '',
            time_limit_sec: cfg.time
          }
          
          // Only include weight for weighted assignments
          if (assignment?.grading_policy === 'weighted') {
            testData.weight = cfg.weight
          }
          
          await apiFetch(`/api/tests/${t.id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(testData)
          })
        }
      }
      await load()
      utShowPreview = false
      err = ''
    } catch (e: any) {
      err = e.message
    }
  }

  async function delTest(tid: number) {
    const confirmed = await confirmModal.open({
      title: 'Delete test',
      body: 'This test will be removed for all students.',
      confirmLabel: 'Delete',
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    })
    if (!confirmed) return
    try {
      await apiFetch(`/api/tests/${tid}`, { method: 'DELETE' })
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  async function deleteAllTests() {
    const confirmed = await confirmModal.open({
      title: 'Delete all tests',
      body: 'All tests for this assignment will be permanently deleted. This cannot be undone.',
      confirmLabel: 'Delete all',
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    })
    if (!confirmed) return
    try {
      await apiFetch(`/api/assignments/${id}/tests`, { method: 'DELETE' })
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  async function updateTest(t: any) {
    try {
      const testData: any = {
        stdin: t.stdin,
        expected_stdout: t.expected_stdout,
        time_limit_sec: parseFloat(t.time_limit_sec) || undefined
      }
      
      // Only include weight for weighted assignments
      if (assignment?.grading_policy === 'weighted') {
        testData.weight = parseFloat(t.weight) || 1
      }
      
      await apiFetch(`/api/tests/${t.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(testData)
      })
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  async function saveLLMSettings(){
    try{
      await apiFetch(`/api/assignments/${id}` ,{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({
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
          llm_strictness: Number.isFinite(llmStrictness as any) ? Math.min(100, Math.max(0, Number(llmStrictness))) : 50,
          llm_rubric: llmRubric.trim() ? llmRubric : null
        })
      })
      await load()
    }catch(e:any){ err=e.message }
  }
</script>

{#if role !== 'teacher' && role !== 'admin'}
  <div class="alert alert-error">
    <span>You do not have permission to manage tests.</span>
  </div>
{:else}
  {#if !assignment}
    <div class="flex items-center gap-3">
      <span class="loading loading-spinner loading-md"></span>
      <p>Loading…</p>
    </div>
  {:else}
    <div class="mb-4 flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-semibold">Manage tests — {assignment.title}</h1>
        {#if assignment.llm_interactive}
          <p class="text-sm opacity-70">Configure AI testing (LLM-Interactive) for this assignment.</p>
        {:else}
          <p class="text-sm opacity-70">Create IO tests, build Python unittest-based tests, or use AI to generate them.</p>
        {/if}
        {#if assignment?.manual_review}
          <div class="alert alert-info mt-2">
            <span>Manual review is enabled for this assignment. Tests are optional and won't auto-assign points.</span>
          </div>
        {/if}
      </div>
      <a class="btn" href={`/assignments/${id}`}>Back to assignment</a>
    </div>
    <div class="card-elevated p-6 space-y-6">
      {#if assignment.llm_interactive}
        <div class="grid gap-4">
          <div class="space-y-3">
            <div class="divider">AI testing settings</div>
            <div class="grid sm:grid-cols-2 gap-3">
              <label class="flex items-center gap-2 sm:col-span-2">
                <input type="checkbox" class="checkbox" bind:checked={llmFeedback}>
                <span class="label-text">LLM feedback visible to students</span>
              </label>
              <label class="flex items-center gap-2">
                <input type="checkbox" class="checkbox" bind:checked={llmAutoAward}>
                <span class="label-text">Auto-award full points if all scenarios pass</span>
              </label>
              <label class="form-control w-full sm:col-span-2">
                <span class="label-text">Scenarios JSON (optional)</span>
                <textarea class="textarea textarea-bordered h-40" bind:value={llmScenarios} placeholder={exampleScenario}></textarea>
              </label>
              <div class="sm:col-span-2 grid gap-3">
                <label class="form-control w-full">
                  <div class="flex items-center justify-between">
                    <span class="label-text">Strictness level</span>
                    <div class="text-sm opacity-70">{llmStrictness} / 100</div>
                  </div>
                  <input type="range" min="0" max="100" step="10" class="range range-primary" bind:value={llmStrictness}>
                  <div class="flex justify-between text-xs opacity-70 mt-1">
                    <span>Beginner</span>
                    <span>Intermediate</span>
                    <span>Advanced</span>
                    <span>PRO</span>
                  </div>
                </label>
                <label class="form-control w-full">
                  <span class="label-text">Teacher rubric (what is OK vs WRONG)</span>
                  <textarea class="textarea textarea-bordered h-32" bind:value={llmRubric} placeholder="Describe what is acceptable and what should be considered wrong. This text will guide the LLM's evaluation."></textarea>
                </label>
              </div>
              <div class="sm:col-span-2 flex justify-end">
                <button class="btn btn-primary" on:click={saveLLMSettings}><Save size={16}/> Save settings</button>
              </div>
            </div>
          </div>
        </div>
      {:else}
        <div class="alert">
          <div>
            <span class="font-medium">Tip:</span>
            Use <code>student_code(...)</code> in assertions to run the student's program with inputs.
          </div>
        </div>

        <!-- Tabs -->
        <div role="tablist" class="tabs tabs-lifted">
        <input type="radio" name="tests-tab" role="tab" class="tab" aria-label="Existing tests" checked>
        <div role="tabpanel" class="tab-content bg-base-100 border-base-300 rounded-box p-4">
          <div class="flex items-center justify-between mb-2">
            <div class="text-sm opacity-70">{tests?.length || 0} tests</div>
            <button class="btn btn-error btn-sm" on:click={deleteAllTests} disabled={!tests || tests.length === 0}><Trash2 size={14}/> Delete all</button>
          </div>
          <div class="grid gap-3 max-h-[32rem] overflow-y-auto">
            {#each tests as t, i}
              <div class="rounded-xl border border-base-300/60 p-3 space-y-2">
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2 font-semibold">
                    <span class="opacity-70">#{i + 1}</span>
                    {#if t.unittest_name}
                      <span class="badge badge-primary gap-1"><FlaskConical size={14}/> unittest</span>
                      <span class="badge badge-outline ml-1">{t.unittest_name}</span>
                    {:else}
                      <span class="badge badge-secondary gap-1">IO</span>
                    {/if}
                  </div>
                  <div class="flex gap-2">
                    {#if t.unittest_name && t.unittest_code}
                      <button class="btn btn-xs" on:click={() => openEditUnitTest(t)}><Code size={14}/> Edit</button>
                    {/if}
                    <button class="btn btn-xs" on:click={() => updateTest(t)}><Save size={14}/> Save</button>
                    <button class="btn btn-xs btn-error" on:click={() => delTest(t.id)}><Trash2 size={14}/> Delete</button>
                  </div>
                </div>
                {#if !t.unittest_name}
                  <div class="grid sm:grid-cols-2 gap-2">
                    <label class="form-control w-full space-y-1">
                      <span class="label-text">Input</span>
                      <input class="input input-bordered w-full" placeholder="stdin" bind:value={t.stdin}>
                    </label>
                    <label class="form-control w-full space-y-1">
                      <span class="label-text">Expected output</span>
                      <input class="input input-bordered w-full" placeholder="expected stdout" bind:value={t.expected_stdout}>
                    </label>
                  </div>
                {:else}
                  {#if t.unittest_code && t.unittest_name}
                    <details class="mt-1">
                      <summary class="cursor-pointer text-sm opacity-70 flex items-center gap-1"><Eye size={14}/> View test method code</summary>
                      <pre class="mt-2 whitespace-pre-wrap text-xs opacity-80 p-2 rounded-lg bg-base-200">{extractMethodFromUnittest(t.unittest_code, t.unittest_name)}</pre>
                    </details>
                  {/if}
                {/if}
                <div class="grid gap-2" class:sm:grid-cols-2={assignment?.grading_policy === 'weighted'}>
                  <label class="form-control w-full space-y-1">
                    <span class="label-text flex items-center gap-1"><Clock size={14}/> <span>Time limit (s)</span></span>
                    <input class="input input-bordered w-full" placeholder="seconds" bind:value={t.time_limit_sec}>
                  </label>
                  {#if assignment?.grading_policy === 'weighted'}
                    <label class="form-control w-full space-y-1">
                      <span class="label-text flex items-center gap-1"><Scale size={14}/> <span>Points</span></span>
                      <input class="input input-bordered w-full" placeholder="points" bind:value={t.weight}>
                    </label>
                  {/if}
                </div>
              </div>
            {/each}
            {#if !(tests && tests.length)}<p><i>No tests</i></p>{/if}
          </div>
        </div>

        <input type="radio" name="tests-tab" role="tab" class="tab" aria-label="Add IO test">
        <div role="tabpanel" class="tab-content bg-base-100 border-base-300 rounded-box p-4">
          <div class="border-base-300/60 space-y-2">
            <h4 class="font-semibold flex items-center gap-2"><Code size={18}/> Add IO test</h4>
            <div class="grid gap-2" class:sm:grid-cols-2={assignment?.grading_policy === 'weighted'}>
              <label class="form-control w-full space-y-1">
                <span class="label-text">Input</span>
                <input class="input input-bordered w-full" placeholder="stdin" bind:value={tStdin}>
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text">Expected output</span>
                <input class="input input-bordered w-full" placeholder="expected stdout" bind:value={tStdout}>
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text flex items-center gap-1"><Clock size={14}/> <span>Time limit (s)</span></span>
                <input class="input input-bordered w-full" placeholder="seconds" bind:value={tLimit}>
              </label>
              {#if assignment?.grading_policy === 'weighted'}
                <label class="form-control w-full space-y-1">
                  <span class="label-text flex items-center gap-1"><Scale size={14}/> <span>Points</span></span>
                  <input class="input input-bordered w-full" placeholder="points" bind:value={tWeight}>
                </label>
              {/if}
            </div>
            <div>
              <button class="btn btn-primary" on:click={addTest} disabled={!tStdin || !tStdout}><Plus size={16}/> Add</button>
            </div>
          </div>
        </div>

        <input type="radio" name="tests-tab" role="tab" class="tab" aria-label="Unittest builder">
        <div role="tabpanel" class="tab-content bg-base-100 border-base-300 rounded-box p-4 space-y-4">
          <div class="grid sm:grid-cols-2 gap-3">
            <label class="form-control w-full space-y-1">
              <span class="label-text">Class name</span>
              <input class="input input-bordered w-full" bind:value={utClassName} placeholder="TestAssignment">
            </label>
            <div class="flex items-end gap-2">
              <button class="btn btn-outline" on:click={addUTTest}><Plus size={16}/> Add test method</button>
              <button class="btn" on:click={() => { utShowPreview = !utShowPreview; refreshPreview() }}><Eye size={16}/> {utShowPreview ? 'Hide' : 'Preview'} code</button>
              <button class="btn btn-primary" disabled={utTests.length === 0} on:click={uploadGeneratedUnitTests}><FlaskConical size={16}/> Create tests</button>
            </div>
          </div>

          <div>
            <button class="btn btn-outline btn-sm" on:click={() => showAdvanced = !showAdvanced}>{showAdvanced ? 'Hide' : 'Advanced: setUp/tearDown'}</button>
            {#if showAdvanced}
              <div class="grid sm:grid-cols-2 gap-3 mt-3">
                <div>
                  <div class="label"><span class="label-text">setUp() (optional)</span></div>
                  <CodeMirror bind:value={utSetup} lang={python()} readOnly={false} />
                </div>
                <div>
                  <div class="label"><span class="label-text">tearDown() (optional)</span></div>
                  <CodeMirror bind:value={utTeardown} lang={python()} readOnly={false} />
                </div>
              </div>
            {/if}
          </div>

          <div class="space-y-3">
            {#each utTests as ut, ti}
              <div class="rounded-xl border border-base-300/60 p-3 space-y-2 ut-method">
                <div class="flex items-center justify-between gap-3 ut-method-header">
                  <div class="grid gap-2 flex-1" class:sm:grid-cols-2={assignment?.grading_policy === 'weighted'}>
                    <label class="form-control w-full space-y-1">
                      <span class="label-text">Method name</span>
                      <input class="input input-bordered w-full" bind:value={ut.name} placeholder="test_something">
                    </label>
                    <label class="form-control w-full space-y-1">
                      <span class="label-text">Description</span>
                      <input class="input input-bordered w-full" bind:value={ut.description} placeholder="What this test checks">
                    </label>
                  </div>
                  <div class="flex items-end gap-2">
                    <button class="btn btn-ghost btn-xs" on:click={() => removeUTTest(ti)}><Trash2 size={14}/> Remove</button>
                  </div>
                </div>
                <div class="grid gap-2" class:sm:grid-cols-2={assignment?.grading_policy === 'weighted'}>
                  <label class="form-control w-full space-y-1">
                    <span class="label-text flex items-center gap-1"><Clock size={14}/> <span>Time limit (s)</span></span>
                    <input class="input input-bordered w-full" bind:value={ut.timeLimit}>
                  </label>
                  {#if assignment?.grading_policy === 'weighted'}
                    <label class="form-control w-full space-y-1">
                      <span class="label-text flex items-center gap-1"><Scale size={14}/> <span>Points</span></span>
                      <input class="input input-bordered w-full" bind:value={ut.weight}>
                    </label>
                  {/if}
                </div>
                <div class="space-y-2 ut-assertions">
                  <div class="flex items-center justify-between">
                    <div class="font-medium text-primary">Assertions</div>
                    <div class="join">
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'equals')}><Plus size={12}/> Equals</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'notEquals')}><Plus size={12}/> Not equals</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'contains')}><Plus size={12}/> Contains</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'notContains')}><Plus size={12}/> Not contains</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'regex')}><Plus size={12}/> Regex</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'raises')}><Plus size={12}/> Raises</button>
                      <button class="btn btn-xs join-item" on:click={() => addUTAssertion(ti, 'custom')}><Plus size={12}/> Custom</button>
                    </div>
                  </div>
                  <div class="space-y-2">
                    {#each ut.assertions as a, ai}
                      <div class="rounded-lg border border-base-300/60 p-2 space-y-2 ut-assertion-item">
                        <div class="flex items-center justify-between">
                          <span class="badge badge-primary">{a.kind}</span>
                          <button class="btn btn-ghost btn-xs" on:click={() => removeUTAssertion(ti, ai)}><Trash2 size={14}/> Remove</button>
                        </div>
                        {#if a.kind === 'custom'}
                          <label class="form-control w-full space-y-1">
                            <span class="label-text">Custom Python (inside test method)</span>
                            <textarea class="textarea textarea-bordered h-24" value={getCustom(a)} on:input={(e) => setCustom(a, (e.target as HTMLTextAreaElement).value)} placeholder="self.assertTrue(...)"></textarea>
                          </label>
                        {:else if a.kind === 'regex'}
                          <div class="grid sm:grid-cols-2 gap-2">
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Inputs (one per line)</span>
                              <textarea class="textarea textarea-bordered h-24" value={getInputs(a)} on:input={(e) => setInputs(a, (e.target as HTMLTextAreaElement).value)} placeholder="2\n3"></textarea>
                            </label>
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Regex pattern</span>
                              <input class="input input-bordered w-full" value={getPattern(a)} on:input={(e) => setPattern(a, (e.target as HTMLInputElement).value)} placeholder="^5$">
                            </label>
                          </div>
                        {:else if a.kind === 'raises'}
                          <div class="grid sm:grid-cols-2 gap-2">
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Inputs (one per line)</span>
                              <textarea class="textarea textarea-bordered h-24" value={getInputs(a)} on:input={(e) => setInputs(a, (e.target as HTMLTextAreaElement).value)} placeholder="bad\ninput"></textarea>
                            </label>
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Exception type</span>
                              <input class="input input-bordered w-full" value={getException(a)} on:input={(e) => setException(a, (e.target as HTMLInputElement).value)} placeholder="ValueError">
                            </label>
                          </div>
                        {:else}
                          <div class="grid sm:grid-cols-2 gap-2">
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Inputs (one per line)</span>
                              <textarea class="textarea textarea-bordered h-24" value={getInputs(a)} on:input={(e) => setInputs(a, (e.target as HTMLTextAreaElement).value)} placeholder="2\n3"></textarea>
                            </label>
                            <label class="form-control w-full space-y-1">
                              <span class="label-text">Expected</span>
                              <input class="input input-bordered w-full" value={getExpected(a)} on:input={(e) => setExpected(a, (e.target as HTMLInputElement).value)} placeholder="5">
                            </label>
                          </div>
                        {/if}
                      </div>
                    {/each}
                    {#if ut.assertions.length === 0}
                      <p class="text-sm opacity-70">Add assertions to this test.</p>
                    {/if}
                  </div>
                </div>
              </div>
            {/each}
          </div>

          {#if utShowPreview}
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <h4 class="font-semibold">Generated Python</h4>
                <div class="join">
                  <button class="btn btn-sm join-item" on:click={refreshPreview}><Code size={14}/> Refresh</button>
                  <button class="btn btn-sm join-item" on:click={copyPreview}><Copy size={14}/> {copiedPreview ? 'Copied' : 'Copy'}</button>
                </div>
              </div>
              <CodeMirror bind:value={utPreviewCode} lang={python()} readOnly={true} />
            </div>
          {/if}
        </div>

        <input type="radio" name="tests-tab" role="tab" class="tab" aria-label="AI generate">
        <div role="tabpanel" class="tab-content bg-base-100 border-base-300 rounded-box p-4 space-y-4">
          <div class="grid sm:grid-cols-3 gap-3">
            <label class="form-control w-full space-y-1">
              <span class="label-text">Test count mode</span>
              <div class="join">
                <button type="button" class="btn join-item {aiAuto ? 'btn-primary' : 'btn-outline'}" on:click={() => aiAuto = true}>Auto</button>
                <button type="button" class="btn join-item {aiAuto ? 'btn-outline' : 'btn-primary'}" on:click={() => aiAuto = false}>Manual</button>
              </div>
              {#if !aiAuto}
                <div class="mt-2">
                  <input type="number" min="1" class="input input-bordered w-full" bind:value={aiNumTests} placeholder="How many tests?">
                </div>
              {/if}
              <span class="text-xs opacity-70">Auto lets the model decide the right number of tests.</span>
            </label>
            <div class="sm:col-span-2">
              <label class="form-control w-full space-y-1">
                <span class="label-text">Additional instructions (optional)</span>
                <input class="input input-bordered w-full" bind:value={aiInstructions} placeholder="Edge cases to cover, constraints, etc.">
              </label>
            </div>
          </div>
          <div class="flex gap-2">
            <button class="btn btn-primary" on:click={generateWithAI} disabled={aiGenerating}><FlaskConical size={16}/> {aiGenerating ? 'Generating…' : 'Generate with AI'}</button>
            <button class="btn" on:click={uploadAIUnitTestsCode} disabled={!aiCode}><UploadIcon size={16}/> Save as tests</button>
          </div>
          {#if aiCode}
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <h4 class="font-semibold">AI Python (editable)</h4>
                <span class="text-xs opacity-70">You can edit this before saving.</span>
              </div>
              <CodeMirror bind:value={aiCode} lang={python()} readOnly={false} />
            </div>
          {/if}
          {#if hasAIBuilder}
            <div class="alert">
              <span>AI also prepared a builder structure below. You can tweak it in the Unittest builder tab and upload from there.</span>
            </div>
          {/if}

          <div class="divider">Optional: test on teacher solution</div>
          <div class="grid sm:grid-cols-2 gap-3 items-end">
            <div>
              <h4 class="font-semibold mb-2 flex items-center gap-2"><UploadIcon size={18}/> Upload teacher solution</h4>
              <input type="file" accept=".py,.zip" class="file-input file-input-bordered w-full" on:change={(e) => (teacherSolutionFile = (e.target as HTMLInputElement).files?.[0] || null)}>
            </div>
            <div class="flex gap-2">
              <button class="btn" disabled={!teacherSolutionFile || teacherRunLoading} on:click={runTeacherSolution}><FlaskConical size={16}/> {teacherRunLoading ? 'Running…' : 'Run tests on solution'}</button>
            </div>
          </div>
          {#if teacherRun}
            <div class="mt-2 rounded-xl border border-base-300/60 p-3 space-y-2">
              <div class="font-medium">Results: {teacherRun.passed}/{teacherRun.total} passed</div>
              <div class="grid gap-2 max-h-64 overflow-y-auto">
                {#each teacherRun.results as r}
                  <div class="rounded-lg border border-base-300/60 p-2 text-sm">
                    <div class="flex items-center justify-between">
                      <div>
                        <span class="badge mr-2">#{r.test_case_id}</span>
                        {#if r.unittest_name}<span class="badge badge-primary">{r.unittest_name}</span>{/if}
                      </div>
                      <span class="badge {r.status === 'passed' ? 'badge-success' : 'badge-error'}">{r.status}</span>
                    </div>
                    {#if r.stderr}
                      <pre class="mt-1 whitespace-pre-wrap opacity-80">{r.stderr}</pre>
                    {/if}
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </div>

        <input type="radio" name="tests-tab" role="tab" class="tab" aria-label="Upload .py">
        <div role="tabpanel" class="tab-content bg-base-100 border-base-300 rounded-box p-4">
          <h4 class="font-semibold mb-2 flex items-center gap-2"><FileUp size={18}/> Upload unittest file</h4>
          <input type="file" accept=".py" class="file-input file-input-bordered w-full" on:change={(e) => (unittestFile = (e.target as HTMLInputElement).files?.[0] || null)}>
          <div class="mt-2">
            <button class="btn" on:click={uploadUnitTests} disabled={!unittestFile}><UploadIcon size={16}/> Upload</button>
          </div>
          <p class="text-xs opacity-70 mt-2">Each method named <code>test_*</code> in classes derived from <code>unittest.TestCase</code> will become a separate test. Use <code>student_code(...)</code> to run the student's program with inputs.</p>
        </div>
        </div>
      {/if}
    </div>
  {/if}
{/if}

{#if err}
  <div class="alert alert-error mt-4"><span>{err}</span></div>
{/if}

<dialog bind:this={editDialog} class="modal">
  <div class="modal-box w-11/12 max-w-4xl space-y-3">
    <h3 class="font-semibold">Edit unittest method</h3>
    <p class="text-xs opacity-70">{editingTest?.unittest_name}</p>
    <CodeMirror bind:value={editingMethodCode} lang={python()} readOnly={false} />
    <div class="modal-action">
      <button class="btn" on:click={saveEditUnitTest}><Save size={16}/> Save</button>
      <form method="dialog">
        <button class="btn btn-ghost" on:click={closeEditUnitTest}>Close</button>
      </form>
    </div>
  </div>
</dialog>

<ConfirmModal bind:this={confirmModal} />

<style>
  :global(.card-elevated){
    border-radius: 1rem;
    border: 1px solid color-mix(in oklab, currentColor 20%, transparent);
    background: var(--fallback-b1, oklch(var(--b1)));
    box-shadow: 0 8px 24px rgba(0,0,0,.06);
  }
  :global(.ut-method){
    border-color: color-mix(in oklab, oklch(var(--p)) 28%, oklch(var(--bc)) 72%);
    background: color-mix(in oklab, var(--fallback-b1, oklch(var(--b1))) 94%, oklch(var(--p)) 6%);
  }
  :global(.ut-method-header){
    padding: .25rem .25rem;
    border-radius: .5rem;
    background: color-mix(in oklab, oklch(var(--p)) 10%, transparent);
  }
  :global(.ut-assertions){
    position: relative;
    margin-left: .25rem;
    padding-left: .75rem;
    border-left: 3px solid oklch(var(--p));
    background: color-mix(in oklab, transparent 92%, oklch(var(--p)) 8%);
    border-radius: .5rem;
    padding-top: .5rem;
    padding-bottom: .5rem;
  }
  :global(.ut-assertion-item){
    background: color-mix(in oklab, var(--fallback-b1, oklch(var(--b1))) 90%, oklch(var(--p)) 10%);
    border-color: color-mix(in oklab, oklch(var(--p)) 40%, oklch(var(--bc)) 60%);
  }
</style>

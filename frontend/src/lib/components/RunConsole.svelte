<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import CodeMirror from '$lib/components/ui/CodeMirror.svelte';
  import { python } from '@codemirror/lang-python';
  import { Plus, Trash2, Play, Code as CodeIcon } from 'lucide-svelte';
  import { t, translator } from '$lib/i18n';

  export let submissionId: number;

  let ws: WebSocket | null = null;
  let connected = false;
  let connecting = false;
  let running = false;
  let scrollEl: HTMLDivElement;
  let inputEl: HTMLInputElement;
  let inputValue = '';
  let timeoutMs: number = 60000; // default 60s

  let translate = t;
  $: translate = $translator;

  type FnParameter = { name: string; type?: string };
  type FnReturn = { name: string; type?: string };
  type FnCase = { name: string; args: string[]; returns: string[]; timeLimit: string };
  type FnMeta = { name: string; params: FnParameter[]; returns: FnReturn[] };

  let fnSignature = '';
  let fnSignatureError = '';
  let fnMeta: FnMeta | null = null;
  let fnCases: FnCase[] = [];

  function hasHeaderColon(text: string): boolean {
    let depthParen = 0;
    let inString: string | null = null;
    for (let i = 0; i < text.length; i++) {
      const ch = text[i];
      if (inString) {
        if (ch === inString && text[i - 1] !== '\\') {
          inString = null;
        }
        continue;
      }
      if (ch === '"' || ch === "'") {
        inString = ch;
        continue;
      }
      if (ch === '(') {
        depthParen++;
        continue;
      }
      if (ch === ')') {
        if (depthParen > 0) depthParen--;
        continue;
      }
      if (ch === ':' && depthParen === 0) {
        return true;
      }
    }
    return false;
  }

  function splitTopLevel(input: string, separator = ','): string[] {
    const parts: string[] = [];
    let current = '';
    let depthParen = 0;
    let depthBracket = 0;
    let depthBrace = 0;
    let depthAngle = 0;
    let inString: string | null = null;

    for (let i = 0; i < input.length; i++) {
      const ch = input[i];
      if (inString) {
        current += ch;
        if (ch === inString && input[i - 1] !== '\\') {
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
        case '(':
          depthParen++;
          break;
        case ')':
          if (depthParen > 0) depthParen--;
          break;
        case '[':
          depthBracket++;
          break;
        case ']':
          if (depthBracket > 0) depthBracket--;
          break;
        case '{':
          depthBrace++;
          break;
        case '}':
          if (depthBrace > 0) depthBrace--;
          break;
        case '<':
          depthAngle++;
          break;
        case '>':
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
        current = '';
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
    let result = '';
    for (let i = 0; i < line.length; i++) {
      const ch = line[i];
      if (inString) {
        result += ch;
        if (ch === inString && line[i - 1] !== '\\') {
          inString = null;
        }
        continue;
      }
      if (ch === '"' || ch === "'") {
        inString = ch;
        result += ch;
        continue;
      }
      if (ch === '#') {
        break;
      }
      result += ch;
    }
    return result.trimEnd();
  }

  function parseFunctionSignatureBlock(block: string): { meta: FnMeta | null; error: string } {
    const raw = String(block || '').replace(/\r\n?/g, '\n');
    const lines = raw.split('\n').map((l) => l.trim()).filter((l) => l);
    if (lines.length === 0) {
      return { meta: null, error: '' };
    }
    const defIdx = lines.findIndex((l) => l.startsWith('def ') || l.startsWith('async def '));
    if (defIdx === -1) {
      return { meta: null, error: translate('frontend/src/lib/components/RunConsole.svelte::add-python-function-definition-for-example-def-solve-data') };
    }
    let signature = lines[defIdx];
    let depth = (signature.match(/\(/g) || []).length - (signature.match(/\)/g) || []).length;
    let cursor = defIdx + 1;
    while (cursor < lines.length && (depth > 0 || !hasHeaderColon(signature))) {
      const part = lines[cursor];
      signature += ' ' + part;
      depth += (part.match(/\(/g) || []).length - (part.match(/\)/g) || []).length;
      cursor++;
    }
    signature = stripInlineComment(signature);
    const colonIndex = signature.lastIndexOf(':');
    if (colonIndex === -1) {
      return { meta: null, error: translate('frontend/src/lib/components/RunConsole.svelte::could-not-parse-function-definition-use-python-syntax') };
    }
    let header = signature.slice(0, colonIndex).trim();
    if (header.startsWith('async ')) {
      header = header.slice('async '.length).trimStart();
    }
    if (!header.startsWith('def ')) {
      return { meta: null, error: translate('frontend/src/lib/components/RunConsole.svelte::could-not-parse-function-definition-use-python-syntax') };
    }
    const nameStart = header.indexOf('def ') + 4;
    const openIdx = header.indexOf('(', nameStart);
    const name = header.slice(nameStart, openIdx).trim();
    if (!name || !/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(name)) {
      return { meta: null, error: translate('frontend/src/lib/components/RunConsole.svelte::could-not-parse-function-definition-use-python-syntax') };
    }
    const closeIdx = header.lastIndexOf(')');
    if (openIdx === -1 || closeIdx === -1 || closeIdx < openIdx) {
      return { meta: null, error: translate('frontend/src/lib/components/RunConsole.svelte::could-not-parse-function-definition-use-python-syntax') };
    }
    const paramsRaw = header.slice(openIdx + 1, closeIdx);
    const afterParams = header.slice(closeIdx + 1).trim();
    let returnRaw: string | undefined;
    if (!afterParams) {
      returnRaw = '';
    } else if (afterParams.startsWith('->')) {
      returnRaw = afterParams.slice(2).trim();
    } else {
      return { meta: null, error: translate('frontend/src/lib/components/RunConsole.svelte::could-not-parse-function-definition-use-python-syntax') };
    }
    const params: FnParameter[] = [];
    if (paramsRaw.trim()) {
      const pieces = splitTopLevel(paramsRaw);
      for (const piece of pieces) {
        const part = piece.trim();
        if (!part) continue;
        if (part.startsWith('*')) {
          return { meta: null, error: translate('frontend/src/lib/components/RunConsole.svelte::only-positional-arguments-supported-in-builder') };
        }
        const [namePartRaw, typePartRaw] = part.split(':').map((p) => p.trim());
        const namePart = namePartRaw.split('=')[0].trim();
        if (!namePart || !/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(namePart)) {
          return { meta: null, error: translate('frontend/src/lib/components/RunConsole.svelte::argument_invalid_parameter_prefix') + namePartRaw + translate('frontend/src/lib/components/RunConsole.svelte::argument_invalid_parameter_suffix') };
        }
        const typePart = typePartRaw ? typePartRaw.trim() : undefined;
        params.push({ name: namePart, type: typePart });
      }
    }
    let returns: FnReturn[] = [];
    const ret = (returnRaw ?? '').trim();
    if (!ret) {
      returns = [{ name: 'return', type: undefined }];
    } else if (/^none$/i.test(ret)) {
      returns = [];
    } else if (ret.startsWith('(') && ret.endsWith(')')) {
      const inner = ret.slice(1, -1);
      const parts = splitTopLevel(inner);
      if (parts.length === 0) {
        returns = [];
      } else {
        returns = parts.map((p, idx) => ({ name: `value${idx + 1}`, type: p }));
      }
    } else {
      returns = [{ name: 'return', type: ret }];
    }
    return { meta: { name, params, returns }, error: '' };
  }

  function describeTypeControl(type?: string): { control: 'text' | 'textarea' | 'number' | 'integer' | 'boolean'; placeholder?: string } {
    const t = String(type || '').toLowerCase();
    if (!t) return { control: 'text' };
    if (/(bool|boolean)/.test(t)) return { control: 'boolean' };
    if (/(int|integer|long)/.test(t)) return { control: 'integer' };
    if (/(float|double|decimal)/.test(t)) return { control: 'number' };
    if (/(list|tuple|set|dict|map|array|json|sequence)/.test(t)) return { control: 'textarea' };
    if (/(str|string|char)/.test(t)) return { control: 'text' };
    return { control: 'text' };
  }

  function ensureCaseShape(c: FnCase, meta: FnMeta | null): FnCase {
    const argCount = meta?.params.length ?? 0;
    const returnCount = meta ? meta.returns.length : 0;
    const adjustedArgs = [...(c.args ?? [])];
    const adjustedReturns = [...(c.returns ?? [])];
    while (adjustedArgs.length < argCount) adjustedArgs.push('');
    if (adjustedArgs.length > argCount) adjustedArgs.length = argCount;
    while (adjustedReturns.length < returnCount) adjustedReturns.push('');
    if (adjustedReturns.length > returnCount) adjustedReturns.length = returnCount;
    return {
      name: c.name,
      args: adjustedArgs,
      returns: adjustedReturns,
      timeLimit: c.timeLimit ?? '1'
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
    return a.name === b.name && arraysEqual(a.args, b.args) && arraysEqual(a.returns, b.returns) && a.timeLimit === b.timeLimit;
  }

  function createEmptyCase(meta: FnMeta | null, idx: number): FnCase {
    const argCount = meta?.params.length ?? 0;
    const returnCount = meta ? meta.returns.length : 0;
    const args = Array.from({ length: argCount }, () => '');
    const returns = Array.from({ length: returnCount }, () => '');
    return {
      name: translate('frontend/src/lib/components/RunConsole.svelte::case_prefix') + (idx + 1),
      args,
      returns,
      timeLimit: '1'
    };
  }

  function parseJSONish(value: string): any {
    const text = value.trim();
    if (!text) return null;
    const normalized = text
      .replace(/'/g, '"')
      .replace(/\bTrue\b/g, 'true')
      .replace(/\bFalse\b/g, 'false')
      .replace(/\bNone\b/g, 'null');
    try {
      return JSON.parse(normalized);
    } catch (err) {
      return undefined;
    }
  }

  function coerceValueForType(raw: string, typeHint?: string): any {
    const value = raw.trim();
    if (!value) return null;
    const hint = String(typeHint || '').toLowerCase();
    if (/(bool|boolean)/.test(hint)) {
      if (/^(true|false)$/i.test(value)) return /^true$/i.test(value);
      if (/^[01]$/.test(value)) return value === '1';
    }
    if (/(int|integer|long)/.test(hint)) {
      const parsedInt = parseInt(value, 10);
      if (!Number.isNaN(parsedInt)) return parsedInt;
    }
    if (/(float|double|decimal)/.test(hint)) {
      const parsedFloat = parseFloat(value);
      if (!Number.isNaN(parsedFloat)) return parsedFloat;
    }
    if (/(str|string|char)/.test(hint)) {
      return value;
    }
    if (/(list|tuple|set|dict|map|array|json|sequence)/.test(hint)) {
      const parsed = parseJSONish(value);
      if (parsed !== undefined) return parsed;
    }
    if (hint && hint.includes('none')) {
      return null;
    }
    if (/^(true|false)$/i.test(value)) return /^true$/i.test(value);
    if (/^-?\d+$/.test(value)) {
      const parsedInt = parseInt(value, 10);
      if (!Number.isNaN(parsedInt)) return parsedInt;
    }
    if (/^-?\d*\.\d+$/.test(value)) {
      const parsedFloat = parseFloat(value);
      if (!Number.isNaN(parsedFloat)) return parsedFloat;
    }
    const parsedJSON = parseJSONish(value);
    if (parsedJSON !== undefined) return parsedJSON;
    return value;
  }

  function updateFnCaseMeta(caseIndex: number, key: 'name' | 'timeLimit', value: string) {
    fnCases = fnCases.map((c, idx) => (idx === caseIndex ? { ...c, [key]: value } : c));
  }

  function updateFnArg(caseIndex: number, argIndex: number, value: string) {
    fnCases = fnCases.map((c, idx) => {
      if (idx !== caseIndex) return c;
      const nextArgs = [...c.args];
      nextArgs[argIndex] = value;
      return { ...c, args: nextArgs };
    });
  }

  function updateFnReturn(caseIndex: number, returnIndex: number, value: string) {
    fnCases = fnCases.map((c, idx) => {
      if (idx !== caseIndex) return c;
      const nextReturns = [...c.returns];
      nextReturns[returnIndex] = value;
      return { ...c, returns: nextReturns };
    });
  }

  function stringToBool(value: string): boolean {
    const normalized = String(value ?? '').trim().toLowerCase();
    return normalized === 'true' || normalized === '1' || normalized === 'yes' || normalized === 'on';
  }

  function addFnCase() {
    if (!fnMeta) {
      addOut(translate('frontend/src/lib/components/RunConsole.svelte::define-function-signature-first') + '\n', 'error');
      return;
    }
    fnCases = [...fnCases, createEmptyCase(fnMeta, fnCases.length)];
  }

  function removeFnCase(idx: number) {
    fnCases = fnCases.filter((_, i) => i !== idx);
  }

  $: {
    const { meta, error } = parseFunctionSignatureBlock(fnSignature);
    fnMeta = meta;
    fnSignatureError = error;
    if (meta && fnCases.length) {
      const nextCases = fnCases.map((c, idx) => ensureCaseShape({ ...c, name: c.name || (translate('frontend/src/lib/components/RunConsole.svelte::case_prefix') + (idx + 1)) }, meta));
      const changed = nextCases.some((caseItem, idx) => !casesEqual(caseItem, fnCases[idx]));
      if (changed) {
        fnCases = nextCases;
      }
    }
  }

  type Item = { type: 'stdout'|'stderr'|'sys'|'error'|'input'; data: string };
  let items: Item[] = [];

  // GUI mirroring (noVNC) state
  let showGUI = false;
  let guiBase = '';
  let guiUrl = '';
  let terminalCollapsed = false;

  function wsUrl(): string {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    // Prefer dedicated /ws path for better proxy compatibility; backend provides both.
    return `${proto}://${location.host}/ws/submissions/${submissionId}/run`;
  }

  function ensureWS(): Promise<void> {
    if (connected || connecting) return Promise.resolve();
    return new Promise((resolve) => {
      connecting = true;
      const url = wsUrl();
      ws = new WebSocket(url);
      ws.onopen = () => {
        connected = true;
        connecting = false;
        resolve();
      };
      ws.onclose = () => {
        connected = false;
        running = false;
      };
      ws.onerror = () => {
        addSys(translate('frontend/src/lib/components/RunConsole.svelte::connection-error'));
      };
      ws.onmessage = (e) => {
        try {
          const msg = JSON.parse(typeof e.data === 'string' ? e.data : '');
          switch (msg.type) {
            case 'gui':
              // Server indicates a GUI-capable session; embed noVNC and minimize terminal
              showGUI = true;
              guiBase = String(msg.base || '').replace(/[^\/]$/, '$&/');
              guiUrl = `${guiBase}vnc.html?autoconnect=1&resize=scale&path=websockify`;
              terminalCollapsed = true;
              break;
            case 'started':
              addSys(translate('frontend/src/lib/components/RunConsole.svelte::session-active'));
              running = true;
              break;
            case 'stdout':
              addOut(msg.data ?? '', 'stdout');
              break;
            case 'stderr':
              addOut(msg.data ?? '', 'stderr');
              break;
            case 'error':
              addOut(msg.message ?? translate('frontend/src/lib/components/RunConsole.svelte::error'), 'error');
              running = false;
              break;
            case 'exit':
              if (msg.timedOut) {
                addOut(translate('frontend/src/lib/components/RunConsole.svelte::timed-out'), 'error');
              }
              addSys(
                msg.code === 0
                  ? translate('frontend/src/lib/components/RunConsole.svelte::process-exited-successfully')
                  : translate('frontend/src/lib/components/RunConsole.svelte::process-exited-with-code-x').replace('{code}', msg.code)
              );
              running = false;
              break;
            case 'function_result': {
              const fn = (msg.function ?? 'function').toString();
              const status = (msg.status ?? 'unknown').toString().toUpperCase();
              addSys(translate('frontend/src/lib/components/RunConsole.svelte::function-result-status').replace('{functionName}', fn).replace('{status}', status));
              if (msg.return_repr) {
                addSys(translate('frontend/src/lib/components/RunConsole.svelte::return-repr').replace('{value}', msg.return_repr));
              } else if (msg.return_json) {
                addSys(translate('frontend/src/lib/components/RunConsole.svelte::return-json').replace('{value}', msg.return_json));
              }
              if (msg.expected_json) {
                addSys(translate('frontend/src/lib/components/RunConsole.svelte::expected').replace('{value}', msg.expected_json));
              }
              if (msg.error) {
                addOut(msg.error + '\n', 'error');
              }
              if (msg.stdout) {
                addOut(msg.stdout, 'stdout');
              }
              if (msg.stderr) {
                addOut(msg.stderr, 'stderr');
              }
              break;
            }
          }
        } catch {
          // ignore
        }
      };
    });
  }

  function addOut(s: string, type: Item['type'] = 'stdout') {
    if (!s) return;
    items = [...items, { type, data: s }];
    tickScroll();
  }
  function addSys(s: string) { addOut(s + '\n', 'sys'); }

  function clearOutput() {
    items = [];
  }

  async function execute() {
    clearOutput();
    await ensureWS();
    if (!ws) return;
    ws.send(JSON.stringify({ type: 'execute', timeout_ms: timeoutMs }));
  }

  type FunctionCallConfig = {
    functionName: string;
    args?: any[];
    expected?: any;
    timeoutMs?: number;
  };

  async function sendFunctionCall(config: FunctionCallConfig) {
    await ensureWS();
    if (!ws) return;
    const fn = config.functionName.trim();
    if (!fn) {
      addOut(translate('frontend/src/lib/components/RunConsole.svelte::function-name-required') + '\n', 'error');
      return;
    }
    const payload: Record<string, any> = {
      type: 'call_function',
      function: fn,
      kwargs: '{}'
    };
    if (Array.isArray(config.args)) {
      payload.args = JSON.stringify(config.args);
    }
    if (config.expected !== undefined) {
      payload.expected = JSON.stringify(config.expected);
    }
    if (typeof config.timeoutMs === 'number' && Number.isFinite(config.timeoutMs) && config.timeoutMs > 0) {
      payload.timeout_ms = Math.round(config.timeoutMs);
    }
    ws.send(JSON.stringify(payload));
  }

  function buildCasePayload(caseIndex: number): FunctionCallConfig | null {
    if (!fnMeta) {
      addOut(translate('frontend/src/lib/components/RunConsole.svelte::define-function-signature-first') + '\n', 'error');
      return null;
    }
    const caseItem = fnCases[caseIndex];
    if (!caseItem) {
      return null;
    }
    const argsValues = fnMeta.params.map((param, idx) => coerceValueForType(caseItem.args[idx] ?? '', param.type));
    let expected: any = undefined;
    if (fnMeta.returns.length > 0) {
      const hasValue = fnMeta.returns.some((_, idx) => (caseItem.returns[idx] ?? '').trim().length > 0);
      if (hasValue) {
        const returnValues = fnMeta.returns.map((ret, idx) => coerceValueForType(caseItem.returns[idx] ?? '', ret.type));
        expected = fnMeta.returns.length === 1 ? returnValues[0] : returnValues;
      }
    }
    let timeoutMsValue: number | undefined;
    const seconds = parseFloat(caseItem.timeLimit);
    if (!Number.isNaN(seconds) && seconds > 0) {
      timeoutMsValue = Math.round(seconds * 1000);
    }
    return {
      functionName: fnMeta.name,
      args: argsValues,
      expected,
      timeoutMs: timeoutMsValue
    };
  }

  async function runFnCase(caseIndex: number) {
    const config = buildCasePayload(caseIndex);
    if (!config) return;
    await sendFunctionCall(config);
  }

  function runAllFnCases() {
    if (!fnMeta || fnCases.length === 0) {
      addOut(translate('frontend/src/lib/components/RunConsole.svelte::add-at-least-one-case-to-run') + '\n', 'error');
      return;
    }
    fnCases.forEach((_, idx) => {
      setTimeout(() => {
        runFnCase(idx);
      }, idx * 150);
    });
  }

  function stop() {
    if (!ws) return;
    ws.send(JSON.stringify({ type: 'stop' }));
  }

  function refreshGUI() {
    if (showGUI && guiUrl) {
      // append a cache-buster to force reload
      const u = new URL(guiUrl, location.origin);
      u.searchParams.set('t', String(Date.now()));
      guiUrl = u.pathname + '?' + u.searchParams.toString();
    }
  }

  function sendInput() {
    if (!ws || inputValue.trim() === '' && inputValue !== '') {
      // allow empty line too
    }
    if (!ws) return;
    const data = inputValue + '\n';
    ws.send(JSON.stringify({ type: 'input', data }));
    items = [...items, { type: 'input', data: inputValue + '\n' }];
    inputValue = '';
    tickScroll();
  }

  function tickScroll() {
    // next microtask
    setTimeout(() => {
      if (scrollEl) scrollEl.scrollTop = scrollEl.scrollHeight;
    }, 0);
  }

  onMount(() => {
    // auto-connect so an existing session continues streaming
    ensureWS();
  });
  onDestroy(() => {
    if (ws) { try { ws.close(); } catch {} 
      ws = null; }
  });
</script>

{#if showGUI}
  <div class="rounded-box overflow-hidden border border-cyan-400/30 bg-base-100/60 mb-3">
    <div class="flex items-center justify-between px-4 py-2 bg-base-300/60">
      <div class="flex items-center gap-2">
        <span class="w-2 h-2 rounded-full bg-amber-400 animate-pulse"></span>
        <span class="font-semibold tracking-wide">{translate('frontend/src/lib/components/RunConsole.svelte::gui-window-tkinter')}</span>
        <span class="text-xs opacity-70 ml-2">{translate('frontend/src/lib/components/RunConsole.svelte::novnc-embedded')}</span>
      </div>
      <div class="flex items-center gap-2">
        <button class="btn btn-sm" on:click={() => terminalCollapsed = !terminalCollapsed}>{terminalCollapsed ? translate('frontend/src/lib/components/RunConsole.svelte::expand-terminal') : translate('frontend/src/lib/components/RunConsole.svelte::minimize-terminal')}</button>
        <button class="btn btn-sm btn-outline" on:click={refreshGUI}>{translate('frontend/src/lib/components/RunConsole.svelte::reload-gui')}</button>
      </div>
    </div>
    <div class="gui-frame-wrap">
      <iframe title="GUI" src={guiUrl} class="gui-frame" allow="clipboard-read; clipboard-write"></iframe>
    </div>
  </div>
{/if}

<div class="rounded-box overflow-hidden border border-cyan-400/30 bg-gradient-to-br from-[#0b1220] via-[#0a1a2a] to-[#060b12] shadow-[0_0_35px_rgba(0,255,200,0.18)]">
  <div class="flex items-center justify-between px-4 py-2 bg-base-300/60">
    <div class="flex items-center gap-2">
      <span class="w-2 h-2 rounded-full" class:animate-pulse={running} class:bg-green-400={running} class:bg-gray-400={!running}></span>
      <span class="font-semibold tracking-wide">{translate('frontend/src/lib/components/RunConsole.svelte::manual-run')}</span>
      <span class="text-xs opacity-70 ml-2">{translate('frontend/src/lib/components/RunConsole.svelte::docker-sandboxed')}</span>
    </div>
    <div class="flex items-center gap-2">
      <button class="btn btn-sm btn-primary" on:click={execute} disabled={connecting || running}>{translate('frontend/src/lib/components/RunConsole.svelte::execute')}</button>
      <button class="btn btn-sm btn-secondary" on:click={stop} disabled={!running}>{translate('frontend/src/lib/components/RunConsole.svelte::stop')}</button>
    </div>
  </div>
  <div bind:this={scrollEl} class="font-mono text-sm p-3 overflow-auto" style="height: {showGUI ? (terminalCollapsed ? '5rem' : '14rem') : '18rem'};">
    {#if items.length === 0}
      <div class="opacity-80 text-sm text-gray-200">{translate('frontend/src/lib/components/RunConsole.svelte::click-execute-to-run-script-output-appears-here')}</div>
    {/if}
    {#each items as it}
      {#if it.type === 'stdout'}
        <pre class="whitespace-pre-wrap text-white/90">{it.data}</pre>
      {:else if it.type === 'stderr'}
        <pre class="whitespace-pre-wrap text-red-300">{it.data}</pre>
      {:else if it.type === 'error'}
        <pre class="whitespace-pre-wrap bg-error/10 text-error px-2 py-1 rounded">{it.data}</pre>
      {:else if it.type === 'sys'}
        <pre class="whitespace-pre-wrap text-cyan-300">{it.data}</pre>
      {:else if it.type === 'input'}
        <pre class="whitespace-pre-wrap text-emerald-300">› {it.data}</pre>
      {/if}
    {/each}
  </div>
  <div class="px-3 pb-3">
    <div class="flex items-center gap-2">
      <input
        bind:this={inputEl}
        class="input input-bordered input-sm w-full"
        bind:value={inputValue}
        placeholder={running ? translate('frontend/src/lib/components/RunConsole.svelte::type-input-and-press-enter') : translate('frontend/src/lib/components/RunConsole.svelte::execute-first-to-send-input')}
        on:keydown={(e)=>{ if(e.key==='Enter'){ e.preventDefault(); if(running) sendInput(); } }}
        disabled={!running}
      />
      <button class="btn btn-sm" on:click={sendInput} disabled={!running}>{translate('frontend/src/lib/components/RunConsole.svelte::send')}</button>
    </div>
  </div>
</div>

<div class="card-elevated mt-4 p-4 space-y-4">
  <div class="rounded-2xl border border-base-300/70 bg-base-200/40 p-4 space-y-3">
    <div>
      <h4 class="font-semibold flex items-center gap-2"><CodeIcon size={18}/> {translate('frontend/src/lib/components/RunConsole.svelte::define-function-signature')}</h4>
      <p class="text-sm opacity-70">{translate('frontend/src/lib/components/RunConsole.svelte::write-python-function-header-describe-arguments')}</p>
    </div>
    <CodeMirror bind:value={fnSignature} lang={python()} readOnly={false} placeholder={'def my_function(arr: list, target: int) -> bool'} />
    {#if fnSignatureError}
      <div class="alert alert-error text-sm">{fnSignatureError}</div>
    {:else if fnMeta}
      <div class="flex flex-wrap gap-2 text-xs">
        <span class="badge badge-outline badge-sm">{translate('frontend/src/lib/components/RunConsole.svelte::function_label')} {fnMeta.name}</span>
        {#if fnMeta.params.length}
          {#each fnMeta.params as param}
            <span class="badge badge-outline badge-sm">{param.name}{param.type ? `: ${param.type}` : ''}</span>
          {/each}
        {:else}
          <span class="badge badge-outline badge-sm">{translate('frontend/src/lib/components/RunConsole.svelte::no-arguments')}</span>
        {/if}
        {#if fnMeta.returns.length === 0}
          <span class="badge badge-outline badge-sm">{translate('frontend/src/lib/components/RunConsole.svelte::returns-none')}</span>
        {:else}
          {#each fnMeta.returns as ret, ri}
            <span class="badge badge-outline badge-sm">{fnMeta.returns.length > 1 ? translate('frontend/src/lib/components/RunConsole.svelte::return_n').replace('{n}', ri + 1) : translate('frontend/src/lib/components/RunConsole.svelte::return_label')}{ret.type ? `: ${ret.type}` : ''}</span>
          {/each}
        {/if}
      </div>
    {/if}
  </div>

  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="text-sm opacity-70">{fnCases.length} {fnCases.length === 1 ? translate('frontend/src/lib/components/RunConsole.svelte::case_singular') : translate('frontend/src/lib/components/RunConsole.svelte::case_plural')}</div>
    <div class="flex items-center gap-2 flex-wrap">
      <button class="btn btn-outline" on:click={addFnCase} disabled={!fnMeta}><Plus size={16}/> {translate('frontend/src/lib/components/RunConsole.svelte::add-case')}</button>
      <button class="btn btn-primary" on:click={runAllFnCases} disabled={!fnMeta || fnCases.length === 0}><Play size={16}/> {translate('frontend/src/lib/components/RunConsole.svelte::run-all-cases')}</button>
    </div>
  </div>

  {#if !fnMeta}
    <div class="rounded-xl border border-dashed border-base-300/80 p-6 text-center text-sm opacity-70">{translate('frontend/src/lib/components/RunConsole.svelte::define-function-signature-to-add-test-cases')}</div>
  {:else}
    <div class="space-y-3">
      {#each fnCases as fc, fi}
        <div class="rounded-2xl border border-base-300/70 bg-base-100 p-4 space-y-4 shadow-sm">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <input class="input input-bordered w-full flex-1" value={fc.name} on:input={(e) => updateFnCaseMeta(fi, 'name', (e.target as HTMLInputElement).value)} placeholder={translate('frontend/src/lib/components/RunConsole.svelte::case_prefix') + (fi + 1)} />
            <div class="flex items-center gap-2 flex-wrap">
              <label class="form-control w-32">
                <span class="label-text text-xs">{translate('frontend/src/lib/components/RunConsole.svelte::time-limit-s')}</span>
                <input class="input input-bordered w-full" value={fc.timeLimit} on:input={(e) => updateFnCaseMeta(fi, 'timeLimit', (e.target as HTMLInputElement).value)}/>
              </label>
              <button class="btn btn-primary btn-xs" on:click={() => runFnCase(fi)}><Play size={14}/> {translate('frontend/src/lib/components/RunConsole.svelte::run_button')}</button>
              <button class="btn btn-ghost btn-xs" on:click={() => removeFnCase(fi)}><Trash2 size={14}/> {translate('frontend/src/lib/components/RunConsole.svelte::remove')}</button>
            </div>
          </div>
          {#if fnMeta.params.length}
            <div class="space-y-2">
              <h5 class="text-sm font-semibold">{translate('frontend/src/lib/components/RunConsole.svelte::arguments_heading')}</h5>
              <div class="grid gap-3 md:grid-cols-2">
                {#each fnMeta.params as param, pi}
                  {@const control = describeTypeControl(param.type)}
                  <div class="form-control space-y-1">
                    <span class="label-text text-xs font-semibold uppercase tracking-wide flex items-center gap-2">
                      {param.name}
                      {#if param.type}
                        <span class="badge badge-outline badge-sm">{param.type}</span>
                      {/if}
                    </span>
                    {#if control.control === 'boolean'}
                      <label class="label cursor-pointer justify-start gap-2 rounded-lg border border-base-300/60 px-3 py-2 bg-base-200/60">
                        <input type="checkbox" class="toggle toggle-sm" checked={stringToBool(fc.args[pi])} on:change={(e) => updateFnArg(fi, pi, (e.target as HTMLInputElement).checked ? 'true' : 'false')}/>
                        <span class="label-text text-sm">{stringToBool(fc.args[pi]) ? translate('frontend/src/lib/components/RunConsole.svelte::true_label') : translate('frontend/src/lib/components/RunConsole.svelte::false_label')}</span>
                      </label>
                    {:else if control.control === 'textarea'}
                      <textarea class="textarea textarea-bordered h-24" value={fc.args[pi]} on:input={(e) => updateFnArg(fi, pi, (e.target as HTMLTextAreaElement).value)} placeholder={param.type ?? translate('frontend/src/lib/components/RunConsole.svelte::value_placeholder')}></textarea>
                    {:else}
                      <input class="input input-bordered w-full" type={control.control === 'integer' || control.control === 'number' ? 'number' : 'text'} step={control.control === 'integer' ? '1' : control.control === 'number' ? 'any' : undefined} value={fc.args[pi]} on:input={(e) => updateFnArg(fi, pi, (e.target as HTMLInputElement).value)} placeholder={param.type ?? translate('frontend/src/lib/components/RunConsole.svelte::value_placeholder')} />
                    {/if}
                  </div>
                {/each}
              </div>
            </div>
          {/if}
          {#if fnMeta.returns.length > 0}
            <div class="space-y-2">
              <h5 class="text-sm font-semibold">{fnMeta.returns.length > 1 ? translate('frontend/src/lib/components/RunConsole.svelte::expected-return-values') : translate('frontend/src/lib/components/RunConsole.svelte::expected-return')}</h5>
              <div class="grid gap-3 md:grid-cols-2">
                {#each fnMeta.returns as ret, ri}
                  {@const control = describeTypeControl(ret.type)}
                  <div class="form-control space-y-1">
                    <span class="label-text text-xs font-semibold uppercase tracking-wide flex items-center gap-2">
                      {fnMeta.returns.length > 1 ? translate('frontend/src/lib/components/RunConsole.svelte::return_n').replace('{n}', ri + 1) : translate('frontend/src/lib/components/RunConsole.svelte::return_label')}
                      {#if ret.type}
                        <span class="badge badge-outline badge-sm">{ret.type}</span>
                      {/if}
                    </span>
                    {#if control.control === 'boolean'}
                      <label class="label cursor-pointer justify-start gap-2 rounded-lg border border-base-300/60 px-3 py-2 bg-base-200/60">
                        <input type="checkbox" class="toggle toggle-sm" checked={stringToBool(fc.returns[ri])} on:change={(e) => updateFnReturn(fi, ri, (e.target as HTMLInputElement).checked ? 'true' : 'false')}/>
                        <span class="label-text text-sm">{stringToBool(fc.returns[ri]) ? translate('frontend/src/lib/components/RunConsole.svelte::true_label') : translate('frontend/src/lib/components/RunConsole.svelte::false_label')}</span>
                      </label>
                    {:else if control.control === 'textarea'}
                      <textarea class="textarea textarea-bordered h-24" value={fc.returns[ri]} on:input={(e) => updateFnReturn(fi, ri, (e.target as HTMLTextAreaElement).value)} placeholder={ret.type ?? translate('frontend/src/lib/components/RunConsole.svelte::value_placeholder')}></textarea>
                    {:else}
                      <input class="input input-bordered w-full" type={control.control === 'integer' || control.control === 'number' ? 'number' : 'text'} step={control.control === 'integer' ? '1' : control.control === 'number' ? 'any' : undefined} value={fc.returns[ri]} on:input={(e) => updateFnReturn(fi, ri, (e.target as HTMLInputElement).value)} placeholder={ret.type ?? translate('frontend/src/lib/components/RunConsole.svelte::value_placeholder')} />
                    {/if}
                  </div>
                {/each}
              </div>
            </div>
          {:else}
            <div class="rounded-lg border border-dashed border-base-300/70 bg-base-200/50 px-4 py-3 text-xs opacity-70">
              {translate('frontend/src/lib/components/RunConsole.svelte::function-returns-none-inspect-side-effects')}
            </div>
          {/if}
        </div>
      {/each}
      {#if fnCases.length === 0}
        <div class="rounded-xl border border-dashed border-base-300/80 p-6 text-center text-sm opacity-70">{translate('frontend/src/lib/components/RunConsole.svelte::no-function-cases-yet-add-case')}</div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .rounded-box { border-radius: 14px; }
  .gui-frame-wrap { width: 100%; height: 60vh; background: #0a0f18; }
  .gui-frame { width: 100%; height: 100%; border: 0; display: block; }
</style>

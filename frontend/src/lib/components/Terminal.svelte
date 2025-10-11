<script lang="ts">
  import { onDestroy } from 'svelte';
  import { t, translator } from '$lib/i18n';

  export let submissionId: number;

  let ws: WebSocket | null = null;
  let connected = false;
  let output = '';
  let cmd = '';
  let scrollEl: HTMLDivElement;
  let inputEl: HTMLInputElement;
  let decoder = new TextDecoder();
  const seenTokens = new Set<string>();
  let compgenMode = false;
  let compgenTimer: ReturnType<typeof setTimeout> | null = null;
  const PROMPT = '~% ';

  // simple client-side autosuggestions (history + common commands)
  let history: string[] = [];
  let historyIndex = -1;
  let suggestedFull = '';
  let suggestSuffix = '';
  const commonCommands: string[] = [
    'ls', 'ls -la', 'cat main.py', 'python main.py', 'python3 main.py',
    'pip list', 'pytest -q', 'python -m pytest -q', 'echo "Hello"', 'pwd'
  ];

  let translate;
  $: translate = $translator;

  function wsUrl(): string {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    // Prefer dedicated /ws path for compatibility with strict reverse proxies
    return `${proto}://${location.host}/ws/submissions/${submissionId}/terminal`;
  }

  function start() {
    stop();
    output = '';
    ws = new WebSocket(wsUrl());
    ws.binaryType = 'arraybuffer';
    ws.onopen = () => { connected = true; queueMicrotask(() => inputEl?.focus()); seedSuggestions(); };
    ws.onclose = () => { connected = false };
    ws.onmessage = (ev: MessageEvent) => {
      let text = '';
      if (ev.data instanceof ArrayBuffer) {
        text = decoder.decode(new Uint8Array(ev.data));
      } else if (typeof ev.data === 'string') {
        text = ev.data;
      }
      if (!text) return;
      text = sanitize(text);
      // Remove any remote prompt echoes at the start of lines
      text = text.replace(/(^|\n)~%\s/g, '$1');
      // Remove our own local echo of input lines when bash echoes them back
      // This targets a line that exactly equals the previously sent cmd
      if (lastSent !== null) {
        const escaped = lastSent.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
        const re = new RegExp(`(^|\\n)${escaped}(?=\\n|$)`, 'g')
        text = text.replace(re, '$1')
        lastSent = null
      }
      // filter out hidden compgen block from visible output while indexing tokens
      const START = '__CG_START__'
      const END = '__CG_END__'
      while (text.length > 0) {
        if (!compgenMode) {
          const i = text.indexOf(START)
          if (i === -1) {
            output += text;
            indexTokens(text);
            text = '';
          } else {
            const before = text.slice(0, i);
            if (before) { output += before; indexTokens(before); }
            compgenMode = true;
            text = text.slice(i + START.length);
          }
        } else {
          const j = text.indexOf(END);
          if (j === -1) {
            indexTokens(text);
            text = '';
          } else {
            const inside = text.slice(0, j);
            if (inside) indexTokens(inside);
            compgenMode = false;
            text = text.slice(j + END.length);
          }
        }
      }
      queueMicrotask(() => {
        if (scrollEl) scrollEl.scrollTop = scrollEl.scrollHeight;
      });
    };
  }

  function stop() {
    if (ws) { try { ws.close(); } catch {}
    }
    ws = null;
    connected = false;
  }

  let lastSent: string | null = null;
  function send() {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const line = cmd.endsWith('\n') ? cmd : cmd + '\n';
    if (cmd.trim().length > 0) {
      if (history.length === 0 || history[history.length - 1] !== cmd) history.push(cmd);
      historyIndex = -1;
    }
    // Echo prompt + command locally only if the line is a shell command (not pure input)
    const isPureInput = cmd.trim() === '' || /^(\d+|[A-Za-z]+)$/.test(cmd.trim());
    if (!isPureInput) {
      output += `${PROMPT}${cmd}\n`;
    } else {
      // For stdin-like inputs, do not render a prompt, just the raw input echoed by the program later
      output += `${cmd}\n`;
    }
    ws.send(line);
    lastSent = cmd;
    cmd = '';
    suggestedFull = '';
    suggestSuffix = '';
    queueMicrotask(() => inputEl?.focus());
  }

  function ctrlC() {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(new Uint8Array([3]));
  }

  function clearOut() { output = ''; }

  function updateSuggestion() {
    const q = cmd.trim();
    if (!q) { suggestedFull = ''; suggestSuffix = ''; return; }
    const haystack = [...history].reverse().concat([...seenTokens]).concat(commonCommands);
    const found = haystack.find((s) => s.toLowerCase().startsWith(q.toLowerCase()) && s.toLowerCase() !== q.toLowerCase());
    if (found) {
      suggestedFull = found;
      suggestSuffix = found.slice(cmd.length);
    }
    else {
      suggestedFull = '';
      suggestSuffix = '';
    }
    // Ask bash for dynamic suggestions (commands/files) with throttling
    if (connected) {
      if (compgenTimer) clearTimeout(compgenTimer);
      compgenTimer = setTimeout(() => fetchCompgen(q), 120);
    }
  }

  function indexTokens(text: string) {
    // naive tokenization for filenames/commands to improve suggestions
    for (const raw of text.split(/\s+/)) {
      const token = raw.trim();
      if (!token) continue;
      if (/^[A-Za-z0-9_\.\-/]+$/.test(token) && token.length <= 64) {
        seenTokens.add(token);
      }
    }
  }

  function fetchCompgen(prefix: string) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const escaped = prefix.replace(/'/g, "'\\''");
    const cmd = `echo __CG_START__ && compgen -cdfa -- '${escaped}' | head -n 200 && echo __CG_END__\n`;
    try { ws.send(cmd); } catch {}
  }

  function sanitize(text: string): string {
    // remove bracketed paste codes and similar esacpes
    text = text.replace(/\x1b\[\?2004[hl]/g, '');
    // strip docker bash prompt like nobody@abc:/code$ (with optional ANSI)
    text = text.replace(/(^|\n)(?:\x1b\[[0-9;]*m)*[\w.-]+@[^:]+:\/code\$\s*/g, '$1');
    return text;
  }

  function acceptSuggestion() {
    if (suggestedFull) {
      cmd = suggestedFull;
      suggestedFull = '';
      suggestSuffix = '';
      queueMicrotask(() => inputEl?.setSelectionRange(cmd.length, cmd.length));
    }
  }

  function prevHistory() {
    if (history.length === 0) return;
    if (historyIndex === -1) historyIndex = history.length - 1;
    else if (historyIndex > 0) historyIndex--;
    cmd = history[historyIndex] || '';
  }

  function nextHistory() {
    if (history.length === 0) return;
    if (historyIndex === -1) return;
    if (historyIndex < history.length - 1) {
      historyIndex++;
      cmd = history[historyIndex];
    } else {
      historyIndex = -1;
      cmd = '';
    }
  }

  $: updateSuggestion();
  onDestroy(() => stop());

  function seedSuggestions() {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    // harvest available commands without polluting visible output
    ws.send("echo __CG_START__ && compgen -c | sort -u | head -n 2000 && echo __CG_END__\n");
  }
</script>

<div class="rounded-box p-0 overflow-hidden border border-cyan-400/30 bg-gradient-to-br from-[#0b1220] via-[#0a1a2a] to-[#060b12] shadow-[0_0_35px_rgba(0,255,200,0.18)]">
  <div class="flex items-center justify-between px-4 py-2 bg-base-300/60">
    <div class="flex items-center gap-2">
      <span class="w-2 h-2 rounded-full bg-green-400 animate-pulse"></span>
      <span class="font-semibold tracking-wide">{t('frontend/src/lib/components/Terminal.svelte::interactive_test_session')}</span>
      <span class="text-xs opacity-70 ml-2">{t('frontend/src/lib/components/Terminal.svelte::docker_sandboxed')}</span>
    </div>
    <div class="flex gap-2">
      {#if !connected}
        <button class="btn btn-sm btn-primary" on:click={start}>{translate('frontend/src/lib/components/Terminal.svelte::start_button')}</button>
      {:else}
        <button class="btn btn-sm btn-secondary" on:click={stop}>{translate('frontend/src/lib/components/Terminal.svelte::stop_button')}</button>
      {/if}
      <button class="btn btn-sm" on:click={clearOut}>{translate('frontend/src/lib/components/Terminal.svelte::clear_button')}</button>
      <button class="btn btn-sm btn-outline" on:click={ctrlC} disabled={!connected}>{translate('frontend/src/lib/components/Terminal.svelte::ctrl_c_button')}</button>
    </div>
  </div>
  <div bind:this={scrollEl} class="terminal-area font-mono text-sm p-3 h-72 overflow-auto">
    <pre class="whitespace-pre-wrap text-emerald-200/90">{output}</pre>
    <div class="input-line" class:opacity-50={!connected}>
      <span class="prompt">{PROMPT}</span>
      <div class="input-stack">
        <input
          bind:this={inputEl}
          class="term-input"
          bind:value={cmd}
          placeholder={!connected ? translate('frontend/src/lib/components/Terminal.svelte::start_session_placeholder') : ''}
          on:keydown={(e)=>{
            if(e.key==='Enter'){ e.preventDefault(); send(); }
            else if(e.key==='Tab'){ e.preventDefault(); acceptSuggestion(); }
            else if(e.key==='ArrowUp'){ e.preventDefault(); prevHistory(); }
            else if(e.key==='ArrowDown'){ e.preventDefault(); nextHistory(); }
            else if(e.key==='ArrowRight'){
              const atEnd = inputEl && inputEl.selectionStart === inputEl.selectionEnd && inputEl.selectionEnd === cmd.length
              if(atEnd && suggestSuffix){ e.preventDefault(); acceptSuggestion() }
            }
            else if(e.key==='c' && (e.ctrlKey||e.metaKey)){ e.preventDefault(); ctrlC(); }
          }}
          disabled={!connected}
        >
        {#if suggestSuffix}
          <span class="suggestion" style="left: {cmd.length}ch;">{suggestSuffix}</span>
        {/if}
      </div>
    </div>
  </div>
</div>

<style>
  .rounded-box { border-radius: 14px; }
  .terminal-area {
    background: linear-gradient(180deg, rgba(2,8,18,0.8), rgba(3,10,20,0.85)),
                repeating-linear-gradient(
                  to bottom,
                  rgba(0, 255, 200, 0.035) 0px,
                  rgba(0, 255, 200, 0.035) 2px,
                  rgba(0,0,0,0.0) 3px,
                  rgba(0,0,0,0.0) 6px
                );
    color: #b6ffd8;
  }
  .input-line { display: flex; align-items: center; gap: 0.25rem; }
  .prompt {
    color: #7fffd4;
    text-shadow: 0 0 6px rgba(0,255,200,0.55), 0 0 12px rgba(0,255,200,0.25);
  }
  .input-stack { position: relative; display: inline-block; flex: 1; }
  .term-input {
    width: 100%;
    background: transparent;
    border: none;
    outline: none;
    color: #d2ffe6;
    caret-color: #00ffd1;
  }
  .term-input::placeholder { color: rgba(214, 255, 240, 0.35); }
  .suggestion { position: absolute; top: 0; color: rgba(214,255,240,0.28); pointer-events: none; }
</style>

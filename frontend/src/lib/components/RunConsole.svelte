<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  export let submissionId: number;

  let ws: WebSocket | null = null;
  let connected = false;
  let connecting = false;
  let running = false;
  let scrollEl: HTMLDivElement;
  let inputEl: HTMLInputElement;
  let inputValue = '';
  let timeoutMs: number = 60000; // default 60s

  type Item = { type: 'stdout'|'stderr'|'sys'|'error'|'input'; data: string };
  let items: Item[] = [];

  // GUI mirroring (noVNC) state
  let showGUI = false;
  let guiBase = '';
  let guiUrl = '';
  let terminalCollapsed = false;

  function wsUrl(): string {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    return `${proto}://${location.host}/api/submissions/${submissionId}/run`;
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
        addSys('Connection error');
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
              addSys('Session active.');
              running = true;
              break;
            case 'stdout':
              addOut(msg.data ?? '', 'stdout');
              break;
            case 'stderr':
              addOut(msg.data ?? '', 'stderr');
              break;
            case 'error':
              addOut(msg.message ?? 'Error', 'error');
              running = false;
              break;
            case 'exit':
              if (msg.timedOut) {
                addOut('Timed out.', 'error');
              }
              addSys(`Process exited ${msg.code === 0 ? 'successfully' : 'with code ' + msg.code}.`);
              running = false;
              break;
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
        <span class="font-semibold tracking-wide">GUI window (Tkinter)</span>
        <span class="text-xs opacity-70 ml-2">noVNC embedded</span>
      </div>
      <div class="flex items-center gap-2">
        <button class="btn btn-sm" on:click={() => terminalCollapsed = !terminalCollapsed}>{terminalCollapsed ? 'Expand terminal' : 'Minimize terminal'}</button>
        <button class="btn btn-sm btn-outline" on:click={refreshGUI}>Reload GUI</button>
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
      <span class="font-semibold tracking-wide">Manual run</span>
      <span class="text-xs opacity-70 ml-2">Docker sandboxed</span>
    </div>
    <div class="flex items-center gap-2">
      <button class="btn btn-sm btn-primary" on:click={execute} disabled={connecting || running}>Execute</button>
      <button class="btn btn-sm btn-secondary" on:click={stop} disabled={!running}>Stop</button>
    </div>
  </div>
  <div bind:this={scrollEl} class="font-mono text-sm p-3 overflow-auto" style="height: {showGUI ? (terminalCollapsed ? '5rem' : '14rem') : '18rem'};">
    {#if items.length === 0}
      <div class="opacity-80 text-sm text-gray-200">Click Execute to run the student's script. Output appears here.</div>
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
        placeholder={running ? 'Type input and press Enter…' : 'Execute first to send input'}
        on:keydown={(e)=>{ if(e.key==='Enter'){ e.preventDefault(); if(running) sendInput(); } }}
        disabled={!running}
      />
      <button class="btn btn-sm" on:click={sendInput} disabled={!running}>Send</button>
    </div>
  </div>
</div>

<style>
  .rounded-box { border-radius: 14px; }
  .gui-frame-wrap { width: 100%; height: 60vh; background: #0a0f18; }
  .gui-frame { width: 100%; height: 100%; border: 0; display: block; }
</style>



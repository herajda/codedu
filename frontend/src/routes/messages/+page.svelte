<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch, apiJSON } from '$lib/api';
  import { getKey, encryptText, decryptText } from '$lib/e2ee';
  import { auth } from '$lib/auth';
  import type { User } from '$lib/auth';

  let searchTerm = '';
  let results: User[] = [];
  let convo: any[] = [];
  let convos: any[] = [];
  let target: User | null = null;
  let msg = '';
  let err = '';
  let es: EventSource | null = null;

  const pageSize = 20;
  let offset = 0;
  let hasMore = true;

  async function loadConvos() {
    const r = await apiJSON('/api/conversations');
    const k = getKey();
    for (const c of r) {
      if (k) {
        try { c.text = await decryptText(k, c.content); }
        catch { c.text = '[decrypt error]'; }
      } else {
        c.text = '[locked]';
      }
    }
    convos = Array.isArray(r) ? r : [];
  }

  async function search() {
    const r = await apiJSON(`/api/user-search?q=${encodeURIComponent(searchTerm)}`);
    results = Array.isArray(r) ? r : [];
  }

  async function openChat(u: any) {
    target = u;
    convo = [];
    offset = 0;
    hasMore = true;
    await load();
  }

  async function load(more = false) {
    if (!target) return;
    const list = await apiJSON(`/api/messages/${target.id}?limit=${pageSize}&offset=${offset}`);
    const k = getKey();
    for (const m of list) {
      if (k) {
        try { m.text = await decryptText(k, m.content); }
        catch { m.text = '[decrypt error]'; }
      } else {
        m.text = '[locked]';
      }
    }
    if (more) convo = [...convo, ...list];
    else convo = list;
    offset += list.length;
    if (list.length < pageSize) hasMore = false;
  }

  async function send() {
    err = '';
    if (!target) return;
    const k = getKey();
    if (!k) { err = 'missing key'; return; }
    const ct = await encryptText(k, msg);
    const res = await apiFetch('/api/messages', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({ to: target.id, content: ct })
    });
    if (res.ok) { msg=''; offset=0; await load(); await loadConvos(); }
    else { err = (await res.json()).error; }
  }

  onMount(() => {
    loadConvos();
    es = new EventSource('/api/messages/events');
    es.addEventListener('message', async (ev) => {
      const d = JSON.parse((ev as MessageEvent).data);
      if (target && (d.sender_id === target.id || d.recipient_id === target.id)) {
        const k = getKey();
        if (k) {
          try { d.text = await decryptText(k, d.content); }
          catch { d.text = '[decrypt error]'; }
        } else {
          d.text = '[locked]';
        }
        convo = [...convo, d];
      }
      await loadConvos();
    });
  });

  onDestroy(() => { es?.close(); });
</script>

<h1 class="text-2xl font-bold mb-4">Messages</h1>
<div class="mb-4 space-x-2">
  <input class="input input-bordered" placeholder="Search" bind:value={searchTerm} />
  <button class="btn" on:click={search}>Search</button>
</div>
{#if convos.length}
  <h2 class="font-bold mb-2">Recent</h2>
  {#each convos as c}
    <div class="mb-2">
      <button class="link" on:click={() => openChat(c)}>{c.name ?? c.email}</button>
      <span class="text-sm text-gray-500 ml-2">{c.text}</span>
    </div>
  {/each}
{/if}
{#each results as u}
  <div class="mb-2"><button class="link" on:click={() => openChat(u)}>{u.name ?? u.email}</button></div>
{/each}

{#if target}
  <h2 class="font-bold mb-2">Chat with {target.name ?? target.email}</h2>
  {#if hasMore}
    <button class="btn btn-sm mb-2" on:click={() => load(true)}>Load more</button>
  {/if}
  <div class="space-y-2 max-h-60 overflow-y-auto mb-2 border p-2">
    {#each convo as m}
      <div class={m.sender_id === $auth?.id ? 'text-right' : 'text-left'}>
        <span class="badge badge-outline">{m.text}</span>
      </div>
    {/each}
  </div>
  <div class="flex space-x-2">
    <input class="input input-bordered flex-1" bind:value={msg} />
    <button class="btn" on:click={send}>Send</button>
  </div>
  {#if err}<p class="text-error">{err}</p>{/if}
{/if}

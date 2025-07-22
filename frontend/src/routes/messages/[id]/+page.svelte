<script lang="ts">
  import { onMount, onDestroy, afterUpdate } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { apiFetch, apiJSON } from '$lib/api';
  import { createEventSource } from '$lib/sse';
  import { getKey, encryptText, decryptText } from '$lib/e2ee';
  import { auth } from '$lib/auth';

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    convo = [];
    offset = 0;
    hasMore = true;
    load();
  }
  const name = $page.url.searchParams.get('name') ?? $page.url.searchParams.get('email') ?? id;
  const initial = (name ?? '').charAt(0).toUpperCase();

  let convo: any[] = [];
  let msg = '';
  let err = '';
  let esCtrl: { close: () => void } | null = null;
  let chatBox: HTMLDivElement | null = null;
  const gapMs = 5 * 60 * 1000;

  function showAvatar(idx: number, m: any) {
    if (m.sender_id === $auth?.id) return false;
    if (idx === 0) return true;
    const prev = convo[idx - 1];
    const t1 = new Date(prev.created_at).getTime();
    const t2 = new Date(m.created_at).getTime();
    return prev.sender_id !== m.sender_id || t2 - t1 > gapMs;
  }

  afterUpdate(() => { if (chatBox) chatBox.scrollTop = chatBox.scrollHeight; });

  const pageSize = 20;
  let offset = 0;
  let hasMore = true;

  async function load(more = false) {
    const list = await apiJSON(`/api/messages/${id}?limit=${pageSize}&offset=${offset}`);
    list.reverse();
    const k = getKey();
    for (const m of list) {
      if (k) {
        try { m.text = await decryptText(k, m.content); }
        catch { m.text = '[decrypt error]'; }
      } else {
        m.text = '[locked]';
      }
    }
    if (more) convo = [...list, ...convo];
    else convo = list;
    offset += list.length;
    if (list.length < pageSize) hasMore = false;
  }

  async function send() {
    err = '';
    const k = getKey();
    if (!k) { err = 'missing key'; return; }
    const ct = await encryptText(k, msg);
    const res = await apiFetch('/api/messages', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({ to: parseInt(id), content: ct })
    });
    if (res.ok) { msg=''; offset=0; await load(); }
    else { err = (await res.json()).error; }
  }

  onMount(() => {
    load();
    esCtrl = createEventSource(
      '/api/messages/events',
      (src) => {
        src.addEventListener('message', async (ev) => {
          const d = JSON.parse((ev as MessageEvent).data);
          if (d.sender_id === parseInt(id) || d.recipient_id === parseInt(id)) {
            const k = getKey();
            if (k) {
              try { d.text = await decryptText(k, d.content); }
              catch { d.text = '[decrypt error]'; }
            } else {
              d.text = '[locked]';
            }
            convo = [...convo, d];
          }
        });
      },
      {
        onError: (m) => { err = m; },
        onOpen: () => { err = ''; }
      }
    );
  });

  onDestroy(() => { esCtrl?.close(); });
  function back() { goto('/messages'); }
</script>

<h1 class="text-2xl font-bold mb-4">Chat with {name}</h1>
<button class="btn btn-sm mb-4" on:click={back}>Back</button>
{#if hasMore}
  <button class="btn btn-sm mb-2" on:click={() => load(true)}>Load more</button>
{/if}
<div class="chat-area tight-chat max-h-60 overflow-y-auto mb-2 border p-2" bind:this={chatBox}>
  {#each convo as m, i}
    <div class={`chat ${m.sender_id === $auth?.id ? 'chat-end' : 'chat-start'}`}
    >
      {#if showAvatar(i, m)}
        <div class="chat-image avatar">
          <div class="w-8 rounded-full bg-neutral text-neutral-content flex items-center justify-center">{initial}</div>
        </div>
      {/if}
      <div class={`chat-bubble bubble-tail ${m.sender_id === $auth?.id ? 'chat-bubble-primary self-bubble' : 'chat-bubble-secondary other-bubble'}`}>{m.text}</div>
      <div class="chat-footer chat-timestamp">{new Date(m.created_at).toLocaleString()}</div>
    </div>
  {/each}
</div>
<div class="flex space-x-2">
  <input class="input input-bordered flex-1" bind:value={msg} />
  <button class="btn" on:click={send}>Send</button>
</div>
{#if err}<p class="text-error">{err}</p>{/if}

<script lang="ts">
  import { onMount, onDestroy, afterUpdate } from 'svelte';
  import { page } from '$app/stores';
  import { apiFetch, apiJSON } from '$lib/api';
  import { getKey, encryptText, decryptText } from '$lib/e2ee';
  import { auth } from '$lib/auth';
  import { goto } from '$app/navigation';

  let id = $page.params.id;
  $: if ($page.params.id !== id) { id = $page.params.id; load(); }

  let name = $page.url.searchParams.get('name') ?? '';
  let email = $page.url.searchParams.get('email') ?? '';

  let convo: any[] = [];
  let msg = '';
  let err = '';
  let es: EventSource | null = null;
  let chatBox: HTMLDivElement | null = null;

  const pageSize = 20;
  let offset = 0;
  let hasMore = true;

  afterUpdate(() => {
    if (chatBox) chatBox.scrollTop = chatBox.scrollHeight;
  });

  async function load(more = false) {
    const list = await apiJSON(`/api/messages/${id}?limit=${pageSize}&offset=${offset}`);
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
    const k = getKey();
    if (!k) { err = 'missing key'; return; }
    const ct = await encryptText(k, msg);
    const res = await apiFetch('/api/messages', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({ to: Number(id), content: ct })
    });
    if (res.ok) { msg=''; offset=0; await load(); }
    else { err = (await res.json()).error; }
  }

  onMount(() => {
    load();
    es = new EventSource('/api/messages/events');
    es.addEventListener('message', async (ev) => {
      const d = JSON.parse((ev as MessageEvent).data);
      if (d.sender_id === Number(id) || d.recipient_id === Number(id)) {
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
  });

  onDestroy(() => { es?.close(); });
</script>

<h1 class="text-2xl font-bold mb-4">Chat with {name || email || id}</h1>
<button class="btn btn-sm mb-4" on:click={() => goto('/messages')}>Back</button>

{#if hasMore}
  <button class="btn btn-sm mb-2" on:click={() => load(true)}>Load more</button>
{/if}
<div class="space-y-2 max-h-60 overflow-y-auto mb-2 border p-2" bind:this={chatBox}>
  {#each convo as m}
    <div class={`chat ${m.sender_id === $auth?.id ? 'chat-end' : 'chat-start'}`}>
      <div class={`chat-bubble ${m.sender_id === $auth?.id ? 'chat-bubble-primary' : 'chat-bubble-secondary'}`}>{m.text}</div>
      <div class="chat-footer opacity-50 text-xs">{new Date(m.created_at).toLocaleString()}</div>
    </div>
  {/each}
</div>
<div class="flex space-x-2">
  <input class="input input-bordered flex-1" bind:value={msg} />
  <button class="btn" on:click={send}>Send</button>
</div>
{#if err}<p class="text-error">{err}</p>{/if}

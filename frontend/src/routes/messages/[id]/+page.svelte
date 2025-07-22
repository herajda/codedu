<script lang="ts">
  import { onMount, onDestroy, afterUpdate } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { apiFetch, apiJSON } from '$lib/api';
  import { createEventSource } from '$lib/sse';
  import { getKey, encryptText, decryptText } from '$lib/e2ee';
  import { auth } from '$lib/auth';
  import { Messages, Message, Messagebar } from 'framework7-svelte';

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    convo = [];
    offset = 0;
    hasMore = true;
    load();
  }
  const name = $page.url.searchParams.get('name') ?? $page.url.searchParams.get('email') ?? id;

  let convo: any[] = [];
  let msg = '';
  let err = '';
  let esCtrl: { close: () => void } | null = null;
  let chatBox: HTMLDivElement | null = null;

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

  function messagebarSubmit(e: CustomEvent) {
    send();
    const clear = e.detail?.[1];
    if (typeof clear === 'function') clear();
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
<Messages autoLayout class="max-h-60 overflow-y-auto mb-2 border p-2" bind:this={chatBox}>
  {#each convo as m}
    <Message
      type={m.sender_id === $auth?.id ? 'sent' : 'received'}
      text={m.text}
      footer={new Date(m.created_at).toLocaleString()}
    />
  {/each}
</Messages>
<Messagebar
  placeholder="Message"
  bind:value={msg}
  sendLink="Send"
  on:submit={messagebarSubmit}
/>
{#if err}<p class="text-error">{err}</p>{/if}

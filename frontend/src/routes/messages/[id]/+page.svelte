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
  let name = $page.url.searchParams.get('name') ?? $page.url.searchParams.get('email') ?? '';
  let contactAvatar: string | null = null;

  let convo: any[] = [];
  let msg = '';
  let err = '';
  let esCtrl: { close: () => void } | null = null;
  let chatBox: HTMLDivElement | null = null;
  let msgInput: HTMLTextAreaElement | null = null;

  function adjustHeight() {
    if (msgInput) {
      msgInput.style.height = 'auto';
      msgInput.style.height = msgInput.scrollHeight + 'px';
    }
  }

  $: msg, adjustHeight();

  function formatTime(d: string | number | Date) {
    return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  function formatDate(d: string | number | Date) {
    const date = new Date(d)
    const today = new Date()
    const yesterday = new Date()
    yesterday.setDate(today.getDate() - 1)
    if (date.toDateString() === today.toDateString()) return 'Today'
    if (date.toDateString() === yesterday.toDateString()) return 'Yesterday'
    return date.toLocaleDateString()
  }

  function hyphenateLongWords(text: string, max = 20) {
    return text.replace(new RegExp(`\\S{${max},}`, 'g'), (word) => {
      const parts = []
      for (let i = 0; i < word.length; i += max) parts.push(word.slice(i, i + max))
      return parts.join('\u00AD')
    })
  }

  function sameDate(a: string | number | Date, b: string | number | Date) {
    return new Date(a).toDateString() === new Date(b).toDateString()
  }

  let prevLen = 0;
  let preserveScroll = false;
  let prevHeight = 0;
  let prevTop = 0;
  afterUpdate(() => {
    if (!chatBox) return;
    if (preserveScroll) {
      chatBox.scrollTop = chatBox.scrollHeight - prevHeight + prevTop;
      preserveScroll = false;
      prevLen = convo.length;
    } else if (convo.length !== prevLen) {
      chatBox.scrollTop = chatBox.scrollHeight;
      prevLen = convo.length;
    }
  });

  const pageSize = 20;
  let offset = 0;
  let hasMore = true;

  async function load(more = false) {
    if (more && chatBox) {
      preserveScroll = true;
      prevHeight = chatBox.scrollHeight;
      prevTop = chatBox.scrollTop;
    }
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
      m.showTime = false;
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

  onMount(async () => {
    load();
    try {
      const info = await apiJSON(`/api/users/${id}`);
      contactAvatar = info.avatar ?? null;
      if (!name) name = info.name ?? info.email ?? id;
    } catch {}
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
            d.showTime = false;
            convo = [...convo, d];
          }
        });
      },
      {
        onError: (m) => { err = m; },
        onOpen: () => { err = ''; }
      }
    );
    adjustHeight();
  });

  onDestroy(() => { esCtrl?.close(); });
  function back() { goto('/messages'); }
</script>

<button class="btn btn-sm mb-4" on:click={back}>Back</button>

<div class="card bg-base-100 shadow fixed inset-x-0 bottom-0 top-16 sm:left-60 z-40 flex flex-col">
  <div class="p-4 border-b flex items-center gap-3">
    <div class="avatar">
      <div class="w-10 h-10 rounded-full overflow-hidden">
        {#if contactAvatar}
          <img src={contactAvatar} alt="Contact" class="w-full h-full object-cover" />
        {:else}
          <img src="/placeholder.svg?height=40&width=40" alt="Contact" />
        {/if}
      </div>
    </div>
    <div>
      <h2 class="font-semibold">{name}</h2>
      <p class="text-xs opacity-60">Chat</p>
    </div>
  </div>
  <div class="flex-1 overflow-hidden">
    <div class="h-full overflow-y-auto p-4 space-y-4" bind:this={chatBox}>
      {#if hasMore}
        <div class="text-center">
          <button class="btn btn-sm" on:click={() => load(true)}>Load more</button>
        </div>
      {/if}
      {#each convo as m, index (m.id)}
        {#if index === 0 || !sameDate(m.created_at, convo[index-1].created_at)}
          <div class="flex justify-center">
            <span class="text-xs bg-base-200 px-2 py-1 rounded-md">{formatDate(m.created_at)}</span>
          </div>
        {/if}
        <div class={`flex ${m.sender_id === $auth?.id ? 'justify-end' : 'justify-start'}`}>
          <div class="flex gap-2 max-w-[80%]">
            {#if m.sender_id !== $auth?.id}
              <div class="avatar self-end">
                <div class="w-8 h-8 rounded-full overflow-hidden">
                  {#if contactAvatar}
                    <img src={contactAvatar} alt="Contact" class="w-full h-full object-cover" />
                  {:else}
                    <img src="/placeholder.svg?height=32&width=32" alt="Contact" />
                  {/if}
                </div>
              </div>
            {/if}
            <div>
              <div class={`rounded-lg p-3 whitespace-pre-wrap break-words ${m.sender_id === $auth?.id ? 'bg-primary text-primary-content' : 'bg-base-200'}`}
                on:click={() => { m.showTime = !m.showTime; convo = [...convo]; }}>
                {hyphenateLongWords(m.text)}
              </div>
              {#if m.showTime}
                <div class={`flex items-center mt-1 text-xs opacity-60 ${m.sender_id === $auth?.id ? 'justify-end' : 'justify-start'}`}>{formatTime(m.created_at)}</div>
              {/if}
            </div>
          </div>
        </div>
      {/each}
    </div>
  </div>
  <div class="p-3 border-t">
    <div class="flex items-center gap-2">
      <textarea
        class="textarea textarea-bordered flex-1 resize-none overflow-hidden"
        rows="1"
        style="min-height:0;height:auto"
        placeholder="Type a message..."
        bind:value={msg}
        bind:this={msgInput}
        on:input={adjustHeight}
      ></textarea>
      <button class="btn btn-primary" on:click={send} disabled={!msg.trim()}>Send</button>
    </div>
    {#if err}<p class="text-error mt-2">{err}</p>{/if}
  </div>
</div>

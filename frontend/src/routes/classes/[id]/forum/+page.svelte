<script lang="ts">
  import { onMount, afterUpdate } from 'svelte';
  import { page } from '$app/stores';
  import { apiJSON, apiFetch } from '$lib/api';
  import { createEventSource } from '$lib/sse';
  import { auth } from '$lib/auth';

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    load();
    connect();
  }

  let msgs: any[] = [];
  let text = '';
  let err = '';
  let chatBox: HTMLDivElement | null = null;
  let esCtrl: { close: () => void } | null = null;

  async function load() {
    try {
      msgs = await apiJSON(`/api/classes/${id}/forum`);
    } catch (e: any) {
      err = e.message;
    }
  }

  function connect() {
    esCtrl?.close();
    esCtrl = createEventSource(`/api/classes/${id}/forum/events`, es => {
      es.addEventListener('message', e => {
        try {
          const m = JSON.parse((e as MessageEvent).data);
          msgs = [...msgs, m];
        } catch {}
      });
    });
  }

  onMount(() => {
    load();
    connect();
    return () => esCtrl?.close();
  });

  async function send() {
    const t = text.trim();
    if (!t) return;
    try {
      const res = await apiFetch(`/api/classes/${id}/forum`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: t })
      });
      if (res.ok) {
        text = '';
      }
    } catch (e) {
      console.error('send failed', e);
    }
  }

  function displayName(m: any) {
    return m.name ?? m.email?.split('@')[0] ?? 'Unknown';
  }

  afterUpdate(() => {
    if (chatBox) {
      chatBox.scrollTop = chatBox.scrollHeight;
    }
  });
</script>

<h1 class="text-2xl font-semibold mb-4">Class Forum</h1>
{#if err}
  <p class="text-error">{err}</p>
{/if}
<div class="flex flex-col h-[70vh] max-h-[70vh]">
  <div class="flex-1 overflow-y-auto space-y-2 p-2" bind:this={chatBox}>
    {#each msgs as m}
      <div class={`flex items-end gap-2 ${m.user_id === $auth?.id ? 'justify-end text-right' : 'justify-start'}`}>
        {#if m.user_id !== $auth?.id}
          <div class="avatar flex-shrink-0">
            <div class="w-8 h-8 rounded-full overflow-hidden">
              <img src={m.avatar ?? '/avatars/a1.svg'} alt="" class="w-full h-full object-cover" />
            </div>
          </div>
        {/if}
        <div>
          <div class="text-xs opacity-70">{m.user_id === $auth?.id ? 'You' : displayName(m)}</div>
          <div class="px-3 py-2 rounded-lg bg-base-200 break-words max-w-xs">{m.text}</div>
        </div>
        {#if m.user_id === $auth?.id}
          <div class="avatar flex-shrink-0">
            <div class="w-8 h-8 rounded-full overflow-hidden">
              <img src={$auth?.avatar ?? '/avatars/a1.svg'} alt="" class="w-full h-full object-cover" />
            </div>
          </div>
        {/if}
      </div>
    {/each}
  </div>
  <form class="mt-2 flex gap-2" on:submit|preventDefault={send}>
    <input class="input input-bordered flex-1" bind:value={text} placeholder="Type a message" />
    <button type="submit" class="btn btn-primary">Send</button>
  </form>
</div>


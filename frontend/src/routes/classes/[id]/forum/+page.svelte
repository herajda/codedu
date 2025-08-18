<script lang="ts">
  import { onMount, afterUpdate } from 'svelte';
  import { page } from '$app/stores';
  import { apiJSON, apiFetch } from '$lib/api';
  import { createEventSource } from '$lib/sse';
  import { auth } from '$lib/auth';
  import { Paperclip, ImagePlus, Smile, Send, X } from 'lucide-svelte';
  import { compressImage } from '$lib/utils/compressImage';

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    load();
    connect();
  }

  let msgs: any[] = [];
  let text = '';
  let imageData: string | null = null;
  let fileData: string | null = null;
  let fileName: string | null = null;
  let err = '';
  let chatBox: HTMLDivElement | null = null;
  let msgInput: HTMLTextAreaElement | null = null;
  let photoInput: HTMLInputElement | null = null;
  let fileInput: HTMLInputElement | null = null;
  let showAttachmentMenu = false;
  let showEmojiPicker = false;
  let esCtrl: { close: () => void } | null = null;

  // Common emojis for the picker
  const commonEmojis = [
    '😀','😃','😄','😁','😆','😅','😂','🤣','😊','😇',
    '🙂','🙃','😉','😌','😍','🥰','😘','😗','😙','😚',
    '😋','😛','😝','😜','🤪','🤨','🧐','🤓','😎','🤩',
    '🥳','😏','😒','😞','😔','😟','😕','🙁','☹️','😣',
    '😖','😫','😩','🥺','😢','😭','😤','😠','😡','🤬',
    '🤯','😳','🥵','🥶','😱','😨','😰','😥','😓','🤗',
    '🤔','🤭','🤫','🤥','😶','😐','😑','😯','😦','😧',
    '😮','😲','🥱','😴','🤤','😪','😵','🤐','🥴','🤢',
    '🤮','🤧','😷','🤒','🤕','🤑','🤠','💩','🤡','👹',
    '👺','👻','👽','👾','🤖','😺','😸','😹','😻','😼'
  ];

  function insertEmoji(emoji: string) {
    const cursor = msgInput?.selectionStart || text.length;
    text = text.slice(0, cursor) + emoji + text.slice(cursor);
    setTimeout(() => {
      msgInput?.focus();
      msgInput?.setSelectionRange(cursor + emoji.length, cursor + emoji.length);
    }, 0);
  }

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

  afterUpdate(() => {
    if (chatBox) chatBox.scrollTop = chatBox.scrollHeight;
  });

  function displayName(m: any) {
    return m.name ?? m.email?.split('@')[0] ?? 'Unknown';
  }

  function choosePhoto() { photoInput?.click(); }
  function chooseFile() { fileInput?.click(); }

  async function photoChanged(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;
    const compressed = await compressImage(f, 1280, 0.8);
    const r = new FileReader();
    r.onload = () => { imageData = r.result as string; };
    r.readAsDataURL(compressed);
  }

  async function fileChanged(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;
    const r = new FileReader();
    r.onload = () => { fileData = r.result as string; fileName = f.name; };
    r.readAsDataURL(f);
  }

  async function send() {
    const t = text.trim();
    if (!t && !imageData && !fileData) return;
    try {
      const res = await apiFetch(`/api/classes/${id}/forum`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: t, image: imageData, file: fileData, file_name: fileName })
      });
      if (res.ok) {
        text = '';
        imageData = null;
        fileData = null;
        fileName = null;
      }
    } catch (e) {
      console.error('send failed', e);
    }
  }
</script>

<h1 class="text-2xl font-semibold mb-4">Class Forum</h1>
{#if err}
  <p class="text-error">{err}</p>
{/if}
<div class="flex flex-col h-[70vh] max-h-[70vh]">
  <div class="flex-1 overflow-y-auto space-y-4 p-4" bind:this={chatBox}>
    {#each msgs as m}
      <div class={`flex gap-3 ${m.user_id === $auth?.id ? 'flex-row-reverse text-right' : ''}`}>
        <div class="avatar flex-shrink-0">
          <div class="w-8 h-8 rounded-full overflow-hidden shrink-0">
            <img src={m.user_id === $auth?.id ? $auth?.avatar ?? '/avatars/a1.svg' : m.avatar ?? '/avatars/a1.svg'} alt="" class="w-full h-full object-cover" />
          </div>
        </div>
        <div class="max-w-xs space-y-1">
          <div class="text-xs opacity-70">{m.user_id === $auth?.id ? 'You' : displayName(m)}</div>
          {#if m.image}
            <img src={m.image} alt="attachment" class="rounded-lg max-w-full" />
          {/if}
          {#if m.file_name && m.file}
            <a class="block p-2 rounded-lg bg-base-200" href={m.file} download={m.file_name}>{m.file_name}</a>
          {/if}
          {#if m.text}
            <div class="px-3 py-2 rounded-lg bg-base-200 break-words">{m.text}</div>
          {/if}
        </div>
      </div>
    {/each}
  </div>

  <div class="mt-2">
    {#if imageData}
      <div class="relative mb-2">
        <img src={imageData} alt="preview" class="max-h-32 rounded" />
        <button class="btn btn-circle btn-sm btn-ghost absolute top-1 right-1" on:click={() => imageData = null}>
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}
    {#if fileName}
      <div class="relative mb-2 p-2 bg-base-200 rounded">
        <span>{fileName}</span>
        <button class="btn btn-circle btn-sm btn-ghost absolute top-1 right-1" on:click={() => { fileName=null; fileData=null; }}>
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}

    <div class="flex items-end gap-2">
      <input type="file" accept="image/*" class="hidden" bind:this={photoInput} on:change={photoChanged} />
      <input type="file" class="hidden" bind:this={fileInput} on:change={fileChanged} />

      <div class="relative">
        <button type="button" class="btn btn-circle btn-ghost" on:click={() => showAttachmentMenu = !showAttachmentMenu}>
          <Paperclip class="w-4 h-4" />
        </button>
        {#if showAttachmentMenu}
          <div class="absolute bottom-full left-0 mb-2 bg-base-100 border border-base-300 rounded shadow p-2">
            <button type="button" class="btn btn-ghost btn-sm gap-2 w-full justify-start" on:click={choosePhoto}>
              <ImagePlus class="w-4 h-4" /> Photo
            </button>
            <button type="button" class="btn btn-ghost btn-sm gap-2 w-full justify-start" on:click={chooseFile}>
              <Paperclip class="w-4 h-4" /> File
            </button>
          </div>
        {/if}
      </div>

      <div class="relative">
        <button type="button" class="btn btn-circle btn-ghost" on:click={() => showEmojiPicker = !showEmojiPicker}>
          <Smile class="w-4 h-4" />
        </button>
        {#if showEmojiPicker}
          <div class="absolute bottom-full left-0 mb-2 bg-base-100 border border-base-300 rounded shadow p-2 w-64 max-h-48 overflow-y-auto">
            <div class="grid grid-cols-8 gap-1">
              {#each commonEmojis as emoji}
                <button type="button" class="w-8 h-8 text-lg hover:bg-base-200 rounded" on:click={() => insertEmoji(emoji)}>{emoji}</button>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <textarea
        class="textarea textarea-bordered flex-1"
        rows="1"
        bind:value={text}
        bind:this={msgInput}
        placeholder="Type a message..."
        on:keydown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } }}
      ></textarea>

      <button type="button" class="btn btn-circle btn-primary" on:click={send} disabled={!text.trim() && !imageData && !fileData}>
        <Send class="w-4 h-4" />
      </button>
    </div>
  </div>
</div>

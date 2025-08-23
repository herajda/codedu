<script lang="ts">
  import { onMount, afterUpdate } from 'svelte';
  import { page } from '$app/stores';
  import { apiJSON, apiFetch } from '$lib/api';
  import { createEventSource } from '$lib/sse';
  import { auth } from '$lib/auth';
  import { Paperclip, ImagePlus, Smile, Send, X, ChevronLeft, ChevronRight } from 'lucide-svelte';
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

  let lightboxOpen = false;
  let modalImage: string | null = null;
  let imageUrls: string[] = [];
  let currentImageIndex = -1;

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

  const quickReactions = ['👍','❤️','😂','😮','😢','👏'];

  async function toggleForumReaction(mid: number, emoji: string) {
    try {
      const res = await apiJSON(`/api/classes/${id}/forum/messages/${mid}/reactions`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ emoji })
      });
      const m = msgs.find(x => x.id === mid);
      if (m) m.reactions = res.reactions;
      msgs = [...msgs];
    } catch {}
  }

  function insertEmoji(emoji: string) {
    const cursor = msgInput?.selectionStart || text.length;
    text = text.slice(0, cursor) + emoji + text.slice(cursor);
    setTimeout(() => {
      msgInput?.focus();
      msgInput?.setSelectionRange(cursor + emoji.length, cursor + emoji.length);
    }, 0);
    showEmojiPicker = false;
  }

  function handleClickOutside(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.emoji-picker') && !target.closest('.attachment-menu')) {
      showEmojiPicker = false;
      showAttachmentMenu = false;
    }
  }

  function adjustHeight() {
    if (msgInput) {
      msgInput.style.height = 'auto';
      msgInput.style.height = Math.min(msgInput.scrollHeight, 120) + 'px';
    }
  }

  $: text, adjustHeight();

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (text.trim() || imageData || fileData) send();
    }
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
      es.addEventListener('reaction', e => {
        try {
          const d = JSON.parse((e as MessageEvent).data);
          const m = msgs.find(x => x.id === d.message_id);
          if (m) { m.reactions = d.reactions; msgs = [...msgs]; }
        } catch {}
      });
    });
  }

  onMount(() => {
    load();
    connect();
    adjustHeight();
    document.addEventListener('click', handleClickOutside);
    return () => {
      esCtrl?.close();
      document.removeEventListener('click', handleClickOutside);
    };
  });

  afterUpdate(() => {
    if (chatBox) chatBox.scrollTop = chatBox.scrollHeight;
  });

  $: imageUrls = msgs.filter(m => !!m.image).map(m => m.image as string);

  function openImage(src: string) {
    modalImage = src;
    currentImageIndex = imageUrls.indexOf(src);
    lightboxOpen = true;
  }

  function closeLightbox() {
    lightboxOpen = false;
    modalImage = null;
    currentImageIndex = -1;
  }

  function showPrevImage() {
    if (!imageUrls.length) return;
    currentImageIndex = (currentImageIndex - 1 + imageUrls.length) % imageUrls.length;
    modalImage = imageUrls[currentImageIndex];
  }

  function showNextImage() {
    if (!imageUrls.length) return;
    currentImageIndex = (currentImageIndex + 1) % imageUrls.length;
    modalImage = imageUrls[currentImageIndex];
  }

  function handleLightboxKeydown(e: KeyboardEvent) {
    if (!lightboxOpen) return;
    if (e.key === 'Escape') closeLightbox();
    if (e.key === 'ArrowLeft') showPrevImage();
    if (e.key === 'ArrowRight') showNextImage();
  }

  function formatTime(d: string | number | Date) {
    return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  function hyphenateLongWords(text: string, max = 20) {
    return text.replace(new RegExp(`\\S{${max},}`, 'g'), word => {
      const parts: string[] = [];
      for (let i = 0; i < word.length; i += max) parts.push(word.slice(i, i + max));
      return parts.join('\u00AD');
    });
  }

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
        adjustHeight();
      }
    } catch (e) {
      console.error('send failed', e);
    }
  }
</script>

{#if err}
  <p class="text-error">{err}</p>
{/if}

<div class="flex flex-col h-[70vh] max-h-[70vh]">
  <div class="flex-1 overflow-y-auto p-4 space-y-6" bind:this={chatBox}>
    {#each msgs as m, i (m.id)}
      <div class={`flex ${m.user_id === $auth?.id ? 'justify-end' : 'justify-start'}`}>
        <div class="flex gap-3 max-w-[85%] sm:max-w-[75%] items-end">
          {#if m.user_id !== $auth?.id}
            <div class="avatar flex-shrink-0">
              <div class="w-8 h-8 rounded-full overflow-hidden ring-1 ring-base-300/50 shrink-0">
                <img src={m.avatar ?? '/avatars/a1.svg'} alt="" class="w-full h-full object-cover" />
              </div>
            </div>
          {/if}

          <div class="relative flex flex-col">
            <div class="text-xs opacity-70 mb-1">
              {m.user_id === $auth?.id ? 'You' : displayName(m)}
            </div>

            {#if m.image}
              <div class="mb-2">
                <button type="button" class="block p-0 m-0 bg-transparent border-0 focus:outline-none focus:ring-2 focus:ring-primary/50 rounded-2xl" on:click={() => openImage(m.image)} aria-label="Open attachment">
                  <img src={m.image} alt="attachment" class="max-w-[70vw] sm:max-w-xs w-full rounded-2xl shadow-lg" />
                </button>
              </div>
            {/if}

            {#if m.file_name && m.file}
              <a class="block p-2 mb-2 rounded-2xl bg-base-200/80 backdrop-blur-sm border border-base-300/30" href={m.file} download={m.file_name}>{m.file_name}</a>
            {/if}

            {#if m.text}
              <div
                class={`message-bubble relative rounded-2xl px-4 py-3 whitespace-pre-wrap break-words shadow-sm transition-all duration-200 ${
                  m.user_id === $auth?.id
                    ? 'bg-gradient-to-br from-primary to-primary/80 text-primary-content rounded-br-md'
                    : 'bg-base-200/80 backdrop-blur-sm border border-base-300/30 rounded-bl-md'
                }`}
                on:click={() => { m.showTime = !m.showTime; msgs = [...msgs]; }}
                role="button"
                tabindex="0"
                on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); m.showTime = !m.showTime; msgs = [...msgs]; } }}
              >
                {hyphenateLongWords(m.text)}

                {#if m.reactions && m.reactions.length}
                  <div class="mt-2 flex flex-wrap gap-1">
                    {#each m.reactions as r}
                      <button class={`px-2 py-0.5 rounded-full text-sm border ${r.mine ? 'bg-primary/20 border-primary/30' : 'bg-base-100/60 border-base-300/50'} hover:scale-105 transition`} on:click={() => toggleForumReaction(m.id, r.emoji)}>{r.emoji} {r.count}</button>
                    {/each}
                  </div>
                {/if}

                <div class="absolute -top-7 left-2 opacity-0 group-hover:opacity-100 transition-opacity flex gap-1">
                  {#each quickReactions as e}
                    <button class="btn btn-ghost btn-xs" on:click|stopPropagation={() => toggleForumReaction(m.id, e)}>{e}</button>
                  {/each}
                </div>

                {#if m.showTime}
                  <div class={`absolute -bottom-5 ${m.user_id === $auth?.id ? 'right-0' : 'left-0'} text-xs opacity-60`}>
                    {formatTime(m.created_at)}
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        </div>
      </div>
    {/each}
  </div>

  <div class="chat-input-area p-4 border-t border-base-300/30 bg-base-100/80 backdrop-blur-sm">
    {#if imageData}
      <div class="relative mb-3">
        <img src={imageData} alt="preview" class="max-h-32 rounded-lg shadow-sm" />
        <button
          class="btn btn-circle btn-sm btn-ghost absolute top-2 right-2 bg-base-100/80 backdrop-blur-sm hover:bg-base-200/80"
          on:click={() => imageData = null}
        >
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}
    {#if fileName}
      <div class="relative mb-3 p-2 bg-base-200/80 backdrop-blur-sm border border-base-300/30 rounded-lg">
        <span>{fileName}</span>
        <button
          class="btn btn-circle btn-sm btn-ghost absolute top-2 right-2 bg-base-100/80 backdrop-blur-sm hover:bg-base-200/80"
          on:click={() => { fileName = null; fileData = null; }}
        >
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}

    <div class="flex items-end gap-3">
      <input type="file" accept="image/*" class="hidden" bind:this={photoInput} on:change={photoChanged} />
      <input type="file" class="hidden" bind:this={fileInput} on:change={fileChanged} />

      <div class="relative attachment-menu">
        <button
          class="btn btn-circle btn-ghost hover:bg-base-200/80 transition-all duration-200"
          on:click={() => showAttachmentMenu = !showAttachmentMenu}
        >
          <Paperclip class="w-4 h-4" />
        </button>
        {#if showAttachmentMenu}
          <div class="absolute bottom-full left-0 mb-2 bg-base-100 rounded-lg shadow-lg border border-base-300/30 p-2 backdrop-blur-sm">
            <button class="btn btn-ghost btn-sm gap-2 w-full justify-start" on:click={choosePhoto}>
              <ImagePlus class="w-4 h-4" />
              Photo
            </button>
            <button class="btn btn-ghost btn-sm gap-2 w-full justify-start" on:click={chooseFile}>
              <Paperclip class="w-4 h-4" />
              File
            </button>
          </div>
        {/if}
      </div>

      <div class="relative emoji-picker">
        <button
          class="btn btn-circle btn-ghost hover:bg-base-200/80 transition-all duration-200"
          on:click={() => showEmojiPicker = !showEmojiPicker}
        >
          <Smile class="w-4 h-4" />
        </button>
        {#if showEmojiPicker}
          <div class="absolute bottom-full left-0 mb-2 bg-base-100 rounded-lg shadow-lg border border-base-300/30 p-3 backdrop-blur-sm w-64 max-h-48 overflow-y-auto">
            <div class="grid grid-cols-8 gap-1">
              {#each commonEmojis as emoji}
                <button
                  class="w-8 h-8 text-lg hover:bg-base-200 rounded transition-colors flex items-center justify-center"
                  on:click={() => insertEmoji(emoji)}
                >
                  {emoji}
                </button>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <div class="flex-1 relative">
        <textarea
          class="textarea textarea-bordered w-full resize-none overflow-hidden bg-base-200/50 backdrop-blur-sm border-base-300/50 focus:border-primary/50 focus:bg-base-100/80 transition-all duration-200 rounded-2xl"
          rows="1"
          style="min-height:0;height:auto"
          placeholder="Type a message..."
          bind:value={text}
          bind:this={msgInput}
          on:input={adjustHeight}
          on:keydown={handleKeydown}
        ></textarea>
      </div>

      <button
        class="btn btn-circle btn-primary shadow-lg hover:shadow-xl transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
        on:click={send}
        disabled={!text.trim() && !imageData && !fileData}
        aria-label="Send message"
      >
        <Send class="w-4 h-4" />
      </button>
    </div>

    {#if err}
      <div class="mt-2 p-2 bg-error/10 border border-error/20 rounded-lg">
        <p class="text-error text-sm">{err}</p>
      </div>
    {/if}
  </div>
</div>

<!-- Image Lightbox Overlay -->
{#if lightboxOpen && modalImage}
  <div class="fixed top-0 bottom-0 right-0 left-0 z-[100] bg-black/80 backdrop-blur-sm flex items-center justify-center" on:click|self={closeLightbox} in:fade={{ duration: 150 }} out:fade={{ duration: 150 }} role="dialog" aria-modal="true" aria-label="Image viewer" tabindex="-1" on:keydown={handleLightboxKeydown}>
    <!-- Controls -->
    <div class="absolute top-0 left-0 right-0 p-4 flex items-center justify-end gap-2">
      <a class="btn btn-sm md:btn-md no-animation bg-white/20 hover:bg-white/30 text-white border-0" href={modalImage} download on:click|stopPropagation aria-label="Download image">Download</a>
      <button class="btn btn-circle no-animation bg-white/20 hover:bg-white/30 text-white border-0" on:click|stopPropagation={closeLightbox} aria-label="Close">
        <X class="w-5 h-5" />
      </button>
    </div>

    <!-- Image -->
    <button type="button" class="bg-transparent p-0 m-0 border-0 focus:outline-none" on:click|stopPropagation aria-label="Image">
      <img src={modalImage} alt="" class="max-h-[90vh] max-w-[90vw] object-contain rounded-xl shadow-2xl" transition:scale={{ duration: 200, start: 0.98 }} />
    </button>

    <!-- Nav Arrows -->
    {#if imageUrls.length > 1}
      <button class="absolute left-4 md:left-8 top-1/2 -translate-y-1/2 rounded-full p-2 md:p-3 bg-white/15 hover:bg-white/25 text-white shadow-sm border border-transparent focus:outline-none focus:ring-2 focus:ring-white/50" on:click|stopPropagation={showPrevImage} aria-label="Previous image">
        <ChevronLeft class="w-6 h-6" />
      </button>
      <button class="absolute right-4 md:right-8 top-1/2 -translate-y-1/2 rounded-full p-2 md:p-3 bg-white/15 hover:bg-white/25 text-white shadow-sm border border-transparent focus:outline-none focus:ring-2 focus:ring-white/50" on:click|stopPropagation={showNextImage} aria-label="Next image">
        <ChevronRight class="w-6 h-6" />
      </button>
    {/if}
  </div>
{/if}

<style>
  .message-bubble {
    position: relative;
    transition: all 0.2s ease;
  }

  .message-bubble:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  }
</style>

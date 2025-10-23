<script lang="ts">
  import { onMount, afterUpdate } from 'svelte';
  import { page } from '$app/stores';
  import { apiJSON, apiFetch } from '$lib/api';
  import { createEventSource } from '$lib/sse';
  import { auth } from '$lib/auth';
  import { Paperclip, ImagePlus, Smile, Send, X, ChevronLeft, ChevronRight, MessageSquare, Trash2 } from 'lucide-svelte';
  import { compressImage } from '$lib/utils/compressImage';
  import { fade, scale } from 'svelte/transition';
  import { sidebarCollapsed } from '$lib/sidebar';
  import { t, translator } from '$lib/i18n';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';

  let translate;
  $: translate = $translator;

  let id = $page.params.id;
  $: if ($page.params.id !== id) {
    id = $page.params.id;
    // reset paging state on class change
    msgs = [];
    offset = 0;
    hasMore = true;
    load();
    connect();
  }

  let msgs: any[] = [];
  let text = '';
  let cls: any = null;
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
  let deleting: Record<string, boolean> = {};
  let confirmModal: InstanceType<typeof ConfirmModal>;

  // Pagination & scroll preservation (mirrors chat behavior)
  const pageSize = 20;
  let offset = 0;
  let hasMore = true;
  let prevLen = 0;
  let preserveScroll = false;
  let prevHeight = 0;
  let prevTop = 0;

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

  async function load(more = false) {
    try {
      if (more && chatBox) {
        preserveScroll = true;
        prevHeight = chatBox.scrollHeight;
        prevTop = chatBox.scrollTop;
      }
      const list: any[] = await apiJSON(`/api/classes/${id}/forum?limit=${pageSize}&offset=${offset}`);
      // Backend returns newest-first; reverse to show oldest->newest within chunk
      list.reverse();
      for (const m of list) {
        (m as any).showTime = false;
        (m as any)._fromHistory = true;
      }
      if (more) msgs = [...list, ...msgs];
      else msgs = list;
      offset += list.length;
      hasMore = list.length >= pageSize;
    } catch (e: any) {
      err = e.message;
    }
    try {
      cls = await apiJSON(`/api/classes/${id}`);
    } catch {} 
  }

  function connect() {
    esCtrl?.close();
    esCtrl = createEventSource(`/api/classes/${id}/forum/events`, es => {
      es.addEventListener('message', e => {
        try {
          const m = JSON.parse((e as MessageEvent).data);
          if (!m || !m.id) return;
          if (msgs.some(existing => existing.id === m.id)) return;
          (m as any).showTime = false;
          (m as any)._fromHistory = false;
          msgs = [...msgs, m];
        } catch {}
      });
      es.addEventListener('deleted', e => {
        try {
          const payload = JSON.parse((e as MessageEvent).data);
          if (!payload || !payload.id) return;
          removeLocalMessage(payload.id);
        } catch {}
      });
    });
  }

  onMount(() => {
    offset = 0; hasMore = true; msgs = [];
    load();
    connect();
    adjustHeight();
    // Focus input on open to mirror chat UX
    setTimeout(() => msgInput?.focus(), 0);
    document.addEventListener('click', handleClickOutside);
    document.addEventListener('keydown', handleLightboxKeydown);
    return () => {
      esCtrl?.close();
      document.removeEventListener('click', handleClickOutside);
      document.removeEventListener('keydown', handleLightboxKeydown);
    };
  });

  afterUpdate(() => {
    if (!chatBox) return;
    if (preserveScroll) {
      chatBox.scrollTop = chatBox.scrollHeight - prevHeight + prevTop;
      preserveScroll = false;
      prevLen = msgs.length;
    } else if (msgs.length !== prevLen) {
      chatBox.scrollTop = chatBox.scrollHeight;
      prevLen = msgs.length;
    }
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

  // Keyboard navigation is handled globally; handler checks lightboxOpen

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

  function isEmojiOnly(text: string): boolean {
    const trimmed = text.trim();
    if (!trimmed) return false;
    const emojiOnly = /^(?:\p{Extended_Pictographic}(?:\uFE0F|\u200D\p{Extended_Pictographic})*)+$/u;
    return emojiOnly.test(trimmed);
  }

  function displayName(m: any) {
    return m.name ?? m.email?.split('@')[0] ?? t('frontend/src/routes/classes/[id]/forum/+page.svelte::unknown');
  }

  // Deterministic placeholder avatar (avoid Math.random quirks on re-render)
  function placeholderAvatar(seed: string): string {
    let h = 0;
    for (let i = 0; i < seed.length; i++) h = ((h << 5) - h + seed.charCodeAt(i)) >>> 0;
    const n = (h % 50) + 1;
    return `/avatars/a${n}.svg`;
  }
  function avatarFor(m: any): string {
    return m.avatar ?? placeholderAvatar(String(m.user_id ?? m.email ?? 'x'));
  }

  function canDelete(m: any): boolean {
    if (!$auth) return false;
    if (m.user_id === $auth.id) return true;
    if ($auth.role === 'admin') return true;
    if ($auth.role === 'teacher' && cls?.class?.teacher_id === $auth.id) return true;
    return false;
  }

  function removeLocalMessage(id: string) {
    const existing = msgs.find(msg => msg.id === id);
    if (!existing) return;
    const wasHistory = !!existing._fromHistory;
    if (modalImage && existing.image === modalImage) {
      closeLightbox();
    }
    msgs = msgs.filter(msg => msg.id !== id);
    if (wasHistory && offset > 0) {
      offset = Math.max(0, offset - 1);
    }
  }

  async function deleteMessage(m: any) {
    if (!canDelete(m) || deleting[m.id]) return;
    const confirmed = await confirmModal?.open({
      title: translate('frontend/src/routes/classes/[id]/forum/+page.svelte::delete_message_confirm'),
      confirmLabel: translate('frontend/src/routes/classes/[id]/forum/+page.svelte::delete_message_label'),
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    });
    if (!confirmed) return;
    deleting = { ...deleting, [m.id]: true };
    try {
      const res = await apiFetch(`/api/classes/${id}/forum/${m.id}`, { method: 'DELETE' });
      if (!res.ok) {
        const message = await res.text();
        throw new Error(message || t('frontend/src/routes/classes/[id]/forum/+page.svelte::delete_message_error'));
      }
      removeLocalMessage(m.id);
    } catch (e: any) {
      err = e?.message ?? t('frontend/src/routes/classes/[id]/forum/+page.svelte::delete_message_error');
    } finally {
      const { [m.id]: _removed, ...rest } = deleting;
      deleting = rest;
    }
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
        // Ensure the just-sent message appears immediately like chat
        offset = 0; hasMore = true; await load();
        // Jump to the newest message and keep typing
        setTimeout(() => {
          if (chatBox) chatBox.scrollTop = chatBox.scrollHeight;
          msgInput?.focus();
        }, 0);
      }
    }
    catch (e) {
      console.error('send failed', e);
    }
  }
</script>

<div class={`chat-window fixed top-16 bottom-0 right-0 left-0 ${$sidebarCollapsed ? 'sm:left-0' : 'sm:left-60'} z-40 flex flex-col bg-gradient-to-br from-base-100/95 to-base-200/95 backdrop-blur-xl border-l border-base-300/30`}>
  <div class="chat-header relative z-30 mx-2 sm:mx-4 mt-2 sm:mt-3 flex items-center justify-between p-2 sm:p-4 rounded-xl bg-base-100/80 backdrop-blur supports-[backdrop-filter]:bg-base-100/85 border border-base-300/30 shadow-lg">
    <div class="flex items-center gap-3 min-w-0">
      <div class="p-2 bg-primary/10 rounded-lg">
        <MessageSquare class="w-5 h-5 text-primary" />
      </div>
      <div class="min-w-0">
        <h2 class="font-semibold text-lg leading-tight">{translate('frontend/src/routes/classes/[id]/forum/+page.svelte::forum_title')}</h2>
        <div class="text-sm text-base-content/60 truncate" title={(cls?.class?.name ?? cls?.name) ?? ''}>{cls?.class?.name ?? cls?.name ?? translate('frontend/src/routes/classes/[id]/forum/+page.svelte::class_discussion_fallback')}</div>
      </div>
    </div>
  </div>

  <div class="flex-1 overflow-hidden relative z-0">
    <div class="h-full overflow-y-auto p-6 space-y-6" bind:this={chatBox}>
      {#if hasMore}
        <div class="text-center">
          <button class="btn btn-outline btn-sm glass" on:click={() => load(true)}>
            {translate('frontend/src/routes/classes/[id]/forum/+page.svelte::load_more_messages')}
          </button>
        </div>
      {/if}
      {#each msgs as m, i (m.id)}
        <div class={`flex ${m.user_id === $auth?.id ? 'justify-end' : 'justify-start'}`}>
          <div class="flex gap-3 max-w-[85%] sm:max-w-[75%] items-end">
            {#if m.user_id !== $auth?.id}
              <div class="avatar flex-shrink-0">
                <div class="w-8 h-8 rounded-full overflow-hidden ring-1 ring-base-300/50 shrink-0">
                  <img src={avatarFor(m)} alt="" class="w-full h-full object-cover" />
                </div>
              </div>
            {/if}

            <div class="relative flex flex-col">
              <div class={`flex items-center gap-2 mb-1 ${m.user_id === $auth?.id ? 'justify-end text-right' : ''}`}>
                <div class={`text-xs opacity-70 ${m.user_id === $auth?.id ? 'text-right' : ''}`}>
                  {m.user_id === $auth?.id ? translate('frontend/src/routes/classes/[id]/forum/+page.svelte::you') : displayName(m)}
                </div>
                {#if canDelete(m)}
                  <button
                    type="button"
                    class="btn btn-ghost btn-xs px-2 h-6 min-h-6 text-base-content/60 hover:text-error focus-visible:ring-2 focus-visible:ring-error/40"
                    disabled={!!deleting[m.id]}
                    title={t('frontend/src/routes/classes/[id]/forum/+page.svelte::delete_message_label')}
                    aria-label={t('frontend/src/routes/classes/[id]/forum/+page.svelte::delete_message_label')}
                    on:click|stopPropagation={() => deleteMessage(m)}
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                {/if}
              </div>

              {#if m.image}
                <div class="mb-2">
                  <button type="button" class="block p-0 m-0 bg-transparent border-0 focus:outline-none focus:ring-2 focus:ring-primary/50 rounded-2xl" on:click={() => openImage(m.image)} aria-label={t('frontend/src/routes/classes/[id]/forum/+page.svelte::open_attachment_aria')}>
                    <img src={m.image} alt={t('frontend/src/routes/classes/[id]/forum/+page.svelte::attachment_alt')} class="max-w-[70vw] sm:max-w-xs w-full max-h-96 object-contain rounded-2xl shadow-lg" />
                  </button>
                </div>
              {/if}

              {#if m.file_name && m.file}
                <a
                  href={m.file}
                  download={m.file_name}
                  class="flex items-center gap-3 p-3 mb-2 bg-base-200/50 rounded-2xl border border-base-300/30 hover:bg-base-200/70 transition-colors"
                >
                  <div class="w-12 h-12 bg-primary/10 rounded-xl flex items-center justify-center flex-shrink-0">
                    <Paperclip class="w-6 h-6 text-primary" />
                  </div>
                  <div class="flex-1 min-w-0">
                    <p class="text-sm font-medium truncate">{m.file_name || translate('frontend/src/routes/classes/[id]/forum/+page.svelte::file_name_fallback')}</p>
                    <p class="text-xs text-base-content/60">{translate('frontend/src/routes/classes/[id]/forum/+page.svelte::click_to_download')}</p>
                  </div>
                  <div class="flex-shrink-0">
                    <svg class="w-5 h-5 text-base-content/60" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                    </svg>
                  </div>
                </a>
              {/if}

              {#if m.text}
                {#if isEmojiOnly(m.text)}
                  <div
                    class="select-none text-5xl leading-none"
                    on:click={() => { m.showTime = !m.showTime; msgs = [...msgs]; }}
                    role="button"
                    tabindex="0"
                    on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); m.showTime = !m.showTime; msgs = [...msgs]; } }}
                  >
                    {m.text}
                  </div>
                  
                  <!-- Time display below emoji -->
                  {#if m.showTime}
                    <div class={`text-xs opacity-60 mt-1 ${m.user_id === $auth?.id ? 'text-right' : 'text-left'}`}>
                      <span class="text-base-content/60">{formatTime(m.created_at)}</span>
                    </div>
                  {/if}
                {:else}
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
                  </div>
                  
                  <!-- Time display below message -->
                  {#if m.showTime}
                    <div class={`text-xs opacity-60 mt-1 ${m.user_id === $auth?.id ? 'text-right' : 'text-left'}`}>
                      <span class="text-base-content/60">{formatTime(m.created_at)}</span>
                    </div>
                  {/if}
                {/if}
              {/if}
            </div>
          </div>
        </div>
      {/each}
    </div>
  </div>

  <div class="chat-input-area mx-2 sm:mx-4 mb-2 sm:mb-3 p-4 rounded-xl bg-base-100/80 backdrop-blur supports-[backdrop-filter]:bg-base-100/85 border border-base-300/30 shadow-md">
    {#if imageData}
      <div class="relative mb-3">
        <img src={imageData} alt={t('frontend/src/routes/classes/[id]/forum/+page.svelte::image_preview_alt')} class="max-h-32 rounded-lg shadow-sm" />
        <button
          class="btn btn-circle btn-sm btn-ghost absolute top-2 right-2 bg-base-100/80 backdrop-blur-sm hover:bg-base-200/80"
          on:click={() => imageData = null}
        >
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}
    {#if fileData}
      <div class="relative mb-3">
        <div class="flex items-center gap-3 p-3 bg-base-200/50 rounded-lg border border-base-300/50">
          <div class="w-10 h-10 bg-primary/10 rounded-lg flex items-center justify-center">
            <Paperclip class="w-5 h-5 text-primary" />
          </div>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium truncate">{fileName}</p>
            <p class="text-xs text-base-content/60">{(fileData.length * 0.75 / 1024).toFixed(1)} KB</p>
          </div>
          <button
            class="btn btn-circle btn-sm btn-ghost hover:bg-base-200/80"
            on:click={() => { fileName = null; fileData = null; }}
          >
            <X class="w-4 h-4" />
          </button>
        </div>
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
              {translate('frontend/src/routes/classes/[id]/forum/+page.svelte::attachment_photo')}
            </button>
            <button class="btn btn-ghost btn-sm gap-2 w-full justify-start" on:click={chooseFile}>
              <Paperclip class="w-4 h-4" />
              {translate('frontend/src/routes/classes/[id]/forum/+page.svelte::attachment_file')}
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
          placeholder={t('frontend/src/routes/classes/[id]/forum/+page.svelte::type_a_message_placeholder')}
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
        aria-label={t('frontend/src/routes/classes/[id]/forum/+page.svelte::send_message_aria')}
      >
        <Send class="w-4 h-4" />
      </button>
    </div>
  </div>
</div>

<!-- Image Lightbox Overlay -->
{#if lightboxOpen && modalImage}
  <div class={`fixed top-0 bottom-0 right-0 left-0 ${$sidebarCollapsed ? 'sm:left-0' : 'sm:left-60'} z-[100] bg-black/80 backdrop-blur-sm flex items-center justify-center`} on:click|self={closeLightbox} in:fade={{ duration: 150 }} out:fade={{ duration: 150 }} role="dialog" aria-modal="true" aria-label={t('frontend/src/routes/classes/[id]/forum/+page.svelte::image_viewer_aria')}>
    <!-- Controls -->
    <div class="absolute top-0 left-0 right-0 p-4 flex items-center justify-end gap-2">
      <a class="btn btn-sm md:btn-md no-animation bg-white/20 hover:bg-white/30 text-white border-0" href={modalImage} download on:click|stopPropagation aria-label={t('frontend/src/routes/classes/[id]/forum/+page.page.svelte::download_image_aria')}>{translate('frontend/src/routes/classes/[id]/forum/+page.svelte::lightbox_download')}</a>
      <button class="btn btn-circle no-animation bg-white/20 hover:bg-white/30 text-white border-0" on:click|stopPropagation={closeLightbox} aria-label={t('frontend/src/routes/classes/[id]/forum/+page.svelte::close_aria')}>
        <X class="w-5 h-5" />
      </button>
    </div>

    <!-- Image -->
    <button type="button" class="bg-transparent p-0 m-0 border-0 focus:outline-none" on:click|stopPropagation aria-label={t('frontend/src/routes/classes/[id]/forum/+page.svelte::image_aria')}>
      <img src={modalImage} alt="" class="max-h-[90vh] max-w-[90vw] object-contain rounded-xl shadow-2xl" transition:scale={{ duration: 200, start: 0.98 }} />
    </button>

    <!-- Nav Arrows -->
    {#if imageUrls.length > 1}
      <button class="absolute left-4 md:left-8 top-1/2 -translate-y-1/2 rounded-full p-2 md:p-3 bg-white/15 hover:bg-white/25 text-white shadow-sm border border-transparent focus:outline-none focus:ring-2 focus:ring-white/50" on:click|stopPropagation={showPrevImage} aria-label={t('frontend/src/routes/classes/[id]/forum/+page.svelte::previous_image_aria')}>
        <ChevronLeft class="w-6 h-6" />
      </button>
      <button class="absolute right-4 md:right-8 top-1/2 -translate-y-1/2 rounded-full p-2 md:p-3 bg-white/15 hover:bg-white/25 text-white shadow-sm border border-transparent focus:outline-none focus:ring-2 focus:ring-white/50" on:click|stopPropagation={showNextImage} aria-label={t('frontend/src/routes/classes/[id]/forum/+page.svelte::next_image_aria')}>
        <ChevronRight class="w-6 h-6" />
      </button>
    {/if}
  </div>
{/if}

<ConfirmModal bind:this={confirmModal} />

<style>
  .message-bubble {
    position: relative;
    transition: all 0.2s ease;
  }

  .message-bubble:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  }

  .chat-window {
    background: linear-gradient(135deg, hsl(var(--b1) / 0.95) 0%, hsl(var(--b2) / 0.95) 100%);
  }

  .chat-input-area {
    background: linear-gradient(180deg, hsl(var(--b1) / 0.8) 0%, hsl(var(--b1) / 0.95) 100%);
  }

  /* Custom scrollbar for chat (match DM chat) */
  .overflow-y-auto::-webkit-scrollbar {
    width: 6px;
  }
  .overflow-y-auto::-webkit-scrollbar-track {
    background: transparent;
  }
  .overflow-y-auto::-webkit-scrollbar-thumb {
    background: hsl(var(--bc) / 0.2);
    border-radius: 3px;
  }
  .overflow-y-auto::-webkit-scrollbar-thumb:hover {
    background: hsl(var(--bc) / 0.3);
  }
</style>

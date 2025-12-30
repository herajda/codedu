<script lang="ts">
  import { onMount, onDestroy, afterUpdate } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { apiFetch, apiJSON } from '$lib/api';
  import { createEventSource } from '$lib/sse';
  import { auth } from '$lib/auth';
  import { compressImage } from '$lib/utils/compressImage';
  import { formatDate as formatDisplayDate } from '$lib/date';
  import {
    ImagePlus,
    Send,
    ChevronLeft,
    ChevronRight,
    ChevronUp,
    ChevronDown,
    Check,
    CheckCheck,
    MoreVertical,
    Search,
    Paperclip,
    X,
    Smile,
    Table,
    MessageSquare,
    Download,
    Reply,
    CornerDownRight
  } from 'lucide-svelte';
import { renderMarkdown } from '$lib/markdown';
import MarkdownEditor from '$lib/MarkdownEditor.svelte';
  import { fade, scale } from 'svelte/transition';
  import { sidebarCollapsed } from '$lib/sidebar';
  import UserProfileModal from '$lib/components/UserProfileModal.svelte';
  import { onlineUsers } from '$lib/stores/onlineUsers';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import { t, translator, type Translator } from '$lib/i18n';

  let translate: Translator;
  $: translate = $translator;

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
  let imageData: string | null = null;
  let fileInput: HTMLInputElement | null = null;
  let fileData: string | null = null;
  let fileName: string | null = null;
  let generalFileInput: HTMLInputElement | null = null;
  let modalImage: string | null = null;
  let lightboxOpen = false;
  let currentImageIndex: number = -1;
  let confirmModal: InstanceType<typeof ConfirmModal>;
  let esCtrl: { close: () => void } | null = null;
  let chatBox: HTMLDivElement | null = null;
  let msgInput: HTMLTextAreaElement | null = null;
  let showAttachmentMenu = false;
  let showEmojiPicker = false;
  let showProfile = false;
  let showSearch = false;
  let structured = false;
  let searchQuery = '';
  let searchResults: number[] = [];
  let searchPos = 0;
  let searchInput: HTMLInputElement | null = null;
  let msgEls: HTMLDivElement[] = [];

  // Reply feature state
  let replyToMessage: any = null; // The message being replied to
  let highlightedMessageId: number | null = null; // ID of message to highlight
  
  function setReplyTo(message: any) {
    replyToMessage = message;
    // Focus the input after setting reply
    setTimeout(() => msgInput?.focus(), 0);
  }
  
  function clearReply() {
    replyToMessage = null;
  }
  
  let highlightTimeout: any = null;
  function scrollToMessage(messageId: number) {
    // Find the index of the message in the conversation
    const messageIndex = convo.findIndex(m => m.id === messageId);
    if (messageIndex === -1) return;
    
    // Get the message element
    const messageEl = msgEls[messageIndex];
    if (!messageEl) return;
    
    // Scroll to the message
    messageEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
    
    // Highlight the message
    highlightedMessageId = messageId;
    
    // Clear previous timeout if any
    if (highlightTimeout) clearTimeout(highlightTimeout);
    
    // Remove highlight after 3 seconds
    highlightTimeout = setTimeout(() => {
      highlightedMessageId = null;
      highlightTimeout = null;
    }, 3000);
  }

  function registerMsgEl(node: HTMLDivElement, idx: number) {
    msgEls[idx] = node;
  }

  // Common emojis for the picker
  const commonEmojis = [
    '😀', '😃', '😄', '😁', '😆', '😅', '😂', '🤣', '😊', '😇',
    '🙂', '🙃', '😉', '😌', '😍', '🥰', '😘', '😗', '😙', '😚',
    '😋', '😛', '😝', '😜', '🤪', '🤨', '🧐', '🤓', '😎', '🤩',
    '🥳', '😏', '😒', '😞', '😔', '😟', '😕', '🙁', '☹️', '😣',
    '😖', '😫', '😩', '🥺', '😢', '😭', '😤', '😠', '😡', '🤬',
    '🤯', '😳', '🥵', '🥶', '😱', '😨', '😰', '😥', '😓', '🤗',
    '🤔', '🤭', '🤫', '🤥', '😶', '😐', '😑', '😯', '😦', '😧',
    '😮', '😲', '🥱', '😴', '🤤', '😪', '😵', '🤐', '🥴', '🤢',
    '🤮', '🤧', '😷', '🤒', '🤕', '🤑', '🤠', '💩', '🤡', '👹',
    '👺', '👻', '👽', '👾', '🤖', '😺', '😸', '😹', '😻', '😼'
  ];

  function insertEmoji(emoji: string) {
    const cursorPos = msgInput?.selectionStart || msg.length;
    const before = msg.slice(0, cursorPos);
    const after = msg.slice(cursorPos);
    msg = before + emoji + after;
    
    // Set cursor position after the inserted emoji
    setTimeout(() => {
      if (msgInput) {
        const newPos = cursorPos + emoji.length;
        msgInput.setSelectionRange(newPos, newPos);
        msgInput.focus();
      }
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

  $: msg, adjustHeight();

  function toggleSearch() {
    showSearch = !showSearch;
    if (showSearch) {
      setTimeout(() => searchInput?.focus(), 0);
    } else {
      searchQuery = '';
      searchResults = [];
    }
  }

  function updateSearch() {
    searchResults = [];
    if (!searchQuery.trim()) return;
    const q = searchQuery.toLowerCase();
    searchResults = convo.map((m, i) => m.text?.toLowerCase().includes(q) ? i : -1).filter(i => i >= 0);
    searchPos = 0;
    if (searchResults.length > 0) scrollToResult();
  }

  function scrollToResult() {
    const idx = searchResults[searchPos];
    const el = msgEls[idx];
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }

  function nextResult() {
    if (searchResults.length === 0) return;
    searchPos = (searchPos + 1) % searchResults.length;
    scrollToResult();
  }

  function prevResult() {
    if (searchResults.length === 0) return;
    searchPos = (searchPos - 1 + searchResults.length) % searchResults.length;
    scrollToResult();
  }

  $: searchQuery, updateSearch();
  $: convo, updateSearch();

  function formatTime(d: string | number | Date) {
    return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  function formatDateLabel(d: string | number | Date) {
    const date = new Date(d)
    const today = new Date()
    const yesterday = new Date()
    yesterday.setDate(today.getDate() - 1)
    if (date.toDateString() === today.toDateString()) return t('frontend/src/routes/messages/[id]/+page.svelte::today');
    if (date.toDateString() === yesterday.toDateString()) return t('frontend/src/routes/messages/[id]/+page.svelte::yesterday');
    return formatDisplayDate(date)
  }

  function hyphenateLongWords(text: string, max = 20) {
    return text.replace(new RegExp(`\S{${max},}`, 'g'), (word) => {
      const parts = []
      for (let i = 0; i < word.length; i += max) parts.push(word.slice(i, i + max))
      return parts.join('\u00AD')
    })
  }

  function isEmojiOnly(text: string): boolean {
    const trimmed = text.trim()
    if (!trimmed) return false
    // Match sequences composed purely of emoji graphemes, including ZWJ and VS16
    const emojiOnly = /^(?:\p{Extended_Pictographic}(?:\uFE0F|\u200D\p{Extended_Pictographic})*)+$/u
    return emojiOnly.test(trimmed)
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
    for (const m of list) {
      m.showTime = false;
    }
    if (more) convo = [...list, ...convo];
    else convo = list;
    offset += list.length;
    if (list.length < pageSize) hasMore = false;
    msgEls = [];
    updateSearch();
  }

  async function send() {
    err = '';
    if (!msg.trim() && !imageData && !fileData) return;
    const payload: any = { to: id, text: msg, image: imageData, file_name: fileName, file: fileData, structured };
    if (replyToMessage) {
      payload.reply_to = replyToMessage.id;
    }
    const res = await apiFetch('/api/messages', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify(payload)
    });
    if (res.ok) { msg=''; imageData=null; fileData=null; fileName=null; structured=false; replyToMessage=null; offset=0; await load(); }
    else { err = (await res.json()).error; }
  }

  onMount(() => {
    load();
    (async () => {
      try {
        const info = await apiJSON(`/api/users/${id}`);
        contactAvatar = info.avatar ?? null;
        if (!name) name = info.name ?? info.email ?? id;
      } catch {}
    })();
    esCtrl = createEventSource(
      '/api/messages/events',
      (src) => {
        src.addEventListener('message', async (ev) => {
          const d = JSON.parse((ev as MessageEvent).data);
          if (d.sender_id === id || d.recipient_id === id) {
            d.showTime = false;
            convo = [...convo, d];
            if (d.sender_id === id) {
              await apiFetch(`/api/messages/${id}/read`, { method: 'PUT' });
              d.is_read = true;
            }
          }
        });
        src.addEventListener('read', (ev) => {
          const d = JSON.parse((ev as MessageEvent).data);
          if (d.reader_id === id) {
            for (const m of convo) {
              if (m.sender_id === $auth?.id && m.recipient_id === id) {
                m.is_read = true;
              }
            }
            convo = [...convo];
          }
        });
      },
      {
        onError: (m) => { err = m; },
        onOpen: () => { err = ''; }
      }
    );
    adjustHeight();
    
    // Add click outside handler and global keydown listener
    document.addEventListener('click', handleClickOutside);
    document.addEventListener('keydown', handleLightboxKeydown);
    return () => {
      esCtrl?.close();
      document.removeEventListener('click', handleClickOutside);
      document.removeEventListener('keydown', handleLightboxKeydown);
    };
  });
  function back() { goto('/messages'); }

  function chooseFile() { fileInput?.click(); }
  function chooseGeneralFile() { generalFileInput?.click(); }

  async function fileChanged(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;
    const compressed = await compressImage(f, 1280, 0.8);
    const r = new FileReader();
    r.onload = () => { imageData = r.result as string; };
    r.readAsDataURL(compressed);
  }

  function generalFileChanged(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;

    // Check file size (20MB limit)
    if (f.size > 20 * 1024 * 1024) {
      alert(t('frontend/src/routes/messages/[id]/+page.svelte::file_size_must_be_less_than_20mb'));
      return;
    }

    fileName = f.name;
    const r = new FileReader();
    r.onload = () => { fileData = r.result as string; };
    r.readAsDataURL(f);
  }

  function openProfile() {
    showProfile = true;
  }

  async function blockThisUser() {
    const confirmed = await confirmModal.open({
      title: t('frontend/src/routes/messages/[id]/+page.svelte::block_user_title'),
      body: t('frontend/src/routes/messages/[id]/+page.svelte::block_user_body'),
      confirmLabel: t('frontend/src/routes/messages/[id]/+page.svelte::block_user_confirm_label'),
      confirmClass: 'btn btn-error',
      cancelClass: 'btn'
    });
    if (!confirmed) return;
    try {
      await apiFetch(`/api/users/${id}/block`, { method: 'POST' });
      goto('/messages');
    } catch (e) {
      console.error('Failed to block user', e);
    }
  }

  // Derived list of image URLs in the conversation (in message order)
  $: imageUrls = convo.filter((m) => !!m.image).map((m) => m.image as string);

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

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (msg.trim() || imageData || fileData) {
        send();
      }
    }
  }
</script>

<svelte:head>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@100..900&display=swap" rel="stylesheet">
  <title>{name ? `${name} | CodEdu` : 'Messages | CodEdu'}</title>
</svelte:head>

<div class="messages-page flex flex-col h-[calc(100vh-6rem)] sm:h-[calc(100vh-7.1rem)] overflow-hidden">
  <!-- Premium Profile Header -->
  <section class="relative bg-base-100 rounded-3xl border border-base-300 shadow-md mt-0 sm:mt-0 mb-2 p-4 sm:p-6 shrink-0 z-20">
    <div class="absolute inset-0 overflow-hidden rounded-3xl pointer-events-none">
      <div class="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent"></div>
      <div class="absolute -top-24 -right-24 w-64 h-64 bg-primary/10 rounded-full blur-3xl"></div>
    </div>
    <div class="relative flex items-center justify-between gap-4">
      <div class="flex items-center gap-4 min-w-0">
        <button 
          class="btn btn-ghost btn-circle hover:bg-base-200/80 transition-all duration-200" 
          on:click={back} 
          aria-label={translate('frontend/src/routes/messages/[id]/+page.svelte::back_aria_label')}
        >
          <ChevronLeft size={24} />
        </button>

        <div class="relative">
          <div class="avatar">
            <div class="w-12 h-12 sm:w-14 sm:h-14 rounded-full overflow-hidden">
              {#if contactAvatar}
                <img src={contactAvatar} alt={translate('frontend/src/routes/messages/[id]/+page.svelte::contact_avatar_alt')} class="w-full h-full object-cover" />
              {:else}
                <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-xl font-bold text-primary">
                  {(name || translate('frontend/src/routes/messages/[id]/+page.svelte::unknown_avatar_initial')).charAt(0).toUpperCase()}
                </div>
              {/if}
            </div>
          </div>
          {#if $onlineUsers.some(u => u.id === id)}
            <div class="absolute -bottom-1 -right-1 w-4 h-4 bg-success rounded-full border-2 border-base-100 shadow-sm animate-pulse"></div>
          {:else}
            <div class="absolute -bottom-1 -right-1 w-4 h-4 bg-base-300 rounded-full border-2 border-base-100 shadow-sm"></div>
          {/if}
        </div>

        <div class="flex flex-col min-w-0">
          <h1 class="text-xl sm:text-2xl font-black tracking-tight truncate">{name}</h1>
          <div class="flex items-center gap-2">
            {#if $onlineUsers.some(u => u.id === id)}
              <span class="text-[10px] font-black uppercase tracking-widest text-success">{translate('frontend/src/routes/messages/[id]/+page.svelte::online')}</span>
            {:else}
              <span class="text-[10px] font-black uppercase tracking-widest opacity-40">{translate('frontend/src/routes/messages/[id]/+page.svelte::offline')}</span>
            {/if}
          </div>
        </div>
      </div>
      
      <div class="flex items-center gap-2">
        <button class="btn btn-ghost btn-circle w-10 h-10 hover:bg-primary/10 hover:text-primary transition-all" on:click={toggleSearch}>
          <Search size={20} />
        </button>
        <div class="dropdown dropdown-end">
          <button class="btn btn-ghost btn-circle w-10 h-10 hover:bg-primary/10 hover:text-primary transition-all">
            <MoreVertical size={20} />
          </button>
          <ul class="dropdown-content menu p-2 shadow-2xl bg-base-100 rounded-2xl w-52 border border-base-200 mt-2 z-50">
            <li><button class="rounded-xl font-medium" on:click={openProfile}>{translate('frontend/src/routes/messages/[id]/+page.svelte::view_profile')}</button></li>
            <li><button class="rounded-xl font-medium text-error" on:click={blockThisUser}>{translate('frontend/src/routes/messages/[id]/+page.svelte::block_user')}</button></li>
          </ul>
        </div>
      </div>
    </div>
  </section>

  {#if showSearch}
    <div class="mb-4 p-3 rounded-2xl bg-base-100/80 border border-base-200 flex items-center gap-2 shadow-sm" in:fade out:fade>
      <input class="input input-sm bg-base-200/50 border-transparent focus:border-primary/30 flex-1 rounded-xl font-medium" placeholder={t('frontend/src/routes/messages/[id]/+page.svelte::search_messages_placeholder')} bind:value={searchQuery} bind:this={searchInput} />
      <div class="flex gap-1">
        <button class="btn btn-ghost btn-sm btn-square rounded-lg" on:click={prevResult}><ChevronUp size={16} /></button>
        <button class="btn btn-ghost btn-sm btn-square rounded-lg" on:click={nextResult}><ChevronDown size={16} /></button>
        <button class="btn btn-ghost btn-sm btn-square rounded-lg text-error" on:click={toggleSearch}><X size={16} /></button>
      </div>
    </div>
  {/if}

  <div class="flex-1 min-h-0 bg-base-100 rounded-[2.5rem] border border-base-300 shadow-md overflow-hidden relative mb-2 flex flex-col">
    <!-- Messages Area -->
    <div class="flex-1 h-full overflow-y-auto p-6 space-y-6 scroll-smooth custom-scrollbar" bind:this={chatBox}>
      {#if hasMore}
        <div class="text-center py-4">
          <button class="btn btn-ghost btn-sm rounded-xl gap-2 font-black uppercase tracking-widest text-[10px] border border-base-300 hover:bg-base-200" on:click={() => load(true)}>
            <ChevronRight class="rotate-[-90deg] w-3 h-3" />
            {translate('frontend/src/routes/messages/[id]/+page.svelte::load_more_messages')}
          </button>
        </div>
      {/if}

      {#if convo.length === 0 && !hasMore}
        <div class="h-full flex flex-col items-center justify-center text-center opacity-30 py-20">
          <div class="w-20 h-20 rounded-full bg-base-200 flex items-center justify-center mb-4">
            <MessageSquare size={40} />
          </div>
          <p class="text-sm font-black uppercase tracking-[0.2em]">
            {t('frontend/src/routes/messages/[id]/+page.svelte::no_messages_yet') || 'No messages yet.'}
          </p>
        </div>
      {/if}
      
      {#each convo as m, index (m.id)}
        {#if index === 0 || !sameDate(m.created_at, convo[index-1].created_at)}
          <div class="flex justify-center my-4">
            <div class="bg-base-200/60 backdrop-blur-sm px-4 py-1.5 rounded-full text-[10px] font-black uppercase tracking-widest text-base-content/50 border border-base-200">
              {formatDateLabel(m.created_at)}
            </div>
          </div>
        {/if}
        
        <div
          class="{`flex ${m.sender_id === $auth?.id ? 'justify-end' : 'justify-start'} group ${searchResults.includes(index) ? 'bg-primary/10 -mx-6 px-6 py-2' : ''} ${highlightedMessageId === m.id ? 'highlighted-message' : ''}`}"
          use:registerMsgEl={index}
          in:fade={{ duration: 200 }}
        >
          <div class={`flex gap-3 max-w-[85%] sm:max-w-[70%] items-end ${m.sender_id === $auth?.id ? 'flex-row-reverse' : 'flex-row'}`}>
            {#if m.sender_id !== $auth?.id}
              <div class="avatar flex-shrink-0 mb-1">
                <div class="w-10 h-10 rounded-full overflow-hidden shrink-0 bg-base-300">
                  {#if contactAvatar}
                    <img src={contactAvatar} alt="Contact" class="w-full h-full object-cover" />
                  {:else}
                    <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-sm font-black text-primary">
                      {(name || 'U').charAt(0).toUpperCase()}
                    </div>
                  {/if}
                </div>
              </div>
            {/if}
            
            <div class={`flex flex-col group/msg ${m.sender_id === $auth?.id ? 'items-end' : 'items-start'}`}>
              <div class="relative flex flex-col">
                <!-- Reply preview (if this message is a reply) -->
                {#if m.reply_to_id && m.reply_text}
                  <button
                    type="button"
                    class={`flex items-start gap-2 mb-2 px-3 py-2 rounded-xl text-xs border-l-2 w-full text-left transition-all hover:scale-[1.02] cursor-pointer ${
                      m.sender_id === $auth?.id 
                        ? 'bg-primary-content/10 border-primary-content/30 hover:bg-primary-content/20' 
                        : 'bg-base-200/50 border-primary/30 hover:bg-base-200/70'
                    }`}
                    on:click|stopPropagation={() => scrollToMessage(m.reply_to_id)}
                    title={t('frontend/src/routes/messages/[id]/+page.svelte::click_to_view_original')}
                  >
                    <CornerDownRight size={12} class="mt-0.5 shrink-0 opacity-50" />
                    <div class="flex-1 min-w-0">
                      <p class={`text-[9px] font-black uppercase tracking-wider mb-0.5 ${
                        m.sender_id === $auth?.id ? 'opacity-60' : 'text-primary opacity-80'
                      }`}>
                        {m.reply_sender_id === $auth?.id ? t('frontend/src/routes/messages/[id]/+page.svelte::you') : name}
                      </p>
                      <p class="opacity-70 line-clamp-2">{m.reply_text}</p>
                    </div>
                  </button>
                {/if}
                
                {#if m.image}
                  <div class="mb-3 rounded-3xl overflow-hidden shadow-xl border border-base-200 bg-base-200/50 group/img relative">
                    <button type="button" class="block p-0 m-0 w-full" on:click={() => openImage(m.image)}>
                      <img src={m.image} alt="" class="max-w-full sm:max-w-md max-h-96 object-contain hover:scale-[1.02] transition-transform duration-500" />
                    </button>
                  </div>
                {/if}

                {#if m.file}
                  <a
                    href="{`/api/messages/file/${m.id}`}"
                    download="{m.file_name || t('frontend/src/routes/messages/[id]/+page.svelte::file_link_text')}"
                    class="flex items-center gap-4 p-4 mb-3 bg-base-100/80 rounded-3xl border border-base-200 hover:border-primary/30 hover:shadow-lg transition-all group/file"
                  >
                    <div class="w-12 h-12 bg-primary/10 rounded-2xl flex items-center justify-center flex-shrink-0 group-hover/file:bg-primary group-hover/file:text-primary-content transition-colors">
                      <Paperclip size={24} />
                    </div>
                    <div class="flex-1 min-w-0">
                      <p class="text-sm font-black truncate tracking-tight">{m.file_name || translate('frontend/src/routes/messages/[id]/+page.svelte::file_name_display')}</p>
                      <p class="text-[10px] font-bold uppercase tracking-widest opacity-40">{translate('frontend/src/routes/messages/[id]/+page.svelte::click_to_download')}</p>
                    </div>
                  </a>
                {/if}
                
                {#if m.text}
                  {#if isEmojiOnly(m.text)}
                    <div
                      class="text-6xl py-2 cursor-pointer transition-transform hover:scale-110"
                      on:click={() => { m.showTime = !m.showTime; convo = [...convo]; }}
                      role="button"
                      tabindex="0"
                      on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); m.showTime = !m.showTime; convo = [...convo]; } }}
                    >
                      {m.text}
                    </div>
                  {:else}
                    <div 
                      class={`relative rounded-[2rem] px-5 py-4 shadow-sm transition-all duration-300 group/bubble message-bubble w-fit ${
                        m.sender_id === $auth?.id
                          ? 'bg-primary text-primary-content rounded-br-lg shadow-primary/20 hover:shadow-primary/30 [&_a]:text-primary-content'
                          : 'bg-base-200 border border-base-300 shadow-sm text-base-content rounded-bl-lg hover:border-primary/20'
                      }`}
                      on:click={() => { m.showTime = !m.showTime; convo = [...convo]; }}
                      role="button"
                      tabindex="0"
                      on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); m.showTime = !m.showTime; convo = [...convo]; } }}
                    >
                      {#if m.structured}
                        <div class="markdown prose prose-sm max-w-none prose-headings:text-inherit prose-p:text-inherit prose-strong:text-inherit prose-code:text-inherit prose-pre:bg-black/10 prose-a:text-inherit prose-a:underline">
                          {@html renderMarkdown(m.text)}
                        </div>
                      {:else}
                        <p class="text-sm font-medium leading-relaxed">{hyphenateLongWords(m.text)}</p>
                      {/if}

                      <!-- Message Status Overlay for sent messages -->
                      {#if m.sender_id === $auth?.id}
                        <div class="absolute -bottom-1 -left-2 flex items-center gap-1 opacity-0 group-hover/bubble:opacity-100 transition-opacity">
                          {#if m.is_read}
                            <CheckCheck size={12} class="text-primary" />
                          {:else}
                            <Check size={12} class="text-base-content/40" />
                          {/if}
                        </div>
                      {/if}
                    </div>
                  {/if}
                {/if}

                {#if m.showTime}
                  <div class={`text-[9px] font-black uppercase tracking-widest opacity-40 mt-2 px-2 flex items-center gap-2 ${m.sender_id === $auth?.id ? 'justify-end' : 'justify-start'}`} in:fade>
                    {formatTime(m.created_at)}
                    {#if m.sender_id === $auth?.id}
                      <span class="inline-flex">
                        {#if m.is_read}
                          <CheckCheck size={10} class="text-primary" />
                        {:else}
                          <Check size={10} />
                        {/if}
                      </span>
                    {/if}
                  </div>
                {/if}

                <!-- Reply button - appears on hover -->
                <button
                  class={`btn btn-ghost btn-xs h-7 px-2 rounded-lg gap-1 opacity-0 group-hover/msg:opacity-100 transition-opacity mt-1 text-base-content/50 hover:text-primary hover:bg-primary/10 ${m.sender_id === $auth?.id ? 'self-end' : 'self-start'}`}
                  on:click|stopPropagation={() => setReplyTo(m)}
                  title={t('frontend/src/routes/messages/[id]/+page.svelte::reply_button')}
                >
                  <Reply size={12} />
                  <span class="text-[9px] font-bold uppercase tracking-wider">{t('frontend/src/routes/messages/[id]/+page.svelte::reply_button')}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      {/each}
    </div>
    <!-- Input Area -->
    <div class="p-4 sm:p-5 border-t border-base-200 bg-base-100 relative z-10">
      {#if imageData || fileData}
        <div class="flex flex-wrap gap-3 mb-4" in:fade>
          {#if imageData}
            <div class="relative group">
              <img src={imageData} alt="" class="h-24 w-24 object-cover rounded-2xl border-4 border-base-100 shadow-lg" />
              <button class="absolute -top-2 -right-2 btn btn-circle btn-xs btn-error shadow-lg" on:click={() => imageData = null}><X size={12} /></button>
            </div>
          {/if}
          {#if fileData}
            <div class="flex items-center gap-3 p-3 bg-base-100 rounded-2xl border-2 border-primary/20 shadow-sm max-w-xs">
              <div class="w-8 h-8 bg-primary/10 text-primary rounded-xl flex items-center justify-center"><Paperclip size={14} /></div>
              <span class="text-xs font-black truncate flex-1">{fileName}</span>
              <button class="btn btn-ghost btn-xs btn-circle" on:click={() => {fileData=null; fileName=null;}}><X size={12} /></button>
            </div>
          {/if}
        </div>
      {/if}

      <!-- Reply composer preview -->
      {#if replyToMessage}
        <div class="flex items-start gap-3 mb-4 p-3 bg-base-200/50 rounded-2xl border-l-4 border-primary" in:fade>
          <Reply size={16} class="text-primary shrink-0 mt-0.5" />
          <div class="flex-1 min-w-0">
            <p class="text-[10px] font-black text-primary uppercase tracking-widest mb-1">
              {t('frontend/src/routes/messages/[id]/+page.svelte::replying_to')} {replyToMessage.sender_id === $auth?.id ? t('frontend/src/routes/messages/[id]/+page.svelte::yourself') : name}
            </p>
            <p class="text-sm text-base-content/70 line-clamp-2">
              {replyToMessage.text || (replyToMessage.image ? t('frontend/src/routes/messages/[id]/+page.svelte::image_message') : t('frontend/src/routes/messages/[id]/+page.svelte::file_message'))}
            </p>
          </div>
          <button 
            class="btn btn-ghost btn-xs btn-circle shrink-0" 
            on:click={clearReply}
            title={t('frontend/src/routes/messages/[id]/+page.svelte::cancel_reply')}
          >
            <X size={14} />
          </button>
        </div>
      {/if}

      <div class="flex flex-col gap-3">
        <div class="flex items-center gap-2">
          <!-- Hidden file inputs -->
          <input type="file" accept="image/*" class="hidden" bind:this={fileInput} on:change={fileChanged} />
          <input type="file" class="hidden" bind:this={generalFileInput} on:change={generalFileChanged} />
          
          <div class="attachment-menu dropdown dropdown-top">
            <button class="btn btn-ghost btn-sm h-10 w-10 rounded-xl p-0 hover:bg-primary/10 hover:text-primary transition-all">
              <Paperclip size={20} />
            </button>
            <ul class="dropdown-content menu p-2 shadow-2xl bg-base-100 rounded-2xl w-48 mb-2 border border-base-200">
              <li><button on:click={chooseFile} class="rounded-xl"><ImagePlus size={16} class="text-primary" /> {translate('frontend/src/routes/messages/[id]/+page.svelte::photo_attachment')}</button></li>
              <li><button on:click={chooseGeneralFile} class="rounded-xl"><Paperclip size={16} class="text-secondary" /> {translate('frontend/src/routes/messages/[id]/+page.svelte::file_attachment')}</button></li>
            </ul>
          </div>
          
          <div class="relative">
            <button type="button" class={`emoji-picker btn btn-ghost btn-sm h-10 w-10 rounded-xl p-0 transition-all ${showEmojiPicker ? 'text-primary bg-primary/10' : ''}`} on:click={() => showEmojiPicker = !showEmojiPicker}>
              <Smile size={20} />
            </button>

            {#if showEmojiPicker}
              <div class="emoji-picker absolute bottom-full left-0 mb-4 bg-base-100 p-4 rounded-[2rem] shadow-2xl border border-base-200 w-80 z-50 overflow-hidden" in:fade={{ duration: 150 }}>
                <div class="grid grid-cols-7 gap-1 max-h-48 overflow-y-auto pr-2 custom-scrollbar">
                  {#each commonEmojis as emoji}
                    <button type="button" class="w-9 h-9 flex items-center justify-center text-xl hover:bg-base-200 rounded-xl transition-colors" on:click={() => insertEmoji(emoji)}>{emoji}</button>
                  {/each}
                </div>
              </div>
            {/if}
          </div>
          
          <button class={`btn btn-ghost btn-sm h-10 w-10 rounded-xl p-0 transition-all ${structured ? 'text-primary bg-primary/10' : ''}`} on:click={() => structured = !structured} title={t('frontend/src/routes/messages/[id]/+page.svelte::structured_messaging')}>
            <Table size={20} />
          </button>
        </div>

        <div class="flex items-end gap-3 relative">
          <div class="flex-1 relative">
            {#if structured}
              <div class="w-full bg-base-100 rounded-2xl border-2 border-base-200 focus-within:border-primary/30 shadow-sm overflow-hidden transition-all overflow-y-auto max-h-40" in:fade>
                <MarkdownEditor 
                  bind:value={msg} 
                  placeholder={t('frontend/src/routes/messages/[id]/+page.svelte::type_a_message_placeholder')}
                  className="forum-md-editor"
                  showExtraButtons={false}
                />
              </div>
            {:else}
              <textarea
                class="textarea w-full bg-base-200/50 focus:bg-base-100 border-2 border-transparent focus:border-primary/30 transition-all duration-300 rounded-[1.5rem] px-5 py-4 text-sm font-medium resize-none max-h-40 min-h-[56px] custom-scrollbar"
                placeholder={t('frontend/src/routes/messages/[id]/+page.svelte::type_a_message_placeholder')}
                bind:value={msg}
                bind:this={msgInput}
                on:input={adjustHeight}
                on:keydown={handleKeydown}
              ></textarea>
            {/if}
          </div>
          
          <button
            class="btn btn-primary h-14 w-14 rounded-2xl p-0 shadow-lg shadow-primary/20 hover:shadow-xl hover:shadow-primary/30 transition-all duration-300 shrink-0"
            on:click={send}
            disabled={!msg.trim() && !imageData && !fileData}
          >
            <Send size={24} />
          </button>
        </div>
        {#if err}
          <div class="mt-2 p-2 bg-error/10 border border-error/20 rounded-xl" in:fade>
            <p class="text-error text-xs font-bold uppercase tracking-tight text-center">{err}</p>
          </div>
        {/if}
      </div>
    </div>
    </div>
  </div>

<!-- Image Lightbox Overlay -->
{#if lightboxOpen && modalImage}
  <div 
    class="fixed inset-0 z-[100] bg-black/90 backdrop-blur-md flex items-center justify-center p-4" 
    in:fade={{ duration: 200 }} 
    out:fade={{ duration: 200 }} 
    on:click|self={closeLightbox}
    role="button"
    tabindex="-1"
    aria-label="Close lightbox"
  >
    <div class="absolute top-6 right-6 flex items-center gap-4">
      <a href={modalImage} download class="btn btn-ghost text-white font-black uppercase tracking-widest text-xs gap-2"><Download size={18}/> {t('frontend/src/routes/messages/[id]/+page.svelte::download_button')}</a>
      <button class="btn btn-circle btn-ghost text-white" on:click={closeLightbox}><X size={24}/></button>
    </div>

    <img src={modalImage} alt="" class="max-w-full max-h-full object-contain rounded-xl shadow-2xl" transition:scale={{ duration: 300, start: 0.95 }} />

    {#if imageUrls.length > 1}
      <button class="absolute left-6 btn btn-circle btn-ghost text-white lg:btn-lg" on:click|stopPropagation={showPrevImage}><ChevronLeft size={32}/></button>
      <button class="absolute right-6 btn btn-circle btn-ghost text-white lg:btn-lg" on:click|stopPropagation={showNextImage}><ChevronRight size={32}/></button>
    {/if}
  </div>
{/if}

{#if showProfile}
  <UserProfileModal userId={id} on:close={() => (showProfile = false)} />
{/if}

<ConfirmModal bind:this={confirmModal} />

<style>
  .messages-page {
    font-family: 'Outfit', sans-serif;
  }

  .custom-scrollbar::-webkit-scrollbar {
    width: 6px;
  }
  .custom-scrollbar::-webkit-scrollbar-track {
    background: transparent;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb {
    background: hsl(var(--bc) / 0.1);
    border-radius: 10px;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb:hover {
    background: hsl(var(--bc) / 0.2);
  }

  :global(.token.operator), :global(.token.entity), :global(.token.url), :global(.language-css .token.string), :global(.style .token.string) {
    background: transparent !important;
  }

  /* Force link visibility in sent messages (blue background) */
  :global(.bg-primary a), :global(.bg-primary a *) {
    color: white !important;
    text-decoration: underline !important;
    opacity: 1 !important;
  }
  :global(.bg-primary a:hover) {
    color: rgba(255, 255, 255, 0.8) !important;
  }

  /* Specific message bubble tweaks */
  :global(.group\/bubble .markdown a) {
    color: inherit !important;
  }

  /* Improved High-Visibility Highlighted message row style */
  .highlighted-message {
    position: relative;
    z-index: 10;
    transition: background-color 0.3s ease;
  }

  .highlighted-message::before {
    content: '';
    position: absolute;
    inset: -0.5rem -1.5rem;
    background: linear-gradient(90deg, 
      hsl(var(--p) / 0.3) 0%, 
      hsl(var(--p) / 0.1) 40%, 
      transparent 100%
    );
    border-left: 6px solid hsl(var(--p));
    pointer-events: none;
    animation: highlight-row-entrance 4s cubic-bezier(0.16, 1, 0.3, 1) forwards;
    z-index: -1;
  }

  @keyframes highlight-row-entrance {
    0% { opacity: 0; transform: translateX(-20px); }
    5% { opacity: 1; transform: translateX(0); }
    80% { opacity: 1; }
    100% { opacity: 0; }
  }

  /* Intense Pop effect for the message bubble */
  .highlighted-message :global(.message-bubble) {
    animation: bubble-highlight-intense 4s cubic-bezier(0.34, 1.56, 0.64, 1) forwards !important;
    position: relative;
    z-index: 20;
  }

  @keyframes bubble-highlight-intense {
    0% { 
      transform: scale(1); 
      box-shadow: 0 0 0 0 hsl(var(--p) / 0);
    }
    5% { 
      transform: scale(1.15); 
      background-color: hsl(var(--p)) !important;
      color: hsl(var(--pc)) !important;
      box-shadow: 0 0 0 15px hsl(var(--p) / 0.4), 0 30px 60px -15px hsl(var(--p) / 0.6);
      border-color: white !important;
      filter: saturate(1.8) brightness(1.3) contrast(1.1);
      z-index: 50;
    }
    20% {
      transform: scale(1.1);
      background-color: hsl(var(--p)) !important;
      color: hsl(var(--pc)) !important;
      box-shadow: 0 0 0 10px hsl(var(--p) / 0.3), 0 20px 40px -12px hsl(var(--p) / 0.5);
      filter: saturate(1.5) brightness(1.2);
    }
    80% { 
      transform: scale(1.08); 
      background-color: hsl(var(--p) / 0.9) !important;
      color: hsl(var(--pc)) !important;
      opacity: 1; 
      box-shadow: 0 0 0 6px hsl(var(--p) / 0.2), 0 15px 30px -8px hsl(var(--p) / 0.4);
      filter: brightness(1.1);
    }
    100% { 
      transform: scale(1); 
      box-shadow: 0 0 0 0 hsl(var(--p) / 0);
      filter: brightness(1);
    }
  }

  /* If it's an image, also highlight it */
  .highlighted-message :global(.group\/img) {
    animation: bubble-highlight-intense 4s cubic-bezier(0.34, 1.56, 0.64, 1) forwards !important;
    border: 3px solid hsl(var(--p)) !important;
  }
</style>

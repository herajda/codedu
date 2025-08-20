<script lang="ts">
  import { onMount, onDestroy, afterUpdate } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { apiFetch, apiJSON } from '$lib/api';
  import { createEventSource } from '$lib/sse';
  import { auth } from '$lib/auth';
  import { compressImage } from '$lib/utils/compressImage';
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
    Smile
  } from 'lucide-svelte';
  import { fade, scale } from 'svelte/transition';
  import { sidebarCollapsed } from '$lib/sidebar';
  import UserProfileModal from '$lib/components/UserProfileModal.svelte';

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
  let modalImage: string | null = null;
  let lightboxOpen = false;
  let currentImageIndex: number = -1;
  let esCtrl: { close: () => void } | null = null;
  let chatBox: HTMLDivElement | null = null;
  let msgInput: HTMLTextAreaElement | null = null;
  let showAttachmentMenu = false;
  let showEmojiPicker = false;
  let showProfile = false;
  let showSearch = false;
  let searchQuery = '';
  let searchResults: number[] = [];
  let searchPos = 0;
  let searchInput: HTMLInputElement | null = null;
  let msgEls: HTMLDivElement[] = [];

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
    if (!msg.trim() && !imageData) return;
    const res = await apiFetch('/api/messages', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({ to: parseInt(id), text: msg, image: imageData })
    });
    if (res.ok) { msg=''; imageData=null; offset=0; await load(); }
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
            d.showTime = false;
            convo = [...convo, d];
            if (d.sender_id === parseInt(id)) {
              await apiFetch(`/api/messages/${id}/read`, { method: 'PUT' });
              d.is_read = true;
            }
          }
        });
        src.addEventListener('read', (ev) => {
          const d = JSON.parse((ev as MessageEvent).data);
          if (d.reader_id === parseInt(id)) {
            for (const m of convo) {
              if (m.sender_id === $auth?.id && m.recipient_id === parseInt(id)) {
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
    
    // Add click outside handler
    document.addEventListener('click', handleClickOutside);
  });

  onDestroy(() => { 
    esCtrl?.close(); 
    document.removeEventListener('click', handleClickOutside);
  });
  function back() { goto('/messages'); }

  function chooseFile() { fileInput?.click(); }
  async function fileChanged(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;
    const compressed = await compressImage(f, 1280, 0.8);
    const r = new FileReader();
    r.onload = () => { imageData = r.result as string; };
    r.readAsDataURL(compressed);
  }

  function openProfile() {
    showProfile = true;
  }

  async function blockThisUser() {
    if (!confirm('Block this user?')) return;
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

  // Attach keyboard navigation only while lightbox is open
  $: if (lightboxOpen) {
    document.addEventListener('keydown', handleLightboxKeydown);
  } else {
    document.removeEventListener('keydown', handleLightboxKeydown);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (msg.trim() || imageData) {
        send();
      }
    }
  }
</script>

<!-- Enhanced Chat Window -->
<div class={`chat-window fixed top-16 bottom-0 right-0 left-0 ${$sidebarCollapsed ? 'sm:left-0' : 'sm:left-60'} z-40 flex flex-col bg-gradient-to-br from-base-100/95 to-base-200/95 backdrop-blur-xl border-l border-base-300/30`}>
  <!-- Enhanced Header -->
  <div class="chat-header relative z-30 mx-2 sm:mx-4 mt-2 sm:mt-3 flex items-center justify-between p-2 sm:p-4 rounded-xl bg-base-100/80 backdrop-blur supports-[backdrop-filter]:bg-base-100/85 border border-base-300/30 shadow-md">
    <div class="flex items-center gap-3 min-w-0">
      <button 
        class="btn btn-ghost btn-circle hover:bg-base-200/80 transition-all duration-200" 
        on:click={back} 
        aria-label="Back"
      >
        <ChevronLeft class="w-5 h-5" />
      </button>
      
      <!-- Enhanced Avatar -->
      <div class="relative">
        <div class="avatar">
          <div class="w-12 h-12 rounded-full overflow-hidden ring-2 ring-primary/20 shadow-lg">
            {#if contactAvatar}
              <img src={contactAvatar} alt="Contact" class="w-full h-full object-cover" />
            {:else}
              <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-lg font-semibold text-primary">
                {(name || 'U').charAt(0).toUpperCase()}
              </div>
            {/if}
          </div>
        </div>
        <!-- Online indicator -->
        <div class="absolute -bottom-1 -right-1 w-4 h-4 bg-success rounded-full border-2 border-base-100 shadow-sm"></div>
      </div>
      
      <div class="flex flex-col min-w-0">
        <h2 class="font-semibold text-lg truncate">{name}</h2>
        <div class="text-sm text-base-content/60 flex items-center gap-1">
          <div class="w-2 h-2 bg-success rounded-full animate-pulse"></div>
          Online
        </div>
      </div>
    </div>
    
    <!-- Header Actions -->
    <div class="flex items-center gap-2">
      <button class="btn btn-ghost btn-circle hover:bg-base-200/80 transition-all duration-200" on:click={toggleSearch}>
        <Search class="w-4 h-4" />
      </button>
      <div class="dropdown dropdown-bottom dropdown-end">
        <button class="btn btn-ghost btn-circle hover:bg-base-200/80 transition-all duration-200">
          <MoreVertical class="w-4 h-4" />
        </button>
        <ul class="dropdown-content menu p-2 shadow-lg bg-base-100 rounded-box w-52 border border-base-300/30 z-50">
          <li><button class="gap-2" on:click={openProfile}>View Profile</button></li>
          <li><button class="gap-2 text-error" on:click={blockThisUser}>Block User</button></li>
        </ul>
      </div>
    </div>
  </div>

  {#if showSearch}
    <div class="mx-2 sm:mx-4 mt-2 sm:mt-3 p-2 rounded-lg bg-base-100/80 backdrop-blur supports-[backdrop-filter]:bg-base-100/85 border border-base-300/30 flex items-center gap-2 shadow-sm" in:fade out:fade>
      <input class="input input-sm input-bordered flex-1" placeholder="Search messages" bind:value={searchQuery} bind:this={searchInput} />
      <button class="btn btn-sm" on:click={prevResult}>
        <ChevronUp class="w-4 h-4" />
      </button>
      <button class="btn btn-sm" on:click={nextResult}>
        <ChevronDown class="w-4 h-4" />
      </button>
      <button class="btn btn-sm" on:click={toggleSearch}>
        <X class="w-4 h-4" />
      </button>
    </div>
  {/if}

  <!-- Enhanced Chat Messages -->
  <div class="flex-1 overflow-hidden relative z-0">
    <div class="h-full overflow-y-auto p-6 space-y-6" bind:this={chatBox}>
      {#if hasMore}
        <div class="text-center">
          <button class="btn btn-outline btn-sm glass" on:click={() => load(true)}>
            Load more messages
          </button>
        </div>
      {/if}
      
      {#each convo as m, index (m.id)}
        {#if index === 0 || !sameDate(m.created_at, convo[index-1].created_at)}
          <div class="flex justify-center">
            <div class="bg-base-200/60 backdrop-blur-sm px-4 py-2 rounded-full text-sm font-medium text-base-content/70 border border-base-300/30">
              {formatDate(m.created_at)}
            </div>
          </div>
        {/if}
        
        <div
          class={`flex ${m.sender_id === $auth?.id ? 'justify-end' : 'justify-start'} group ${searchResults.includes(index) ? 'bg-warning/20' : ''}`}
          use:registerMsgEl={index}
        >
            <div class="flex gap-3 max-w-[85%] sm:max-w-[75%] items-end">
            {#if m.sender_id !== $auth?.id}
              <div class="avatar flex-shrink-0">
                <div class="w-8 h-8 rounded-full overflow-hidden ring-1 ring-base-300/50">
                  {#if contactAvatar}
                    <img src={contactAvatar} alt="Contact" class="w-full h-full object-cover" />
                  {:else}
                    <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-sm font-semibold text-primary">
                      {(name || 'U').charAt(0).toUpperCase()}
                    </div>
                  {/if}
                </div>
              </div>
            {/if}
            
            <div class="relative flex flex-col">
              {#if m.image}
                <div class="mb-2">
                  <button type="button" class="block p-0 m-0 bg-transparent border-0 focus:outline-none focus:ring-2 focus:ring-primary/50 rounded-2xl" on:click={() => openImage(m.image)} aria-label="Open attachment">
                    <img
                      src={m.image}
                      alt="Attachment"
                      class="max-w-[70vw] sm:max-w-xs w-full rounded-2xl shadow-lg"
                    />
                  </button>
                </div>
              {/if}
              
              {#if m.text}
                <div 
                  class={`message-bubble relative rounded-2xl px-4 py-3 whitespace-pre-wrap break-words shadow-sm transition-all duration-200 ${
                    m.sender_id === $auth?.id 
                      ? 'bg-gradient-to-br from-primary to-primary/80 text-primary-content rounded-br-md' 
                      : 'bg-base-200/80 backdrop-blur-sm border border-base-300/30 rounded-bl-md'
                  } ${m.recipient_id === $auth?.id && !m.is_read ? 'ring-2 ring-primary/50 shadow-lg' : ''}`}
                  on:click={() => { m.showTime = !m.showTime; convo = [...convo]; }}
                  role="button"
                  tabindex="0"
                  on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); m.showTime = !m.showTime; convo = [...convo]; } }}
                >
                  {hyphenateLongWords(m.text)}
                  
                  <!-- Message Status -->
                  {#if m.sender_id === $auth?.id}
                    <div class="absolute -bottom-5 right-0 flex items-center gap-1 text-xs opacity-60">
                      {#if m.showTime}<span class="text-base-content/60">{formatTime(m.created_at)}</span>{/if}
                      {#if m.is_read}
                        <CheckCheck class="w-3 h-3 text-primary" />
                      {:else}
                        <Check class="w-3 h-3 text-base-content/40" />
                      {/if}
                    </div>
                  {:else}
                    {#if m.showTime}
                      <div class="absolute -bottom-5 left-0 text-xs opacity-60">
                        <span class="text-base-content/60">{formatTime(m.created_at)}</span>
                      </div>
                    {/if}
                  {/if}
                </div>
              {/if}
            </div>
          </div>
        </div>
      {/each}
    </div>
  </div>

  <!-- Enhanced Input Area -->
  <div class="chat-input-area mx-2 sm:mx-4 mb-2 sm:mb-3 p-4 rounded-xl bg-base-100/80 backdrop-blur supports-[backdrop-filter]:bg-base-100/85 border border-base-300/30 shadow-md">
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
    
    <div class="flex items-end gap-3">
      <!-- Hidden file input -->
      <input type="file" accept="image/*" class="hidden" bind:this={fileInput} on:change={fileChanged} />
      
      <!-- Attachment Menu -->
      <div class="relative attachment-menu">
        <button 
          class="btn btn-circle btn-ghost hover:bg-base-200/80 transition-all duration-200" 
          on:click={() => showAttachmentMenu = !showAttachmentMenu}
        >
          <Paperclip class="w-4 h-4" />
        </button>
        {#if showAttachmentMenu}
          <div class="absolute bottom-full left-0 mb-2 bg-base-100 rounded-lg shadow-lg border border-base-300/30 p-2 backdrop-blur-sm">
            <button class="btn btn-ghost btn-sm gap-2 w-full justify-start" on:click={chooseFile}>
              <ImagePlus class="w-4 h-4" />
              Photo
            </button>
            <button class="btn btn-ghost btn-sm gap-2 w-full justify-start">
              <Paperclip class="w-4 h-4" />
              File
            </button>
          </div>
        {/if}
      </div>
      
      <!-- Emoji Picker -->
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
      
      <!-- Message Input -->
      <div class="flex-1 relative">
        <textarea
          class="textarea textarea-bordered w-full resize-none overflow-hidden bg-base-200/50 backdrop-blur-sm border-base-300/50 focus:border-primary/50 focus:bg-base-100/80 transition-all duration-200 rounded-2xl"
          rows="1"
          style="min-height:0;height:auto"
          placeholder="Type a message..."
          bind:value={msg}
          bind:this={msgInput}
          on:input={adjustHeight}
          on:keydown={handleKeydown}
        ></textarea>
      </div>
      
      <!-- Send Button -->
      <button 
        class="btn btn-circle btn-primary shadow-lg hover:shadow-xl transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed" 
        on:click={send} 
        disabled={!msg.trim() && !imageData} 
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
  <div class={`fixed top-0 bottom-0 right-0 left-0 ${$sidebarCollapsed ? 'sm:left-0' : 'sm:left-60'} z-[100] bg-black/80 backdrop-blur-sm flex items-center justify-center`} on:click|self={closeLightbox} in:fade={{ duration: 150 }} out:fade={{ duration: 150 }} role="dialog" aria-modal="true" aria-label="Image viewer" tabindex="-1" on:keydown={handleLightboxKeydown}>
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

{#if showProfile}
  <UserProfileModal userId={parseInt(id)} on:close={() => (showProfile = false)} />
{/if}

<style>
  .chat-window {
    background: linear-gradient(135deg, hsl(var(--b1) / 0.95) 0%, hsl(var(--b2) / 0.95) 100%);
  }
  
  .message-bubble {
    position: relative;
    transition: all 0.2s ease;
  }
  
  .message-bubble:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  }
  
  .typing-indicator {
    animation: slideIn 0.3s ease-out;
  }
  
  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translateY(10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
  
  .chat-input-area {
    background: linear-gradient(180deg, hsl(var(--b1) / 0.8) 0%, hsl(var(--b1) / 0.95) 100%);
  }
  
  /* Custom scrollbar for chat */
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

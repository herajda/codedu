<script lang="ts">
  import { page } from '$app/stores';
  import { sidebarOpen, sidebarCollapsed } from '$lib/sidebar';
  import { auth } from '$lib/auth';
  import { unreadMessages } from '$lib/stores/messages';
  import { classesStore } from '$lib/stores/classes';
  import { translator } from '$lib/i18n';
  import type { Translator } from '$lib/i18n';
  import { 
    MessageSquare, 
    BookOpen, 
    Compass, 
    ListChecks, 
    FolderOpen, 
    StickyNote, 
    MessagesSquare, 
    LineChart, 
    Sliders,
    Users,
    ChevronDown,
    ChevronRight,
    X,
    LayoutGrid
  } from 'lucide-svelte';

  let translate: Translator;
  $: translate = $translator;

  $: if ($auth) {
    // Load classes when auth state changes
    classesStore.load().catch(() => {
      // Error handling is done in the store
    });
  }

  $: currentPath = $page.url.pathname;

  function isActive(path: string): boolean {
    const normPath = path.replace(/\/$/, '');
    const normCurrent = currentPath.replace(/\/$/, '');
    return normCurrent === normPath;
  }

  function isSection(pathStartsWith: string): boolean {
    const normPath = pathStartsWith.replace(/\/$/, '');
    const normCurrent = currentPath.replace(/\/$/, '');
    return normCurrent === normPath || normCurrent.startsWith(normPath + '/');
  }

  // Persistent state for accordions to prevent unwanted collapses
  let openedSections: Record<string | number, boolean> = {};
  let lastSection = '';
  
  $: {
    // Only auto-open the section when the section actually changes (navigation)
    // This allows manual collapse while staying within the same section
    if (currentPath) {
      let foundSection = '';
      
      $classesStore.classes.forEach(c => {
        const sectionPath = `/classes/${c.id}`;
        if (isSection(sectionPath)) {
          foundSection = sectionPath;
          if (lastSection !== sectionPath) {
            openedSections[c.id] = true;
          }
        }
      });

      if (isSection('/teachers')) {
        foundSection = '/teachers';
        if (lastSection !== '/teachers') {
          openedSections['teachers'] = true;
        }
      }

      if (foundSection !== lastSection) {
        lastSection = foundSection;
        // Re-assign to trigger reactivity for the binding
        openedSections = openedSections;
      }
    }
  }
</script>

<div class={`fixed left-0 z-40 pointer-events-none sm:top-0 sm:h-screen top-16 h-[calc(100dvh-4rem)] ${$sidebarOpen ? 'block' : 'hidden sm:block'}`}>
  <aside class={`relative w-64 h-full overflow-visible transition-transform pointer-events-auto ${$sidebarCollapsed ? '-translate-x-full' : 'translate-x-0'} p-3`}>
    <div class="sidebar-shell h-full overflow-hidden flex flex-col">
      <div class="sidebar-accent"></div>

      <!-- Mobile close -->
      <button
        class="btn btn-square btn-ghost absolute right-2 top-2 sm:hidden z-50 rounded-xl"
        on:click={() => sidebarOpen.set(false)}
        aria-label="Close sidebar"
        type="button"
      >
        <X size={20} />
      </button>

      <div class="sidebar-content h-full overflow-y-auto pr-2 pt-2 pb-6 custom-scrollbar">

        <nav class="nav-section mb-6">
          <a
            href="/dashboard"
            class="nav-link"
            class:is-active={isActive('/dashboard')}
            on:click={() => sidebarOpen.set(false)}
          >
            <div class="icon-box text-primary bg-primary/10">
              <LayoutGrid size={18} />
            </div>
            <span class="truncate font-bold text-sm tracking-tight">{translate('frontend/src/lib/Sidebar.svelte::dashboard_link')}</span>
          </a>
          <a
            href="/messages"
            class="nav-link"
            class:is-active={isActive('/messages')}
            on:click={() => sidebarOpen.set(false)}
          >
            <div class="icon-box text-blue-500 bg-blue-500/10">
              <MessageSquare size={18} />
            </div>
            <span class="truncate font-bold text-sm tracking-tight">{translate('frontend/src/lib/Sidebar.svelte::messages_link')}</span>
            {#if $unreadMessages > 0 && !isActive('/messages')}
              <span class="badge badge-primary badge-sm ml-auto font-black shadow-sm">{$unreadMessages}</span>
            {/if}
          </a>

          {#if $auth?.role === 'teacher' || $auth?.role === 'admin'}
          <a
            href="/files"
            class="nav-link"
            class:is-active={isActive('/files')}
            on:click={() => sidebarOpen.set(false)}
          >
            <div class="icon-box text-warning bg-warning/10">
              <FolderOpen size={18} />
            </div>
            <span class="truncate font-bold text-sm tracking-tight">{translate('frontend/src/lib/Sidebar.svelte::teachers_files_link')}</span>
          </a>
          {/if}
        </nav>

        <div class="divider-glow mx-2 mb-6"></div>

        <div class="section-container">
          <div class="section-title">{translate('frontend/src/lib/Sidebar.svelte::classes_title')}</div>
          <ul class="space-y-3 pb-4">
            {#each $classesStore.classes as c (c.id)}
              <li class="nav-collapsible" data-open={!!openedSections[c.id]}>
                <details bind:open={openedSections[c.id]} class="group/details">
                  <summary class="nav-summary">
                    <div class="icon-box text-primary bg-primary/10">
                      <BookOpen size={18} />
                    </div>
                    <span class="truncate font-bold text-sm tracking-tight flex-1">{c.name}</span>
                    <ChevronDown size={14} class="opacity-30 group-open/details:rotate-180 transition-transform duration-300" />
                  </summary>
                  <div class="nav-group mt-1">
                    {#if $auth?.role === 'student'}
                      <a 
                        class="nav-sublink" 
                        class:is-active={isActive(`/classes/${c.id}/overview`)} 
                        href={`/classes/${c.id}/overview`} 
                      >
                        <span class="nav-emoji">🏠</span>
                        <span class="text-xs font-semibold">{translate('frontend/src/lib/Sidebar.svelte::class_overview_link')}</span>
                      </a>
                    {/if}
                    <a 
                      class="nav-sublink" 
                      class:is-active={isActive(`/classes/${c.id}/assignments`)} 
                      href={`/classes/${c.id}/assignments`} 
                    >
                      <span class="nav-emoji">📋</span>
                      <span class="text-xs font-semibold">{translate('frontend/src/lib/Sidebar.svelte::class_assignments_link')}</span>
                    </a>
                    <a 
                      class="nav-sublink" 
                      class:is-active={isActive(`/classes/${c.id}/files`)} 
                      href={`/classes/${c.id}/files`} 
                    >
                      <span class="nav-emoji">📁</span>
                      <span class="text-xs font-semibold">{translate('frontend/src/lib/Sidebar.svelte::class_files_link')}</span>
                    </a>
                    <a 
                      class="nav-sublink" 
                      class:is-active={isActive(`/classes/${c.id}/notes`)} 
                      href={`/classes/${c.id}/notes`} 
                    >
                      <span class="nav-emoji">📒</span>
                      <span class="text-xs font-semibold">{translate('frontend/src/lib/Sidebar.svelte::class_notes_link')}</span>
                    </a>
                    <a 
                      class="nav-sublink" 
                      class:is-active={isActive(`/classes/${c.id}/forum`)} 
                      href={`/classes/${c.id}/forum`} 
                    >
                      <span class="nav-emoji">💬</span>
                      <span class="text-xs font-semibold">{translate('frontend/src/lib/Sidebar.svelte::class_forum_link')}</span>
                    </a>
                    {#if $auth?.role !== 'student'}
                      <a 
                        class="nav-sublink" 
                        class:is-active={isActive(`/classes/${c.id}/progress`)} 
                        href={`/classes/${c.id}/progress`} 
                      >
                        <span class="nav-emoji">📊</span>
                        <span class="text-xs font-semibold">{translate('frontend/src/lib/Sidebar.svelte::class_progress_link')}</span>
                      </a>
                      <a 
                        class="nav-sublink" 
                        class:is-active={isActive(`/classes/${c.id}/settings`)} 
                        href={`/classes/${c.id}/settings`} 
                      >
                        <span class="nav-emoji">🛠️</span>
                        <span class="text-xs font-semibold">{translate('frontend/src/lib/Sidebar.svelte::class_settings_link')}</span>
                      </a>
                    {/if}
                  </div>
                </details>
              </li>
            {/each}
            {#if !$classesStore.classes.length && !$classesStore.loading && !$classesStore.error}
              <li class="px-4 py-2 text-xs font-bold opacity-30 italic">{translate('frontend/src/lib/Sidebar.svelte::no_classes_message')}</li>
            {/if}
            {#if $classesStore.loading}
              <li class="px-4 py-2 text-xs font-bold opacity-30 animate-pulse">{translate('frontend/src/lib/Sidebar.svelte::loading_classes_message')}</li>
            {/if}
          </ul>
        </div>

        {#if $auth?.role === 'teacher' || $auth?.role === 'admin'}
          <div class="divider-glow mx-2 mb-6"></div>
          <div class="section-container">
            <div class="section-title">{translate('frontend/src/lib/Sidebar.svelte::teachers_title')}</div>
            <ul class="space-y-3 pb-4">
              <li class="nav-collapsible" data-open={!!openedSections['teachers']}>
                <details bind:open={openedSections['teachers']} class="group/details">
                  <summary class="nav-summary">
                    <div class="icon-box text-primary bg-primary/10">
                      <Users size={18} />
                    </div>
                    <span class="truncate font-bold text-sm tracking-tight flex-1">{translate('frontend/src/lib/Sidebar.svelte::teachers_title')}</span>
                    <ChevronDown size={14} class="opacity-30 group-open/details:rotate-180 transition-transform duration-300" />
                  </summary>
                  <div class="nav-group mt-1">
                    <a
                      class="nav-sublink"
                      class:is-active={isActive('/teachers/assignments')}
                      href="/teachers/assignments"
                    >
                      <span class="nav-emoji">📋</span>
                      <span class="text-xs font-semibold">{translate('frontend/src/lib/Sidebar.svelte::teachers_assignments_link')}</span>
                    </a>
                    <a
                      class="nav-sublink"
                      class:is-active={isActive('/teachers/files')}
                      href="/teachers/files"
                    >
                      <span class="nav-emoji">📁</span>
                      <span class="text-xs font-semibold">{translate('frontend/src/lib/Sidebar.svelte::teachers_files_link')}</span>
                    </a>
                    <a
                      class="nav-sublink"
                      class:is-active={isActive('/teachers/forum')}
                      href="/teachers/forum"
                    >
                      <span class="nav-emoji">💬</span>
                      <span class="text-xs font-semibold">{translate('frontend/src/lib/Sidebar.svelte::teachers_forum_link')}</span>
                    </a>
                  </div>
                </details>
              </li>
            </ul>
          </div>
        {/if}



        {#if $classesStore.error}
          <div class="p-4 mt-4 bg-error/10 border border-error/20 rounded-2xl mx-1">
            <p class="text-error text-[10px] font-black uppercase tracking-wider">{$classesStore.error}</p>
          </div>
        {/if}
      </div>
    </div>
  </aside>
</div>

<style>
  .sidebar-shell {
    position: relative;
    border-radius: 1.5rem;
    /* Premium Glassmorphism */
    background: rgba(255, 255, 255, 0.45);
    border: 1px solid rgba(255, 255, 255, 0.4);
    backdrop-filter: blur(20px) saturate(180%);
    -webkit-backdrop-filter: blur(20px) saturate(180%);
    box-shadow:
      0 4px 6px -1px rgba(0, 0, 0, 0.05),
      0 10px 15px -3px rgba(0, 0, 0, 0.05),
      inset 0 1px 0 rgba(255, 255, 255, 0.5);
    padding: 0.5rem;
    transition: all 0.3s ease;
  }

  :global([data-theme='dark']) .sidebar-shell {
    background: rgba(15, 23, 42, 0.45);
    border-color: rgba(255, 255, 255, 0.05);
    box-shadow:
      0 20px 25px -5px rgba(0, 0, 0, 0.3),
      0 10px 10px -5px rgba(0, 0, 0, 0.2),
      inset 0 1px 1px rgba(255, 255, 255, 0.05);
  }

  .sidebar-accent {
    position: absolute;
    left: 0.4rem;
    top: 2rem;
    bottom: 2rem;
    width: 2px;
    border-radius: 2px;
    background: linear-gradient(180deg, transparent, rgba(var(--glow), 0.2), transparent);
    pointer-events: none;
    opacity: 0.5;
  }

  .sidebar-content {
    scrollbar-width: none;
    -ms-overflow-style: none;
  }
  .sidebar-content::-webkit-scrollbar {
    display: none;
  }

  .section-container {
    padding: 0 0.5rem;
  }

  .section-title {
    font-weight: 800;
    font-size: 0.65rem;
    text-transform: uppercase;
    letter-spacing: 0.15em;
    opacity: 0.35;
    padding: 0.75rem 0.5rem;
    font-family: 'Outfit', sans-serif;
  }

  .icon-box {
    width: 32px;
    height: 32px;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    flex-shrink: 0;
  }

  .nav-link {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.625rem 0.75rem;
    margin: 0 0.25rem;
    border-radius: 1rem;
    color: currentColor;
    text-decoration: none;
    transition: all 0.2s ease;
    border: 1px solid transparent;
  }

  .nav-link:hover {
    background: rgba(128, 128, 128, 0.05);
    border-color: rgba(128, 128, 128, 0.1);
    transform: translateX(2px);
  }

  .nav-link.is-active {
    background: white;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.03);
    border-color: rgba(0, 0, 0, 0.04);
  }

  :global([data-theme='dark']) .nav-link.is-active {
    background: rgba(255, 255, 255, 0.03);
    border-color: rgba(255, 255, 255, 0.05);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
  }

  .nav-link.is-active .icon-box {
    background: oklch(var(--p));
    color: oklch(var(--pc));
    box-shadow: 0 0 15px oklch(var(--p) / 0.3);
  }

  .nav-summary {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.625rem 0.75rem;
    margin: 0 0.25rem;
    border-radius: 1rem;
    cursor: pointer;
    transition: all 0.2s ease;
    list-style: none;
    border: 1px solid transparent;
  }

  .nav-summary:hover {
    background: rgba(128, 128, 128, 0.05);
    border-color: rgba(128, 128, 128, 0.1);
    transform: translateX(2px);
  }

  .nav-summary::-webkit-details-marker {
    display: none;
  }

  .nav-group {
    padding-left: 2.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    position: relative;
  }

  .nav-group::before {
    content: '';
    position: absolute;
    left: 1.15rem;
    top: 0;
    bottom: 1rem;
    width: 1px;
    background: linear-gradient(180deg, rgba(128, 128, 128, 0.1), transparent);
  }

  .nav-sublink {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.4rem 0.75rem;
    border-radius: 0.75rem;
    color: currentColor;
    text-decoration: none;
    opacity: 0.6;
    transition: all 0.2s ease;
    position: relative;
  }

  .nav-sublink:hover {
    opacity: 1;
    background: rgba(128, 128, 128, 0.05);
    transform: translateX(2px);
  }

  .nav-sublink.is-active {
    opacity: 1;
    color: oklch(var(--p));
    background: oklch(var(--p) / 0.05);
    font-weight: 700;
  }

  .nav-emoji {
    font-size: 0.95rem;
    transition: all 0.25s cubic-bezier(0.23, 1, 0.32, 1);
    flex-shrink: 0;
    filter: saturate(0.8) opacity(0.85);
    display: flex;
    align-items: center;
    justify-content: center;
    width: 16px;
  }

  .nav-sublink:hover .nav-emoji {
    filter: saturate(1.1) opacity(1);
    transform: scale(1.1) translateY(-1px);
  }

  .nav-sublink.is-active .nav-emoji {
    filter: saturate(1.3) opacity(1);
    transform: scale(1.2);
    filter: drop-shadow(0 0 8px oklch(var(--p) / 0.3));
  }

  /* Glow effects */
  .divider-glow {
    height: 1px;
    background: linear-gradient(90deg, transparent, rgba(128, 128, 128, 0.1), transparent);
  }
</style>

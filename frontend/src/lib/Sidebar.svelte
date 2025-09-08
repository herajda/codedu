<script lang="ts">
  import { page } from '$app/stores';
  import '@fortawesome/fontawesome-free/css/all.min.css';
  import { sidebarOpen, sidebarCollapsed } from '$lib/sidebar';
  import { auth } from '$lib/auth';
  import { unreadMessages } from '$lib/stores/messages';
  import { classesStore } from '$lib/stores/classes';

  $: if ($auth) {
    // Load classes when auth state changes
    classesStore.load().catch(() => {
      // Error handling is done in the store
    });
  }

  function isActive(path: string): boolean {
    return $page.url.pathname === path;
  }

  function isSection(pathStartsWith: string): boolean {
    return $page.url.pathname.startsWith(pathStartsWith);
  }
</script>

<div class={`fixed left-0 z-40 pointer-events-none sm:top-0 sm:h-screen top-16 h-[calc(100dvh-4rem)] ${$sidebarOpen ? 'block' : 'hidden sm:block'}`}>
  <aside class={`relative w-64 h-full overflow-visible transition-transform pointer-events-auto ${$sidebarCollapsed ? '-translate-x-full' : 'translate-x-0'} p-3`}>
    <div class="sidebar-shell h-full overflow-hidden">
      <div class="sidebar-accent"></div>

      <!-- Mobile close -->
      <button
        class="btn btn-square btn-ghost absolute right-2 top-2 sm:hidden"
        on:click={() => sidebarOpen.set(false)}
        aria-label="Close sidebar"
        type="button"
      >
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" class="w-5 h-5">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>

      <div class="sidebar-content h-full overflow-y-auto pr-2">

        <nav class="nav-section">
          <a
            href="/messages"
            class={`nav-link ${isSection('/messages') ? 'is-active' : ''}`}
            on:click={() => sidebarOpen.set(false)}
          >
            <i class="fa-solid fa-message nav-icon" aria-hidden="true"></i>
            <span class="truncate">Messages</span>
            {#if $unreadMessages > 0 && !isSection('/messages')}
              <span class="badge badge-primary badge-sm ml-auto">{$unreadMessages}</span>
            {/if}
          </a>
        </nav>

        <div class="divider-glow my-3"></div>

        <div class="section-title">Classes</div>
        <ul class="space-y-2">
          {#each $classesStore.classes as c}
            <li class="nav-collapsible" data-open={isSection(`/classes/${c.id}`)}>
              <details open={isSection(`/classes/${c.id}`)}>
                <summary class="nav-summary">
                  <i class="fa-solid fa-book nav-icon" aria-hidden="true"></i>
                  <span class="truncate">{c.name}</span>
                </summary>
                <div class="nav-group">
                  {#if $auth?.role === 'student'}
                    <a class={`nav-sublink ${isActive(`/classes/${c.id}/overview`) ? 'is-active' : ''}`} href={`/classes/${c.id}/overview`} on:click={() => sidebarOpen.set(false)}>
                      <i class="fa-regular fa-compass sub-icon" aria-hidden="true"></i>
                      <span>Overview</span>
                    </a>
                  {/if}
                  <a class={`nav-sublink ${isActive(`/classes/${c.id}/assignments`) ? 'is-active' : ''}`} href={`/classes/${c.id}/assignments`} on:click={() => sidebarOpen.set(false)}>
                    <i class="fa-solid fa-list-check sub-icon" aria-hidden="true"></i>
                    <span>Assignments</span>
                  </a>
                  <a class={`nav-sublink ${isActive(`/classes/${c.id}/files`) ? 'is-active' : ''}`} href={`/classes/${c.id}/files`} on:click={() => sidebarOpen.set(false)}>
                    <i class="fa-regular fa-folder-open sub-icon" aria-hidden="true"></i>
                    <span>Files</span>
                  </a>
                  <a class={`nav-sublink ${isActive(`/classes/${c.id}/notes`) ? 'is-active' : ''}`} href={`/classes/${c.id}/notes`} on:click={() => sidebarOpen.set(false)}>
                    <i class="fa-regular fa-note-sticky sub-icon" aria-hidden="true"></i>
                    <span>Notes</span>
                  </a>
                  <a class={`nav-sublink ${isActive(`/classes/${c.id}/forum`) ? 'is-active' : ''}`} href={`/classes/${c.id}/forum`} on:click={() => sidebarOpen.set(false)}>
                    <i class="fa-regular fa-comments sub-icon" aria-hidden="true"></i>
                    <span>Forum</span>
                  </a>
                  {#if $auth?.role !== 'student'}
                    <a class={`nav-sublink ${isActive(`/classes/${c.id}/progress`) ? 'is-active' : ''}`} href={`/classes/${c.id}/progress`} on:click={() => sidebarOpen.set(false)}>
                      <i class="fa-solid fa-chart-line sub-icon" aria-hidden="true"></i>
                      <span>Progress</span>
                    </a>
                    <a class={`nav-sublink ${isActive(`/classes/${c.id}/settings`) ? 'is-active' : ''}`} href={`/classes/${c.id}/settings`} on:click={() => sidebarOpen.set(false)}>
                      <i class="fa-solid fa-sliders sub-icon" aria-hidden="true"></i>
                      <span>Settings</span>
                    </a>
                  {/if}
                </div>
              </details>
            </li>
          {/each}
          {#if !$classesStore.classes.length && !$classesStore.loading && !$classesStore.error}
            <li class="text-sm opacity-60">No classes</li>
          {/if}
          {#if $classesStore.loading}
            <li class="text-sm opacity-60">Loading classes...</li>
          {/if}

          {#if $auth?.role === 'teacher' || $auth?.role === 'admin'}
            <li class="nav-collapsible" data-open={isSection('/teachers')}>
              <details open={isSection('/teachers')}>
                <summary class="nav-summary">
                  <i class="fa-solid fa-people-group nav-icon" aria-hidden="true"></i>
                  <span class="truncate">Teachers' Group</span>
                </summary>
                <div class="nav-group">
                  <a class={`nav-sublink ${isActive('/teachers/forum') ? 'is-active' : ''}`} href="/teachers/forum" on:click={() => sidebarOpen.set(false)}>
                    <i class="fa-regular fa-comments sub-icon" aria-hidden="true"></i>
                    <span>Forum</span>
                  </a>
                  <a class={`nav-sublink ${isActive('/teachers/files') ? 'is-active' : ''}`} href="/teachers/files" on:click={() => sidebarOpen.set(false)}>
                    <i class="fa-regular fa-folder-open sub-icon" aria-hidden="true"></i>
                    <span>Files</span>
                  </a>
                  <a class={`nav-sublink ${isActive('/teachers/assignments') ? 'is-active' : ''}`} href="/teachers/assignments" on:click={() => sidebarOpen.set(false)}>
                    <i class="fa-solid fa-list-check sub-icon" aria-hidden="true"></i>
                    <span>Assignments</span>
                  </a>
                </div>
              </details>
            </li>
          {/if}
        </ul>

        {#if $classesStore.error}
          <p class="text-error mt-3 text-sm">{$classesStore.error}</p>
        {/if}
      </div>
    </div>
  </aside>
</div>

<style>
  .sidebar-shell {
    position: relative;
    border-radius: 1rem;
    /* Clear glass plate base */
    background: rgba(255, 255, 255, 0.58);
    border: 1px solid rgba(255, 255, 255, 0.28);
    backdrop-filter: blur(16px) saturate(160%);
    -webkit-backdrop-filter: blur(16px) saturate(160%);
    /* Stronger, layered drop shadows to increase protrusion */
    box-shadow:
      0 10px 10px rgba(0, 0, 0, 0.10),
      0 10px 10px rgba(0, 0, 0, 0.14),
      inset 0 1px 0 rgba(255, 255, 255, 0.521);
    /* Extra perceived depth via drop-shadow filter */
    filter: drop-shadow(0 1px 10px rgba(0,0,0,0.10)) drop-shadow(0 28px 40px rgba(0,0,0,0.10));
    padding: 0.75rem;
  }

  /* Dark theme glass plate adjustments */
  :global([data-theme='dark']) .sidebar-shell {
    background: rgba(16, 22, 28, 0.62);
    border-color: rgba(255, 255, 255, 0.08);
    box-shadow:
      0 10px 22px rgba(0, 0, 0, 0.35),
      0 34px 70px rgba(0, 0, 0, 0.45),
      inset 0 1px 0 rgba(255, 255, 255, 0.06);
    filter: drop-shadow(0 8px 16px rgba(0,0,0,0.15)) drop-shadow(0 36px 56px rgba(0,0,0,0.35));
  }

  /* Light theme: make glass extra glossy/shiny */
  :global([data-theme='light']) .sidebar-shell {
    background: rgba(255, 255, 255, 0.32);
    border-color: rgba(255, 255, 255, 0.34);
    box-shadow:
      0 12px 24px rgba(0, 0, 0, 0.02),
      0 40px 80px rgba(0, 0, 0, 0.06),
      inset 0 1px 0 rgba(255, 255, 255, 0.02),
      inset 0 18px 40px rgba(255, 255, 255, 0.01);
    filter: drop-shadow(0 10px 16px rgba(0,0,0,0.02)) drop-shadow(0 48px 64px rgba(0,0,0,0.04));
  }

  .sidebar-shell::before {
    content: "";
    position: absolute; inset: 0; border-radius: inherit; pointer-events: none;
    background:
      linear-gradient(180deg, rgba(255,255,255,0.16), rgba(255,255,255,0.06) 24%, rgba(255,255,255,0.02) 52%, transparent 72%),
      radial-gradient(120% 60% at 50% -10%, rgba(255,255,255,0.16), transparent 60%);
    mix-blend-mode: screen;
  }

  /* Light theme: stronger specular highlights and diagonal sheen */
  :global([data-theme='light']) .sidebar-shell::before {
    background:
      /* top-to-bottom glossy curve */
      linear-gradient(180deg, rgba(255,255,255,0.22), rgba(255,255,255,0.10) 26%, rgba(255,255,255,0.04) 54%, transparent 76%),
      /* diagonal sheen */
      linear-gradient(115deg, rgba(255,255,255,0.18) 0%, rgba(255,255,255,0.10) 22%, rgba(255,255,255,0.04) 38%, transparent 50%),
      /* soft crown highlight */
      radial-gradient(120% 60% at 50% -10%, rgba(255,255,255,0.20), transparent 60%),
      /* small sparkle near top-left */
      radial-gradient(40% 24% at 16% 2%, rgba(255,255,255,0.24), transparent 70%);
  }

  .sidebar-shell::after {
    content: "";
    position: absolute; inset: 0; border-radius: inherit; pointer-events: none;
    border: 1px solid rgba(0, 0, 0, 0.06);
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.04);
  }

  /* Light theme: subtle polished edge */
  :global([data-theme='light']) .sidebar-shell::after {
    border-color: rgba(0, 0, 0, 0.04);
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
  }

  .sidebar-accent {
    position: absolute;
    left: 0.4rem;
    top: 0.75rem;
    bottom: 0.75rem;
    width: 2px;
    border-radius: 2px;
    background: linear-gradient(180deg, rgba(var(--glow),0.08), rgba(var(--glow),0.28), rgba(var(--glow),0.08));
    box-shadow: 0 0 12px rgba(var(--glow), 0.15);
    pointer-events: none;
  }

  .sidebar-content { scrollbar-width: thin; padding-right: 0.25rem; }
  .sidebar-content::-webkit-scrollbar { width: 8px; }
  .sidebar-content::-webkit-scrollbar-thumb { background: color-mix(in oklab, hsl(var(--b3)) 40%, transparent); border-radius: 9999px; }

  .section-title {
    font-weight: 600;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    opacity: 0.7;
    padding: 0.5rem 0.75rem;
  }

  .nav-section {
    display: grid;
    gap: 0.25rem;
    padding: 0.25rem 0.25rem 0.25rem 0.5rem;
  }

  .nav-link {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.5rem 0.625rem;
    border-radius: 0.7rem;
    color: currentColor;
    text-decoration: none;
    position: relative;
    transition: background 150ms ease, color 150ms ease, transform 120ms ease, box-shadow 150ms ease;
  }
  /* Gate hover effects so they only trigger when the sidebar itself is hovered.
     This prevents underlying hover artifacts when elements overlay the sidebar (e.g., topbar). */
  .sidebar-content:hover .nav-link:hover { background: hsl(var(--b3) / 0.08); transform: translateX(2px); box-shadow: 0 1px 0 hsl(var(--b3) / 0.08) inset; }
  .nav-link:focus-visible { outline: none; background: hsl(var(--b3) / 0.10); transform: translateX(2px); }
  .nav-link::after {
    content: "›";
    position: absolute;
    right: 8px;
    top: 50%;
    transform: translateY(-50%) translateX(-4px);
    opacity: 0;
    transition: transform 150ms ease, opacity 150ms ease, color 150ms ease;
    font-size: 0.9rem;
    color: color-mix(in oklab, hsl(var(--b3)) 60%, transparent);
    pointer-events: none;
  }
  .sidebar-content:hover .nav-link:hover::after, .nav-link:focus-visible::after { opacity: 0.9; transform: translateY(-50%) translateX(0); color: color-mix(in oklab, hsl(var(--p)) 70%, hsl(var(--pc)) 30%); }
  .nav-link.is-active {
    background: linear-gradient(90deg, hsl(var(--p) / 0.18), transparent 70%);
    color: color-mix(in oklab, hsl(var(--p)) 85%, hsl(var(--pc)) 15%);
  }
  .nav-link.is-active::before {
    content: "";
    position: absolute;
    left: -6px;
    top: 50%;
    transform: translateY(-50%);
    width: 6px; height: 6px; border-radius: 9999px;
    background: radial-gradient(circle at 30% 30%, #00e1ff, #00b3ff 60%, transparent 61%);
    box-shadow: 0 0 10px rgba(0, 200, 255, 0.5);
  }

  .nav-icon { width: 1rem; text-align: center; opacity: 0.9; transition: transform 120ms ease, opacity 120ms ease, filter 200ms ease, color 150ms ease; }
  .sidebar-content:hover .nav-link:hover .nav-icon, .nav-link:focus-visible .nav-icon { transform: translateX(2px) scale(1.05); opacity: 1; color: color-mix(in oklab, hsl(var(--p)) 65%, currentColor 35%); filter: drop-shadow(0 0 6px rgba(var(--glow), 0.15)); }

  .nav-collapsible details { border-radius: 0.8rem; }
  .nav-summary {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.55rem 0.625rem;
    margin-left: 0.25rem;
    border-radius: 0.7rem;
    cursor: pointer;
    transition: background 150ms ease, transform 120ms ease, color 150ms ease;
    list-style: none;
  }
  .sidebar-content:hover .nav-summary:hover { background: hsl(var(--b3) / 0.06); transform: translateX(2px); }
  .nav-summary:focus-visible { outline: none; background: hsl(var(--b3) / 0.10); transform: translateX(2px); }
  .sidebar-content:hover .nav-summary:hover .nav-icon, .nav-summary:focus-visible .nav-icon { transform: translateX(2px) scale(1.04); opacity: 1; color: color-mix(in oklab, hsl(var(--p)) 60%, currentColor 40%); }
  .nav-collapsible[data-open="true"] .nav-summary { background: hsl(var(--b3) / 0.08); }

  .nav-group { display: grid; gap: 0.15rem; padding: 0.25rem 0.25rem 0.25rem 1.7rem; }

  .nav-sublink {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    padding: 0.45rem 0.55rem;
    border-radius: 0.6rem;
    color: currentColor;
    text-decoration: none;
    position: relative;
    transition: background 150ms ease, color 150ms ease, transform 120ms ease, box-shadow 150ms ease;
  }
  .sidebar-content:hover .nav-sublink:hover { background: hsl(var(--b3) / 0.06); transform: translateX(2px); box-shadow: 0 1px 0 hsl(var(--b3) / 0.06) inset; }
  .nav-sublink:focus-visible { outline: none; background: hsl(var(--b3) / 0.10); transform: translateX(2px); }
  .nav-sublink::after {
    content: "›";
    position: absolute;
    right: 6px;
    top: 50%;
    transform: translateY(-50%) translateX(-4px);
    opacity: 0;
    transition: transform 150ms ease, opacity 150ms ease, color 150ms ease;
    font-size: 0.85rem;
    color: color-mix(in oklab, hsl(var(--b3)) 50%, transparent);
    pointer-events: none;
  }
  .sidebar-content:hover .nav-sublink:hover::after, .nav-sublink:focus-visible::after { opacity: 0.85; transform: translateY(-50%) translateX(0); color: color-mix(in oklab, hsl(var(--p)) 70%, hsl(var(--pc)) 30%); }
  .nav-sublink.is-active {
    background: linear-gradient(90deg, hsl(var(--p) / 0.16), transparent 65%);
    color: color-mix(in oklab, hsl(var(--p)) 85%, hsl(var(--pc)) 15%);
  }
  .sub-icon { width: 0.9rem; text-align: center; opacity: 0.9; }
</style>

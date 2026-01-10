<script lang="ts">
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import { fade, fly, scale } from 'svelte/transition';
  import { User, Settings, LogOut, ChevronDown } from 'lucide-svelte';
  
  export let user: any;
  export let translate: (key: string) => string;

  const dispatch = createEventDispatcher();
  let isOpen = false;
  let container: HTMLDivElement;
  let dropdownStyle = "";
  let computedPlacement: 'top' | 'bottom' = 'bottom';

  function toggle() {
    isOpen = !isOpen;
  }

  function handleClickOutside(event: MouseEvent) {
    if (container && !container.contains(event.target as Node)) {
      isOpen = false;
    }
  }

  function updatePosition() {
    if (!container || !isOpen) return;
    const rect = container.getBoundingClientRect();
    
    const spaceBelow = window.innerHeight - rect.bottom;
    const spaceAbove = rect.top;
    
    computedPlacement = spaceBelow < 250 && spaceAbove > spaceBelow ? 'top' : 'bottom';

    let leftPos = rect.right - 200; // Default width 200, align to right
    if (leftPos < 10) leftPos = 10;
    if (leftPos + 200 > window.innerWidth - 10) {
        leftPos = window.innerWidth - 210;
    }

    const baseStyle = `
      position: fixed;
      left: ${leftPos}px;
      width: 200px;
      z-index: 9999;
    `;

    if (computedPlacement === 'bottom') {
      dropdownStyle = `
        ${baseStyle}
        top: ${rect.bottom + 8}px;
      `;
    } else {
      dropdownStyle = `
        ${baseStyle}
        bottom: ${window.innerHeight - rect.top + 8}px;
      `;
    }
  }

  $: if (isOpen) {
    tick().then(updatePosition);
  }

  onMount(() => {
    const handleScroll = () => {
      if (isOpen) updatePosition();
    };
    window.addEventListener('click', handleClickOutside);
    window.addEventListener('scroll', handleScroll, true);
    window.addEventListener('resize', handleScroll);
    
    return () => {
      window.removeEventListener('click', handleClickOutside);
      window.removeEventListener('scroll', handleScroll, true);
      window.removeEventListener('resize', handleScroll);
    };
  });

  function portal(node: HTMLElement) {
    const dialog = container?.closest('dialog');
    const target = dialog || document.body;
    target.appendChild(node);
    updatePosition();
    return {
      destroy() {
        if (node.parentNode) node.parentNode.removeChild(node);
      }
    };
  }

  function handleAction(action: 'settings' | 'logout') {
    isOpen = false;
    dispatch(action);
  }
</script>

<div class="user-dropdown-container" bind:this={container}>
  <button
    type="button"
    class="avatar-trigger group flex items-center gap-2 p-1 rounded-full transition-all duration-300
      {isOpen ? 'bg-primary/10 ring-4 ring-primary/5' : 'hover:bg-base-200'}"
    on:click={toggle}
    aria-haspopup="menu"
    aria-expanded={isOpen}
  >
    <div class="relative">
      {#if user.avatar}
        <div class="w-10 h-10 rounded-full overflow-hidden ring-2 ring-base-100 shadow-sm transition-transform group-hover:scale-105">
          <img
            src={user.avatar}
            alt={translate("frontend/src/routes/+layout.svelte::user_avatar_alt")}
            class="w-full h-full object-cover"
          />
        </div>
      {:else}
        <div
          class="w-10 h-10 rounded-full bg-primary/10 text-primary ring-2 ring-base-100 shadow-sm flex items-center justify-center font-bold transition-transform group-hover:scale-105"
        >
          {user.role.slice(0, 1).toUpperCase()}
        </div>
      {/if}
      <div class="absolute -bottom-0.5 -right-0.5 w-3.5 h-3.5 bg-success rounded-full border-2 border-base-100"></div>
    </div>
    
    <div class="hidden md:flex flex-col items-start pr-2">
        <span class="text-sm font-bold text-base-content leading-none">{user.name || translate("frontend/src/routes/+layout.svelte::user_fallback_name")}</span>
        <span class="text-[10px] font-medium text-base-content/50 uppercase tracking-wider">
            {translate(`frontend/src/routes/+layout.svelte::role_${user.role}`)}
        </span>
    </div>

    <ChevronDown 
      size={16} 
      class="transition-transform duration-300 text-base-content/30 group-hover:text-primary {isOpen ? 'rotate-180 text-primary' : ''}" 
    />
  </button>

  {#if isOpen}
    <div
      use:portal
      style={dropdownStyle}
      class="dropdown-menu overflow-hidden bg-base-100 border border-primary/20 rounded-[1.5rem] shadow-2xl p-1.5"
      in:fly={{ y: computedPlacement === 'top' ? 10 : -10, duration: 300, opacity: 0 }}
      out:fade={{ duration: 150 }}
    >
      <div class="flex flex-col gap-1">
        <div class="px-3 py-2 mb-1 border-b border-base-200/50">
            <p class="text-xs font-semibold text-base-content/40 uppercase tracking-widest">{translate("frontend/src/lib/components/UserProfileModal.svelte::profile_title")}</p>
        </div>

        <button
          type="button"
          class="group flex items-center gap-3 w-full px-3 py-2.5 rounded-xl transition-all duration-200 text-left hover:bg-primary/10 hover:text-primary"
          on:click={() => handleAction('settings')}
        >
          <div class="shrink-0 w-8 h-8 rounded-lg bg-base-200 flex items-center justify-center group-hover:bg-primary/20 transition-colors text-base-content/60 group-hover:text-primary">
            <Settings size={18} />
          </div>
          <span class="font-semibold text-sm">
            {translate("frontend/src/routes/+layout.svelte::settings_button")}
          </span>
        </button>

        <button
          type="button"
          class="group flex items-center gap-3 w-full px-3 py-2.5 rounded-xl transition-all duration-200 text-left hover:bg-error/10 hover:text-error"
          on:click={() => handleAction('logout')}
        >
          <div class="shrink-0 w-8 h-8 rounded-lg bg-base-200 flex items-center justify-center group-hover:bg-error/20 transition-colors text-base-content/60 group-hover:text-error">
            <LogOut size={18} />
          </div>
          <span class="font-semibold text-sm">
            {translate("frontend/src/routes/+layout.svelte::logout_button")}
          </span>
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .user-dropdown-container {
    font-family: 'Inter', sans-serif;
  }
  .avatar-trigger {
    cursor: pointer;
    border: none;
    outline: none;
  }
</style>

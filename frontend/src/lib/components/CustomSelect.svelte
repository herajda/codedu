<script lang="ts">
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import { fade, fly, scale } from 'svelte/transition';
  import { ChevronDown, Check, Search } from 'lucide-svelte';
  import { translator } from '$lib/i18n';

  export let options: { value: any; label: string; icon?: string; flag?: string }[] = [];
  export let value: any;
  export let placeholder = 'Select an option';
  export let label: string = '';
  export let disabled = false;
  export let searchable = false;
  export let small = false;
  export let icon: any = null; // Lucide icon component
  export let placement: 'auto' | 'top' | 'bottom' = 'auto';

  $: t = $translator;

  const dispatch = createEventDispatcher();
  let isOpen = false;
  let searchTerm = '';
  let container: HTMLDivElement;
  let computedPlacement: 'top' | 'bottom' = 'bottom';

  function updatePlacement() {
    if (placement !== 'auto') {
      computedPlacement = placement;
      return;
    }
    if (!container) return;
    const rect = container.getBoundingClientRect();
    const spaceBelow = window.innerHeight - rect.bottom;
    computedPlacement = spaceBelow < 250 ? 'top' : 'bottom';
  }

  $: if (isOpen) updatePlacement();

  $: filteredOptions = searchable 
    ? options.filter(o => o.label.toLowerCase().includes(searchTerm.toLowerCase()))
    : options;

  $: selectedOption = options.find(o => o.value === value);

  function toggle() {
    if (!disabled) isOpen = !isOpen;
  }

  function select(option: any) {
    value = option.value;
    isOpen = false;
    dispatch('change', value);
  }

  function handleClickOutside(event: MouseEvent) {
    if (container && !container.contains(event.target as Node)) {
      isOpen = false;
    }
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

  let dropdownStyle = "";

  function updatePosition() {
    if (!container || !isOpen) return;
    const rect = container.getBoundingClientRect();
    
    // Position the dropdown relative to the trigger
    // We want it to be above the dropdown container
    const triggerHeight = rect.height;
    const triggerWidth = rect.width;
    const triggerTop = rect.top;
    const triggerLeft = rect.left;

    // Use current placement logic to determine if we show on top or bottom
    const spaceBelow = window.innerHeight - rect.bottom;
    const spaceAbove = rect.top;
    
    if (placement === 'auto') {
      computedPlacement = spaceBelow < 250 && spaceAbove > spaceBelow ? 'top' : 'bottom';
    } else {
      computedPlacement = placement;
    }

    // Ensure dropdown doesn't overflow right edge of screen
    let leftPos = triggerLeft;
    const estimatedWidth = Math.max(triggerWidth, 200); // 200 is a safe min-width for dropdowns
    if (leftPos + estimatedWidth > window.innerWidth - 10) {
      leftPos = window.innerWidth - estimatedWidth - 10;
    }
    if (leftPos < 10) leftPos = 10;

    const baseStyle = `
      position: fixed;
      left: ${leftPos}px;
      min-width: ${triggerWidth}px;
      width: max-content;
      max-width: calc(100vw - 40px);
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
    // Wait for Svelte to update the DOM before measuring
    tick().then(updatePosition);
  }
</script>

<div class="custom-select-container flex flex-col gap-1.5" bind:this={container}>
  {#if label}
    <span class="label-text font-semibold text-xs uppercase tracking-wider text-base-content/50 ml-1">
      {label}
    </span>
  {/if}

  <div class="relative w-full {isOpen ? 'z-[100]' : 'z-0'}">
    <button
      type="button"
      class="select-trigger w-full flex items-center justify-between gap-3 {small ? 'px-3 py-2 text-sm rounded-xl' : 'px-4 py-3 rounded-2xl'} border transition-all duration-300 text-left cursor-pointer
        {isOpen ? 'border-primary ring-4 ring-primary/10 bg-base-100 shadow-lg' : 'border-base-300/60 bg-base-100 hover:bg-base-100/80 hover:border-primary/40'}
        {disabled ? 'opacity-50 cursor-not-allowed grayscale' : ''}"
      on:click={toggle}
      {disabled}
      aria-haspopup="listbox"
      aria-expanded={isOpen}
    >
      <div class="flex items-center gap-2.5 overflow-hidden">
        {#if icon}
          <div class="text-primary/60 shrink-0">
            <svelte:component this={icon} size={small ? 16 : 18} />
          </div>
        {/if}
        
        <div class="flex items-center gap-1.5 overflow-hidden">
          {#if selectedOption?.flag}
            <span class="{small ? 'text-lg' : 'text-xl'} leading-none shrink-0">{selectedOption.flag}</span>
          {/if}
          <span class="truncate font-medium {selectedOption ? 'text-base-content' : 'text-base-content/40'}">
            {selectedOption ? selectedOption.label : placeholder}
          </span>
        </div>
      </div>

      <ChevronDown 
        size={small ? 16 : 18} 
        class="transition-transform duration-300 shrink-0 {isOpen ? 'rotate-180 text-primary' : 'text-base-content/30'}" 
      />
    </button>

    {#if isOpen}
      <div
        use:portal
        style={dropdownStyle}
        class="w-full {computedPlacement === 'top' ? 'origin-bottom' : 'origin-top'} overflow-hidden bg-base-100 border border-primary/20 rounded-[1.5rem] shadow-2xl p-1.5"
        in:fly={{ y: computedPlacement === 'top' ? 10 : -10, duration: 300, opacity: 0 }}
        out:fade={{ duration: 150 }}
      >
        {#if searchable}
          <div class="relative mb-1.5 px-1.5 pt-1.5">
            <div class="absolute inset-y-0 left-4 flex items-center pointer-events-none text-base-content/40">
              <Search size={small ? 12 : 14} />
            </div>
            <input
              type="text"
              class="w-full bg-base-200/50 border-none rounded-xl {small ? 'py-1.5 pl-8 pr-3 text-xs' : 'py-2 pl-9 pr-4 text-sm'} focus:ring-2 focus:ring-primary/20 focus:bg-base-200 transition-all outline-none"
              placeholder={t("frontend/src/lib/components/CustomSelect.svelte::search_placeholder")}
              bind:value={searchTerm}
              on:click|stopPropagation
            />
          </div>
        {/if}

        <div class="max-h-60 overflow-y-auto custom-scrollbar flex flex-col gap-0.5 p-0.5">
          {#each filteredOptions as option}
            <button
              type="button"
              class="group flex items-center justify-between gap-4 w-full {small ? 'px-3 py-2 text-xs rounded-lg' : 'px-4 py-3 rounded-xl'} transition-all duration-200 text-left
                {option.value === value ? 'bg-primary text-primary-content shadow-md shadow-primary/20' : 'hover:bg-primary/10 text-base-content/80 hover:text-primary'}
                {option.disabled ? 'opacity-50 cursor-not-allowed pointer-events-none grayscale' : ''}"
              on:click={() => !option.disabled && select(option)}
            >
              <div class="flex items-center gap-2.5">
                {#if option.flag}
                  <span class="{small ? 'text-lg' : 'text-xl'} shrink-0">{option.flag}</span>
                {/if}
                <span class="font-medium whitespace-nowrap">{option.label}</span>
              </div>
              
              {#if option.value === value}
                <div in:scale>
                  <Check size={small ? 14 : 16} />
                </div>
              {/if}
            </button>
          {:else}
            <div class="px-4 py-8 text-center text-sm text-base-content/40 italic">
              {t("frontend/src/lib/components/CustomSelect.svelte::no_options")}
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .custom-scrollbar::-webkit-scrollbar {
    width: 4px;
  }
  .custom-scrollbar::-webkit-scrollbar-track {
    background: transparent;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb {
    background: rgba(var(--p), 0.1);
    border-radius: 10px;
  }
  .custom-select-container {
    font-family: 'Inter', sans-serif;
  }
</style>

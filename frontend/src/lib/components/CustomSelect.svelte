<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
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
    window.addEventListener('click', handleClickOutside);
    return () => window.removeEventListener('click', handleClickOutside);
  });
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
      class="select-trigger w-full flex items-center justify-between gap-3 {small ? 'px-3 py-1.5 text-sm rounded-xl' : 'px-4 py-3 rounded-2xl'} border transition-all duration-300 text-left cursor-pointer
        {isOpen ? 'border-primary ring-4 ring-primary/10 bg-base-100 shadow-lg' : 'border-base-300/60 bg-base-50/50 hover:bg-base-100/80 hover:border-primary/40'}
        {disabled ? 'opacity-50 cursor-not-allowed grayscale' : ''}"
      on:click={toggle}
      {disabled}
      aria-haspopup="listbox"
      aria-expanded={isOpen}
    >
      <div class="flex items-center gap-3 overflow-hidden">
        {#if icon}
          <div class="text-primary/60 shrink-0">
            <svelte:component this={icon} size={18} />
          </div>
        {/if}
        
        <div class="flex items-center gap-2 overflow-hidden">
          {#if selectedOption?.flag}
            <span class="text-xl leading-none shrink-0">{selectedOption.flag}</span>
          {/if}
          <span class="truncate font-medium {selectedOption ? 'text-base-content' : 'text-base-content/40'}">
            {selectedOption ? selectedOption.label : placeholder}
          </span>
        </div>
      </div>

      <ChevronDown 
        size={18} 
        class="transition-transform duration-300 shrink-0 {isOpen ? 'rotate-180 text-primary' : 'text-base-content/30'}" 
      />
    </button>

    {#if isOpen}
      <div
        class="absolute z-[1000] w-full {computedPlacement === 'top' ? 'bottom-full mb-2 origin-bottom' : 'top-full mt-2 origin-top'} overflow-hidden bg-base-100 border border-primary/20 rounded-[2rem] shadow-2xl p-2"
        in:fly={{ y: computedPlacement === 'top' ? 10 : -10, duration: 300, opacity: 0 }}
        out:fade={{ duration: 150 }}
      >
        {#if searchable}
          <div class="relative mb-2 px-2 pt-2">
            <div class="absolute inset-y-0 left-5 flex items-center pointer-events-none text-base-content/40">
              <Search size={14} />
            </div>
            <input
              type="text"
              class="w-full bg-base-200/50 border-none rounded-xl py-2 pl-9 pr-4 text-sm focus:ring-2 focus:ring-primary/20 focus:bg-base-200 transition-all outline-none"
              placeholder={t("frontend/src/lib/components/CustomSelect.svelte::search_placeholder")}
              bind:value={searchTerm}
              on:click|stopPropagation
            />
          </div>
        {/if}

        <div class="max-h-60 overflow-y-auto custom-scrollbar flex flex-col gap-1 p-1">
          {#each filteredOptions as option}
            <button
              type="button"
              class="group flex items-center justify-between w-full px-4 py-3 rounded-xl transition-all duration-200 text-left
                {option.value === value ? 'bg-primary text-primary-content shadow-md shadow-primary/20' : 'hover:bg-primary/10 text-base-content/80 hover:text-primary'}"
              on:click={() => select(option)}
            >
              <div class="flex items-center gap-3 overflow-hidden">
                {#if option.flag}
                  <span class="text-xl shrink-0">{option.flag}</span>
                {/if}
                <span class="truncate font-medium">{option.label}</span>
              </div>
              
              {#if option.value === value}
                <div in:scale>
                  <Check size={16} />
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

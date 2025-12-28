<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { fade } from 'svelte/transition';

  export let value: string = "";
  export let placeholder: string = "";
  export let label: string = "";
  export let icon: any = null; // Lucide icon component
  export let type: string = "text";
  export let disabled: boolean = false;
  export let required: boolean = false;
  export let error: string = "";

  const dispatch = createEventDispatcher();
  let focused = false;

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    value = target.value;
    dispatch('input', value);
  }

  function handleBlur() {
    focused = false;
    dispatch('blur');
  }

  function handleFocus() {
    focused = true;
    dispatch('focus');
  }
</script>

<div class="stylish-input-container flex flex-col gap-1.5 w-full">
  {#if label}
    <span class="label-text font-semibold text-xs uppercase tracking-wider text-base-content/50 ml-1">
      {label}
    </span>
  {/if}

  <div class="relative w-full group">
    <div
      class="input-wrapper w-full flex items-center gap-3 px-4 py-3 rounded-2xl border transition-all duration-300
        {focused ? 'border-primary ring-4 ring-primary/10 bg-base-100 shadow-lg' : 'border-base-300/60 bg-base-50/50 group-hover:bg-base-100/80 group-hover:border-primary/40'}
        {error ? 'border-error ring-error/10' : ''}
        {disabled ? 'opacity-50 cursor-not-allowed grayscale' : ''}"
    >
      {#if icon}
        <div class="shrink-0 transition-colors duration-300 {focused ? 'text-primary' : 'text-base-content/40'}">
          <svelte:component this={icon} size={18} />
        </div>
      {/if}

      <input
        {type}
        {value}
        {placeholder}
        {disabled}
        {required}
        class="w-full bg-transparent border-none outline-none font-medium text-base-content placeholder:text-base-content/30"
        on:input={handleInput}
        on:focus={handleFocus}
        on:blur={handleBlur}
      />

      {#if value && !disabled}
        <button
          type="button"
          class="shrink-0 text-base-content/20 hover:text-error transition-colors duration-200"
          on:click={() => { value = ""; dispatch('input', value); }}
          transition:fade={{ duration: 150 }}
        >
          <i class="fa-solid fa-circle-xmark"></i>
        </button>
      {/if}
    </div>

    {#if error}
      <p class="text-[10px] text-error font-medium mt-1 ml-1 uppercase tracking-wider" in:fade>
        {error}
      </p>
    {/if}
  </div>
</div>

<style>
  .stylish-input-container {
    font-family: 'Inter', sans-serif;
  }
</style>

<script lang="ts">
  import DeadlinePicker from '$lib/components/DeadlinePicker.svelte';
  let picker: DeadlinePicker;
  let result = '';

  async function openPicker() {
    result = await picker.open() ?? 'cancelled';
  }
</script>

<div class="p-10 flex flex-col items-center gap-5">
  <h1 class="text-2xl font-bold">DeadlinePicker Test Page</h1>
  <button class="btn btn-primary" on:click={openPicker}>Open Picker</button>
  <p>Result: {result}</p>
  <DeadlinePicker bind:this={picker} />
</div>

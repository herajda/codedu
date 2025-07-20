<!-- src/lib/components/OutputBlock.svelte -->
<script lang="ts">
  export let label: 'stdout' | 'stderr' | 'result';
  export let text = '';
  const colors = {
    stdout: 'emerald',
    stderr: 'rose',
    result: 'slate'
  }[label];
</script>

{#if text}
  <pre class={`my-1 rounded-md bg-${colors}-50 text-${colors}-800 border-l-4 border-${colors}-400 font-mono whitespace-pre-wrap p-1 px-2`}>{text}</pre>
{/if}

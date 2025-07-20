<!-- src/lib/components/OutputBlock.svelte -->
<script lang="ts">
  export let label: 'stdout' | 'stderr' | 'result';
  export let text = '';
  const stdout = label === 'stdout';
  const stderr = label === 'stderr';
  const result = label === 'result';
</script>

{#if text}
  <pre
    class="my-1 rounded-md font-mono whitespace-pre-wrap p-1 px-2 border-l-4"
    class:bg-emerald-50={stdout}
    class:text-emerald-800={stdout}
    class:border-emerald-400={stdout}
    class:bg-rose-50={stderr}
    class:text-rose-800={stderr}
    class:border-rose-400={stderr}
    class:bg-slate-50={result}
    class:text-slate-800={result}
    class:border-slate-400={result}
  >{text}</pre>
{/if}

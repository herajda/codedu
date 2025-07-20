<script lang="ts">
  export let src: string;
</script>

{#if src}
  <img class="my-1" src={`data:image/png;base64,${src}`} alt="figure" />
{/if}

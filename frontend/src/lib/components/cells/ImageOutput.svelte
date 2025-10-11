<script lang="ts">
  import { t } from '$lib/i18n';

  export let src: string;
</script>

{#if src}
  <img class="my-1" src={`data:image/png;base64,${src}`} alt={t('frontend/src/lib/components/cells/ImageOutput.svelte::figure')} />
{/if}

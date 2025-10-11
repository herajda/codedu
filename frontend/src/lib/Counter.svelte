
<script lang="ts">
  import { translator } from '$lib/i18n'

  let count: number = $state(0)
  const increment = () => {
    count += 1
  }

  let translate;
  $: translate = $translator;
</script>

<button onclick={increment}>
  {translate('frontend/src/lib/Counter.svelte::count_is', { count })}
</button>

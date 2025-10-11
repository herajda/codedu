<script lang="ts">
  import { goto } from '$app/navigation';
  import { t } from '$lib/i18n';

  // Redirect to default tab
  goto('/teachers/forum');
</script>

<p>{t('frontend/src/routes/teachers/+page.svelte::redirecting')}</p>
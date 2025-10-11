<script lang="ts">
  import { auth } from '$lib/auth';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { t } from '$lib/i18n';

  $: role = $auth?.role ?? '';
  $: if (role) {
    const id = $page.params.id;
    const target = role === 'student'
      ? `/classes/${id}/overview`
      : `/classes/${id}/assignments`;
    goto(target);
  }
</script>

<p>{t('frontend/src/routes/classes/[id]/+page.svelte::redirecting')}</p>

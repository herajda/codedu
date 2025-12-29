<script lang="ts">
  import { page } from '$app/stores';
  import { auth } from '$lib/auth';
  import { translator } from '$lib/i18n';

  $: active = $page.url.pathname;
  $: isTeacher = $auth?.role === 'teacher' || $auth?.role === 'admin';

  let translate;
  $: translate = $translator;
</script>

{#if isTeacher}
  <div class="flex min-h-full">
    <div class="flex-1 p-3 sm:p-4">

      <slot />
    </div>
  </div>
{:else}
  <p>{translate('frontend/src/routes/teachers/+layout.svelte::forbidden_message')}</p>
{/if}

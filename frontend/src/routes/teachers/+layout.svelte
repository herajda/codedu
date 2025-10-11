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
      <div class="mb-4 border-b border-base-300/50">
        <div class="tabs tabs-boxed bg-base-200">
          <a href="/teachers/forum" class={`tab ${active.startsWith('/teachers/forum') ? 'tab-active' : ''}`}>{translate('frontend/src/routes/teachers/+layout.svelte::forum_tab_label')}</a>
          <a href="/teachers/files" class={`tab ${active.startsWith('/teachers/files') ? 'tab-active' : ''}`}>{translate('frontend/src/routes/teachers/+layout.svelte::files_tab_label')}</a>
          <a href="/teachers/assignments" class={`tab ${active.startsWith('/teachers/assignments') ? 'tab-active' : ''}`}>{translate('frontend/src/routes/teachers/+layout.svelte::assignments_tab_label')}</a>
        </div>
      </div>
      <slot />
    </div>
  </div>
{:else}
  <p>{translate('frontend/src/routes/teachers/+layout.svelte::forbidden_message')}</p>
{/if}

<script lang="ts">
  import { page } from '$app/stores';
  import { auth } from '$lib/auth';

  $: active = $page.url.pathname;
  $: isTeacher = $auth?.role === 'teacher' || $auth?.role === 'admin';
</script>

{#if isTeacher}
  <div class="flex min-h-full">
    <div class="flex-1 p-3 sm:p-4">
      <div class="mb-4 border-b border-base-300/50">
        <div class="tabs tabs-boxed bg-base-200">
          <a href="/teachers/forum" class={`tab ${active.startsWith('/teachers/forum') ? 'tab-active' : ''}`}>Forum</a>
          <a href="/teachers/files" class={`tab ${active.startsWith('/teachers/files') ? 'tab-active' : ''}`}>Files</a>
          <a href="/teachers/assignments" class={`tab ${active.startsWith('/teachers/assignments') ? 'tab-active' : ''}`}>Assignments</a>
        </div>
      </div>
      <slot />
    </div>
  </div>
{:else}
  <p>Forbidden</p>
{/if}

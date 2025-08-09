<script lang="ts">
  import { auth } from '$lib/auth';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';

  $: role = $auth?.role ?? '';
  $: if (role) {
    const id = $page.params.id;
    const target = role === 'student'
      ? `/classes/${id}/overview`
      : `/classes/${id}/assignments`;
    goto(target);
  }
</script>

<p>Redirecting...</p>

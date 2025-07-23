<script lang="ts">
  import { sveltify } from 'svelte-preprocess-react';
  import { MessagingComponent } from '$lib/react/messaging';

  // Wrap the React component using svelte-preprocess-react
  const react = sveltify({ MessagingComponent });
</script>

<react.MessagingComponent />

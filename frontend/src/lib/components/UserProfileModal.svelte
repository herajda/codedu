<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { X } from 'lucide-svelte';

  export let userId: number;
  const dispatch = createEventDispatcher();
  let user: any = null;

  onMount(async () => {
    try {
      user = await apiJSON(`/api/users/${userId}`);
    } catch (e) {
      console.error('Failed to load profile', e);
    }
  });

  function close() {
    dispatch('close');
  }
</script>

<div class="modal modal-open">
  <div class="modal-box w-full max-w-md">
    <div class="flex items-center justify-between mb-4">
      <h3 class="font-bold text-lg">Profile</h3>
      <button class="btn btn-ghost btn-sm btn-square" on:click={close}>
        <X class="w-4 h-4" />
      </button>
    </div>

    {#if user}
      <div class="flex flex-col items-center gap-4">
        <div class="avatar">
          <div class="w-24 h-24 rounded-full overflow-hidden ring-2 ring-base-300/60">
            {#if user.avatar}
              <img src={user.avatar} alt="Avatar" class="w-full h-full object-cover" />
            {:else}
              <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-3xl font-semibold text-primary">
                {(user.name ?? user.email ?? '?').charAt(0).toUpperCase()}
              </div>
            {/if}
          </div>
        </div>
        <h4 class="text-xl font-semibold">{user.name ?? user.email}</h4>
        {#if user.email}
          <p class="text-base-content/60">{user.email}</p>
        {/if}
      </div>
    {:else}
      <div class="flex justify-center p-4">Loading...</div>
    {/if}

    <div class="modal-action">
      <button class="btn" on:click={close}>Close</button>
    </div>
  </div>
  <form method="dialog" class="modal-backdrop" on:click={close}>
    <button>close</button>
  </form>
</div>

<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { goto } from '$app/navigation';
  import { getKey, decryptText } from '$lib/e2ee';

  let convos: any[] = [];

  onMount(() => { loadConvos(); });

  async function loadConvos() {
    const list = await apiJSON('/api/messages');
    const k = getKey();
    for (const c of list) {
      if (c.content === '') {
        c.text = '';
      } else if (k) {
        try { c.text = await decryptText(k, c.content); }
        catch { c.text = '[decrypt error]'; }
      } else {
        c.text = '[locked]';
      }
    }
    convos = list;
  }

  function openChat(u: any) {
    const p = new URLSearchParams();
    if (u.name) p.set('name', u.name);
    else if (u.email) p.set('email', u.email);
    const id = u.other_id ?? u.id;
    goto(`/messages/${id}?${p.toString()}`);
  }
</script>

<h1 class="text-2xl font-bold mb-4">Messages</h1>
<div class="mb-4">
  <input
    class="input input-bordered w-full sm:max-w-xs"
    placeholder="Search users"
    on:input={(e) => goto(`/messages/search?q=${encodeURIComponent((e.target as HTMLInputElement).value)}`)}
  />
</div>

<div class="space-y-2 mt-4">
  {#each convos as c}
    <div class="flex items-center gap-3 p-2 rounded-lg hover:bg-base-200 cursor-pointer" on:click={() => openChat(c)}>
      <div class="avatar">
        <div class="w-12 h-12 rounded-full overflow-hidden">
          {#if c.avatar}
            <img src={c.avatar} alt="Avatar" class="w-full h-full object-cover" />
          {:else}
            <img src="/placeholder.svg?height=48&width=48" alt="Avatar" />
          {/if}
        </div>
      </div>
      <div class="flex-1">
        <div class="font-semibold">{c.name ?? c.email}</div>
        <div class={`text-sm opacity-70 truncate ${c.unread_count>0 ? 'font-bold' : ''}`}>{c.text || (c.image ? '[image]' : '')}</div>
      </div>
      {#if c.unread_count > 0}
        <span class="badge badge-primary badge-sm ml-2">{c.unread_count}</span>
      {/if}
      <div class="text-xs opacity-60 whitespace-nowrap ml-auto">{new Date(c.created_at).toLocaleString()}</div>
    </div>
  {/each}
</div>

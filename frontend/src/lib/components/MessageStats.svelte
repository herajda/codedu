<script lang="ts">
  import { MessageCircle, Clock, Users, TrendingUp } from 'lucide-svelte';
  import { translator } from '$lib/i18n';

  export let conversations: any[] = [];
  export let totalMessages = 0;
  export let unreadCount = 0;

  let translate;
  $: translate = $translator;

  $: activeChats = conversations.filter(c => !c.archived).length;
  $: archivedChats = conversations.filter(c => c.archived).length;
  $: todayMessages = conversations.filter(c => {
    const today = new Date();
    const messageDate = new Date(c.created_at);
    return messageDate.toDateString() === today.toDateString();
  }).length;
  $: thisWeekMessages = conversations.filter(c => {
    const today = new Date();
    const weekAgo = new Date();
    weekAgo.setDate(today.getDate() - 7);
    const messageDate = new Date(c.created_at);
    return messageDate >= weekAgo;
  }).length;

  const stats = [
    {
      labelKey: 'frontend/src/lib/components/MessageStats.svelte::active-chats',
      value: activeChats,
      icon: Users,
      color: 'text-blue-500',
      bgColor: 'bg-blue-500/10'
    },
    {
      labelKey: 'frontend/src/lib/components/MessageStats.svelte::unread-messages',
      value: unreadCount,
      icon: MessageCircle,
      color: 'text-orange-500',
      bgColor: 'bg-orange-500/10'
    },
    {
      labelKey: 'frontend/src/lib/components/MessageStats.svelte::today',
      value: todayMessages,
      icon: Clock,
      color: 'text-green-500',
      bgColor: 'bg-green-500/10'
    },
    {
      labelKey: 'frontend/src/lib/components/MessageStats.svelte::this-week',
      value: thisWeekMessages,
      icon: TrendingUp,
      color: 'text-purple-500',
      bgColor: 'bg-purple-500/10'
    }
  ];
</script>

<div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
  {#each stats as stat}
    <div class="card-elevated p-4">
      <div class="flex items-center gap-3">
        <div class="p-2 rounded-lg {stat.bgColor}">
          <svelte:component this={stat.icon} class="w-5 h-5 {stat.color}" />
        </div>
        <div>
          <p class="text-2xl font-bold">{stat.value}</p>
          <p class="text-sm text-base-content/60">{translate(stat.labelKey)}</p>
        </div>
      </div>
    </div>
  {/each}
</div>

<script lang="ts">
  import { MessageCircle, Clock, Users, TrendingUp } from 'lucide-svelte';

  export let conversations: any[] = [];
  export let totalMessages = 0;
  export let unreadCount = 0;

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
      label: 'Active Chats',
      value: activeChats,
      icon: Users,
      color: 'text-blue-500',
      bgColor: 'bg-blue-500/10'
    },
    {
      label: 'Unread Messages',
      value: unreadCount,
      icon: MessageCircle,
      color: 'text-orange-500',
      bgColor: 'bg-orange-500/10'
    },
    {
      label: 'Today',
      value: todayMessages,
      icon: Clock,
      color: 'text-green-500',
      bgColor: 'bg-green-500/10'
    },
    {
      label: 'This Week',
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
          <p class="text-sm text-base-content/60">{stat.label}</p>
        </div>
      </div>
    </div>
  {/each}
</div>

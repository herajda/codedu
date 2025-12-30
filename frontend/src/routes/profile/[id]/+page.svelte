<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { apiJSON } from '$lib/api';
  import { auth } from '$lib/auth';
  import { t, translator } from '$lib/i18n';
  import { 
    ChevronLeft, 
    MessageSquare, 
    Mail, 
    User as UserIcon, 
    BookOpen, 
    GraduationCap,
    ArrowRight
  } from 'lucide-svelte';
  import { fade } from 'svelte/transition';

  let userId = $page.params.id;
  let user: any = null;
  let loading = true;
  let error = '';

  $: translate = $translator;

  onMount(async () => {
    try {
      user = await apiJSON(`/api/users/${userId}`);
    } catch (e) {
      console.error('Failed to load profile', e);
      error = 'Failed to load profile';
    } finally {
      loading = false;
    }
  });

  function back() {
    window.history.back();
  }

  function messageUser() {
    goto(`/messages/${userId}?name=${encodeURIComponent(user.name || user.email)}`);
  }
</script>

<svelte:head>
  <title>{user ? `${user.name || user.email} | CodEdu` : 'Profile | CodEdu'}</title>
</svelte:head>

<div class="profile-page min-h-[calc(100vh-4rem)] bg-base-200/30 p-4 sm:p-8">
  <div class="max-w-4xl mx-auto">
    <button class="btn btn-ghost gap-2 mb-6 rounded-xl hover:bg-base-300/50" on:click={back}>
      <ChevronLeft size={20} />
      {t('frontend/src/routes/profile/[id]/+page.svelte::back')}
    </button>

    {#if loading}
      <div class="flex flex-col items-center justify-center py-20 gap-4">
        <span class="loading loading-spinner loading-lg text-primary"></span>
        <p class="text-sm font-black uppercase tracking-widest opacity-40">{t('frontend/src/routes/profile/[id]/+page.svelte::loading_profile')}</p>
      </div>
    {:else if error}
      <div class="alert alert-error shadow-lg rounded-2xl">
        <span>{error}</span>
      </div>
    {:else if user}
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <!-- Profile Card -->
        <div class="md:col-span-1" in:fade={{ delay: 100 }}>
          <div class="bg-base-100 rounded-[2.5rem] border border-base-300 shadow-xl overflow-hidden">
            <div class="h-32 bg-gradient-to-br from-primary/80 to-secondary/80 relative">
               <div class="absolute inset-0 bg-black/10"></div>
            </div>
            <div class="px-6 pb-8 text-center -mt-16 relative">
              <div class="avatar mb-4">
                <div class="w-32 h-32 rounded-[2rem] ring-8 ring-base-100 overflow-hidden shadow-2xl bg-base-300">
                  {#if user.avatar}
                    <img src={user.avatar} alt={user.name} class="w-full h-full object-cover" />
                  {:else}
                    <div class="w-full h-full bg-gradient-to-br from-primary/20 to-secondary/20 flex items-center justify-center text-5xl font-black text-primary">
                      {(user.name || user.email || '?').charAt(0).toUpperCase()}
                    </div>
                  {/if}
                </div>
              </div>
              <h1 class="text-2xl font-black tracking-tight mb-1 truncate px-2">{user.name || user.email}</h1>
              <div class="badge badge-primary badge-outline font-black uppercase tracking-widest text-[10px] py-3 px-4 rounded-full mb-6">
                {user.role}
              </div>
              
              <div class="flex flex-col gap-2">
                <button class="btn btn-primary rounded-2xl gap-2 shadow-lg shadow-primary/20" on:click={messageUser}>
                  <MessageSquare size={18} />
                  {t('frontend/src/routes/profile/[id]/+page.svelte::message_user')}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Info Card -->
        <div class="md:col-span-2 space-y-6" in:fade={{ delay: 200 }}>
          <div class="bg-base-100 rounded-[2.5rem] border border-base-300 shadow-xl p-8">
            <h2 class="text-xl font-black tracking-tight mb-8 flex items-center gap-3">
              <UserIcon class="text-primary" size={24} />
              {t('frontend/src/routes/profile/[id]/+page.svelte::about_user')}
            </h2>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-8">
              <div class="space-y-1">
                <p class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/routes/profile/[id]/+page.svelte::full_name')}</p>
                <p class="text-lg font-bold">{user.name || '-'}</p>
              </div>
               <div class="space-y-1">
                <p class="text-[10px] font-black uppercase tracking-widest opacity-40">{t('frontend/src/routes/profile/[id]/+page.svelte::email_address')}</p>
                <div class="flex items-center gap-2">
                  <Mail size={16} class="opacity-40" />
                  <p class="text-lg font-bold truncate">{user.email}</p>
                </div>
              </div>
            </div>
          </div>

          {#if user.shared_classes && user.shared_classes.length > 0}
            <div class="bg-base-100 rounded-[2.5rem] border border-base-300 shadow-xl p-8" in:fade={{ delay: 300 }}>
              <h2 class="text-xl font-black tracking-tight mb-6 flex items-center gap-3">
                <BookOpen class="text-secondary" size={24} />
                {t('frontend/src/routes/profile/[id]/+page.svelte::shared_classes')}
              </h2>
              
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {#each user.shared_classes as cls}
                  {#if $auth?.role === 'teacher' && user.role === 'student'}
                    <a 
                      href="/classes/{cls.id}/progress/{user.id}"
                      class="flex items-center justify-between p-4 rounded-2xl bg-base-200/50 border border-base-300 hover:border-primary/50 hover:bg-base-100 transition-all group"
                    >
                      <div class="flex items-center gap-3">
                        <div class="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center text-primary group-hover:bg-primary group-hover:text-primary-content transition-colors shrink-0">
                          <GraduationCap size={20} />
                        </div>
                        <span class="font-bold truncate">{cls.name}</span>
                      </div>
                      <ArrowRight size={18} class="opacity-0 group-hover:opacity-100 -translate-x-2 group-hover:translate-x-0 transition-all shrink-0" />
                    </a>
                  {:else if $auth?.role === 'student' && user.role === 'teacher'}
                    <a 
                      href="/classes/{cls.id}/overview"
                      class="flex items-center justify-between p-4 rounded-2xl bg-base-200/50 border border-base-300 hover:border-secondary/50 hover:bg-base-100 transition-all group"
                    >
                       <div class="flex items-center gap-3">
                        <div class="w-10 h-10 rounded-xl bg-secondary/10 flex items-center justify-center text-secondary group-hover:bg-secondary group-hover:text-secondary-content transition-colors shrink-0">
                          <BookOpen size={20} />
                        </div>
                        <span class="font-bold truncate">{cls.name}</span>
                      </div>
                      <ArrowRight size={18} class="opacity-0 group-hover:opacity-100 -translate-x-2 group-hover:translate-x-0 transition-all shrink-0" />
                    </a>
                  {/if}
                {/each}
              </div>
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .profile-page {
    font-family: 'Outfit', sans-serif;
  }
</style>

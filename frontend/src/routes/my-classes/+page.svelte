<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON } from '$lib/api';
  import { goto } from '$app/navigation';
  import { t } from '$lib/i18n';
  type Class = { id:number; name:string; teacher_email:string };
  let list:Class[]=[];
  let err='';
  onMount(async()=>{
    try {
      const result = await apiJSON('/api/classes');
      list = Array.isArray(result) ? result : [];
    }
    catch(e:any){ err=e.message }
  });
</script>

<h1 class="text-2xl font-bold mb-4">{t('frontend/src/routes/my-classes/+page.svelte::my_classes')}</h1>

<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
  {#each list as c}
    <div class="card bg-base-100 shadow">
      <div class="card-body">
        <h2 class="card-title">{c.name}</h2>
        <p class="text-sm text-base-content/70">{c.teacher_email}</p>
        <div class="card-actions justify-end">
          <button class="btn btn-primary btn-sm" on:click={()=>goto(`/classes/${c.id}`)}>{t('frontend/src/routes/my-classes/+page.svelte::open_class')}</button>
        </div>
      </div>
    </div>
  {/each}
  {#if !list.length && !err}
    <p>{t('frontend/src/routes/my-classes/+page.svelte::no_classes_yet')}</p>
  {/if}
</div>

{#if err}<p class="text-error mt-4">{err}</p>{/if}
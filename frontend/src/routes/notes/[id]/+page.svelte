<script lang="ts">
import { onMount } from 'svelte';
import { apiJSON, apiFetch } from '$lib/api';
import { page } from '$app/stores';
import { auth } from '$lib/auth';
import { get } from 'svelte/store';

let id = $page.params.id;
$: if ($page.params.id !== id){ id = $page.params.id; load(); }
const role: string = get(auth)?.role ?? '';

let note:any = null;
let loading=true;
let err='';

async function load(){
  loading=true; err='';
  try{ note = await apiJSON(`/api/notes/${id}`); }catch(e:any){err=e.message}
  loading=false;
}

async function publish(){
  try{ await apiFetch(`/api/notes/${id}/publish`,{method:'PUT'}); await load(); }catch(e:any){err=e.message}
}

onMount(load);
</script>

{#if loading}
<p>Loading…</p>
{:else if err}
<p class="text-error">{err}</p>
{:else}
<h1 class="text-2xl font-bold mb-4">{note.path}</h1>
<pre class="whitespace-pre-wrap">{note.content}</pre>
{#if (role==='teacher' || role==='admin') && !note.published}
  <button class="btn mt-4" on:click={publish}>Publish</button>
{/if}
{/if}

<script lang="ts">
  import { onMount } from 'svelte'
  import { page } from '$app/stores'
  import { apiFetch, apiJSON } from '$lib/api'
  import { auth } from '$lib/auth'

  $: id = $page.params.id
  $: role = $auth?.role ?? ''

  let assignment: any = null
  let tests: any[] = []
  let err = ''

  // local inputs for creating/uploading tests
  let tStdin = ''
  let tStdout = ''
  let tLimit = ''
  let tWeight = '1'
  let unittestFile: File | null = null

  async function load() {
    err = ''
    try {
      const data = await apiJSON(`/api/assignments/${id}`)
      assignment = data.assignment
      tests = data.tests ?? []
    } catch (e: any) {
      err = e.message
    }
  }

  onMount(async () => {
    await load()
  })

  async function addTest() {
    try {
      await apiFetch(`/api/assignments/${id}/tests`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          stdin: tStdin,
          expected_stdout: tStdout,
          weight: parseFloat(tWeight) || 1,
          time_limit_sec: parseFloat(tLimit) || undefined
        })
      })
      tStdin = tStdout = tLimit = ''
      tWeight = '1'
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  async function uploadUnitTests() {
    if (!unittestFile) return
    const fd = new FormData()
    fd.append('file', unittestFile)
    try {
      await apiFetch(`/api/assignments/${id}/tests/upload`, { method: 'POST', body: fd })
      unittestFile = null
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  async function delTest(tid: number) {
    if (!confirm('Delete this test?')) return
    try {
      await apiFetch(`/api/tests/${tid}`, { method: 'DELETE' })
      await load()
    } catch (e: any) {
      err = e.message
    }
  }

  async function updateTest(t: any) {
    try {
      await apiFetch(`/api/tests/${t.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          stdin: t.stdin,
          expected_stdout: t.expected_stdout,
          weight: parseFloat(t.weight) || 1,
          time_limit_sec: parseFloat(t.time_limit_sec) || undefined
        })
      })
      await load()
    } catch (e: any) {
      err = e.message
    }
  }
</script>

{#if role !== 'teacher' && role !== 'admin'}
  <div class="alert alert-error">
    <span>You do not have permission to manage tests.</span>
  </div>
{:else}
  {#if !assignment}
    <div class="flex items-center gap-3">
      <span class="loading loading-spinner loading-md"></span>
      <p>Loading…</p>
    </div>
  {:else}
    <div class="mb-4 flex items-center justify-between">
      <h1 class="text-2xl font-semibold">Manage tests — {assignment.title}</h1>
      <a class="btn" href={`/assignments/${id}`}>Back to assignment</a>
    </div>
    <div class="card-elevated p-6 space-y-4">
      <p class="text-sm opacity-70">Specify stdin, expected stdout, time limit and weight. You can also upload a unittest file.</p>
      <div class="grid gap-3 max-h-[32rem] overflow-y-auto">
        {#each tests as t, i}
          <div class="rounded-xl border border-base-300/60 p-3 space-y-2">
            <div class="flex items-center justify-between">
              <div class="font-semibold">Test {i + 1}
                {#if t.unittest_name}
                  <span class="badge badge-outline ml-2">{t.unittest_name}</span>
                {/if}
              </div>
              <div class="flex gap-2">
                <button class="btn btn-xs" on:click={() => updateTest(t)}>Save</button>
                <button class="btn btn-xs btn-error" on:click={() => delTest(t.id)}>Delete</button>
              </div>
            </div>
            {#if !t.unittest_name}
              <div class="grid sm:grid-cols-2 gap-2">
                <label class="form-control w-full space-y-1">
                  <span class="label-text">Input</span>
                  <input class="input input-bordered w-full" placeholder="stdin" bind:value={t.stdin}>
                </label>
                <label class="form-control w-full space-y-1">
                  <span class="label-text">Expected output</span>
                  <input class="input input-bordered w-full" placeholder="expected stdout" bind:value={t.expected_stdout}>
                </label>
              </div>
            {/if}
            <div class="grid sm:grid-cols-2 gap-2">
              <label class="form-control w-full space-y-1">
                <span class="label-text">Time limit (s)</span>
                <input class="input input-bordered w-full" placeholder="seconds" bind:value={t.time_limit_sec}>
              </label>
              <label class="form-control w-full space-y-1">
                <span class="label-text">Weight</span>
                <input class="input input-bordered w-full" placeholder="points" bind:value={t.weight}>
              </label>
            </div>
          </div>
        {/each}
        {#if !(tests && tests.length)}<p><i>No tests</i></p>{/if}
      </div>
      <div class="border-t pt-3 space-y-2">
        <h4 class="font-semibold">Add test</h4>
        <div class="grid sm:grid-cols-2 gap-2">
          <label class="form-control w-full space-y-1">
            <span class="label-text">Input</span>
            <input class="input input-bordered w-full" placeholder="stdin" bind:value={tStdin}>
          </label>
          <label class="form-control w-full space-y-1">
            <span class="label-text">Expected output</span>
            <input class="input input-bordered w-full" placeholder="expected stdout" bind:value={tStdout}>
          </label>
          <label class="form-control w-full space-y-1">
            <span class="label-text">Time limit (s)</span>
            <input class="input input-bordered w-full" placeholder="seconds" bind:value={tLimit}>
          </label>
          <label class="form-control w-full space-y-1">
            <span class="label-text">Weight</span>
            <input class="input input-bordered w-full" placeholder="points" bind:value={tWeight}>
          </label>
        </div>
        <div>
          <button class="btn btn-primary" on:click={addTest} disabled={!tStdin || !tStdout}>Add</button>
        </div>

        <h4 class="font-semibold mt-2">Upload unittest file</h4>
        <input type="file" accept=".py" class="file-input file-input-bordered w-full" on:change={(e) => (unittestFile = (e.target as HTMLInputElement).files?.[0] || null)}>
        <div>
          <button class="btn" on:click={uploadUnitTests} disabled={!unittestFile}>Upload</button>
        </div>
      </div>
    </div>
  {/if}
{/if}

{#if err}
  <div class="alert alert-error mt-4"><span>{err}</span></div>
{/if}

<style>
  :global(.card-elevated){
    border-radius: 1rem;
    border: 1px solid color-mix(in oklab, currentColor 20%, transparent);
    background: var(--fallback-b1, oklch(var(--b1)));
    box-shadow: 0 8px 24px rgba(0,0,0,.06);
  }
</style>



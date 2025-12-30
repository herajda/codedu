<script lang="ts">
  import { slide } from "svelte/transition";
  import { FileUp, Trash2, Plus, Upload as UploadIcon } from "lucide-svelte";
  import { t, translator } from "$lib/i18n";
  import {
    readFileBase64,
    readFileText,
    textToBase64,
  } from "$lib/utils/testFiles";

  export type AttachedFile = { name: string; content: string };

  export let files: AttachedFile[] = [];
  export let selectedIndex = -1;
  export let fileName = "";
  export let fileText = "";
  export let open = false;
  export let allowMultiple = true;
  export let onError: ((message: string) => void) | null = null;

  let translate = t;
  $: translate = $translator;

  function emitError(message: string) {
    if (onError) onError(message);
    else console.error(message);
  }

  function syncSelection(index: number) {
    if (index < 0 || index >= files.length) {
      selectedIndex = -1;
      fileName = "";
      fileText = "";
      return;
    }
    open = true;
    const target = files[index];
    if (!target) return;
    selectedIndex = index;
    fileName = target.name;
    try {
      fileText = atob(target.content);
    } catch {
      fileText = "";
    }
  }

  function removeFile(idx: number) {
    files = files.filter((_, fi) => fi !== idx);
    if (selectedIndex === idx) {
      selectedIndex = -1;
      fileName = "";
      fileText = "";
    } else if (selectedIndex > idx) {
      selectedIndex--;
    }
  }

  async function handleUpload(event: Event) {
    const input = event.target as HTMLInputElement;
    const picked = input.files || [];
    if (!picked.length) return;
    const next = [...files];
    for (let i = 0; i < picked.length; i++) {
      const file = picked[i];
      try {
        const b64 = await readFileBase64(file);
        next.push({ name: file.name, content: b64 });
        if (i === picked.length - 1) {
          selectedIndex = next.length - 1;
          fileName = file.name;
          try {
            fileText = await readFileText(file);
          } catch {
            fileText = "";
          }
        }
      } catch (err: any) {
        emitError(
          err?.message ||
            translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::file_read_error",
            ),
        );
      }
    }
    files = next;
  }

  function addStagedFile() {
    if (!fileName) return;
    const b64 = textToBase64(fileText);
    files = [...files, { name: fileName, content: b64 }];
    fileName = "";
    fileText = "";
    selectedIndex = -1;
  }
</script>

<div class="space-y-2">
  <div class="flex items-center gap-3">
    <button
      class="btn btn-sm btn-outline gap-2"
      on:click={() => (open = !open)}
    >
      <FileUp size={14} />
      {files.length > 0
        ? translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::edit_files",
          )
        : translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_files",
          )}
      {#if files.length > 0} ({files.length}) {/if}
    </button>
    <div class="flex flex-wrap gap-2">
      {#each files as f, i}
        <button
          class="badge badge-sm gap-1.5 px-2 py-1 cursor-pointer hover:bg-base-300 transform active:scale-95 transition-transform h-auto"
          class:badge-primary={selectedIndex === i}
          class:badge-neutral={selectedIndex !== i}
          on:click={() => syncSelection(i)}
        >
          {f.name}
          <span
            class="btn btn-ghost btn-xs btn-circle text-error min-h-0 h-4 w-4"
            on:click|stopPropagation={() => removeFile(i)}
          >
            <Trash2 size={10} />
          </span>
        </button>
      {/each}
      {#if files.length > 0}
        <button
          class="badge badge-sm badge-outline gap-1 px-2 py-1 cursor-pointer border-dashed h-auto"
          class:badge-active={selectedIndex === -1}
          on:click={() => {
            selectedIndex = -1;
            open = true;
            fileName = "";
            fileText = "";
          }}
        >
          <Plus size={10} /> New
        </button>
      {/if}
    </div>
  </div>

  {#if open}
    <div
      transition:slide
      class="rounded-xl border border-dashed border-base-300/70 bg-base-200/40 p-4 space-y-4 shadow-inner"
    >
      <div class="grid gap-4 sm:grid-cols-2">
        <div class="form-control w-full">
          <div class="label">
            <span class="label-text"
              >{translate(
                "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_upload",
              )}</span
            >
          </div>
          <div
            class="relative flex min-h-[120px] flex-col items-center justify-center rounded-lg border-2 border-dashed border-base-300 bg-base-100 hover:bg-base-200 hover:border-primary/50 transition-all cursor-pointer group"
          >
            <input
              type="file"
              multiple={allowMultiple}
              class="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-10"
              on:click={(e) => ((e.target as HTMLInputElement).value = "")}
              on:change={handleUpload}
            />
            <div
              class="flex flex-col items-center gap-2 text-xs opacity-60 group-hover:opacity-100 transition-opacity pointer-events-none"
            >
              <UploadIcon size={24} class="text-primary" />
              <span class="font-medium"
                >{translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_drag_drop_hint",
                )}</span
              >
            </div>
          </div>
        </div>

        <div class="flex flex-col gap-2">
          <label class="form-control w-full">
            <div class="label">
              <span class="label-text"
                >{translate(
                  "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_name",
                )}</span
              >
            </div>
            <input
              class="input input-bordered w-full"
              placeholder="data.txt"
              bind:value={fileName}
              on:input={() => {
                if (selectedIndex >= 0) {
                  files[selectedIndex].name = fileName;
                  files = files;
                }
              }}
            />
          </label>
          <p class="text-xs opacity-60 mt-auto">
            {translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_hint",
            )}
          </p>
        </div>
      </div>

      <label class="form-control w-full space-y-1">
        <span class="label-text"
          >{translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents",
          )}</span
        >
        <textarea
          class="textarea textarea-bordered w-full font-mono text-xs leading-relaxed"
          rows="5"
          placeholder={translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::test_file_contents_hint",
          )}
          bind:value={fileText}
          on:input={() => {
            if (selectedIndex >= 0) {
              files[selectedIndex].content = textToBase64(fileText);
              files = files;
            }
          }}
        ></textarea>
      </label>

      <div class="flex items-center justify-between gap-2">
        {#if selectedIndex === -1}
          <button
            class="btn btn-sm btn-secondary"
            disabled={!fileName}
            on:click={addStagedFile}
            >{translate(
              "frontend/src/routes/assignments/[id]/tests/+page.svelte::add_staged_file_to_list",
            )}</button
          >
        {:else}
          <div><!-- Spacer --></div>
        {/if}
        <button class="btn btn-sm btn-primary" on:click={() => (open = false)}>
          {translate(
            "frontend/src/routes/assignments/[id]/tests/+page.svelte::done",
          )}
        </button>
      </div>
    </div>
  {/if}
</div>

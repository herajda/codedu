<script lang="ts">
  export interface FileNode {
    name: string
    content?: string
    children?: FileNode[]
  }
  export let node: FileNode
  export let select: (n: FileNode) => void
  import FileTreeItem from './FileTreeItem.svelte'
</script>

<li>
  {#if node.children}
    <details class="ml-2">
      <summary class="cursor-pointer">{node.name}</summary>
      <ul class="menu pl-4">
        {#each node.children as child}
          <FileTreeItem {child} select={select} />
        {/each}
      </ul>
    </details>
  {:else}
    <button class="btn btn-sm btn-ghost justify-start" on:click={() => select(node)}>{node.name}</button>
  {/if}
</li>

<script lang="ts">
  import FileTreeItem from './FileTreeItem.svelte'
  import type { FileNode } from './FileTreeItem.svelte'
  export let nodes: FileNode[] = []
  export let select: (n: FileNode) => void
</script>

<ul class="menu bg-base-200 rounded">
  {#each nodes as node}
    <FileTreeItem {node} {select} />
  {/each}
</ul>

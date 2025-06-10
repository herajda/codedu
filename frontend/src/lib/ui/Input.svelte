<script lang="ts" module>
import { tv } from 'tailwind-variants';
export const inputVariants = tv({
  base: 'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50'
});
</script>

<script lang="ts">
import { cn } from '../utils';
import type { HTMLInputAttributes } from 'svelte/elements';
let { ref = $bindable(null), class: className, ...restProps }: HTMLInputAttributes & { ref?: HTMLInputElement | null } = $props();
const mergedProps = $derived({ ...restProps, class: cn(inputVariants(), className) });
</script>

<input {...mergedProps} bind:this={ref} />

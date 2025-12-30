<script lang="ts">
  // Pure presentational background. Avoids interaction and layout.
  // Uses CSS variables controlled by data-theme on <html>.
  // Respects prefers-reduced-motion.
</script>

<div class="fixed inset-0 -z-10 overflow-hidden bg-[#fafafa] dark:bg-[#050505] pointer-events-none">
    <div class="absolute inset-0 opacity-[0.4] dark:opacity-[0.2]" style="background-image: url('https://www.transparenttextures.com/patterns/cubes.png');"></div>
    <div class="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] rounded-full bg-primary/20 blur-[120px] animate-pulse"></div>
    <div class="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] rounded-full bg-secondary/20 blur-[120px] animate-pulse" style="animation-delay: 2s;"></div>
    <div class="absolute top-[20%] right-[10%] w-[25%] h-[25%] rounded-full bg-accent/10 blur-[100px] animate-pulse" style="animation-delay: 1s;"></div>
</div>

<style>
  @media (prefers-reduced-motion: reduce) {
    .animate-pulse {
      animation: none;
    }
  }
</style>

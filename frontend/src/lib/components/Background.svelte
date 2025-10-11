<script lang="ts">
  // Pure presentational background. Avoids interaction and layout.
  // Uses CSS variables controlled by data-theme on <html>.
  // Respects prefers-reduced-motion.
</script>

<div class="app-bg pointer-events-none fixed inset-0 z-0">
  <div class="app-gradient absolute inset-0"></div>
  <div class="app-aurora absolute inset-0"></div>
  <div class="app-grid absolute inset-0"></div>
</div>

<style>
  .app-bg { position: relative; }

  .app-gradient {
    background:
      radial-gradient(1200px 700px at 80% -10%, var(--bg-2), transparent 70%),
      radial-gradient(1000px 500px at 10% 10%, var(--bg-3), transparent 60%),
      radial-gradient(900px 600px at 50% 120%, var(--bg-4), transparent 60%),
      linear-gradient(180deg, var(--bg-1), transparent 60%);
  }

  .app-aurora {
    filter: blur(40px) saturate(120%);
    opacity: 0.55;
    background:
      radial-gradient(600px 300px at 20% 30%, hsl(190 95% 60% / 0.35), transparent 70%),
      radial-gradient(700px 380px at 80% 20%, hsl(210 92% 58% / 0.35), transparent 70%),
      radial-gradient(800px 420px at 50% 70%, hsl(165 85% 55% / 0.25), transparent 70%);
    animation: floatA 22s ease-in-out infinite alternate;
  }

  @keyframes floatA {
    0% { transform: translate3d(0, 0, 0) scale(1); }
    50% { transform: translate3d(0, -1.5%, 0) scale(1.02); }
    100% { transform: translate3d(0, 0.5%, 0) scale(1.015); }
  }

  .app-grid {
    --grid: color-mix(in oklab, hsl(var(--grid-color) / var(--grid-opacity)), transparent);
    background-image:
      linear-gradient(to right, var(--grid) 1px, transparent 1px),
      linear-gradient(to bottom, var(--grid) 1px, transparent 1px);
    background-size: 48px 48px;
    mask-image: radial-gradient(60% 60% at 50% 20%, black 40%, transparent 80%);
  }

  @media (prefers-reduced-motion: reduce) {
    .app-aurora { animation: none; }
  }
</style>

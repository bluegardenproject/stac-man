<script setup lang="ts">
// Neon backdrop for the home hero only. Renders behind the default
// hero content so the title gradient and CTAs read on top of it.
// Light mode shows a pastel sun gradient; dark mode adds a subtle
// scanline grid for the synthwave motif.
</script>

<template>
  <div class="hero-neon" aria-hidden="true">
    <div class="hero-neon__sun" />
    <div class="hero-neon__grid" />
  </div>
</template>

<style scoped>
.hero-neon {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 0;
}

.hero-neon__sun {
  position: absolute;
  left: 50%;
  top: 65%;
  width: 90vw;
  max-width: 900px;
  aspect-ratio: 1 / 1;
  transform: translate(-50%, -50%);
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.55;
  background: radial-gradient(
    circle at 50% 50%,
    var(--sm-neon-pink) 0%,
    var(--sm-neon-magenta) 30%,
    var(--sm-neon-purple) 55%,
    transparent 75%
  );
}

:root:not(.dark) .hero-neon__sun {
  opacity: 0.35;
  background: radial-gradient(
    circle at 50% 50%,
    rgba(255, 16, 240, 0.5) 0%,
    rgba(191, 0, 255, 0.35) 35%,
    rgba(0, 240, 255, 0.25) 65%,
    transparent 80%
  );
}

.hero-neon__grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(to right, rgba(255, 16, 240, 0.08) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(0, 240, 255, 0.08) 1px, transparent 1px);
  background-size: 40px 40px;
  mask-image: radial-gradient(ellipse at 50% 100%, black 30%, transparent 75%);
  -webkit-mask-image: radial-gradient(ellipse at 50% 100%, black 30%, transparent 75%);
  opacity: 0;
  transition: opacity 200ms ease;
}

.dark .hero-neon__grid {
  opacity: 1;
}
</style>

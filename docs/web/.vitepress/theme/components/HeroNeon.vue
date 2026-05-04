<script setup lang="ts">
// Neon backdrop for the home hero only. Mirrors the logo's sunset
// composition: a warm sun rising through pink/purple, with a faint
// retro grid in dark mode for the synthwave motif. The hex literals
// here are intentionally kept in sync with the --logo-* tokens in
// style.css; CSS variables don't compose inside scoped @apply-style
// gradients, so we duplicate. If the palette in style.css changes,
// also update the radial-gradient stops below.
</script>

<template>
  <div class="hero-neon" aria-hidden="true">
    <div class="hero-neon__sun" />
    <div class="hero-neon__horizon" />
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

/* Soft round sun. In light mode it sits low and warm; in dark mode
   it climbs slightly higher and glows pink/orange the way the logo's
   sun does over the navy sky. */
.hero-neon__sun {
  position: absolute;
  left: 50%;
  top: 70%;
  width: 90vw;
  max-width: 900px;
  aspect-ratio: 1 / 1;
  transform: translate(-50%, -50%);
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.45;
  background: radial-gradient(
    circle at 50% 50%,
    rgba(255, 210, 63, 0.55) 0%,
    rgba(255, 106, 61, 0.45) 25%,
    rgba(255, 20, 147, 0.35) 55%,
    rgba(123, 47, 190, 0.20) 75%,
    transparent 90%
  );
}

.dark .hero-neon__sun {
  top: 65%;
  opacity: 0.65;
  background: radial-gradient(
    circle at 50% 50%,
    rgba(255, 164, 43, 0.55) 0%,
    rgba(255, 20, 147, 0.45) 30%,
    rgba(123, 47, 190, 0.35) 60%,
    rgba(63, 215, 255, 0.18) 85%,
    transparent 95%
  );
}

/* Horizon band: a wide elliptical wash that suggests the band of
   color where the sunset meets the water in the logo. Subtle in
   light mode, more pronounced in dark mode. */
.hero-neon__horizon {
  position: absolute;
  left: 50%;
  bottom: -10%;
  width: 140vw;
  height: 60%;
  transform: translateX(-50%);
  border-radius: 50% / 100%;
  filter: blur(60px);
  opacity: 0.25;
  background: linear-gradient(
    180deg,
    transparent 0%,
    rgba(255, 61, 127, 0.30) 35%,
    rgba(123, 47, 190, 0.30) 70%,
    rgba(14, 18, 56, 0) 100%
  );
}

.dark .hero-neon__horizon {
  opacity: 0.55;
  background: linear-gradient(
    180deg,
    transparent 0%,
    rgba(255, 20, 147, 0.40) 30%,
    rgba(123, 47, 190, 0.40) 65%,
    rgba(14, 18, 56, 0.0) 100%
  );
}

/* Retro grid — only visible in dark mode. The pink/cyan lines echo
   the goggle-reflection palette in the logo. */
.hero-neon__grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(to right, rgba(255, 20, 147, 0.10) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(63, 215, 255, 0.10) 1px, transparent 1px);
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

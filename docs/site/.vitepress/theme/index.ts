import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import { h } from 'vue'

import HeroNeon from './components/HeroNeon.vue'
import './style.css'

const theme: Theme = {
  extends: DefaultTheme,
  Layout() {
    // Inject the neon hero backdrop on the home page only; the doc body
    // stays content-first. The slot is ignored on non-home pages.
    return h(DefaultTheme.Layout, null, {
      'home-hero-before': () => h(HeroNeon),
    })
  },
}

export default theme

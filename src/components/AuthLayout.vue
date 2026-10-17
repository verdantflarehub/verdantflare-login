<script setup>
import BrandPanel from "./BrandPanel.vue";
import { wwwBaseUrl } from "../config/site";
</script>

<template>
  <main class="auth-shell">
    <BrandPanel />
    <section class="auth-stage">
      <div class="mobile-brand">
        <img src="/brand/verdantflare-logo.svg" alt="" />
        <span>Verdant<span>Flare</span></span>
      </div>
      <div class="auth-content">
        <slot />
      </div>
      <footer class="legal-links">
        <a :href="`${wwwBaseUrl}/privacy`" target="_blank" rel="noopener noreferrer">隐私政策</a>
        <span aria-hidden="true" />
        <a :href="`${wwwBaseUrl}/terms`" target="_blank" rel="noopener noreferrer">服务条款</a>
      </footer>
    </section>
  </main>
</template>

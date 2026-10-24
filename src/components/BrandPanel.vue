<script setup>
import { wwwBaseUrl } from "../config/site";
</script>

<template>
  <aside class="brand-panel" aria-label="VerdantFlare 品牌介绍">
    <a class="brand-lockup" :href="wwwBaseUrl" aria-label="VerdantFlare 首页">
      <img src="/brand/verdantflare-logo.png" alt="" />
      <span>Verdant<span>Flare</span></span>
    </a>

    <div class="brand-statement">
      <h2>让创意，<br />持续<span>生长</span>。</h2>
      <p>一个账号，连接模型、应用与创作工作流。</p>
    </div>
  </aside>
</template>

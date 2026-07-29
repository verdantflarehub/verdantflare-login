<script setup>
import { onMounted, ref } from "vue";
import { authApi } from "../api/auth";
import AppIcon from "../components/AppIcon.vue";
import { navigate } from "../router";

const loading = ref(true);
const error = ref("");
const redirectTo = ref("");

onMounted(async () => {
  try {
    const result = await authApi.completeCallback(new URLSearchParams(window.location.search));
    redirectTo.value = result.redirectTo;
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "身份验证失败";
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="result-state">
    <template v-if="loading"><span class="result-icon pending"><span class="spinner dark" /></span><h1>正在完成登录</h1><p>我们正在校验身份提供商返回的授权信息。</p></template>
    <template v-else-if="error"><span class="result-icon error"><AppIcon name="warning" :size="30" /></span><h1>无法完成登录</h1><p>{{ error }}</p><button class="button primary" @click="navigate('/sign-in')">重新登录</button></template>
    <template v-else><span class="result-icon"><AppIcon name="check" :size="30" /></span><h1>身份验证成功</h1><p>企业身份已验证，可以安全进入 Hub。</p><a class="button primary" :href="redirectTo">进入 Hub <AppIcon name="arrow" :size="18" /></a></template>
  </div>
</template>

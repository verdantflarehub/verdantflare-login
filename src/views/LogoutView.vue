<script setup>
import { onMounted, ref } from "vue";
import { authApi, safeReturnTo } from "../api/auth";
import AppIcon from "../components/AppIcon.vue";
import { navigate } from "../router";

const loading = ref(true);
const error = ref("");
const returnTo = safeReturnTo(new URLSearchParams(window.location.search).get("return_to"));

onMounted(async () => {
  try {
    await authApi.logout(returnTo);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "退出登录失败";
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="result-state">
    <template v-if="loading"><span class="result-icon pending"><span class="spinner dark" /></span><h1>正在安全退出</h1><p>正在结束当前设备上的登录会话。</p></template>
    <template v-else-if="error"><span class="result-icon error"><AppIcon name="warning" :size="30" /></span><h1>暂时无法退出</h1><p>{{ error }}</p><button class="button primary" @click="navigate('/sign-in')">返回登录</button></template>
    <template v-else><span class="result-icon"><AppIcon name="check" :size="30" /></span><h1>你已安全退出</h1><p>当前设备上的 VerdantFlare 会话已经结束。</p><button class="button primary" @click="navigate(`/sign-in?return_to=${encodeURIComponent(returnTo)}`)">重新登录</button></template>
  </div>
</template>

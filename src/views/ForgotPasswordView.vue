<script setup>
import { computed, ref } from "vue";
import { authApi, safeReturnTo } from "../api/auth";
import AppIcon from "../components/AppIcon.vue";
import FormMessage from "../components/FormMessage.vue";
import { navigate } from "../router";

const returnTo = safeReturnTo(new URLSearchParams(window.location.search).get("return_to"));
const email = ref("");
const loading = ref(false);
const error = ref("");
const sent = ref(false);
const signInUrl = `/sign-in?return_to=${encodeURIComponent(returnTo)}`;
const canSubmit = computed(() => /^\S+@\S+\.\S+$/.test(email.value) && !loading.value);

const submit = async () => {
  loading.value = true;
  error.value = "";
  try {
    await authApi.forgotPassword(email.value);
    sent.value = true;
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "发送失败，请稍后重试";
  } finally {
    loading.value = false;
  }
};
</script>

<template>
  <div v-if="sent" class="result-state">
    <span class="result-icon mail"><AppIcon name="mail" :size="32" /></span>
    <h1>检查你的邮箱</h1>
    <p>如果 <strong>{{ email }}</strong> 已注册，你会收到一封密码重置邮件。</p>
    <a class="button primary" :href="signInUrl" @click.prevent="navigate(signInUrl)">返回登录</a>
  </div>

  <form v-else class="auth-form" novalidate @submit.prevent="submit">
    <header class="form-heading"><h1>找回密码</h1><p>输入注册邮箱，我们会发送安全的密码重置链接。</p></header>
    <label class="field"><span>邮箱</span><span class="input-wrap"><AppIcon name="mail" :size="20" /><input v-model.trim="email" type="email" autocomplete="email" placeholder="name@company.com" required /></span></label>
    <FormMessage v-if="error" :text="error" />
    <button class="button primary" type="submit" :disabled="!canSubmit"><span v-if="loading" class="spinner" />{{ loading ? "正在发送…" : "发送重置链接" }}</button>
    <p class="form-switch"><a :href="signInUrl" @click.prevent="navigate(signInUrl)">返回登录</a></p>
    <p class="security-note"><AppIcon name="shield" :size="17" />为保护账号安全，我们不会提示邮箱是否已注册</p>
  </form>
</template>

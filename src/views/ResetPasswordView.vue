<script setup>
import { computed, ref } from "vue";
import { authApi, safeReturnTo } from "../api/auth";
import AppIcon from "../components/AppIcon.vue";
import FormMessage from "../components/FormMessage.vue";
import { navigate } from "../router";

const query = new URLSearchParams(window.location.search);
const returnTo = safeReturnTo(query.get("return_to"));
const token = (query.get("token") || "").trim();
const password = ref("");
const confirmation = ref("");
const showPassword = ref(false);
const loading = ref(false);
const error = ref("");
const done = ref(false);
const signInUrl = `/sign-in?return_to=${encodeURIComponent(returnTo)}`;
const forgotPasswordUrl = `/forgot-password?return_to=${encodeURIComponent(returnTo)}`;
const validPassword = computed(() => password.value.length >= 8 && /[A-Za-z]/.test(password.value) && /\d/.test(password.value));
const canSubmit = computed(() => Boolean(token) && validPassword.value && password.value === confirmation.value && !loading.value);

const submit = async () => {
  if (!token || loading.value) return;
  error.value = "";
  if (password.value !== confirmation.value) {
    error.value = "两次输入的密码不一致";
    return;
  }
  loading.value = true;
  try {
    await authApi.resetPassword({ token, password: password.value });
    done.value = true;
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "密码重置失败";
  } finally {
    loading.value = false;
  }
};
</script>

<template>
  <div v-if="!token" class="result-state">
    <span class="result-icon error"><AppIcon name="warning" :size="30" /></span>
    <h1>重置链接无效</h1>
    <p>链接缺少密码重置信息，请重新申请重置邮件，并打开邮件中的完整链接。</p>
    <a class="button primary" :href="forgotPasswordUrl" @click.prevent="navigate(forgotPasswordUrl)">重新申请重置链接</a>
    <a :href="signInUrl" @click.prevent="navigate(signInUrl)">返回登录</a>
  </div>
  <div v-else-if="done" class="result-state">
    <span class="result-icon"><AppIcon name="check" :size="30" /></span>
    <h1>密码已更新</h1>
    <p>请使用新密码重新登录你的 VerdantFlare 账号。</p>
    <a class="button primary" :href="signInUrl" @click.prevent="navigate(signInUrl)">返回登录</a>
  </div>

  <form v-else class="auth-form" novalidate @submit.prevent="submit">
    <header class="form-heading"><h1>设置新密码</h1><p>新密码至少 8 位，并同时包含字母与数字。</p></header>
    <label class="field"><span>新密码</span><span class="input-wrap"><AppIcon name="lock" :size="20" /><input v-model="password" :type="showPassword ? 'text' : 'password'" autocomplete="new-password" placeholder="至少 8 位，包含字母与数字" /><button class="icon-button" type="button" :aria-label="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword"><AppIcon :name="showPassword ? 'eyeOff' : 'eye'" :size="20" /></button></span></label>
    <label class="field"><span>确认新密码</span><span class="input-wrap"><AppIcon name="lock" :size="20" /><input v-model="confirmation" :type="showPassword ? 'text' : 'password'" autocomplete="new-password" placeholder="再次输入新密码" /></span></label>
    <FormMessage v-if="error" :text="error" />
    <p v-if="error" class="form-switch"><a :href="forgotPasswordUrl" @click.prevent="navigate(forgotPasswordUrl)">重新申请重置链接</a></p>
    <button class="button primary" type="submit" :disabled="!canSubmit"><span v-if="loading" class="spinner" />{{ loading ? "正在保存…" : "更新密码" }}</button>
    <p class="form-switch"><a :href="signInUrl" @click.prevent="navigate(signInUrl)">返回登录</a></p>
  </form>
</template>

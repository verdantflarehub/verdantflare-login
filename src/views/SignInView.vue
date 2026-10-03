<script setup>
import { computed, ref } from "vue";
import { authApi, hubUrl, safeReturnTo } from "../api/auth";
import { managementPathFor } from "../api/management";
import AppIcon from "../components/AppIcon.vue";
import FormMessage from "../components/FormMessage.vue";
import { navigate } from "../router";

const query = new URLSearchParams(window.location.search);
const returnTo = safeReturnTo(query.get("return_to"));
const devAdminEmail = import.meta.env.VITE_DEV_ADMIN_EMAIL || "";
const email = ref(query.get("email") || "");
const password = ref("");
const showPassword = ref(false);
const loading = ref(false);
const error = ref("");
const redirectTo = ref("");
const managementTo = ref("");
const canSubmit = computed(() =>
  (/^\S+@\S+\.\S+$/.test(email.value) || (devAdminEmail && email.value.toLowerCase() === "admin"))
  && password.value.length >= 8 && !loading.value,
);

const withReturnTo = (path) => `${path}?return_to=${encodeURIComponent(returnTo)}`;

const submit = async () => {
  error.value = "";
  loading.value = true;
  try {
    const loginEmail = devAdminEmail && email.value.toLowerCase() === "admin" ? devAdminEmail : email.value;
    const result = await authApi.signIn({ email: loginEmail, password: password.value, returnTo });
    redirectTo.value = result.redirectTo;
    try {
      const managementPath = managementPathFor(await authApi.getCenterContext());
      managementTo.value = managementPath ? hubUrl(managementPath) : "";
    } catch {
      managementTo.value = "";
    }
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "登录失败，请重试";
  } finally {
    loading.value = false;
  }
};
</script>

<template>
  <div v-if="redirectTo" class="result-state">
    <span class="result-icon"><AppIcon name="check" :size="30" /></span>
    <h1>登录成功</h1>
    <p>安全会话已建立，可以继续进入 VerdantFlare Hub。</p>
    <div class="result-actions">
      <a class="button primary" :href="redirectTo">进入 Hub <AppIcon name="arrow" :size="18" /></a>
      <a v-if="managementTo" class="button secondary" :href="managementTo">进入管理端 <AppIcon name="arrow" :size="18" /></a>
    </div>
  </div>

  <form v-else class="auth-form" novalidate @submit.prevent="submit">
    <header class="form-heading">
      <h1>欢迎回来</h1>
      <p>登录后继续使用 Hub、API 与 Studio。</p>
    </header>

    <label class="field">
      <span>{{ devAdminEmail ? "邮箱或用户名" : "邮箱" }}</span>
      <span class="input-wrap">
        <AppIcon name="mail" :size="20" />
        <input v-model.trim="email" :type="devAdminEmail ? 'text' : 'email'" autocomplete="username" :placeholder="devAdminEmail ? 'admin 或 name@company.com' : 'name@company.com'" required />
      </span>
    </label>

    <label class="field">
      <span class="field-row"><span>密码</span><a :href="withReturnTo('/forgot-password')" @click.prevent="navigate(withReturnTo('/forgot-password'))">忘记密码？</a></span>
      <span class="input-wrap">
        <AppIcon name="lock" :size="20" />
        <input v-model="password" :type="showPassword ? 'text' : 'password'" autocomplete="current-password" placeholder="请输入密码" minlength="8" required />
        <button class="icon-button" type="button" :aria-label="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword">
          <AppIcon :name="showPassword ? 'eyeOff' : 'eye'" :size="20" />
        </button>
      </span>
    </label>

    <FormMessage v-if="error" :text="error" />
    <button class="button primary" type="submit" :disabled="!canSubmit">
      <span v-if="loading" class="spinner" aria-hidden="true" />
      {{ loading ? "正在登录…" : "登录" }}
    </button>

    <div class="divider"><span>或</span></div>

    <a class="button secondary" :href="authApi.enterpriseSso(returnTo)">
      <AppIcon name="building" :size="21" />使用企业 SSO 登录
    </a>

    <p class="form-switch">还没有账号？ <a :href="withReturnTo('/sign-up')" @click.prevent="navigate(withReturnTo('/sign-up'))">创建账号</a></p>
    <p class="security-note"><AppIcon name="shield" :size="17" />受安全连接保护</p>
  </form>
</template>

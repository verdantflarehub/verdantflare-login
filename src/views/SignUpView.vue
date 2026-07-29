<script setup>
import { computed, onUnmounted, ref } from "vue";
import { authApi, safeReturnTo } from "../api/auth";
import AppIcon from "../components/AppIcon.vue";
import FormMessage from "../components/FormMessage.vue";
import { navigate } from "../router";

const returnTo = safeReturnTo(new URLSearchParams(window.location.search).get("return_to"));
const email = ref("");
const code = ref("");
const password = ref("");
const accepted = ref(false);
const showPassword = ref(false);
const loading = ref(false);
const sending = ref(false);
const countdown = ref(0);
const error = ref("");
const createdEmail = ref("");
let countdownTimer;

const isEmail = computed(() => /^\S+@\S+\.\S+$/.test(email.value));
const canSubmit = computed(() => isEmail.value && /^\d{6}$/.test(code.value) && /[A-Za-z]/.test(password.value) && /\d/.test(password.value) && password.value.length >= 8 && accepted.value && !loading.value);
const signInUrl = computed(() => `/sign-in?return_to=${encodeURIComponent(returnTo)}&email=${encodeURIComponent(email.value)}`);

const startCountdown = () => {
  window.clearInterval(countdownTimer);
  countdown.value = 60;
  countdownTimer = window.setInterval(() => {
    countdown.value -= 1;
    if (countdown.value <= 0) window.clearInterval(countdownTimer);
  }, 1000);
};

const sendCode = async () => {
  if (!isEmail.value || sending.value || countdown.value) return;
  error.value = "";
  sending.value = true;
  try {
    const result = await authApi.sendVerificationCode(email.value);
    if (result.debugCode) code.value = result.debugCode;
    startCountdown();
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "验证码发送失败";
  } finally {
    sending.value = false;
  }
};

const submit = async () => {
  error.value = "";
  loading.value = true;
  try {
    const result = await authApi.signUp({ email: email.value, code: code.value, password: password.value, accepted: accepted.value, returnTo });
    createdEmail.value = result.email;
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "创建账号失败";
  } finally {
    loading.value = false;
  }
};

onUnmounted(() => window.clearInterval(countdownTimer));
</script>

<template>
  <div v-if="createdEmail" class="result-state verify-state">
    <span class="result-icon"><AppIcon name="check" :size="30" /></span>
    <h1>账号创建成功</h1>
    <p><strong>{{ createdEmail }}</strong> 已完成邮箱验证，现在可以登录 VerdantFlare。</p>
    <a :href="signInUrl" @click.prevent="navigate(signInUrl)">返回登录</a>
  </div>

  <form v-else class="auth-form signup-form" novalidate @submit.prevent="submit">
    <header class="form-heading">
      <h1>创建账号</h1>
      <p>开始使用 VerdantFlare Hub、API 与 Studio。</p>
    </header>

    <label class="field"><span>邮箱</span><span class="input-wrap"><AppIcon name="mail" :size="20" /><input v-model.trim="email" type="email" autocomplete="email" placeholder="name@company.com" required /></span></label>

    <label class="field">
      <span>邮箱验证码</span>
      <span class="code-row">
        <span class="input-wrap"><input v-model="code" type="text" inputmode="numeric" autocomplete="one-time-code" maxlength="6" placeholder="输入 6 位验证码" required /></span>
        <button class="button code-button" type="button" :disabled="!isEmail || sending || countdown" @click="sendCode">
          {{ countdown ? `${countdown}s` : sending ? "发送中…" : "获取验证码" }}
        </button>
      </span>
    </label>

    <label class="field"><span>设置密码</span><span class="input-wrap"><AppIcon name="lock" :size="20" /><input v-model="password" :type="showPassword ? 'text' : 'password'" autocomplete="new-password" placeholder="至少 8 位，包含字母与数字" minlength="8" required /><button class="icon-button" type="button" :aria-label="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword"><AppIcon :name="showPassword ? 'eyeOff' : 'eye'" :size="20" /></button></span></label>

    <label class="check-row"><input v-model="accepted" type="checkbox" /><span>我已阅读并同意<a href="https://www.verdantflarehub.com/terms">《服务条款》</a>和<a href="https://www.verdantflarehub.com/privacy">《隐私政策》</a></span></label>
    <FormMessage v-if="error" :text="error" />

    <button class="button primary" type="submit" :disabled="!canSubmit"><span v-if="loading" class="spinner" />{{ loading ? "正在创建…" : "创建账号" }}</button>
    <p class="form-switch">已有账号？ <a :href="signInUrl" @click.prevent="navigate(signInUrl)">返回登录</a></p>
  </form>
</template>

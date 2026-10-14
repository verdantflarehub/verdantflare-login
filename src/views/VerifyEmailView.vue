<script setup>
import { onUnmounted, ref } from "vue";
import { authApi, safeReturnTo } from "../api/auth";
import AppIcon from "../components/AppIcon.vue";
import FormMessage from "../components/FormMessage.vue";
import { navigate } from "../router";

const query = new URLSearchParams(window.location.search);
const returnTo = safeReturnTo(query.get("return_to"));
const email = (query.get("email") || "").trim();
const validEmail = /^\S+@\S+\.\S+$/.test(email);
const sending = ref(false);
const countdown = ref(0);
const sent = ref(false);
const error = ref("");
let timer;
const signInUrl = `/sign-in?return_to=${encodeURIComponent(returnTo)}&email=${encodeURIComponent(query.get("email") || "")}`;
const signUpUrl = `/sign-up?return_to=${encodeURIComponent(returnTo)}${validEmail ? `&email=${encodeURIComponent(email)}` : ""}`;

const resend = async () => {
  if (!validEmail || sending.value || countdown.value) return;
  error.value = "";
  sending.value = true;
  try {
    await authApi.resendVerification(email);
    sent.value = true;
    window.clearInterval(timer);
    countdown.value = 60;
    timer = window.setInterval(() => {
      countdown.value -= 1;
      if (!countdown.value) window.clearInterval(timer);
    }, 1000);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "验证码发送失败，请稍后重试";
  } finally {
    sending.value = false;
  }
};

onUnmounted(() => window.clearInterval(timer));
</script>

<template>
  <div class="result-state verify-state">
    <span class="result-icon mail"><AppIcon name="mail" :size="32" /></span>
    <h1>验证你的邮箱</h1>
    <p v-if="!validEmail">请返回创建账号页面，填写邮箱并获取验证码。</p>
    <p v-else-if="sent" role="status">验证码发送请求已受理。请检查 <strong>{{ email }}</strong> 的收件箱与垃圾邮件，并在创建账号页面填写验证码、完成注册。</p>
    <p v-else>为 <strong>{{ email }}</strong> 获取注册验证码后，请在创建账号页面填写验证码、完成注册。</p>
    <FormMessage v-if="error" :text="error" />
    <button v-if="validEmail" class="button secondary" type="button" :disabled="sending || countdown" @click="resend">{{ countdown ? `${countdown} 秒后可重发` : sending ? "正在发送…" : sent ? "重新发送验证码" : "获取验证码" }}</button>
    <a class="button primary" :href="signUpUrl" @click.prevent="navigate(signUpUrl)">返回创建账号</a>
    <a :href="signInUrl" @click.prevent="navigate(signInUrl)">返回登录</a>
  </div>
</template>

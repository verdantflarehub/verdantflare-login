<script setup>
import { onUnmounted, ref } from "vue";
import { authApi, safeReturnTo } from "../api/auth";
import AppIcon from "../components/AppIcon.vue";
import { navigate } from "../router";

const query = new URLSearchParams(window.location.search);
const returnTo = safeReturnTo(query.get("return_to"));
const email = query.get("email") || "你的注册邮箱";
const sending = ref(false);
const countdown = ref(0);
let timer;
const signInUrl = `/sign-in?return_to=${encodeURIComponent(returnTo)}&email=${encodeURIComponent(query.get("email") || "")}`;

const resend = async () => {
  sending.value = true;
  try {
    await authApi.resendVerification(email);
    countdown.value = 60;
    timer = window.setInterval(() => {
      countdown.value -= 1;
      if (!countdown.value) window.clearInterval(timer);
    }, 1000);
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
    <p>验证邮件已发送至 <strong>{{ email }}</strong>，完成验证后即可登录。</p>
    <button class="button secondary" type="button" :disabled="sending || countdown" @click="resend">{{ countdown ? `${countdown} 秒后可重发` : sending ? "正在发送…" : "重新发送邮件" }}</button>
    <a :href="signInUrl" @click.prevent="navigate(signInUrl)">返回登录</a>
  </div>
</template>

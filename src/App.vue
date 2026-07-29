<script setup>
import { computed, watch } from "vue";
import AuthLayout from "./components/AuthLayout.vue";
import { currentPath, navigate } from "./router";
import CallbackView from "./views/CallbackView.vue";
import ForgotPasswordView from "./views/ForgotPasswordView.vue";
import LogoutView from "./views/LogoutView.vue";
import ResetPasswordView from "./views/ResetPasswordView.vue";
import SignInView from "./views/SignInView.vue";
import SignUpView from "./views/SignUpView.vue";
import VerifyEmailView from "./views/VerifyEmailView.vue";

const route = computed(() => {
  if (currentPath.value === "/sign-in") return SignInView;
  if (currentPath.value === "/sign-up") return SignUpView;
  if (currentPath.value === "/forgot-password") return ForgotPasswordView;
  if (currentPath.value === "/reset-password") return ResetPasswordView;
  if (currentPath.value === "/verify-email") return VerifyEmailView;
  if (currentPath.value.startsWith("/callback/")) return CallbackView;
  if (currentPath.value === "/logout") return LogoutView;
  return null;
});

watch(currentPath, (path) => {
  if (path === "/" || !route.value) navigate(`/sign-in${window.location.search}`);
}, { immediate: true });
</script>

<template>
  <AuthLayout>
    <Transition name="view" mode="out-in">
      <component :is="route" v-if="route" :key="currentPath" />
    </Transition>
  </AuthLayout>
</template>

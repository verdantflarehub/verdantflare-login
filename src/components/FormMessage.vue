<script setup>
import AppIcon from "./AppIcon.vue";

defineProps({
  type: { type: String, default: "error" },
  text: { type: String, required: true },
});
</script>

<template>
  <p class="form-message" :class="type" :role="type === 'error' ? 'alert' : 'status'">
    <AppIcon :name="type === 'error' ? 'warning' : 'check'" :size="17" />
    {{ text }}
  </p>
</template>

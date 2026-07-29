<script setup>
defineProps({
  name: { type: String, required: true },
  size: { type: Number, default: 20 },
});

const paths = {
  mail: ["M4 5h16v14H4z", "m4 7 8 6 8-6"],
  lock: ["M6 10h12v10H6z", "M9 10V7a3 3 0 0 1 6 0v3"],
  eye: ["M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6S2.5 12 2.5 12Z", "M12 9a3 3 0 1 1 0 6 3 3 0 0 1 0-6Z"],
  eyeOff: ["m3 3 18 18", "M10.6 6.2A9.8 9.8 0 0 1 12 6c6 0 9.5 6 9.5 6a14.8 14.8 0 0 1-2.1 3.1", "M6.2 6.2C3.8 8 2.5 12 2.5 12s3.5 6 9.5 6a9.7 9.7 0 0 0 3.2-.5", "M9.9 9.9a3 3 0 0 0 4.2 4.2"],
  building: ["M4 21h16", "M6 21V7l6-4v18", "M12 9h6v12", "M9 10h.01", "M9 14h.01", "M15 13h.01", "M15 17h.01"],
  shield: ["M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z", "m9 12 2 2 4-4"],
  arrow: ["M5 12h14", "m14 7 5 5-5 5"],
  check: ["m5 12 4 4L19 6"],
  warning: ["M12 3 2.7 20h18.6L12 3Z", "M12 9v4", "M12 17h.01"],
};
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path v-for="item in paths[name] || []" :key="item" :d="item" />
  </svg>
</template>

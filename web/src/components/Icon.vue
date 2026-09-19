<script setup lang="ts">
defineProps<{ name: string; size?: number }>();
const paths: Record<string, string> = {
  plus: "M12 5v14M5 12h14",
  search: "m21 21-4.5-4.5M19 11a8 8 0 1 1-16 0 8 8 0 0 1 16 0",
  filter: "M4 7h16M7 12h10M10 17h4",
  close: "m6 6 12 12M18 6 6 18",
  back: "m15 18-6-6 6-6",
  camera:
    "M14.5 4h-5L7 7H3v13h18V7h-4l-2.5-3ZM16 13a4 4 0 1 1-8 0 4 4 0 0 1 8 0",
  image: "M3 3h18v18H3zM3 16l5-5 5 5 3-3 5 5M8 7h.01",
  pin: "M20 10c0 6-8 12-8 12S4 16 4 10a8 8 0 1 1 16 0ZM15 10a3 3 0 1 1-6 0 3 3 0 0 1 6 0",
  ruler: "m3 16 13-13 5 5L8 21zM7 12l2 2M10 9l2 2M13 6l2 2",
  edit: "m15 5 4 4M4 20l4-1L21 6l-4-4L4 15v5Z",
  trash: "M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7",
  download: "M12 3v12m-5-5 5 5 5-5M5 16v5h14v-5",
  grid: "M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z",
  check: "m5 12 4 4L19 6",
  chevron: "m9 5 7 7-7 7",
  more: "M5 12h.01M12 12h.01M19 12h.01",
  box: "m3 7 9-4 9 4-9 4-9-4Zm0 0v10l9 4 9-4V7M12 11v10",
  restore: "M3 10a9 9 0 1 1 2 8M3 4v6h6M12 7v5l3 2",
  copy: "M9 9h12v12H9zM5 15H3V3h12v2",
  arrow: "M12 19V5m-6 6 6-6 6 6",
  alert: "m12 3 10 18H2L12 3Zm0 6v5m0 3h.01",
  scissors:
    "M6 8a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm0 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0ZM5 10l16 11M5 18 21 3",
  menu: "M4 6h16M4 12h16M4 18h16",
};
</script>
<template>
  <svg
    :width="size || 20"
    :height="size || 20"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.65"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="paths[name] || paths.box" />
  </svg>
</template>

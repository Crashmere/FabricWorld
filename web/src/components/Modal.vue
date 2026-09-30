<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import Icon from "./Icon.vue";
defineProps<{ title: string; photo?: boolean }>();
const emit = defineEmits<{ close: [] }>();
const box = ref<HTMLElement>();
let previous: Element | null;
let scroll = 0;
function keys(e: KeyboardEvent) {
  if (e.key === "Escape") emit("close");
}
onMounted(() => {
  previous = document.activeElement;
  scroll = window.scrollY;
  document.body.style.position = "fixed";
  document.body.style.top = "-" + scroll + "px";
  document.body.style.width = "100%";
  document.addEventListener("keydown", keys);
  box.value?.querySelector<HTMLElement>("button")?.focus();
});
onUnmounted(() => {
  document.body.style.position = "";
  document.body.style.top = "";
  document.body.style.width = "";
  window.scrollTo(0, scroll);
  document.removeEventListener("keydown", keys);
  (previous as HTMLElement)?.focus?.();
});
</script>
<template>
  <Teleport to="body"
    ><div class="modal-scrim" :class="{ 'photo-scrim': photo }" @click.self="emit('close')">
      <section
        ref="box"
        class="modal"
        :class="{ 'modal-photo': photo }"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
      >
        <header>
          <h2>{{ title }}</h2>
          <button class="icon-button" aria-label="关闭" @click="emit('close')">
            <Icon name="close" />
          </button>
        </header>
        <slot />
      </section></div
  ></Teleport>
</template>

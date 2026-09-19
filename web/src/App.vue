<script setup lang="ts">
import { ref, nextTick, watch, onMounted, onUnmounted } from "vue";
import { useRoute } from "vue-router";
import { notice } from "./api";
import Icon from "./components/Icon.vue";
const brandIcon = import.meta.env.BASE_URL + "fabricworld.svg?v=swatches";
const offline = ref(!navigator.onLine);
const update = () => (offline.value = !navigator.onLine);
const route = useRoute();
const menuOpen = ref(false);
const menu = ref<HTMLElement>();
const menuButton = ref<HTMLButtonElement>();
function closeMenu(focus = false) {
  menuOpen.value = false;
  if (focus) menuButton.value?.focus();
}
function outside(e: PointerEvent) {
  if (!menu.value?.contains(e.target as Node)) closeMenu();
}
function leaveMenu(e: FocusEvent) {
  if (!menu.value?.contains(e.relatedTarget as Node | null)) closeMenu();
}
function menuKeys(e: KeyboardEvent) {
  if (menuOpen.value && e.key === "Escape") {
    e.preventDefault();
    closeMenu(true);
  }
}
async function openWithKeyboard() {
  menuOpen.value = true;
  await nextTick();
  menu.value?.querySelector<HTMLElement>("nav a")?.focus();
}
watch(() => route.fullPath, () => closeMenu());
onMounted(() => {
  window.addEventListener("online", update);
  window.addEventListener("offline", update);
  document.addEventListener("pointerdown", outside);
  document.addEventListener("keydown", menuKeys);
});
onUnmounted(() => {
  window.removeEventListener("online", update);
  window.removeEventListener("offline", update);
  document.removeEventListener("pointerdown", outside);
  document.removeEventListener("keydown", menuKeys);
});
</script>
<template>
  <header class="site-header">
    <RouterLink class="brand" to="/" aria-label="FabricWorld 布料库">
      <img class="brand-mark" :src="brandIcon" width="40" height="40" alt="" />
      <span>Fabric<span class="brand-light">World</span></span>
    </RouterLink>
    <div ref="menu" class="header-menu" @focusout="leaveMenu">
      <button ref="menuButton" type="button" class="menu-toggle"
        aria-label="打开菜单" :aria-expanded="menuOpen" aria-controls="header-navigation"
        @click="menuOpen = !menuOpen" @keydown.down.prevent="openWithKeyboard">
        <Icon name="menu" :size="24" />
      </button>
      <nav v-if="menuOpen" id="header-navigation" class="header-dropdown" aria-label="网站菜单">
        <RouterLink to="/trash" @click="closeMenu()"><Icon name="trash" />回收站</RouterLink>
        <RouterLink to="/materials" @click="closeMenu()"><Icon name="box" />材质管理</RouterLink>
      </nav>
    </div>
  </header>
  <div v-if="offline" class="offline" role="status">
    网络已断开，尚未保存的输入会保留在当前页面。
  </div>
  <main>
    <RouterView v-slot="{ Component, route }"
      ><component :is="Component" :key="route.path"
    /></RouterView>
  </main>
  <Transition name="toast"
    ><div v-if="notice" class="toast" role="status">
      <Icon name="check" />{{ notice }}
    </div></Transition
  >
</template>

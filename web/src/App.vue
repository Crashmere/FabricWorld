<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { notice } from "./api";
import Icon from "./components/Icon.vue";
const brandIcon = import.meta.env.BASE_URL + "fabricworld.svg";
const offline = ref(!navigator.onLine);
const update = () => (offline.value = !navigator.onLine);
onMounted(() => {
  window.addEventListener("online", update);
  window.addEventListener("offline", update);
});
onUnmounted(() => {
  window.removeEventListener("online", update);
  window.removeEventListener("offline", update);
});
</script>
<template>
  <header class="site-header">
    <RouterLink class="brand" to="/" aria-label="FabricWorld 布料库">
      <img class="brand-mark" :src="brandIcon" width="40" height="40" alt="" />
      <span>Fabric<span class="brand-light">World</span></span>
    </RouterLink>
    <nav>
      <RouterLink to="/" class="nav-link"
        ><Icon name="grid" /><span>布料库</span></RouterLink
      ><RouterLink to="/trash" class="nav-link"
        ><Icon name="trash" /><span>回收站</span></RouterLink
      >
    </nav>
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

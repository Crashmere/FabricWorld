<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { notice } from "./api";
import Icon from "./components/Icon.vue";
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
    <RouterLink class="brand" to="/" aria-label="FabricWorld 布料库"
      ><span class="brand-mark"><Icon name="box" :size="25" /></span
      ><span>Fabric<span class="brand-light">World</span></span></RouterLink
    >
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

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { APIError, key, request, toast, write } from "../api";
import Icon from "../components/Icon.vue";
import Modal from "../components/Modal.vue";
type Material = { name: string; count: number; trashCount: number; version: string };
type Removal = { name: string; affected: number };
type Pending = { material: Material; operationKey: string };
const items = ref<Material[]>([]);
const loading = ref(true), busy = ref(false), error = ref(""), actionError = ref(""), search = ref("");
const pending = ref<Pending | null>(null), uncertain = ref(false);
const storageKey = "fabricworld:material-removal";
const filtered = computed(() => items.value.filter(item => item.name.toLocaleLowerCase().includes(search.value.trim().toLocaleLowerCase())));
async function load() {
  loading.value = true;
  error.value = "";
  try { items.value = await request<Material[]>("materials"); }
  catch (e) { error.value = (e as Error).message; }
  finally { loading.value = false; }
}
function confirm(material: Material) {
  pending.value = { material, operationKey: key() };
  actionError.value = "";
}
function close() {
  if (!busy.value && !uncertain.value) pending.value = null;
}
function remember() {
  try { sessionStorage.setItem(storageKey, JSON.stringify(pending.value)); } catch {}
}
function forget() {
  try { sessionStorage.removeItem(storageKey); } catch {}
}
async function completed(result: Removal) {
  uncertain.value = false;
  forget();
  pending.value = null;
  toast("已从 " + result.affected + " 条布料中移除“" + result.name + "”");
  await load();
}
async function remove() {
  if (!pending.value || busy.value) return;
  busy.value = true;
  actionError.value = "";
  uncertain.value = true;
  remember();
  const { material, operationKey } = pending.value;
  try {
    await completed(await write<Removal>("materials/remove", "POST", { name: material.name, version: material.version }, operationKey));
  } catch (e) {
    uncertain.value = !(e instanceof APIError) || e.status === 0 || e.status >= 500;
    if (!uncertain.value) forget();
    actionError.value = (e as Error).message;
    if (e instanceof APIError && e.status === 409) {
      pending.value = null;
      await load();
      error.value = "相关布料已发生变化，列表已刷新，请重新选择并确认移除。";
    } else if (uncertain.value) {
      actionError.value += " 可查询结果，或按原请求重试。";
    }
  } finally { busy.value = false; }
}
async function resolve() {
  if (!pending.value || busy.value) return;
  busy.value = true;
  try { await completed(await request<Removal>("operations/" + pending.value.operationKey)); }
  catch (e) { actionError.value = (e as Error).message + "。可按原请求重试。"; }
  finally { busy.value = false; }
}
onMounted(async () => {
  try {
    const cached = JSON.parse(sessionStorage.getItem(storageKey) || "null");
    if (cached?.material?.name && cached?.material?.version && cached?.operationKey) {
      pending.value = cached;
      uncertain.value = true;
      await resolve();
    }
  } catch {}
  await load();
});
</script>

<template>
  <div class="page materials-page">
    <RouterLink class="back-link" to="/"><Icon name="back" />返回布料库</RouterLink>
    <div class="page-heading compact">
      <div>
        <div class="eyebrow">YOUR FABRIC MATERIALS</div>
        <h1>材质管理</h1>
        <p>查看已添加的材质，以及使用它们的布料数量。</p>
      </div>
    </div>
    <p v-if="error" class="error-banner" role="alert">{{ error }}<button type="button" @click="load">刷新列表</button></p>
    <p class="field-hint material-count-hint">数量包含回收站中的布料。同一条布料含多种材质时，会分别计入。</p>
    <label class="material-search"><span class="sr-only">搜索材质</span><input v-model="search" type="search" placeholder="搜索材质名称" /></label>
    <p v-if="loading" class="loading-text">正在加载材质…</p>
    <ul v-else-if="filtered.length" class="material-list">
      <li v-for="item in filtered" :key="item.name" class="material-row">
        <div class="material-info">
          <h2>{{ item.name }}</h2>
          <p>{{ item.count }} 条布料<span v-if="item.trashCount"> · {{ item.trashCount }} 条在回收站</span></p>
        </div>
        <button type="button" class="button secondary material-remove" :aria-label="'移除材质 ' + item.name" @click="confirm(item)"><Icon name="trash" :size="17" />移除</button>
      </li>
    </ul>
    <p v-else class="loading-text">{{ search ? '没有找到匹配的材质。' : '还没有已添加的材质。在记录或编辑布料时添加并保存即可。' }}</p>
    <Modal v-if="pending" title="移除材质？" @close="close">
      <div class="filter-form">
        <p>将从 {{ pending.material.count }} 条布料中移除“{{ pending.material.name }}”及其百分比<span v-if="pending.material.trashCount">，其中包含 {{ pending.material.trashCount }} 条回收站记录</span>。</p>
        <p>布料记录、照片和成分说明会保留。若只剩一种材质，其比例恢复为 100%。</p>
        <p v-if="actionError" class="error-banner" role="alert">{{ actionError }}</p>
        <div class="modal-actions">
          <button v-if="!uncertain" type="button" class="button secondary" :disabled="busy" @click="close">取消</button>
          <button v-else type="button" class="button secondary" :disabled="busy" @click="resolve">查询结果</button>
          <button type="button" class="button danger" :disabled="busy" @click="remove">{{ busy ? '处理中…' : uncertain ? '重试移除' : '确认移除' }}</button>
        </div>
      </div>
    </Modal>
  </div>
</template>

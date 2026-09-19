<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { APIError, key, request, toast, write } from "../api";
import Icon from "../components/Icon.vue";
import Modal from "../components/Modal.vue";
type Material = { name: string; count: number; trashCount: number; version: string };
type Result = { name: string; affected?: number };
type Pending = { material: Material; operationKey: string; action?: "add" | "remove" };
const items = ref<Material[]>([]);
const loading = ref(true), busy = ref(false), error = ref(""), actionError = ref(""), search = ref("");
const pending = ref<Pending | null>(null), uncertain = ref(false);
const storageKey = "fabricworld:material-removal";
const searchName = computed(() => search.value.trim());
const adding = computed(() => pending.value?.action === "add");
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
function add() {
  if (!searchName.value || filtered.value.length || loading.value || error.value || busy.value) return;
  pending.value = { action: "add", material: { name: searchName.value, count: 0, trashCount: 0, version: "" }, operationKey: key() };
  submit();
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
async function completed(result: Result) {
  const wasAdding = adding.value;
  uncertain.value = false;
  forget();
  pending.value = null;
  toast(wasAdding ? "已添加材质“" + result.name + "”" : "已移除材质“" + result.name + "”，更新了 " + result.affected + " 条布料");
  await load();
}
async function submit() {
  if (!pending.value || busy.value) return;
  busy.value = true;
  actionError.value = "";
  uncertain.value = true;
  remember();
  const { material, operationKey } = pending.value;
  try {
    const path = adding.value ? "materials" : "materials/remove";
    const body = adding.value ? { name: material.name } : { name: material.name, version: material.version };
    await completed(await write<Result>(path, "POST", body, operationKey));
  } catch (e) {
    uncertain.value = !(e instanceof APIError) || e.status === 0 || e.status >= 500;
    if (!uncertain.value) forget();
    actionError.value = (e as Error).message;
    if (e instanceof APIError && e.code === "material_changed") {
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
  try { await completed(await request<Result>("operations/" + pending.value.operationKey)); }
  catch (e) { actionError.value = (e as Error).message + "。可按原请求重试。"; }
  finally { busy.value = false; }
}
onMounted(async () => {
  try {
    const cached = JSON.parse(sessionStorage.getItem(storageKey) || "null");
    if (cached?.material?.name && (cached.action === "add" || cached.material.version) && cached?.operationKey) {
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
    <label class="material-search"><span class="sr-only">搜索材质</span><input v-model="search" type="search" maxlength="40" placeholder="搜索材质名称" /></label>
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
    <div v-else-if="!error" class="material-empty">
      <p>{{ searchName ? '没有找到匹配的材质。' : '还没有已添加的材质。输入名称即可添加，也可以在记录布料时添加。' }}</p>
      <button v-if="searchName" type="button" class="button primary material-add" :disabled="busy" @click="add"><Icon name="plus" /><span>添加“{{ searchName }}”</span></button>
    </div>
    <Modal v-if="pending" :title="adding ? '添加材质' : '移除材质？'" @close="close">
      <div class="filter-form">
        <p v-if="adding">添加“{{ pending.material.name }}”后，即可在记录或编辑布料时选择。</p>
        <template v-else>
          <p v-if="pending.material.count">将从 {{ pending.material.count }} 条布料中移除“{{ pending.material.name }}”及其百分比<span v-if="pending.material.trashCount">，其中包含 {{ pending.material.trashCount }} 条回收站记录</span>。</p>
          <p v-else>移除材质“{{ pending.material.name }}”？目前没有布料使用它。</p>
          <p v-if="pending.material.count">布料记录、照片和成分说明会保留。若只剩一种材质，其比例恢复为 100%。</p>
        </template>
        <p v-if="actionError" class="error-banner" role="alert">{{ actionError }}</p>
        <div class="modal-actions">
          <button v-if="!uncertain" type="button" class="button secondary" :disabled="busy" @click="close">取消</button>
          <button v-else type="button" class="button secondary" :disabled="busy" @click="resolve">查询结果</button>
          <button type="button" class="button" :class="adding ? 'primary' : 'danger'" :disabled="busy" @click="submit">{{ busy ? '处理中…' : adding ? '重试添加' : uncertain ? '重试移除' : '确认移除' }}</button>
        </div>
      </div>
    </Modal>
  </div>
</template>

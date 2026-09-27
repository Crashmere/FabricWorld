<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { APIError, key, mediaURL, request, toast, write } from "../api";
import { dateText } from "../types";
import type { Work, WorkChange } from "../works";
import Icon from "../components/Icon.vue";
import Modal from "../components/Modal.vue";
import PhotoViewer from "../components/PhotoViewer.vue";
const route = useRoute(), router = useRouter();
const w = ref<Work>(), error = ref(""), loading = ref(true), busy = ref(false), selected = ref(0);
const lightbox = ref(false), confirmDelete = ref(false), showHistory = ref(false), history = ref<WorkChange[]>([]);
type Operation = { key: string; action: "delete" | "restore"; revision: number; at: number };
const pending = ref<Operation>();
const operationStorage = "fabricworld:work-action:" + route.params.id;
function remember(op?: Operation) {
  pending.value = op;
  try { if (op) sessionStorage.setItem(operationStorage, JSON.stringify(op)); else sessionStorage.removeItem(operationStorage); } catch {}
}
async function completed(result: Work, action: Operation["action"]) {
  remember(); confirmDelete.value = false;
  if (action === "delete") { toast("成品已移入回收站，可在 30 天内恢复"); await router.replace("/works"); }
  else { w.value = result; toast("成品已恢复"); }
}
async function resolve() {
  if (!pending.value || busy.value) return;
  busy.value = true;
  try { await completed(await request<Work>("operations/" + pending.value.key), pending.value.action); }
  catch (e) { error.value = (e as Error).message + "。请查询或按原提交重试。"; }
  finally { busy.value = false; }
}
async function mutate(action: Operation["action"]) {
  if (!w.value || busy.value) return;
  const op = pending.value || { key: key(), action, revision: w.value.revision, at: Date.now() };
  if (Date.now() - op.at >= 7 * 86400000) { error.value = "提交超过 7 天，请先刷新核对记录状态，勿重复提交。"; return; }
  busy.value = true; error.value = ""; remember(op);
  try { await completed(await write<Work>("works/" + w.value.id + (op.action === "restore" ? "/restore" : ""), op.action === "restore" ? "POST" : "DELETE", { revision: op.revision }, op.key), op.action); }
  catch (e) {
    error.value = (e as Error).message; confirmDelete.value = false;
    if (e instanceof APIError && e.status > 0 && e.status < 500) remember();
  } finally { busy.value = false; }
}
async function load() {
  loading.value = true;
  try { w.value = await request<Work>("works/" + route.params.id); selected.value = 0; }
  catch (e) { error.value = (e as Error).message; }
  finally { loading.value = false; }
}
async function changes() {
  showHistory.value = !showHistory.value;
  if (showHistory.value) {
    try { history.value = await request<WorkChange[]>("works/" + route.params.id + "/changes"); }
    catch (e) { error.value = (e as Error).message; }
  }
}
onMounted(async () => {
  try { pending.value = JSON.parse(sessionStorage.getItem(operationStorage) || "null") || undefined; } catch {}
  await load();
  if (pending.value) await resolve();
});
</script>
<template>
  <div class="page detail-page work-detail">
    <RouterLink class="back-link" :to="w?.deletedAt ? '/works/trash' : '/works'"><Icon name="back" />{{ w?.deletedAt ? '返回成品回收站' : '返回成品集' }}</RouterLink>
    <div v-if="error" class="error-banner" role="alert">{{ error }}<button :disabled="busy" @click="load">刷新记录</button></div>
    <div v-if="pending" class="info-banner work-recovery"><p>上次操作结果尚未确认。</p><button :disabled="busy" @click="resolve">查询上次结果</button><button :disabled="busy" @click="mutate(pending.action)">按原提交重试</button></div>
    <p v-if="loading" class="loading-text">正在展开这件作品…</p>
    <template v-else-if="w">
      <div v-if="w.deletedAt" class="info-banner">成品已于 {{ dateText(w.deletedAt) }} 移入回收站。<button :disabled="busy || Boolean(pending)" @click="mutate('restore')">恢复成品</button></div>
      <div class="detail-layout">
        <section class="gallery">
          <button v-if="w.photos[selected] && !w.deletedAt" class="main-photo" aria-label="查看照片大图" @click="($event.currentTarget as HTMLElement).focus(); lightbox = true"><img :src="mediaURL(w.photos[selected]!.id, 'main')" :alt="w.name" /><span><Icon name="search" :size="16" />查看大图</span></button>
          <div v-else class="main-photo no-photo-art"><Icon name="scissors" :size="52" /><span>{{ w.deletedAt ? '恢复后可查看照片' : '还没有成品照片' }}</span></div>
          <div v-if="w.photos.length > 1 && !w.deletedAt" class="gallery-thumbs"><button v-for="(p, i) in w.photos" :key="p.id" :class="{selected: selected === i}" :aria-label="'查看第 ' + (i + 1) + ' 张照片'" @click="selected = i"><img :src="mediaURL(p.id)" :alt="'照片 ' + (i + 1)" /></button></div>
        </section>
        <section class="detail-info">
          <div class="detail-title"><div class="eyebrow">MADE BY HAND</div><h1>{{ w.name }}</h1><div class="detail-tags"><span v-if="w.category">{{ w.category }}</span><span v-for="tag in w.tags" :key="tag">{{ tag }}</span></div><p class="muted">{{ w.completedDate ? w.completedDate + ' 完成' : '完成日期待补充' }}</p></div>
          <div v-if="!w.deletedAt && !pending" class="detail-actions"><RouterLink class="button primary" :to="'/works/' + w.id + '/edit'"><Icon name="edit" />编辑成品</RouterLink></div>
          <div class="detail-block"><h2>纸样与尺码</h2><dl class="purchase-info"><dt>纸样 / 教程</dt><dd>{{ w.pattern || '未填写' }}</dd><dt>尺码</dt><dd>{{ w.size || '未填写' }}</dd><dt>为谁制作</dt><dd>{{ w.recipient || '未填写' }}</dd></dl></div>
          <div class="detail-block"><h2><Icon name="box" />所用布料</h2><p v-if="!w.fabrics.length" class="muted">还没有关联布料。</p><div v-for="f in w.fabrics" :key="f.fabricId" class="work-source-detail"><RouterLink v-if="f.available" :to="'/fabrics/' + f.fabricId"><strong>{{ f.name }}</strong><Icon name="chevron" :size="17" /></RouterLink><strong v-else>{{ f.name }} <span class="optional">来源已不可用</span></strong><p v-if="f.note" class="notes">{{ f.note }}</p></div><p v-if="w.fabrics.some(f => f.available)" class="field-hint">用布后可打开布料详情更新余料。</p></div>
          <div class="detail-block"><h2>制作心得</h2><p class="notes">{{ w.notes || '还没有记录心得，下次可以慢慢补充。' }}</p></div>
          <div class="detail-bottom"><button class="text-button" @click="changes"><Icon name="restore" :size="17" />{{ showHistory ? '收起修改历史' : '查看修改历史' }}</button><button v-if="!w.deletedAt" class="icon-button danger-text" aria-label="删除成品" :disabled="busy || Boolean(pending)" @click="confirmDelete = true"><Icon name="trash" /></button></div>
          <p class="record-time">录入于 {{ dateText(w.createdAt) }} · 最近修改 {{ dateText(w.updatedAt) }}</p>
        </section>
      </div>
      <section v-if="showHistory" class="history-panel"><h2>修改历史</h2><div v-for="h in history" :key="h.revision" class="history-item"><span class="history-dot"></span><div><strong>{{ h.revision === 1 ? '首次记录' : h.action === 'delete' ? '移入回收站' : h.action === 'restore' ? '恢复成品' : '编辑成品' }}</strong><time>{{ dateText(h.at) }}</time><details><summary>查看当时的信息</summary><p>{{ h.after.name }} · {{ h.after.category || '未分类' }} · {{ h.after.completedDate || '日期未填' }}</p><p>纸样：{{ h.after.pattern || '未填' }} · 尺码：{{ h.after.size || '未填' }} · 为谁制作：{{ h.after.recipient || '未填' }}</p><p>标签：{{ h.after.tags.join('、') || '未填' }} · 照片 {{ h.after.photoIds.length }} 张</p><p v-for="f in h.after.fabrics" :key="f.fabricId">{{ f.name }}：{{ f.note || '无用布说明' }}</p><p class="notes">{{ h.after.notes }}</p></details></div></div></section>
      <Modal v-if="confirmDelete" title="将成品移入回收站？" @close="!busy && (confirmDelete = false)"><div class="filter-form"><p>“{{ w.name }}”及其照片保留 30 天，可以恢复。关联布料和库存不受影响。</p><div class="modal-actions"><button class="button secondary" :disabled="busy" @click="confirmDelete = false">保留成品</button><button class="button danger" :disabled="busy" @click="mutate('delete')">{{ busy ? '处理中…' : '移入回收站' }}</button></div></div></Modal>
      <PhotoViewer v-if="lightbox && w.photos[selected]" :src="mediaURL(w.photos[selected]!.id, 'main')" :alt="w.name" :title="w.name" @close="lightbox = false"><template v-if="w.photos.length > 1" #navigation><button class="icon-button" :disabled="selected === 0" aria-label="上一张" @click="selected--"><Icon name="back" /></button><span>{{ selected + 1 }} / {{ w.photos.length }}</span><button class="icon-button" :disabled="selected === w.photos.length - 1" aria-label="下一张" @click="selected++"><Icon name="chevron" /></button></template></PhotoViewer>
    </template>
  </div>
</template>

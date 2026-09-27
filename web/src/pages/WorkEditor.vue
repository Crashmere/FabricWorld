<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { onBeforeRouteLeave, useRoute, useRouter } from "vue-router";
import { APIError, key, request, toast, write } from "../api";
import { newWork, workCategories, type Work } from "../works";
import type { Fabric, FabricList } from "../types";
import Icon from "../components/Icon.vue";
import WorkPhotos from "../components/WorkPhotos.vue";
const route = useRoute(), router = useRouter(), editing = Boolean(route.params.id);
const w = ref(newWork()), loading = ref(true), initialized = ref(false), saving = ref(false), saved = ref(false);
const error = ref(""), field = ref(""), recovered = ref(false), pending = ref(0), tags = ref("");
const uncertain = ref(false), conflict = ref(false), expired = ref(false), latest = ref<Work>();
const draftKey = "fabricworld:work-draft:" + (route.params.id || "new");
let initial = "", operationKey = key(), lastBody = "", operationAt = 0;
const fabricQuery = ref(""), fabrics = ref<FabricList>(), searching = ref(false), fabricError = ref("");
let searchSequence = 0;
const dirty = computed(() => initialized.value && JSON.stringify(w.value) !== initial);
function persist() {
  if (!initialized.value || saved.value) return;
  try { sessionStorage.setItem(draftKey, JSON.stringify({ work: w.value, tags: tags.value, operationKey, lastBody, operationAt, uncertain: uncertain.value, at: Date.now() })); } catch {}
}
watch(w, persist, { deep: true });
watch(tags, value => { w.value.tags = value.split(/[，,、]/).map(s => s.trim()).filter(Boolean); persist(); });
watch(() => w.value.photos, photos => { w.value.photoIds = photos.map(p => p.id); }, { deep: true });
async function finished(result: Work) {
  saved.value = true; uncertain.value = false;
  sessionStorage.removeItem(draftKey);
  toast(editing ? "成品修改已保存" : "成品已记录");
  await router.replace("/works/" + result.id);
}
async function send(body: Work) {
  saving.value = true; uncertain.value = true; error.value = ""; field.value = "";
  persist();
  try { await finished(await write<Work>(editing ? "works/" + route.params.id : "works", editing ? "PUT" : "POST", body, operationKey)); }
  catch (e) {
    error.value = (e as Error).message;
    if (e instanceof APIError) {
      field.value = e.field;
      uncertain.value = e.status === 0 || e.status >= 500;
      conflict.value = e.code === "revision_conflict";
      if (conflict.value) {
        try { latest.value = await request<Work>("works/" + route.params.id); } catch {}
      }
    }
    if (!uncertain.value) { operationKey = key(); lastBody = ""; }
  } finally { saving.value = false; persist(); }
}
async function save() {
  if (saving.value || !initialized.value || pending.value) return;
  if (uncertain.value || expired.value) { error.value = "上次提交结果尚未确认，请先查询结果或按原内容重试。"; return; }
  if (conflict.value) return;
  const body = JSON.parse(JSON.stringify(w.value)) as Work;
  operationKey = key(); lastBody = JSON.stringify(body); operationAt = Date.now();
  await send(body);
}
async function resolve() {
  if (saving.value) return;
  saving.value = true;
  try { await finished(await request<Work>("operations/" + operationKey)); }
  catch (e) { error.value = (e as Error).message + (expired.value ? "。请先到成品集核对，勿重复提交。" : "。可按原内容重试，提交编号保持不变。"); }
  finally { saving.value = false; }
}
async function retry() {
  if (saving.value || !lastBody || expired.value) return;
  if (Date.now() - operationAt >= 7 * 86400000) {
    expired.value = true;
    error.value = "提交已超过 7 天，请先到成品集核对，勿重复提交。";
    return;
  }
  await send(JSON.parse(lastBody));
}
async function reconcile() {
  if (saving.value || !window.confirm("已核对最新记录？将保留当前输入，按最新版本重新提交；其他地方的修改可能被当前输入覆盖。")) return;
  try {
    const current = await request<Work>("works/" + route.params.id);
    if (current.deletedAt) { error.value = "成品已删除，请先从回收站恢复。"; return; }
    w.value.revision = current.revision; conflict.value = false; latest.value = undefined; error.value = "";
    persist();
  } catch (e) { error.value = (e as Error).message; }
}
async function searchFabrics(more = false) {
  const seq = ++searchSequence;
  searching.value = true; fabricError.value = "";
  const p = new URLSearchParams({ status: "all", q: fabricQuery.value });
  if (more) p.set("offset", String(fabrics.value?.items.length || 0));
  try {
    const result = await request<FabricList>("fabrics?" + p);
    if (seq === searchSequence) fabrics.value = { ...result, items: more ? [...(fabrics.value?.items || []), ...result.items] : result.items };
  } catch (e) { if (seq === searchSequence) fabricError.value = (e as Error).message; }
  finally { if (seq === searchSequence) searching.value = false; }
}
function addFabric(f: Fabric) {
  if (w.value.fabrics.length >= 20 || w.value.fabrics.some(x => x.fabricId === f.id)) return;
  w.value.fabrics.push({ fabricId: f.id, name: f.name, note: "", available: true });
}
function beforeUnload(e: BeforeUnloadEvent) { persist(); if (dirty.value || pending.value || saving.value) e.preventDefault(); }
onBeforeRouteLeave(() => {
  if (saved.value) return true;
  if (saving.value) return false;
  return !(dirty.value || pending.value || uncertain.value) || window.confirm("离开编辑页面？文字和已上传照片保留在本次会话，未上传照片需要重新选择。");
});
onMounted(async () => {
  window.addEventListener("beforeunload", beforeUnload);
  try {
    let cached: { work: Work; tags: string; operationKey: string; lastBody: string; operationAt: number; uncertain: boolean; at: number } | null = null;
    try { cached = JSON.parse(sessionStorage.getItem(draftKey) || "null"); } catch {}
    // Resolve an interrupted commit before comparing revisions, including when
    // the old draft is older than the normal 24-hour upload lifetime.
    if (cached?.uncertain && cached.lastBody && cached.operationKey) {
      w.value = cached.work; tags.value = cached.tags; operationKey = cached.operationKey;
      lastBody = cached.lastBody; operationAt = cached.operationAt || cached.at; uncertain.value = true;
      expired.value = Date.now() - operationAt >= 7 * 86400000;
      initial = JSON.stringify(w.value); initialized.value = true; recovered.value = true;
      await resolve();
      return;
    }
    if (editing) w.value = await request<Work>("works/" + route.params.id);
    if (w.value.deletedAt) { error.value = "请先从回收站恢复这件成品。"; return; }
    initial = JSON.stringify(w.value);
    if (cached && Date.now() - cached.at < 86400000) {
      const current = w.value;
      w.value = cached.work; recovered.value = true;
      if (editing && cached.work.revision !== current.revision) {
        conflict.value = true; latest.value = current;
        error.value = "草稿保留了你的输入，但服务端已有新版本，请先核对。";
      }
    } else if (!editing && typeof route.query.fabric === "string") {
      const source = await request<Fabric>("fabrics/" + route.query.fabric);
      if (!source.deletedAt) addFabric(source);
    }
    tags.value = w.value.tags.join("，"); initialized.value = true;
  } catch (e) { error.value = (e as Error).message; }
  finally { loading.value = false; }
});
onUnmounted(() => window.removeEventListener("beforeunload", beforeUnload));
</script>
<template>
  <div class="page editor-page work-editor">
    <RouterLink class="back-link" :to="editing ? '/works/' + route.params.id : '/works'"><Icon name="back" />{{ editing ? '返回成品详情' : '返回成品集' }}</RouterLink>
    <div class="page-heading compact"><div><div class="eyebrow">{{ editing ? 'REFINE THE DETAILS' : 'A FINISHED PIECE' }}</div><h1>{{ editing ? '编辑成品' : '记录成品' }}</h1><p>记下纸样、用布和心得，让下一次制作更从容。</p></div></div>
    <p v-if="recovered" class="info-banner">已恢复本次会话的成品草稿。未上传的照片请重新选择。</p>
    <div v-if="error" class="error-banner" role="alert">{{ error }}</div>
    <div v-if="uncertain" class="info-banner work-recovery"><p>{{ expired ? '提交已超过 7 天，请先到成品集核对是否保存成功。' : '上次提交结果尚未确认，先查询或重试原提交。' }}</p><button type="button" :disabled="saving" @click="resolve">查询上次结果</button><button v-if="!expired" type="button" :disabled="saving" @click="retry">按原内容重试</button></div>
    <section v-if="conflict" class="info-banner work-recovery"><p>当前输入已保留。最新记录：{{ latest?.name }} · {{ latest?.completedDate || '日期未填' }} · {{ latest?.pattern || '纸样未填' }}</p><p v-if="latest?.notes" class="notes">{{ latest.notes }}</p><RouterLink :to="'/works/' + route.params.id" target="_blank">打开最新记录核对</RouterLink><button type="button" @click="reconcile">已核对，保留当前输入</button></section>
    <p v-if="loading" class="loading-text">正在加载成品…</p>
    <form v-else-if="initialized" id="work-form" class="editor-layout" @submit.prevent="save">
      <fieldset class="photo-column" :disabled="saving || uncertain"><WorkPhotos v-model="w.photos" v-model:pending="pending" /></fieldset>
      <fieldset class="fields-column" :disabled="saving || uncertain">
        <section class="form-panel"><div class="section-title"><h2><span class="section-number">01</span>这件作品</h2></div>
          <label>成品名称<input v-model="w.name" required maxlength="200" placeholder="例如：秋天的亚麻衬衫" :aria-invalid="field === 'name'" /></label>
          <div class="field-row purchase-fields"><label>类别 <span class="optional">选填</span><input v-model="w.category" list="work-categories" maxlength="40" placeholder="选择或输入类别" /><datalist id="work-categories"><option v-for="c in workCategories" :key="c" :value="c" /></datalist></label><label>完成日期 <span class="optional">选填</span><span class="date-input"><input v-model="w.completedDate" type="date" min="1900-01-01" max="2200-12-31" :aria-invalid="field === 'completedDate'" /></span></label></div>
          <label>纸样 / 教程 <span class="optional">选填</span><input v-model="w.pattern" maxlength="300" placeholder="纸样名称、编号或教程出处" /></label>
          <div class="field-row"><label>尺码<input v-model="w.size" maxlength="80" placeholder="例如：M，腰围按需调整" /></label><label>为谁制作<input v-model="w.recipient" maxlength="80" placeholder="例如：自己、家人" /></label></div>
          <label>标签<input v-model="tags" maxlength="820" placeholder="例如：春秋，练习作品，以逗号分隔" :aria-invalid="field === 'tags'" /></label>
        </section>
        <section class="form-panel"><div class="section-title"><h2><span class="section-number">02</span>所用布料</h2><span>{{ w.fabrics.length }} / 20</span></div>
          <p class="field-hint">可以关联多块布料，包括已用完的布料。保存成品后，可前往布料详情更新余料。</p>
          <div v-for="(f, i) in w.fabrics" :key="f.fabricId" class="work-fabric-choice"><div><strong>{{ f.name }}</strong><button type="button" class="icon-button" :aria-label="'取消关联 ' + f.name" @click="w.fabrics.splice(i, 1)"><Icon name="close" :size="17" /></button></div><span v-if="!f.available" class="field-hint">来源布料已不可用，仍保留关联记录。</span><label>用布说明<input v-model="f.note" maxlength="300" :aria-label="f.name + ' 用布说明'" placeholder="例如：主布约 1.5 米；用于领口和袖口" /></label></div>
          <div class="work-fabric-search"><label>查找布料<input v-model="fabricQuery" placeholder="输入布料名称、材质…" @keydown.enter.prevent="searchFabrics()" /></label><button type="button" class="button secondary" :disabled="searching" @click="searchFabrics()">{{ searching ? '查找中…' : '查找' }}</button></div>
          <p v-if="fabricError" class="error-banner" role="alert">{{ fabricError }}</p>
          <div v-if="fabrics" class="work-fabric-results"><p v-if="!fabrics.total" class="field-hint">没有找到布料，也可以稍后再关联。</p><button v-for="f in fabrics.items" :key="f.id" type="button" :disabled="w.fabrics.some(x => x.fabricId === f.id) || w.fabrics.length >= 20" @click="addFabric(f)"><span>{{ f.name }}<small>{{ f.materials.join(' / ') }}{{ f.status === 'used' ? ' · 已用完' : '' }}</small></span><Icon :name="w.fabrics.some(x => x.fabricId === f.id) ? 'check' : 'plus'" :size="18" /></button><button v-if="fabrics.items.length < fabrics.total" type="button" :disabled="searching" @click="searchFabrics(true)">加载更多布料</button></div>
        </section>
        <section class="form-panel"><div class="section-title"><h2><span class="section-number">03</span>制作心得</h2></div><label>制作心得<textarea v-model="w.notes" rows="7" maxlength="8000" placeholder="纸样做了哪些调整？哪里最满意？下次想改进什么？" /></label></section>
      </fieldset>
    </form>
    <div v-if="initialized && !uncertain" class="save-bar"><span>{{ pending ? '请等待照片上传，或重试 / 移除失败照片' : '保存这次手作的记忆' }}</span><button class="button primary" type="submit" form="work-form" :disabled="saving || Boolean(pending) || conflict"><Icon name="check" />{{ saving ? '正在保存…' : editing ? '保存修改' : '保存成品' }}</button></div>
  </div>
</template>

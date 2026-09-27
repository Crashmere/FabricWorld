<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { base, mediaURL, request } from "../api";
import type { WorkList } from "../works";
import Icon from "../components/Icon.vue";
import ChoiceField from "../components/ChoiceField.vue";
const route = useRoute(), router = useRouter();
const trash = computed(() => route.path === "/works/trash");
const data = ref<WorkList>({ items: [], total: 0, offset: 0, categories: [] });
const search = ref(""), loading = ref(false), error = ref("");
let sequence = 0;
function params() {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(route.query)) if (typeof v === "string") p.set(k, v);
  p.delete("offset"); p.delete("trash");
  if (trash.value) p.set("trash", "1");
  return p;
}
async function load(more = false) {
  const seq = ++sequence, p = params();
  if (more) p.set("offset", String(data.value.items.length));
  loading.value = true; error.value = "";
  try {
    const result = await request<WorkList>("works?" + p);
    if (seq === sequence) data.value = { ...result, items: more ? [...data.value.items, ...result.items] : result.items };
  } catch (e) { if (seq === sequence) error.value = (e as Error).message; }
  finally { if (seq === sequence) loading.value = false; }
}
function query(values: Record<string, string>) {
  const q = { ...route.query, ...values }; delete q.offset;
  for (const k of Object.keys(q)) if (!q[k]) delete q[k];
  void router.replace({ path: route.path, query: q });
}
function exportURL(format: string) { const p = params(); p.delete("trash"); p.set("format", format); return base + "api/works/export?" + p; }
watch(() => route.fullPath, () => { search.value = String(route.query.q || ""); void load(); }, { immediate: true });
</script>
<template>
  <div class="page works-page">
    <nav v-if="trash" class="module-tabs" aria-label="回收站分类"><RouterLink to="/trash">布料回收站</RouterLink><RouterLink to="/works/trash" class="active" aria-current="page">成品回收站</RouterLink></nav>
    <div class="page-heading">
      <div><div class="eyebrow">{{ trash ? 'KEEP THE MEMORIES' : 'MADE BY HAND' }}</div><h1>{{ trash ? '成品回收站' : '我的成品' }}</h1><p>{{ trash ? '删除的成品保留 30 天，打开记录即可恢复。' : '从一块布，到一件喜欢的作品。' }}</p></div>
      <RouterLink v-if="!trash" class="button primary" to="/works/new"><Icon name="plus" />记录成品</RouterLink>
    </div>
    <section class="collection-controls">
      <form class="search-box" @submit.prevent="query({ q: search })"><Icon name="search" /><input v-model="search" aria-label="搜索成品" placeholder="搜索成品、纸样、布料、心得…" /><button v-if="search" type="button" class="icon-button" aria-label="清除搜索" @click="search = ''; query({q: ''})"><Icon name="close" :size="17" /></button><button class="search-submit" type="submit">搜索</button></form>
      <div class="work-filters">
        <label><span class="sr-only">成品类别筛选</span><ChoiceField label="成品类别筛选" :model-value="String(route.query.category || '')" :options="[{ value: '', label: '全部类别' }, ...[...new Set([...data.categories, String(route.query.category || '')])].filter(Boolean)]" @update:model-value="query({ category: $event })" /></label>
        <label><span class="sr-only">成品排序</span><ChoiceField label="成品排序" :model-value="String(route.query.sort || 'completed')" :options="[{ value: 'completed', label: '最近完成' }, { value: 'updated', label: '最近修改' }, { value: 'name', label: '名称排序' }]" @update:model-value="query({ sort: $event })" /></label>
      </div>
    </section>
    <div v-if="route.query.fabric" class="info-banner">正在查看使用指定布料的成品。<button @click="query({fabric: ''})">查看全部</button></div>
    <div class="work-results"><span>{{ data.total }} 件成品</span><div v-if="!trash && data.total"><a :href="exportURL('csv')" download>导出表格</a><a :href="exportURL('zip')" download>导出含照片</a></div></div>
    <div v-if="error" class="error-banner" role="alert">{{ error }}<button @click="load()">重试</button></div>
    <p v-if="loading && !data.items.length" class="loading-text">正在展开成品集…</p>
    <div v-else-if="!data.items.length && !error" class="empty-state"><div class="empty-art"><Icon name="scissors" :size="48" /></div><h2>{{ trash ? '回收站是空的' : route.query.q || route.query.category || route.query.fabric ? '没有找到符合条件的成品' : '第一件作品，值得被记住' }}</h2><p>{{ trash ? '删除的成品会暂存在这里。' : '照片、纸样和制作心得，都可以在这里慢慢积累。' }}</p><RouterLink v-if="!trash" to="/works/new" class="button primary"><Icon name="plus" />记录第一件成品</RouterLink></div>
    <div v-else class="fabric-grid work-grid" :aria-busy="loading">
      <article v-for="item in data.items" :key="item.id" class="fabric-card work-card"><RouterLink :to="'/works/' + item.id" class="card-link">
        <div class="card-photo"><img v-if="item.photos[0] && !trash" :src="mediaURL(item.photos[0].id)" :alt="item.name" loading="lazy" /><div v-else class="no-photo-art"><Icon name="scissors" :size="42" /></div><span v-if="item.category" class="work-category">{{ item.category }}</span></div>
        <div class="card-body"><h2>{{ item.name }}</h2><p>{{ item.completedDate || '完成日期待补充' }}</p><p v-if="item.pattern" class="work-pattern">{{ item.pattern }}</p><div v-if="item.fabrics.length" class="work-source"><Icon name="box" :size="14" />{{ item.fabrics.map(f => f.name).join(' · ') }}</div></div>
      </RouterLink></article>
    </div>
    <div v-if="data.items.length < data.total" class="load-more"><button class="button secondary" :disabled="loading" @click="load(true)">{{ loading ? '加载中…' : '加载更多成品' }}</button></div>
    <RouterLink v-if="!trash" class="back-link work-trash-link" to="/works/trash"><Icon name="trash" :size="16" />成品回收站</RouterLink>
  </div>
</template>

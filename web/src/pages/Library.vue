<script setup lang="ts">
import { computed, ref, watch, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { base, request, write, key, toast, mediaURL } from "../api";
import { dimensions, statuses, type Fabric, type FabricList } from "../types";
import Icon from "../components/Icon.vue";
import Modal from "../components/Modal.vue";
const route = useRoute(),
  router = useRouter();
const trash = computed(() => route.path === "/trash");
const data = ref<FabricList>({ items: [], total: 0, offset: 0, stats: {} });
const loading = ref(false),
  error = ref(""),
  filterOpen = ref(false),
  exportOpen = ref(false),
  search = ref(String(route.query.q || "")),
  suggestions = ref<Record<string, string[]>>({});
const filters = ref<Record<string, string>>({});
const includeUsed = ref(false);
let sequence = 0;
const activeFilters = computed(
  () =>
    ["material", "location", "color", "tag", "min_width", "min_length"].filter(
      (k) => route.query[k],
    ).length,
);
const status = computed(() => String(route.query.status || "stock"));
async function load(more = false) {
  const seq = ++sequence;
  loading.value = true;
  error.value = "";
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(route.query))
    if (typeof v === "string") p.set(k, v);
  p.set("status", trash.value ? "all" : status.value);
  if (trash.value) p.set("trash", "1");
  if (more) p.set("offset", String(data.value.items.length));
  try {
    const result = await request<FabricList>("fabrics?" + p);
    if (seq !== sequence) return;
    data.value = {
      ...result,
      items: more ? [...data.value.items, ...result.items] : result.items,
    };
  } catch (e) {
    if (seq === sequence) error.value = (e as Error).message;
  } finally {
    if (seq === sequence) loading.value = false;
  }
}
function query(values: Record<string, string>) {
  const q: Record<string, string> = {};
  for (const [k, v] of Object.entries({ ...route.query, ...values }))
    if (typeof v === "string" && v) q[k] = v;
  router.replace({ path: route.path, query: q });
}
function showFilters() {
  filters.value = {};
  for (const k of [
    "material",
    "location",
    "color",
    "tag",
    "min_width",
    "min_length",
  ])
    filters.value[k] = String(route.query[k] || "");
  filterOpen.value = true;
}
function apply() {
  query(filters.value);
  filterOpen.value = false;
}
async function restore(f: Fabric) {
  try {
    await write(
      "fabrics/" + f.id + "/restore",
      "POST",
      { revision: f.revision },
      key(),
    );
    toast("布料已恢复");
    await load();
  } catch (e) {
    error.value = (e as Error).message;
  }
}
function download(format: string) {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(route.query))
    if (typeof v === "string") p.set(k, v);
  p.set("format", format);
  p.set("status", includeUsed.value ? "all" : "stock");
  const a = document.createElement("a");
  a.href = base + "api/export?" + p;
  a.download = "";
  a.click();
  exportOpen.value = false;
  toast("正在准备导出文件，请留意浏览器下载");
}
watch(
  () => route.fullPath,
  () => {
    search.value = String(route.query.q || "");
    load();
  },
  { immediate: true },
);
onMounted(async () => {
  try {
    suggestions.value = await request("suggestions");
  } catch {}
});
</script>
<template>
  <div class="library page">
    <div class="page-heading">
      <div>
        <div class="eyebrow">
          {{ trash ? "RECYCLE BIN" : "YOUR FABRIC COLLECTION" }}
        </div>
        <h1>
          {{ trash ? "回收站" : "我的布料" }}<span class="heading-dot"></span>
        </h1>
        <p>
          {{
            trash
              ? "删除的布料保留 30 天，可在这里恢复。"
              : "照片、尺寸和余料，都收在这里。"
          }}
        </p>
      </div>
      <div class="heading-actions">
        <button
          v-if="!trash"
          class="icon-button mobile-export"
          aria-label="导出布料库"
          @click="exportOpen = true"
        >
          <Icon name="download" /></button
        ><button
          v-if="!trash"
          class="button secondary export-desktop"
          @click="exportOpen = true"
        >
          <Icon name="download" />导出</button
        ><RouterLink v-if="!trash" class="button primary desktop-add" to="/new"
          ><Icon name="plus" />记录新布料</RouterLink
        >
      </div>
    </div>
    <div v-if="!trash" class="overview">
      <div>
        <span class="stat-number">{{ data.stats.total || 0 }}</span
        ><span class="stat-label">已记录布料</span>
      </div>
      <div>
        <span class="stat-number">{{ data.stats.stock || 0 }}</span
        ><span class="stat-label">还在布料库</span>
      </div>
      <div>
        <span class="stat-number">{{ data.stats.incomplete || 0 }}</span
        ><span class="stat-label">待补充资料</span>
      </div>
      <div class="overview-decoration"><Icon name="scissors" :size="43" /></div>
    </div>
    <section class="collection-controls">
      <div class="search-row">
        <form class="search-box" @submit.prevent="query({ q: search })">
          <Icon name="search" /><input
            v-model="search"
            placeholder="搜索名称、材质、收纳位置…"
            aria-label="搜索布料"
          /><button
            v-if="search"
            type="button"
            class="icon-button"
            aria-label="清除搜索"
            @click="
              search = '';
              query({ q: '' });
            "
          >
            <Icon name="close" :size="17" /></button
          ><button class="search-submit" type="submit">搜索</button>
        </form>
        <button
          class="button filter-button"
          :class="{ selected: activeFilters }"
          @click="showFilters"
        >
          <Icon name="filter" />筛选<span
            v-if="activeFilters"
            class="count-badge"
            >{{ activeFilters }}</span
          >
        </button>
      </div>
      <div class="tabs-row">
        <div v-if="!trash" class="status-tabs">
          <button
            v-for="(label, value) in {
              stock: '在库布料',
              unused: '未使用',
              using: '使用中',
              used: '已用完',
              all: '全部',
            }"
            :key="value"
            :class="{ active: status === value }"
            @click="query({ status: value })"
          >
            {{ label }}
          </button>
        </div>
        <span v-else>{{ data.total }} 条记录</span
        ><label class="sort-select"
          ><span class="sr-only">排序</span
          ><select
            :value="route.query.sort || 'purchase'"
            @change="
              query({ sort: ($event.target as HTMLSelectElement).value })
            "
          >
            <option value="purchase">最近购入</option>
            <option value="updated">最近修改</option>
            <option value="name">名称排序</option>
          </select></label
        >
      </div>
    </section>
    <div v-if="activeFilters" class="active-filters">
      <span>已应用 {{ activeFilters }} 项筛选</span
      ><button
        @click="
          query({
            material: '',
            location: '',
            color: '',
            tag: '',
            min_width: '',
            min_length: '',
          })
        "
      >
        清除筛选 <Icon name="close" :size="14" />
      </button>
    </div>
    <div v-if="error" class="error-banner" role="alert">
      {{ error }}<button @click="load()">重试</button>
    </div>
    <div
      v-if="loading && !data.items.length"
      class="fabric-grid"
      aria-label="正在加载"
    >
      <div v-for="i in 6" :key="i" class="skeleton-card">
        <div></div>
        <span></span><span></span>
      </div>
    </div>
    <div v-else-if="!data.items.length" class="empty-state">
      <div class="empty-illustration">
        <div class="fabric-fold fold-one"></div>
        <div class="fabric-fold fold-two"></div>
        <div class="fabric-fold fold-three"></div>
        <span><Icon :name="trash ? 'restore' : 'plus'" :size="24" /></span>
      </div>
      <h2>
        {{
          trash
            ? "这里还没有删除的布料"
            : search || activeFilters
              ? "没有找到匹配的布料"
              : "从第一块布料开始"
        }}
      </h2>
      <p>
        {{
          trash
            ? "布料库里删除的记录会暂存在这里。"
            : search || activeFilters
              ? "试试其他关键词，或减少筛选条件。"
              : "拍一张照片，记下材质和尺寸。下次需要时，一眼就能找到。"
        }}
      </p>
      <RouterLink
        v-if="!trash && !search && !activeFilters"
        class="button primary"
        to="/new"
        ><Icon name="camera" />记录第一块布料</RouterLink
      >
    </div>
    <div v-else class="fabric-grid">
      <article v-for="f in data.items" :key="f.id" class="fabric-card">
        <RouterLink :to="'/fabrics/' + f.id" class="card-link"
          ><div class="card-image" :class="{ 'no-image': !f.photos.length }">
            <img
              v-if="f.photos[0] && !trash"
              :src="mediaURL(f.photos[0].id)"
              :alt="f.name"
              loading="lazy"
              :width="f.photos[0].width"
              :height="f.photos[0].height"
            />
            <div v-else class="no-photo-art">
              <Icon name="image" :size="34" /><span>还没有照片</span>
            </div>
            <span class="status-badge" :class="f.status">{{
              statuses[f.status]
            }}</span
            ><span v-if="f.photos.length > 1" class="photo-count"
              ><Icon name="image" :size="13" />{{ f.photos.length }}</span
            >
          </div>
          <div class="card-body">
            <div class="card-material">
              {{ f.materials.join(" / ") || "材质待补充" }}
            </div>
            <h2>{{ f.name }}</h2>
            <div class="card-meta">
              <Icon name="ruler" :size="15" /><span
                >{{ f.pieces[0] ? dimensions(f.pieces[0]) : "已用完"
                }}<template v-if="f.pieces.length > 1">
                  等 {{ f.pieces.length }} 组</template
                ></span
              >
            </div>
            <div class="card-meta">
              <Icon name="pin" :size="15" /><span>{{
                f.location || "未填写收纳位置"
              }}</span>
            </div>
            <div v-if="f.tags.length" class="card-tags">
              <span v-for="tag in f.tags.slice(0, 2)" :key="tag">{{ tag }}</span
              ><span v-if="f.tags.length > 2">+{{ f.tags.length - 2 }}</span>
            </div>
          </div></RouterLink
        ><button v-if="trash" class="restore-button" @click="restore(f)">
          <Icon name="restore" :size="17" />恢复布料
        </button>
      </article>
    </div>
    <div v-if="data.items.length" class="list-footer">
      <span>已显示 {{ data.items.length }} / {{ data.total }} 条</span
      ><button
        v-if="data.items.length < data.total"
        class="button secondary"
        :disabled="loading"
        @click="load(true)"
      >
        {{ loading ? "加载中…" : "加载更多" }}
      </button>
    </div>
    <RouterLink v-if="!trash" class="mobile-add" to="/new"
      ><Icon name="plus" />新增布料</RouterLink
    >
    <Modal v-if="filterOpen" title="筛选布料" @close="filterOpen = false"
      ><div class="filter-form">
        <label
          >材质<select v-model="filters.material">
            <option value="">全部材质</option>
            <option v-for="m in suggestions.materials" :key="m">{{ m }}</option>
          </select></label
        ><label
          >收纳位置<select v-model="filters.location">
            <option value="">全部位置</option>
            <option v-for="m in suggestions.location" :key="m">{{ m }}</option>
          </select></label
        >
        <div class="field-row">
          <label
            >颜色<select v-model="filters.color">
              <option value="">全部颜色</option>
              <option v-for="m in suggestions.color" :key="m">{{ m }}</option>
            </select></label
          ><label
            >标签<select v-model="filters.tag">
              <option value="">全部标签</option>
              <option v-for="m in suggestions.tags" :key="m">{{ m }}</option>
            </select></label
          >
        </div>
        <div class="field-row">
          <label
            >最小幅宽 / cm<input
              v-model="filters.min_width"
              inputmode="decimal"
              placeholder="不限" /></label
          ><label
            >最小长度 / cm<input
              v-model="filters.min_length"
              inputmode="decimal"
              placeholder="不限"
          /></label>
        </div>
        <p class="field-hint">
          尺寸筛选仅匹配已测量的完整矩形布片，不自动旋转。
        </p>
        <div class="modal-actions">
          <button
            class="button secondary"
            @click="Object.keys(filters).forEach((k) => (filters[k] = ''))"
          >
            重置</button
          ><button class="button primary" @click="apply">查看结果</button>
        </div>
      </div></Modal
    >
    <Modal v-if="exportOpen" title="导出布料库" @close="exportOpen = false"
      ><div class="filter-form">
        <p class="muted">
          按当前搜索与筛选条件导出。照片包包含处理后的照片和完整记录。
        </p>
        <label class="check-label"
          ><input
            type="checkbox"
            v-model="includeUsed"
          />包含已用完的布料</label
        ><button class="export-option" @click="download('csv')">
          <Icon name="grid" /><span
            ><strong>表格 CSV</strong
            ><small>适合在 Excel 中查看和整理</small></span
          ><Icon name="download" /></button
        ><button class="export-option" @click="download('zip')">
          <Icon name="image" /><span
            ><strong>照片与记录 ZIP</strong
            ><small>标准照片与 JSON 数据清单</small></span
          ><Icon name="download" />
        </button></div
    ></Modal>
  </div>
</template>

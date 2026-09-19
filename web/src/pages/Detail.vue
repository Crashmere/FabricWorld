<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { request, write, key, toast, mediaURL } from "../api";
import {
  dateText,
  dimensions,
  materialText,
  statuses,
  type Fabric,
  type Change,
} from "../types";
import Icon from "../components/Icon.vue";
import Modal from "../components/Modal.vue";
const route = useRoute(),
  router = useRouter(),
  fabric = ref<Fabric>(),
  error = ref(""),
  loading = ref(true),
  busy = ref(false),
  selected = ref(0),
  lightbox = ref(false),
  confirmDelete = ref(false),
  history = ref<Change[]>([]),
  showHistory = ref(false);
async function load() {
  loading.value = true;
  try {
    fabric.value = await request("fabrics/" + route.params.id);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}
async function remove() {
  if (!fabric.value || busy.value) return;
  busy.value = true;
  try {
    await write(
      "fabrics/" + fabric.value.id,
      "DELETE",
      { revision: fabric.value.revision },
      key(),
    );
    toast("已移入回收站，可在 30 天内恢复");
    await router.replace("/");
  } catch (e) {
    error.value = (e as Error).message;
    confirmDelete.value = false;
  } finally {
    busy.value = false;
  }
}
async function restore() {
  if (!fabric.value || busy.value) return;
  busy.value = true;
  try {
    fabric.value = await write(
      "fabrics/" + fabric.value.id + "/restore",
      "POST",
      { revision: fabric.value.revision },
      key(),
    );
    toast("布料已恢复");
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}
async function changes() {
  showHistory.value = !showHistory.value;
  if (showHistory.value) {
    try {
      history.value = await request("fabrics/" + route.params.id + "/changes");
    } catch (e) {
      error.value = (e as Error).message;
    }
  }
}
onMounted(load);
</script>
<template>
  <div class="page detail-page">
    <RouterLink class="back-link" :to="fabric?.deletedAt ? '/trash' : '/'"
      ><Icon name="back" />返回{{
        fabric?.deletedAt ? "回收站" : "布料库"
      }}</RouterLink
    >
    <div v-if="error" class="error-banner" role="alert">
      {{ error }}<button @click="load">刷新</button>
    </div>
    <p v-if="loading" class="loading-text">正在展开这块布料…</p>
    <template v-else-if="fabric"
      ><div v-if="fabric.deletedAt" class="info-banner">
        这条布料已在 {{ dateText(fabric.deletedAt) }} 移入回收站。<button
          :disabled="busy"
          @click="restore"
        >
          恢复布料
        </button>
      </div>
      <div class="detail-layout">
        <section class="gallery">
          <button
            v-if="fabric.photos[selected] && !fabric.deletedAt"
            class="main-photo"
            @click="lightbox = true"
            aria-label="查看照片大图"
          >
            <img
              :src="mediaURL(fabric.photos[selected]!.id, 'main')"
              :alt="fabric.name"
            /><span><Icon name="search" :size="16" />查看大图</span>
          </button>
          <div v-else class="main-photo no-photo-art">
            <Icon name="image" :size="52" /><span>{{
              fabric.deletedAt ? "恢复后可查看照片" : "还没有照片"
            }}</span>
          </div>
          <div
            v-if="fabric.photos.length > 1 && !fabric.deletedAt"
            class="gallery-thumbs"
          >
            <button
              v-for="(p, i) in fabric.photos"
              :key="p.id"
              :class="{ selected: i === selected }"
              :aria-label="'查看第 ' + (i + 1) + ' 张照片'"
              @click="selected = i"
            >
              <img :src="mediaURL(p.id)" :alt="'照片 ' + (i + 1)" />
            </button>
          </div>
        </section>
        <section class="detail-info">
          <div class="detail-title">
            <span class="status-badge" :class="fabric.status">{{
              statuses[fabric.status]
            }}</span>
            <h1>{{ fabric.name }}</h1>
            <div class="detail-materials">
              {{ materialText(fabric) || "材质待补充" }}
            </div>
            <p v-if="fabric.composition" class="muted">
              {{ fabric.composition }}
            </p>
          </div>
          <div v-if="!fabric.deletedAt" class="detail-actions">
            <RouterLink
              :to="'/fabrics/' + fabric.id + '/edit'"
              class="button primary"
              ><Icon name="edit" />编辑资料</RouterLink
            >
          </div>
          <div class="detail-block">
            <div class="stock-heading">
              <h2><Icon name="ruler" />剩余尺寸</h2>
              <RouterLink
                v-if="!fabric.deletedAt && fabric.status !== 'used'"
                :to="'/fabrics/' + fabric.id + '/remnant'"
                class="button secondary"
                ><Icon name="scissors" :size="16" />更新余料</RouterLink
              >
            </div>
            <p v-if="!fabric.pieces.length" class="muted">这块布料已经用完。</p>
            <div v-for="(p, i) in fabric.pieces" :key="i" class="piece-summary">
              <span class="piece-index">{{
                String(i + 1).padStart(2, "0")
              }}</span>
              <div>
                <strong>{{ dimensions(p) }}</strong>
                <p v-if="p.note">{{ p.note }}</p>
              </div>
            </div>
          </div>
          <div class="detail-block">
            <h2><Icon name="pin" />收纳位置</h2>
            <p>{{ fabric.location || "还没有填写收纳位置" }}</p>
          </div>
          <div v-if="fabric.color || fabric.tags.length" class="detail-block">
            <h2>颜色与标签</h2>
            <div class="detail-tags">
              <span v-if="fabric.color">{{ fabric.color }}</span
              ><span v-for="tag in fabric.tags" :key="tag">{{ tag }}</span>
            </div>
          </div>
          <div
            v-if="fabric.purchaseDate || fabric.shop || fabric.price"
            class="detail-block"
          >
            <h2>购买信息</h2>
            <dl class="purchase-info">
              <template v-if="fabric.purchaseDate"
                ><dt>日期</dt>
                <dd>{{ fabric.purchaseDate }}</dd></template
              ><template v-if="fabric.shop"
                ><dt>店铺</dt>
                <dd>{{ fabric.shop }}</dd></template
              ><template v-if="fabric.price"
                ><dt>总价</dt>
                <dd>¥ {{ fabric.price }}</dd></template
              >
            </dl>
          </div>
          <div v-if="fabric.notes" class="detail-block">
            <h2>备注</h2>
            <p class="notes">{{ fabric.notes }}</p>
          </div>
          <div class="detail-bottom">
            <button class="text-button" @click="changes">
              <Icon name="restore" :size="17" />{{
                showHistory ? "收起修改历史" : "查看修改历史"
              }}
            </button>
            <div v-if="!fabric.deletedAt">
              <RouterLink
                class="icon-button"
                :to="'/new?copy=' + fabric.id"
                aria-label="复制信息新建布料"
                ><Icon name="copy" /></RouterLink
              ><button
                class="icon-button danger-text"
                aria-label="删除布料"
                @click="confirmDelete = true"
              >
                <Icon name="trash" />
              </button>
            </div>
          </div>
          <p class="record-time">
            录入于 {{ dateText(fabric.createdAt) }} · 最近修改
            {{ dateText(fabric.updatedAt) }}
          </p>
        </section>
      </div>
      <section v-if="showHistory" class="history-panel">
        <h2>修改历史</h2>
        <div v-for="h in history" :key="h.revision" class="history-item">
          <span class="history-dot"></span>
          <div>
            <strong>{{
              h.revision === 1
                ? "首次记录"
                : h.action === "delete"
                  ? "移入回收站"
                  : h.action === "restore"
                    ? "恢复布料"
                    : h.action === "remnant"
                      ? "更新余料"
                      : "编辑记录"
            }}</strong
            ><time>{{ dateText(h.at) }}</time>
            <details>
              <summary>查看当时的信息</summary>
              <p>
                {{ h.after?.name }} ·
                {{ h.after ? materialText(h.after) || "材质待补充" : "材质待补充" }}
              </p>
              <p>
                {{ h.after?.pieces.map(dimensions).join("；") || "已用完" }}
              </p>
              <p v-if="h.before">
                修改前尺寸：{{
                  h.before.pieces.map(dimensions).join("；") || "已用完"
                }}
              </p>
              <p v-if="h.after?.location">收纳位置：{{ h.after.location }}</p>
            </details>
          </div>
        </div>
      </section>
      <Modal
        v-if="confirmDelete"
        title="移入回收站？"
        @close="!busy && (confirmDelete = false)"
        ><div class="filter-form">
          <p>“{{ fabric.name }}”及其照片会保留 30 天，你可以在回收站恢复。</p>
          <div class="modal-actions">
            <button
              class="button secondary"
              :disabled="busy"
              @click="confirmDelete = false"
            >
              保留布料</button
            ><button class="button danger" :disabled="busy" @click="remove">
              {{ busy ? "处理中…" : "移入回收站" }}
            </button>
          </div>
        </div></Modal
      ><Modal v-if="lightbox" :title="fabric.name" @close="lightbox = false"
        ><img
          class="lightbox-image"
          :src="mediaURL(fabric.photos[selected]!.id, 'main')"
          :alt="fabric.name" />
        <div class="lightbox-nav">
          <button
            class="icon-button"
            :disabled="selected === 0"
            aria-label="上一张"
            @click="selected--"
          >
            <Icon name="back" /></button
          ><span>{{ selected + 1 }} / {{ fabric.photos.length }}</span
          ><button
            class="icon-button"
            :disabled="selected === fabric.photos.length - 1"
            aria-label="下一张"
            @click="selected++"
          >
            <Icon name="chevron" />
          </button></div></Modal
    ></template>
  </div>
</template>

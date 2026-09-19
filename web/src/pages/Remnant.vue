<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { onBeforeRouteLeave, useRoute, useRouter } from "vue-router";
import { APIError, key, mediaURL, request, toast, write } from "../api";
import { dimensions, materialText, newPiece, type Fabric, type Piece } from "../types";
import Icon from "../components/Icon.vue";
import Modal from "../components/Modal.vue";
import PieceFields from "../components/PieceFields.vue";
const route = useRoute(),
  router = useRouter();
const id = String(route.params.id);
const draftKey = "fabricworld:remnant:" + id;
const fabric = ref<Fabric>();
const pieces = ref<Piece[]>([]);
const used = ref(false),
  saving = ref(false),
  loading = ref(true);
const error = ref(""),
  info = ref(""),
  initialized = ref(false);
const uncertain = ref(false),
  confirmUsed = ref(false);
const errorBox = ref<HTMLElement>();
let revision = 0,
  opKey = key(),
  lastBody = "",
  initial = "",
  saved = false;
const snapshot = () =>
  JSON.stringify({ pieces: pieces.value, used: used.value });
const dirty = computed(() => initialized.value && snapshot() !== initial);
function persist() {
  if (!initialized.value || saved) return;
  try {
    sessionStorage.setItem(
      draftKey,
      JSON.stringify({
        pieces: pieces.value,
        used: used.value,
        revision,
        opKey,
        lastBody,
        uncertain: uncertain.value,
        at: Date.now(),
      }),
    );
  } catch {}
}
watch([pieces, used], persist, { deep: true });
async function complete(result: Fabric) {
  saved = true;
  uncertain.value = false;
  sessionStorage.removeItem(draftKey);
  toast(result.status === "used" ? "已标记为用完" : "剩余尺寸已更新");
  await router.replace("/fabrics/" + result.id);
}
function showError(e: unknown) {
  error.value = (e as Error).message;
  requestAnimationFrame(() =>
    errorBox.value?.scrollIntoView({ block: "center", behavior: "smooth" }),
  );
}
async function resolve() {
  if (saving.value) return;
  saving.value = true;
  try {
    await complete(await request<Fabric>("operations/" + opKey));
  } catch (e) {
    showError(new Error((e as Error).message + "。可以按原内容重试保存。"));
  } finally {
    saving.value = false;
    persist();
  }
}
function submit() {
  if (saving.value || !initialized.value) return;
  if (used.value) confirmUsed.value = true;
  else save();
}
async function save() {
  if (saving.value || !initialized.value) return;
  confirmUsed.value = false;
  error.value = "";
  const body = {
    revision,
    action: "remnant",
    status: used.value ? "used" : "using",
    pieces: used.value ? [] : pieces.value,
  };
  const text = JSON.stringify(body);
  if (lastBody && lastBody !== text) {
    if (uncertain.value) {
      showError(new Error("上次提交结果尚未确认，请先查询上次结果。"));
      return;
    }
    opKey = key();
  }
  lastBody = text;
  saving.value = true;
  uncertain.value = true;
  persist();
  try {
    await complete(await write<Fabric>("fabrics/" + id, "PUT", body, opKey));
  } catch (e) {
    uncertain.value =
      !(e instanceof APIError) || e.status === 0 || e.status >= 500;
    if (!uncertain.value) opKey = key();
    showError(e);
  } finally {
    saving.value = false;
    persist();
  }
}
function beforeUnload(e: BeforeUnloadEvent) {
  persist();
  if (dirty.value || saving.value || uncertain.value) e.preventDefault();
}
onBeforeRouteLeave(() => {
  if (saved) return true;
  if (saving.value) return false;
  if (dirty.value || uncertain.value)
    return window.confirm("离开余料更新？输入已暂存在本次浏览会话中。");
  return true;
});
onMounted(async () => {
  window.addEventListener("beforeunload", beforeUnload);
  try {
    fabric.value = await request<Fabric>("fabrics/" + id);
    if (fabric.value.deletedAt) throw new Error("请先从回收站恢复这条布料。");
    revision = fabric.value.revision;
    pieces.value = fabric.value.pieces.length
      ? structuredClone(fabric.value.pieces.map((p) => ({ ...p })))
      : [newPiece()];
    used.value = fabric.value.status === "used";
    initial = snapshot();
    let cached;
    try {
      cached = JSON.parse(sessionStorage.getItem(draftKey) || "null");
    } catch {}
    if (cached && Date.now() - cached.at < 24 * 60 * 60 * 1000) {
      if (cached.uncertain || cached.revision === revision) {
        pieces.value = cached.pieces;
        used.value = cached.used;
        revision = cached.revision;
        opKey = cached.opKey;
        lastBody = cached.lastBody;
        uncertain.value = cached.uncertain;
        info.value = "已恢复本次浏览会话中的余料草稿。";
      } else {
        info.value = "布料已在另一处修改，已加载最新尺寸。";
        sessionStorage.removeItem(draftKey);
      }
    }
    initialized.value = true;
    if (uncertain.value) await resolve();
  } catch (e) {
    showError(e);
  } finally {
    loading.value = false;
  }
});
onUnmounted(() => window.removeEventListener("beforeunload", beforeUnload));
</script>
<template>
  <div class="page editor-page remnant-page">
    <RouterLink class="back-link" :to="'/fabrics/' + id"
      ><Icon name="back" />返回布料详情</RouterLink
    >
    <div class="page-heading compact">
      <div>
        <h1>更新余料</h1>
        <p>用过这块布料后，记下现在还剩多少。</p>
      </div>
    </div>
    <p v-if="info" class="info-banner">{{ info }}</p>
    <div v-if="error" ref="errorBox" class="error-banner" role="alert">
      {{ error }}
    </div>
    <div v-if="uncertain && !loading" class="info-banner">
      上次保存结果待确认。<button :disabled="saving" @click="resolve">
        查询上次结果
      </button>
    </div>
    <p v-if="loading" class="loading-text">正在加载布料…</p>
    <template v-else-if="fabric && initialized">
      <div class="remnant-context">
        <img
          v-if="fabric.photos[0]"
          :src="mediaURL(fabric.photos[0].id)"
          alt=""
          width="56"
          height="56"
        />
        <span v-else class="remnant-context-icon"
          ><Icon name="scissors" :size="24"
        /></span>
        <div>
          <strong>{{ fabric.name }}</strong>
          <p>{{ materialText(fabric) || "材质待补充" }}</p>
        </div>
      </div>
      <form id="remnant-form" @submit.prevent="submit">
        <fieldset :disabled="saving" class="form-panel">
          <legend class="sr-only">剩余布料</legend>
          <div class="stock-choice" role="radiogroup" aria-label="使用后的布料">
            <label :class="{ selected: !used }"
              ><input
                v-model="used"
                type="radio"
                :value="false"
                name="stock"
              />还有剩余</label
            >
            <label :class="{ selected: used }"
              ><input
                v-model="used"
                type="radio"
                :value="true"
                name="stock"
              />已经用完</label
            >
          </div>
          <div v-if="used" class="used-explanation">
            <Icon name="check" :size="28" /><strong>没有剩余布片了</strong>
            <p>保存后标记为“已用完”，以前的尺寸保留在修改历史中。</p>
          </div>
          <template v-else>
            <div class="remnant-instructions">
              <h2>现在剩下的尺寸</h2>
              <p>填写剩余尺寸，多块布片可分组记录。</p>
            </div>
            <PieceFields v-model="pieces" />
            <details class="previous-dimensions">
              <summary>查看上次记录</summary>
              <p v-for="(p, i) in fabric.pieces" :key="i">
                {{ dimensions(p) }}
              </p>
              <p v-if="!fabric.pieces.length">上次标记为已用完。</p>
            </details>
          </template>
        </fieldset>
      </form>
      <div class="save-bar">
        <span>{{ used ? "保存前会再次确认" : "保存后标记为使用中" }}</span
        ><button
          class="button primary"
          type="submit"
          form="remnant-form"
          :disabled="saving"
        >
          <Icon v-if="!saving" name="check" />{{
            saving ? "正在保存…" : used ? "标记已用完" : "保存余料"
          }}
        </button>
      </div>
    </template>
    <Modal
      v-if="confirmUsed"
      title="这块布料已经用完？"
      @close="confirmUsed = false"
    >
      <p>
        “{{
          fabric?.name
        }}”将不再显示在在库布料中。照片、资料和修改历史都会保留。
      </p>
      <div class="modal-actions">
        <button class="button secondary" @click="confirmUsed = false">
          继续修改</button
        ><button class="button primary" @click="save">确认已用完</button>
      </div>
    </Modal>
  </div>
</template>

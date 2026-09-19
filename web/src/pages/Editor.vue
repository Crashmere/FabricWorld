<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter, onBeforeRouteLeave } from "vue-router";
import { APIError, key, mediaURL, request, toast, upload, write } from "../api";
import {
  materials,
  newFabric,
  newPiece,
  statuses,
  type Fabric,
  type Photo,
} from "../types";
import Icon from "../components/Icon.vue";
import PieceFields from "../components/PieceFields.vue";
const route = useRoute(),
  router = useRouter(),
  editing = Boolean(route.params.id);
const f = ref<Fabric>(newFabric()),
  loading = ref(editing),
  saving = ref(false),
  error = ref(""),
  errorField = ref(""),
  customMaterial = ref(""),
  tags = ref(""),
  suggestions = ref<Record<string, string[]>>({}),
  draftRecovered = ref(false),
  initialized = ref(false),
  saved = ref(false);
const draftKey = "fabricworld:draft:" + (route.params.id || "new");
let initial = "",
  opKey = key(),
  lastBody = "",
  uncertain = false;
type Pending = {
  localId: string;
  file: File;
  preview: string;
  progress: number;
  error: string;
  active: boolean;
  key: string;
};
const pending = ref<Pending[]>([]),
  camera = ref<HTMLInputElement>(),
  album = ref<HTMLInputElement>();
const uploading = computed(() => pending.value.some((p) => p.active));
const dirty = computed(
  () => initialized.value && JSON.stringify(f.value) !== initial,
);
function toggleMaterial(m: string) {
  const i = f.value.materials.indexOf(m);
  if (i >= 0) f.value.materials.splice(i, 1);
  else if (f.value.materials.length < 20) f.value.materials.push(m);
}
function addMaterial() {
  const m = customMaterial.value.trim();
  if (m && !f.value.materials.includes(m) && f.value.materials.length < 20)
    f.value.materials.push(m);
  customMaterial.value = "";
}
async function processQueue() {
  if (uploading.value) return;
  while (true) {
    const p = pending.value.find((x) => !x.error && !x.active);
    if (!p) break;
    p.active = true;
    try {
      const photo = await upload(p.file, p.key, (n) => (p.progress = n));
      f.value.photos.push(photo);
      f.value.photoIds.push(photo.id);
      URL.revokeObjectURL(p.preview);
      pending.value = pending.value.filter((x) => x.localId !== p.localId);
    } catch (e) {
      p.error = (e as Error).message;
    } finally {
      p.active = false;
    }
  }
}
function files(list: FileList | File[] | null) {
  if (!list) return;
  for (const file of Array.from(list)) {
    if (f.value.photos.length + pending.value.length >= 10) {
      error.value = "每条最多 10 张照片";
      break;
    }
    if (file.size > 25 * 1024 * 1024) {
      error.value = file.name + " 超过 25 MB，请选择较小的照片";
      continue;
    }
    pending.value.push({
      localId: key(),
      file,
      preview: URL.createObjectURL(file),
      progress: 0,
      error: "",
      active: false,
      key: key(),
    });
  }
  processQueue();
}
function selected(e: Event) {
  const input = e.target as HTMLInputElement;
  files(input.files);
  input.value = "";
}
function retry(p: Pending) {
  p.error = "";
  processQueue();
}
function removePending(p: Pending) {
  if (p.active) return;
  URL.revokeObjectURL(p.preview);
  pending.value = pending.value.filter((x) => x !== p);
}
function movePhoto(i: number, delta: number) {
  const j = i + delta;
  if (j < 0 || j >= f.value.photos.length) return;
  const copy = [...f.value.photos];
  [copy[i], copy[j]] = [copy[j]!, copy[i]!];
  f.value.photos = copy;
  f.value.photoIds = copy.map((x) => x.id);
}
function removePhoto(id: string) {
  f.value.photos = f.value.photos.filter((p) => p.id !== id);
  f.value.photoIds = f.value.photos.map((p) => p.id);
}
function changeStatus(e: Event) {
  const status = (e.target as HTMLSelectElement).value as Fabric["status"];
  if (
    status === "used" &&
    f.value.pieces.length &&
    !window.confirm("标记已用完后，当前剩余尺寸会归零，修改历史仍会保留。")
  ) {
    (e.target as HTMLSelectElement).value = f.value.status;
    return;
  }
  f.value.status = status;
  if (status === "used") f.value.pieces = [];
  else if (!f.value.pieces.length) f.value.pieces = [newPiece()];
}
async function save() {
  if (saving.value || loading.value) return;
  error.value = "";
  errorField.value = "";
  if (pending.value.length) {
    error.value = "请等待照片上传完成，或重试 / 移除失败的照片";
    return;
  }
  f.value.tags = tags.value
    .split(/[，,、]/)
    .map((x) => x.trim())
    .filter(Boolean);
  const body = { ...f.value, action: "save" };
  const text = JSON.stringify(body);
  if (lastBody && lastBody !== text) {
    if (uncertain) {
      error.value = "上次提交结果尚未确认，请先重新查询，避免重复保存。";
      return;
    }
    opKey = key();
  }
  lastBody = text;
  saving.value = true;
  uncertain = true;
  // Persist before sending: a reload can happen after the server commits but
  // before the response arrives. Keep the original key until it is resolved.
  persist();
  try {
    const result = await write<Fabric>(
      editing ? "fabrics/" + route.params.id : "fabrics",
      editing ? "PUT" : "POST",
      body,
      opKey,
    );
    uncertain = false;
    saved.value = true;
    sessionStorage.removeItem(draftKey);
    toast(editing ? "修改已保存" : "新布料已记录");
    await router.replace("/fabrics/" + result.id);
  } catch (e) {
    error.value = (e as Error).message;
    if (e instanceof APIError) {
      errorField.value = e.field;
      uncertain = e.status === 0 || e.status >= 500;
    }
    if (!uncertain) opKey = key();
  } finally {
    saving.value = false;
    persist();
  }
}
async function resolve() {
  saving.value = true;
  try {
    const result = await request<Fabric>("operations/" + opKey);
    saved.value = true;
    sessionStorage.removeItem(draftKey);
    await router.replace("/fabrics/" + result.id);
  } catch (e) {
    error.value = (e as Error).message + "。可以按原内容重试保存。";
  } finally {
    saving.value = false;
  }
}
function persist() {
  if (!initialized.value || saved.value) return;
  try {
    sessionStorage.setItem(
      draftKey,
      JSON.stringify({
        fabric: f.value,
        tags: tags.value,
        operationKey: opKey,
        lastBody,
        uncertain,
        at: Date.now(),
      }),
    );
  } catch {}
}
watch([f, tags], persist, { deep: true });
function beforeUnload(e: BeforeUnloadEvent) {
  persist();
  if (dirty.value || pending.value.length || saving.value) {
    e.preventDefault();
  }
}
onBeforeRouteLeave(() => {
  if (saved.value) return true;
  if (saving.value) return false;
  if (dirty.value || pending.value.length)
    return window.confirm(
      "离开编辑页面？已输入的文字暂存在本次浏览会话中，未上传的照片需要重新选择。",
    );
  return true;
});
onMounted(async () => {
  window.addEventListener("beforeunload", beforeUnload);
  try {
    if (editing) f.value = await request<Fabric>("fabrics/" + route.params.id);
    if (f.value.deletedAt) {
      error.value = "请先从回收站恢复这条布料";
      return;
    }
    if (route.query.copy) {
      const source = await request<Fabric>("fabrics/" + route.query.copy);
      f.value = {
        ...newFabric(),
        name: source.name,
        materials: source.materials,
        composition: source.composition,
        color: source.color,
        tags: source.tags,
        location: source.location,
        notes: source.notes,
      };
    }
    initial = JSON.stringify(f.value);
    tags.value = f.value.tags.join("，");
    try {
      const cached = JSON.parse(sessionStorage.getItem(draftKey) || "null");
      if (
        cached &&
        Date.now() - cached.at < 24 * 60 * 60 * 1000 &&
        (!editing || cached.fabric.revision === f.value.revision)
      ) {
        f.value = cached.fabric;
        tags.value = cached.tags;
        opKey = cached.operationKey || key();
        lastBody = cached.lastBody || "";
        uncertain = cached.uncertain || false;
        draftRecovered.value = true;
      }
    } catch {}
    initialized.value = true;
    try {
      suggestions.value = await request("suggestions");
    } catch {}
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
});
onUnmounted(() => {
  window.removeEventListener("beforeunload", beforeUnload);
  pending.value.forEach((p) => URL.revokeObjectURL(p.preview));
});
</script>
<template>
  <div class="page editor-page">
    <RouterLink
      class="back-link"
      :to="editing ? '/fabrics/' + route.params.id : '/'"
      ><Icon name="back" />{{
        editing ? "返回布料详情" : "返回布料库"
      }}</RouterLink
    >
    <div class="page-heading compact">
      <div>
        <div class="eyebrow">
          {{ editing ? "EDIT YOUR FABRIC" : "A NEW ADDITION" }}
        </div>
        <h1>
          {{ editing ? "编辑资料" : "记录新布料" }}
        </h1>
        <p>
          {{
            editing
              ? "补充照片、材质、收纳位置和购买信息。"
              : "先拍张照片，其他信息可以慢慢补充。"
          }}
        </p>
      </div>
    </div>
    <p v-if="draftRecovered" class="info-banner">
      已恢复本次浏览会话中的草稿。<button @click="draftRecovered = false">
        知道了
      </button>
    </p>
    <div v-if="error" class="error-banner" role="alert">
      {{ error }}<button v-if="uncertain" @click="resolve">查询上次结果</button>
    </div>
    <p v-if="loading" class="loading-text">正在加载布料…</p>
    <form
      v-else-if="initialized"
      id="fabric-form"
      class="editor-layout"
      @submit.prevent="save"
    >
      <fieldset :disabled="saving" class="photo-column">
        <section class="form-panel">
          <div class="section-title">
            <h2><Icon name="camera" />布料照片</h2>
            <span>{{ f.photos.length }} / 10</span>
          </div>
          <div
            v-if="!f.photos.length && !pending.length"
            class="upload-empty"
            @dragover.prevent
            @drop.prevent="files($event.dataTransfer?.files || null)"
          >
            <div class="upload-icon"><Icon name="image" :size="36" /></div>
            <h3>给这块布料拍张照</h3>
            <p>拍全貌、花纹，或近距离的质感</p>
            <button
              type="button"
              class="button primary"
              @click="camera?.click()"
            >
              <Icon name="camera" />拍照</button
            ><button type="button" class="text-button" @click="album?.click()">
              从相册或文件中选择</button
            ><span class="desktop-hint">也可以将图片拖到这里</span>
          </div>
          <div
            v-else
            class="edit-photos"
            @dragover.prevent
            @drop.prevent="files($event.dataTransfer?.files || null)"
          >
            <div v-for="(p, i) in f.photos" :key="p.id" class="edit-photo">
              <img :src="mediaURL(p.id)" :alt="'布料照片 ' + (i + 1)" /><span
                v-if="i === 0"
                class="cover-label"
                >封面</span
              ><button
                type="button"
                class="photo-remove"
                :aria-label="'移除照片 ' + (i + 1)"
                @click="removePhoto(p.id)"
              >
                <Icon name="close" :size="15" />
              </button>
              <div class="photo-order">
                <button
                  type="button"
                  :disabled="i === 0"
                  :aria-label="'前移照片 ' + (i + 1)"
                  @click="movePhoto(i, -1)"
                >
                  <Icon name="back" :size="16" /></button
                ><button
                  type="button"
                  :disabled="i === f.photos.length - 1"
                  :aria-label="'后移照片 ' + (i + 1)"
                  @click="movePhoto(i, 1)"
                >
                  <Icon name="chevron" :size="16" />
                </button>
              </div>
            </div>
            <div
              v-for="p in pending"
              :key="p.localId"
              class="edit-photo pending-photo"
            >
              <img
                :src="p.preview"
                alt="待上传照片"
                @error="($event.target as HTMLImageElement).style.opacity = '0'"
              />
              <div class="upload-status">
                <span>{{
                  p.error || (p.progress >= 95 ? "正在处理…" : p.progress + "%")
                }}</span
                ><button v-if="p.error" type="button" @click="retry(p)">
                  重试
                </button>
              </div>
              <button
                v-if="!p.active"
                type="button"
                class="photo-remove"
                aria-label="移除待上传照片"
                @click="removePending(p)"
              >
                <Icon name="close" :size="15" />
              </button>
            </div>
          </div>
          <div v-if="f.photos.length || pending.length" class="upload-actions">
            <button
              type="button"
              class="button secondary"
              :disabled="f.photos.length + pending.length >= 10"
              @click="camera?.click()"
            >
              <Icon name="camera" />拍照</button
            ><button
              type="button"
              class="button secondary"
              :disabled="f.photos.length + pending.length >= 10"
              @click="album?.click()"
            >
              <Icon name="plus" />选择照片
            </button>
          </div>
          <input
            ref="camera"
            class="sr-only"
            type="file"
            accept="image/*"
            capture="environment"
            aria-label="拍照上传"
            @change="selected"
          /><input
            ref="album"
            class="sr-only"
            type="file"
            accept="image/jpeg,image/png,image/webp,image/heic,image/heif,.heic,.heif"
            multiple
            aria-label="选择照片文件"
            @change="selected"
          />
          <p class="field-hint">
            支持 JPG、PNG、WebP、HEIC，每张不超过 25 MB。第一张作为封面。
          </p>
        </section>
      </fieldset>
      <fieldset :disabled="saving" class="fields-column">
        <section class="form-panel">
          <div class="section-title">
            <h2><span class="section-number">01</span>基本信息</h2>
          </div>
          <label
            >布料名称<input
              v-model="f.name"
              maxlength="100"
              placeholder="例如：蓝色小花棉布"
              :aria-invalid="errorField === 'name'"
          /></label>
          <div class="field-hint">有照片时可留空，保存后自动命名。</div>
          <div class="field-label">材质</div>
          <div class="material-options">
            <button
              v-for="m in [...new Set([...materials, ...f.materials])]"
              :key="m"
              type="button"
              :class="{ chosen: f.materials.includes(m) }"
              :aria-pressed="f.materials.includes(m)"
              @click="toggleMaterial(m)"
            >
              {{ m
              }}<Icon v-if="f.materials.includes(m)" name="check" :size="13" />
            </button>
          </div>
          <div class="custom-material">
            <input
              v-model="customMaterial"
              maxlength="40"
              placeholder="其他材质"
              aria-label="自定义材质"
              @keydown.enter.prevent="addMaterial"
            /><button
              type="button"
              class="button secondary"
              @click="addMaterial"
            >
              添加
            </button>
          </div>
          <label
            >成分说明 <span class="optional">选填</span
            ><input
              v-model="f.composition"
              maxlength="200"
              placeholder="例如：棉 70% / 麻 30%" /></label
          ><label
            >收纳位置 <span class="optional">选填</span
            ><input
              v-model="f.location"
              list="locations"
              maxlength="160"
              placeholder="例如：衣柜上层 · 2 号箱" /><datalist id="locations">
              <option
                v-for="item in suggestions.location"
                :key="item"
                :value="item"
              /></datalist
          ></label>
        </section>
        <section class="form-panel">
          <details
            :open="
              !editing || errorField === 'pieces' || errorField === 'status'
            "
          >
            <summary>
              <span class="section-number">02</span>
              <h2>{{ editing ? "修正尺寸或状态" : "尺寸与状态" }}</h2>
              <Icon name="chevron" :size="17" />
            </summary>
            <div class="details-fields">
              <p v-if="editing" class="field-hint">
                补测或更正录入信息时在这里修改；使用后记录剩余布片，请从详情页选择“更新余料”。
              </p>
              <label
                >当前状态<select :value="f.status" @change="changeStatus">
                  <option
                    v-for="(label, value) in statuses"
                    :key="value"
                    :value="value"
                  >
                    {{ label }}
                  </option>
                </select></label
              >
              <p v-if="f.status === 'used'" class="field-hint">
                已用完，没有剩余布片。以前的尺寸保留在修改历史中。
              </p>
              <PieceFields v-else v-model="f.pieces" />
            </div>
          </details>
        </section>
        <section class="form-panel">
          <details
            :open="
              Boolean(
                f.color ||
                  tags ||
                  f.purchaseDate ||
                  f.shop ||
                  f.price ||
                  f.notes,
              )
            "
          >
            <summary>
              <span class="section-number">03</span>
              <h2>更多细节</h2>
              <span class="optional">选填</span
              ><Icon name="chevron" :size="17" />
            </summary>
            <div class="details-fields">
              <div class="field-row">
                <label
                  >颜色<input
                    v-model="f.color"
                    maxlength="40"
                    placeholder="例如：雾蓝" /></label
                ><label
                  >标签<input v-model="tags" placeholder="衬衫，花卉，春夏"
                /></label>
              </div>
              <div class="field-row purchase-fields">
                <label
                  >购买日期<span class="date-input">
                    <input
                      v-model="f.purchaseDate"
                      type="date"
                    /> </span></label
                ><label
                  >购买总价 / 元<input
                    v-model="f.price"
                    inputmode="decimal"
                    placeholder="0.00"
                /></label>
              </div>
              <label
                >购买店铺<input
                  v-model="f.shop"
                  maxlength="160"
                  placeholder="在哪里遇见这块布料" /></label
              ><label
                >备注<textarea
                  v-model="f.notes"
                  rows="4"
                  maxlength="4000"
                  placeholder="厚薄、手感、预想用途，或其他想记住的事…"
                />
              </label>
            </div>
          </details>
        </section>
      </fieldset>
    </form>
    <div v-if="initialized" class="save-bar">
      <span>{{
        uploading
          ? "照片上传中…"
          : editing
            ? "修改后请保存"
            : "拍照后即可保存，资料可稍后补充"
      }}</span
      ><button
        class="button primary"
        type="submit"
        form="fabric-form"
        :disabled="saving || Boolean(pending.length)"
      >
        <Icon v-if="!saving" name="check" />{{
          saving ? "正在保存…" : editing ? "保存修改" : "保存布料"
        }}
      </button>
    </div>
  </div>
</template>

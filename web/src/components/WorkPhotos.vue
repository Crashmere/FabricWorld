<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { key, mediaURL, upload } from "../api";
import type { Photo } from "../types";
import Icon from "./Icon.vue";
import PhotoViewer from "./PhotoViewer.vue";
const photos = defineModel<Photo[]>({ required: true });
const pendingCount = defineModel<number>("pending", { required: true });
type Pending = { id: string; file: File; url: string; progress: number; error: string; active: boolean; key: string };
const pending = ref<Pending[]>([]), camera = ref<HTMLInputElement>(), album = ref<HTMLInputElement>();
const error = ref(""), preview = ref<{ id: string; src: string; alt: string }>();
const full = computed(() => photos.value.length + pending.value.length >= 10);
let disposed = false, processing = false;
watch(() => pending.value.length, n => pendingCount.value = n, { flush: "sync" });
async function process() {
  if (processing) return;
  processing = true;
  try {
    while (!disposed) {
      const p = pending.value.find(x => !x.error);
      if (!p) break;
      p.active = true;
      try {
        const result = await upload(p.file, p.key, n => p.progress = n);
        if (disposed) break;
        photos.value = [...photos.value, result];
        if (preview.value?.id === p.id) {
          preview.value = { id: result.id, src: mediaURL(result.id, "main"), alt: p.file.name };
          await nextTick();
        }
        URL.revokeObjectURL(p.url);
        pending.value = pending.value.filter(x => x.id !== p.id);
      } catch (e) { p.error = (e as Error).message; }
      finally { p.active = false; }
    }
  } finally { processing = false; }
}
function files(list: FileList | null) {
  error.value = "";
  for (const file of Array.from(list || [])) {
    if (full.value) { error.value = "每件成品最多 10 张照片"; break; }
    if (file.size > 25 * 1024 * 1024) { error.value = file.name + " 超过 25 MB"; continue; }
    pending.value.push({ id: key(), file, url: URL.createObjectURL(file), progress: 0, error: "", active: false, key: key() });
  }
  void process();
}
function selected(e: Event) { const input = e.target as HTMLInputElement; files(input.files); input.value = ""; }
function removePending(p: Pending) {
  if (p.active) return;
  if (preview.value?.id === p.id) preview.value = undefined;
  URL.revokeObjectURL(p.url);
  pending.value = pending.value.filter(x => x.id !== p.id);
}
function move(i: number, delta: number) {
  const copy = [...photos.value], j = i + delta;
  if (j < 0 || j >= copy.length) return;
  [copy[i], copy[j]] = [copy[j]!, copy[i]!];
  photos.value = copy;
}
function show(id: string, src: string, alt: string, e: MouseEvent) {
  (e.currentTarget as HTMLElement).focus({ preventScroll: true });
  preview.value = { id, src, alt };
}
onUnmounted(() => { disposed = true; pending.value.forEach(p => URL.revokeObjectURL(p.url)); });
</script>
<template>
  <section class="form-panel">
    <div class="section-title"><h2><Icon name="camera" />成品照片</h2><span>{{ photos.length }} / 10</span></div>
    <div v-if="!photos.length && !pending.length" class="upload-empty" @dragover.prevent @drop.prevent="files($event.dataTransfer?.files || null)">
      <div class="upload-icon"><Icon name="scissors" :size="36" /></div>
      <h3>留住完成的这一刻</h3><p>穿着效果、整体造型，或细节与缝线</p>
      <button type="button" class="button primary" @click="camera?.click()"><Icon name="camera" />拍照</button>
      <button type="button" class="text-button" @click="album?.click()">从相册或文件中选择</button>
    </div>
    <div v-else class="edit-photos">
      <div v-for="(p, i) in photos" :key="p.id" class="edit-photo">
        <button type="button" class="photo-preview-trigger" :aria-label="'查看成品照片 ' + (i + 1) + ' 大图'" @click="show(p.id, mediaURL(p.id, 'main'), '成品照片', $event)"><img :src="mediaURL(p.id)" alt="成品照片" /></button>
        <span v-if="i === 0" class="cover-label">封面</span>
        <button type="button" class="photo-remove" :aria-label="'移除照片 ' + (i + 1)" @click="photos = photos.filter(x => x.id !== p.id)"><Icon name="close" :size="15" /></button>
        <div class="photo-order"><button type="button" :disabled="i === 0" :aria-label="'前移照片 ' + (i + 1)" @click="move(i, -1)"><Icon name="back" :size="16" /></button><button type="button" :disabled="i === photos.length - 1" :aria-label="'后移照片 ' + (i + 1)" @click="move(i, 1)"><Icon name="chevron" :size="16" /></button></div>
      </div>
      <div v-for="p in pending" :key="p.id" class="edit-photo pending-photo">
        <button type="button" class="photo-preview-trigger" :aria-label="'查看待上传照片 ' + p.file.name" @click="show(p.id, p.url, p.file.name, $event)"><img :src="p.url" alt="待上传照片" /></button>
        <div class="upload-status"><span>{{ p.error || (p.progress >= 95 ? '正在处理…' : p.progress + '%') }}</span><button v-if="p.error" type="button" @click="p.error = ''; process()">重试</button></div>
        <button v-if="!p.active" type="button" class="photo-remove" aria-label="移除待上传照片" @click="removePending(p)"><Icon name="close" :size="15" /></button>
      </div>
    </div>
    <div v-if="photos.length || pending.length" class="upload-actions"><button type="button" class="button secondary" :disabled="full" @click="camera?.click()"><Icon name="camera" />拍照</button><button type="button" class="button secondary" :disabled="full" @click="album?.click()"><Icon name="plus" />选择照片</button></div>
    <input ref="camera" class="sr-only" type="file" accept="image/*" capture="environment" aria-label="拍照上传" @change="selected" />
    <input ref="album" class="sr-only" type="file" accept="image/jpeg,image/png,image/webp,image/heic,image/heif,.heic,.heif" multiple aria-label="选择照片文件" @change="selected" />
    <p class="field-hint">最多 10 张，每张不超过 25 MB。第一张作为封面，也可以稍后补照片。</p>
    <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
    <PhotoViewer v-if="preview" :src="preview.src" :alt="preview.alt" @close="preview = undefined" />
  </section>
</template>

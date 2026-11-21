<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { APIError, key, mediaURL, toast, write, type RotationResult } from "../api";
import Icon from "./Icon.vue";
import Modal from "./Modal.vue";

const props = withDefaults(defineProps<{
  src: string; alt: string; title?: string; photoId?: string; revision?: number; draft?: boolean; rotateDisabled?: boolean;
}>(), { title: "照片预览", revision: 0 });
const emit = defineEmits<{ close: []; rotated: [result: RotationResult] }>();
const stage = ref<HTMLElement>();
const natural = ref({ width: 0, height: 0 });
const viewport = ref({ width: 0, height: 0 });
const scale = ref(1), x = ref(0), y = ref(0), failed = ref(false), dragging = ref(false);
const turns = ref(0), saving = ref(false), saveError = ref("");
let operation: { key: string; turns: number; revision: number; draft: boolean } | undefined;
let preparedImage: HTMLImageElement | undefined;
const maxScale = 6;
type Point = { x: number; y: number };
const pointers = new Map<number, Point>();
let observer: ResizeObserver | undefined;
const ready = computed(() => !failed.value && natural.value.width > 0 && viewport.value.width > 0);
const fitted = computed(() => {
  if (!ready.value) return { width: 0, height: 0 };
  const width = turns.value % 2 ? natural.value.height : natural.value.width;
  const height = turns.value % 2 ? natural.value.width : natural.value.height;
  const ratio = Math.min(viewport.value.width / width, viewport.value.height / height);
  return { width: natural.value.width * ratio, height: natural.value.height * ratio };
});
const displayed = computed(() => turns.value % 2
  ? { width: fitted.value.height, height: fitted.value.width } : fitted.value);
const imageStyle = computed(() => ({
  width: fitted.value.width + "px",
  height: fitted.value.height + "px",
  visibility: ready.value ? "visible" as const : "hidden" as const,
  transform: `translate(-50%, -50%) translate(${x.value}px, ${y.value}px) rotate(${turns.value * 90}deg) scale(${scale.value})`,
}));
function constrain() {
  const limitX = Math.max(0, (displayed.value.width * scale.value - viewport.value.width) / 2);
  const limitY = Math.max(0, (displayed.value.height * scale.value - viewport.value.height) / 2);
  x.value = Math.max(-limitX, Math.min(limitX, x.value));
  y.value = Math.max(-limitY, Math.min(limitY, y.value));
}
function clearPointers() {
  for (const id of pointers.keys()) {
    if (stage.value?.hasPointerCapture(id)) stage.value.releasePointerCapture(id);
  }
  pointers.clear();
  dragging.value = false;
}
function reset() {
  scale.value = 1;
  x.value = y.value = 0;
  clearPointers();
}
function rotate(delta: number) {
  if (!ready.value || !props.photoId || saving.value || operation || props.rotateDisabled) return;
  turns.value = (turns.value + delta + 4) % 4;
  saveError.value = "";
  reset();
}
function restoreRotation() {
  if (saving.value || operation) return;
  turns.value = 0;
  saveError.value = "";
  reset();
}
function close() {
  if (saving.value) return;
  if (turns.value && !window.confirm(operation
    ? "保存结果尚未确认，关闭后请刷新记录核对方向。仍要关闭？"
    : "照片方向还没有保存，放弃这次旋转？")) return;
  emit("close");
}
async function saveRotation() {
  if (!props.photoId || !turns.value || saving.value || props.rotateDisabled) return;
  operation ??= { key: key(), turns: turns.value, revision: props.revision, draft: Boolean(props.draft) };
  saving.value = true;
  saveError.value = "";
  try {
    const { key: operationKey, ...body } = operation;
    const result = await write<RotationResult>("media/" + props.photoId + "/rotate", "POST", body, operationKey);
    preparedImage = new Image();
    preparedImage.src = mediaURL(result.photo.id, "main");
    // A committed save must not wait indefinitely for its preview download.
    let previewTimeout: ReturnType<typeof setTimeout> | undefined;
    try {
      await Promise.race([preparedImage.decode(), new Promise<never>((_, reject) => {
        previewTimeout = setTimeout(() => reject(new Error("preview timeout")), 3000);
      })]);
    } catch { preparedImage = undefined; }
    finally { clearTimeout(previewTimeout); }
    emit("rotated", result);
    await nextTick();
    operation = undefined;
    toast(props.draft ? "照片方向已调整，保存资料后生效" : "照片方向已保存");
  } catch (e) {
    saveError.value = (e as Error).message;
    if (e instanceof APIError && e.status > 0 && e.status < 500) operation = undefined;
    else saveError.value += "。结果尚未确认，请按原提交重试保存。";
  } finally { saving.value = false; }
}
function measure() {
  if (!stage.value) return;
  viewport.value = { width: stage.value.clientWidth, height: stage.value.clientHeight };
  reset();
}
function loaded(event: Event) {
  const image = event.target as HTMLImageElement;
  natural.value = { width: image.naturalWidth, height: image.naturalHeight };
  measure();
}
function point(event: MouseEvent | PointerEvent): Point {
  const rect = stage.value!.getBoundingClientRect();
  return { x: event.clientX - rect.left - rect.width / 2, y: event.clientY - rect.top - rect.height / 2 };
}
function zoom(next: number, anchor: Point = { x: 0, y: 0 }) {
  if (!ready.value) return;
  const bounded = Math.max(1, Math.min(maxScale, next));
  const ratio = bounded / scale.value;
  x.value = anchor.x - (anchor.x - x.value) * ratio;
  y.value = anchor.y - (anchor.y - y.value) * ratio;
  scale.value = bounded;
  constrain();
}
function wheel(event: WheelEvent) {
  const delta = event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? viewport.value.height : 1);
  zoom(scale.value * Math.exp(-Math.max(-100, Math.min(100, delta)) * 0.005), point(event));
}
function doubleClick(event: MouseEvent) {
  if (scale.value > 1) reset();
  else zoom(2, point(event));
}
function down(event: PointerEvent) {
  if (!ready.value || event.button !== 0 || pointers.size >= 2) return;
  stage.value?.focus({ preventScroll: true });
  pointers.set(event.pointerId, point(event));
  stage.value?.setPointerCapture(event.pointerId);
  dragging.value = true;
}
function midpoint(points: Point[]): Point {
  return { x: (points[0]!.x + points[1]!.x) / 2, y: (points[0]!.y + points[1]!.y) / 2 };
}
function distance(points: Point[]) {
  return Math.hypot(points[0]!.x - points[1]!.x, points[0]!.y - points[1]!.y);
}
function move(event: PointerEvent) {
  const previous = pointers.get(event.pointerId);
  if (!previous) return;
  const before = [...pointers.values()];
  const current = point(event);
  pointers.set(event.pointerId, current);
  if (pointers.size === 2) {
    const after = [...pointers.values()];
    const oldDistance = distance(before);
    if (oldDistance < 1) return;
    const oldCenter = midpoint(before), center = midpoint(after);
    const next = Math.max(1, Math.min(maxScale, scale.value * distance(after) / oldDistance));
    const ratio = next / scale.value;
    x.value = center.x - (oldCenter.x - x.value) * ratio;
    y.value = center.y - (oldCenter.y - y.value) * ratio;
    scale.value = next;
  } else {
    x.value += current.x - previous.x;
    y.value += current.y - previous.y;
  }
  constrain();
}
function up(event: PointerEvent) {
  pointers.delete(event.pointerId);
  if (stage.value?.hasPointerCapture(event.pointerId)) stage.value.releasePointerCapture(event.pointerId);
  dragging.value = pointers.size > 0;
}
function keys(event: KeyboardEvent) {
  if (event.ctrlKey || event.metaKey || event.altKey) return;
  if (["+", "=", "-", "0", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(event.key)) {
    event.preventDefault();
    if (event.key === "+" || event.key === "=") zoom(scale.value * 1.5);
    else if (event.key === "-") zoom(scale.value / 1.5);
    else if (event.key === "0") reset();
    else {
      if (event.key === "ArrowLeft") x.value += 50;
      if (event.key === "ArrowRight") x.value -= 50;
      if (event.key === "ArrowUp") y.value += 50;
      if (event.key === "ArrowDown") y.value -= 50;
      constrain();
    }
  }
}
watch(() => props.src, () => {
  failed.value = false;
  natural.value = preparedImage?.src === new URL(props.src, window.location.href).href
    ? { width: preparedImage.naturalWidth, height: preparedImage.naturalHeight } : { width: 0, height: 0 };
  preparedImage = undefined;
  turns.value = 0;
  operation = undefined;
  saveError.value = "";
  reset();
});
onMounted(() => {
  observer = new ResizeObserver(measure);
  if (stage.value) observer.observe(stage.value);
});
onUnmounted(() => { observer?.disconnect(); clearPointers(); });
</script>

<template>
  <Modal :title="title" photo :busy="saving" @close="close">
    <div ref="stage" class="photo-viewer-stage" :class="{ zoomed: scale > 1, dragging }"
      role="region" aria-label="图片缩放区域" tabindex="0"
      @wheel.prevent="wheel" @dblclick.prevent="doubleClick" @keydown="keys"
      @pointerdown.prevent="down" @pointermove.prevent="move" @pointerup="up"
      @pointercancel="up" @lostpointercapture="up">
      <p v-if="failed" class="photo-viewer-message" role="status">{{ src.startsWith('blob:')
        ? '这张照片暂时无法预览，上传完成后可查看大图。'
        : '照片加载失败，请关闭后重新打开。' }}</p>
      <template v-else>
        <p v-if="!ready" class="photo-viewer-message" role="status">正在加载照片…</p>
        <img :key="src" class="photo-viewer-image" :src="src" :alt="alt" :style="imageStyle"
          draggable="false" @load="loaded" @error="failed = true" />
      </template>
    </div>
    <footer class="photo-viewer-footer">
      <div class="photo-viewer-controls">
        <button type="button" class="icon-button" aria-label="缩小图片" :disabled="!ready || scale <= 1" @click="zoom(scale / 1.5)"><Icon name="minus" /></button>
        <output class="photo-viewer-scale" aria-label="图片缩放比例">{{ Math.round(scale * 100) }}%</output>
        <button type="button" class="icon-button" aria-label="放大图片" :disabled="!ready || scale >= maxScale" @click="zoom(scale * 1.5)"><Icon name="plus" /></button>
        <button type="button" class="photo-viewer-fit" :disabled="!ready" @click="reset">适应窗口</button>
      </div>
      <div class="photo-viewer-rotation">
        <button type="button" class="icon-button" aria-label="向左旋转90°" title="向左旋转90°" :disabled="!ready || !photoId || saving || Boolean(operation) || rotateDisabled" @click="rotate(-1)"><Icon name="rotateLeft" /></button>
        <button type="button" class="icon-button" aria-label="向右旋转90°" title="向右旋转90°" :disabled="!ready || !photoId || saving || Boolean(operation) || rotateDisabled" @click="rotate(1)"><Icon name="rotateRight" /></button>
        <button type="button" class="photo-viewer-fit" :disabled="!turns || saving || Boolean(operation)" @click="restoreRotation">还原方向</button>
        <button type="button" class="photo-viewer-save" :disabled="!turns || saving || rotateDisabled" @click="saveRotation">{{ saving ? '正在保存…' : '保存旋转' }}</button>
      </div>
      <fieldset v-if="$slots.navigation" class="photo-viewer-navigation" :disabled="saving || Boolean(turns)"><slot name="navigation" /></fieldset>
      <p v-if="saveError" class="photo-viewer-error" role="alert">{{ saveError }}</p>
      <p class="photo-viewer-hint">{{ turns ? (draft ? '方向会随本次资料一起保存' : '保存或还原方向后可切换照片') : !photoId ? '上传完成后可旋转保存' : '滚轮或双指缩放 · 放大后拖动查看' }}</p>
    </footer>
  </Modal>
</template>

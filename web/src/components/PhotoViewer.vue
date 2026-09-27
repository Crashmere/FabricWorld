<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import Icon from "./Icon.vue";
import Modal from "./Modal.vue";

const props = withDefaults(defineProps<{ src: string; alt: string; title?: string }>(), { title: "照片预览" });
const emit = defineEmits<{ close: [] }>();
const stage = ref<HTMLElement>();
const natural = ref({ width: 0, height: 0 });
const viewport = ref({ width: 0, height: 0 });
const scale = ref(1), x = ref(0), y = ref(0), failed = ref(false), dragging = ref(false);
const maxScale = 6;
type Point = { x: number; y: number };
const pointers = new Map<number, Point>();
let observer: ResizeObserver | undefined;
const ready = computed(() => !failed.value && natural.value.width > 0 && viewport.value.width > 0);
const fitted = computed(() => {
  if (!ready.value) return { width: 0, height: 0 };
  const ratio = Math.min(viewport.value.width / natural.value.width, viewport.value.height / natural.value.height);
  return { width: natural.value.width * ratio, height: natural.value.height * ratio };
});
const imageStyle = computed(() => ({
  width: fitted.value.width + "px",
  height: fitted.value.height + "px",
  visibility: ready.value ? "visible" as const : "hidden" as const,
  transform: `translate(-50%, -50%) translate(${x.value}px, ${y.value}px) scale(${scale.value})`,
}));
function constrain() {
  const limitX = Math.max(0, (fitted.value.width * scale.value - viewport.value.width) / 2);
  const limitY = Math.max(0, (fitted.value.height * scale.value - viewport.value.height) / 2);
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
  natural.value = { width: 0, height: 0 };
  reset();
});
onMounted(() => {
  observer = new ResizeObserver(measure);
  if (stage.value) observer.observe(stage.value);
});
onUnmounted(() => { observer?.disconnect(); clearPointers(); });
</script>

<template>
  <Modal :title="title" photo @close="emit('close')">
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
      <div v-if="$slots.navigation" class="photo-viewer-navigation"><slot name="navigation" /></div>
      <p class="photo-viewer-hint">滚轮或双指缩放 · 放大后拖动查看</p>
    </footer>
  </Modal>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from "vue";
const props = withDefaults(defineProps<{
  open: boolean;
  anchor?: HTMLElement;
  minWidth?: number;
  maxHeight?: number;
}>(), { minWidth: 180, maxHeight: 320 });
const emit = defineEmits<{ close: [restoreFocus: boolean] }>();
const panel = ref<HTMLElement>();
const placement = ref("below");
const style = ref<Record<string, string>>({});
let observer: ResizeObserver | undefined;
let frame = 0;
function position() {
  if (!props.open || !props.anchor || !panel.value) return;
  const rect = props.anchor.getBoundingClientRect(), viewport = window.visualViewport;
  const x = viewport?.offsetLeft || 0, y = viewport?.offsetTop || 0;
  const width = viewport?.width || innerWidth, height = viewport?.height || innerHeight;
  if (rect.bottom < y || rect.top > y + height || !props.anchor.getClientRects().length) {
    emit("close", false); return;
  }
  const below = y + height - rect.bottom - 18, above = rect.top - y - 18;
  const up = below < props.maxHeight && above > below;
  const available = Math.max(48, Math.min(props.maxHeight, up ? above : below));
  const popupWidth = Math.min(width - 24, Math.max(rect.width, props.minWidth));
  placement.value = up ? "above" : "below";
  style.value = {
    width: popupWidth + "px", maxHeight: available + "px",
    left: Math.max(x + 12, Math.min(rect.left, x + width - popupWidth - 12)) + "px",
    top: up ? "auto" : Math.max(y + 12, rect.bottom + 6) + "px",
    bottom: up ? Math.max(12, innerHeight - rect.top + 6) + "px" : "auto",
  };
}
function schedule(event?: Event) {
  if (event?.target instanceof Node && panel.value?.contains(event.target)) return;
  cancelAnimationFrame(frame); frame = requestAnimationFrame(position);
}
function outside(event: Event) {
  const target = event.target as Node;
  if (!panel.value?.contains(target) && !props.anchor?.contains(target)) emit("close", false);
}
function keys(event: KeyboardEvent) {
  if (event.key === "Escape") {
    event.preventDefault(); event.stopImmediatePropagation(); emit("close", true);
  }
}
function other(event: Event) {
  if ((event as CustomEvent).detail !== panel.value) emit("close", false);
}
function leaving(element: Element) {
  (element as HTMLElement).inert = true;
  element.setAttribute("aria-hidden", "true");
}
function entering(element: Element) {
  (element as HTMLElement).inert = false;
  element.removeAttribute("aria-hidden");
}
function cleanup() {
  cancelAnimationFrame(frame); observer?.disconnect();
  document.removeEventListener("pointerdown", outside, true);
  document.removeEventListener("focusin", outside, true);
  document.removeEventListener("keydown", keys, true);
  document.removeEventListener("fabricworld:picker", other);
  window.removeEventListener("scroll", schedule, true);
  window.removeEventListener("resize", schedule);
  window.visualViewport?.removeEventListener("resize", schedule);
  window.visualViewport?.removeEventListener("scroll", schedule);
}
watch(() => props.open, async open => {
  cleanup();
  if (!open) return;
  await nextTick();
  if (!props.open || !panel.value) return;
  // WebKit can focus a field before scrolling it into the mobile viewport.
  // Bring the anchor into view before deciding where its panel can fit.
  props.anchor?.scrollIntoView({ block: "nearest", inline: "nearest", behavior: "instant" });
  // A styled top-layer element stays inside the modal's DOM/focus boundary,
  // while escaping scrolling containers. The browser draws no form menu.
  panel.value.showPopover?.(); position();
  document.dispatchEvent(new CustomEvent("fabricworld:picker", { detail: panel.value }));
  document.addEventListener("fabricworld:picker", other);
  document.addEventListener("pointerdown", outside, true);
  document.addEventListener("focusin", outside, true);
  document.addEventListener("keydown", keys, true);
  window.addEventListener("scroll", schedule, true);
  window.addEventListener("resize", schedule);
  window.visualViewport?.addEventListener("resize", schedule);
  window.visualViewport?.addEventListener("scroll", schedule);
  observer = new ResizeObserver(() => schedule());
  if (props.anchor) observer.observe(props.anchor);
}, { flush: "post" });
onBeforeUnmount(cleanup);
</script>
<template>
  <Transition name="picker" @before-leave="leaving" @before-enter="entering">
    <div v-if="open" ref="panel" popover="manual" class="dropdown-panel"
      :data-placement="placement" :style="style" @click.stop.prevent @pointerdown.stop>
      <slot />
    </div>
  </Transition>
</template>

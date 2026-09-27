<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from "vue";
import Icon from "./Icon.vue";
import DropdownPanel from "./DropdownPanel.vue";
defineOptions({ inheritAttrs: false });
type Option = { value: string; label: string; disabled?: boolean };
const props = withDefaults(defineProps<{
  modelValue?: string;
  options: (string | Option)[];
  label: string;
  editable?: boolean;
  placeholder?: string;
  maxlength?: number;
  disabled?: boolean;
}>(), { modelValue: "", editable: false, placeholder: "请选择" });
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
const id = "choice-" + useId();
const anchor = ref<HTMLElement>(), control = ref<HTMLElement>(), list = ref<HTMLElement>();
const open = ref(false), active = ref(-1);
const options = computed(() => props.options.map(option => typeof option === "string" ? { value: option, label: option } : option));
const visible = computed(() => props.editable && props.modelValue
  ? options.value.filter(option => option.label.toLocaleLowerCase().includes(props.modelValue.toLocaleLowerCase())) : options.value);
const text = computed(() => options.value.find(option => option.value === props.modelValue)?.label ?? (props.modelValue || props.placeholder));
let prefix = "", typedAt = 0;
function unavailable() { return props.disabled || control.value?.matches(":disabled"); }
async function highlight(index: number) {
  active.value = index;
  await nextTick();
  list.value?.querySelector<HTMLElement>("[data-active=true]")?.scrollIntoView({ block: "nearest" });
}
function show() {
  if (unavailable()) return;
  open.value = true;
  const selected = visible.value.findIndex(option => option.value === props.modelValue && !option.disabled);
  void highlight(props.editable ? -1 : selected >= 0 ? selected : visible.value.findIndex(option => !option.disabled));
}
function close(focus = false) {
  open.value = false; active.value = -1; prefix = "";
  if (focus) control.value?.focus({ preventScroll: true });
}
function choose(index: number) {
  const option = visible.value[index];
  if (!option || option.disabled || unavailable()) return;
  emit("update:modelValue", option.value); close(true);
}
function input(event: Event) {
  emit("update:modelValue", (event.target as HTMLInputElement).value);
  if (!open.value) show();
  active.value = -1;
}
function keys(event: KeyboardEvent) {
  if (event.isComposing || unavailable()) return;
  if (event.key === "Tab") { close(); return; }
  if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key) && (!props.editable || open.value || event.key.startsWith("Arrow"))) {
    event.preventDefault();
    if (!open.value) { show(); if (!props.editable) return; }
    const indexes = visible.value.flatMap((option, index) => option.disabled ? [] : [index]);
    if (!indexes.length) return;
    const current = indexes.indexOf(active.value);
    let next = event.key === "Home" ? 0 : event.key === "End" ? indexes.length - 1
      : event.key === "ArrowDown" ? (current + 1) % indexes.length : (current < 0 ? indexes.length : current) - 1;
    if (next < 0) next = indexes.length - 1;
    void highlight(indexes[next]!); return;
  }
  if (event.key === "Enter" || (!props.editable && event.key === " ")) {
    if (!props.editable || open.value) {
      event.preventDefault();
      if (!open.value) show(); else if (active.value >= 0) choose(active.value); else close();
    }
    return;
  }
  if (!props.editable && event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
    event.preventDefault(); if (!open.value) show();
    prefix = Date.now() - typedAt > 700 ? event.key : prefix + event.key; typedAt = Date.now();
    const index = visible.value.findIndex(option => !option.disabled && option.label.toLocaleLowerCase().startsWith(prefix.toLocaleLowerCase()));
    if (index >= 0) void highlight(index);
  }
}
watch(visible, () => {
  if (!open.value) return;
  // Input events clear the cursor. A later parent render must not erase a
  // keyboard choice the user has already made while suggestions update.
  if (props.editable) { if (active.value >= visible.value.length) active.value = -1; }
  else active.value = visible.value.findIndex(option => option.value === props.modelValue);
});
watch(() => props.disabled, disabled => { if (disabled) close(); });
</script>
<template>
  <span class="choice-field" :class="{ 'is-open': open, 'is-editable': editable }">
    <span ref="anchor" class="choice-anchor">
      <input v-if="editable" ref="control" v-bind="$attrs" class="choice-input" type="text"
        :value="modelValue" :placeholder="placeholder" :maxlength="maxlength" :disabled="disabled"
        :aria-label="label" role="combobox" aria-autocomplete="list" aria-haspopup="listbox"
        :aria-expanded="open" :aria-controls="open ? id : undefined"
        :aria-activedescendant="open && active >= 0 ? id + '-' + active : undefined"
        autocomplete="off" @input="input" @focus="show" @click="show" @keydown="keys" />
      <button v-else ref="control" v-bind="$attrs" type="button" class="choice-trigger" :disabled="disabled"
        :aria-label="label" role="combobox" aria-haspopup="listbox" :aria-expanded="open"
        :aria-controls="open ? id : undefined" :aria-activedescendant="open && active >= 0 ? id + '-' + active : undefined"
        @click.prevent="open ? close() : show()" @keydown="keys">
        <span class="choice-value">{{ text }}</span><Icon class="choice-chevron" name="chevron" :size="16" />
      </button>
      <button v-if="editable" type="button" tabindex="-1" class="choice-toggle" :disabled="disabled"
        aria-label="展开建议" :title="label + '建议'" @pointerdown.prevent @click.prevent="open ? close(true) : (control?.focus(), show())">
        <Icon class="choice-chevron" name="chevron" :size="16" />
      </button>
    </span>
    <DropdownPanel :open="open" :anchor="anchor" @close="close">
      <div :id="id" ref="list" role="listbox" :aria-label="label" class="choice-list">
        <div v-for="(option, index) in visible" :id="id + '-' + index" :key="option.value"
          role="option" :aria-selected="option.value === modelValue" :aria-disabled="option.disabled || undefined"
          class="choice-option" :data-active="index === active" :class="{ selected: option.value === modelValue }"
          @pointermove="!option.disabled && ($event.pointerType === 'mouse') && (active = index)"
          @pointerdown.prevent @click.stop.prevent="choose(index)">
          <span>{{ option.label }}</span><Icon v-if="option.value === modelValue" class="choice-mark" name="check" :size="17" />
        </div>
        <div v-if="!visible.length" class="choice-empty">{{ editable ? '没有匹配的建议，可以直接填写。' : '暂无可选项' }}</div>
      </div>
    </DropdownPanel>
  </span>
</template>

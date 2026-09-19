<script setup lang="ts">
import { newPiece, type Piece } from "../types";
import Icon from "./Icon.vue";
const pieces = defineModel<Piece[]>({ required: true });
function changeUnit(p: Piece, e: Event) {
  const next = (e.target as HTMLSelectElement).value as "m" | "cm";
  if (next === p.unit) return;
  for (const field of ["width", "length"] as const) {
    const raw = p[field];
    if (raw && /^\d+(\.\d+)?$/.test(raw)) {
      const mm = Math.round(Number(raw) * (p.unit === "m" ? 1000 : 10));
      p[field] = String(mm / (next === "m" ? 1000 : 10));
    }
  }
  p.unit = next;
}
</script>
<template>
  <div v-for="(p, i) in pieces" :key="i" class="piece-editor">
    <div class="piece-heading">
      <strong>布片 {{ i + 1 }}</strong>
      <button
        v-if="pieces.length > 1"
        type="button"
        class="icon-button"
        :aria-label="'删除布片 ' + (i + 1)"
        @click="pieces.splice(i, 1)"
      >
        <Icon name="close" :size="17" />
      </button>
    </div>
    <div class="dimensions-row">
      <label
        >幅宽<input
          v-model="p.width"
          inputmode="decimal"
          placeholder="待测量"
          :aria-label="'布片 ' + (i + 1) + ' 幅宽'"
      /></label>
      <span class="dimension-cross">×</span>
      <label
        >长度<input
          v-model="p.length"
          inputmode="decimal"
          placeholder="待测量"
          :aria-label="'布片 ' + (i + 1) + ' 长度'"
      /></label>
      <label class="unit-field"
        >单位<select
          :value="p.unit"
          :aria-label="'布片 ' + (i + 1) + ' 单位'"
          @change="changeUnit(p, $event)"
        >
          <option value="cm">cm</option>
          <option value="m">m</option>
        </select></label
      >
    </div>
    <div class="piece-bottom">
      <label class="quantity"
        >相同尺寸片数<input
          v-model.number="p.count"
          type="number"
          min="1"
          max="999"
          inputmode="numeric"
          :aria-label="'布片 ' + (i + 1) + ' 片数'"
      /></label>
      <label class="check-label"
        ><input
          v-model="p.irregular"
          type="checkbox"
          :aria-label="'布片 ' + (i + 1) + ' 不规则余料'"
        />不规则余料</label
      >
    </div>
    <label v-if="p.irregular || p.note"
      >形状备注<input
        v-model="p.note"
        maxlength="200"
        placeholder="例如：右下角缺了一块，尺寸为最大宽长"
        :aria-label="'布片 ' + (i + 1) + ' 形状备注'"
    /></label>
  </div>
  <button
    class="text-button"
    type="button"
    :disabled="pieces.length >= 50"
    @click="pieces.push(newPiece())"
  >
    <Icon name="plus" :size="17" />添加另一组尺寸
  </button>
  <p class="field-hint">不同尺寸的布片分组记录；尚未测量的尺寸可以留空。</p>
</template>

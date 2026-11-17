<script setup lang="ts">
// A slider for the four settings that are genuinely continuous.
//
// The track is a 3px line, not a rounded rectangle: DESIGN gives every filled
// shape a radius (12 card / 8 part), and a 3px bar cannot honour either. A line
// has no radius, so it stays outside that rule instead of inventing a fourth one.
import { computed, ref } from "vue";

const props = defineProps<{
  label: string;
  modelValue: number;
  min: number;
  max: number;
  step: number;
  display: string;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: number): void;
  (e: "commit", v: number): void;
}>();

const track = ref<HTMLElement | null>(null);
const dragging = ref(false);

const pct = computed(() => {
  const span = props.max - props.min;
  if (span <= 0) return 0;
  return Math.min(100, Math.max(0, ((props.modelValue - props.min) / span) * 100));
});

function quantize(v: number): number {
  const q = Math.round((v - props.min) / props.step) * props.step + props.min;
  return Math.min(props.max, Math.max(props.min, Math.round(q * 1000) / 1000));
}

function valueAt(clientX: number): number {
  const el = track.value;
  if (!el) return props.modelValue;
  const r = el.getBoundingClientRect();
  if (r.width <= 0) return props.modelValue;
  const p = Math.min(1, Math.max(0, (clientX - r.left) / r.width));
  return quantize(props.min + p * (props.max - props.min));
}

function down(ev: PointerEvent) {
  if (props.disabled) return;
  dragging.value = true;
  (ev.currentTarget as HTMLElement).setPointerCapture?.(ev.pointerId);
  emit("update:modelValue", valueAt(ev.clientX));
}

function move(ev: PointerEvent) {
  if (!dragging.value) return;
  emit("update:modelValue", valueAt(ev.clientX));
}

// The write happens on release, not on every pointermove: each save is a file
// rewrite, and a drag would otherwise produce a hundred of them.
function up(ev: PointerEvent) {
  if (!dragging.value) return;
  dragging.value = false;
  emit("commit", valueAt(ev.clientX));
}

function onKey(ev: KeyboardEvent) {
  if (props.disabled) return;
  const d = ev.shiftKey ? props.step * 10 : props.step;
  let v = props.modelValue;
  if (ev.key === "ArrowLeft" || ev.key === "ArrowDown") v -= d;
  else if (ev.key === "ArrowRight" || ev.key === "ArrowUp") v += d;
  else return;
  ev.preventDefault();
  const q = quantize(v);
  emit("update:modelValue", q);
  emit("commit", q);
}
</script>

<template>
  <div class="slider" :class="{ disabled }" tabindex="0" role="slider"
       :aria-valuemin="min" :aria-valuemax="max" :aria-valuenow="modelValue"
       :aria-label="label" @keydown="onKey">
    <div class="head">
      <span class="label">{{ label }}</span>
      <span class="val">{{ display }}</span>
    </div>
    <div ref="track" class="track"
         @pointerdown="down" @pointermove="move" @pointerup="up" @pointercancel="up">
      <span class="fill" :style="{ width: pct + '%' }" />
      <span class="knob" :class="{ active: dragging }" :style="{ left: pct + '%' }" />
    </div>
  </div>
</template>

<style scoped>
.slider {
  outline: none;
}

.slider.disabled {
  opacity: 0.4;
  pointer-events: none;
}

.head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  height: 16px;
}

.label {
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
}

.val {
  font-family: var(--font-text);
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
  font-variant-numeric: tabular-nums;
}

/* The hit area is 16px tall so the 3px line is not a pixel hunt; the line
   itself is painted by the pseudo-element. */
.track {
  position: relative;
  height: 16px;
  cursor: ew-resize;
  touch-action: none;
}

.track::before {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  top: 50%;
  height: 3px;
  margin-top: -1.5px;
  background: var(--pressed);
}

.fill {
  position: absolute;
  left: 0;
  top: 50%;
  height: 3px;
  margin-top: -1.5px;
  background: var(--accent);
}

.knob {
  position: absolute;
  top: 50%;
  width: 12px;
  height: 12px;
  margin: -6px 0 0 -6px;
  border-radius: 50%;
  background: var(--accent);
  transition: transform 160ms var(--ease);
}

.knob.active {
  transform: scale(1.15);
}

.slider:focus-visible .knob {
  box-shadow: 0 0 0 2px var(--accent);
}
</style>

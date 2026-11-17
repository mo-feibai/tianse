<script setup lang="ts">
// The sky axis: the night as one continuous gradient, with the current moment
// marked on it.
//
// The colours come from Go, computed by the same function that builds the gamma
// ramp. Nothing here is decorative — if the band looks orange at 19:20, the
// display is orange at 19:20, because both numbers came out of the same call.
import { computed, onBeforeUnmount, ref, watch } from "vue";
import type { Axis } from "../api";

const props = defineProps<{ axis: Axis | null; nowKelvin: number; paused: boolean; manual: boolean }>();
const emit = defineEmits<{
  (e: "preview", kelvin: number): void;
  (e: "commit", kelvin: number): void;
  (e: "auto"): void;
}>();

const band = ref<HTMLElement | null>(null);
const dragging = ref(false);
const dragStarted = ref(0);
const previewOffset = ref<number | null>(null);
const hint = ref("");
const entered = ref(false);

let hintTimer: number | undefined;

// One-shot entrance: the band grows from the left edge. This is the only
// choreography in the window, and it exists to explain what the product does.
onBeforeUnmount(() => window.clearTimeout(hintTimer));
requestAnimationFrame(() => {
  requestAnimationFrame(() => (entered.value = true));
});

const gradient = computed(() => {
  const a = props.axis;
  if (!a) return "transparent";
  if (a.Polar) return a.PolarColor || "#1D2025";
  const stops = a.Stops ?? [];
  if (stops.length === 0) return "transparent";
  return (
    "linear-gradient(90deg, " +
    stops.map((s) => `${s.Color} ${(s.Offset * 100).toFixed(2)}%`).join(", ") +
    ")"
  );
});

const ticks = computed(() => props.axis?.Ticks ?? []);
const markerOffset = computed(() => {
  if (previewOffset.value !== null) return previewOffset.value;
  return props.axis?.Now ?? 0;
});

// The "now" label rides under the marker. Near either edge it would be clipped
// by the card, so it flips its anchor instead of being hidden — a label that
// vanishes at dusk is worse than a label that shifts a few pixels.
const nowLabelStyle = computed(() => {
  const pct = markerOffset.value * 100;
  if (pct < 8) return { left: "0", transform: "none" };
  if (pct > 92) return { right: "0", left: "auto", transform: "none" };
  return { left: pct.toFixed(3) + "%", transform: "translateX(-50%)" };
});

// Same edge-flip as the now label: a mark near either end would push its label
// off the card.
function markLabelStyle(offset: number) {
  const pct = offset * 100;
  if (pct < 8) return { left: "0", transform: "none" };
  if (pct > 92) return { right: "0", left: "auto", transform: "none" };
  return { left: pct.toFixed(3) + "%", transform: "translateX(-50%)" };
}

// While dragging, the label reports the temperature being previewed rather than
// the clock time: the clock would not move, and the number would contradict the
// reading above.
const nowLabel = computed(() => {
  // The reading above already shows the previewed temperature; repeating it
  // here would say the same number twice and leave the one thing the user
  // cannot see anywhere else — how to get back — unsaid.
  if (previewOffset.value !== null) return "预览中 · 松手回到自动";
  return "现在 " + (props.axis?.NowText ?? "");
});

// Interpolating the stop list rather than solving the curve again: the band has
// 130 samples over ~13 hours, so a linear read between two neighbours is within
// a couple of kelvin of the exact answer.
function kelvinAt(offset: number): number {
  const stops = props.axis?.Stops ?? [];
  if (stops.length === 0) return props.nowKelvin;
  const clamped = Math.min(1, Math.max(0, offset));
  const pos = clamped * (stops.length - 1);
  const i = Math.min(stops.length - 2, Math.floor(pos));
  const f = pos - i;
  return Math.round(stops[i].Kelvin + (stops[i + 1].Kelvin - stops[i].Kelvin) * f);
}

function offsetFromEvent(ev: PointerEvent): number {
  const el = band.value;
  if (!el) return 0;
  const rect = el.getBoundingClientRect();
  if (rect.width <= 0) return 0;
  return Math.min(1, Math.max(0, (ev.clientX - rect.left) / rect.width));
}

function onDown(ev: PointerEvent) {
  if (!props.axis || props.axis.Polar || props.paused) return;
  dragging.value = true;
  dragStarted.value = performance.now();
  previewOffset.value = offsetFromEvent(ev);
  emit("preview", kelvinAt(previewOffset.value));
  (ev.target as HTMLElement).setPointerCapture?.(ev.pointerId);
}

function onMove(ev: PointerEvent) {
  if (!dragging.value) return;
  previewOffset.value = offsetFromEvent(ev);
  emit("preview", kelvinAt(previewOffset.value));
}

function onUp() {
  if (!dragging.value) return;
  dragging.value = false;
  const held = performance.now() - dragStarted.value;
  const offset = previewOffset.value;

  // A deliberate drag pins the temperature; a quick flick is just a look.
  // DESIGN 5.3: no implicit locking, so the threshold is a hold, not a distance.
  if (held > 3000 && offset !== null) {
    const kelvin = kelvinAt(offset);
    emit("commit", kelvin);
    hint.value = "已进入手动模式，点击轴回到自动";
    window.clearTimeout(hintTimer);
    hintTimer = window.setTimeout(() => (hint.value = ""), 6000);
    previewOffset.value = null;
    return;
  }

  previewOffset.value = null;
  emit("auto");
}

function onBandClick() {
  if (props.manual) emit("auto");
}
</script>

<template>
  <section class="card sunk">
    <div class="band-wrap" :class="{ entered, paused }">
      <div
        ref="band"
        class="band"
        :style="{ backgroundImage: gradient }"
        @pointerdown="onDown"
        @pointermove="onMove"
        @pointerup="onUp"
        @pointercancel="onUp"
        @click="onBandClick"
      >
        <span
          v-for="t in ticks"
          :key="t.Offset"
          class="tick"
          :style="{ left: (t.Offset * 100).toFixed(3) + '%' }"
        />
        <!-- The sun's own crossings sit inside the band, not at its ends: the
             band runs from two hours before sunset to two hours after
             sunrise. Drawn as full-height marks because they are events, not
             measurements. -->
        <span
          v-if="!axis?.Polar && axis"
          class="edge-mark"
          :style="{ left: (axis.SunsetOffset * 100).toFixed(3) + '%' }"
        />
        <span
          v-if="!axis?.Polar && axis"
          class="edge-mark"
          :style="{ left: (axis.SunriseOffset * 100).toFixed(3) + '%' }"
        />
        <span
          v-if="!axis?.Polar"
          class="marker"
          :class="{ preview: previewOffset !== null }"
          :style="{ left: (markerOffset * 100).toFixed(3) + '%' }"
        />
      </div>
      <!-- The diamond rides above the band's top edge, so it sits outside the
           rounded clipping box: inside, the 8px radius would cut it in half. -->
      <span
        v-if="!axis?.Polar"
        class="diamond"
        :class="{ preview: previewOffset !== null }"
        :style="{ left: (markerOffset * 100).toFixed(3) + '%' }"
      />

      <div class="labels">
        <!-- The ends are the shoulders of the curve, not events: a clock time
             there would be a fourth and fifth number competing with the three
             that matter. Only the polar case needs text. -->
        <span v-if="axis?.Polar" class="edge-label">无日出日落</span>
        <span
          v-if="!axis?.Polar && axis"
          class="mark-label"
          :style="markLabelStyle(axis.SunsetOffset)"
        >日入 {{ axis.SunsetText }}</span>
        <!-- The centre slot holds one thing at a time: the clock, or a message
             that temporarily replaces it. Two centred labels would collide. -->
        <span v-if="hint" class="hint">{{ hint }}</span>
        <span
          v-else-if="!axis?.Polar"
          class="now-label"
          :class="{ previewing: previewOffset !== null }"
          :style="nowLabelStyle"
        >{{ nowLabel }}</span>
        <span
          v-if="!axis?.Polar && axis"
          class="mark-label"
          :style="markLabelStyle(axis.SunriseOffset)"
        >日出 {{ axis.SunriseText }}</span>
        <span v-if="axis?.Polar" class="edge-label">固定时间表</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.card {
  width: 720px;
  height: 168px;
  border-radius: 12px;
  padding: 24px 32px;
  position: relative;
  overflow: hidden;
  background: var(--sunk);
  box-shadow: var(--edge);
}

/* The jade surface: a sheen from the top-left, the grain, and the band's own
   light. Kept as pseudo-elements so the card can stay one flat colour. */
.card::before {
  content: "";
  position: absolute;
  inset: 0;
  background: var(--sheen);
  pointer-events: none;
}

.card::after {
  content: "";
  position: absolute;
  inset: 0;
  background-image: url("/textures/noise-128.png");
  background-repeat: repeat;
  opacity: 0.025;
  pointer-events: none;
}

.band-wrap {
  position: relative;
  z-index: 1;
  width: 656px;
  opacity: 0;
  transform: scaleX(0.02);
  transform-origin: left center;
  transition: transform 420ms cubic-bezier(0.16, 1, 0.3, 1), opacity 200ms linear;
}

.band-wrap.entered {
  opacity: 1;
  transform: scaleX(1);
}

.band-wrap.paused {
  opacity: 0.4;
}

.band {
  position: relative;
  width: 656px;
  height: 96px;
  /* 8px, the element radius: the band is a reading inside a card, not a card.
     Clipping the gradient is the point — a 96px block of colour with square
     corners reads as a swatch glued onto the card. */
  border-radius: 8px;
  overflow: hidden;
  cursor: ew-resize;
  touch-action: none;
}

.tick {
  position: absolute;
  top: 0;
  width: 1px;
  height: 6px;
  background: var(--tick);
}

.edge-mark {
  position: absolute;
  top: 0;
  width: 1px;
  height: 96px;
  background: var(--xiang);
  opacity: 0.6;
}

.marker {
  position: absolute;
  top: 0;
  width: 1px;
  height: 96px;
  background: var(--accent);
  transform: translateX(-0.5px);
  transition: left 300ms var(--ease);
}

.marker.preview {
  transition: none;
}

.diamond {
  position: absolute;
  top: -4px;
  width: 5px;
  height: 5px;
  background: var(--accent);
  transform: translateX(-50%) rotate(45deg);
  transition: left 300ms var(--ease);
  pointer-events: none;
}

.diamond.preview {
  transition: none;
}

.labels {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 16px;
  margin-top: 8px;
}

/* Absolutely placed so it can ride the marker without pushing the two edge
   labels around. */
.now-label {
  position: absolute;
  bottom: 0;
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--text);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  transition: left 300ms var(--ease);
  pointer-events: none;
}

/* While previewing, the label is an instruction rather than a reading, so it
   steps back to the quiet colour the other hints use. */
.now-label.previewing {
  color: var(--ink);
}

.edge-label {
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
  opacity: 0.7;
  font-variant-numeric: tabular-nums;
}

/* Anchored to its mark, the way the now-label is anchored to the marker. */
.mark-label {
  position: absolute;
  bottom: 0;
  transform: translateX(-50%);
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--xiang);
  opacity: 0.85;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
  pointer-events: none;
}

.hint {
  position: absolute;
  left: 50%;
  bottom: 0;
  transform: translateX(-50%);
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
  white-space: nowrap;
}
</style>

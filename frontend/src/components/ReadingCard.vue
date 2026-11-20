<script setup lang="ts">
// The two numbers the whole product exists to produce.
import { computed, ref, watch } from "vue";

const props = defineProps<{
  kelvin: number;
  brightness: number;
  brightnessOk: boolean;
  /** Panel level read at launch, in panel percent. -1 = no backlight. */
  brightnessBase: number;
  /** Multiplier currently applied, in percent. 0 = the feature is off. */
  brightnessRatio: number;
  /** The fixed baseline in panel percent, or 0 if unset. */
  brightnessFixedBase: number;
  /** Which baseline the multiplier applies to: "panel", "startup" or "fixed". */
  brightnessBaseline: string;
  /** The user moved the backlight by hand and the engine stepped off it. */
  brightnessManual: boolean;
  night: boolean;
  previewing: boolean;
}>();

// Roll the kelvin figure instead of snapping it. 280ms, integers only: a
// fractional number ticking under the eye reads as a glitch, not as precision.
const shown = ref(props.kelvin);
let raf = 0;

watch(
  () => props.kelvin,
  (to) => {
    cancelAnimationFrame(raf);
    const from = shown.value;
    if (from === to) return;
    const start = performance.now();
    const step = (t: number) => {
      const p = Math.min(1, (t - start) / 280);
      const eased = 1 - Math.pow(1 - p, 3);
      shown.value = Math.round(from + (to - from) * eased);
      if (p < 1) raf = requestAnimationFrame(step);
    };
    raf = requestAnimationFrame(step);
  }
);

// KingHwa OldSong has no tnum (verified: GSUB carries fwid/pwid/zero but no
// tnum), and its digits run 0.507–0.595 em wide. Rolling a proportional number
// makes the whole figure twitch sideways every frame, so each digit gets its
// own fixed box — DESIGN 3.4's fallback path.
const DIGIT_BOX = "0.62em";
const kelvinChars = computed(() => String(shown.value).padStart(4, " ").split(""));
// The right-hand figure is always a real percentage when the panel answers
// DDC/CI: while the engine drives the backlight it is the target, and while the
// feature is off it is what the panel reports. Only two states have no number —
// a panel that refuses DDC/CI ("不可控") and one with no controllable backlight
// at all (engine sends -1, shown as a dash).
const brightnessText = computed(() => {
  if (!props.brightnessOk) return "不可控";
  if (props.brightness < 0) return "—";
  return `${props.brightness}%`;
});

// The number on the left of this line is a panel percentage, and the settings
// page can express its ratios against two different baselines. Under "panel" the
// two agree and there is nothing to add. Under "startup" the settings number is a
// multiplier on the launch level, so the arithmetic is printed: 100% of a panel
// that boots at 23% is 23%. The manual case replaces both, because the figure on
// the right is then the user's own doing and the engine is not steering it.
const brightnessNote = computed(() => {
  if (props.brightnessManual) return "手动";
  if (!props.brightnessOk || props.brightnessBase < 0) return "";
  if (props.brightnessBaseline === "fixed" && props.brightnessFixedBase > 0) {
    if (props.brightnessRatio <= 0) return `固定 ${props.brightnessFixedBase}%`;
    return `固定 ${props.brightnessFixedBase}% × ${props.brightnessRatio}%`;
  }
  if (props.brightnessBaseline !== "startup") return "";
  if (props.brightnessRatio <= 0) return `开机 ${props.brightnessBase}%`;
  return `开机 ${props.brightnessBase}% × ${props.brightnessRatio}%`;
});
</script>

<template>
  <section class="card">
    <div class="pair">
      <div class="left">
        <span class="value" :class="{ night, previewing }">
          <span v-for="(c, i) in kelvinChars" :key="i" class="d" :style="{ width: DIGIT_BOX }">{{ c }}</span>
        </span>
        <span class="unit">K</span>
      </div>
      <div class="right">
        <div class="readout">
          <span class="label">亮度</span>
          <span class="sub" :class="{ warn: !brightnessOk }">{{ brightnessText }}</span>
        </div>
        <span v-if="brightnessNote" class="note">{{ brightnessNote }}</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.card {
  width: 720px;
  height: 128px;
  border-radius: 12px;
  padding: 24px 32px;
  position: relative;
  overflow: hidden;
  background: var(--surface);
  box-shadow: var(--edge);
}

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
  opacity: 0.025;
  pointer-events: none;
}

.pair {
  position: relative;
  z-index: 1;
  height: 80px;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.left {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.value {
  font-family: var(--font-display);
  font-size: 56px;
  line-height: 1;
  color: var(--text);
  transition: color 200ms var(--ease);
  white-space: pre;
}

/* One box per digit, so 4520 -> 4500 does not shuffle the glyphs sideways. */
.d {
  display: inline-block;
  text-align: center;
}

/* Crossing the hysteresis threshold tints the number. It is the only place the
   interface admits that a decision was made. */
.value.night {
  color: var(--xiang);
}

.value.previewing {
  color: var(--accent);
}

.unit {
  font-family: var(--font-text);
  font-size: 14px;
  color: var(--ink);
}

.right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}

.readout {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.label {
  font-family: var(--font-text);
  font-size: 14px;
  color: var(--ink);
}

.sub {
  font-family: var(--font-display);
  font-size: 28px;
  line-height: 1.1;
  color: var(--text);
  font-variant-numeric: tabular-nums;
}

.sub.warn {
  font-family: var(--font-text);
  font-size: 14px;
  color: var(--ink);
}

/* The derivation, not a second reading: 12px ink so it never competes with the
   number above it. */
.note {
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
  opacity: 0.7;
  font-variant-numeric: tabular-nums;
}
</style>

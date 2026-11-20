<script setup lang="ts">
// The second page. Everything the instrument panel does not need to say out
// loud: where you are, how deep the night goes, how dim the screen may become,
// and when the light/dark switch flips.
//
// Values are edited locally and written once, on release (see Slider). The card
// material is the same as the main page: no new surface was invented for
// settings, because a settings screen that looks different is a settings screen
// the user has to learn twice.
import { computed } from "vue";
import Slider from "../components/Slider.vue";
import Segmented from "../components/Segmented.vue";
import CityPicker from "../components/CityPicker.vue";
import SwitchRow from "../components/SwitchRow.vue";
import type { City } from "../api";

const props = defineProps<{
  config: any | null;
  cities: City[];
  autostart: boolean;
  /** Panel level at startup, in panel percent. -1 = no controllable backlight. */
  brightnessBase: number;
  /** Multiplier the curve is applying right now, in percent. 0 = off. */
  brightnessRatio: number;
  /** The fixed baseline in panel percent, or 0 if unset. */
  brightnessFixedBase: number;
  /** Which baseline the ratios multiply: "panel", "startup" or "fixed". */
  brightnessBaseline: string;
  /** The engine stepped off the backlight because the user moved it by hand. */
  brightnessManual: boolean;
}>();
const emit = defineEmits<{
  (e: "input", path: string, value: unknown): void;
  (e: "commit"): void;
  (e: "city", city: City): void;
  (e: "autostart", enabled: boolean): void;
}>();

// Version is a build constant, not a setting. vite.config.ts reads it from the
// VERSION file at the repository root, the same file the installer and the Go
// build use, so there is nothing to keep in sync by hand.
const VERSION = __APP_VERSION__;

const num = (v: unknown, fallback: number): number =>
  typeof v === "number" && Number.isFinite(v) ? v : fallback;

const temp = computed(() => props.config?.Temp ?? {});
const bright = computed(() => props.config?.Brightness ?? {});
const theme = computed(() => props.config?.Theme ?? {});
const loc = computed(() => props.config?.Location ?? {});

const day = computed(() => num(temp.value.Day, 6500));
const night = computed(() => num(temp.value.Night, 3400));
const lead = computed(() => num(temp.value.StartBeforeSunsetMin, 120));
const settle = computed(() => num(temp.value.SettleAfterSunsetMin, 30));

const dayRatio = computed(() => num(bright.value.DayRatio, 1));
const nightRatio = computed(() => num(bright.value.NightRatio, 0.45));
const floor = computed(() => num(bright.value.FloorRatio, 0.05));
const step = computed(() => num(bright.value.StepThreshold, 0.03));

const baseline = computed(() => String(bright.value.Baseline ?? "panel"));
const fixedPct = computed(() => num(bright.value.BasePercent, 0));
const BASELINES = [
  { value: "panel", label: "面板满亮" },
  { value: "startup", label: "开机亮度" },
  { value: "fixed", label: "固定基准" },
];

// The three baselines differ by one factor: the panel's own full scale, the
// level it happened to be at when the app launched, or a panel percentage the
// user fixed. Switching the baseline has to rescale every ratio by the ratio
// between the two, or the screen jumps the moment the user picks another word.
// The factors come from numbers the engine reports, so nothing has to be
// remembered here and the conversion works in both directions.
const factorOf = (which: string, pct: number): number => {
  if (which === "startup") return props.brightnessBase > 0 ? props.brightnessBase / 100 : 1;
  if (which === "fixed") return pct > 0 ? pct / 100 : 1;
  return 1;
};
const baselineFactor = (which: string): number => factorOf(which, fixedPct.value);

const clampRatio = (v: number): number => Math.min(1, Math.max(0.05, v));

function setBaseline(next: string) {
  if (next === baseline.value) return;
  // Picking "fixed" for the first time needs a value. Seed it with the launch
  // reading so the screen does not move on the switch; the user can edit it next.
  const newFixed = next === "fixed" && fixedPct.value <= 0
    ? (props.brightnessBase > 0 ? props.brightnessBase : 100)
    : fixedPct.value;
  const k = baselineFactor(baseline.value) / factorOf(next, newFixed);
  emit("input", "Brightness.Baseline", next);
  if (newFixed !== fixedPct.value) emit("input", "Brightness.BasePercent", newFixed);
  emit("input", "Brightness.DayRatio", clampRatio(dayRatio.value * k));
  emit("input", "Brightness.NightRatio", clampRatio(nightRatio.value * k));
  emit("input", "Brightness.FloorRatio", clampRatio(floor.value * k));
  emit("commit");
}

function setFixedBase(v: number) {
  const pct = Math.min(100, Math.max(5, Math.round(v)));
  const k = baselineFactor("fixed") / (pct / 100);
  emit("input", "Brightness.BasePercent", pct);
  emit("input", "Brightness.DayRatio", clampRatio(dayRatio.value * k));
  emit("input", "Brightness.NightRatio", clampRatio(nightRatio.value * k));
  emit("input", "Brightness.FloorRatio", clampRatio(floor.value * k));
}

const mode = computed(() => String(theme.value.Mode ?? "follow_sky"));
const darkAt = computed(() => num(theme.value.DarkAt, 4500));
const lightAt = computed(() => num(theme.value.LightAt, 5200));

// The two thresholds only exist for the sky-driven mode. In the other three the
// decision comes from somewhere else, so they are shown but inert rather than
// hidden — a control that vanishes makes the page jump.
const thresholdsLive = computed(() => mode.value === "follow_sky");

const MODES = [
  { value: "follow_sky", label: "跟随天色" },
  { value: "follow_system", label: "跟随系统" },
  { value: "always_light", label: "强制浅色" },
  { value: "always_dark", label: "强制深色" },
];

const city = computed(() => String(loc.value.City ?? ""));

const coords = computed(() => {
  const lat = num(loc.value.Latitude, NaN);
  const lon = num(loc.value.Longitude, NaN);
  if (!Number.isFinite(lat) || !Number.isFinite(lon)) return "—";
  const ns = lat >= 0 ? "北纬" : "南纬";
  const ew = lon >= 0 ? "东经" : "西经";
  return `${ns} ${Math.abs(lat).toFixed(2)}°　${ew} ${Math.abs(lon).toFixed(2)}°`;
});

const timezone = computed(() => String(loc.value.Timezone ?? "—"));

const pct = (v: number) => `${Math.round(v * 100)}%`;

// The three ratios below multiply a baseline, and which baseline is a choice.
// Under "panel" a ratio of 100% means the panel's own maximum, so there is
// nothing to explain; under "startup" it means the level the panel was at on
// launch. Under "fixed" the headrow already shows the number, so neither needs
// a second line here.
const brightnessNote = computed(() => {
  if (props.brightnessBase < 0) return "此屏不接受亮度控制";
  if (props.brightnessManual) return "已手动接管，改设置即恢复";
  if (baseline.value === "fixed") return "";
  if (baseline.value !== "startup") return "";
  const base = props.brightnessBase;
  if (props.brightnessRatio <= 0) return `开机${base}%`;
  // Kept short on purpose: the full derivation lives on the reading card, and
  // a long formula here pushes the baseline switch onto a second line.
  return `开机${base}%×${props.brightnessRatio}%=面板${Math.round((base * props.brightnessRatio) / 100)}%`;
});
</script>

<template>
  <div class="settings">
    <div class="col wide">
      <section class="card">
        <h2 class="head">色温</h2>
        <div class="grid">
          <Slider
            label="白天色温"
            :model-value="day"
            :min="5000"
            :max="6500"
            :step="100"
            :display="`${day} K`"
            @update:model-value="emit('input', 'Temp.Day', $event)"
            @commit="emit('commit')"
          />
          <Slider
            label="夜间色温"
            :model-value="night"
            :min="2700"
            :max="4500"
            :step="100"
            :display="`${night} K`"
            @update:model-value="emit('input', 'Temp.Night', $event)"
            @commit="emit('commit')"
          />
          <Slider
            label="日落前开始"
            :model-value="lead"
            :min="0"
            :max="240"
            :step="15"
            :display="`${lead} 分钟`"
            @update:model-value="emit('input', 'Temp.StartBeforeSunsetMin', $event)"
            @commit="emit('commit')"
          />
          <Slider
            label="入夜用时"
            :model-value="settle"
            :min="0"
            :max="120"
            :step="10"
            :display="`${settle} 分钟`"
            @update:model-value="emit('input', 'Temp.SettleAfterSunsetMin', $event)"
            @commit="emit('commit')"
          />
        </div>
      </section>

      <section class="card">
        <div class="headrow">
          <h2 class="head">亮度</h2>
          <span v-if="brightnessNote" class="basis">{{ brightnessNote }}</span>
          <Segmented class="seg" :options="BASELINES" :model-value="baseline" @update:model-value="setBaseline" />
        </div>
        <div class="grid">
          <Slider
            v-if="baseline === 'fixed'"
            label="固定基准"
            :model-value="fixedPct > 0 ? fixedPct / 100 : (props.brightnessBase > 0 ? props.brightnessBase / 100 : 1)"
            :min="0.05"
            :max="1"
            :step="0.01"
            :display="pct(fixedPct > 0 ? fixedPct / 100 : (props.brightnessBase > 0 ? props.brightnessBase / 100 : 1))"
            @update:model-value="setFixedBase($event * 100)"
            @commit="emit('commit')"
          />
          <Slider
            label="白天亮度"
            :model-value="dayRatio"
            :min="0.05"
            :max="1"
            :step="0.01"
            :display="pct(dayRatio)"
            @update:model-value="emit('input', 'Brightness.DayRatio', $event)"
            @commit="emit('commit')"
          />
          <Slider
            label="夜间亮度"
            :model-value="nightRatio"
            :min="0.05"
            :max="1"
            :step="0.01"
            :display="pct(nightRatio)"
            @update:model-value="emit('input', 'Brightness.NightRatio', $event)"
            @commit="emit('commit')"
          />
          <Slider
            label="最低下限"
            :model-value="floor"
            :min="0.05"
            :max="1"
            :step="0.01"
            :display="pct(floor)"
            @update:model-value="emit('input', 'Brightness.FloorRatio', $event)"
            @commit="emit('commit')"
          />
          <Slider
            v-if="baseline !== 'fixed'"
            label="写入阈值"
            :model-value="step"
            :min="0.01"
            :max="0.1"
            :step="0.01"
            :display="pct(step)"
            @update:model-value="emit('input', 'Brightness.StepThreshold', $event)"
            @commit="emit('commit')"
          />
        </div>
      </section>

      <section class="card grow">
        <div class="headrow">
          <h2 class="head">明暗</h2>
          <span class="basis">界面与系统同切</span>
          <Segmented
            class="seg"
            :options="MODES"
            :model-value="mode"
            @update:model-value="emit('input', 'Theme.Mode', $event); emit('commit')"
          />
        </div>
        <h3 class="sub">切换阈值</h3>
        <div class="grid">
          <Slider
            label="转暗"
            :model-value="darkAt"
            :min="3000"
            :max="6500"
            :step="100"
            :display="`${darkAt} K`"
            :disabled="!thresholdsLive"
            @update:model-value="emit('input', 'Theme.DarkAt', $event)"
            @commit="emit('commit')"
          />
          <Slider
            label="回亮"
            :model-value="lightAt"
            :min="3000"
            :max="6500"
            :step="100"
            :display="`${lightAt} K`"
            :disabled="!thresholdsLive"
            @update:model-value="emit('input', 'Theme.LightAt', $event)"
            @commit="emit('commit')"
          />
        </div>
      </section>
    </div>

    <div class="col narrow">
      <section class="card">
        <h2 class="head">位置</h2>
        <CityPicker
          :cities="cities"
          :model-value="city"
          @update:model-value="(name) => { const c = cities.find((x) => x.Name === name); if (c) emit('city', c); }"
        />
        <p class="meta">{{ coords }}</p>
        <p class="meta">{{ timezone }}</p>
        <h3 class="sub">启动</h3>
        <SwitchRow
          label="开机自启"
          :model-value="autostart"
          @update:model-value="emit('autostart', $event)"
        />
      </section>

      <section class="card grow">
        <h2 class="head">关于</h2>
        <p class="blurb">天色随日落入夜，把屏幕的色温、亮度与明暗一起带下去。</p>
        <dl class="rows">
          <div class="row">
            <dt>版本</dt>
            <dd>{{ VERSION }}</dd>
          </div>
          <div class="row">
            <dt>字体</dt>
            <dd>京华老宋体 · 霞鹜文楷</dd>
          </div>
          <div class="row">
            <dt>授权</dt>
            <dd>MIT</dd>
          </div>
          <div class="row block">
            <dt>数据目录</dt>
            <dd class="path">%APPDATA%\Tianse\</dd>
          </div>
        </dl>
      </section>
    </div>
  </div>
</template>

<style scoped>
.settings {
  display: flex;
  gap: 20px;
  height: 100%;
}

.col {
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: 100%;
}

.wide {
  width: 460px;
}

.narrow {
  width: 240px;
}

/* Same jade as the main page: L2 face, L3 sheen, L4 inset edge, L5 grain. */
.card {
  position: relative;
  border-radius: 12px;
  padding: 24px;
  background: var(--surface);
  box-shadow: var(--edge);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* No overflow:hidden here. The city list is a popup inside this card and would
   be cut off at the card edge; the two pseudo-element layers get the radius
   themselves instead. */

.narrow .card {
  background: var(--sunk);
  /* The narrow column holds more lines in the same 528px, so its inner rhythm
     is one step tighter. 8 is on the scale; 12 overflows the frame. */
  gap: 8px;
}

.card::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 12px;
  background: var(--sheen);
  pointer-events: none;
}

.card::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 12px;
  background-image: url("/textures/noise-128.png");
  opacity: 0.025;
  pointer-events: none;
}

.card > * {
  position: relative;
  z-index: 1;
}

/* The picker opens a panel over the rows below it, which are siblings at the
   same z-index and would paint on top. Lifting the whole picker keeps the open
   list above the text it covers. */
.card > .picker,
.card > :has(> .picker) {
  z-index: 2;
}

/* The last card in each column absorbs the leftover height so both columns end
   on the same line. A ragged bottom would read as a missing card.
   The slack goes into one spacer rather than being split by space-between:
   equal gaps would also open up between the heading and the control below it,
   and a heading floating away from its card reads as a broken card. */
.grow {
  flex: 1;
}

.sub {
  margin: 0;
  font-family: var(--font-text);
  font-size: 12px;
  font-weight: 400;
  line-height: 1.5;
  letter-spacing: 0.08em;
  color: var(--ink);
  opacity: 0.7;
}

.head {
  margin: 0;
  font-family: var(--font-text);
  font-size: 12px;
  font-weight: 400;
  line-height: 1.5;
  letter-spacing: 0.08em;
  color: var(--ink);
}

.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 24px;
}

/* The brightness ratios multiply a baseline the user picks here, so the switch
   itself belongs on the heading line: it is a property of the whole card, not of
   any one slider. The derivation below only appears under "startup" — under
   "panel" a ratio of 100% is already the panel's own 100% and there is nothing
   to explain, which is the point of offering that baseline at all. */
.headrow {
  display: flex;
  align-items: baseline;
  gap: 12px;
  min-height: 32px;
}

/* Two characters of heading must never wrap: the brightness row's switch takes
   most of the width, and a wrapped "明暗" turns a 32 px row into 39 px. */
.headrow .head {
  white-space: nowrap;
}

/* The switch is a property of the whole card, so it sits at the far end of the
   heading line and the derivation, when there is one, fills the space before
   it. Both have to stay on this one line: a second line costs 16 px and the wide
   column has 44 px of slack in total. */
.headrow .seg {
  margin-left: auto;
  /* Three labels at the default 20 px gap crowd the fixed-baseline input off
     this row. Tightening to 16 px keeps everything on one line; wrapping onto two
     lines makes the row 39 px instead of 32 px and the labels read as broken. */
  gap: 16px;
}

.meta {
  margin: 0;
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
  font-variant-numeric: tabular-nums;
}

/* The fixed-baseline value lives on the heading line too, since it is a property
   of the whole card. The number box is kept narrow so "基准 NN%" stays one line. */
.fixedbase {
  margin: 0;
  font-family: var(--font-text);
  font-size: 12px;
  color: var(--ink);
  opacity: 0.8;
  white-space: nowrap;
}

.fixednum {
  width: 40px;
  padding: 2px 4px;
  font: inherit;
  font-variant-numeric: tabular-nums;
  text-align: right;
  color: var(--text);
  background: var(--sunk);
  border: 1px solid var(--edge);
  border-radius: 4px;
}

.basis {
  margin: 0;
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
  opacity: 0.7;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.blurb {
  margin: 0;
  font-family: var(--font-text);
  font-size: 14px;
  line-height: 1.7;
  color: var(--ink);
  text-wrap: pretty;
}

.rows {
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

/* A path is longer than the 192px a narrow card leaves once its padding is
   taken out. A value that ends in "…" hides the only part of the path the user
   needs, so these rows stack instead of aligning right. The paths are written
   relative to the data directory above them, which keeps each one on one line. */
.row.block {
  display: block;
}

.row.block .path {
  margin-top: 2px;
  text-align: left;
  white-space: normal;
  overflow-wrap: anywhere;
}

dt {
  flex: none;
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
}

dd {
  margin: 0;
  min-width: 0;
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--text);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

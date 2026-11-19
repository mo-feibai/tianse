<script setup lang="ts">
// The window. Two pages share one 900x620 frame: the instrument panel, and the
// settings that feed it. No router — there are two pages, and a router would be
// a dependency plus a URL scheme for a state that is one string.
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import TitleBar from "./components/TitleBar.vue";
import ReadingCard from "./components/ReadingCard.vue";
import SkyAxis from "./components/SkyAxis.vue";
import SwitchCard from "./components/SwitchCard.vue";
import StatusCard from "./components/StatusCard.vue";
import FirstRunBar from "./components/FirstRunBar.vue";
import SettingsView from "./views/SettingsView.vue";
import {
  emptyStatus,
  fetchAutostart,
  fetchAxis,
  fetchCities,
  fetchConfig,
  fetchStatus,
  saveConfig,
  setAutostart,
  setManualTemperature,
  type Axis,
  type City,
  type Status,
} from "./api";
import { themeVars } from "./theme";

const status = ref<Status>(emptyStatus());
const axis = ref<Axis | null>(null);
const config = ref<any | null>(null);
const cities = ref<City[]>([]);
// Read from the registry, not from the config file: a cleaner tool or a manual
// edit can delete the Run entry, and this switch must show the truth.
const autostart = ref(false);
const preview = ref<number | null>(null);
const page = ref<"main" | "settings">("main");

let timer: number | undefined;
let axisAt = 0;

const shownKelvin = computed(() => preview.value ?? status.value.Kelvin);
const night = computed(() => status.value.ThemeDark);
const vars = computed(() => themeVars(status.value));

async function refreshStatus() {
  const s = await fetchStatus();
  if (s) status.value = s;
}

async function refreshAxis(force = false) {
  // The band covers one night, so it only needs rebuilding when the day turns
  // over. Polling it every second would re-send 130 stops for nothing.
  const now = Date.now();
  if (!force && now - axisAt < 60000) return;
  const a = await fetchAxis();
  if (a) {
    axis.value = a;
    axisAt = now;
  }
}

async function refreshConfig() {
  const c = await fetchConfig();
  if (c) config.value = c;
}

function onPreview(kelvin: number) {
  preview.value = kelvin;
}

async function onCommit(kelvin: number) {
  preview.value = null;
  await setManualTemperature(kelvin);
  await refreshStatus();
}

async function onAuto() {
  preview.value = null;
  if (!status.value.Manual) return;
  await setManualTemperature(0);
  await refreshStatus();
}

// The three switches on the main page are whole features, so they take the
// copy-and-save path: a half-written config would leave the engine reading a
// temperature range that no longer matches the enabled flag.
async function patch(apply: (c: any) => void) {
  const base = config.value ?? (await fetchConfig());
  if (!base) return;
  const next = JSON.parse(JSON.stringify(base));
  apply(next);
  config.value = next;
  await saveConfig(next);
  await refreshStatus();
}

function toggleTemp(on: boolean) {
  patch((c) => (c.Temp.Enabled = on));
}

function toggleBrightness(on: boolean) {
  patch((c) => (c.Brightness.Enabled = on));
}

function toggleTheme(on: boolean) {
  patch((c) => (c.Theme.Enabled = on));
}

// Settings edit a single leaf, live, and write once on release. Editing in
// place keeps the slider responsive; the write is what costs a file rewrite.
function setPath(path: string, value: unknown) {
  const root = config.value;
  if (!root) return;
  const parts = path.split(".");
  let node = root;
  for (let i = 0; i < parts.length - 1; i++) {
    if (node[parts[i]] == null) node[parts[i]] = {};
    node = node[parts[i]];
  }
  node[parts[parts.length - 1]] = value;
}

async function onSave() {
  if (!config.value) return;
  await saveConfig(config.value);
  await refreshStatus();
  await refreshAxis(true);
}

async function onCity(city: City) {
  const root = config.value;
  if (!root) return;
  root.Location = {
    ...(root.Location ?? {}),
    City: city.Name,
    Latitude: city.Latitude,
    Longitude: city.Longitude,
    Timezone: city.Timezone,
  };
  await onSave();
}

// The first launch is the only moment the app may ask anything: which city it
// should follow the sun for, and whether it should start with Windows. Both are
// already defaulted, so dismissing without touching either is a valid answer.
const firstRun = computed(() => config.value?.App?.FirstRun === true);

async function dismissFirstRun() {
  const root = config.value;
  if (!root) return;
  if (root.App) root.App.FirstRun = false;
  await onSave();
}

async function onFirstRunAutostart(enabled: boolean) {
  await onAutostart(enabled);
}

async function onFirstRunCity() {
  page.value = "settings";
}

async function onAutostart(enabled: boolean) {
  autostart.value = enabled;
  const actual = await setAutostart(enabled);
  autostart.value = actual;
}

onMounted(async () => {
  await Promise.all([refreshStatus(), refreshConfig()]);
  cities.value = await fetchCities();
  autostart.value = await fetchAutostart();
  await refreshAxis(true);
  timer = window.setInterval(async () => {
    await refreshStatus();
    await refreshAxis();
  }, 1000);
});

onBeforeUnmount(() => window.clearInterval(timer));
</script>

<template>
  <div class="shell" :style="vars">
    <TitleBar :page="page" @navigate="page = $event" />

    <!-- Two absolutely-positioned pages on one stage: the frame never resizes,
         so the slide is a pure transform and costs no layout. -->
    <div class="stage">
      <div class="page main" :class="{ off: page !== 'main' }" :aria-hidden="page !== 'main'">
        <main class="content">
          <FirstRunBar
            v-if="firstRun"
            :city="config?.Location?.City ?? ''"
            :autostart="autostart"
            :brightness="config?.Brightness?.Enabled ?? false"
            @city="onFirstRunCity"
            @autostart="onFirstRunAutostart"
            @brightness="toggleBrightness"
            @done="dismissFirstRun"
          />
          <ReadingCard
            v-else
            :kelvin="shownKelvin"
            :brightness="status.BrightnessPercent"
            :brightness-ok="status.BrightnessOK"
            :brightness-base="status.BrightnessBase"
            :brightness-ratio="status.BrightnessRatio"
            :brightness-baseline="status.BrightnessBaseline"
            :brightness-manual="status.BrightnessManual"
            :night="night"
            :previewing="preview !== null"
          />
          <SkyAxis
            :axis="axis"
            :now-kelvin="status.Kelvin"
            :paused="status.Paused"
            :manual="status.Manual"
            @preview="onPreview"
            @commit="onCommit"
            @auto="onAuto"
          />
          <div class="row">
            <SwitchCard
              :temp="config?.Temp?.Enabled ?? false"
              :brightness="config?.Brightness?.Enabled ?? false"
              :theme="config?.Theme?.Enabled ?? false"
              :brightness-ok="status.BrightnessOK"
              @update:temp="toggleTemp"
              @update:brightness="toggleBrightness"
              @update:theme="toggleTheme"
            />
            <StatusCard :status="status" />
          </div>
        </main>
      </div>

      <div class="page settings" :class="{ off: page !== 'settings' }" :aria-hidden="page !== 'settings'">
        <div class="content">
          <SettingsView
            :config="config"
            :cities="cities"
            :autostart="autostart"
            :brightness-base="status.BrightnessBase"
            :brightness-ratio="status.BrightnessRatio"
            :brightness-baseline="status.BrightnessBaseline"
            :brightness-manual="status.BrightnessManual"
            @input="setPath"
            @commit="onSave"
            @city="onCity"
            @autostart="onAutostart"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.shell {
  width: 900px;
  height: 620px;
  background: var(--bg);
  color: var(--text);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
}

/* The ambient light layer: a wash from above, tinted by the sky. It sits under
   every card, so its strength never touches text contrast (DESIGN 2.5). */
.shell::before {
  content: "";
  position: absolute;
  inset: 0;
  background: radial-gradient(
    140% 120% at 30% -20%,
    color-mix(in srgb, var(--accent) 22%, transparent) 0%,
    transparent 62%
  );
  pointer-events: none;
  transition: background 3000ms linear;
}

.stage {
  position: relative;
  z-index: 1;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.page {
  position: absolute;
  inset: 0;
  transition: transform 240ms var(--ease), opacity 240ms var(--ease);
}

.page.main {
  transform: translateX(0);
}

.page.main.off {
  transform: translateX(-40px);
  opacity: 0;
  pointer-events: none;
}

.page.settings {
  transform: translateX(40px);
  opacity: 0;
  pointer-events: none;
}

.page.settings:not(.off) {
  transform: translateX(0);
  opacity: 1;
  pointer-events: auto;
}

.content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 20px 90px;
  height: 100%;
}

.settings .content {
  padding: 20px 90px 24px;
}

.row {
  display: flex;
  gap: 20px;
}

@media (prefers-reduced-motion: reduce) {
  .page {
    transition: none;
  }
}
</style>

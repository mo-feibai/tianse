// Preview data for design review without a desktop session.
//
// The real numbers come from Go. This module exists so the layout can be looked
// at in a plain browser (query string ?preview=1) when WebView2 cannot start —
// it is never imported by the shipped window, because api.ts only consults it
// when the flag is present.

import type { Axis, AxisStop, City, Status } from "./api";

export const previewRequested = (): boolean =>
  typeof location !== "undefined" && /[?&]preview=1\b/.test(location.search);

// ?light=1 swaps the palette so the light surface can be reviewed too. The real
// window picks it from the sun position, which a browser cannot know.
const previewLight = (): boolean =>
  typeof location !== "undefined" && /[?&]light=1\b/.test(location.search);

// Blackbody approximation, the same shape as internal/color/temperature.go.
function kelvinRGB(k: number): [number, number, number] {
  const t = k / 100;
  let r: number;
  let g: number;
  let b: number;
  if (t <= 66) {
    r = 255;
    g = 99.4708025861 * Math.log(t) - 161.1195681661;
    b = t <= 19 ? 0 : 138.5177312231 * Math.log(t - 10) - 305.0447927307;
  } else {
    r = 329.698727446 * Math.pow(t - 60, -0.1332047592);
    g = 288.1221695283 * Math.pow(t - 60, -0.0755148492);
    b = 255;
  }
  return [r, g, b].map((v) => Math.min(255, Math.max(0, v))) as [number, number, number];
}

// Scale to a fixed relative luminance so the band never collapses to black at
// the warm end. Mirrors color.NormaliseHex.
function bandColor(k: number, target: number, boost = 1): string {
  const [r, g, b] = kelvinRGB(k);
  const lum = (c: number) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  };
  const src = 0.2126 * lum(r) + 0.7152 * lum(g) + 0.0722 * lum(b);
  const k2 = Math.pow(target / Math.max(src, 1e-6), 1 / 2.2);
  const hex = (c: number) =>
    Math.round(Math.min(255, Math.max(0, c * k2)))
      .toString(16)
      .padStart(2, "0");
  if (boost === 1) return "#" + hex(r) + hex(g) + hex(b);
  // Chroma boost around the target grey: normalising to a luminance washes the
  // warm end out, so the light theme pushes each channel away from that grey.
  const base = 255 * Math.pow(target, 1 / 2.2);
  const push = (c: number) => base + (Math.min(255, c * k2) - base) * boost;
  return (
    "#" +
    Math.round(push(r)).toString(16).padStart(2, "0") +
    Math.round(push(g)).toString(16).padStart(2, "0") +
    Math.round(push(b)).toString(16).padStart(2, "0")
  );
}

export function previewAxis(): Axis {
  const stops: AxisStop[] = [];
  const n = 130;
  // Same rule as the Go side: the band keeps 2.1:1 against the card it rests on,
  // which means it goes darker than the paper in the light theme and lighter
  // than the panel in the dark one.
  const q = new URLSearchParams(location.search);
  const lum = previewLight() ? Number(q.get("lum") ?? 0.42) : 0.075;
  const boost = previewLight() ? Number(q.get("boost") ?? 1.3) : 1;
  // The same curve the engine runs: the ramp starts 120 min before sunset,
  // reaches the night value 30 min after it, holds all night, then climbs back
  // from 30 min before sunrise to 120 min after. Sampling the band with the real
  // shape is the whole point of the band — a decorative bell curve would show a
  // sky the tool never produces.
  const blend = (p: number) => {
    const t = (1 - Math.cos(Math.PI * Math.min(1, Math.max(0, p)))) / 2;
    return 6500 + (3400 - 6500) * t;
  };
  const night = 13.43 * 60; // sunset 17:12 to sunrise 06:38, minutes
  const beforeSunset = 120;
  const afterSunset = 30;
  // The band spans the shoulders of the curve: two hours before sunset to two
  // hours after sunrise. Sunset-to-sunrise leaves three quarters of the width
  // flat at the night temperature.
  const full = q.get("span") !== "night";
  const lead = full ? beforeSunset : 0;
  const span = full ? night + beforeSunset * 2 : night;
  for (let i = 0; i < n; i++) {
    const p = i / (n - 1);
    const t = p * span - lead; // minutes after sunset
    let k: number;
    if (t < afterSunset) {
      k = blend((beforeSunset + t) / (beforeSunset + afterSunset));
    } else if (t > night - afterSunset - beforeSunset) {
      const up = (t - (night - afterSunset - beforeSunset)) / (beforeSunset + afterSunset);
      k = blend(1 - up);
    } else {
      k = 3400;
    }
    stops.push({ Offset: p, Color: bandColor(k, lum, boost), Kelvin: Math.round(k) });
  }
  const ticks: { Offset: number; Label: string }[] = [];
  const startHour = full ? 15 : 17;
  for (let h = startHour; h <= startHour + 18; h += 2) {
    const hour = h % 24;
    ticks.push({ Offset: (h - (17 + 12 / 60) + lead / 60) / (span / 60), Label: String(hour).padStart(2, "0") + ":00" });
  }
  const nowMin = 22 * 60 + 48 - (17 * 60 + 12);
  return {
    Stops: stops,
    Ticks: ticks.filter((t) => t.Offset >= 0 && t.Offset <= 1),
    Now: (nowMin + lead) / span,
    NowText: "22:48",
    Start: full ? "15:12" : "17:12",
    End: full ? "08:38" : "06:38",
    SunsetOffset: full ? beforeSunset / span : 0,
    SunriseOffset: full ? (beforeSunset + night) / span : 1,
    SunsetText: "17:12",
    SunriseText: "06:38",
    Polar: false,
    PolarColor: "",
    Empty: false,
  };
}

export function previewStatus(): Status {
  const light = previewLight();
  // ?held=1 previews the full-screen hold state, which is otherwise only
  // reachable by launching a game.
  const held = /[?&]held=1\b/.test(location.search);
  // Automatic brightness off: the panel still reports its level over DDC, so the
  // reading card keeps showing a real percentage instead of a dash.
  const brightnessOff = /[?&]boff=1\b/.test(location.search);
  // ?bok=0 previews a panel that refuses DDC/CI: the card must read 不可控.
  const brightnessOK = !/[?&]bok=0\b/.test(location.search);
  return {
    Kelvin: light ? 5800 : 4520,
    Phase: "evening",
    Progress: 0.42,
    Sunset: "17:12",
    Sunrise: "06:38",
    NextChange: "19:42",
    NextChangeK: 3400,
    Paused: false,
    Polar: false,
    BrightnessOn: !brightnessOff,
    BrightnessOK: brightnessOK,
    BrightnessPercent: 62,
    ThemeOn: true,
    ThemeDark: false,
    ThemeMode: "follow_sky",
    SurfaceDark: !light,
    Accent: light ? "#2B7780" : "#4CC6D1",
    Ink: light ? "#606D74" : "#8D9FAE",
    Xiang: light ? "#92680F" : "#D9A441",
    City: "杭州",
    DisplayName: "显示器 1",
    DisplayNum: 1,
    Warning: "",
    GammaOK: true,
    Manual: false,
    Held: held,
  };
}

export function previewConfig(): any {
  // ?first=0 hides the first-launch card so the reading card can be reviewed,
  // and ?boff=1 previews automatic brightness switched off — the panel still
  // answers DDC, so the figure must stay a real number.
  const q = new URLSearchParams(location.search);
  const firstRun = q.get("first") !== "0";
  return {
    Version: 1,
    Location: { City: "杭州", Latitude: 30.2741, Longitude: 120.1551, Timezone: "Asia/Shanghai" },
    Temp: { Enabled: true, Day: 6500, Night: 3400, StartBeforeSunsetMin: 120, SettleAfterSunsetMin: 30 },
    Brightness: { Enabled: true, DayRatio: 1, NightRatio: 0.45, FloorRatio: 0.3, StepThreshold: 0.03 },
    Theme: { Enabled: true, Mode: "follow_sky", DarkAt: 4500, LightAt: 5200 },
    Schedule: { Mode: "sun", Fixed: { NightStart: "19:00", NightEnd: "07:00" } },
    Displays: {},
    UI: { Theme: "auto", AccentDrift: true, LastPage: "main" },
    App: { Autostart: false, PauseUntil: "", FirstRun: firstRun },
  };
}

// A slice of the real table, enough to exercise the picker's scroll and the
// province column.
export const previewCities: City[] = [
  { Name: "北京", Province: "北京", Latitude: 39.9042, Longitude: 116.4074, Timezone: "Asia/Shanghai" },
  { Name: "上海", Province: "上海", Latitude: 31.2304, Longitude: 121.4737, Timezone: "Asia/Shanghai" },
  { Name: "广州", Province: "广东", Latitude: 23.1291, Longitude: 113.2644, Timezone: "Asia/Shanghai" },
  { Name: "深圳", Province: "广东", Latitude: 22.5431, Longitude: 114.0579, Timezone: "Asia/Shanghai" },
  { Name: "成都", Province: "四川", Latitude: 30.5728, Longitude: 104.0668, Timezone: "Asia/Shanghai" },
  { Name: "杭州", Province: "浙江", Latitude: 30.2741, Longitude: 120.1551, Timezone: "Asia/Shanghai" },
  { Name: "南京", Province: "江苏", Latitude: 32.0603, Longitude: 118.7969, Timezone: "Asia/Shanghai" },
  { Name: "武汉", Province: "湖北", Latitude: 30.5928, Longitude: 114.3055, Timezone: "Asia/Shanghai" },
  { Name: "西安", Province: "陕西", Latitude: 34.3416, Longitude: 108.9398, Timezone: "Asia/Shanghai" },
  { Name: "重庆", Province: "重庆", Latitude: 29.563, Longitude: 106.5516, Timezone: "Asia/Shanghai" },
  { Name: "哈尔滨", Province: "黑龙江", Latitude: 45.8038, Longitude: 126.535, Timezone: "Asia/Shanghai" },
  { Name: "乌鲁木齐", Province: "新疆", Latitude: 43.8256, Longitude: 87.6168, Timezone: "Asia/Shanghai" },
];

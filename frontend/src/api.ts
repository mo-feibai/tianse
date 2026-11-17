// The only place the UI touches the backend.
//
// Every call is wrapped so a failing engine cannot leave a component in a
// half-updated state: the caller always gets a value or a logged error, never an
// unhandled rejection.

import * as Service from "../bindings/tianse/internal/service/service.js";
import { previewAxis, previewCities, previewConfig, previewRequested, previewStatus } from "./mock";

export interface Status {
  Kelvin: number;
  Phase: string;
  Progress: number;
  Sunset: string;
  Sunrise: string;
  NextChange: string;
  NextChangeK: number;
  Paused: boolean;
  Polar: boolean;
  BrightnessOn: boolean;
  BrightnessOK: boolean;
  BrightnessPercent: number;
  ThemeOn: boolean;
  ThemeDark: boolean;
  ThemeMode: string;
  SurfaceDark: boolean;
  Accent: string;
  Ink: string;
  Xiang: string;
  City: string;
  DisplayName: string;
  DisplayNum: number;
  Warning: string;
  GammaOK: boolean;
  Manual: boolean;
  /** A full-screen app owns the screen; the engine is holding its last frame. */
  Held: boolean;
}

export interface AxisStop {
  Offset: number;
  Color: string;
  Kelvin: number;
}

export interface AxisTick {
  Offset: number;
  Label: string;
}

export interface Axis {
  Stops: AxisStop[] | null;
  Ticks: AxisTick[] | null;
  Now: number;
  NowText: string;
  Start: string;
  End: string;
  /** Where the sun crosses the horizon inside the band, 0..1. The band spans
   * the shoulders of the curve, so these are interior marks, not the ends. */
  SunsetOffset: number;
  SunriseOffset: number;
  SunsetText: string;
  SunriseText: string;
  Polar: boolean;
  PolarColor: string;
  Empty: boolean;
}

export interface City {
  Name: string;
  Province: string;
  Latitude: number;
  Longitude: number;
  Timezone: string;
}

export interface Display {
  Device: string;
  Description: string;
  Primary: boolean;
  Width: number;
  Height: number;
  BrightnessSupported: boolean;
  BrightnessMin: number;
  BrightnessCurrent: number;
  BrightnessMax: number;
}

export const emptyStatus = (): Status => ({
  Kelvin: 6500,
  Phase: "day",
  Progress: 0,
  Sunset: "",
  Sunrise: "",
  NextChange: "",
  NextChangeK: 0,
  Paused: false,
  Polar: false,
  BrightnessOn: false,
  BrightnessOK: false,
  BrightnessPercent: -1,
  ThemeOn: false,
  ThemeDark: false,
  ThemeMode: "follow_sky",
  SurfaceDark: true,
  Accent: "#5AA4AE",
  Ink: "#879BAA",
  Xiang: "#D9A441",
  City: "",
  DisplayName: "",
  DisplayNum: 1,
  Warning: "",
  GammaOK: true,
  Manual: false,
  Held: false,
});

export async function fetchStatus(): Promise<Status | null> {
  if (previewRequested()) return previewStatus();
  try {
    return (await Service.Status()) as unknown as Status;
  } catch (err) {
    console.error("status", err);
    return null;
  }
}

export async function fetchAxis(): Promise<Axis | null> {
  if (previewRequested()) return previewAxis();
  try {
    return (await Service.Axis()) as unknown as Axis;
  } catch (err) {
    console.error("axis", err);
    return null;
  }
}

export async function fetchConfig(): Promise<any | null> {
  if (previewRequested()) return previewConfig();
  try {
    return await Service.Config();
  } catch (err) {
    console.error("config", err);
    return null;
  }
}

export async function saveConfig(cfg: any): Promise<boolean> {
  if (previewRequested()) return true;
  try {
    await Service.SaveConfig(cfg);
    return true;
  } catch (err) {
    console.error("save config", err);
    return false;
  }
}

export async function setManualTemperature(kelvin: number): Promise<void> {
  try {
    await Service.SetManualTemperature(kelvin);
  } catch (err) {
    console.error("manual temperature", err);
  }
}

export async function resetGamma(): Promise<void> {
  try {
    await Service.ResetGamma();
  } catch (err) {
    console.error("reset gamma", err);
  }
}

export async function togglePaused(): Promise<boolean> {
  try {
    return await Service.TogglePaused();
  } catch (err) {
    console.error("toggle paused", err);
    return false;
  }
}

export async function fetchAutostart(): Promise<boolean> {
  if (previewRequested()) return false;
  try {
    return await Service.Autostart();
  } catch (err) {
    console.error("autostart", err);
    return false;
  }
}

export async function setAutostart(enabled: boolean): Promise<boolean> {
  if (previewRequested()) return enabled;
  try {
    await Service.SetAutostart(enabled);
    return enabled;
  } catch (err) {
    console.error("set autostart", err);
    return !enabled;
  }
}

export async function fetchCities(): Promise<City[]> {
  if (previewRequested()) return previewCities;
  try {
    return (await Service.Cities()) as unknown as City[];
  } catch (err) {
    console.error("cities", err);
    return [];
  }
}

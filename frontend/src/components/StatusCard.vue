<script setup lang="ts">
// Where the machine reports back. Everything here is a fact about the current
// state, not a control.
import { computed } from "vue";
import type { Status } from "../api";

const props = defineProps<{ status: Status }>();

const phase = computed(() => {
  if (props.status.Paused) return "已暂停";
  if (props.status.Held) return "全屏中";
  if (props.status.Polar) return "极昼极夜";
  if (props.status.Manual) return "手动";
  switch (props.status.Phase) {
    case "day":
      return "白天";
    case "evening":
      return "入夜";
    case "night":
      return "夜间";
    case "morning":
      return "破晓";
    default:
      return "—";
  }
});

const next = computed(() => {
  if (props.status.Paused) return "恢复后继续";
  // While holding, the clock is not the useful fact — that the screen is being
  // left alone is. The time is still shown again the moment the app closes.
  if (props.status.Held) return "让出屏幕";
  if (!props.status.NextChange) return "—";
  return props.status.NextChange;
});


</script>

<template>
  <section class="card">
    <div class="block">
      <span class="label">下次切换</span>
      <span class="value">{{ next }}</span>
    </div>
    <div class="block">
      <span class="label">状态 · 显示器</span>
      <span class="sub ellipsis">{{ phase }} · {{ status.DisplayName || "—" }}</span>
    </div>
    <p v-if="status.Warning" class="warn">{{ status.Warning }}</p>
  </section>
</template>

<style scoped>
.card {
  width: 240px;
  height: 176px;
  border-radius: 12px;
  padding: 24px;
  position: relative;
  overflow: hidden;
  background: var(--sunk);
  box-shadow: var(--edge);
  display: flex;
  flex-direction: column;
  gap: 12px;
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

.block {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.label {
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
}

.value {
  font-family: var(--font-display);
  font-size: 28px;
  line-height: 1.1;
  color: var(--text);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* Two facts on one line rather than two labelled rows: 240 px is not wide
   enough for three label/value pairs without clipping the monitor name. */
.sub {
  font-family: var(--font-text);
  font-size: 14px;
  line-height: 1.7;
  color: var(--text);
}

.ellipsis {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.warn {
  position: relative;
  z-index: 1;
  margin: 0;
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--xiang);
}
</style>

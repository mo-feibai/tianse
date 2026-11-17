<script setup lang="ts">
// The three switches, and nothing else. Each one is a whole feature turned on or
// off; none of them is a preference in disguise.
import SwitchRow from "./SwitchRow.vue";

defineProps<{ temp: boolean; brightness: boolean; theme: boolean; brightnessOk: boolean }>();
const emit = defineEmits<{
  (e: "update:temp", v: boolean): void;
  (e: "update:brightness", v: boolean): void;
  (e: "update:theme", v: boolean): void;
}>();
</script>

<template>
  <section class="card">
    <SwitchRow
      label="色温曲线"
      :model-value="temp"
      @update:model-value="emit('update:temp', $event)"
    />
    <SwitchRow
      label="自动亮度"
      :hint="brightnessOk ? undefined : '此屏不支持'"
      :model-value="brightness"
      @update:model-value="emit('update:brightness', $event)"
    />
    <SwitchRow
      label="明暗跟随"
      :model-value="theme"
      @update:model-value="emit('update:theme', $event)"
    />
  </section>
</template>

<style scoped>
.card {
  width: 460px;
  height: 176px;
  border-radius: 12px;
  padding: 24px 32px;
  position: relative;
  overflow: hidden;
  background: var(--surface);
  box-shadow: var(--edge);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
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
</style>

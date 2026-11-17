<script setup lang="ts">
// A hand-drawn switch: the native control cannot be tinted, and its 20px height
// matches nothing else in the window.
defineProps<{ label: string; modelValue: boolean; hint?: string }>();
const emit = defineEmits<{ (e: "update:modelValue", v: boolean): void }>();
</script>

<template>
  <div class="row" role="switch" :aria-checked="modelValue" tabindex="0"
       @click="emit('update:modelValue', !modelValue)"
       @keydown.enter.prevent="emit('update:modelValue', !modelValue)"
       @keydown.space.prevent="emit('update:modelValue', !modelValue)">
    <span class="label">{{ label }}</span>
    <span v-if="hint" class="hint">{{ hint }}</span>
    <span class="track" :class="{ on: modelValue }">
      <span class="dot" />
    </span>
  </div>
</template>

<style scoped>
.row {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 40px;
  cursor: pointer;
  outline: none;
}

.label {
  font-family: var(--font-text);
  font-size: 14px;
  line-height: 1.7;
  color: var(--text);
  flex: 1;
}

.hint {
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
}

.track {
  position: relative;
  width: 40px;
  height: 20px;
  border-radius: 10px;
  background: var(--sunk);
  box-shadow: var(--edge);
  transition: background 160ms var(--ease);
  flex: none;
}

.track.on {
  background: var(--accent);
}

.dot {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--ink);
  transition: transform 160ms var(--ease), background 160ms var(--ease);
}

.track.on .dot {
  transform: translateX(20px);
  background: #ffffff;
}

.row:focus-visible .track {
  box-shadow: 0 0 0 2px var(--accent);
}
</style>

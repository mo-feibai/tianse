<script setup lang="ts">
// Exclusive choice drawn as text with an underline rather than a filled pill.
//
// A pill inside a card would need a radius smaller than the card's, and the
// design forbids nesting radii. An underline is a 2px bar — a line, so it needs
// no radius at all, and it keeps the row to a single text height.
defineProps<{ options: { value: string; label: string }[]; modelValue: string }>();
const emit = defineEmits<{ (e: "update:modelValue", v: string): void }>();
</script>

<template>
  <div class="seg">
    <button
      v-for="o in options"
      :key="o.value"
      class="item"
      :class="{ on: o.value === modelValue }"
      @click="emit('update:modelValue', o.value)"
    >
      {{ o.label }}
    </button>
  </div>
</template>

<style scoped>
.seg {
  display: flex;
  gap: 20px;
  height: 32px;
}

.item {
  position: relative;
  border: 0;
  padding: 0 0 8px;
  background: transparent;
  font-family: var(--font-text);
  font-size: 14px;
  line-height: 1.7;
  color: var(--ink);
  cursor: pointer;
  transition: color 160ms var(--ease);
}

.item:hover {
  color: var(--text);
}

.item.on {
  color: var(--text);
}

.item.on::after {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 2px;
  background: var(--accent);
}

.item:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
</style>

<script setup lang="ts">
// The city list. A native <select> cannot be styled in WebView2 — the popup is
// drawn by the OS — so the list is ours: same card material, 8px radius, a 1px
// outline instead of a shadow (DESIGN 2.6 forbids drop shadows).
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import type { City } from "../api";

const props = defineProps<{ cities: City[]; modelValue: string }>();
const emit = defineEmits<{ (e: "update:modelValue", v: string): void }>();

const open = ref(false);
const root = ref<HTMLElement | null>(null);

const current = computed(() => props.cities.find((c) => c.Name === props.modelValue));

function pick(c: City) {
  open.value = false;
  emit("update:modelValue", c.Name);
}

// Close on any press outside. Pointerdown rather than click so the panel closes
// before the press lands on whatever is underneath.
function onOutside(ev: PointerEvent) {
  if (root.value && !root.value.contains(ev.target as Node)) open.value = false;
}

function onKey(ev: KeyboardEvent) {
  if (ev.key === "Escape") open.value = false;
}

onMounted(() => document.addEventListener("pointerdown", onOutside));
onBeforeUnmount(() => document.removeEventListener("pointerdown", onOutside));
</script>

<template>
  <div ref="root" class="picker" @keydown="onKey">
    <button class="trigger" :class="{ open }" @click="open = !open">
      <span class="name">{{ current?.Name ?? modelValue ?? "选择城市" }}</span>
      <span class="chev" aria-hidden="true">
        <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
          <path d="M2 4l3 3 3-3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
        </svg>
      </span>
    </button>

    <div v-if="open" class="panel">
      <button
        v-for="c in cities"
        :key="c.Name"
        class="row"
        :class="{ on: c.Name === modelValue }"
        @click="pick(c)"
      >
        <span class="row-name">{{ c.Name }}</span>
        <span class="row-prov">{{ c.Province }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.picker {
  position: relative;
  width: 100%;
}

.trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  height: 32px;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  background: var(--raised);
  box-shadow: var(--edge);
  color: var(--text);
  font-family: var(--font-text);
  font-size: 14px;
  cursor: pointer;
  transition: background 160ms var(--ease);
}

.trigger:hover,
.trigger.open {
  background: var(--pressed);
}

.name {
  min-width: 0;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chev {
  display: grid;
  place-items: center;
  color: var(--ink);
  transition: transform 160ms var(--ease);
}

.trigger.open .chev {
  transform: rotate(180deg);
}

/* 浮层：L3 raised, 8px radius, 1px outline. Sits above everything. */
.panel {
  position: absolute;
  left: 0;
  top: 40px;
  z-index: 20;
  width: 100%;
  min-width: 192px;
  max-height: 264px;
  overflow-y: auto;
  padding: 8px;
  border-radius: 8px;
  background: var(--raised);
  outline: 1px solid var(--shadow);
  outline-offset: -1px;
}

.panel::-webkit-scrollbar {
  width: 8px;
}

.panel::-webkit-scrollbar-thumb {
  background: var(--pressed);
  border-radius: 4px;
}

.row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  width: 100%;
  height: 32px;
  padding: 0 8px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--text);
  font-family: var(--font-text);
  font-size: 14px;
  cursor: pointer;
  transition: background 120ms var(--ease);
}

.row:hover {
  background: var(--surface);
}

.row.on .row-name {
  color: var(--accent);
}

.row-prov {
  font-size: 12px;
  color: var(--ink);
}
</style>

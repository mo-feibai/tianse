<script setup lang="ts">
// The window is frameless, so this row is both the title and the drag handle.
//
// The left slot is the only thing that changes between pages: the wordmark on the
// main page, a back arrow once the settings are open. Navigation lives on the
// right, next to the window buttons, because that is where the eye already is
// when it is looking for "the way out".
import { Window } from "@wailsio/runtime";

defineProps<{ page: "main" | "settings" }>();
const emit = defineEmits<{ (e: "navigate", page: "main" | "settings"): void }>();

function minimise() {
  Window.Minimise();
}

// Close means "hide to the tray" (SPEC decision 9). The Go side cancels the
// close event, so this is the same path the X button has always taken.
function hide() {
  Window.Hide();
}
</script>

<template>
  <header class="bar">
    <button v-if="page === 'settings'" class="back" @click="emit('navigate', 'main')">
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
        <path d="M8.5 3.5L5 7l3.5 3.5" stroke="currentColor" stroke-width="1.2"
              stroke-linecap="round" stroke-linejoin="round" />
      </svg>
      <span class="mark">天色</span>
    </button>
    <span v-else class="mark">天色</span>

    <nav class="actions">
      <button v-if="page === 'main'" class="icon" title="设置" @click="emit('navigate', 'settings')">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <circle cx="7" cy="7" r="2.3" stroke="currentColor" stroke-width="1.1" />
          <path d="M7 1.3v1.5M7 11.2v1.5M1.3 7h1.5M11.2 7h1.5M3.0 3.0l1.05 1.05M9.95 9.95L11 11M11 3.0l-1.05 1.05M4.05 9.95L3 11"
                stroke="currentColor" stroke-width="1.1" stroke-linecap="round" />
        </svg>
      </button>
      <button class="icon" title="最小化" @click="minimise">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <path d="M3 7h8" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" />
        </svg>
      </button>
      <button class="icon" title="隐藏到托盘" @click="hide">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <path d="M3.5 3.5l7 7M10.5 3.5l-7 7" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" />
        </svg>
      </button>
    </nav>
  </header>
</template>

<style scoped>
.bar {
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px 0 90px;
  --wails-draggable: drag;
  flex: none;
}

.mark {
  font-family: var(--font-display);
  font-size: 26px;
  line-height: 1.2;
  letter-spacing: 0.08em;
  color: var(--text);
}

/* The back button carries the same wordmark, so the product never disappears
   from the window; only a 14px chevron marks the way back. */
.back {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: -18px;
  padding: 0 4px;
  border: 0;
  background: transparent;
  color: var(--ink);
  cursor: pointer;
  --wails-draggable: no-drag;
  transition: color 160ms var(--ease);
}

.back:hover {
  color: var(--text);
}

.back .mark {
  color: inherit;
}

.actions {
  display: flex;
  gap: 4px;
  --wails-draggable: no-drag;
}

.icon {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border: 0;
  padding: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--ink);
  cursor: pointer;
  transition: background 160ms var(--ease), color 160ms var(--ease);
}

.icon:hover {
  background: var(--raised);
  color: var(--text);
}

.icon:active {
  background: var(--pressed);
}
</style>

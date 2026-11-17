<script setup lang="ts">
// Shown once, on the first launch, in the reading card's slot — same 720x128
// rect, so the 620 px budget below it does not move (DESIGN 4.1).
//
// It replaces the reading card rather than floating over it: before the city is
// confirmed, the kelvin number above is a reading of the wrong sun. The app
// already works on the default city, so this is a correction offer, not a setup
// step — both questions are pre-answered and "开始使用" alone is a valid answer.
defineProps<{ city: string; autostart: boolean; brightness: boolean }>();
const emit = defineEmits<{
  (e: "city"): void;
  (e: "autostart", v: boolean): void;
  (e: "brightness", v: boolean): void;
  (e: "done"): void;
}>();
</script>

<template>
  <section class="card">
    <p class="say">
      天色按<strong class="place">{{ city || "北京" }}</strong>的日落工作
    </p>

    <div class="foot">
      <button class="link" type="button" @click="emit('city')">换个城市</button>
      <span class="sep" aria-hidden="true">·</span>
      <span class="small">自动亮度</span>
      <button
        class="track"
        :class="{ on: brightness }"
        type="button"
        role="switch"
        :aria-checked="brightness"
        @click="emit('brightness', !brightness)"
      >
        <span class="dot" />
      </button>
      <span class="sep" aria-hidden="true">·</span>
      <span class="small">开机自启</span>
      <button
        class="track"
        :class="{ on: autostart }"
        type="button"
        role="switch"
        :aria-checked="autostart"
        @click="emit('autostart', !autostart)"
      >
        <span class="dot" />
      </button>
      <button class="done" type="button" @click="emit('done')">开始使用</button>
    </div>
  </section>
</template>

<style scoped>
/* L3 raised with a drawn outline instead of a shadow: it is a layer above the
   cards it sits among, and this app has no shadows anywhere (DESIGN 2.6). */
.card {
  flex: none;
  width: 720px;
  height: 128px;
  box-sizing: border-box;
  padding: 24px 32px;
  border-radius: 12px;
  background: var(--raised);
  box-shadow: var(--edge);
  outline: 1px solid var(--shadow);
  outline-offset: -1px;
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

/* Same two material layers as every other card (DESIGN 2.6): the lit gradient
   and the grain. They carry their own radius because the card cannot clip. */
.card::before,
.card::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 12px;
  pointer-events: none;
}

.card::before {
  background: var(--sheen);
}

.card::after {
  opacity: 0.025;
  background-image: url("/textures/noise-128.png");
  background-size: 128px 128px;
}

/* The title step (18px, 京华老宋体) — this card holds one sentence, and it is
   the only thing on the first launch the user has to read. */
.say {
  margin: 0;
  position: relative;
  z-index: 1;
  font-family: var(--font-display);
  font-size: 18px;
  line-height: 1.4;
  color: var(--text);
}

.place {
  font-weight: 400;
  color: var(--accent);
}

.foot {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 12px;
}

.link,
.done {
  border: 0;
  background: none;
  padding: 0;
  cursor: pointer;
  font-family: var(--font-text);
  font-size: 14px;
  line-height: 1.7;
  color: var(--accent);
  white-space: nowrap;
}

.link:hover,
.done:hover {
  text-decoration: underline;
  text-underline-offset: 4px;
}

/* The primary action is the last thing in the row, and the row is right-aligned
   against the card edge — the same edge the brightness reading sits on. */
.done {
  margin-left: auto;
  color: var(--accent);
}

.sep {
  font-family: var(--font-text);
  font-size: 12px;
  color: var(--ink);
  opacity: 0.5;
}

.small {
  font-family: var(--font-text);
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
  white-space: nowrap;
}

/* Same geometry as SwitchRow: one action must not look like two things. */
.track {
  position: relative;
  width: 40px;
  height: 20px;
  border: 0;
  border-radius: 10px;
  padding: 0;
  cursor: pointer;
  background: var(--sunk);
  box-shadow: var(--edge);
  transition: background 160ms var(--ease);
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

.track:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
</style>

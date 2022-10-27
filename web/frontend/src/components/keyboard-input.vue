<!-- eslint-disable id-length -->
<script setup lang="ts">

export type Keys = 'w' | 'a' | 's' | 'd'

import { onClickOutside } from '@vueuse/core';
import { mdiArrowUp as w, mdiRestore as a, mdiReload as d, mdiArrowDown as s } from '@mdi/js';
import Icon from './icon.vue';
import { onUnmounted } from 'vue';

interface Emits {
  (event: 'keydown', key: Keys): void
  (event: 'keyup', key: Keys): void
  (event: 'toggle', active: boolean): void
}

const emit = defineEmits<Emits>();

const keyIcons = { w, a, s, d };
const root = $ref<HTMLElement>();

const pressedKeys = $ref({
  w: false,
  a: false,
  s: false,
  d: false,
});

let isActive = $ref(false);

const keysLayout = [['w'], ['a', 's', 'd']] as const;

const normalizeKey = (key: string): Keys | null => {
  return ({
    w: 'w',
    a: 'a',
    s: 's',
    d: 'd',
    arrowup: 'w',
    arrowleft: 'a',
    arrowdown: 's',
    arrowright: 'd',
  } as Record<string, Keys>)[key.toLowerCase()] ?? null;
};

const emitKeyDown = (key: Keys) => {
  if (!isActive) {
    // eslint-disable-next-line no-use-before-define
    toggleKeyboard(true);
  }
  pressedKeys[key] = true;
  emit('keydown', key);
};

const emitKeyUp = (key: Keys) => {
  pressedKeys[key] = false;
  emit('keyup', key);
};

const handleKeyDown = (event: KeyboardEvent) => {
  event.preventDefault();
  event.stopPropagation();

  const key = normalizeKey(event.key);

  if (key === null || pressedKeys[key]) {
    return;
  }

  emitKeyDown(key);
};

const handleKeyUp = (event: KeyboardEvent) => {
  event.preventDefault();
  event.stopPropagation();

  const key = normalizeKey(event.key);
  if (key !== null) {
    emitKeyUp(key);
  }
};

const toggleKeyboard = (nowActive: boolean) => {
  if (nowActive) {
    window.addEventListener('keydown', handleKeyDown, false);
    window.addEventListener('keyup', handleKeyUp, false);
  } else {
    window.removeEventListener('keydown', handleKeyDown);
    window.removeEventListener('keyup', handleKeyUp);
  }

  isActive = nowActive;
  emit('toggle', isActive);
};

const handlePointerDown = (key: Keys) => {
  emitKeyDown(key);
  window.addEventListener('pointerup', () => emitKeyUp(key), { once: true });
};

onClickOutside($$(root), () => {
  toggleKeyboard(false);
});

onUnmounted(() => {
  toggleKeyboard(false);
});

</script>

<template>
  <div
    ref="root"
    class="h-23 flex w-full flex-col items-center"
  >
    <div class="flex w-48 gap-2 pb-4">
      <v-switch
        :label="isActive ? 'Keyboard Enabled' : 'Keyboard Disabled'"
        class="pr-4"
        :value="isActive ? 'on' : 'off'"
        @input="toggleKeyboard($event.target.value === 'on')"
      />
    </div>
    <div
      v-for="(lineKeys, index) in keysLayout"
      :key="index"
      class="my-1 flex flex-row justify-center gap-2"
    >
      <button
        v-for="key in lineKeys"
        :key="key"
        class="flex items-center gap-1.5 border border-gray-500 px-3 py-1 outline-none"
        :class="{
          'bg-gray-200 text-gray-800': pressedKeys[key],
          'bg-white': !pressedKeys[key],
        }"
        @pointerdown="handlePointerDown(key)"
      >
        {{ key.toUpperCase() }}
        <Icon :path="keyIcons[key]" />
      </button>
    </div>
  </div>
</template>

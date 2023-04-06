<!-- eslint-disable id-length -->
<script setup lang="ts">

export type Keys = 'w' | 'a' | 's' | 'd'

import { $ref } from 'vue/macros';
import { mdiArrowUp as w, mdiRestore as a, mdiReload as d, mdiArrowDown as s } from '@mdi/js';
import Icon from './icon.vue';
import { watch, onUnmounted } from 'vue';

const emit = defineEmits<{(event: 'keydown', key: Keys): void
  (event: 'keyup', key: Keys): void
  (event: 'toggle', active: boolean): void
  (event: 'update-keyboard-state', value: boolean): void
}>();

const props = defineProps<{
  isActive: boolean,
}>();

const keyIcons = { w, a, s, d };

const pressedKeys = $ref({
  w: false,
  a: false,
  s: false,
  d: false,
});

const keysLayout = [['a'], ['w', 's'], ['d']] as const;

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

  emit('update-keyboard-state', nowActive);
  emit('toggle', nowActive);
};

const handlePointerDown = (key: Keys) => {
  emitKeyDown(key);
};

const handlePointerUp = (key: Keys) => {
  emitKeyUp(key);
};

watch(() => props.isActive, (active) => {
  if (!active) {
    toggleKeyboard(false);
  }
});

onUnmounted(() => {
  toggleKeyboard(false);
});

</script>

<template>
  <div>
    <v-switch
      :label="props.isActive ? 'Keyboard Enabled' : 'Keyboard Disabled'"
      class="w-fit pr-4"
      :value="props.isActive ? 'on' : 'off'"
      @input="toggleKeyboard($event.detail.value)"
    />
    <div class="flex justify-center gap-2">
      <div
        v-for="(lineKeys, index) in keysLayout"
        :key="index"
        class="my-1 flex flex-col gap-2 self-end"
      >
        <button
          v-for="key in lineKeys"
          :key="key"
          class="flex select-none items-center gap-1.5 border border-gray-500 px-3 py-1 uppercase outline-none"
          :class="{
            'bg-gray-200 text-gray-800': pressedKeys[key],
            'bg-white': !pressedKeys[key],
          }"
          @pointerdown="handlePointerDown(key)"
          @pointerup="handlePointerUp(key)"
          @pointerleave="handlePointerUp(key)"
        >
          {{ key }}
          <Icon :path="keyIcons[key]" />
        </button>
      </div>
    </div>
  </div>
</template>

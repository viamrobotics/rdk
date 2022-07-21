<script setup lang="ts">

import { ref } from 'vue';
import { onClickOutside } from '@vueuse/core';
import { mdiRestore, mdiReload, mdiArrowUp, mdiArrowDown } from '@mdi/js';
import Icon from './icon.vue';

interface Emits {
  (event: 'keyboard-ctl', pressedKeys: Record<string, boolean>): void
}

const emit = defineEmits<Emits>();

const pressedKeysMap = {
  w: 'forward',
  s: 'backward',
  a: 'left',
  d: 'right',
} as const;

const pressedKeys = ref({
  forward: false,
  left: false,
  backward: false,
  right: false,
});

const root = ref();
const isActive = ref(false);

const keyLetters = {
  forward: 'W',
  left: 'A',
  backward: 'S',
  right: 'D',
};

const keyIcons = {
  forward: mdiArrowUp,
  left: mdiRestore,
  backward: mdiArrowDown,
  right: mdiReload,
};

const keysLayout = [['forward'], ['left', 'backward', 'right']] as const;

const setKeyPressed = (key: keyof typeof pressedKeys.value, value = true) => {
  pressedKeys.value[key] = value;
  emit('keyboard-ctl', pressedKeys.value);
};

const onUseKeyboardNav = (event: KeyboardEvent) => {
  const key = pressedKeysMap[event.key as 'w' | 's' | 'a' | 'd'];

  if (!key) {
    return; 
  }

  setKeyPressed(key, event.type === 'keydown');
  event.preventDefault();
};

const toggleKeyboard = () => {
  if (isActive.value) {
    removeKeyboardListeners();
  } else {
    addKeyboardListeners();
  }
};

const addKeyboardListeners = () => {
  isActive.value = true;
  window.addEventListener('keydown', onUseKeyboardNav, false);
  window.addEventListener('keyup', onUseKeyboardNav, false);
};

const removeKeyboardListeners = () => {
  isActive.value = false;
  window.removeEventListener('keydown', onUseKeyboardNav);
  window.removeEventListener('keyup', onUseKeyboardNav);
};

onClickOutside(root, () => {
  removeKeyboardListeners();
});

</script>

<template>
  <div
    ref="root"
    class="h-23 flex w-full flex-col items-center"
  >
    <div
      class="flex gap-2 pb-4"
      @click="toggleKeyboard"
    >
      <v-switch
        class="pr-4"
        :value="isActive ? 'on' : 'off'"
      />
      <h3>
        Keyboard {{ isActive ? 'active' : 'disabled' }}
      </h3>
    </div>
    <div
      v-for="(lineKeys, index) in keysLayout"
      :key="index"
      class="my-1 flex flex-row justify-center gap-2"
    >
      <button
        v-for="key in lineKeys"
        :key="key"
        class="flex items-center gap-1.5 border border-gray-500 bg-white px-3 py-1 dark:bg-gray-900"
        :class="{
          'bg-gray-200 dark:bg-gray-800 text-gray-800 dark:text-gray-200 keyboard-button-pressed': pressedKeys[key],
          'keyboard-button-not-pressed': !pressedKeys[key],
        }"
        @pointerdown="setKeyPressed(key, true)"
        @pointerup="setKeyPressed(key, false)"
      >
        {{ keyLetters[key] }}
        <Icon :path="keyIcons[key]" />
      </button>
    </div>
  </div>
</template>

<style>

.keyboard-button-not-pressed:focus {
  background-color: white;
}
.keyboard-button-pressed:focus {
  background-color: rgba(228, 228, 231, 1);
}

</style>

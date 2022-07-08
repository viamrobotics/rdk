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
  87: 'forward',
  83: 'backward',
  65: 'left',
  68: 'right',
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
  const key = pressedKeysMap[event.keyCode];

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
    class="h-23 flex flex-col"
  >
    <div class="flex pb-4">
      <v-switch
        class="pr-4"
        :value="isActive ? 'on' : 'off'"
        @input="toggleKeyboard()"
      />
      <h3 v-if="isActive">
        Keyboard active
      </h3>
      <h3 v-else>
        Keyboard disabled
      </h3>
    </div>
    <div
      v-for="(lineKeys, index) in keysLayout"
      :key="index"
      class="flex flex-row justify-center"
    >
      <v-button
        v-for="key in lineKeys"
        :key="key"
        icon="check"
        :label="keyLetters[key]"
        class="w-15 m-1 h-10 border-gray-500 bg-white dark:bg-gray-900"
        :class="{
          'bg-gray-200 dark:bg-gray-800 text-gray-800 dark:text-gray-200 keyboard-button-pressed':
            pressedKeys[key],
          'keyboard-button-not-pressed': !pressedKeys[key],
        }"
        @pointerdown="setKeyPressed(key, true)"
        @pointerup="setKeyPressed(key, false)"
      >
        <Icon :path="keyIcons[key]">
          Check
        </Icon>
      </v-button>
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

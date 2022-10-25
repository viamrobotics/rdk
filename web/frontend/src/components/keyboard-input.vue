<script setup lang="ts">

import { onClickOutside } from '@vueuse/core';
import { mdiRestore, mdiReload, mdiArrowUp, mdiArrowDown } from '@mdi/js';
import Icon from './icon.vue';
import { grpc } from '@improbable-eng/grpc-web';
import { displayError } from '../lib/error';
import baseApi from '../gen/proto/api/component/base/v1/base_pb.esm';

interface Emits {
  (event: 'keyboard-ctl', pressedKeys: Record<string, boolean>): void
}

interface Props {
  name: string;
}

const emit = defineEmits<Emits>();
const props = defineProps<Props>();

const pressedKeys = $ref({
  forward: false,
  left: false,
  backward: false,
  right: false,
});

const root = $ref<HTMLElement>();
let isActive = $ref(false);

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

const setKeyPressed = (key: 'forward' | 'left' | 'backward' | 'right', value = true) => {
  pressedKeys[key] = value;
  emit('keyboard-ctl', pressedKeys);
};

const onUseKeyboardNav = (event: KeyboardEvent) => {
  event.preventDefault();

  const down = event.type === 'keydown';

  switch (event.key.toLowerCase()) {
    case 'arrowleft':
    case 'a': {
      return setKeyPressed('left', down);
    }
    case 'arrowright':
    case 'd': {
      return setKeyPressed('right', down);
    }
    case 'arrowup':
    case 'w': {
      return setKeyPressed('forward', down);
    }
    case 'arrowdown':
    case 's': {
      return setKeyPressed('backward', down);
    }
  }
};

const addKeyboardListeners = () => {
  isActive = true;
  window.addEventListener('keydown', onUseKeyboardNav, false);
  window.addEventListener('keyup', onUseKeyboardNav, false);
};

const removeKeyboardListeners = () => {
  isActive = false;
  window.removeEventListener('keydown', onUseKeyboardNav);
  window.removeEventListener('keyup', onUseKeyboardNav);
};

const stopBase = () => {
  const req = new baseApi.StopRequest();
  req.setName(props.name);
  window.baseService.stop(req, new grpc.Metadata(), displayError);
};

const toggleKeyboard = () => {
  if (isActive) {
    removeKeyboardListeners();
    stopBase();
  } else {
    addKeyboardListeners();
  }
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
      class="flex w-48 gap-2 pb-4"
      @click="toggleKeyboard"
    >
      <v-switch
        :label="isActive ? 'Keyboard Enabled' : 'Keyboard Disabled'"
        class="pr-4"
        :value="isActive ? 'on' : 'off'"
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
        class="flex items-center gap-1.5 border border-gray-500 bg-white px-3 py-1"
        :class="{
          'bg-gray-200 text-gray-800 keyboard-button-pressed': pressedKeys[key],
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

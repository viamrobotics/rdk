<script setup lang="ts">
import { ref } from 'vue';
import { throttle, debounce } from "lodash";
import { mdiRestore, mdiReload, mdiArrowUp, mdiArrowDown } from "@mdi/js";
// import ViamIcon from "./icon.vue";

interface Emits {
  (event: "keyboard-ctl"): void
}

const emit = defineEmits<Emits>()

const PressedKeysMap = {
  "87": "forward",
  "83": "backward",
  "65": "left",
  "68": "right",
} as const;

// TODO: remove debounce if not needed
const inputDelay = 0;
const eventsDelay = 0;

const pressedKeys = {
  forward: false,
  left: false,
  backward: false,
  right: false,
} as const;

const isActive = ref(false);

const keyLetters = {
  forward: "W",
  left: "A",
  backward: "S",
  right: "D",
};

const keyIcons = {
  forward: mdiArrowUp,
  left: mdiRestore,
  backward: mdiArrowDown,
  right: mdiReload,
};

  //for template section
const keysLayout = [["forward"], ["left", "backward", "right"]];

const sendKeysState = debounce(() => {
  emit("keyboard-ctl");
}, inputDelay);

const emitEvent = throttle((eventName: string) => {
  this.emitEventInstantly(eventName);
}, eventsDelay);

const emitEventInstantly = (eventName: string) => {
  emit(eventName, this.pressedKeys);
};
  
const setKeyPressed = (key: string, value = true) => {
  this.pressedKeys[key] = value;
  this.sendKeysState();
};

const onUseKeyboardNav = (event: KeyboardEvent) => {
  const key = PressedKeysMap[event.keyCode];
  if (!key) return;
  this.setKeyPressed(key, event.type === "keydown");
  event.preventDefault();
};

const toggleKeyboard = () => {
  if (this.const  === true) {
    this.removeKeyboardListeners();
  } else {
    this.addKeyboardListeners();
  }
}

const addKeyboardListeners = () => {
  isActive.value = true;
  window.addEventListener("keydown", this.onUseKeyboardNav, false);
  window.addEventListener("keyup", this.onUseKeyboardNav, false);
}

const removeKeyboardListeners = () => {
  isActive.value = false;
  window.removeEventListener("keydown", this.onUseKeyboardNav);
  window.removeEventListener("keyup", this.onUseKeyboardNav);
}

</script>

<template>
  <div class="flex flex-col h-23">
    <div class="flex pb-4">
      <v-switch
        :value="isActive ? 'on' : 'off'"
        class="pr-4"
        @change="toggleKeyboard()"
      />
      <h3 v-if="isActive">{{ isActive ? 'Keyboard active' : 'Keyboard disabled' }}</h3>
    </div>
    <div
      v-for="(lineKeys, index) in keysLayout"
      :key="index"
      class="flex flex-row justify-center"
    >
      <template v-for="key in lineKeys" :key="key">
        <v-button
          variant="primary"
          label="label"
          @pointerdown="setKeyPressed(this.key, true)"
          @pointerup="setKeyPressed(this.key, false)"
        >
        </v-button>
      </template>
      <!-- <ViamButton
        v-for="key in lineKeys"
        
        color="primary"
        group
        variant="primary"
        
        class="w-15 h-10 m-1 bg-white dark:bg-gray-900 border-gray-500"
        :class="{
          'bg-gray-200 dark:bg-gray-800 text-gray-800 dark:text-gray-200 keyboard-button-pressed':
            pressedKeys[key],
          'keyboard-button-not-pressed': !pressedKeys[key],
        }"
      >
        <template v-slot:icon>
          <ViamIcon :path="keyIcons[key]">Check</ViamIcon>
        </template>
        <span class="text-2xl">{{ keyLetters[key] }}</span>
      </ViamButton> -->
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

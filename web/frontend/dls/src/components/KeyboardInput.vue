<template>
  <div class="flex flex-col h-23">
    <div class="flex pb-4">
      <ViamSwitch
        class="pr-4"
        centered
        :option="isActive"
        @change="toggleKeyboard()"
      ></ViamSwitch>
      <h3 v-if="isActive">Keyboard active</h3>
      <h3 v-else>Keyboard disabled</h3>
    </div>
    <div
      v-for="(lineKeys, index) in keysLayout"
      :key="index"
      class="flex flex-row justify-center"
    >
      <ViamButton
        v-for="key in lineKeys"
        :key="key"
        color="primary"
        group
        variant="primary"
        @pointerdown="setKeyPressed(key, true)"
        @pointerup="setKeyPressed(key, false)"
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
      </ViamButton>
    </div>
  </div>
</template>
<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { throttle, debounce } from 'lodash';
import { mdiRestore, mdiReload, mdiArrowUp, mdiArrowDown } from '@mdi/js';
import ViamIcon from './ViamIcon.vue';
import ViamButton from './Button.vue';
import ViamSwitch from './Switch.vue';

const PressedKeysMap: { [index: string]: string } = {
  '87': 'forward',
  '83': 'backward',
  '65': 'left',
  '68': 'right',
};

// TODO: remove debounce if not needed
const inputDelay = 0;
const eventsDelay = 0;

@Component({
  components: {
    ViamIcon,
    ViamButton,
    ViamSwitch,
  },
})
export default class KeyboardInput extends Vue {
  pressedKeys: { [index: string]: boolean } = {
    forward: false,
    left: false,
    backward: false,
    right: false,
  };

  mdiRestore = mdiRestore;
  mdiReload = mdiReload;
  mdiArrowUp = mdiArrowUp;
  mdiArrowDown = mdiArrowDown;
  isActive = false;

  keyLetters = {
    forward: 'W',
    left: 'A',
    backward: 'S',
    right: 'D',
  };

  keyIcons = {
    forward: mdiArrowUp,
    left: mdiRestore,
    backward: mdiArrowDown,
    right: mdiReload,
  };

  //for template section
  keysLayout = [['forward'], ['left', 'backward', 'right']];

  sendKeysState = debounce(() => {
    this.handleKeysStateInstantly();
  }, inputDelay);

  handleKeysStateInstantly(): void {
    this.emitEvent('keyboard-ctl');
  }

  emitEvent = throttle((eventName: string) => {
    this.emitEventInstantly(eventName);
  }, eventsDelay);

  emitEventInstantly(eventName: string): void {
    this.$emit(eventName, this.pressedKeys);
  }
  setKeyPressed(key: string, value = true): void {
    this.pressedKeys[key] = value;
    this.sendKeysState();
  }

  onUseKeyboardNav(event: KeyboardEvent): void {
    const key = PressedKeysMap[event.keyCode];
    if (!key) return;
    this.setKeyPressed(key, event.type === 'keydown');
    event.preventDefault();
  }

  toggleKeyboard(): void {
    if (this.isActive === true) {
      this.removeKeyboardListeners();
    } else {
      this.addKeyboardListeners();
    }
  }

  addKeyboardListeners(): void {
    this.isActive = true;
    window.addEventListener('keydown', this.onUseKeyboardNav, false);
    window.addEventListener('keyup', this.onUseKeyboardNav, false);
  }

  removeKeyboardListeners(): void {
    this.isActive = false;
    window.removeEventListener('keydown', this.onUseKeyboardNav);
    window.removeEventListener('keyup', this.onUseKeyboardNav);
  }
}
</script>
<style>
.keyboard-button-not-pressed:focus {
  background-color: white;
}
.keyboard-button-pressed:focus {
  background-color: rgba(228, 228, 231, 1);
}
</style>

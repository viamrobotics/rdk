<template>
  <div class="flex flex-col w-64 h-23">
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
        @mouseup="setKeyPressed(key, false)"
        @mousedown="setKeyPressed(key, true)"
        class="w-15 h-10 m-1 bg-white dark:bg-gray-900 border-gray-500"
        :class="{
          'bg-gray-200 dark:bg-gray-800 text-gray-800 dark:text-gray-200':
            pressedKeys[key],
        }"
      >
        <template v-slot:icon>
          <ViamIcon :path="keyIcons[key]">Check</ViamIcon>
        </template>
        <span class="text-3xl">{{ keyLetters[key] }}</span>
      </ViamButton>
    </div>
  </div>
</template>
<script lang="ts">
import { Component, Vue } from "vue-property-decorator";
import { throttle, debounce} from "lodash";
import { mdiRestore, mdiReload, mdiArrowUp, mdiArrowDown } from "@mdi/js";
import ViamIcon from "./ViamIcon.vue";
import ViamButton from "./Button.vue";

const PressedKeysMap: { [index: string]: string } = {
  "87": "forward",
  "83": "backward",
  "65": "left",
  "68": "right",
};

const inputDelay = 100;
const eventsDelay = 500;


@Component({
  components: {
    ViamIcon,
    ViamButton,
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

  keyLetters = {
    forward: "W",
    left: "A",
    backward: "S",
    right: "D",
  };

  keyIcons = {
    forward: mdiArrowUp,
    left: mdiRestore,
    backward: mdiArrowDown,
    right: mdiReload,
  };

  //for template section
  keysLayout = [["forward"], ["left", "backward", "right"]];



  sendKeysState = debounce(() => {
    this.handleKeysStateInstantly();
  }, inputDelay);

  handleKeysStateInstantly(): void {
    if (Object.values(this.pressedKeys).every((item) => item === false)) {
      return;
    }

    const { forward, left, right, backward } = this.pressedKeys;

    if (forward && right) this.emitEvent("arc-right");
    else if (forward && left) this.emitEvent("arc-left");
    else if (backward && right) this.emitEvent("back-arc-right");
    else if (backward && left) this.emitEvent("back-arc-left");
    else if (forward) this.emitEvent("forward");
    else if (backward) this.emitEvent("backward");
    else if (left) this.emitEvent("spin-counter-clockwise");
    else if (right) this.emitEvent("spin-clockwise");
  }
  emitEvent = throttle((eventName: string) => {
    this.emitEventInstantly(eventName);
  }, eventsDelay);

  emitEventInstantly(eventName: string): void {
    console.log(`event will be fired ${eventName}`)
    this.$emit(eventName)
  }
  setKeyPressed(key: string, value = true): void {
    this.pressedKeys[key] = value;
    this.sendKeysState();
  }

  onUseKeyboardNav(event: KeyboardEvent): void {
    const key = PressedKeysMap[event.keyCode];
    if (!key) return;
    this.setKeyPressed(key, event.type === "keydown");
    event.preventDefault();
  }

  beforeDestroy(): void {
    window.removeEventListener("keydown", this.onUseKeyboardNav);
    window.removeEventListener("keyup", this.onUseKeyboardNav);
  }
  mounted(): void {
    window.addEventListener("keydown", this.onUseKeyboardNav, false);
    window.addEventListener("keyup", this.onUseKeyboardNav, false);
  }
}
</script>

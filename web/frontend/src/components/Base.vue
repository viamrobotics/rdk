<template>
  <v-collapse :title="baseName">
    <v-breadcrumbs slot="header" :crumbs="crumbs.join(',')" />

    <v-button slot="header" variant="danger" icon="stop" label="STOP" @click="baseStop" />

      <div
        class="border border-t-0 border-black pt-2 pb-4 h-80"
        v-click-outside="removeKeyboardListeners"
        :style="{ height: height }"
      >
        <div>
          <Tabs>
            <Tab
              :selected="selectedItem === 'keyboard'"
              @select="selectedItem = 'keyboard'"
              @click="$emit('base-change-tab', selectedItem)"
              >Keyboard</Tab
            >
            <Tab
              :selected="selectedItem === 'discrete'"
              @select="selectedItem = 'discrete'"
              @click="resetDiscreteState()"
              >Discrete</Tab
            >
          </Tabs>
        </div>
        <div
          v-if="selectedItem === 'keyboard'"
          class="p-4"
          :style="{ height: height }"
        >
          <div>
            <div>
              <div class="grid grid-cols-2">
                <div class="flex pt-6">
                  <KeyboardInput
                    @keyboard-ctl="keyboardCtl"
                    ref="keyboardRef"
                  >
                  </KeyboardInput>
                </div>
                <div class="flex" v-if="camera">
                  <div class="pr-4">
                    <div class="w-64">
                      <p
                        class="mb-1 text-gray-800 font-label dark:text-gray-200"
                      >
                        Select Camera
                      </p>
                      <div class="relative">
                        <ViamSelect
                          :options="cameraOptions"
                          v-model="selectedValue"
                          @selected="$emit('show-base-camera')"
                        >
                        </ViamSelect>
                      </div>
                    </div>
                  </div>
                  <div
                    class="w-48 h-48 transition-all duration-300 ease-in-out"
                    v-if="selectedValue !== 'NoCamera'"
                    :id="streamId"
                  ></div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div
          v-if="selectedItem === 'discrete'"
          class="pr-4 pl-4 pt-4 flex"
          :style="{ height: height }"
        >
          <div class="flex-grow">
            <div>
              <div class="column">
                <p class="text-xs">Movement Mode</p>
                <RadioButtons
                  :options="['Straight', 'Spin']"
                  defaultOption="Straight"
                  :disabledOptions="[]"
                  v-on:selectOption="setMovementMode($event)"
                />
              </div>
            </div>
            <div
              :class="movementMode === 'Spin' ? 'inline-flex' : 'flex'"
              class="items-center pt-4"
            >
              <div class="column pr-2" v-if="movementMode === 'Straight'">
                <p class="text-xs">Movement Type</p>
                <RadioButtons
                  :options="['Continuous', 'Discrete']"
                  defaultOption="Continuous"
                  :disabledOptions="[]"
                  v-on:selectOption="setMovementType($event)"
                />
              </div>
              <div v-if="movementMode === 'Straight'" class="column pr-2">
                <p class="text-xs">Direction</p>
                <RadioButtons
                  :options="['Forwards', 'Backwards']"
                  defaultOption="Forwards"
                  :disabledOptions="[]"
                  v-on:selectOption="setDirection($event)"
                />
              </div>
              <NumberInput
                v-model="speed"
                class="mr-2"
                inputId="speed"
                label="Speed (mm/sec)"
                v-if="movementMode === 'Straight'"
              ></NumberInput>
              <NumberInput
                v-model="increment"
                class="mr-2"
                inputId="distance"
                :disabled="movementType === 'Continuous'"
                label="Distance (mm)"
                v-if="movementMode === 'Straight'"
              ></NumberInput>
              <NumberInput
                v-model="spinSpeed"
                class="mr-2"
                inputId="spinspeed"
                label="Speed (deg/sec)"
                v-if="movementMode === 'Spin'"
              ></NumberInput>
            </div>
            <div
              :class="movementMode === 'Spin' ? 'inline-flex' : 'flex'"
              class="pt-4"
              v-if="movementMode === 'Spin'"
            >
              <div class="column pr-2">
                <p class="text-xs">Movement Type</p>
                <RadioButtons
                  data-cy="movement-type-radio"
                  :options="['Clockwise', 'Counterclockwise']"
                  defaultOption="Clockwise"
                  :disabledOptions="[]"
                  v-on:selectOption="setSpinType($event)"
                />
              </div>
              <div class="column pl-4">
                <Range
                  id="angle"
                  :min="0"
                  :key="movementMode"
                  :max="360"
                  :step="90"
                  v-model="angle"
                  unit="°"
                  name="Angle"
                ></Range>
              </div>
            </div>
          </div>
          <div class="self-end">
            <ViamButton
              color="success"
              group
              variant="primary"
              @click="baseRun()"
            >
              <template v-slot:icon>
                <ViamIcon color="white" :path="mdiPlayCircleOutline"
                  >RUN</ViamIcon
                >
              </template>
              RUN
            </ViamButton>
          </div>
        </div>
      </div>
  </v-collapse>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import "vue-class-component/hooks";
import Collapse from "./Collapse.vue";
import NumberInput from "./NumberInput.vue";
import Breadcrumbs from "./Breadcrumbs.vue";
import ViamIcon from "./ViamIcon.vue";
import {
  mdiRestore,
  mdiPlayCircleOutline,
  mdiCloseOctagonOutline,
} from "@mdi/js";
import Tabs from "./Tabs.vue";
import Tab from "./Tab.vue";
import RadioButtons from "./RadioButtons.vue";
import ViamButton from "./Button.vue";

@Component({
  components: {
    Collapse,
    Breadcrumbs,
    ViamIcon,
    RadioButtons,
    Tabs,
    Tab,
    ViamButton,
    NumberInput,
  },
})
export default class Base extends Vue {
  @Prop({ default: null }) streamName!: string;
  @Prop({ default: null }) baseName!: string;
  @Prop({ default: null }) crumbs!: [string];
  @Prop({ default: true }) connectedCamera!: boolean;

  mdiRestore = mdiRestore;
  mdiPlayCircleOutline = mdiPlayCircleOutline;
  mdiCloseOctagonOutline = mdiCloseOctagonOutline;

  camera = this.connectedCamera;
  height = "auto";
  selectedValue = "NoCamera";
  isContinuous = true;
  streamId = "stream-preview-" + this.streamName;
  selectedItem = "keyboard";
  movementMode = "Straight";
  movementType = "Continuous";
  direction = "Forwards";
  spinType = "Clockwise";
  increment = 1000;

  speed = 200; // straight mm/s
  spinSpeed = 90; // spin deg/s
  angle = 0;

  cameraOptions = [
    { value: "NoCamera", label: "No Camera" },
    { value: "Camera1", label: "Camera1" },
  ];

  resetDiscreteState(): void {
    this.movementMode = "Straight";
    this.movementType = "Continuous";
    this.direction = "Forwards";
    this.spinType = "Clockwise";
  }

  setMovementMode(e: string): void {
    this.movementMode = e;
    this.movementType = "Continuous";
  }
  setMovementType(e: string): void {
    this.movementType = e;
  }
  setSpinType(e: string): void {
    this.spinType = e;
  }
  setDirection(e: string): void {
    this.direction = e;
  }
  baseRun(): void {
    if (this.movementMode == "Spin") {
      this.$emit("base-spin", {
        direction: this.spinType == "Clockwise" ? -1 : 1,
        speed: this.spinSpeed,
        angle: this.angle,
      });
    } else if (this.movementMode == "Straight") {
      this.$emit("base-straight", {
        movementType: this.movementType,
        direction: this.direction == "Forwards" ? 1 : -1,
        speed: this.speed,
        distance: this.increment,
      });
    } else {
      console.log("Unrecognized discrete movement mode: " + this.movementMode);
    }
  }
  baseStop(e: Event): void {
    e.preventDefault();
    e.stopPropagation();
    this.$emit("base-stop");
  }
  keyboardCtl(keysPressed: Record<string, unknown>): void {
    this.$emit("keyboard-ctl", {
      forward: keysPressed.forward,
      backward: keysPressed.backward,
      right: keysPressed.right,
      left: keysPressed.left,
    });
  }
  removeKeyboardListeners(): void {
    // eslint-disable-next-line
    const keyboardRef: any = this.$refs.keyboardRef;
    keyboardRef.removeKeyboardListeners();
  }
}
</script>

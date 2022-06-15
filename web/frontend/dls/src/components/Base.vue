<template>
  <div>
    <Collapse>
      <div class="flex float-left">
        <h2 class="p-4 text-xl">{{ baseName }}</h2>
        <Breadcrumbs :crumbs="crumbs" disabled="true"></Breadcrumbs>
      </div>
      <div class="p-2 float-right">
        <ViamButton color="danger" group variant="primary" @click="baseStop">
          <template v-slot:icon>
            <ViamIcon color="white" :path="mdiCloseOctagonOutline"
              >STOP</ViamIcon
            >
          </template>
          STOP
        </ViamButton>
      </div>
      <template v-slot:content>
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
                    :options="['Continous', 'Discrete']"
                    defaultOption="Continous"
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
                  class="mr-4 w-32"
                  inputId="speed"
                  label="Speed (mm/sec)"
                ></NumberInput>
                <NumberInput
                  v-model="increment"
                  class="mr-4 w-32"
                  inputId="distance"
                  :disabled="movementType === 'Continous'"
                  label="Distance (mm)"
                  v-if="movementMode === 'Straight'"
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
                    v-model="maxClusteringRadius"
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
      </template>
    </Collapse>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';
import 'vue-class-component/hooks';
import Collapse from './Collapse.vue';
import NumberInput from './NumberInput.vue';
import Breadcrumbs from './Breadcrumbs.vue';
import ViamIcon from './ViamIcon.vue';
import {
  mdiRestore,
  mdiPlayCircleOutline,
  mdiCloseOctagonOutline,
} from '@mdi/js';
import Tabs from './Tabs.vue';
import Tab from './Tab.vue';
import RadioButtons from './RadioButtons.vue';
import 'vue-popperjs/dist/vue-popper.css';
import ViamButton from './Button.vue';

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
  height = 'auto';
  selectedValue = 'NoCamera';
  isContinuous = true;
  streamId = 'stream-preview-' + this.streamName;
  selectedItem = 'keyboard';
  pressedKey = 0;
  movementMode = 'Straight';
  movementType = 'Continous';
  direction = 'Forwards';
  spinType = 'Clockwise';
  increment = 1000;
  maxClusteringRadius = 90;
  maxDistance = Math.pow(2, 32);

  speed = 200;
  angle = 0;
  cameraOptions = [
    { value: 'NoCamera', label: 'No Camera' },
    { value: 'Camera1', label: 'Camera1' },
  ];

  resetDiscreteState(): void {
    this.movementMode = 'Straight';
    this.movementType = 'Continous';
    this.direction = 'Forwards';
    this.spinType = 'Clockwise';
  }

  setMovementMode(e: string): void {
    console.log(e);
    this.movementMode = e;
  }
  setMovementType(e: string): void {
    console.log(e);
    this.movementType = e;
  }
  setSpinType(e: string): void {
    console.log(e);
    this.spinType = e;
  }
  setDirection(e: string): void {
    console.log(e);
    this.direction = e;
  }
  baseRun(): void {
    this.$emit(
      'base-run',
      this.movementMode,
      this.movementType,
      this.spinType,
      this.direction
    );
  }
  baseStop(e: Event): void {
    e.preventDefault();
    e.stopPropagation();
    this.$emit(
      'base-stop',
      this.movementMode,
      this.movementType,
      this.spinType,
      this.direction
    );
  }
  keyboardCtl(keysPressed: Record<string, unknown>): void {
    let toEmit = {
      baseName: this.baseName,
      forward: keysPressed.forward,
      backward: keysPressed.backward,
      right: keysPressed.right,
      left: keysPressed.left,
    };
    this.$emit('keyboard-ctl', toEmit);
  }
  removeKeyboardListeners(): void {
    // eslint-disable-next-line
    const keyboardRef: any = this.$refs.keyboardRef;
    keyboardRef.removeKeyboardListeners();
  }
}
</script>

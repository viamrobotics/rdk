<script setup lang="ts">

import { ref } from 'vue';
import KeyboardInput from './keyboard-input.vue';

interface Props {
  streamName: string
  baseName: string
  crumbs: string[]
  connectedCamera: boolean
}

interface Emits {
  (event: 'base-stop'): void
  (event: 'show-base-camera'): void
  (event: 'base-change-tab', selectedItem: string): void
  (event: 'base-spin', data: { direction: 1 | -1, speed: number, angle: number }): void
  (event: 'base-straight', data: { movementType: string, direction: 1 | -1, speed: number, distance: number }): void
  (event: 'keyboard-ctl', data: Record<string, boolean>): void
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const camera = ref(props.connectedCamera);
const selectedValue = ref('NoCamera');
const selectedItem = ref<'Keyboard' | 'Discrete'>('Keyboard');
const movementMode = ref('Straight');
const movementType = ref('Continuous');
const direction = ref('Forwards');
const spinType = ref('Clockwise');
const increment = ref(1000);
const speed = ref(200); // straight mm/s
const spinSpeed = ref(90); // spin deg/s
const angle = ref(0);

const cameraOptions = [
  { value: 'NoCamera', label: 'No Camera' },
  { value: 'Camera1', label: 'Camera1' },
];

const handleTabSelect = (tab: 'Keyboard' | 'Discrete') => {
  selectedItem.value = tab;

  if (tab === 'Keyboard') {
    emit('base-change-tab', selectedItem.value.toLowerCase());
  } else {
    resetDiscreteState();
  }
};

const resetDiscreteState = () => {
  movementMode.value = 'Straight';
  movementType.value = 'Continuous';
  direction.value = 'Forwards';
  spinType.value = 'Clockwise';
};

const setMovementMode = (mode: string) => {
  movementMode.value = mode;
  movementType.value = 'Continuous';
};

const setMovementType = (type: string) => {
  movementType.value = type;
};

const setSpinType = (type: string) => {
  spinType.value = type;
};

const setDirection = (dir: string) => {
  direction.value = dir;
};

const baseRun = () => {
  if (movementMode.value === 'Spin') {
    emit('base-spin', {
      direction: spinType.value === 'Clockwise' ? -1 : 1,
      speed: spinSpeed.value,
      angle: angle.value,
    });
  } else if (movementMode.value === 'Straight') {
    emit('base-straight', {
      movementType: movementType.value,
      direction: direction.value === 'Forwards' ? 1 : -1,
      speed: speed.value,
      distance: increment.value,
    });
  } else {
    console.log(`Unrecognized discrete movement mode: ${movementMode.value}`);
  }
};

const keyboardCtl = (keysPressed: Record<string, boolean>) => {
  emit('keyboard-ctl', keysPressed);
};

const handleCameraOptionsInput = (event: CustomEvent) => {
  selectedValue.value = event.detail.value;
  emit('show-base-camera');
};

</script>

<template>
  <v-collapse :title="baseName">
    <v-breadcrumbs
      slot="title"
      :crumbs="crumbs.join(',')"
    />

    <v-button
      slot="header"
      variant="danger"
      icon="stop-circle"
      label="STOP"
      @click="emit('base-stop')"
    />

    <div class="border border-t-0 border-black pt-2 pb-4">
      <v-tabs
        tabs="Keyboard, Discrete"
        :selected="selectedItem"
        @input="handleTabSelect($event.detail.value)"
      />

      <div
        v-if="selectedItem === 'Keyboard'"
        class="h-auto p-4"
      >
        <div class="grid grid-cols-2">
          <div class="flex pt-6">
            <KeyboardInput
              @keyboard-ctl="keyboardCtl"
            />
          </div>
          <div
            v-if="camera"
            class="flex"
          >
            <div class="w-64 pr-4">
              <v-select
                label="Select Camera"
                :options="cameraOptions.map(option => option.label).join(',')"
                :selected="selectedValue"
                @input="handleCameraOptionsInput"
              />
            </div>
            <div
              v-if="selectedValue !== 'NoCamera'"
              :id="`stream-preview-${props.streamName}`"
              class="h-48 w-48 transition-all duration-300 ease-in-out"
            />
          </div>
        </div>
      </div>
      <div
        v-if="selectedItem === 'Discrete'"
        class="flex h-auto px-4 pt-4"
      >
        <div class="grow">
          <v-radio
            label="Movement Mode"
            options="Straight, Spin"
            :selected="movementMode"
            @input="setMovementMode($event.detail.value)"
          />
          <div class="flex items-center gap-2 pt-4">
            <v-radio
              v-if="movementMode === 'Straight'"
              label="Movement Type"
              options="Continuous, Discrete"
              :selected="movementType"
              @input="setMovementType($event.detail.value)"
            />
            <v-radio
              v-if="movementMode === 'Straight'"
              label="Direction"
              options="Forwards, Backwards"
              :selected="direction"
              @input="setDirection($event.detail.value)"
            />
            <v-input
              v-if="movementMode === 'Straight'"
              type="number"
              :value="speed"
              label="Speed (mm/sec)"
              @input="speed = $event.detail.value"
            />
            <div
              v-if="movementMode === 'Straight'"
              :class="{ 'pointer-events-none opacity-50': movementType === 'Continuous' }"
            >
              <v-input
                type="number"
                :value="increment"
                :readonly="movementType === 'Continuous' ? 'true' : 'false'"
                label="Distance (mm)"
                @input="increment = $event.detail.value"
              />
            </div>
            <v-input
              v-if="movementMode === 'Spin'"
              type="number"
              :value="spinSpeed"
              label="Speed (deg/sec)"
              @input="spinSpeed = $event.detail.value"
            />
            <v-radio
              v-if="movementMode === 'Spin'"
              label="Movement Type"
              options="Clockwise, Counterclockwise"
              :selected="spinType"
              @input="setSpinType($event.detail.value)"
            />
            <div
              v-if="movementMode === 'Spin'"
              class="w-72 pl-6"
            >
              <v-slider
                :min="0"
                :max="360"
                :step="90"
                suffix="°"
                label="Angle"
                :value="angle"
                @input="angle = $event.detail.value"
              />
            </div>
          </div>
        </div>
        <div class="self-end">
          <v-button
            icon="play-circle-filled"
            variant="success"
            label="RUN"
            @click="baseRun()"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>

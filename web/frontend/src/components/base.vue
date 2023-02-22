<script setup lang="ts">

import { onMounted, onUnmounted } from 'vue';
import { onClickOutside } from '@vueuse/core';
import { BaseClient, Client, type ServiceError, commonApi } from '@viamrobotics/sdk';
import { filterResources } from '../lib/resource';
import { displayError } from '../lib/error';
import KeyboardInput, { type Keys } from './keyboard-input.vue';
import Camera from './camera/camera.vue';
import { rcLogConditionally } from '../lib/log';
import { selectedMap } from '../lib/camera-state';

interface Props {
  name: string;
  resources: commonApi.ResourceName.AsObject[];
  client: Client;
}

const enum Keymap {
  LEFT = 'a',
  RIGHT = 'd',
  FORWARD = 'w',
  BACKWARD = 's'
}

const props = defineProps<Props>();

type Tabs = 'Keyboard' | 'Discrete'
type MovementTypes = 'Continuous' | 'Discrete'
type MovementModes = 'Straight' | 'Spin'
type SpinTypes = 'Clockwise' | 'Counterclockwise'
type Directions = 'Forwards' | 'Backwards'
type View = 'Stacked' | 'Grid'

const baseClient = new BaseClient(props.client, props.name, { requestLogger: rcLogConditionally });
const root = $ref<HTMLElement>();

const refreshFrequency = $ref('Every Second');
const triggerRefresh = $ref(false);

const openCameras = $ref<Record<string, boolean | undefined>>({});
let selectedView = $ref<View>('Stacked');
let selectedMode = $ref<Tabs>('Keyboard');
let movementMode = $ref<MovementModes>('Straight');
let movementType = $ref<MovementTypes>('Continuous');
let direction = $ref<Directions>('Forwards');
let spinType = $ref<SpinTypes>('Clockwise');
let disableRefresh = $ref(true);
let disableViews = $ref(true);

const increment = $ref(1000);
// straight mm/s
const speed = $ref(300);
// deg/s
const spinSpeed = $ref(90);
const angle = $ref(0);
const power = $ref(50);

const pressed = new Set<Keys>();
let stopped = true;

const keyboardStates = $ref({
  isActive: false,
});

const resources = $computed(() => filterResources(props.resources, 'rdk', 'component', 'camera'));

const resetDiscreteState = () => {
  movementMode = 'Straight';
  movementType = 'Continuous';
  direction = 'Forwards';
  spinType = 'Clockwise';
};

const setMovementMode = (mode: MovementModes) => {
  movementMode = mode;
};

const setMovementType = (type: MovementTypes) => {
  movementType = type;
};

const setSpinType = (type: SpinTypes) => {
  spinType = type;
};

const setDirection = (dir: Directions) => {
  direction = dir;
};

const stop = async () => {
  stopped = true;
  try {
    await baseClient.stop();
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const digestInput = async () => {
  let linearValue = 0;
  let angularValue = 0;

  for (const item of pressed) {
    switch (item) {
      case Keymap.FORWARD: {
        linearValue += Number(0.01 * power);
        break;
      }
      case Keymap.BACKWARD: {
        linearValue -= Number(0.01 * power);
        break;
      }
      case Keymap.LEFT: {
        angularValue += Number(0.01 * power);
        break;
      }
      case Keymap.RIGHT: {
        angularValue -= Number(0.01 * power);
        break;
      }
    }
  }

  const linear = new commonApi.Vector3();
  const angular = new commonApi.Vector3();
  linear.setY(linearValue);
  angular.setZ(angularValue);

  try {
    await baseClient.setPower(linear, angular);
  } catch (error) {
    displayError(error as ServiceError);

    if (pressed.size <= 0) {
      stop();
    }
  }
};

const handleKeyDown = (key: Keys) => {
  pressed.add(key);
  digestInput();
};

const handleKeyUp = (key: Keys) => {
  pressed.delete(key);

  if (pressed.size > 0) {
    stopped = false;
    digestInput();
  } else {
    stop();
  }
};

const handleBaseStraight = async (event: {
  distance: number
  speed: number
  direction: number
  movementType: MovementTypes
}) => {
  if (event.movementType === 'Continuous') {
    const linear = new commonApi.Vector3();
    const angular = new commonApi.Vector3();
    linear.setY(event.speed * event.direction);

    try {
      await baseClient.setVelocity(linear, angular);
    } catch (error) {
      displayError(error as ServiceError);
    }
  } else {
    try {
      await baseClient.moveStraight(event.distance, event.speed * event.direction);
    } catch (error) {
      displayError(error as ServiceError);
    }
  }
};

const baseRun = async () => {
  if (movementMode === 'Spin') {
    try {
      await baseClient.spin(angle * (spinType === 'Clockwise' ? -1 : 1), spinSpeed);
    } catch (error) {
      displayError(error as ServiceError);
    }
  } else if (movementMode === 'Straight') {
    handleBaseStraight({
      movementType,
      direction: direction === 'Forwards' ? 1 : -1,
      speed,
      distance: increment,
    });
  }
};

const handleViewSelect = (viewMode: View) => {
  selectedView = viewMode;

  let liveCameras = 0;
  for (const camera of resources) {
    if (openCameras[camera.name]) {
      liveCameras += 1;
    }
  }
  disableViews = liveCameras > 1;

};

const handleTabSelect = (controlMode: Tabs) => {
  selectedMode = controlMode;

  if (controlMode === 'Discrete') {
    resetDiscreteState();
  }
};

const handleVisibilityChange = () => {
  if (document.visibilityState === 'hidden') {
    pressed.clear();
    stop();
  }
};

const handleToggle = () => {
  if (keyboardStates.isActive) {
    return;
  }

  if (pressed.size > 0 || !stopped) {
    stop();
  }
};

const handleUpdateKeyboardState = (on:boolean) => {
  keyboardStates.isActive = on;
};

const handleSwitch = (cameraName: string) => {
  openCameras[cameraName] = !openCameras[cameraName];

  for (const camera of resources) {
    if (openCameras[camera.name]) {
      disableRefresh = false;
      return;
    }
  }
  disableRefresh = true;
};

onClickOutside($$(root), () => {
  keyboardStates.isActive = false;
});

onMounted(() => {
  window.addEventListener('visibilitychange', handleVisibilityChange);

  for (const camera of resources) {
    openCameras[camera.name] = false;
  }
});

onUnmounted(() => {
  stop();
  window.removeEventListener('visibilitychange', handleVisibilityChange);
});
</script>

<template>
  <div ref="root">
    <v-collapse
      :title="name"
      class="base"
    >
      <v-breadcrumbs
        slot="title"
        crumbs="base"
      />

      <v-button
        slot="header"
        variant="danger"
        icon="stop-circle"
        label="STOP"
        @click="stop"
      />

      <div class="flex flex-wrap sm:flex-nowrap gap-4 border border-t-0 border-black">
        <div class="flex flex-col gap-4 p-4 min-w-fit">
          <h2 class="font-bold">
            Motor Controls
          </h2>
          <v-radio
            label="Control Mode"
            options="Keyboard, Discrete"
            :selected="selectedMode"
            @input="handleTabSelect($event.detail.value)"
          />

          <div v-if="selectedMode === 'Keyboard'">
            <KeyboardInput
              :is-active="keyboardStates.isActive"
              @keydown="handleKeyDown"
              @keyup="handleKeyUp"
              @toggle="handleToggle"
              @update-keyboard-state="isOn => { handleUpdateKeyboardState(isOn) }"
            />
            <v-slider
              id="power"
              class="pt-2 w-full max-w-xs"
              :min="0"
              :max="100"
              :step="1"
              suffix="%"
              label="Power %"
              :value="power"
              @input="power = $event.detail.value"
            />
          </div>

          <div
            v-if="selectedMode === 'Discrete'"
            class="flex flex-col gap-4"
          >
            <v-radio
              label="Movement Mode"
              options="Straight, Spin"
              :selected="movementMode"
              @input="setMovementMode($event.detail.value)"
            />
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
            <v-button
              icon="play-circle-filled"
              variant="success"
              label="RUN"
              @click="baseRun()"
            />
          </div>

          <hr class="my-4 border-t border-gray-400">

          <h2 class="font-bold">
            Live Feeds
          </h2>

          <v-radio
            label="View"
            options="Stacked, Grid"
            :selected="selectedView"
            :disable="disableViews ? 'true' : 'false'"
            @input="handleViewSelect($event.detail.value)"
          />

          <div
            v-if="resources"
            class="flex flex-col gap-2"
          >
            <template
              v-for="camera in resources"
              :key="camera.name"
            >
              <v-switch
                :label="camera.name"
                :aria-label="`Refresh frequency for ${camera.name}`"
                :value="openCameras[camera.name] ? 'on' : 'off'"
                @input="handleSwitch(camera.name)"
              />
            </template>

            <div class="flex items-end gap-2 mt-2">
              <v-select
                v-model="refreshFrequency"
                label="Refresh frequency"
                aria-label="Refresh frequency"
                :options="Object.keys(selectedMap).join(',')"
                :disabled="disableRefresh ? 'true' : 'false'"
              />

              <v-button
                :class="refreshFrequency === 'Live' ? 'invisible' : ''"
                icon="refresh"
                label="Refresh"
                :disabled="disableRefresh ? 'true' : 'false'"
                @click="triggerRefresh = !triggerRefresh"
              />
            </div>
          </div>
        </div>
        <div
          data-parent="base"
          class="justify-start gap-4 sm:border-l border-black p-4"
          :class="selectedView === 'Stacked' ? 'flex flex-col' : 'grid grid-cols-2 gap-4'"
        >
          <!-- ******* CAMERAS *******  -->
          <template
            v-for="camera in resources"
            :key="`base ${camera.name}`"
          >
            <Camera
              v-show="openCameras[camera.name]"
              :camera-name="camera.name"
              parent-name="base"
              :client="client"
              :resources="resources"
              :show-refresh="true"
              :show-export-screenshot="false"
              :refresh-rate="refreshFrequency"
              :trigger-refresh="triggerRefresh"
            />
          </template>
        </div>
      </div>
    </v-collapse>
  </div>
</template>

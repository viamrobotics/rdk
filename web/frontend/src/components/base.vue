<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { onMounted, onUnmounted } from 'vue';
import { onClickOutside } from '@vueuse/core';
import { Client, type ServiceError, baseApi, commonApi, StreamClient } from '@viamrobotics/sdk';
import { filterResources } from '../lib/resource';
import { displayError } from '../lib/error';
import KeyboardInput, { type Keys } from './keyboard-input.vue';
import { rcLogConditionally } from '../lib/log';
import { cameraStreamStates, baseStreamStates } from '../lib/camera-state';

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

const root = $ref<HTMLElement>();

let selectedMode = $ref<Tabs>('Keyboard');
let movementMode = $ref<MovementModes>('Straight');
let movementType = $ref<MovementTypes>('Continuous');
let direction = $ref<Directions>('Forwards');
let spinType = $ref<SpinTypes>('Clockwise');
const increment = $ref(1000);
// straight mm/s
const speed = $ref(300);
// deg/s
const spinSpeed = $ref(90);
const angle = $ref(0);

let selectCameras = $ref('');

const power = $ref(50);

const pressed = new Set<Keys>();
let stopped = true;

const keyboardStates = $ref({
  isActive: false,
});

const resources =filterResources(props.resources, 'rdk', 'component', 'camera')
const initStreamState = () => {
  for (const value of resources) {
    baseStreamStates.set(value.name, false);
  }
};

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

const stop = () => {
  stopped = true;
  const req = new baseApi.StopRequest();
  req.setName(props.name);
  rcLogConditionally(req);
  props.client.baseService.stop(req, new grpc.Metadata(), displayError);
};

const digestInput = () => {
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

  const req = new baseApi.SetPowerRequest();
  req.setName(props.name);
  req.setLinear(linear);
  req.setAngular(angular);

  rcLogConditionally(req);
  props.client.baseService.setPower(req, new grpc.Metadata(), (error) => {
    displayError(error);

    if (pressed.size <= 0) {
      stop();
    }
  });
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

const handleBaseStraight = (name: string, event: {
  distance: number
  speed: number
  direction: number
  movementType: MovementTypes
}) => {
  if (event.movementType === 'Continuous') {
    const linear = new commonApi.Vector3();
    linear.setY(event.speed * event.direction);

    const req = new baseApi.SetVelocityRequest();
    req.setName(name);
    req.setLinear(linear);
    req.setAngular(new commonApi.Vector3());

    rcLogConditionally(req);
    props.client.baseService.setVelocity(req, new grpc.Metadata(), displayError);
  } else {
    const req = new baseApi.MoveStraightRequest();
    req.setName(name);
    req.setMmPerSec(event.speed * event.direction);
    req.setDistanceMm(event.distance);

    rcLogConditionally(req);
    props.client.baseService.moveStraight(req, new grpc.Metadata(), displayError);
  }
};

const baseRun = () => {
  if (movementMode === 'Spin') {

    const req = new baseApi.SpinRequest();
    req.setName(props.name);
    req.setAngleDeg(angle * (spinType === 'Clockwise' ? -1 : 1));
    req.setDegsPerSec(spinSpeed);

    rcLogConditionally(req);
    props.client.baseService.spin(req, new grpc.Metadata(), displayError);

  } else if (movementMode === 'Straight') {

    handleBaseStraight(props.name, {
      movementType,
      direction: direction === 'Forwards' ? 1 : -1,
      speed,
      distance: increment,
    });

  }
};

const viewPreviewCamera = (value: string) => {
  const streams = new StreamClient(props.client);

  for (const [key] of baseStreamStates) {
    if (value === key) {
      if (!baseStreamStates.get(key)) {
        try {
          // Only add stream if other components have not already
          if (!cameraStreamStates.get(key) && !baseStreamStates.get(key)) {
            streams.add(key);
          }
        } catch (error) {
          displayError(error as ServiceError);
        }

        baseStreamStates.set(key, true);
      } else {
        try {
          // Only remove stream if other components are not using the stream
          if (!cameraStreamStates.get(key)) {
            streams.remove(key);
          }
        } catch (error) {
          displayError(error as ServiceError);
        }
        
        baseStreamStates.set(key, false);
      }
    }
  }
};

const handleTabSelect = (controlMode: Tabs) => {
  selectedMode = controlMode;

  /*
   * deselect options from select cameras select
   * TODO: handle better with xstate and reactivate on return
   */
  selectCameras = '';
  viewPreviewCamera(selectCameras);

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

// const tempDisableKeyboard = (disableKeyboard: boolean) => {
//   keyboardStates.tempDisable = disableKeyboard;
// };

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

onClickOutside($$(root), () => {
  keyboardStates.isActive = false;
});

onMounted(() => {
  initStreamState();
  window.addEventListener('visibilitychange', handleVisibilityChange);
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

      <div class="flex gap-4 border border-t-0 border-black">
        <div class="flex flex-col gap-4 p-4 min-w-[18em]">
          <h2 class="font-bold">Motor Controls</h2>
          <v-radio
            class="mb-4"
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

          <hr class="my-4 border-t border-gray-400"/>

          <h2 class="font-bold">Live Feeds</h2>

          <div v-if="resources">
            <template
              v-for="value of resources"
            >
              <v-switch
                :label="value.name"
                @input="viewPreviewCamera(value.name)"
              />
            </template>
          </div>
        </div>
        <div class="flex flex-col gap-4 border-l border-black p-4">
          <template
            v-for="basecamera in resources"
            :key="basecamera.name"
          >
            <div
              v-if="basecamera"
              :data-stream-preview="basecamera.name"
              :class="{ 'hidden': !baseStreamStates.get(basecamera.name) }"
            />
          </template>
        </div>
      </div>
    </v-collapse>
  </div>
</template>

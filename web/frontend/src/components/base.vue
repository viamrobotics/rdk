<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { onMounted } from 'vue';
import baseApi from '../gen/proto/api/component/base/v1/base_pb.esm';
import commonApi from '../gen/proto/api/common/v1/common_pb.esm';
import { toast } from '../lib/toast';
import { filterResources, type Resource } from '../lib/resource';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';
import KeyboardInput from './keyboard-input.vue';
import { addStream, removeStream } from '../lib/stream';
import type { ServiceError } from '../gen/proto/stream/v1/stream_pb_service.esm';
import { type BaseServiceClient, type StreamServiceClient, createBaseService, createStreamService } from '../api';

interface Props {
  name: string;
  resources: Resource[];
}

const props = defineProps<Props>();

type Tabs = 'Keyboard' | 'Discrete'
type MovementTypes = 'Continuous' | 'Discrete'
type MovementModes = 'Straight' | 'Spin'
type SpinTypes = 'Clockwise' | 'Counterclockwise'
type Directions = 'Forwards' | 'Backwards'

interface Emits {
  (event: 'showcamera', value: string): void
}

const emit = defineEmits<Emits>();

let baseService: BaseServiceClient;
let streamService: StreamServiceClient;

let selectedItem = $ref<Tabs>('Keyboard');
let movementMode = $ref<MovementModes>('Straight');
let movementType = $ref<MovementTypes>('Continuous');
let direction = $ref<Directions>('Forwards');
let spinType = $ref<SpinTypes>('Clockwise');
const increment = $ref(1000);
// straight mm/s
const speed = $ref(200);
// deg/s
const spinSpeed = $ref(90);
const angle = $ref(0);

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

const handleBaseActionStop = (name: string) => {
  const req = new baseApi.StopRequest();
  req.setName(name);
  baseService.stop(req, new grpc.Metadata(), displayError);
};

/*
 * Base keyboard control calculations.
 * Input: State of keys. e.g. {straight : true, backward : false, right : false, left: false}
 * Output: linearY and angularZ throttle
 */
const computeKeyboardBaseControls = (keysPressed: Record<string, boolean>) => {
  let linear = 0;
  let angular = 0;

  if (keysPressed.forward) {
    linear = 1;
  } else if (keysPressed.backward) {
    linear = -1;
  }

  if (keysPressed.right) {
    angular = -1;
  } else if (keysPressed.left) {
    angular = 1;
  }

  return { linear, angular };
};

const baseKeyboardCtl = (name: string, controls: Record<string, boolean>) => {
  if (Object.values(controls).every((item) => item === false)) {
    handleBaseActionStop(name);
    return;
  }

  const inputs = computeKeyboardBaseControls(controls);
  const linear = new commonApi.Vector3();
  const angular = new commonApi.Vector3();
  linear.setY(inputs.linear);
  angular.setZ(inputs.angular);

  const req = new baseApi.SetPowerRequest();
  req.setName(name);
  req.setLinear(linear);
  req.setAngular(angular);

  rcLogConditionally(req);
  baseService.setPower(req, new grpc.Metadata(), displayError);
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
    baseService.setVelocity(req, new grpc.Metadata(), displayError);
    return;
  }

  const req = new baseApi.MoveStraightRequest();
  req.setName(name);
  req.setMmPerSec(event.speed * event.direction);
  req.setDistanceMm(event.distance);

  rcLogConditionally(req);
  baseService.moveStraight(req, new grpc.Metadata(), displayError);
};

const baseRun = () => {
  if (movementMode === 'Spin') {
    const req = new baseApi.SpinRequest();
    req.setName(props.name);
    req.setAngleDeg(angle * (spinType === 'Clockwise' ? -1 : 1));
    req.setDegsPerSec(spinSpeed);

    rcLogConditionally(req);
    baseService.spin(req, new grpc.Metadata(), displayError);
    return;
  }

  if (movementMode === 'Straight') {
    handleBaseStraight(props.name, {
      movementType,
      direction: direction === 'Forwards' ? 1 : -1,
      speed,
      distance: increment,
    });
    return;
  }

  toast.error(`Unrecognized discrete movement mode: ${movementMode}`);
};

const viewPreviewCamera = async (name: string, isOn: boolean) => {
  if (isOn) {
    try {
      await addStream(name, streamService);
    } catch (error) {
      displayError(error as ServiceError);
    }
    return;
  }

  try {
    await removeStream(name, streamService);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const handleTabSelect = (tab: Tabs) => {
  selectedItem = tab;

  if (tab === 'Keyboard') {
    viewPreviewCamera(props.name, true);
  } else {
    viewPreviewCamera(props.name, false);
    resetDiscreteState();
  }
};

const handleSelectCamera = (event: string) => {
  emit('showcamera', event);
};

onMounted(() => {
  baseService = createBaseService();
  streamService = createStreamService();
});

</script>

<template>
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
      @click="handleBaseActionStop(name)"
    />

    <div class="border border-t-0 border-black pt-2">
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
          <div class="mt-2">
            <KeyboardInput
              :name="name"
              @keyboard-ctl="baseKeyboardCtl(name, $event)"
            />
          </div>
          <div v-if="filterResources(resources, 'rdk', 'component', 'camera')">
            <v-select
              class="mb-4"
              variant="multiple"
              placeholder="Select Cameras"
              :options="
                filterResources(resources, 'rdk', 'component', 'camera')
                  .map(({ name }) => name)
                  .join(',')
              "
              @input="handleSelectCamera($event.detail.value)"
            />
            <template
              v-for="basecamera in filterResources(
                resources,
                'rdk',
                'component',
                'camera'
              )"
              :key="basecamera.name"
            >
              <div
                v-if="basecamera"
                :data-stream-preview="basecamera.name"
                class="mb-4 border border-white"
              />
            </template>
          </div>
        </div>
      </div>
      <div
        v-if="selectedItem === 'Discrete'"
        class="flex h-auto p-4"
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

<script setup lang="ts">
import { grpc } from '@improbable-eng/grpc-web';
import { ref, onMounted } from 'vue';
import { computeKeyboardBaseControls, BaseControlHelper } from '../rc/control_helpers';
import baseApi from '../gen/proto/api/component/base/v1/base_pb.esm';
import commonApi from '../gen/proto/api/common/v1/common_pb.esm';
import { toast } from '../lib/toast';
import { filterResources, type Resource } from '../lib/resource';
import { displayError } from '../lib/error';
import KeyboardInput from './keyboard-input.vue';
import { addStream, removeStream } from '../lib/stream';
import type { ServiceError } from '../gen/proto/stream/v1/stream_pb_service.esm';

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
  (event: 'base-camera-state', value: Map<string, boolean>): void
}

const emit = defineEmits<Emits>();

const selectedItem = ref<Tabs>('Keyboard');
const movementMode = ref<MovementModes>('Straight');
const movementType = ref<MovementTypes>('Continuous');
const direction = ref<Directions>('Forwards');
const spinType = ref<SpinTypes>('Clockwise');
const increment = ref(1000);
// straight mm/s
const speed = ref(200);
// deg/s
const spinSpeed = ref(90);
const angle = ref(0);

const videoStreamStates = new Map<string, boolean>();
const selectCameras = ref('');

const initStreamState = () => {
  for (const value of filterResources(props.resources, 'rdk', 'component', 'camera')) {
    videoStreamStates.set(value.name, false);
  }
};

onMounted(() => {
  initStreamState();
});

const resetDiscreteState = () => {
  movementMode.value = 'Straight';
  movementType.value = 'Continuous';
  direction.value = 'Forwards';
  spinType.value = 'Clockwise';
};

const setMovementMode = (mode: MovementModes) => {
  movementMode.value = mode;
};

const setMovementType = (type: MovementTypes) => {
  movementType.value = type;
};

const setSpinType = (type: SpinTypes) => {
  spinType.value = type;
};

const setDirection = (dir: Directions) => {
  direction.value = dir;
};

const handleBaseActionStop = (name: string) => {
  const req = new baseApi.StopRequest();
  req.setName(name);
  window.baseService.stop(req, new grpc.Metadata(), displayError);
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
  BaseControlHelper.setPower(name, linear, angular, displayError);
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

    BaseControlHelper.setVelocity(
      name,
      linear,
      new commonApi.Vector3(),
      displayError
    );
  } else {
    BaseControlHelper.moveStraight(
      name,
      event.distance,
      event.speed * event.direction,
      displayError
    );
  }
};

const baseRun = () => {
  if (movementMode.value === 'Spin') {
    BaseControlHelper.spin(
      props.name,
      angle.value * (spinType.value === 'Clockwise' ? -1 : 1),
      spinSpeed.value,
      displayError
    );
  } else if (movementMode.value === 'Straight') {
    handleBaseStraight(props.name, {
      movementType: movementType.value,
      direction: direction.value === 'Forwards' ? 1 : -1,
      speed: speed.value,
      distance: increment.value,
    });
  } else {
    toast.error(`Unrecognized discrete movement mode: ${movementMode.value}`);
  }
};

const viewPreviewCamera = (name: string) => {
  for (const [key, value] of videoStreamStates) {
    const streamContainers = document.querySelector(`[data-stream="${key}"]`);

    // Only turn on if state is off
    if (name.includes(key) && value === false) {
      try {
        // Only add stream if other components have not already
        if (streamContainers?.classList.contains('hidden')) {
          addStream(key);
        }
        videoStreamStates.set(key, true);
        emit('base-camera-state', videoStreamStates);
      } catch (error) {
        displayError(error as ServiceError);
      }
    // Only turn off if state is on
    } else if (!name.includes(key) && value === true) {
      try {
        // Only remove stream if other components are not using the stream
        if (streamContainers?.classList.contains('hidden')) {
          removeStream(key);
        }
        videoStreamStates.set(key, false);
        emit('base-camera-state', videoStreamStates);
      } catch (error) {
        displayError(error as ServiceError);
      }
    }
  }
};

const handleTabSelect = (tab: Tabs) => {
  selectedItem.value = tab;

  /*
   * deselect options from select cameras select
   * TODO: handle better with xstate and reactivate on return
   */
  selectCameras.value = '';
  viewPreviewCamera(selectCameras.value);

  if (tab === 'Discrete') {
    resetDiscreteState();
  }
};

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
          <div>
            <KeyboardInput
              :name="name"
              @keyboard-ctl="baseKeyboardCtl(name, $event)"
            />
          </div>
          <div v-if="filterResources(resources, 'rdk', 'component', 'camera')">
            <v-select
              v-model="selectCameras"
              class="mb-4"
              variant="multiple"
              placeholder="Select Cameras"
              aria-label="Select Cameras"
              :options="
                filterResources(resources, 'rdk', 'component', 'camera')
                  .map(({ name }) => name)
                  .join(',')
              "
              @input="viewPreviewCamera($event.detail.value)"
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
                :class="{ 'hidden': !videoStreamStates.get(basecamera.name) }"
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

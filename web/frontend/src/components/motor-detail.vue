
<script setup lang="ts">

import { ref } from 'vue';
import type { Status } from '../gen/proto/api/component/motor/v1/motor_pb.esm';
import InfoButton from './info-button.vue';

interface Props {
  motorName: string
  crumbs: string[]
  motorStatus: Status.AsObject
}

interface Emits {
  (event: 'motor-run', data: {
    direction: number,
    position: number,
    rpm: number,
    power: number,
    type: string,
    speed: number,
    revolutions: number,
  }): void
  (event: 'motor-stop'): void
}

defineProps<Props>();
const emit = defineEmits<Emits>();

const movementType = ref('Go');
const direction = ref<-1 | 1>(1);
const position = ref(0);
const rpm = ref(0);
const power = ref(25);
const type = ref('go');
const speed = ref(0);
const revolutions = ref(0);

const setMovementType = (value: string) => {
  movementType.value = value;
  switch (value) {
  case 'Go':
    type.value = 'go';
    break;
  case 'Go For':
    type.value = 'goFor';
    break;
  case 'Go To':
    type.value = 'goTo';
    break;
  }
};

const setDirection = (value: string) => {
  switch (value) {
  case 'Forwards':
    direction.value = 1;
    break;
  case 'Backwards':
    direction.value = -1;
    break;
  default:
    direction.value = 1;
  }
};

const motorRun = () => {
  emit('motor-run', {
    direction: direction.value,
    position: position.value,
    rpm: rpm.value,
    power: power.value,
    type: type.value,
    speed: speed.value,
    revolutions: revolutions.value,
  });
};

const motorStop = () => {
  emit('motor-stop');
};

</script>

<template>
  <v-collapse :title="motorName">
    <v-breadcrumbs
      slot="title"
      :crumbs="crumbs.join(',')"
    />
    <div
      slot="header"
      class="flex items-center justify-between gap-2"
    >
      <v-badge
        v-if="motorStatus.positionReporting"
        :label="`Position ${motorStatus.position}`"
      />
      <v-badge
        v-if="motorStatus.isPowered"
        variant="green"
        label="Running"
      />
      <v-badge
        v-else-if="!motorStatus.isPowered"
        variant="gray"
        label="Idle"
      />
      <v-button
        variant="danger"
        icon="stop-circle"
        label="STOP"
        @click="motorStop"
      />
    </div>

    <div>
      <div
        class="grid grid-cols-1 border border-t-0 border-black p-4"
      >
        <div class="grid">
          <div class="column">
            <v-radio
              label="Set Power"
              :options="
                motorStatus.positionReporting
                  ? 'Go, Go For, Go To'
                  : 'Go'
              "
              :selected="movementType"
              @input="setMovementType($event.detail.value)"
            />
          </div>
          <div
            v-if="movementType === 'Go To'"
            class="flex pt-4"
          >
            <div class="place-self-end pr-2">
              <span class="text-2xl">{{ movementType }}</span>
              <InfoButton
                class="pb-2"
                :info-rows="['Relative to Home']"
              />
            </div>
            <v-input
              type="number"
              label="Position in Revolutions"
              :value="position"
              class="w-48 pr-2"
              @input="position = $event.detail.value"
            />
            <v-input
              type="number"
              class="w-32 pr-2"
              label="RPM"
              :value="rpm"
              @input="rpm = $event.detail.value"
            />
          </div>
          <div
            v-if="movementType === 'Go For'"
            class="flex pt-4"
          >
            <div class="place-self-end pr-2">
              <span class="text-2xl">{{ movementType }}</span>
              <InfoButton
                class="pb-2"
                :info-rows="['Relative to where the robot is currently']"
              />
            </div>
            <v-input
              type="number"
              class="w-32 pr-2"
              label="# in Revolutions"
              :value="revolutions"
              @input="revolutions = $event.detail.value"
            />
            <div class="column pr-4">
              <v-radio
                label="Direction of Rotation"
                options="Forwards, Backwards"
                :selected="direction === 1 ? 'Forwards' : 'Backwards'"
                @input="setDirection($event.detail.value)"
              />
            </div>
            <v-input
              type="number"
              label="RPM"
              class="w-32 pr-2"
              :value="rpm"
              @input="rpm = $event.detail.value"
            />
          </div>
          <div
            v-if="movementType === 'Go'"
            class="flex items-start pt-4"
          >
            <div class="place-self-end pr-2">
              <span class="text-2xl">{{ movementType }}</span>
              <InfoButton
                class="pb-2"
                :info-rows="['Continuously moves']"
              />
            </div>
            <div class="column pr-4">
              <v-radio
                label="Direction of Rotation"
                options="Forwards, Backwards"
                :selected="direction === 1 ? 'Forwards' : 'Backwards'"
                @input="setDirection($event.detail.value)"
              />
            </div>
            <div class="min-w-[400px]">
              <v-slider
                id="power"
                class="pt-2"
                :min="0"
                :max="100"
                :step="1"
                suffix="%"
                name="Power %"
                :value="power"
                @input="power = $event.detail.value"
              />
            </div>
          </div>
        </div>
        <div class="flex flex-row-reverse">
          <v-button
            icon="play-circle-filled"
            variant="success"
            label="RUN"
            @click="motorRun()"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>

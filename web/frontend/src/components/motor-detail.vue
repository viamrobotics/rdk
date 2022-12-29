<script setup lang="ts">
import { onMounted } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Client, motorApi, MotorClient, ServiceError } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import InfoButton from './info-button.vue';

interface Props {
  name: string;
  status: motorApi.Status.AsObject;
  client: Client;
}

const props = defineProps<Props>();

type MovementTypes = 'go' | 'goFor' | 'goTo';

const position = $ref(0);
const rpm = $ref(0);
const power = $ref(50);
const revolutions = $ref(0);

let movementType = $ref('Go');
let direction = $ref<-1 | 1>(1);
let type = $ref<MovementTypes>('go');
let properties = $ref<motorApi.GetPropertiesResponse.AsObject | undefined>();

const setMovementType = (value: string) => {
  movementType = value;
  switch (value) {
    case 'Go': {
      type = 'go';
      break;
    }
    case 'Go For': {
      type = 'goFor';
      break;
    }
    case 'Go To': {
      type = 'goTo';
      break;
    }
  }
};

const setDirection = (value: string) => {
  switch (value) {
    case 'Forwards': {
      direction = 1;
      break;
    }
    case 'Backwards': {
      direction = -1;
      break;
    }
    default: {
      direction = 1;
    }
  }
};

const setPower = async () => {
  const mc = new MotorClient(props.client, props.name);
  try {
    await mc.setPower(power);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const goFor = async () => {
  const mc = new MotorClient(props.client, props.name);
  try {
    await mc.goFor(rpm, revolutions);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const goTo = async () => {
  const mc = new MotorClient(props.client, props.name);
  try {
    await mc.goTo(rpm, position);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const motorRun = () => {
  switch (type) {
    case 'go': {
      return setPower();
    }
    case 'goFor': {
      return goFor();
    }
    case 'goTo': {
      return goTo();
    }
  }
  return null;
};

const motorStop = async () => {
  const mc = new MotorClient(props.client, props.name);
  try {
    await mc.motorStop();
  } catch (error) {
    displayError(error as ServiceError);
  }
};

onMounted(() => {
  const req = new motorApi.GetPropertiesRequest();
  req.setName(props.name);

  props.client.motorService.getProperties(req, new grpc.Metadata(), (err, resp) => {
    if (err) {
      return displayError(err);
    }

    properties = resp!.toObject();
  });
});
</script>

<template>
  <v-collapse
    :title="name"
    class="motor"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="motor"
    />
    <div
      slot="header"
      class="flex items-center justify-between gap-2"
    >
      <v-badge
        v-if="properties?.positionReporting"
        :label="`Position ${status.position}`"
      />
      <v-badge
        v-if="status.isPowered"
        variant="green"
        label="Running"
      />
      <v-badge
        v-else-if="!status.isPowered"
        variant="gray"
        label="Idle"
      />
      <v-button
        variant="danger"
        icon="stop-circle"
        label="STOP"
        @click.stop="motorStop"
      />
    </div>

    <div>
      <div class="border border-t-0 border-black p-4">
        <v-radio
          label="Set Power"
          :options="properties?.positionReporting ? 'Go, Go For, Go To' : 'Go'"
          :selected="movementType"
          class="mb-4"
          @input="setMovementType($event.detail.value)"
        />
        <div class="flex flex-wrap gap-4 mb-4">
          <div
            v-if="movementType === 'Go To'"
            class="flex flex-wrap gap-2 pt-4"
          >
            <div class="flex items-center gap-1 place-self-end pr-2">
              <span class="text-lg">{{ movementType }}</span>
              <InfoButton :info-rows="['Relative to Home']" />
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
            class="flex flex-wrap gap-4"
          >
            <div class="flex items-center gap-1 place-self-end pr-2">
              <span class="text-lg">{{ movementType }}</span>
              <InfoButton :info-rows="['Relative to where the robot is currently']" />
            </div>
            <v-input
              type="number"
              class="w-32"
              label="# in Revolutions"
              :value="revolutions"
              @input="revolutions = $event.detail.value"
            />
            <v-radio
              label="Direction of Rotation"
              options="Forwards, Backwards"
              :selected="direction === 1 ? 'Forwards' : 'Backwards'"
              @input="setDirection($event.detail.value)"
            />
            <v-input
              type="number"
              label="RPM"
              class="w-32"
              :value="rpm"
              @input="rpm = $event.detail.value"
            />
          </div>
          <div
            v-if="movementType === 'Go'"
            class="flex flex-wrap gap-4"
          >
            <div class="flex flex-wrap gap-2">
              <span class="text-lg">{{ movementType }}</span>
              <InfoButton :info-rows="['Continuously moves']" />
            </div>
            <v-radio
              label="Direction of Rotation"
              options="Forwards, Backwards"
              :selected="direction === 1 ? 'Forwards' : 'Backwards'"
              @input="setDirection($event.detail.value)"
            />
            <div class="w-80">
              <v-slider
                id="power"
                class="ml-2 pt-2 max-w-xs"
                :min="0"
                :max="100"
                :step="1"
                suffix="%"
                label="Power %"
                :value="power"
                @input="power = $event.detail.value"
              />
            </div>
          </div>
        </div>
        <div class="flex flex-row-reverse flex-wrap">
          <v-button
            icon="play-circle-filled"
            variant="success"
            label="RUN"
            @click="motorRun"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>

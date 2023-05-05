<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue';
import { $ref } from 'vue/macros';
import { grpc } from '@improbable-eng/grpc-web';
import { Client, encoderApi, ResponseStream, robotApi, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';
import { scheduleAsyncPoll } from '../lib/schedule';

const props = defineProps<{
  name: string
  client: Client
  statusStream: ResponseStream<robotApi.StreamStatusResponse> | null
}>();

let properties = $ref<encoderApi.GetPropertiesResponse.AsObject>();
let positionTicks = $ref(0);
let positionDegrees = $ref(0);

let cancelPoll: (() => void) | undefined;

const getProperties = () => new Promise<encoderApi.GetPropertiesResponse.AsObject>((resolve, reject) => {
  const request = new encoderApi.GetPropertiesRequest();
  request.setName(props.name);

  rcLogConditionally(request);
  props.client.encoderService.getProperties(request, new grpc.Metadata(), (error, response) => {
    if (error) {
      return reject((error as ServiceError).message);
    }

    resolve(response!.toObject());
  });
});

const getPosition = () => new Promise<number>((resolve, reject) => {
  const request = new encoderApi.GetPositionRequest();
  request.setName(props.name);
  rcLogConditionally(request);
  props.client.encoderService.getPosition(request, new grpc.Metadata(), (error, resp) => {
    if (error) {
      return reject(new Error((error as ServiceError).message));
    }

    resolve(resp!.toObject().value);
  });
});

const getPositionDegrees = () => new Promise<number>((resolve, reject) => {
  const request = new encoderApi.GetPositionRequest();
  request.setPositionType(2);
  rcLogConditionally(request);

  props.client.encoderService.getPosition(request, new grpc.Metadata(), (error, resp) => {
    if (error) {
      return reject(new Error((error as ServiceError).message));
    }

    resolve(resp!.toObject().value);
  });
});

const refresh = async () => {
  try {
    positionTicks = await getPosition();

    if (properties?.angleDegreesSupported) {
      positionDegrees = await getPositionDegrees();
    }
  } catch (error) {
    displayError(error as ServiceError);
  }

  cancelPoll = scheduleAsyncPoll(refresh, 500);
};

const reset = () => {
  const request = new encoderApi.ResetPositionRequest();
  request.setName(props.name);

  rcLogConditionally(request);

  props.client.encoderService.resetPosition(request, new grpc.Metadata(), (error) => {
    if (error) {
      return displayError(error as ServiceError);
    }
  });
};

onMounted(async () => {
  try {
    properties = await getProperties();
    refresh();
  } catch (error) {
    displayError(error as ServiceError);
  }

  props.statusStream?.on('end', () => cancelPoll?.());
});

onUnmounted(() => {
  cancelPoll?.();
});

</script>

<template>
  <v-collapse
    :title="name"
    class="encoder"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="encoder"
    />
    <div class="border-medium overflow-auto border border-t-0 p-4 text-left">
      <table class="bborder-medium table-auto border">
        <tr
          v-if="properties && (properties.ticksCountSupported ||
            (!properties.ticksCountSupported && !properties.angleDegreesSupported))"
        >
          <th class="border-medium border p-2">
            Count
          </th>
          <td class="border-medium border p-2">
            {{ positionTicks.toFixed(2) || 0 }}
          </td>
        </tr>
        <tr
          v-if="properties && (properties.angleDegreesSupported ||
            (!properties.ticksCountSupported && !properties.angleDegreesSupported))"
        >
          <th class="border-medium border p-2">
            Angle (degrees)
          </th>
          <td class="border-medium border p-2">
            {{ positionDegrees.toFixed(2) || 0 }}
          </td>
        </tr>
      </table>
      <div class="mt-2 flex gap-2">
        <v-button
          label="Reset"
          class="flex-auto"
          @click="reset"
        />
      </div>
    </div>
  </v-collapse>
</template>

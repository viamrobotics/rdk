<script setup lang="ts">
import { onMounted, onUnmounted, toRaw } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Client, encoderApi, EncoderClient, ServiceError } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';

const props = defineProps<{
  name: string
  client: Client
}>();

const encoderClient = new EncoderClient(props.client, props.name, {
  requestLogger: rcLogConditionally,
});

let properties = $ref<encoderApi.GetPropertiesResponse.AsObject | undefined>();
let positionTicks = $ref<encoderApi.GetPositionResponse.AsObject | undefined>();
let positionDegrees = $ref<encoderApi.GetPositionResponse.AsObject | undefined>();

let refreshId = -1;

const refresh = () => {
  const req = new encoderApi.GetPositionRequest();
  req.setName(props.name);

  rcLogConditionally(req);
  props.client.encoderService.getPosition(
    req,
    new grpc.Metadata(),
    (err: ServiceError, resp: encoderApi.GetPositionResponse) => {
      if (err) {
        return displayError(err);
      }

      positionTicks = resp!.toObject();
    }
  );

  if (properties!.angleDegreesSupported) {
    req.setPositionType(2);
    rcLogConditionally(req);
    props.client.encoderService.getPosition(
      req,
      new grpc.Metadata(),
      (err: ServiceError, resp: encoderApi.GetPositionResponse) => {
        if (err) {
          return displayError(err);
        }

        positionDegrees = resp!.toObject();
      }
    );
  }

  refreshId = window.setTimeout(refresh, 500);
};

const reset = () => {
  const req = new encoderApi.ResetPositionRequest();
  req.setName(props.name);

  rcLogConditionally(req);
  encoderClient.resetPosition(req, new grpc.Metadata(), displayError);
};

onMounted(async () => {
  try {
    properties = await encoderClient.getProperties();
    refreshId = window.setTimeout(refresh, 500);
  } catch (error) {
    displayError(error as ServiceError);
  }
});

onUnmounted(() => {
  clearTimeout(refreshId);
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
    <div class="overflow-auto border border-t-0 border-black p-4 text-left">
      <table class="border border-t-0 border-black p-4">
        <tr
          v-if="properties.ticksCountSupported ||
            (!properties.ticksCountSupported && !properties.angleDegreesSupported)"
        >
          <th class="border border-black p-2">
            Count
          </th>
          <td class="border border-black p-2">
            {{ toRaw(positionTicks).value || 0 }}
          </td>
        </tr>
        <tr
          v-if="properties.angleDegreesSupported ||
            (!properties.ticksCountSupported && !properties.angleDegreesSupported)"
        >
          <th class="border border-black p-2">
            Angle (degrees)
          </th>
          <td class="border border-black p-2">
            {{ toRaw(positionDegrees).value || 0 }}
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

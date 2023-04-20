<script setup lang="ts">
import { onMounted, onUnmounted, toRaw } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Client, encoderApi, ServiceError } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';

const props = defineProps<{
  name: string
  client: Client
}>();

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
    (error: ServiceError, resp: encoderApi.GetPositionResponse) => {
      if (error) {
        return displayError(error);
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
      (error: ServiceError, resp: encoderApi.GetPositionResponse) => {
        if (error) {
          return displayError(error);
        }

        positionDegrees = resp!.toObject();
      }
    );
  }

  refreshId = window.setTimeout(refresh, 500);
};

const reset = async () => {
  const req = new encoderApi.ResetPositionRequest();
  req.setName(props.name);

  rcLogConditionally(req);
  await props.client.encoderService.resetPosition(
    req,
    new grpc.Metadata(),
    (error: ServiceError) => {
      if (error) {
        return displayError(error);
      }
    }
  );
};

onMounted(async () => {
  try {
    const req = new encoderApi.GetPropertiesRequest();
    req.setName(props.name);

    rcLogConditionally(req);
    await props.client.encoderService.getProperties(
      req,
      new grpc.Metadata(),
      (error: ServiceError, resp: encoderApi.GetPropertiesResponse) => {
        if (error) {
          if (error.message === 'Response closed without headers') {
            refreshId = window.setTimeout(refresh, 500);
            return;
          }
          return displayError(error);
        }
        properties = resp!.toObject();
      }
    );
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
    <div class="border-border-1 overflow-auto border border-t-0 p-4 text-left">
      <table class="bborder-border-1 table-auto border">
        <tr
          v-if="properties.ticksCountSupported ||
            (!properties.ticksCountSupported && !properties.angleDegreesSupported)"
        >
          <th class="border-border-1 border p-2">
            Count
          </th>
          <td class="border-border-1 border p-2">
            {{ (toRaw(positionTicks).value).toFixed(2) || 0 }}
          </td>
        </tr>
        <tr
          v-if="properties.angleDegreesSupported ||
            (!properties.ticksCountSupported && !properties.angleDegreesSupported)"
        >
          <th class="border-border-1 border p-2">
            Angle (degrees)
          </th>
          <td class="border-border-1 border p-2">
            {{ (toRaw(positionDegrees).value).toFixed(2) || 0 }}
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

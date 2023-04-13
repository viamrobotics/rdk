<script setup lang="ts">
import { onMounted, computed } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Client, encoderApi, EncoderClient, ServiceError } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';

const props = defineProps<{
  name: string
  status: {
    position: Record<string, { value: number }>
  }
  client: Client
}>();

// const encoderClient = new EncoderClient(props.client, props.name, {
//   requestLogger: rcLogConditionally,
// });

let properties = $ref<encoderApi.GetPropertiesResponse.AsObject | undefined>();

const position = computed(() => {
  const req = new encoderApi.GetPositionRequest();
  req.setName(props.name);
  const resp = props.client.encoderService.getPosition(req, new grpc.Metadata(), displayError);
  console.log('response:', resp.AsObject);
  return resp.AsObject[0];
});

const reset = () => {
  const req = new encoderApi.ResetPositionRequest();
  req.setName(props.name);

  rcLogConditionally(req);
  props.client.encoderService.resetPosition(req, new grpc.Metadata(), displayError);
};

onMounted(async () => {
  try {
    properties = await props.client.encoderService.getProperties();
  } catch (error) {
    displayError(error as ServiceError);
  }
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
            {{ status.position.value || 0 }}
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
            {{ position || 0 }}
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

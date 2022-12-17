<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { Client, gripperApi } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';

interface Props {
  name: string
  client: Client
}

const props = defineProps<Props>();

const stop = () => {
  const request = new gripperApi.StopRequest();
  request.setName(props.name);

  rcLogConditionally(request);
  props.client.gripperService.stop(request, new grpc.Metadata(), displayError);
};

const open = () => {
  const request = new gripperApi.OpenRequest();
  request.setName(props.name);

  rcLogConditionally(request);
  props.client.gripperService.open(request, new grpc.Metadata(), displayError);
};

const grab = () => {
  const request = new gripperApi.GrabRequest();
  request.setName(props.name);

  rcLogConditionally(request);
  props.client.gripperService.grab(request, new grpc.Metadata(), displayError);
};

</script>

<template>
  <v-collapse
    :title="name"
    class="gripper"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="gripper"
    />
    <div
      slot="header"
      class="flex items-center justify-between gap-2"
    >
      <v-button
        variant="danger"
        icon="stop-circle"
        label="STOP"
        @click.stop="stop"
      />
    </div>
    <div class="flex gap-2 border border-t-0 border-black p-4">
      <v-button
        label="Open"
        @click="open"
      />
      <v-button
        label="Grab"
        @click="grab"
      />
    </div>
  </v-collapse>
</template>

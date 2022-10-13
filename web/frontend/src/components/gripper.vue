<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { displayError } from '../lib/error';
import { gripperApi, createGripperService, GripperServiceClient } from '../api';
import { onMounted } from 'vue';

interface Props {
  name: string
}

const props = defineProps<Props>();

let gripperService: GripperServiceClient;

const stop = () => {
  const request = new gripperApi.StopRequest();
  request.setName(props.name);
  gripperService.stop(request, new grpc.Metadata(), displayError);
};

const open = () => {
  const request = new gripperApi.OpenRequest();
  request.setName(props.name);
  gripperService.open(request, new grpc.Metadata(), displayError);
};

const grab = () => {
  const request = new gripperApi.GrabRequest();
  request.setName(props.name);
  gripperService.grab(request, new grpc.Metadata(), displayError);
};

onMounted(() => {
  gripperService = createGripperService();
});

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

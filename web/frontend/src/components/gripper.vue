<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import gripperApi from '../gen/proto/api/component/gripper/v1/gripper_pb.esm';
import { displayError } from '../lib/error';

interface Props {
  name: string
}

const props = defineProps<Props>();

const stop = () => {
  const request = new gripperApi.StopRequest();
  request.setName(props.name);
  window.gripperService.stop(request, new grpc.Metadata(), displayError);
};

const action = (action: string) => {
  let req;
  switch (action) {
    case 'open':
      req = new gripperApi.OpenRequest();
      req.setName(props.name);
      window.gripperService.open(req, new grpc.Metadata(), displayError);
      break;
    case 'grab':
      req = new gripperApi.GrabRequest();
      req.setName(props.name);
      window.gripperService.grab(req, new grpc.Metadata(), displayError);
      break;
  }
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
        @click="action('open')"
      />
      <v-button
        label="Grab"
        @click="action('grab')"
      />
    </div>
  </v-collapse>
</template>

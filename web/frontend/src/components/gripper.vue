<script setup lang="ts">

import { Client, GripperClient, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';

const props = defineProps<{
  name: string
  client: Client
}>();

const gripperClient = new GripperClient(props.client, props.name, { requestLogger: rcLogConditionally });

const stop = async () => {
  try {
    await gripperClient.stop();
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const open = async () => {
  try {
    await gripperClient.open();
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const grab = async () => {
  try {
    await gripperClient.grab();
  } catch (error) {
    displayError(error as ServiceError);
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
    <div class="border-border-1 flex gap-2 border border-t-0 p-4">
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

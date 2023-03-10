<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { Client, servoApi } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';

const props = defineProps<{
  name: string
  status: servoApi.Status.AsObject
  rawStatus: servoApi.Status.AsObject
  client: Client
}>();

const stop = () => {
  const req = new servoApi.StopRequest();
  req.setName(props.name);

  rcLogConditionally(req);
  props.client.servoService.stop(req, new grpc.Metadata(), displayError);
};

const move = (amount: number) => {
  const servo = props.rawStatus;

  // @ts-expect-error @TODO Proto is incorrectly typing this. It expects servo.positionDeg
  const oldAngle = servo.position_deg ?? 0;

  const angle = oldAngle + amount;

  const req = new servoApi.MoveRequest();
  req.setName(props.name);
  req.setAngleDeg(angle);

  rcLogConditionally(req);
  props.client.servoService.move(req, new grpc.Metadata(), (error) => {
    if (error) {
      return displayError(error);
    }
  });
};

</script>

<template>
  <div>
    <v-collapse
      :title="name"
      class="servo"
    >
      <v-breadcrumbs
        slot="title"
        crumbs="servo"
      />
      <v-button
        slot="header"
        label="STOP"
        icon="stop-circle"
        variant="danger"
        @click="stop"
      />
      <div class="border border-t-0 border-black p-4">
        <h3 class="mb-1 text-sm">
          Angle: {{ status.positionDeg }}
        </h3>

        <div class="flex gap-1.5">
          <v-button
            label="-10"
            @click="move(-10)"
          />
          <v-button
            label="-1"
            @click="move(-1)"
          />
          <v-button
            label="1"
            @click="move(1)"
          />
          <v-button
            label="10"
            @click="move(10)"
          />
        </div>
      </div>
    </v-collapse>
  </div>
</template>

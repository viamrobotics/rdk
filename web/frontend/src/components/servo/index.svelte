<script lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { Client, type ServiceError, servoApi } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { rcLogConditionally } from '@/lib/log';
import Collapse from '../collapse.svelte';

export let name: string;
export let status: servoApi.Status.AsObject;
export let rawStatus: servoApi.Status.AsObject;
export let client: Client;

const stop = () => {
  const req = new servoApi.StopRequest();
  req.setName(name);

  rcLogConditionally(req);
  client.servoService.stop(req, new grpc.Metadata(), displayError);
};

const move = (amount: number) => {
  const servo = rawStatus;

  // @ts-expect-error @TODO Proto is incorrectly typing this. It expects servo.positionDeg
  const oldAngle = servo.position_deg ?? 0;

  const angle = oldAngle + amount;

  const req = new servoApi.MoveRequest();
  req.setName(name);
  req.setAngleDeg(angle);

  rcLogConditionally(req);
  client.servoService.move(req, new grpc.Metadata(), (error: ServiceError | null) => {
    if (error) {
      return displayError(error);
    }
  });
};

</script>

<Collapse title={name}>
  <v-breadcrumbs slot="title" crumbs="servo" />
  <v-button
    slot="header"
    label="STOP"
    icon="stop-circle"
    variant="danger"
    on:click={stop}
  />
  <div class="border border-t-0 border-medium p-4">
    <h3 class="mb-1 text-sm">Angle: {status.positionDeg}</h3>

    <div class="flex gap-1.5">
      <v-button label="-10" on:click={() => move(-10)} />
      <v-button label="-1" on:click={() => move(-1)} />
      <v-button label="1" on:click={() => move(1)} />
      <v-button label="10" on:click={() => move(10)} />
    </div>
  </div>
</Collapse>

<script lang="ts">

import { type ServiceError, servoApi } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { rcLogConditionally } from '@/lib/log';
import Collapse from '@/lib/components/collapse.svelte';
import { move } from '@/api/servo';
import { useRobotClient } from '@/hooks/robot-client';

export let name: string;
export let status: undefined | { position_deg: number };

const { robotClient } = useRobotClient();

const stop = () => {
  const req = new servoApi.StopRequest();
  req.setName(name);

  rcLogConditionally(req);
  $robotClient.servoService.stop(req, displayError);
};

const handleMove = async (amount: number) => {
  const oldAngle = status?.position_deg ?? 0;
  const angle = oldAngle + amount;

  try {
    await move($robotClient, name, angle);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

</script>

<Collapse title={name}>
  <v-breadcrumbs slot="title" crumbs="servo" />
  <v-button
    slot="header"
    label="Stop"
    icon="stop-circle-outline"
    variant="danger"
    on:click={stop}
  />
  <div class="border border-t-0 border-medium p-4">
    <h3 class="mb-1 text-sm">Angle: {status?.position_deg ?? 0}</h3>

    <div class="flex gap-1.5">
      <v-button label="-10" on:click={async () => handleMove(-10)} />
      <v-button label="-1" on:click={async () => handleMove(-1)} />
      <v-button label="1" on:click={async () => handleMove(1)} />
      <v-button label="10" on:click={async () => handleMove(10)} />
    </div>
  </div>
</Collapse>

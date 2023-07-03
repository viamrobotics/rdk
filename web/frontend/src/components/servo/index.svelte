<script lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { type ServiceError, servoApi } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { rcLogConditionally } from '@/lib/log';
import Collapse from '@/lib/components/collapse.svelte';
import { move } from '@/api/servo';
import { useClient } from '@/hooks/client';

export let name: string;
export let status: undefined | { position_deg: number };

const { client } = useClient();

const stop = () => {
  const req = new servoApi.StopRequest();
  req.setName(name);

  rcLogConditionally(req);
  $client.servoService.stop(req, new grpc.Metadata(), displayError);
};

const handleMove = async (amount: number) => {
  const oldAngle = status?.position_deg ?? 0;
  const angle = oldAngle + amount;

  try {
    await move($client, name, angle);
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
    icon="stop-circle"
    variant="danger"
    on:click={stop}
  />
  <div class="border border-t-0 border-medium p-4">
    <h3 class="mb-1 text-sm">Angle: {status?.position_deg ?? 0}</h3>

    <div class="flex gap-1.5">
      <v-button label="-10" on:click={() => handleMove(-10)} />
      <v-button label="-1" on:click={() => handleMove(-1)} />
      <v-button label="1" on:click={() => handleMove(1)} />
      <v-button label="10" on:click={() => handleMove(10)} />
    </div>
  </div>
</Collapse>

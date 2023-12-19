<script lang="ts">
import { type ServiceError, servoApi } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { rcLogConditionally } from '@/lib/log';
import { move } from '@/api/servo';
import { useRobotClient } from '@/hooks/robot-client';
import { useStop } from '@/lib/components/collapse';

export let name: string;
export let status: { position_deg: number } | undefined = undefined;

const { robotClient } = useRobotClient();

const handleMove = async (amount: number) => {
  const oldAngle = status?.position_deg ?? 0;
  const angle = oldAngle + amount;

  try {
    await move($robotClient, name, angle);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const { onStop } = useStop();

onStop(() => {
  const req = new servoApi.StopRequest();
  req.setName(name);

  rcLogConditionally(req);
  $robotClient.servoService.stop(req, displayError);
})

</script>

<div class="border border-t-0 border-medium p-4">
  <h3 class="mb-1 text-sm">Angle: {status?.position_deg ?? 0}</h3>

  <div class="flex gap-1.5">
    <v-button label="-10" on:click={async () => handleMove(-10)} />
    <v-button label="-1" on:click={async () => handleMove(-1)} />
    <v-button label="1" on:click={async () => handleMove(1)} />
    <v-button label="10" on:click={async () => handleMove(10)} />
  </div>
</div>

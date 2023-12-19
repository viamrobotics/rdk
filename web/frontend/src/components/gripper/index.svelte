<script lang="ts">

import { gripperApi } from '@viamrobotics/sdk';
import { displayError } from '../../lib/error';
import { rcLogConditionally } from '../../lib/log';
import { useRobotClient } from '@/hooks/robot-client';
import { useStop } from '@/lib/components/collapse.svelte';

export let name: string;

const { robotClient } = useRobotClient();

const open = () => {
  const request = new gripperApi.OpenRequest();
  request.setName(name);

  rcLogConditionally(request);
  $robotClient.gripperService.open(request, displayError);
};

const grab = () => {
  const request = new gripperApi.GrabRequest();
  request.setName(name);

  rcLogConditionally(request);
  $robotClient.gripperService.grab(request, displayError);
};

const { onStop } = useStop();

onStop(() => {
  const request = new gripperApi.StopRequest();
  request.setName(name);

  rcLogConditionally(request);
  $robotClient.gripperService.stop(request, displayError);
});

</script>

<div class="flex gap-2 border border-t-0 border-medium p-4">
  <v-button
    label="Open"
    on:click={open}
  />
  <v-button
    label="Grab"
    on:click={grab}
  />
</div>

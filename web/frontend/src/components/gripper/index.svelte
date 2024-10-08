<script lang="ts">
import { useRobotClient } from '@/hooks/robot-client';
import Collapse from '@/lib/components/collapse.svelte';
import { gripperApi } from '@viamrobotics/sdk';
import { displayError } from '../../lib/error';
import { rcLogConditionally } from '../../lib/log';

export let name: string;

const { robotClient } = useRobotClient();

const stop = () => {
  const request = new gripperApi.StopRequest({ name });

  rcLogConditionally(request);
  $robotClient.gripperService.stop(request).catch(displayError);
};

const open = () => {
  const request = new gripperApi.OpenRequest({ name });

  rcLogConditionally(request);
  $robotClient.gripperService.open(request).catch(displayError);
};

const grab = () => {
  const request = new gripperApi.GrabRequest({ name });

  rcLogConditionally(request);
  $robotClient.gripperService.grab(request).catch(displayError);
};
</script>

<Collapse title={name}>
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
      icon="stop-circle-outline"
      label="Stop"
      on:click|stopPropagation={stop}
    />
  </div>
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
</Collapse>

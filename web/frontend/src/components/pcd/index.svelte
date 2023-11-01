<script lang="ts">

import { CameraClient, type ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import PCD from './pcd-view.svelte';
import { useRobotClient } from '@/hooks/robot-client';

export let cameraName: string;

const { robotClient } = useRobotClient();

let pcdExpanded = false;
let pointcloud: Uint8Array | undefined;

const renderPCD = async () => {
  try {
    pointcloud = await new CameraClient($robotClient, cameraName).getPointCloud();
  } catch (error) {
    notify.danger(`Error getting point cloud: ${(error as ServiceError).message}`);
  }
};

const togglePCDExpand = () => {
  pcdExpanded = !pcdExpanded;
  if (pcdExpanded) {
    renderPCD();
  }
};
</script>

<div class="pt-4">
  <div class="flex gap-2 align-top">
    <v-switch
      tooltip='When turned on, point cloud will be recalculated'
      label='View point cloud data'
      value={pcdExpanded ? 'on' : 'off'}
      on:input={togglePCDExpand}
    />
  </div>

  {#if pcdExpanded}
    <PCD {pointcloud} />
  {/if}
</div>

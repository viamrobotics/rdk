<script lang="ts">

import { CameraClient, Client } from '@viamrobotics/sdk';
import { toast } from '@/lib/toast';
import PCD from './pcd-view.svelte';

export let cameraName: string;
export let client: Client;

let pcdExpanded = false;
let pointcloud: Uint8Array | undefined;

const renderPCD = async () => {
  try {
    pointcloud = await new CameraClient(client, cameraName).getPointCloud();
  } catch (error) {
    toast.error(`Error getting point cloud: ${error}`);
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

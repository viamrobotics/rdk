<script setup lang="ts">
import {
  CameraClient,
  Client,
  commonApi,
} from '@viamrobotics/sdk';
import { toast } from '../../lib/toast';
import PCD from './pcd-view.vue';

interface Props {
  cameraName: string;
  showRefresh: boolean;
  resources: commonApi.ResourceName.AsObject[];
  client: Client;
}

const props = defineProps<Props>();

let pcdExpanded = $ref(false);
let pointcloud = $ref<Uint8Array | undefined>();

const renderPCD = async () => {
  try {
    pointcloud = await new CameraClient(props.client, props.cameraName).getPointCloud();
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

<template>
  <div class="pt-4">
    <div class="flex gap-2 align-top">
      <v-switch
        :label="pcdExpanded ? 'Hide Point Cloud Data' : 'View Point Cloud Data'"
        :value="pcdExpanded ? 'on' : 'off'"
        @input="togglePCDExpand"
      />

      <v-tooltip
        text="When turned on, point cloud will be recalculated."
        location="top"
      >
        <v-icon
          name="info-outline"
        />
      </v-tooltip>
    </div>

    <PCD
      v-if="pcdExpanded"
      :resources="resources"
      :pointcloud="pointcloud"
      :camera-name="cameraName"
      :client="client"
    />
  </div>
</template>

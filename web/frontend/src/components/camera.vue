<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { ref } from 'vue';
import type { Resource } from '../lib/resource';
import { displayError } from '../lib/error';
import cameraApi from '../gen/proto/api/component/camera/v1/camera_pb.esm';
import { toast } from '../lib/toast';
import InfoButton from './info-button.vue';
import PCD from './pcd.vue';

interface Props {
  cameraName: string
  resources: Resource[]
}

interface Emits {
  (event: 'download-raw-data'): void
  (event: 'toggle-camera', camera: boolean): void
  (event: 'selected-camera-view', value: string): void
  (event: 'refresh-camera', value: string): void
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

let pcdExpanded = $ref(false);
let pointcloud = $ref<Uint8Array | undefined>();

const camera = ref(false);
const selectedValue = ref('live');

const toggleExpand = () => {
  camera.value = !camera.value;
  emit('toggle-camera', camera.value);
};

const renderPCD = () => {
  const request = new cameraApi.GetPointCloudRequest();
  request.setName(props.cameraName);
  request.setMimeType('pointcloud/pcd');
  window.cameraService.getPointCloud(request, new grpc.Metadata(), (error, response) => {
    if (error) {
      toast.error(`Error getting point cloud: ${error}`);
      return;
    }
    pointcloud = response!.getPointCloud_asU8();
  });
};

const togglePCDExpand = () => {
  pcdExpanded = !pcdExpanded;
  if (pcdExpanded) {
    renderPCD();
  }
};

const selectCameraView = () => {
  emit('selected-camera-view', selectedValue.value);
};

const refreshCamera = () => {
  emit('refresh-camera', selectedValue.value);
};

const exportScreenshot = (cameraName: string) => {
  const req = new cameraApi.RenderFrameRequest();
  req.setName(cameraName);
  req.setMimeType('image/jpeg');

  window.cameraService.renderFrame(req, new grpc.Metadata(), (err, resp) => {
    if (err) {
      return displayError(err);
    }

    const blob = new Blob([resp!.getData_asU8()], { type: 'image/jpeg' });
    window.open(URL.createObjectURL(blob), '_blank');
  });
};

</script>

<template>
  <v-collapse
    :title="cameraName"
    class="camera"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="camera"
    />
    <div class="h-auto border-x border-b border-black p-2">
      <div class="container mx-auto">
        <div class="pt-4">
          <div class="flex items-center gap-2">
            <v-switch
              id="camera"
              :label="camera ? 'Hide Camera' : 'View Camera'"
              :aria-label="camera ? 'Hide Camera' : 'View Camera'"
              :value="camera ? 'on' : 'off'"
              @input="toggleExpand"
            />
          </div>

          <div class="float-right pb-4">
            <div class="flex">
              <div
                v-if="camera"
                class="w-64"
              >
                <p class="font-label mb-1 text-gray-800">
                  Refresh frequency
                </p>
                <div class="relative">
                  <select
                    v-model="selectedValue"
                    class="
                      m-0 w-full appearance-none border border-solid border-black bg-white bg-clip-padding
                      px-3 py-1.5 text-xs font-normal text-gray-700 focus:outline-none
                    "
                    aria-label="Default select example"
                    @change="selectCameraView"
                  >
                    <option value="manual">
                      Manual Refresh
                    </option>
                    <option value="30">
                      Every 30 seconds
                    </option>
                    <option value="10">
                      Every 10 seconds
                    </option>
                    <option value="1">
                      Every second
                    </option>
                    <option value="live">
                      Live
                    </option>
                  </select>
                  <div
                    class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2"
                  >
                    <svg
                      class="h-4 w-4 stroke-2 text-gray-700"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-linejoin="round"
                      stroke-linecap="round"
                      fill="none"
                    >
                      <path d="M18 16L12 22L6 16" />
                    </svg>
                  </div>
                </div>
              </div>
              <div class="px-2 pt-7">
                <v-button
                  v-if="camera"
                  icon="refresh"
                  label="Refresh"
                  @click="refreshCamera"
                />
              </div>
              <div class="pr-2 pt-7">
                <v-button
                  v-if="camera"
                  icon="camera"
                  label="Export Screenshot"
                  @click="exportScreenshot(cameraName)"
                />
              </div>
            </div>
          </div>
          <div
            :aria-label="`${cameraName} camera stream`"
            :data-stream="cameraName"
            :class="{ 'hidden': !camera }"
            class="clear-both h-fit transition-all duration-300 ease-in-out"
          />
        </div>
        <div class="pt-4">
          <div class="flex items-center gap-2 align-top">
            <v-switch
              :label="pcdExpanded ? 'Hide Point Cloud Data' : 'View Point Cloud Data'"
              :value="pcdExpanded ? 'on' : 'off'"
              @input="togglePCDExpand"
            />
            <InfoButton :info-rows="['When turned on, point cloud will be recalculated']" />
          </div>

          <PCD
            v-if="pcdExpanded"
            :resources="resources"
            :pointcloud="pointcloud"
            :camera-name="cameraName"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>

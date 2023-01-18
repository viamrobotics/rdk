<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { displayError } from '../lib/error';
import {
  StreamClient,
  CameraClient,
  Camera,
  Client,
  commonApi,
  ServiceError,
} from '@viamrobotics/sdk';
import { toast } from '../lib/toast';
import InfoButton from './info-button.vue';
import PCD from './pcd.vue';
import { cameraStreamStates, baseStreamStates } from '../lib/camera-state';

interface Props {
  cameraName: string;
  resources: commonApi.ResourceName.AsObject[];
  client: Client;
}

interface Emits {
  (event: 'clear-interval'): void;
  (event: 'selected-camera-view', value: number): void;
}

const selectedMap = {
  Live: -1,
  'Manual Refresh': 0,
  'Every 30 Seconds': 30,
  'Every 10 Seconds': 10,
  'Every Second': 1,
} as const;

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

let pcdExpanded = $ref(false);
let pointcloud = $ref<Uint8Array | undefined>();
let camera = $ref(false);

const selectedValue = ref('Live');

const initStreamState = () => {
  cameraStreamStates.set(props.cameraName, false);
};

const clearStreamContainer = (camName: string) => {
  const streamContainers = document.querySelectorAll(
    `[data-stream="${camName}"]`
  );
  for (const streamContainer of streamContainers) {
    streamContainer.querySelector('video')?.remove();
    streamContainer.querySelector('img')?.remove();
  }
};

const viewCamera = async (isOn: boolean) => {
  const streams = new StreamClient(props.client);
  if (isOn) {
    try {
      // only add stream if not already active
      if (
        !baseStreamStates.get(props.cameraName) &&
        !cameraStreamStates.get(props.cameraName)
      ) {
        await streams.add(props.cameraName);
      }
    } catch (error) {
      displayError(error as ServiceError);
    }
    cameraStreamStates.set(props.cameraName, true);
  } else {
    try {
      // only remove camera stream if active and base stream is not active
      if (
        !baseStreamStates.get(props.cameraName) &&
        cameraStreamStates.get(props.cameraName)
      ) {
        await streams.remove(props.cameraName);
      }
    } catch (error) {
      displayError(error as ServiceError);
    }
    cameraStreamStates.set(props.cameraName, false);
  }
};

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

const selectCameraView = () => {
  emit('clear-interval');
  clearStreamContainer(props.cameraName);

  if (selectedValue.value !== 'Live') {
    const selectedInterval: number = selectedMap[selectedValue.value as keyof typeof selectedMap];
    viewCamera(false);
    emit('selected-camera-view', selectedInterval);
    return;
  }

  viewCamera(true);
};

const toggleExpand = () => {
  camera = !camera;

  emit('clear-interval');
  clearStreamContainer(props.cameraName);
  if (camera) {
    selectCameraView();
  } else {
    viewCamera(false);
  }
};

const refreshCamera = () => {
  emit('selected-camera-view', selectedMap[selectedValue.value as keyof typeof selectedMap]);
  emit('clear-interval');
};

const exportScreenshot = async (cameraName: string) => {
  let blob;
  try {
    blob = await new CameraClient(props.client, cameraName).renderFrame(
      Camera.MimeType.JPEG
    );
  } catch (error) {
    displayError(error as ServiceError);
    return;
  }

  window.open(URL.createObjectURL(blob), '_blank');
};

onMounted(() => {
  initStreamState();
});
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
          <div class="flex mb-4 items-center gap-2">
            <v-switch
              id="camera"
              :label="camera ? 'Hide Camera' : 'View Camera'"
              :aria-label="
                camera
                  ? `Hide Camera: ${$props.cameraName}`
                  : `View Camera: ${$props.cameraName}`
              "
              :value="camera ? 'on' : 'off'"
              @input="toggleExpand"
            />
          </div>

          <div class="pb-4">
            <div class="flex flex-wrap">
              <div
                v-if="camera"
                class="flex flex-wrap justify-items-end gap-2"
              >
                <div class="">
                  <div class="relative">
                    <v-select
                      v-model="selectedValue"
                      label="Refresh frequency"
                      aria-label="Default select example"
                      :options="Object.keys(selectedMap).join(',')"
                      @input="selectCameraView"
                    />
                  </div>
                </div>
                <div class="self-end">
                  <v-button
                    v-if="camera && selectedValue === 'Manual Refresh'"
                    icon="refresh"
                    label="Refresh"
                    @click="refreshCamera"
                  />
                </div>
                <div class="self-end">
                  <v-button
                    v-if="camera"
                    icon="camera"
                    label="Export Screenshot"
                    @click="exportScreenshot(cameraName)"
                  />
                </div>
              </div>
            </div>
          </div>
          <div
            :aria-label="`${cameraName} stream`"
            :data-stream="cameraName"
            :class="{ hidden: !camera }"
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
            <InfoButton
              :info-rows="['When turned on, point cloud will be recalculated']"
            />
          </div>

          <PCD
            v-if="pcdExpanded"
            :resources="resources"
            :pointcloud="pointcloud"
            :camera-name="cameraName"
            :client="client"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>

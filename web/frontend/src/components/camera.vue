<script setup lang="ts">
import { onMounted } from 'vue';
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
import { cameraStreamStates } from '../lib/camera-state';

interface Props {
  cameraName: string;
  parentName: string;
  showSwitch: boolean;
  showRefresh: boolean;
  resources: commonApi.ResourceName.AsObject[];
  client: Client;
}

const selectedMap = {
  Live: -1,
  'Manual Refresh': 0,
  'Every 30 Seconds': 30,
  'Every 10 Seconds': 10,
  'Every Second': 1,
} as const;

const props = defineProps<Props>();
const refreshFrequency = $ref('Live');

let pcdExpanded = $ref(false);
let pointcloud = $ref<Uint8Array | undefined>();
let camera = $ref(false);
let cameraFrameIntervalId = $ref(-1);
let streamActive = false;

const initStreamState = () => {
  cameraStreamStates.set(`${props.parentName}-${props.cameraName}`, { 
    on: false,
    live: true,
    parent: props.parentName,
    name: props.cameraName
 });
};

const clearStreamContainer = () => {
  for (let [key, value] of cameraStreamStates) {

    const streamContainer = document.querySelector(
      `[data-parent="${value.parent}"] [data-stream="${value.name}"]`
    );

    if (cameraStreamStates.get(key)?.live) {
      console.log('live');
      streamContainer?.querySelector('img')?.classList.add('hidden');
      streamContainer?.querySelector('video')?.classList.remove('hidden');
    } else {
      console.log('not live');
      streamContainer?.querySelector('img')?.classList.remove('hidden');
      streamContainer?.querySelector('video')?.classList.add('hidden');
    }
  }
};

const viewCamera = async (isOn: boolean) => {
  const streams = new StreamClient(props.client);
  if (isOn) {
    try {
      // only add stream if not already active
      if ( !streamActive ) {
        await streams.add(props.cameraName);
      }
    } catch (error) {
      displayError(error as ServiceError);
    }
    streamActive = true;
  } else {
    try {
      // only remove camera stream if active and base stream is not active
      if ( streamActive ) {
        await streams.remove(props.cameraName);
      }
    } catch (error) {
      displayError(error as ServiceError);
    }
    streamActive = false;
  }
};

const viewFrame = async (cameraName: string) => {
  let blob;
  try {
    blob = await new CameraClient(props.client, cameraName).renderFrame(Camera.MimeType.JPEG);
  } catch (error) {
    displayError(error as ServiceError);
    return;
  }

  const streamContainers = document.querySelectorAll(
    `[data-parent="${props.parentName}"] [data-stream="${cameraName}"]`
  );

  for (const streamContainer of streamContainers) {
    if (!streamContainer.querySelector('img')) {
      const image = new Image();
      image.src = URL.createObjectURL(blob);
      streamContainer.append(image);
    } else {
      streamContainer.setAttribute('src', URL.createObjectURL(blob))
    }
  }
};

const viewCameraFrame = (cameraName: string, time: number) => {
  clearFrameInterval();

  // Live
  if (time === -1) {
    return;
  }

  viewFrame(cameraName);
  if (time > 0) {
    cameraFrameIntervalId = window.setInterval(() => {
      viewFrame(cameraName);
    }, Number(time) * 1000);
  }
};

const clearFrameInterval = () => {
  window.clearInterval(cameraFrameIntervalId);
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
  clearFrameInterval();
  const selectedInterval: number = selectedMap[refreshFrequency as keyof typeof selectedMap];  

  if (refreshFrequency !== 'Live') {
    viewCamera(false);
  } else {
    viewCamera(true);
  }
  
  viewCameraFrame(props.cameraName, selectedInterval);

  cameraStreamStates.set(`${props.parentName}-${props.cameraName}`, { 
    on: camera,
    live: (refreshFrequency === 'Live' ? true : false) ,
    parent: props.parentName,
    name: props.cameraName
  });

  clearStreamContainer();
};

const toggleExpand = () => {
  camera = !camera;

  clearFrameInterval();

  if (camera) {
    selectCameraView();
  } else {
    viewCamera(false);
  }
  clearStreamContainer();
};

const refreshCamera = () => {
  const selectedInterval: number = selectedMap[refreshFrequency as keyof typeof selectedMap];

  viewCameraFrame(props.cameraName, selectedInterval);
  clearFrameInterval();
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
  <div class="flex flex-col gap-4 border-x border-b border-black p-4">
    <v-switch
      v-if="props.showSwitch"
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

    <div class="flex flex-wrap">
      <div
        v-if="camera"
        class="flex flex-wrap justify-items-end gap-2"
      >
        <div 
          v-if="props.showRefresh"
          class="relative"
        >
          <v-select
            v-model="refreshFrequency"
            label="Refresh frequency"
            aria-label="Default select example"
            :options="Object.keys(selectedMap).join(',')"
            @input="selectCameraView"
          />
        </div>
        <div 
          v-if="props.showRefresh"
          class="self-end"
        >
          <v-button
            v-if="camera && refreshFrequency === 'Manual Refresh'"
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
    
    <div
      :aria-label="`${cameraName} stream`"
      :data-stream="cameraName"
      :class="{ hidden: !camera }"
      class="clear-both h-fit transition-all duration-300 ease-in-out"
    />
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
</template>

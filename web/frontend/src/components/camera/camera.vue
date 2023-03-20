<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue';
import { displayError } from '../../lib/error';
import {
  CameraClient,
  Client,
  commonApi,
  ServiceError,
} from '@viamrobotics/sdk';
import { selectedMap } from '../../lib/camera-state';
import { CameraManager } from './camera-manager';
import { StreamManager } from './camera-stream-manager';

interface Props {
  cameraName: string;
  parentName: string;
  resources: commonApi.ResourceName.AsObject[];
  client: Client;
  showExportScreenshot: boolean;
  refreshRate: string | undefined;
  triggerRefresh: boolean;
  streamManager:StreamManager;
}

const props = defineProps<Props>();

const imgEl = $ref<HTMLImageElement>();

let cameraOn = $ref(false);
let cameraFrameIntervalId = $ref(-1);
let isLive = false;
let cameraManager = $ref<CameraManager>();

const clearFrameInterval = () => {
  window.clearInterval(cameraFrameIntervalId);
};

const viewCameraFrame = (time: number) => {
  console.log(imgEl);
  clearFrameInterval();
  cameraManager.setImageElement(imgEl);
  if (time > 0) {
    cameraFrameIntervalId = window.setInterval(() => {
      cameraManager.setImageElement(imgEl);
    }, Number(time) * 1000);
  }
};

const updateCameraRefreshRate = () => {
  if (props.refreshRate !== 'Live') {
    viewCameraFrame(selectedMap[props.refreshRate as keyof typeof selectedMap]);
  }
};

const setupManager = () => {
  if (!props.streamManager.cameraManagers.get(props.cameraName)) {
    props.streamManager.setCameraManager(props.cameraName);
  }
  cameraManager = props.streamManager.cameraManagers.get(props.cameraName);
};

const exportScreenshot = async (cameraName: string) => {
  let blob;
  try {
    blob = await new CameraClient(props.client, cameraName).renderFrame(
      'image/jpeg'
    );
  } catch (error) {
    displayError(error as ServiceError);
    return;
  }

  window.open(URL.createObjectURL(blob), '_blank');
};

const videoStream = $computed(() => {
  return cameraManager.VideoElement;
});

setupManager();

onMounted(() => {
  cameraOn = true;
  if (props.refreshRate === 'Live') {
    isLive = true;
    cameraManager.addStream();
  }
  updateCameraRefreshRate();
});

onUnmounted(() => {
  if (isLive) {
    cameraManager.removeStream();
  }
  cameraOn = false;
  isLive = false;
  clearFrameInterval();
});

// on refreshRate change update camera and manage live connections
watch(() => props.refreshRate, () => {
  if (isLive && props.refreshRate !== 'Live') {
    isLive = false;
    cameraManager.removeStream();
  }
  if (isLive === false && props.refreshRate === 'Live') {
    isLive = true;
    cameraManager.addStream();
  }
  updateCameraRefreshRate();
});

// on prop change refresh camera
watch(() => props.triggerRefresh, () => {
  updateCameraRefreshRate();
});

</script>

<template>
  <div class="flex flex-col gap-2">
    <template v-if="cameraOn">
      <v-button
        v-if="cameraOn && props.showExportScreenshot"
        :aria-label="`View Camera: ${cameraName}`"
        icon="camera"
        label="Export Screenshot"
        @click="exportScreenshot(cameraName)"
      />
    </template>

    <video
      v-show="props.refreshRate === 'Live'"
      :srcObject.prop="videoStream"
      muted
      autoplay
      controls="false"
      playsinline
      :aria-label="`${cameraName} stream`"
      :class="{ hidden: !cameraOn }"
      class="clear-both h-fit transition-all duration-300 ease-in-out"
    />

    <img
      v-show="props.refreshRate !== 'Live'"
      ref="imgEl"
      :aria-label="`${cameraName} stream`"
      :class="{ hidden: props.refreshRate === 'Live' }"
    >
  </div>
</template>

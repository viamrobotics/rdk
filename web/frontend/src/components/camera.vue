<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue';
import { displayError } from '../lib/error';
import {
  StreamClient,
  CameraClient,
  Camera,
  Client,
  commonApi,
  ServiceError,
} from '@viamrobotics/sdk';
import { cameraStreamStates } from '../lib/camera-state';

interface Props {
  cameraName: string;
  parentName: string;
  resources: commonApi.ResourceName.AsObject[];
  client: Client;
  showExportScreenshot: boolean;
  refreshRate: string | undefined;
  triggerRefresh: boolean;
}

const selectedMap = {
  Live: -1,
  'Manual Refresh': 0,
  'Every 30 Seconds': 30,
  'Every 10 Seconds': 10,
  'Every Second': 1,
} as const;

const props = defineProps<Props>();

const videoEl = $ref<HTMLVideoElement>();
const imgEl = $ref<HTMLImageElement>();

let cameraOn = $ref(false);
let cameraFrameIntervalId = $ref(-1);
let camerasOn = $ref(0);

const manageStreamStates = () => {
  let counter = 0;
  for (const value of cameraStreamStates.values()) {
    if (value.name === props.cameraName && value.on) {
      counter++;
    }
  }
  camerasOn = counter;
};

const viewCamera = async (isOn: boolean) => {
  const streams = new StreamClient(props.client);

  streams.on('track', (event) => {
    const eventStream = event.streams[0];
    if (!eventStream) {
      throw new Error('expected event stream to exist');
    }
    videoEl.srcObject = eventStream;
  });

  if (isOn && props.refreshRate === 'Live') {
    cameraStreamStates.set(`${props.parentName}-${props.cameraName}`, {
      on: true,
      live: props.refreshRate === 'Live',
      name: props.cameraName,
    });
    manageStreamStates();

    if (camerasOn === 1) {
      try {
        await streams.add(props.cameraName);
      } catch (error) {
        displayError(error as ServiceError);
      }
    }
  } else if (props.refreshRate === 'Live') {
    cameraStreamStates.set(`${props.parentName}-${props.cameraName}`, {
      on: false,
      live: props.refreshRate === 'Live',
      name: props.cameraName,
    });
    manageStreamStates();

    if (camerasOn === 0) {
      try {
        await streams.remove(props.cameraName);
      } catch (error) {
        displayError(error as ServiceError);
      }
    }
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

  imgEl.setAttribute('src', URL.createObjectURL(blob));
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

const selectCameraView = () => {
  clearFrameInterval();
  const selectedInterval: number = selectedMap[props.refreshRate as keyof typeof selectedMap];

  if (props.refreshRate === 'Live') {
    videoEl.play();
  } else {
    viewCameraFrame(props.cameraName, selectedInterval);
  }
};

const refreshCamera = () => {
  const selectedInterval: number = selectedMap[props.refreshRate as keyof typeof selectedMap];

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
  cameraOn = true;

  clearFrameInterval();

  selectCameraView();

  if (props.refreshRate === 'Live') {
    viewCamera(true);
  }
});

onUnmounted(() => {
  cameraOn = false;

  clearFrameInterval();

  viewCamera(false);
});

// on prop change select camera view
watch(() => props.refreshRate, () => {
  selectCameraView();
});

// on prop change refresh camera
watch(() => props.triggerRefresh, () => {
  refreshCamera();
});

</script>

<template>
  <div class="flex flex-col gap-4">
    <template
      v-if="cameraOn"
    >
      <v-button
        v-if="cameraOn && props.showExportScreenshot"
        icon="camera"
        label="Export Screenshot"
        @click="exportScreenshot(cameraName)"
      />
    </template>

    <video
      v-show="props.refreshRate === 'Live'"
      ref="videoEl"
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

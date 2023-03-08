<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue';
import { displayError } from '../../lib/error';
import {
  StreamClient,
  CameraClient,
  Client,
  commonApi,
  ServiceError,
} from '@viamrobotics/sdk';
import { cameraStreamStates, selectedMap } from '../../lib/camera-state';

interface Props {
  cameraName: string;
  parentName: string;
  resources: commonApi.ResourceName.AsObject[];
  client: Client;
  showExportScreenshot: boolean;
  refreshRate: string | undefined;
  triggerRefresh: boolean;
}

const props = defineProps<Props>();

let videoStream = $ref<MediaStream>();
const imgEl = $ref<HTMLImageElement>();

let cameraOn = $ref(false);
let cameraFrameIntervalId = $ref(-1);
let camerasOn = $ref(0);

const manageStreamStates = (cameraIsOn: boolean) => {
  cameraStreamStates.set(`${props.parentName}-${props.cameraName}`, {
    on: cameraIsOn,
    live: true,
    name: props.cameraName,
  });

  let counter = 0;
  for (const value of cameraStreamStates.values()) {
    if (value.name === props.cameraName && value.on) {
      counter += 1;
    }
  }
  camerasOn = counter;
};

const viewCamera = async (isOn: boolean) => {
  const streams = new StreamClient(props.client);

  streams.on('track', (event) => {
    let [eventStream] = event.streams;
    eventStream = event.streams[0];
    if (!eventStream) {
      throw new Error('expected event stream to exist');
    }
    // Ignore event if received for the wrong stream, in the case of multiple cameras
    if (eventStream.id !== props.cameraName) {
      return;
    }
    videoStream = eventStream;
  });

  if (props.refreshRate === 'Live') {
    manageStreamStates(isOn);

    if (camerasOn === 1) {
      try {
        await streams.add(props.cameraName);
      } catch (error) {
        displayError(error as ServiceError);
      }
    } else if (camerasOn === 0) {
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
    blob = await new CameraClient(props.client, cameraName).renderFrame('image/jpeg');
  } catch (error) {
    displayError(error as ServiceError);
    return;
  }

  imgEl.setAttribute('src', URL.createObjectURL(blob));
};

const clearFrameInterval = () => {
  window.clearInterval(cameraFrameIntervalId);
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

const selectCameraView = () => {
  clearFrameInterval();
  const selectedInterval: number = selectedMap[props.refreshRate as keyof typeof selectedMap];

  if (props.refreshRate === 'Live') {
    viewCamera(true);
  } else {
    viewCamera(false);

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
      'image/jpeg'
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
});

onUnmounted(() => {
  cameraOn = false;
  manageStreamStates(false);
  clearFrameInterval();
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
  <div class="flex flex-col gap-2">
    <template
      v-if="cameraOn"
    >
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

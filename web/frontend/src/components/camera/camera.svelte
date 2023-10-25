<script lang="ts">

import { onMount } from 'svelte';
import { displayError } from '@/lib/error';
import { CameraClient, type ServiceError } from '@viamrobotics/sdk';
import { selectedMap } from '@/lib/camera-state';
import { useRobotClient, useDisconnect } from '@/hooks/robot-client';

export let cameraName: string;
export let showExportScreenshot: boolean;
export let refreshRate: string | undefined;
export let triggerRefresh = false;

const { robotClient, streamManager } = useRobotClient();

let imgEl: HTMLImageElement;
let videoEl: HTMLVideoElement;

let cameraFrameIntervalId = -1;
let isLive = false;

const cameraManager = $streamManager.setCameraManager(cameraName);

const clearFrameInterval = () => {
  window.clearInterval(cameraFrameIntervalId);
};

const viewCameraFrame = (time: number) => {
  clearFrameInterval();
  cameraManager.setImageSrc(imgEl);
  if (time > 0) {
    cameraFrameIntervalId = window.setInterval(() => {
      cameraManager.setImageSrc(imgEl);
    }, Number(time) * 1000);
  }
};

const updateCameraRefreshRate = () => {
  if (refreshRate !== 'Live') {
    viewCameraFrame(selectedMap[refreshRate as keyof typeof selectedMap]);
  }
};

const exportScreenshot = async () => {
  let blob;
  try {
    blob = await new CameraClient($robotClient, cameraName).renderFrame(
      'image/jpeg'
    );
  } catch (error) {
    displayError(error as ServiceError);
    return;
  }

  window.open(URL.createObjectURL(blob), '_blank');
};

onMount(() => {
  videoEl.srcObject = cameraManager.videoStream;

  cameraManager.onOpen = () => {
    videoEl.srcObject = cameraManager.videoStream;
  };
});

useDisconnect(() => {
  if (isLive) {
    cameraManager.removeStream();
  }

  cameraManager.onOpen = undefined;

  isLive = false;

  clearFrameInterval();
});

// on refreshRate change update camera and manage live connections
$: {
  if (isLive && refreshRate !== 'Live') {
    isLive = false;
    cameraManager.removeStream();
  }

  if (!isLive && refreshRate === 'Live') {
    isLive = true;
    cameraManager.addStream();
  }

  updateCameraRefreshRate();
}

// Refresh camera when the trigger changes
let lastTriggerRefresh = triggerRefresh;
$: if (lastTriggerRefresh !== triggerRefresh) {
  lastTriggerRefresh = triggerRefresh;
  updateCameraRefreshRate();
}

</script>

<div class="flex flex-col gap-2">
  {#if showExportScreenshot}
    <v-button
      class="mb-4"
      aria-label={`View camera: ${cameraName}`}
      icon="camera-outline"
      label="Export screenshot"
      on:click={exportScreenshot}
    />
  {/if}

  <div class="max-w-screen-md">
    <video
      bind:this={videoEl}
      muted
      autoplay
      controls={false}
      playsinline
      aria-label={`${cameraName} stream`}
      class:hidden={refreshRate !== 'Live'}
      class="clear-both h-fit transition-all duration-300 ease-in-out"
    />

    <img
      alt='Camera stream'
      bind:this={imgEl}
      class:hidden={refreshRate === 'Live'}
      aria-label={`${cameraName} stream`}
    >
  </div>
</div>

<script lang="ts">

import { onMount, onDestroy } from 'svelte';
import { displayError } from '@/lib/error';
import {
  CameraClient,
  Client,
  type ResponseStream,
  robotApi,
  type ServiceError,
} from '@viamrobotics/sdk';
import { selectedMap } from '@/lib/camera-state';
import type { CameraManager } from './camera-manager';
import type { StreamManager } from './stream-manager';

export let cameraName: string;
export let client: Client;
export let showExportScreenshot: boolean;
export let refreshRate: string | undefined;
export let streamManager:StreamManager;
export let statusStream: ResponseStream<robotApi.StreamStatusResponse> | null

let imgEl: HTMLImageElement;
let videoEl: HTMLVideoElement;

let cameraOn = false;
let cameraFrameIntervalId = -1;
let isLive = false;
let cameraManager: CameraManager = streamManager.setCameraManager(cameraName);

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

const exportScreenshot = async (cameraName: string) => {
  let blob;
  try {
    blob = await new CameraClient(client, cameraName).renderFrame(
      'image/jpeg'
    );
  } catch (error) {
    displayError(error as ServiceError);
    return;
  }

  window.open(URL.createObjectURL(blob), '_blank');
};

onMount(() => {
  cameraOn = true;
  if (refreshRate === 'Live') {
    isLive = true;
    cameraManager.addStream();
  }

  updateCameraRefreshRate();

  statusStream?.on('end', () => clearFrameInterval());
});

onDestroy(() => {
  if (isLive) {
    cameraManager.removeStream();
  }

  cameraOn = false;
  isLive = false;

  clearFrameInterval();
});

$: if (videoEl) {
  videoEl.srcObject = cameraManager.videoStream
}

// on refreshRate change update camera and manage live connections
$: {
  if (isLive && refreshRate !== 'Live') {
    isLive = false;
    cameraManager.removeStream();
  }

  if (isLive === false && refreshRate === 'Live') {
    isLive = true;
    cameraManager.addStream();
  }

  updateCameraRefreshRate();
}

// on prop change refresh camera
$: updateCameraRefreshRate();

</script>

<div class="flex flex-col gap-2">
  {#if cameraOn && showExportScreenshot}
    <v-button
      class="mb-4"
      aria-label={`View Camera: ${cameraName}`}
      icon="camera"
      label="Export Screenshot"
      on:click={() => exportScreenshot(cameraName)}
    />
  {/if}

  <div class="max-w-screen-md">
    {#if refreshRate === 'Live'}
      <video
        bind:this={videoEl}
        muted
        autoplay
        controls={false}
        playsinline
        aria-label={`${cameraName} stream`}
        class:hidden={!cameraOn}
        class="clear-both h-fit transition-all duration-300 ease-in-out"
      />
    {/if}

    {#if refreshRate !== 'Live'}
      <img
        alt='Camera stream'
        bind:this={imgEl}
        aria-label={`${cameraName} stream`}
      >
    {/if}
  </div>
</div>

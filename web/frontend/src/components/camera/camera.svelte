<script lang="ts">
import { useConnect, useRobotClient } from '@/hooks/robot-client';
import { selectedMap } from '@/lib/camera-state';
import { displayError } from '@/lib/error';
import { setAsyncInterval } from '@/lib/schedule';
import { CameraClient, ConnectError } from '@viamrobotics/sdk';
import { noop } from 'lodash-es';
import LiveCamera from './live-camera.svelte';

export let cameraName: string;
export let showExportScreenshot: boolean;
export let refreshRate: string | undefined;
export let triggerRefresh = false;

const { robotClient, streamManager } = useRobotClient();

let imgEl: HTMLImageElement;

const cameraManager = $streamManager.setCameraManager(cameraName);

let clearFrameInterval = noop;

const viewCameraFrame = (time: number) => {
  clearFrameInterval();
  cameraManager.setImageSrc(imgEl);
  if (time > 0) {
    clearFrameInterval = setAsyncInterval(
      async () => cameraManager.setImageSrc(imgEl),
      Number(time) * 1000,
      true
    );
  }
};

const updateCameraRefreshRate = () => {
  if (refreshRate === 'Live') {
    clearFrameInterval();
  } else {
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
    displayError(error as ConnectError);
    return;
  }

  window.open(URL.createObjectURL(blob), '_blank');
};

useConnect(() => {
  updateCameraRefreshRate();
  return () => clearFrameInterval();
});

// Refresh camera when the trigger changes
let lastTriggerRefresh = triggerRefresh;
let lastRefreshRate = refreshRate;
$: if (
  lastTriggerRefresh !== triggerRefresh ||
  lastRefreshRate !== refreshRate
) {
  lastTriggerRefresh = triggerRefresh;
  lastRefreshRate = refreshRate;
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
    {#if refreshRate === 'Live'}
      <LiveCamera
        {cameraName}
        {cameraManager}
      />
    {/if}

    <img
      alt="Camera stream"
      bind:this={imgEl}
      class:hidden={refreshRate === 'Live'}
      aria-label={`${cameraName} stream`}
    />
  </div>
</div>

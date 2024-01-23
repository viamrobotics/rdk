<script lang='ts'>
import { useConnect } from '@/hooks/robot-client';
import type { CameraManager } from './camera-manager';

export let cameraName: string;
export let cameraManager: CameraManager

let videoEl: HTMLVideoElement;

useConnect(() => {
  videoEl.srcObject = cameraManager.videoStream;
  cameraManager.onOpen = () => {
    videoEl.srcObject = cameraManager.videoStream;
  };

  cameraManager.addStream();

  return () => {
    cameraManager.onOpen = undefined;
    cameraManager.removeStream();
  }
});

</script>

<video
  bind:this={videoEl}
  muted
  autoplay
  controls={false}
  playsinline
  aria-label={`${cameraName} stream`}
  class="clear-both h-fit transition-all duration-300 ease-in-out"
/>
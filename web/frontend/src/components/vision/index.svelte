<script lang="ts">
import Collapse from '@/lib/components/collapse.svelte';
import { useRobotClient } from '@/hooks/robot-client';
import { components } from '@/stores/resources';
import {
  CameraClient,
  VisionClient,
  type Classification,
  type Detection,
  type ServiceError,
} from '@viamrobotics/sdk';
import { rcLogConditionally } from '../../lib/log';
import { displayError } from '@/lib/error';
import { filterSubtype } from '../../lib/resource';
import get from 'lodash-es/get';

export let names: string[];
let [serviceName] = names;

$: cameras = filterSubtype($components, 'camera');

const { robotClient } = useRobotClient();

$: visionClient = new VisionClient($robotClient, serviceName!, {
  requestLogger: rcLogConditionally,
});

let canvasEl: HTMLCanvasElement;
const initialCameras = filterSubtype($components, 'camera');
let cameraName =
  initialCameras.length > 0 ? initialCameras[0]!.name : undefined;
let detections: Detection[] = [];
let classifications: Classification[] = [];
let ranOnce = false;

const pixelRatio: number = window.devicePixelRatio || 1;

const autoRun = async () => {
  if (ranOnce) {
    await run();
  }
};

const selectService = async (event: CustomEvent<{ value: string }>) => {
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition -- it's just incorrect
  if (!event.detail) {
    return;
  }
  serviceName = event.detail.value;
  await autoRun();
};

const selectCamera = async (event: CustomEvent<{ value: string }>) => {
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition -- it's just incorrect
  if (!event.detail) {
    return;
  }
  cameraName = event.detail.value;
  await autoRun();
};

const isNotADetectorError = (error: unknown) => {
  const errorMessages = get(
    error,
    `metadata.headersMap.grpc-message`,
    [] as string[]
  );

  return errorMessages.some((msg) =>
    msg.includes('does not implement a Detector')
  );
};

const run = async () => {
  if (!cameraName) {
    return;
  }
  ranOnce = true;
  const cameraClient = new CameraClient($robotClient, cameraName, {
    requestLogger: rcLogConditionally,
  });
  const mime = 'image/jpeg';
  const imageBlob = await cameraClient.renderFrame(mime);
  const imageData = new Uint8Array(await imageBlob.arrayBuffer());

  const canvasCtx = canvasEl.getContext('2d')!;
  const img = new Image();
  // eslint-disable-next-line @typescript-eslint/no-misused-promises -- not much to do about it
  img.addEventListener('load', async () => {
    canvasEl.width = img.width * pixelRatio;
    canvasEl.height = img.height * pixelRatio;
    canvasEl.style.width = `${img.width}px`;
    canvasEl.style.height = `${img.height}px`;
    canvasCtx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);

    canvasCtx.drawImage(img, 0, 0);

    detections = [];
    classifications = [];
    try {
      detections = await visionClient.getDetections(
        imageData,
        img.width,
        img.height,
        mime
      );

      for (const det of detections) {
        canvasCtx.save();
        canvasCtx.beginPath();
        canvasCtx.strokeStyle = 'red';
        canvasCtx.fillStyle = 'red';
        const width = det.xMax - det.xMin;
        const height = det.yMax - det.yMin;
        canvasCtx.rect(det.xMin, det.yMin, width, height);
        canvasCtx.stroke();
        canvasCtx.font = `12px monospace`;
        canvasCtx.textAlign = 'left';
        canvasCtx.textBaseline = 'top';
        canvasCtx.fillText(
          `${det.className} (${det.confidence})`,
          det.xMin,
          det.yMin,
          width
        );
        canvasCtx.restore();
      }
    } catch (detError) {
      if (!isNotADetectorError(detError)) {
        displayError(detError as ServiceError);
        return;
      }

      try {
        classifications = await visionClient.getClassifications(
          imageData,
          img.width,
          img.height,
          mime,
          100
        );
        let y = 0;
        for (const cls of classifications) {
          canvasCtx.save();
          canvasCtx.fillStyle = 'red';
          canvasCtx.font = `12px monospace`;
          canvasCtx.textAlign = 'left';
          canvasCtx.textBaseline = 'top';
          canvasCtx.fillText(`${cls.className} (${cls.confidence})`, 0, y);
          canvasCtx.restore();
          y += 15;
        }
      } catch (clasError) {
        displayError(clasError as ServiceError);
      }
    }
  });
  img.setAttribute('src', URL.createObjectURL(imageBlob));
};

const onRefreshKeyPress = async (event: KeyboardEvent) => {
  if (event.key === 'Enter' || event.key === 'Space') {
    await run();
  }
};
</script>

<Collapse title="Vision">
  <div
    class="flex flex-wrap gap-4 border border-t-0 border-medium sm:flex-nowrap"
  >
    <div class="flex min-w-fit flex-col gap-4 p-4">
      <v-select
        value={cameraName}
        class="w-fit"
        label="Camera"
        aria-label="Camera"
        options={cameras.map((cam) => cam.name).join(',')}
        on:input={selectCamera}
      />
      <v-notify
        class="max-w-sm"
        variant="info"
        title="Classifications won't show here"
        message="Right now this card only supports detections. Soon your classifications will show here too."
      />
    </div>

    <div class="flex min-w-fit flex-col gap-4 p-4">
      <v-button
        class="mb-4"
        label="Refresh"
        role="button"
        tabindex="0"
        on:keypress={onRefreshKeyPress}
        on:click={run}
      />
      <canvas
        class="max-w-screen-md"
        bind:this={canvasEl}
        aria-label={`frame from ${cameraName}`}
      />
    </div>

    <div class="flex min-w-fit flex-col gap-4 p-4">
      <v-select
        value={names[0]}
        class="w-fit"
        label="Vision service"
        aria-label="Vision service"
        options={names.join(',')}
        on:input={selectService}
      />

      {#if detections.length > 0}
        <table class="table-auto border border-medium">
          <th class="border border-medium p-2"> Position </th>
          <th class="border border-medium p-2"> Dimensions </th>
          <th class="border border-medium p-2"> Class </th>
          <th class="border border-medium p-2"> Confidence </th>
          {#each detections as det}
            <tr>
              <td class="border border-medium p-2">
                ({det.xMin}, {det.yMin})
              </td>
              <td class="border border-medium p-2">
                {det.xMax - det.xMin}x{det.yMax - det.yMin}
              </td>
              <td class="border border-medium p-2">
                {det.className}
              </td>
              <td class="border border-medium p-2">
                {det.confidence}
              </td>
            </tr>
          {/each}
        </table>
      {/if}
      {#if classifications.length > 0}
        <table class="table-auto border border-medium">
          <th class="border border-medium p-2"> Class </th>
          <th class="border border-medium p-2"> Confidence </th>
          {#each classifications as cls}
            <tr>
              <td class="border border-medium p-2">
                {cls.className}
              </td>
              <td class="border border-medium p-2">
                {cls.confidence}
              </td>
            </tr>
          {/each}
        </table>
      {/if}
    </div>
  </div>
</Collapse>

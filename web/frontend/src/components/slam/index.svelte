<script lang="ts">

import { onMount, onDestroy } from 'svelte';
import * as THREE from 'three';
import {
  Client,
  commonApi,
  type ResponseStream,
  robotApi,
  type ServiceError,
} from '@viamrobotics/sdk';
import { displayError, isServiceError } from '@/lib/error';
import PCD from '@/components/pcd/pcd-view.svelte'
import { copyToClipboardWithToast } from '@/lib/copy-to-clipboard';
import Slam2dRenderer from './2d-renderer.svelte';
import { filterResources } from '@/lib/resource';
import { getPointCloudMap, getSLAMPosition } from '@/api/slam';
import { moveOnMap, stopMoveOnMap } from '@/api/motion';
import { toast } from '@/lib/toast';
import { setAsyncInterval } from '@/lib/schedule';

export let name: string
export let resources: commonApi.ResourceName.AsObject[]
export let client: Client
export let statusStream: ResponseStream<robotApi.StreamStatusResponse> | null
export let operations: { op: robotApi.Operation.AsObject; elapsed: number }[]

const refreshErrorMessage = 'Error refreshing map. The map shown may be stale.';

let slam2dTimeoutId: number;
let slam3dTimeoutId: number;
let refreshErrorMessage2d: string | undefined;
let refreshErrorMessage3d: string | undefined;
let refresh2dRate = 'manual';
let refresh3dRate = 'manual';
let pointcloud: Uint8Array | undefined;
let pose: commonApi.Pose | undefined;
let show2d = false;
let show3d = false;
let showAxes = true;
let refresh2DCancelled = true;
let refresh3DCancelled = true;
let destination: THREE.Vector2 | undefined

$: loaded2d = pointcloud !== undefined && pose !== undefined;
$: moveClicked = operations.find(({ op }) => op.method.includes('MoveOnMap'))

// get all resources which are bases
$: baseResources = filterResources(resources, 'rdk', 'component', 'base');

// allowMove is only true if we have a base, there exists a destination and there is no in-flight MoveOnMap req
$: allowMove = baseResources.length === 1 && destination && !moveClicked;

const deleteDestinationMarker = () => {
  destination = undefined
};

const refresh2d = async () => {
  const [map, nextPose] = await Promise.all([
    getPointCloudMap(client, name),
    getSLAMPosition(client, name),
  ])

  /*
   * The pose is returned in millimeters, but we need
   * to convert to meters to display on the frontend.
   */
  nextPose?.setX(nextPose.getX() / 1000);
  nextPose?.setY(nextPose.getY() / 1000);
  nextPose?.setZ(nextPose.getZ() / 1000);

  return { map, nextPose };
};

const handleError = (errorLocation: string, error: unknown): void => {
  if (isServiceError(error)) {
    displayError(error as ServiceError);
  } else {
    displayError(`${errorLocation} hit error: ${error}`);
  }
};

const scheduleRefresh2d = async () => {
  try {
    const response = await refresh2d();
    pointcloud = response.map;
    pose = response.nextPose;
  } catch (error) {
    handleError('refresh2d', error);
    refresh2dRate = 'manual';
    refreshErrorMessage2d = error !== null && typeof error === 'object' && 'message' in error
      ? `${refreshErrorMessage} ${error.message}`
      : `${refreshErrorMessage} ${error}`;
    return;
  }

  if (refresh2DCancelled) {
    return;
  }
  
  slam2dTimeoutId = window.setTimeout(scheduleRefresh2d, Number.parseFloat(refresh2dRate) * 1000);
};

const scheduleRefresh3d = async () => {
  try {
    pointcloud = await getPointCloudMap(client, name);
  } catch (error) {
    handleError('fetchSLAMMap', error);
    refresh3dRate = 'manual';
    refreshErrorMessage3d = error !== null && typeof error === 'object' && 'message' in error
      ? `${refreshErrorMessage} ${error.message}`
      : `${refreshErrorMessage} ${error}`;
    return;
  }

  if (refresh3DCancelled) {
    return;
  }

  slam3dTimeoutId = window.setTimeout(scheduleRefresh3d, Number.parseFloat(refresh3dRate) * 1000);
};

const updateSLAM2dRefreshFrequency = async () => {
  refresh2DCancelled = true;
  window.clearTimeout(slam2dTimeoutId);
  refreshErrorMessage2d = undefined;
  refreshErrorMessage3d = undefined;

  if (refresh2dRate === 'manual') {
    try {
      const response = await refresh2d();
      pointcloud = response.map;
      pose = response.pose
    } catch (error) {
      handleError('refresh2d', error);
      refresh2dRate = 'manual';
      refreshErrorMessage2d = error !== null && typeof error === 'object' && 'message' in error
        ? `${refreshErrorMessage} ${error.message}`
        : `${refreshErrorMessage} ${error}`;
    }
  } else {
    refresh2DCancelled = false;
    scheduleRefresh2d();
  }
};

const updateSLAM3dRefreshFrequency = async () => {
  refresh3DCancelled = true;
  window.clearTimeout(slam3dTimeoutId);
  refreshErrorMessage2d = undefined;
  refreshErrorMessage3d = undefined;

  if (refresh3dRate === 'manual') {
    try {
      pointcloud = await getPointCloudMap(client, name);
    } catch (error) {
      handleError('fetchSLAMMap', error);
      refresh3dRate = 'manual';
      refreshErrorMessage3d = error !== null && typeof error === 'object' && 'message' in error
        ? `${refreshErrorMessage} ${error.message}`
        : `${refreshErrorMessage} ${error}`;
    }
  } else {
    refresh3DCancelled = false;
    setAsyncInterval(scheduleRefresh3d, Number.parseFloat(refresh3dRate) * 1000)
    scheduleRefresh3d();
  }
};

const toggle3dExpand = () => {
  show3d = !show3d;
  if (!show3d) {
    refresh3dRate = 'manual';
    return;
  }
  updateSLAM3dRefreshFrequency();
};

const toggle2dExpand = () => {
  show2d = !show2d;
  if (!show2d) {
    refresh2dRate = 'manual';
    return;
  }
  updateSLAM2dRefreshFrequency();
};

const refresh2dMap = () => {
  refresh2dRate = 'manual'
  updateSLAM2dRefreshFrequency();
};

const refresh3dMap = () => {
  refresh2dRate = 'manual'
  updateSLAM3dRefreshFrequency();
};

const handle2dRenderClick = (event: CustomEvent) => {
  destination = event.detail;
};

const handleUpdateDestX = (event: CustomEvent<{ value: string }>) => {
  destination ??= new THREE.Vector2()
  destination.x = Number.parseFloat(event.detail.value);
};

const handleUpdateDestY = (event: CustomEvent<{ value: string }>) => {
  destination ??= new THREE.Vector2()
  destination.y = Number.parseFloat(event.detail.value);
};

const baseCopyPosition = () => {
  copyToClipboardWithToast(JSON.stringify(pose));
};

const toggleAxes = () => {
  showAxes = !showAxes;
};

const handleMoveClick = async () => {
  try {
    await moveOnMap(client, name, baseResources[0]!.name, destination!.x, destination!.y)
  } catch (error) {
    toast.error((error as ServiceError).message)
  }
}

const handleStopMoveClick = async () => {
  try {
    await stopMoveOnMap(client, operations)
  } catch (error) {
    toast.error((error as ServiceError).message)
  }
}

onMount(() => {
  statusStream?.on('end', () => {
    window.clearTimeout(slam2dTimeoutId);
    window.clearTimeout(slam3dTimeoutId);
  });
});

onDestroy(() => {
  window.clearTimeout(slam2dTimeoutId);
  window.clearTimeout(slam3dTimeoutId);
});

</script>

<v-collapse
  title={name}
  class="slam"
  on:toggle={toggle2dExpand}
>
  <v-breadcrumbs
    slot="title"
    crumbs="slam"
  />
  <v-button
    slot="header"
    variant="danger"
    icon="stop-circle"
    disabled={moveClicked ? 'false' : 'true'}
    label="STOP"
    on:click={handleStopMoveClick}
    on:keydown={handleStopMoveClick}
  />
  <div class="flex flex-wrap gap-4 border border-t-0 border-medium sm:flex-nowrap">
    <div class="flex min-w-fit flex-col gap-4 p-4">
      <div class="float-left pb-4">
        <div class="flex">
          <div class="w-64">
            <p class="mb-1 font-bold text-gray-800">
              Map
            </p>
            <div class="relative">
              <p class="mb-1 text-xs text-gray-500 ">
                Refresh frequency
              </p>
              <select
                bind:value={refresh2dRate}
                class="
                    m-0 w-full appearance-none border border-solid border-medium bg-white bg-clip-padding
                    px-3 py-1.5 text-xs font-normal text-default focus:outline-none
                  "
                aria-label="Default select example"
                on:change={updateSLAM2dRefreshFrequency}
              >
                <option value="manual">
                  Manual Refresh
                </option>
                <option value="30">
                  Every 30 seconds
                </option>
                <option value="10">
                  Every 10 seconds
                </option>
                <option value="5">
                  Every 5 seconds
                </option>
                <option value="1">
                  Every second
                </option>
              </select>
              <div class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2">
                <svg
                  class="h-4 w-4 stroke-2 text-gray-700"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-linejoin="round"
                  stroke-linecap="round"
                  fill="none"
                >
                  <path d="M18 16L12 22L6 16" />
                </svg>
              </div>
            </div>
          </div>
          <div class="px-2 pt-11">
            <v-button
              label="Refresh"
              icon="refresh"
              on:click={refresh2dMap}
            />
          </div>
        </div>
        <hr class="my-4 border-t border-medium">
        <div class="flex flex-row">
          <p class="mb-1 pr-52 font-bold text-gray-800">
            Ending Position
          </p>
          <v-icon
            name="trash"
            on:click={deleteDestinationMarker}
          />
        </div>
        <div class="flex flex-row pb-2">
          <v-input
            type="number"
            label="x"
            incrementor="slider"
            value={destination?.x}
            step="0.1"
            on:input={handleUpdateDestX}
          />
          <v-input
            class="pl-2"
            type="number"
            label="y"
            incrementor="slider"
            value={destination?.y}
            step="0.1"
            on:input={handleUpdateDestY}
          />
        </div>
        <v-button
          class="pt-1"
          label="Move"
          variant="success"
          icon="play-circle-filled"
          disabled={allowMove ? 'false' : 'true'}
          on:click={handleMoveClick}
          on:keydown={handleMoveClick}
        />
        <v-switch
          class="pt-2"
          label="Show Axes"
          value={showAxes ? 'on' : 'off'}
          on:input={toggleAxes}
        />
      </div>
    </div>
    <div class="gap-4x border-border-1 w-full justify-start sm:border-l">
      {#if refreshErrorMessage2d && show2d}
        <div class="border-l-4 border-red-500 bg-gray-100 px-4 py-3">
          {refreshErrorMessage2d}
        </div>
      {/if}

      {#if loaded2d && show2d}
        <div>
          <div class="flex flex-row pl-5 pt-3">
            <div class="flex flex-col">
              <p class="text-xs">
                Current Position
              </p>
              
              {#if pose}
                <div class="flex flex-row items-center">
                  <p class="items-end pr-2 text-xs text-gray-500">x</p>
                  <p>{pose.getX().toFixed(1)}</p>

                  <p class="pl-9 pr-2 text-xs text-gray-500">y</p>
                  <p>{pose.getY().toFixed(1)}</p>

                  <p class="pl-9 pr-2 text-xs text-gray-500">z</p>
                  <p>{pose.getZ().toFixed(1)}</p>
                </div>
              {/if}
            </div>
            <div class="flex flex-col pl-10">
              <p class="text-xs">
                Current Orientation
              </p>

              {#if pose}
                <div class="flex flex-row items-center">
                  <p class="pr-2 text-xs text-gray-500">o<sub>x</sub></p>
                  <p>{pose.getOX().toFixed(1)}</p>

                  <p class="pl-9 pr-2 text-xs text-gray-500">o<sub>y</sub></p>
                  <p>{pose.getOY().toFixed(1)}</p>

                  <p class="pl-9 pr-2 text-xs text-gray-500">o<sub>z</sub></p>
                  <p>{pose.getOZ().toFixed(1)}</p>

                  <p class="pl-9 pr-2 text-xs text-gray-500">&theta;</p>
                  <p>{pose.getTheta().toFixed(1)}</p>
                </div>
              {/if}
            </div>
            <div class="pl-4 pt-2">
              <v-icon
                name="copy"
                on:click={baseCopyPosition}
              />
            </div>
          </div>
          <Slam2dRenderer
            {pointcloud}
            {pose}
            {destination}
            axesVisible={showAxes}
            on:click={handle2dRenderClick}
          />
        </div>
      {/if}
    </div>
  </div>

  <div class="border border-medium border-t-transparent p-4">
    <v-switch
      label="View SLAM Map (3D)"
      value={show3d ? 'on' : 'off'}
      on:input={toggle3dExpand}
    />
    {#if refreshErrorMessage3d && show3d}
      <div class="border-l-4 border-red-500 bg-gray-100 px-4 py-3">
        {refreshErrorMessage3d}
      </div>
    {/if}
    
    {#if show3d}
      <div class="flex items-end gap-2">
        <div class="w-56">
          <p class="font-label mb-1 text-gray-800">
            Refresh frequency
          </p>
          <div class="relative">
            <select
              bind:value={refresh3dRate}
              class="
                m-0 w-full appearance-none border border-solid border-medium bg-white
                bg-clip-padding px-3 py-1.5 text-xs font-normal text-gray-700 focus:outline-none
              "
              aria-label="Default select example"
              on:change={updateSLAM3dRefreshFrequency}
            >
              <option value="manual">
                Manual Refresh
              </option>
              <option value="30">
                Every 30 seconds
              </option>
              <option value="10">
                Every 10 seconds
              </option>
              <option value="5">
                Every 5 seconds
              </option>
              <option value="1">
                Every second
              </option>
            </select>
            <div class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2">
              <svg
                class="h-4 w-4 stroke-2 text-default"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-linejoin="round"
                stroke-linecap="round"
                fill="none"
              >
                <path d="M18 16L12 22L6 16" />
              </svg>
            </div>
          </div>
        </div>

        <v-button
          icon="refresh"
          label="Refresh"
          on:click={refresh3dMap}
        />
      </div>

      <PCD
        {resources}
        {pointcloud}
        {client}
      />
    {/if}
  </div>
</v-collapse>

<script lang="ts">

import * as THREE from 'three';
import { commonApi, type ServiceError } from '@viamrobotics/sdk';
import { copyToClipboard } from '@/lib/copy-to-clipboard';
import { filterSubtype } from '@/lib/resource';
import { getPointCloudMap, getSLAMPosition, getSLAMMapInfo } from '@/api/slam';
import { moveOnMap, stopMoveOnMap } from '@/api/motion';
import { notify } from '@viamrobotics/prime';
import { setAsyncInterval } from '@/lib/schedule';
import { components } from '@/stores/resources';
import Collapse from '@/lib/components/collapse.svelte';
import PCD from '@/components/pcd/pcd-view.svelte';
import Slam2dRenderer from './2d-renderer.svelte';
import { useRobotClient, useDisconnect } from '@/hooks/robot-client';
import { Timestamp } from 'google-protobuf/google/protobuf/timestamp_pb';
export let name: string;

const { robotClient, operations } = useRobotClient();

const refreshErrorMessage = 'Error refreshing map. The map shown may be stale.';

let clear2dRefresh: (() => void) | undefined;
let clear3dRefresh: (() => void) | undefined;

let refreshErrorMessage2d: string | undefined;
let refreshErrorMessage3d: string | undefined;
let refresh2dRate = 'manual';
let refresh3dRate = 'manual';
let pointcloud: Uint8Array | undefined;
let pose: commonApi.Pose | undefined;

let timestamp: Timestamp | undefined;
timestamp = new Timestamp();

let show2d = false;
let show3d = false;
let showAxes = true;
let destination: THREE.Vector2 | undefined;
let labelUnits = 'm';

$: loaded2d = pointcloud !== undefined && pose !== undefined;
$: moveClicked = $operations.find(({ op }) => op.method.includes('MoveOnMap'));
$: unitScale = labelUnits === 'm' ? 1 : 1000;

// get all resources which are bases
$: bases = filterSubtype($components, 'base');

// allowMove is only true if we have a base, there exists a destination and there is no in-flight MoveOnMap req
$: allowMove = bases.length === 1 && destination && !moveClicked;

const deleteDestinationMarker = () => {
  destination = undefined;
};

const refresh2d = async () => {

  try {

    const mapTimestamp = await getSLAMMapInfo($robotClient, name);
    let nextPose;
    if (mapTimestamp?.getSeconds() === timestamp?.getSeconds()) {
      nextPose = await getSLAMPosition($robotClient, name);
    } else {
      [pointcloud, nextPose] = await Promise.all([
        getPointCloudMap($robotClient, name),
        getSLAMPosition($robotClient, name),
      ]);
    }

    /*
     * The pose is returned in millimeters, but we need
     * to convert to meters to display on the frontend.
     */
    nextPose?.setX(nextPose.getX() / 1000);
    nextPose?.setY(nextPose.getY() / 1000);
    nextPose?.setZ(nextPose.getZ() / 1000);
    pose = nextPose;
    timestamp ??= mapTimestamp;
  } catch (error) {
    refreshErrorMessage2d = error !== null && typeof error === 'object' && 'message' in error
      ? `${refreshErrorMessage} ${error.message}`
      : `${refreshErrorMessage} ${error}`;
  }
};

const refresh3d = async () => {
  try {
    const mapTimestamp = await getSLAMMapInfo($robotClient, name);
    if (mapTimestamp?.getSeconds() !== timestamp?.getSeconds()) {
      pointcloud = await getPointCloudMap($robotClient, name);
    }
    timestamp ??= mapTimestamp;
  } catch (error) {
    refreshErrorMessage3d = error !== null && typeof error === 'object' && 'message' in error
      ? `${refreshErrorMessage} ${error.message}`
      : `${refreshErrorMessage} ${error}`;

  }
};

const updateSLAM2dRefreshFrequency = () => {
  clear2dRefresh?.();
  refresh2d();

  refreshErrorMessage2d = undefined;

  if (refresh2dRate !== 'manual') {
    clear2dRefresh = setAsyncInterval(refresh2d, Number.parseFloat(refresh2dRate) * 1000);
  }
};

const updateSLAM3dRefreshFrequency = () => {
  clear3dRefresh?.();
  refresh3d();

  refreshErrorMessage3d = undefined;

  if (refresh3dRate !== 'manual') {
    clear3dRefresh = setAsyncInterval(refresh3d, Number.parseFloat(refresh3dRate) * 1000);
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
  refresh2dRate = 'manual';
  updateSLAM2dRefreshFrequency();
};

const refresh3dMap = () => {
  refresh2dRate = 'manual';
  updateSLAM3dRefreshFrequency();
};

const handle2dRenderClick = (event: CustomEvent) => {
  destination = event.detail;
};

const handleUpdateDestX = (event: CustomEvent<{ value: string }>) => {
  destination ??= new THREE.Vector2();
  destination.x = Number.parseFloat(event.detail.value) * (labelUnits === 'mm' ? 0.001 : 1);
};

const handleUpdateDestY = (event: CustomEvent<{ value: string }>) => {
  destination ??= new THREE.Vector2();
  destination.y = Number.parseFloat(event.detail.value) * (labelUnits === 'mm' ? 0.001 : 1);
};

const baseCopyPosition = () => {
  copyToClipboard(JSON.stringify({
    x: pose?.getX(),
    y: pose?.getY(),
    z: pose?.getZ(),
    ox: 0,
    oy: 0,
    oz: 1,
    th: pose?.getTheta(),
  }));
};

const toggleAxes = () => {
  showAxes = !showAxes;
};

const handleMoveClick = async () => {
  try {
    await moveOnMap($robotClient, name, bases[0]!.name, destination!.x, destination!.y);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

const handleStopMoveClick = async () => {
  try {
    await stopMoveOnMap($robotClient, $operations);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

const toggleExpand = (event: CustomEvent<{ open: boolean }>) => {
  const { open } = event.detail;

  if (open) {
    toggle2dExpand();
  } else {
    clear2dRefresh?.();
    clear3dRefresh?.();
  }
};

useDisconnect(() => {
  clear2dRefresh?.();
  clear3dRefresh?.();
});

</script>

<Collapse
  title={name}
  on:toggle={toggleExpand}
>
  <v-breadcrumbs slot="title" crumbs="slam" />
  <v-button
    slot="header"
    variant="danger"
    icon="stop-circle"
    disabled={moveClicked ? 'false' : 'true'}
    label="Stop"
    on:click={handleStopMoveClick}
    on:keydown={handleStopMoveClick}
  />
  <div class="flex flex-wrap gap-4 border border-t-0 border-medium sm:flex-nowrap">
    <div class="flex min-w-fit flex-col gap-4 p-4">
      <div class="float-left pb-4">
        <div>
          <p class="mb-1 font-bold text-gray-800">
            Map
          </p>
          <div class="flex items-end gap-2 w-64">
            <div class="relative">
              <p class="mb-1 text-xs text-gray-500">
                Refresh frequency
              </p>
              <select
                bind:value={refresh2dRate}
                class="
                    m-0 w-full min-w-[200px] appearance-none border border-solid border-medium bg-white bg-clip-padding
                    px-3 py-1.5 text-xs font-normal text-default focus:outline-none
                  "
                aria-label="Default select example"
                on:change={updateSLAM2dRefreshFrequency}
              >
                <option value="manual">
                  Manual refresh
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
              <v-icon
                name='chevron-down'
                class="pointer-events-none absolute bottom-0 h-[30px] right-0 flex items-center px-2"
              />
            </div>
              <v-button
                label="Refresh"
                icon="refresh"
                on:click={refresh2dMap}
                on:keydown={refresh2dMap}
              />
          </div>
        </div>

        <hr class="my-4 border-t border-medium">
        <div class="flex gap-2 mb-1">
          <p class="font-bold text-gray-800">
            End position
          </p>
          <button
            class='text-xs hover:underline'
            on:click={() => (labelUnits = labelUnits === 'mm' ? 'm' : 'mm')}
          >
            ({labelUnits})
          </button>

        </div>
        <div class="flex flex-row items-end gap-2 pb-2">
          <v-input
            type="number"
            label="x"
            incrementor="slider"
            value={destination ? (destination.x * unitScale).toFixed(5) : ''}
            step={labelUnits === 'mm' ? '10' : '1'}
            on:input={handleUpdateDestX}
          />
          <v-input
            type="number"
            label="y"
            incrementor="slider"
            value={destination ? (destination.y * unitScale).toFixed(5) : ''}
            step={labelUnits === 'mm' ? '10' : '1'}
            on:input={handleUpdateDestY}
          />
          <v-button
            class="pt-1"
            label="Move"
            variant="success"
            icon="play-circle-filled"
            disabled={allowMove ? 'false' : 'true'}
            on:click={handleMoveClick}
            on:keydown={handleMoveClick}
          />
          <v-button
            variant="icon"
            icon="trash"
            on:click={deleteDestinationMarker}
            on:keydown={deleteDestinationMarker}
          />
        </div>

        <v-switch
          class="pt-2"
          label="Show grid"
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
          <div class="flex flex-row pl-5 py-2 border-b border-b-light">
            <div class="flex flex-col gap-0.5">
              <div class='flex gap-2'>
                <p class="text-xs">
                  Current position
                </p>
                <button
                  class='text-xs hover:underline'
                  on:click={() => (labelUnits = labelUnits === 'mm' ? 'm' : 'mm')}
                >
                  ({labelUnits})
                </button>
              </div>

              {#if pose}
                <div class="flex flex-row items-center">
                  <p class="items-end pr-1.5 text-xs text-gray-500">x</p>
                  <p>{(pose.getX() * unitScale).toFixed(1)}</p>

                  <p class="pl-6 pr-1.5 text-xs text-gray-500">y</p>
                  <p>{(pose.getY() * unitScale).toFixed(1)}</p>

                  <p class="pl-6 pr-1.5 text-xs text-gray-500">z</p>
                  <p>{(pose.getZ() * unitScale).toFixed(1)}</p>
                </div>
              {/if}
            </div>
            <div class="flex flex-col gap-0.5 pl-10">
              <p class="text-xs">
                Current orientation
              </p>

              {#if pose}
                <div class="flex flex-row items-center">
                  <p class="pr-1.5 text-xs text-gray-500">o<sub>x</sub></p>
                  <p>{pose.getOX().toFixed(1)}</p>

                  <p class="pl-6 pr-1.5 text-xs text-gray-500">o<sub>y</sub></p>
                  <p>{pose.getOY().toFixed(1)}</p>

                  <p class="pl-6 pr-1.5 text-xs text-gray-500">o<sub>z</sub></p>
                  <p>{pose.getOZ().toFixed(1)}</p>

                  <p class="pl-6 pr-1.5 text-xs text-gray-500">&theta;</p>
                  <p>{pose.getTheta().toFixed(1)}</p>
                </div>
              {/if}
            </div>

            <v-button
              tooltip='Copy pose to clipboard'
              class="pl-4 pt-2"
              variant='icon'
              icon='copy'
              on:click={baseCopyPosition}
              on:keydown={baseCopyPosition}
            />
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
      label="View SLAM map (3D)"
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
        <div class="w-56 mt-3">
          <p class="mb-1 text-xs text-gray-500 ">
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
                Manual refresh
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
            <v-icon
              name='chevron-down'
              class="pointer-events-none absolute bottom-0 h-[30px] right-0 flex items-center px-2"
            />
          </div>
        </div>

        <v-button
          icon="refresh"
          label="Refresh"
          on:click={refresh3dMap}
          on:keydown={refresh3dMap}
        />
      </div>

      <PCD {pointcloud} />
    {/if}
  </div>
</Collapse>

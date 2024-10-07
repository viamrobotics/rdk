<script lang="ts">
/* eslint-disable require-atomic-updates */

import { moveOnMap } from '@/api/motion';
import { useConnect, useRobotClient } from '@/hooks/robot-client';
import Collapse from '@/lib/components/collapse.svelte';
import Dropzone from '@/lib/components/dropzone.svelte';
import { copyToClipboard } from '@/lib/copy-to-clipboard';
import { rcLogConditionally } from '@/lib/log';
import { filterSubtype } from '@/lib/resource';
import { setAsyncInterval } from '@/lib/schedule';
import { components, services } from '@/stores/resources';
import type { SLAMOverrides } from '@/types/overrides';
import { notify } from '@viamrobotics/prime';
import { SlamMap2D } from '@viamrobotics/prime-blocks';
import {
  Code,
  ConnectError,
  motionApi,
  MotionClient,
  ResourceName,
  slamApi,
  SlamClient,
  Timestamp,
  type Pose,
} from '@viamrobotics/sdk';
import { onMount } from 'svelte';
import * as THREE from 'three';
import type { ValueOf } from 'type-fest';
type MappingMode = ValueOf<typeof slamApi.MappingMode>;

export let name: string;
export let motionResourceNames: ResourceName[];
export let overrides: SLAMOverrides | undefined;

const { robotClient } = useRobotClient();
const motionClient = new MotionClient(
  $robotClient,
  motionResourceNames[0]!.name,
  {
    requestLogger: rcLogConditionally,
  }
);
const slamClient = new SlamClient($robotClient, name, {
  requestLogger: rcLogConditionally,
});

const refreshErrorMessage = 'Error refreshing map. The map shown may be stale.';

let clear2dRefresh: (() => void) | undefined;

let refreshErrorMessage2d: string | undefined;
let refreshErrorMessagePaths: string | undefined;
let executionID: string | undefined;
let refresh2dRate = '5';
let pointcloud: Uint8Array | undefined;
let pose: Pose | undefined;
let show2d = false;
let showAxes = true;
let destination: THREE.Vector2 | undefined;
let labelUnits = 'm';
let hasActiveSession = false;
let sessionId = '';
let mappingSessionEnded = false;
let sessionDuration = 0;
let durationInterval: number | undefined;
let newMapName = '';
let mapNameError = '';
let motionPath: Float32Array | undefined;
let mappingSessionStarted = false;
let isLocalizingMode: boolean | undefined;
let lastReconfigured: Timestamp | undefined;
let mappingMode: MappingMode = slamApi.MappingMode.UNSPECIFIED;

$: pointcloudLoaded = Boolean(pointcloud?.length) && pose !== undefined;
$: moveClicked = Boolean(executionID);
$: unitScale = labelUnits === 'm' ? 1 : 1000;

// get all resources which are bases
$: bases = filterSubtype($components, 'base');
$: slamResourceName = filterSubtype($services, 'slam').find(
  (service) => service.name === name
)!;

// allowMove is only true if we have a base, there exists a destination and there is no in-flight MoveOnMap req
$: allowMove = bases.length === 1 && destination && !moveClicked;

const mappingModeToDisplayText: Record<MappingMode, string> = {
  [slamApi.MappingMode.CREATE_NEW_MAP]: 'create',
  [slamApi.MappingMode.LOCALIZE_ONLY]: 'localize',
  [slamApi.MappingMode.UPDATE_EXISTING_MAP]: 'update',
  [slamApi.MappingMode.UNSPECIFIED]: 'undefined',
} as const;

const deleteDestinationMarker = () => {
  destination = undefined;
};

const startDurationTimer = (start: number) => {
  durationInterval = window.setInterval(() => {
    sessionDuration = Date.now() - start;
  }, 400);
};

const setMappingMode = async () => {
  try {
    const props = await slamClient.getProperties();
    mappingMode = props.mappingMode;
  } catch (error) {
    mappingMode = slamApi.MappingMode.UNSPECIFIED;
    notify.danger('can not get slam properties', error as string);
  }
};

const refresh2d = async () => {
  refreshPaths();
  try {
    let nextPose;
    if (overrides?.isCloudSlam && overrides.getMappingSessionPCD) {
      const { map, pose: poseData } =
        await overrides.getMappingSessionPCD(sessionId);
      nextPose = poseData;
      pointcloud = map;
    } else {
      /*
       * Check if reconfiguration has happened
       * If it has, reset the point cloud, update the reconfigured time and check what mode the new
       * SLAM session is in to know whether or not to update the map.
       */
      const statuses = await $robotClient.getStatus([slamResourceName]);
      const lastReconfiguredStatus = (statuses ?? []).find(
        (status) => status.lastReconfigured !== undefined
      );
      const newLastReconfigured = lastReconfiguredStatus?.lastReconfigured;

      // assuming reconfigures do not happen at the nanosecond scale
      if (
        newLastReconfigured?.seconds !== lastReconfigured?.seconds ||
        isLocalizingMode === undefined
      ) {
        lastReconfigured = newLastReconfigured;

        const props = await slamClient.getProperties();
        isLocalizingMode =
          props.mappingMode === slamApi.MappingMode.LOCALIZE_ONLY;
      }

      /*
       * Update the map and pose if the SLAM session is in mapping/updating mode or the pointcloud
       * has yet to be defined else only update the pose
       */
      if (!isLocalizingMode || pointcloud === undefined) {
        let response;
        // only request the edited map if we are in localizing mode
        [pointcloud, response] = await Promise.all([
          slamClient.getPointCloudMap(isLocalizingMode),
          slamClient.getPosition(),
        ]);
        nextPose = response.pose;
      } else {
        const response = await slamClient.getPosition();
        nextPose = response.pose;
      }
    }

    /*
     * The pose is returned in millimeters, but we need
     * to convert to meters to display on the frontend.
     */
    if (nextPose) {
      nextPose.x /= 1000;
      nextPose.y /= 1000;
      nextPose.z /= 1000;
    }
    pose = nextPose;
    refreshErrorMessage2d = undefined;
  } catch (error) {
    refreshErrorMessage2d =
      error !== null && typeof error === 'object' && 'message' in error
        ? `${refreshErrorMessage} ${(error as { message: string }).message}`
        : `${refreshErrorMessage} ${error as string}`;
  }
};

const refreshPaths = async () => {
  try {
    refreshErrorMessagePaths = undefined;
    const base = bases[0]!;
    const listPlanStatusesResponse = await motionClient.listPlanStatuses(true);
    const baseHasPlan = listPlanStatusesResponse.planStatusesWithIds
      .map((plan) => plan.componentName)
      .find(
        (planComponentName) =>
          planComponentName !== undefined &&
          planComponentName.namespace === base.namespace &&
          planComponentName.subtype === base.subtype &&
          planComponentName.type === base.type &&
          planComponentName.name === base.name
      );

    if (!baseHasPlan) {
      motionPath = undefined;
      executionID = undefined;
      return;
    }

    const getPlanResponse = await motionClient.getPlan(base, true);
    if (
      getPlanResponse.currentPlanWithStatus?.status?.state ===
      motionApi.PlanState.IN_PROGRESS
    ) {
      const pathsInMeters: number[] = [];
      for (const step of getPlanResponse.currentPlanWithStatus.plan?.steps ??
        []) {
        const { pose: stepPose } = step.step[1] ?? {};
        if (stepPose) {
          pathsInMeters.push(stepPose.x / 1000, stepPose.y / 1000);
        }
      }

      motionPath = new Float32Array(pathsInMeters);
      executionID = getPlanResponse.currentPlanWithStatus.plan?.executionId;
      return;
    }
    motionPath = undefined;
    executionID = undefined;
  } catch (error) {
    if (
      error !== null &&
      typeof error === 'object' &&
      'code' in error &&
      'message' in error
    ) {
      // This is the error code when the component has not been used in a plan yet.
      if (error.code !== Code.Unknown) {
        refreshErrorMessagePaths = `${refreshErrorMessage} ${
          (error as { message: string }).message
        }`;
      }
    } else {
      refreshErrorMessagePaths = `${refreshErrorMessage} ${error as string}`;
    }
    motionPath = undefined;
    executionID = undefined;
  }
};

const updateSLAM2dRefreshFrequency = () => {
  clear2dRefresh?.();
  refresh2d();

  refreshErrorMessage2d = undefined;
  refreshErrorMessagePaths = undefined;

  if (refresh2dRate !== 'manual') {
    clear2dRefresh = setAsyncInterval(
      refresh2d,
      Number.parseFloat(refresh2dRate) * 1000
    );
  }
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

const handle2dRenderClick = (event: CustomEvent<THREE.Vector3>) => {
  if (!overrides?.isCloudSlam) {
    const roundedX = Number.parseFloat(event.detail.x.toFixed(5));
    const roundedY = Number.parseFloat(event.detail.y.toFixed(5));
    destination = new THREE.Vector2(roundedX, roundedY);
  }
};

const handleUpdateDestX = (event: CustomEvent<{ value: string }>) => {
  if (!overrides?.isCloudSlam) {
    destination ??= new THREE.Vector2();
    destination.x =
      Number.parseFloat(event.detail.value) * (labelUnits === 'mm' ? 0.001 : 1);
  }
};

const handleUpdateDestY = (event: CustomEvent<{ value: string }>) => {
  if (!overrides?.isCloudSlam) {
    destination ??= new THREE.Vector2();
    destination.y =
      Number.parseFloat(event.detail.value) * (labelUnits === 'mm' ? 0.001 : 1);
  }
};

const baseCopyPosition = () => {
  copyToClipboard(
    JSON.stringify({
      x: pose?.x,
      y: pose?.y,
      z: pose?.z,
      ox: 0,
      oy: 0,
      oz: 1,
      th: pose?.theta,
    })
  );
};

const toggleAxes = () => {
  showAxes = !showAxes;
};

const handleMoveClick = async () => {
  try {
    const base = bases[0]!;
    await moveOnMap(
      $robotClient,
      slamResourceName,
      base,
      destination!.x,
      destination!.y
    );
    await refreshPaths();
  } catch (error) {
    notify.danger((error as ConnectError).message);
  }
};

const handleStopMoveClick = async () => {
  try {
    const base = bases[0]!;
    await motionClient.stopPlan(base);
    await refreshPaths();
  } catch (error) {
    notify.danger((error as ConnectError).message);
  }
};

const toggleExpand = (event: CustomEvent<{ open: boolean }>) => {
  const { open } = event.detail;

  if (open) {
    toggle2dExpand();
  } else {
    clear2dRefresh?.();
  }
};

const startMappingIntervals = (start: number) => {
  updateSLAM2dRefreshFrequency();
  startDurationTimer(start);
};

const handleStartMapping = async () => {
  if (overrides) {
    // if input error do not start mapping
    if (mapNameError) {
      return;
    }

    // error may not be present if user has not yet typed in input
    const mapName = overrides.mappingDetails.name ?? newMapName;
    if (!mapName) {
      mapNameError = 'Please enter a name for this map';
      return;
    }

    try {
      hasActiveSession = true;
      if (!mappingSessionStarted) {
        // Get SensorInfo list
        const props = await slamClient.getProperties();

        mappingSessionStarted = true;
        sessionId = await overrides.startMappingSession(
          mapName,
          props.sensorInfo
        );
        startMappingIntervals(Date.now());
      }
    } catch {
      hasActiveSession = false;
      mappingSessionStarted = false;
      sessionDuration = 0;
      clearInterval(durationInterval);
    }
  }
};

const clearRefresh = () => {
  clear2dRefresh?.();
};

const handleEndMapping = () => {
  hasActiveSession = false;
  mappingSessionEnded = true;
  clearRefresh();
  clearInterval(durationInterval);
  overrides?.endMappingSession(sessionId);
};

const formatDisplayTime = (time: number): string => {
  return time < 10 ? `0${time}` : `${time}`;
};

const formatDuration = (milliseconds: number) => {
  let seconds = Math.floor(milliseconds / 1000);
  const hours = Math.floor(seconds / 3600);
  seconds %= 3600;
  const minutes = Math.floor(seconds / 60);
  seconds %= 60;

  return `${formatDisplayTime(hours)}:${formatDisplayTime(
    minutes
  )}:${formatDisplayTime(seconds)}`;
};

const handleViewMap = () => {
  overrides?.viewMap(sessionId);
};

const handleMapNameChange = (event: CustomEvent<{ value: string }>) => {
  newMapName = event.detail.value;
  mapNameError = overrides?.validateMapName(newMapName) ?? '';
};

const handleDrop = (event: CustomEvent<string>) =>
  (motionPath = new Float32Array(
    event.detail.split('\n').flatMap((str) => {
      const [xStr, yStr] = str.split(',');
      if (xStr && yStr) {
        const x = Number.parseFloat(xStr) / 1000;
        const y = Number.parseFloat(yStr) / 1000;
        if (!Number.isNaN(x) && !Number.isNaN(y)) {
          return [x, y];
        }
        return [];
      }
      return [];
    })
  ));

onMount(async () => {
  if (overrides?.isCloudSlam) {
    await setMappingMode();

    const activeSession = await overrides.getActiveMappingSession();

    if (activeSession) {
      hasActiveSession = true;
      sessionId = activeSession.id;
      const startMilliseconds =
        Number(activeSession.timeCloudRunJobStarted?.seconds ?? 0n) * 1000;
      startMappingIntervals(startMilliseconds);
    }
  }
});

useConnect(() => {
  updateSLAM2dRefreshFrequency();

  return () => {
    clearRefresh();
    clearInterval(durationInterval);
  };
});
</script>

<Collapse
  title={name}
  on:toggle={toggleExpand}
>
  <v-breadcrumbs
    slot="title"
    crumbs="slam"
  />
  <v-button
    slot="header"
    variant="danger"
    icon="stop-circle-outline"
    disabled={moveClicked ? 'false' : 'true'}
    label="Stop"
    on:click={handleStopMoveClick}
    on:keydown={handleStopMoveClick}
  />
  <div
    class="flex flex-wrap gap-4 border border-t-0 border-medium sm:flex-nowrap"
  >
    <div class="flex min-w-fit flex-col gap-4 p-4 pr-0">
      <div class="flex flex-col gap-6 pb-4">
        {#if overrides?.isCloudSlam && overrides.mappingDetails}
          <header class="flex flex-col justify-between gap-3 text-xs">
            {#if mappingMode !== slamApi.MappingMode.UNSPECIFIED}
              <div class="flex flex-col">
                <span class="font-bold text-gray-800">Mapping mode</span>
                <span class="capitalize text-subtle-2"
                  >{mappingModeToDisplayText[mappingMode]}</span
                >
              </div>
            {/if}
            <div class="flex gap-8">
              {#if overrides.mappingDetails.name}
                <div class="flex flex-col">
                  <span class="font-bold text-gray-800">Map name</span>
                  <span class="text-subtle-2"
                    >{overrides.mappingDetails.name}</span
                  >
                </div>
              {/if}
              {#if overrides.mappingDetails.version}
                <div class="flex flex-col">
                  <span class="font-bold text-gray-800">Version</span>
                  <span class="text-subtle-2"
                    >{overrides.mappingDetails.version}</span
                  >
                </div>
              {/if}
            </div>
            {#if !overrides.mappingDetails.name}
              <v-input
                label="Map name"
                value={newMapName}
                state={mapNameError ? 'error' : ''}
                message={mapNameError}
                on:input={handleMapNameChange}
              />
            {/if}
          </header>
        {/if}
        <div class="flex min-w-fit grow items-end gap-2">
          {#if overrides && overrides.isCloudSlam}
            <div class="flex grow">
              {#if hasActiveSession || mappingSessionEnded}
                <div class="flex w-full items-center justify-between">
                  <div class="flex items-center gap-1 text-xs">
                    <div
                      class="rounded-full border-success-border bg-success-bg px-2 py-1 text-success-fg"
                      class:border-medium={mappingSessionEnded}
                      class:bg-3={mappingSessionEnded}
                      class:text-default={mappingSessionEnded}
                    >
                      <span>{mappingSessionEnded ? 'Saved' : 'Running'}</span>
                    </div>
                    <span class="text-subtle-2"
                      >{formatDuration(sessionDuration)}</span
                    >
                  </div>
                  {#if hasActiveSession}
                    <v-button
                      label="End session"
                      on:click={handleEndMapping}
                    />
                  {/if}
                  {#if mappingSessionEnded}
                    <v-button
                      label="View map"
                      icon="open-in-new"
                      on:click={handleViewMap}
                    />
                  {/if}
                </div>
              {:else}
                <v-button
                  label="Start session"
                  on:click={handleStartMapping}
                  variant="inverse-primary"
                />
              {/if}
            </div>
          {:else}
            <div>
              <div class="mb-1 flex gap-2">
                <p class="font-bold text-gray-800">End position</p>
                <button
                  class="text-xs hover:underline"
                  on:click={() =>
                    (labelUnits = labelUnits === 'mm' ? 'm' : 'mm')}
                >
                  ({labelUnits})
                </button>
              </div>
              <div class="flex flex-row items-end gap-2 pb-2">
                <v-input
                  type="string"
                  label="x"
                  incrementor="slider"
                  value={destination && !Number.isNaN(destination.x)
                    ? destination.x * unitScale
                    : ''}
                  step={labelUnits === 'mm' ? '10' : '1'}
                  on:input={handleUpdateDestX}
                />
                <v-input
                  type="string"
                  label="y"
                  incrementor="slider"
                  value={destination && !Number.isNaN(destination.y)
                    ? destination.y * unitScale
                    : ''}
                  step={labelUnits === 'mm' ? '10' : '1'}
                  on:input={handleUpdateDestY}
                />
                <v-button
                  class="fill-white pt-1"
                  label="Move"
                  variant="success"
                  icon="play-circle-outline"
                  disabled={allowMove ? 'false' : 'true'}
                  on:click={handleMoveClick}
                  on:keydown={handleMoveClick}
                />
                <v-button
                  variant="icon"
                  icon="trash-can-outline"
                  on:click={deleteDestinationMarker}
                  on:keydown={deleteDestinationMarker}
                />
              </div>
            </div>
          {/if}
        </div>
        <div class="flex w-[70%] items-end gap-2">
          <div class="grow">
            <p class="mb-1 text-xs text-gray-500">Refresh frequency</p>
            <select
              bind:value={refresh2dRate}
              class="
                m-0 w-full min-w-[200px] appearance-none border border-solid border-medium bg-white bg-clip-padding
                px-3 py-1.5 text-xs font-normal text-default focus:outline-none
              "
              aria-label="Default select example"
              on:change={updateSLAM2dRefreshFrequency}
            >
              <option value="manual"> Manual refresh </option>
              <option value="30"> Every 30 seconds </option>
              <option value="10"> Every 10 seconds </option>
              <option value="5"> Every 5 seconds </option>
              {#if !overrides?.isCloudSlam}
                <option value="1"> Every second </option>
              {/if}
            </select>
            <v-icon
              name="chevron-down"
              class="pointer-events-none absolute bottom-0 right-0 flex h-[30px] items-center px-2"
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
      <v-switch
        class="pt-2"
        label="Show grid"
        value={showAxes ? 'on' : 'off'}
        on:input={toggleAxes}
      />
    </div>
    <div class="gap-4x border-border-1 w-full justify-start sm:border-l">
      {#if refreshErrorMessage2d && show2d}
        <div class="border-l-4 border-red-500 bg-gray-100 px-4 py-3">
          {refreshErrorMessage2d}
        </div>
      {/if}
      {#if refreshErrorMessagePaths && show2d}
        <div class="border-l-4 border-red-500 bg-gray-100 px-4 py-3">
          {refreshErrorMessagePaths}
        </div>
      {/if}

      {#if show2d}
        {#if pointcloudLoaded}
          <div>
            <div class="flex flex-row border-b border-b-light py-2 pl-5">
              <div class="flex flex-col gap-0.5">
                <div class="flex gap-2">
                  <p class="text-xs">Current position</p>
                  <button
                    class="text-xs hover:underline"
                    on:click={() =>
                      (labelUnits = labelUnits === 'mm' ? 'm' : 'mm')}
                  >
                    ({labelUnits})
                  </button>
                </div>

                {#if pose}
                  <div class="flex flex-row items-center">
                    <p class="items-end pr-1.5 text-xs text-gray-500">x</p>
                    <p>{(pose.x * unitScale).toFixed(1)}</p>

                    <p class="pl-6 pr-1.5 text-xs text-gray-500">y</p>
                    <p>{(pose.y * unitScale).toFixed(1)}</p>

                    <p class="pl-6 pr-1.5 text-xs text-gray-500">z</p>
                    <p>{(pose.z * unitScale).toFixed(1)}</p>
                  </div>
                {/if}
              </div>
              <div class="flex flex-col gap-0.5 pl-10">
                <p class="text-xs">Current orientation</p>

                {#if pose}
                  <div class="flex flex-row items-center">
                    <p class="pr-1.5 text-xs text-gray-500">o<sub>x</sub></p>
                    <p>{pose.oX.toFixed(1)}</p>

                    <p class="pl-6 pr-1.5 text-xs text-gray-500">
                      o<sub>y</sub>
                    </p>
                    <p>{pose.oY.toFixed(1)}</p>

                    <p class="pl-6 pr-1.5 text-xs text-gray-500">
                      o<sub>z</sub>
                    </p>
                    <p>{pose.oZ.toFixed(1)}</p>

                    <p class="pl-6 pr-1.5 text-xs text-gray-500">&theta;</p>
                    <p>{pose.theta.toFixed(1)}</p>
                  </div>
                {/if}
              </div>

              <v-button
                tooltip="Copy pose to clipboard"
                class="pl-4 pt-2"
                variant="icon"
                icon="content-copy"
                on:click={baseCopyPosition}
                on:keydown={baseCopyPosition}
              />
            </div>

            <Dropzone on:drop={handleDrop}>
              <div class="relative h-[400px] w-full">
                <SlamMap2D
                  {pointcloud}
                  {destination}
                  {motionPath}
                  basePose={pose
                    ? {
                        x: pose.x,
                        y: pose.y,
                        theta: pose.theta,
                      }
                    : undefined}
                  helpers={showAxes}
                  on:click={handle2dRenderClick}
                />
              </div>
            </Dropzone>
          </div>
        {:else if overrides?.isCloudSlam && sessionId}
          <div
            class="flex h-full w-full flex-col items-center justify-center gap-4"
          >
            <div class="animate-[spin_3s_linear_infinite]">
              <v-icon
                name="cog"
                size="4xl"
              />
            </div>
            <div class="flex flex-col items-center text-xs">
              {#if mappingSessionStarted}
                <span>Starting slam session in the cloud.</span>
                <span>This typically takes about 2 minutes.</span>
              {:else}
                <span>Loading point cloud...</span>
              {/if}
            </div>
          </div>
        {/if}
      {/if}
    </div>
  </div>
</Collapse>

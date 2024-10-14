<script lang="ts">
import {
  getAccuracy,
  getAngularVelocity,
  getCompassHeading,
  getLinearAcceleration,
  getLinearVelocity,
  getOrientation,
  getPosition,
  getProperties,
} from '@/api/movement-sensor';
import { useConnect, useRobotClient } from '@/hooks/robot-client';
import Collapse from '@/lib/components/collapse.svelte';
import { displayError } from '@/lib/error';
import { setAsyncInterval } from '@/lib/schedule';
import { Icon, Tooltip } from '@viamrobotics/prime-core';
import {
  ConnectError,
  movementSensorApi as movementsensorApi,
  type commonApi,
} from '@viamrobotics/sdk';

export let name: string;

const { robotClient } = useRobotClient();

let orientation: commonApi.Orientation | undefined;
let angularVelocity: commonApi.Vector3 | undefined;
let linearVelocity: commonApi.Vector3 | undefined;
let linearAcceleration: commonApi.Vector3 | undefined;
let compassHeading: number | undefined;
let coordinate: commonApi.GeoPoint | undefined;
let altitudeM: number | undefined;
let properties: movementsensorApi.GetPropertiesResponse | undefined;
let accuracy: movementsensorApi.GetAccuracyResponse | undefined;

let expanded = false;

const refresh = async () => {
  if (!expanded) {
    return;
  }

  try {
    properties = await getProperties($robotClient, name);

    if (!properties) {
      return;
    }

    const results = await Promise.all([
      properties.orientationSupported
        ? getOrientation($robotClient, name)
        : undefined,
      properties.angularVelocitySupported
        ? getAngularVelocity($robotClient, name)
        : undefined,
      properties.linearAccelerationSupported
        ? getLinearAcceleration($robotClient, name)
        : undefined,
      properties.linearVelocitySupported
        ? getLinearVelocity($robotClient, name)
        : undefined,
      properties.compassHeadingSupported
        ? getCompassHeading($robotClient, name)
        : undefined,
      properties.positionSupported
        ? getPosition($robotClient, name)
        : undefined,
    ] as const);

    orientation = results[0];
    angularVelocity = results[1];
    linearAcceleration = results[2];
    linearVelocity = results[3];
    compassHeading = results[4];
    coordinate = results[5]?.coordinate;
    altitudeM = results[5]?.altitudeM;
  } catch (error) {
    displayError(error as ConnectError);
  }

  /*
   * GetAccuracy is not implemented in older versions of
   * RDK. This results in a "not implemented" or "nil
   * pointer" error on each polling call, which we don't
   * want to spam the frontend with.
   *
   * TODO(RSDK-6637): Guard with accuracySupported
   *                    and displayError for actual errors.
   */
  try {
    accuracy = await getAccuracy($robotClient, name);
  } catch (error: unknown) {
    // eslint-disable-next-line no-console
    console.error(
      `Unhandled GetAccuracy error: ${(error as ConnectError).message}`
    );
  }
};

const handleToggle = (event: CustomEvent<{ open: boolean }>) => {
  expanded = event.detail.open;
};

useConnect(() => {
  refresh();
  const clearInterval = setAsyncInterval(refresh, 500);
  return () => clearInterval();
});
</script>

<Collapse
  title={name}
  on:toggle={handleToggle}
>
  <v-breadcrumbs
    slot="title"
    crumbs="movement_sensor"
  />
  <div class="flex flex-wrap gap-4 border border-t-0 border-medium p-4 text-sm">
    {#if properties?.positionSupported}
      <div class="overflow-auto">
        <h3 class="mb-1">Position</h3>
        <table class="w-full border border-t-0 border-medium p-4">
          <tr>
            <th class="border border-medium p-2"> Latitude </th>
            <td class="border border-medium p-2">
              {coordinate?.latitude.toFixed(6)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> Longitude </th>
            <td class="border border-medium p-2">
              {coordinate?.longitude.toFixed(6)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> Altitude (m) </th>
            <td class="border border-medium p-2">
              {altitudeM?.toFixed(2)}
            </td>
          </tr>
          {#if accuracy?.positionNmeaGgaFix}
            <tr>
              <th class="border border-medium p-2"> NMEA Fix Quality </th>
              <td class="border border-medium p-2">
                {accuracy.positionNmeaGgaFix}
                {#if accuracy.positionNmeaGgaFix === 1 || accuracy.positionNmeaGgaFix === 2}
                  <Tooltip let:tooltipID>
                    <p aria-describedby={tooltipID}>
                      <Icon name="information" />
                    </p>
                    <p slot="description">expect 1m-5m accuracy</p>
                  </Tooltip>
                {/if}
                {#if accuracy.positionNmeaGgaFix === 4 || accuracy.positionNmeaGgaFix === 5}
                  <Tooltip let:tooltipID>
                    <p aria-describedby={tooltipID}>
                      <Icon name="information" />
                    </p>
                    <p slot="description">expect 2cm-50cm accuracy</p>
                  </Tooltip>
                {/if}
              </td>
            </tr>
          {/if}

          {#if accuracy?.positionHdop && accuracy.positionVdop}
            <tr>
              <th class="border border-medium p-2"> HDOP </th>
              <td class="border border-medium p-2">
                {accuracy.positionHdop.toFixed(2)}
              </td>
            </tr>
            <tr>
              <th class="border border-medium p-2"> VDOP </th>
              <td class="border border-medium p-2">
                {accuracy.positionVdop.toFixed(2)}
              </td>
            </tr>
          {/if}
        </table>
        <a
          class="text-[#045681] underline"
          href={`https://www.google.com/maps/search/${coordinate?.latitude},${coordinate?.longitude}`}
        >
          google maps
        </a>
      </div>
    {/if}

    {#if properties?.orientationSupported}
      <div class="overflow-auto">
        <h3 class="mb-1">Orientation (degrees)</h3>
        <table class="w-full border border-t-0 border-medium p-4">
          <tr>
            <th class="border border-medium p-2"> OX </th>
            <td class="border border-medium p-2">
              {orientation?.oX.toFixed(2)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> OY </th>
            <td class="border border-medium p-2">
              {orientation?.oY.toFixed(2)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> OZ </th>
            <td class="border border-medium p-2">
              {orientation?.oZ.toFixed(2)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> Theta </th>
            <td class="border border-medium p-2">
              {orientation?.theta.toFixed(2)}
            </td>
          </tr>
        </table>
      </div>
    {/if}

    {#if properties?.angularVelocitySupported}
      <div class="overflow-auto">
        <h3 class="mb-1">Angular velocity (degrees/second)</h3>
        <table class="w-full border border-t-0 border-medium p-4">
          <tr>
            <th class="border border-medium p-2"> X </th>
            <td class="border border-medium p-2">
              {angularVelocity?.x.toFixed(2)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> Y </th>
            <td class="border border-medium p-2">
              {angularVelocity?.y.toFixed(2)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> Z </th>
            <td class="border border-medium p-2">
              {angularVelocity?.z.toFixed(2)}
            </td>
          </tr>
        </table>
      </div>
    {/if}

    {#if properties?.linearVelocitySupported}
      <div class="overflow-auto">
        <h3 class="mb-1">Linear velocity (m/s)</h3>
        <table class="w-full border border-t-0 border-medium p-4">
          <tr>
            <th class="border border-medium p-2"> X </th>
            <td class="border border-medium p-2">
              {linearVelocity?.x.toFixed(2)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> Y </th>
            <td class="border border-medium p-2">
              {linearVelocity?.y.toFixed(2)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> Z </th>
            <td class="border border-medium p-2">
              {linearVelocity?.z.toFixed(2)}
            </td>
          </tr>
        </table>
      </div>
    {/if}

    {#if properties?.linearAccelerationSupported}
      <div class="overflow-auto">
        <h3 class="mb-1">Linear acceleration (m/second^2)</h3>
        <table class="w-full border border-t-0 border-medium p-4">
          <tr>
            <th class="border border-medium p-2"> X </th>
            <td class="border border-medium p-2">
              {linearAcceleration?.x.toFixed(2)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> Y </th>
            <td class="border border-medium p-2">
              {linearAcceleration?.y.toFixed(2)}
            </td>
          </tr>
          <tr>
            <th class="border border-medium p-2"> Z </th>
            <td class="border border-medium p-2">
              {linearAcceleration?.z.toFixed(2)}
            </td>
          </tr>
        </table>
      </div>
    {/if}

    {#if properties?.compassHeadingSupported}
      <div class="overflow-auto">
        <h3 class="mb-1">Compass heading</h3>
        <table class="w-full border border-t-0 border-medium p-4">
          <tr>
            <th class="border border-medium p-2"> Compass </th>
            <td class="border border-medium p-2">
              {compassHeading?.toFixed(2)}
            </td>
          </tr>
          {#if accuracy?.compassDegreesError}
            <tr>
              <th class="border border-medium p-2"> Compass Degrees Error </th>
              <td class="border border-medium p-2">
                {accuracy.compassDegreesError.toFixed(2)}
              </td>
            </tr>
          {/if}
        </table>
      </div>
    {/if}

    {#if (accuracy?.accuracy.length ?? 0) > 0}
      <div class="overflow-auto">
        <h3 class="mb-1">Accuracy Map</h3>
        <table class="w-full border border-t-0 border-medium p-4">
          {#each Object.entries(accuracy?.accuracy ?? []) as pair (pair[0])}
            <tr>
              <td class="border border-medium p-2">
                {pair[0]}
              </td>

              <td class="border border-medium p-2">
                {pair[1]}
              </td>
            </tr>
          {/each}
        </table>
      </div>
    {/if}
  </div>
</Collapse>

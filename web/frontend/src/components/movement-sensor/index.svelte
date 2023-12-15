<script lang="ts">

import { movementSensorApi as movementsensorApi, type ServiceError, type commonApi } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { setAsyncInterval } from '@/lib/schedule';
import {
  getProperties,
  getOrientation,
  getAngularVelocity,
  getLinearAcceleration,
  getLinearVelocity,
  getCompassHeading,
  getPosition,
} from '@/api/movement-sensor';
import { useRobotClient, useConnect } from '@/hooks/robot-client';

export let name: string;

const { robotClient } = useRobotClient();

let orientation: commonApi.Orientation.AsObject | undefined;
let angularVelocity: commonApi.Vector3.AsObject | undefined;
let linearVelocity: commonApi.Vector3.AsObject | undefined;
let linearAcceleration: commonApi.Vector3.AsObject | undefined;
let compassHeading: number | undefined;
let coordinate: commonApi.GeoPoint.AsObject | undefined;
let altitudeM: number | undefined;
let properties: movementsensorApi.GetPropertiesResponse.AsObject | undefined;

const refresh = async () => {
  properties = await getProperties($robotClient, name);

  if (!properties) {
    return;
  }

  try {
    const results = await Promise.all([
      properties.orientationSupported ? getOrientation($robotClient, name) : undefined,
      properties.angularVelocitySupported ? getAngularVelocity($robotClient, name) : undefined,
      properties.linearAccelerationSupported ? getLinearAcceleration($robotClient, name) : undefined,
      properties.linearVelocitySupported ? getLinearVelocity($robotClient, name) : undefined,
      properties.compassHeadingSupported ? getCompassHeading($robotClient, name) : undefined,
      properties.positionSupported ? getPosition($robotClient, name) : undefined,
    ] as const);

    orientation = results[0];
    angularVelocity = results[1];
    linearAcceleration = results[2];
    linearVelocity = results[3];
    compassHeading = results[4];
    coordinate = results[5]?.coordinate;
    altitudeM = results[5]?.altitudeM;
  } catch (error) {
    displayError(error as ServiceError);
  }
};

useConnect(() => {
  refresh();
  const clearInterval = setAsyncInterval(refresh, 500);
  return () => clearInterval();
})

</script>

<div class="flex flex-wrap gap-4 text-sm border border-t-0 border-medium p-4">
  {#if properties?.positionSupported}
    <div class="overflow-auto">
      <h3 class="mb-1">Position</h3>
      <table class="w-full border border-t-0 border-medium p-4">
        <tr>
          <th class="border border-medium p-2">
            Latitude
          </th>
          <td class="border border-medium p-2">
            {coordinate?.latitude.toFixed(6)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            Longitude
          </th>
          <td class="border border-medium p-2">
            {coordinate?.longitude.toFixed(6)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            Altitude (m)
          </th>
          <td class="border border-medium p-2">
            {altitudeM?.toFixed(2)}
          </td>
        </tr>
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
      <h3 class="mb-1">
        Orientation (degrees)
      </h3>
      <table class="w-full border border-t-0 border-medium p-4">
        <tr>
          <th class="border border-medium p-2">
            OX
          </th>
          <td class="border border-medium p-2">
            {orientation?.oX.toFixed(2)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            OY
          </th>
          <td class="border border-medium p-2">
            {orientation?.oY.toFixed(2)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            OZ
          </th>
          <td class="border border-medium p-2">
            {orientation?.oZ.toFixed(2)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            Theta
          </th>
          <td class="border border-medium p-2">
            {orientation?.theta.toFixed(2)}
          </td>
        </tr>
      </table>
    </div>
  {/if}

  {#if properties?.angularVelocitySupported}
    <div class="overflow-auto">
      <h3 class="mb-1">
        Angular velocity (degrees/second)
      </h3>
      <table class="w-full border border-t-0 border-medium p-4">
        <tr>
          <th class="border border-medium p-2">
            X
          </th>
          <td class="border border-medium p-2">
            {angularVelocity?.x.toFixed(2)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            Y
          </th>
          <td class="border border-medium p-2">
            {angularVelocity?.y.toFixed(2)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            Z
          </th>
          <td class="border border-medium p-2">
            {angularVelocity?.z.toFixed(2)}
          </td>
        </tr>
      </table>
    </div>
  {/if}

  {#if properties?.linearVelocitySupported}
    <div class="overflow-auto">
      <h3 class="mb-1">
        Linear velocity (m/s)
      </h3>
      <table class="w-full border border-t-0 border-medium p-4">
        <tr>
          <th class="border border-medium p-2">
            X
          </th>
          <td class="border border-medium p-2">
            {linearVelocity?.x.toFixed(2)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            Y
          </th>
          <td class="border border-medium p-2">
            {linearVelocity?.y.toFixed(2)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            Z
          </th>
          <td class="border border-medium p-2">
            {linearVelocity?.z.toFixed(2)}
          </td>
        </tr>
      </table>
    </div>
  {/if}

  {#if properties?.linearAccelerationSupported}
    <div class="overflow-auto">
      <h3 class="mb-1">
        Linear acceleration (m/second^2)
      </h3>
      <table class="w-full border border-t-0 border-medium p-4">
        <tr>
          <th class="border border-medium p-2">
            X
          </th>
          <td class="border border-medium p-2">
            {linearAcceleration?.x.toFixed(2)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            Y
          </th>
          <td class="border border-medium p-2">
            {linearAcceleration?.y.toFixed(2)}
          </td>
        </tr>
        <tr>
          <th class="border border-medium p-2">
            Z
          </th>
          <td class="border border-medium p-2">
            {linearAcceleration?.z.toFixed(2)}
          </td>
        </tr>
      </table>
    </div>
  {/if}

  {#if properties?.compassHeadingSupported}
    <div class="overflow-auto">
      <h3 class="mb-1">
        Compass heading
      </h3>
      <table class="w-full border border-t-0 border-medium p-4">
        <tr>
          <th class="border border-medium p-2">
            Compass
          </th>
          <td class="border border-medium p-2">
            {compassHeading?.toFixed(2)}
          </td>
        </tr>
      </table>
    </div>
  {/if}
</div>

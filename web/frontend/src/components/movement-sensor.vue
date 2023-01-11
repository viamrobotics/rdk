<script setup lang="ts">

import { onMounted, onUnmounted } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Client, movementSensorApi as movementsensorApi } from '@viamrobotics/sdk';
import type{ commonApi } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';

interface Props {
  name: string
  client: Client
}

const props = defineProps<Props>();

let orientation = $ref<commonApi.Orientation.AsObject | undefined>();
let angularVelocity = $ref<commonApi.Vector3.AsObject | undefined>();
let linearVelocity = $ref<commonApi.Vector3.AsObject | undefined>();
let linearAcceleration = $ref<commonApi.Vector3.AsObject | undefined>();
let compassHeading = $ref<number | undefined>();
let coordinate = $ref<commonApi.GeoPoint.AsObject | undefined>();
let altitudeMm = $ref<number | undefined>();
let properties = $ref<movementsensorApi.GetPropertiesResponse.AsObject | undefined>();

let refreshId = -1;

const refresh = async () => {
  properties = await new Promise((resolve) => {
    const req = new movementsensorApi.GetPropertiesRequest();
    req.setName(props.name);

    rcLogConditionally(req);
    props.client.movementSensorService.getProperties(req, new grpc.Metadata(), (err, resp) => {
      if (err) {
        return displayError(err);
      }

      resolve(resp!.toObject());
    });
  });

  if (properties?.orientationSupported) {
    const req = new movementsensorApi.GetOrientationRequest();
    req.setName(props.name);

    rcLogConditionally(req);
    props.client.movementSensorService.getOrientation(req, new grpc.Metadata(), (err, resp) => {
      if (err) {
        return displayError(err);
      }

      orientation = resp!.toObject().orientation;
    });
  }

  if (properties?.angularVelocitySupported) {
    const req = new movementsensorApi.GetAngularVelocityRequest();
    req.setName(props.name);

    rcLogConditionally(req);
    props.client.movementSensorService.getAngularVelocity(req, new grpc.Metadata(), (err, resp) => {
      if (err) {
        return displayError(err);
      }

      angularVelocity = resp!.toObject().angularVelocity;
    });
  }

  if (properties?.linearAccelerationSupported) {
    const req = new movementsensorApi.GetLinearAccelerationRequest();
    req.setName(props.name);

    rcLogConditionally(req);
    props.client.movementSensorService.getLinearAcceleration(req, new grpc.Metadata(), (err, resp) => {
      if (err) {
        return displayError(err);
      }

      linearAcceleration = resp!.toObject().linearAcceleration;
    });
  }

  if (properties?.linearVelocitySupported) {
    const req = new movementsensorApi.GetLinearVelocityRequest();
    req.setName(props.name);

    rcLogConditionally(req);
    props.client.movementSensorService.getLinearVelocity(req, new grpc.Metadata(), (err, resp) => {
      if (err) {
        return displayError(err);
      }

      linearVelocity = resp!.toObject().linearVelocity;
    });
  }

  if (properties?.compassHeadingSupported) {
    const req = new movementsensorApi.GetCompassHeadingRequest();
    req.setName(props.name);

    rcLogConditionally(req);
    props.client.movementSensorService.getCompassHeading(req, new grpc.Metadata(), (err, resp) => {
      if (err) {
        return displayError(err);
      }

      compassHeading = resp!.toObject().value;
    });
  }

  if (properties?.positionSupported) {
    const req = new movementsensorApi.GetPositionRequest();
    req.setName(props.name);

    rcLogConditionally(req);
    props.client.movementSensorService.getPosition(req, new grpc.Metadata(), (err, resp) => {
      if (err) {
        return displayError(err);
      }

      const temp = resp!.toObject();
      coordinate = temp.coordinate;
      altitudeMm = temp.altitudeMm;
    });
  }

  refreshId = window.setTimeout(refresh, 500);
};

onMounted(() => {
  refreshId = window.setTimeout(refresh, 500);
});

onUnmounted(() => {
  clearTimeout(refreshId);
});

</script>

<template>
  <v-collapse
    :title="name"
    class="movement"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="movement_sensor"
    />
    <div class="flex flex-wrap gap-4 border border-t-0 border-black p-4">
      <template v-if="properties">
        <div
          v-if="properties.positionSupported"
          class="overflow-auto"
        >
          <h3 class="mb-1">
            Position
          </h3>
          <table class="w-full border border-t-0 border-black p-4">
            <tr>
              <th class="border border-black p-2">
                Latitude
              </th>
              <td class="border border-black p-2">
                {{ coordinate?.latitude.toFixed(6) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Longitude
              </th>
              <td class="border border-black p-2">
                {{ coordinate?.longitude.toFixed(6) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Altitide
              </th>
              <td class="border border-black p-2">
                {{ altitudeMm?.toFixed(2) }}
              </td>
            </tr>
          </table>
          <a
            class="underline text-[#045681]"
            :href="`https://www.google.com/maps/search/${coordinate?.latitude},${coordinate?.longitude}`"
          >
            google maps
          </a>
        </div>

        <div
          v-if="properties.orientationSupported"
          class="overflow-auto"
        >
          <h3 class="mb-1">
            Orientation (degrees)
          </h3>
          <table class="w-full border border-t-0 border-black p-4">
            <tr>
              <th class="border border-black p-2">
                OX
              </th>
              <td class="border border-black p-2">
                {{ orientation?.oX.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                OY
              </th>
              <td class="border border-black p-2">
                {{ orientation?.oY.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                OZ
              </th>
              <td class="border border-black p-2">
                {{ orientation?.oZ.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Theta
              </th>
              <td class="border border-black p-2">
                {{ orientation?.theta.toFixed(2) }}
              </td>
            </tr>
          </table>
        </div>

        <div
          v-if="properties.angularVelocitySupported"
          class="overflow-auto"
        >
          <h3 class="mb-1">
            Angular Velocity (degrees/second)
          </h3>
          <table class="w-full border border-t-0 border-black p-4">
            <tr>
              <th class="border border-black p-2">
                X
              </th>
              <td class="border border-black p-2">
                {{ angularVelocity?.x.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Y
              </th>
              <td class="border border-black p-2">
                {{ angularVelocity?.y.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Z
              </th>
              <td class="border border-black p-2">
                {{ angularVelocity?.z.toFixed(2) }}
              </td>
            </tr>
          </table>
        </div>

        <div
          v-if="properties.linearVelocitySupported"
          class="overflow-auto"
        >
          <h3 class="mb-1">
            Linear Velocity
          </h3>
          <table class="w-full border border-t-0 border-black p-4">
            <tr>
              <th class="border border-black p-2">
                X
              </th>
              <td class="border border-black p-2">
                {{ linearVelocity?.x.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Y
              </th>
              <td class="border border-black p-2">
                {{ linearVelocity?.y.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Z
              </th>
              <td class="border border-black p-2">
                {{ linearVelocity?.z.toFixed(2) }}
              </td>
            </tr>
          </table>
        </div>

        <div
          v-if="properties.linearAccelerationSupported"
          class="overflow-auto"
        >
          <h3 class="mb-1">
            Linear Acceleration (mm/second^2)
          </h3>
          <table class="w-full border border-t-0 border-black p-4">
            <tr>
              <th class="border border-black p-2">
                X
              </th>
              <td class="border border-black p-2">
                {{ linearAcceleration?.x.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Y
              </th>
              <td class="border border-black p-2">
                {{ linearAcceleration?.y.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Z
              </th>
              <td class="border border-black p-2">
                {{ linearAcceleration?.z.toFixed(2) }}
              </td>
            </tr>
          </table>
        </div>

        <div
          v-if="properties.compassHeadingSupported"
          class="overflow-auto"
        >
          <h3 class="mb-1">
            Compass Heading
          </h3>
          <table class="w-full border border-t-0 border-black p-4">
            <tr>
              <th class="border border-black p-2">
                Compass
              </th>
              <td class="border border-black p-2">
                {{ compassHeading?.toFixed(2) }}
              </td>
            </tr>
          </table>
        </div>
      </template>
    </div>
  </v-collapse>
</template>

<script setup lang="ts">

interface Props {
  name: string
  data?: {
    altitudeMm: number
    compassHeading: number
    linearVelocity?: {
      x: number
      y: number
      z: number
    }
    angularVelocity?: {
      x: number
      y: number
      z: number
    }
    properties: {
      compassHeadingSupported: boolean
      linearVelocitySupported: boolean
      angularVelocitySupported: boolean
      positionSupported: boolean
      orientationSupported: boolean
    },
    coordinate?: {
      latitude: number
      longitude: number
    }
    orientation?: {
      oX: number
      oY: number
      oZ: number
      theta: number
    }
  }
}

defineProps<Props>();

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
    <div class="flex items-end border border-t-0 border-black p-4">
      <template v-if="data?.properties">
        <div
          v-if="data.properties.positionSupported"
          class="mr-4 w-1/4"
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
                {{ data.coordinate?.latitude.toFixed(6) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Longitude
              </th>
              <td class="border border-black p-2">
                {{ data.coordinate?.longitude.toFixed(6) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Altitide
              </th>
              <td class="border border-black p-2">
                {{ data.altitudeMm?.toFixed(2) }}
              </td>
            </tr>
          </table>
          <a :href="`https://www.google.com/maps/search/${data.coordinate?.latitude},${data.coordinate?.longitude}`">
            google maps
          </a>
        </div>

        <div
          v-if="data.properties.orientationSupported"
          class="mr-4 w-1/4"
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
                {{ data.orientation?.oX.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                OY
              </th>
              <td class="border border-black p-2">
                {{ data.orientation?.oY.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                OZ
              </th>
              <td class="border border-black p-2">
                {{ data.orientation?.oZ.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Theta
              </th>
              <td class="border border-black p-2">
                {{ data.orientation?.theta.toFixed(2) }}
              </td>
            </tr>
          </table>
        </div>
              
        <div
          v-if="data.properties.angularVelocitySupported"
          class="mr-4 w-1/4"
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
                {{ data.angularVelocity?.x.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Y
              </th>
              <td class="border border-black p-2">
                {{ data.angularVelocity?.y.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Z
              </th>
              <td class="border border-black p-2">
                {{ data.angularVelocity?.z.toFixed(2) }}
              </td>
            </tr>
          </table>
        </div>

        <div
          v-if="data.properties.linearVelocitySupported"
          class="mr-4 w-1/4"
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
                {{ data.linearVelocity?.x.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Y
              </th>
              <td class="border border-black p-2">
                {{ data.linearVelocity?.y.toFixed(2) }}
              </td>
            </tr>
            <tr>
              <th class="border border-black p-2">
                Z
              </th>
              <td class="border border-black p-2">
                {{ data.linearVelocity?.z.toFixed(2) }}
              </td>
            </tr>
          </table>
        </div>

        <div
          v-if="data.properties.compassHeadingSupported"
          class="mr-4 w-1/4"
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
                {{ data.compassHeading?.toFixed(2) }}
              </td>
            </tr>
          </table>
        </div>
      </template>
    </div>
  </v-collapse>
</template>

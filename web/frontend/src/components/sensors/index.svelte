<script lang="ts">
import { useRobotClient } from '@/hooks/robot-client';
import Collapse from '@/lib/components/collapse.svelte';
import { rcLogConditionally } from '@/lib/log';
import { resourceNameToString } from '@/lib/resource';
import { notify } from '@viamrobotics/prime';
import { ConnectError, ResourceName, sensorsApi } from '@viamrobotics/sdk';

export let name: string;
export let sensorNames: ResourceName[];

const { robotClient } = useRobotClient();

interface Reading {
  _type: string;
  lat: number;
  lng: number;
}

const sensorReadings: Record<string, Record<string, Reading>> = {};

const getReadings = (inputNames: ResourceName[]) => {
  const req = new sensorsApi.GetReadingsRequest();
  const names = inputNames.map(
    ({ name: inputName, namespace, type, subtype }) => {
      return new ResourceName({
        namespace,
        type,
        subtype,
        name: inputName,
      });
    }
  );
  req.name = name;
  req.sensorNames = names;

  rcLogConditionally(req);
  $robotClient.sensorsService
    .getReadings(req)
    .then((resp) => {
      for (const item of resp.readings) {
        const { readings } = item;
        const rr: Record<string, Reading> = {};

        for (const [key, value] of Object.entries(readings)) {
          rr[key] = value.toJson() as unknown as Reading;
        }

        sensorReadings[resourceNameToString(item.name)] = rr;
      }
    })
    .catch((error) => {
      if (error instanceof ConnectError) {
        notify.danger(error.message);
      }
    });
};

const getData = (
  readings: Record<string, Record<string, Reading>>,
  sensorName: ResourceName
) => {
  const data = readings[resourceNameToString(sensorName)];
  return data ? Object.entries(data) : [];
};
</script>

<Collapse title="Sensors">
  <div class="overflow-auto border border-t-0 border-medium p-4 text-sm">
    <table class="w-full table-auto border border-medium">
      <tr>
        <th class="border border-medium p-2"> Name </th>
        <th class="border border-medium p-2"> Type </th>
        <th class="border border-medium p-2"> Readings </th>
        <th class="border border-medium p-2 text-center">
          <v-button
            label="Get All Readings"
            on:click|stopPropagation={() => {
              getReadings(sensorNames);
            }}
          />
        </th>
      </tr>
      {#each sensorNames as sensorName (sensorName.name)}
        <tr>
          <td class="border border-medium p-2">
            {sensorName.name}
          </td>
          <td class="border border-medium p-2">
            {sensorName.subtype}
          </td>
          <td class="border border-medium p-2">
            <table style="font-size:.7em; text-align: left;">
              {#each getData(sensorReadings, sensorName) as [sensorField, sensorValue] (sensorField)}
                <tr>
                  <th>{sensorField}</th>
                  <td>
                    {JSON.stringify(sensorValue)}
                    <!-- eslint-disable-next-line no-underscore-dangle -->
                    {#if sensorValue._type === 'geopoint'}
                      <a
                        href={`https://www.google.com/maps/search/${sensorValue.lat},${sensorValue.lng}`}
                        >google maps</a
                      >
                    {/if}
                  </td>
                </tr>
              {/each}
            </table>
          </td>
          <td class="border border-medium p-2 text-center">
            <v-button
              label="Get Readings"
              on:click|stopPropagation={() => {
                getReadings([sensorName]);
              }}
            />
          </td>
        </tr>
      {/each}
    </table>
  </div>
</Collapse>

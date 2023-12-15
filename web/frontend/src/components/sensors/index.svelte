<script lang="ts">
  import { sensorsApi, commonApi, type ServiceError } from '@viamrobotics/sdk';
  import { notify } from '@viamrobotics/prime';
  import { resourceNameToString } from '@/lib/resource';
  import { rcLogConditionally } from '@/lib/log';
  import { useRobotClient } from '@/hooks/robot-client';

  interface SensorName {
    name: string;
    namespace: string;
    type: string;
    subtype: string;
  }

  export let name: string;
  export let sensorNames: SensorName[];

  const { robotClient } = useRobotClient();

  interface Reading {
    _type: string;
    lat: number;
    lng: number;
  }

  const sensorReadings: Record<string, Record<string, Reading>> = {};

  const getReadings = (inputNames: SensorName[]) => {
    const req = new sensorsApi.GetReadingsRequest();
    const names = inputNames.map(({ name: inputName, namespace, type, subtype }) => {
      const resourceName = new commonApi.ResourceName();
      resourceName.setNamespace(namespace);
      resourceName.setType(type);
      resourceName.setSubtype(subtype);
      resourceName.setName(inputName);
      return resourceName;
    });
    req.setName(name);
    req.setSensorNamesList(names);

    rcLogConditionally(req);
    $robotClient.sensorsService.getReadings(
      req,
      (
        error: ServiceError | null,
        response: sensorsApi.GetReadingsResponse | null
      ) => {
        if (error) {
          notify.danger(error.message);
          return;
        }

        for (const item of response!.getReadingsList()) {
          const readings = item.getReadingsMap();
          const rr: Record<string, Reading> = {};

          for (const [key, value] of readings.entries()) {
            rr[key] = value.toJavaScript() as Reading;
          }

          sensorReadings[resourceNameToString(item.getName()!.toObject())] = rr;
        }
      }
    );
  };

  const getData = (readings: Record<string, Record<string, Reading>>, sensorName: SensorName) => {
    const data = readings[resourceNameToString(sensorName)];
    return data ? Object.entries(data) : [];
  };
</script>

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

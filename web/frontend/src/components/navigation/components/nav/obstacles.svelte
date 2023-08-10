<script lang='ts'>

import LnglatInput from '../input/lnglat.svelte';
import GeometryInputs from '../input/geometry.svelte';
import OrientationInput from '../input/orientation.svelte';
import { obstacles, write, hovered, mapCenter, boundingRadius, map } from '../../stores';
import { createObstacle } from '../../lib/obstacle';
import type { Geometry, LngLat } from '@/api/navigation';
import { calculateBoundingBox } from '../../lib/bounding-box';

const handleSelect = (selection: { name: string; location: LngLat }) => {
  const zoom = boundingRadius[selection.name]!;
  const bb = calculateBoundingBox(zoom, selection.location);
  map.current?.fitBounds(bb, { duration: 800, curve: 0.1 });
};

const handleAddObstacle = () => {
  $obstacles = [
    createObstacle(`Obstacle ${$obstacles.length + 1}`, $mapCenter),
    ...$obstacles,
  ];
};

const handleLngLatInput = (index: number, event: CustomEvent<LngLat>) => {
  $obstacles[index]!.location = event.detail;
};

const handleDeleteObstacle = (index: number) => {
  $obstacles = $obstacles.filter((_, i) => i !== index);
};

const handleGeometryInput = (index: number, geoIndex: number) => {
  return (event: CustomEvent<Geometry>) => {
    $obstacles[index]!.geometries[geoIndex] = event.detail;
  };
};

</script>

{#if $obstacles.length === 0}
  <li class='text-xs text-subtle-2 font-sans py-2'>
    {#if write}
      Click to add an obstacle.
    {:else}
      Add a static obstacle in your robot's config.
    {/if}
  </li>
{/if}

{#if $write}
  <v-button
    class='my-4'
    icon='plus'
    label='Add'
    on:click={handleAddObstacle}
  />
{/if}

{#each $obstacles as { name: obstacleName, location, geometries }, index (index)}
  {#if $write}
    <li class='group mb-8 pl-2 border-l border-l-medium'>
      <div class='flex items-end gap-1.5 pb-2'>
        <v-input class='w-full' label='Name' value={obstacleName} />
        <v-button
          class='sm:invisible group-hover:visible text-subtle-1'
          variant='icon'
          icon='trash-can-outline'
          on:click={() => handleDeleteObstacle(index)}
        />
      </div>
      <LnglatInput
        lng={location.lng}
        lat={location.lat}
        on:input={(event) => handleLngLatInput(index, event)}>
        <v-button
          class='sm:invisible group-hover:visible text-subtle-1'
          variant='icon'
          icon='image-filter-center-focus'
          aria-label="Focus"
          on:click={() => handleSelect({ name: obstacleName, location })}
        />

      </LnglatInput>

      {#each geometries as geometry, geoIndex (geoIndex)}
        <GeometryInputs
          {geometry}
          on:input={handleGeometryInput(index, geoIndex)}
        />
        <OrientationInput quaternion={geometry.pose.quaternion} />
      {/each}
    </li>

  {:else}
    <li
      class='group border-b border-b-medium last:border-b-0 py-3 leading-[1]'
      on:mouseenter={() => ($hovered = obstacleName)}
    >
      <div class='flex justify-between items-center gap-1.5'>
        <small>{obstacleName}</small>
        <div class='flex items-center gap-1.5'>
          <small class='text-subtle-2 opacity-60 group-hover:opacity-100'>
            ({location.lat.toFixed(4)}, {location.lng.toFixed(4)})
          </small>
          <v-button
            class='sm:invisible group-hover:visible text-subtle-1'
            variant='icon'
            icon='image-filter-center-focus'
            aria-label="Focus {obstacleName}"
            on:click={() => handleSelect({ name: obstacleName, location })}
          />
        </div>
      </div>
      {#each geometries as geometry}
        <small class='text-subtle-2'>
            {#if geometry.type === 'box'}
              Length: {geometry.length}m, Width: {geometry.width}m, Height: {geometry.height}m
            {:else if geometry.type === 'sphere'}
              Radius: {geometry.radius}m
            {:else if geometry.type === 'capsule'}
              Radius: {geometry.radius}m, Length: { geometry.length}m
            {/if}
        </small>

        {#if geometry.pose.orientationVector.th !== 0}
          <small class='block text-subtle-2 mt-2'>
            Theta: {geometry.pose.orientationVector.th.toFixed(2)}
          </small>
        {/if}
      {/each}
    </li>
  {/if}
{/each}

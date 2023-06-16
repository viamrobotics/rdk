<script lang='ts'>
  
import { onMount, onDestroy } from 'svelte';
import { notify } from '@viamrobotics/prime';
import { navigationApi, type robotApi, type ServiceError, type Client, type ResponseStream } from '@viamrobotics/sdk';
import { Marker, Popup, Map, NavigationControl, type MapMouseEvent } from 'maplibre-gl';
import {
  setWaypoint,
  getWaypoints,
  removeWaypoint,
  getLocation,
} from '@/api/navigation';
import { setAsyncInterval } from '@/lib/schedule';
import { style } from './style'
import ThreeLayer from './threelayer.svelte'

export let name: string;
export let client: Client;
export let statusStream: ResponseStream<robotApi.StreamStatusResponse> | null;

const refreshRate = 500;
const knownWaypoints: Record<string, Marker> = {};
const robotMarker = new Marker({ color: 'red' });
robotMarker.setPopup(new Popup().setHTML('robot'));

let centered = false;
let map: Map | undefined;

let clearUpdateWaypointInterval: undefined | (() => void)
let clearUpdateLocationInterval: undefined | (() => void)
let updateWaypointsId: number;
let updateLocationsId: number;

const handleClick = async (event: MapMouseEvent) => {
  if (event.originalEvent.button > 0) {
    return;
  }

  const { lat, lng } = event.lngLat;

  try {
    await setWaypoint(client, lat, lng, name);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

const updateWaypoints = async () => {
  let waypoints: navigationApi.Waypoint[];

  try {
    waypoints = await getWaypoints(client, name);
  } catch (error) {
    notify.danger((error as ServiceError).message);
    return;
  }

  const currentWaypoints: Record<string, Marker> = {};

  for (const waypoint of waypoints) {
    const position = {
      lat: waypoint.getLocation()?.getLatitude() ?? 0,
      lng: waypoint.getLocation()?.getLongitude() ?? 0,
    };

    const posStr = JSON.stringify(position);

    if (knownWaypoints[posStr]) {
      currentWaypoints[posStr] = knownWaypoints[posStr]!;
      continue;
    }

    const marker = new Marker({ scale: 0.7 });

    marker.setLngLat([position.lng, position.lat]).addTo(map!);

    currentWaypoints[posStr] = marker;
    knownWaypoints[posStr] = marker;

    // eslint-disable-next-line no-loop-func
    marker.getElement().addEventListener('contextmenu', async () => {
      marker.remove();

      try {
        await removeWaypoint(client, name, waypoint.getId());
      } catch (error) {
        notify.danger((error as ServiceError).message);
        marker.addTo(map!);
      }
    });
  }

  const waypointsToDelete = Object.keys(knownWaypoints).filter(
    (elem) => !(elem in currentWaypoints)
  );

  for (const key of waypointsToDelete) {
    const marker = knownWaypoints[key]!;
    marker.remove();
    delete knownWaypoints[key];
  }
};

const updateLocation = async () => {
  try {
    const position = await getLocation(client, name);

    if (!centered) {
      centered = true;
      map!.setCenter(position);
    }

    robotMarker.setLngLat([position.lng, position.lat]).addTo(map!);

  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

onMount(() => {
  map = new Map({
    container: 'navigation-map',
    style,
    center: [-74.5, 40],
    zoom: 9,
    antialias: true,
  });

  map.addControl(new NavigationControl());
  map.on('click', handleClick);

  clearUpdateWaypointInterval = setAsyncInterval(updateWaypoints, refreshRate);
  clearUpdateLocationInterval = setAsyncInterval(updateLocation, refreshRate);

  updateWaypoints();
  updateLocation();

  statusStream?.on('end', () => {
    clearTimeout(updateWaypointsId);
    clearTimeout(updateLocationsId);
  });
})

onDestroy(() => {
  clearUpdateWaypointInterval?.()
  clearUpdateLocationInterval?.()
})

</script>

<div
  id='navigation-map'
  class="mb-2 h-[400px] w-full"
/>

{#if map}
  <ThreeLayer {map} />
{/if}
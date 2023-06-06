<svelte:options immutable />

<script setup lang="ts">

import 'maplibre-gl/dist/maplibre-gl.css';
import { toast } from '@/lib/toast';
import { Client, robotApi, navigationApi, type ServiceError, type ResponseStream } from '@viamrobotics/sdk';
import Collapse from '../collapse.svelte';
import maplibregl from 'maplibre-gl'; 
import { setMode, setWaypoint, getWaypoints, removeWaypoint, getLocation } from '@/api/navigation';

export let name: string
export let client: Client
export let statusStream: ResponseStream<robotApi.StreamStatusResponse> | null

let updateWaypointsId: number;
let updateLocationsId: number;

let refreshRate = 500;

const setNavigationMode = async (event: CustomEvent) => {
  const mode = event.detail.value.toLowerCase() as 'manual' | 'waypoint'

  let pbMode: 0 | 1 | 2 = navigationApi.Mode.MODE_UNSPECIFIED;

  if (mode === 'manual') {
    pbMode = navigationApi.Mode.MODE_MANUAL;
  } else if (mode === 'waypoint') {
    pbMode = navigationApi.Mode.MODE_WAYPOINT;
  }

  try {
    await setMode(client, name, pbMode)
  } catch (error) {
    toast.error((error as ServiceError).message);
  }
};

let map: maplibregl.Map

const createWaypointMarker = () => {
  return new maplibregl.Marker({ scale: 0.7 })
}

const handleClick = async (event: maplibregl.MapMouseEvent) => {
  if (event.originalEvent.button > 0) {
    return
  }

  const { lat, lng } = event.lngLat;

  try {
    await setWaypoint(client, lat, lng, name)
  } catch (error) {
    toast.error((error as ServiceError).message)
  }
}

const initNavigation = async () => {
  const style: maplibregl.StyleSpecification = {
    version: 8,
    sources: {
      osm: {
        type: 'raster',
        tiles: ['https://a.tile.openstreetmap.org/{z}/{x}/{y}.png'],
        tileSize: 256,
        attribution: '&copy; OpenStreetMap Contributors',
        maxzoom: 19,
      },
    },
    layers: [
      {
        id: 'osm',
        type: 'raster',
        source: 'osm',
      },
    ],
  };

  map = new maplibregl.Map({
    container: 'navigation-map',
    style,
    center: [-74.5, 40],
    zoom: 9
  });

  map.addControl(new maplibregl.NavigationControl());
  map.on('click', handleClick);

  let centered = false;

  const knownWaypoints: Record<string, maplibregl.Marker> = {};

  const refresh = async () => {
    let waypoints: navigationApi.Waypoint[]

    try {
      waypoints = await getWaypoints(client, name)
    } catch (error) {
      toast.error((error as ServiceError).message)
      updateWaypointsId = window.setTimeout(refresh, 1000);
      return;
    }

    const currentWaypoints: Record<string, maplibregl.Marker> = {};

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

      const marker = createWaypointMarker()

      marker.setLngLat([position.lng, position.lat]).addTo(map)

      currentWaypoints[posStr] = marker;
      knownWaypoints[posStr] = marker;

      marker.getElement().addEventListener('contextmenu', async () => {
        marker.remove()

        try {
          await removeWaypoint(client, name, waypoint.getId());
        } catch (error) {
          toast.error((error as ServiceError).message)
          marker.addTo(map)
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

    updateWaypointsId = window.setTimeout(refresh, refreshRate);
  };

  refresh();

  const robotMarker = new maplibregl.Marker({ color: 'red' });

  robotMarker.setPopup(new maplibregl.Popup().setHTML('robot'));

  const updateLocation = async () => {
    try {
      const position = await getLocation(client, name)

      if (!centered) {
        centered = true;
        map.setCenter(position);
      }

      robotMarker.setLngLat([position.lng, position.lat]).addTo(map);

      updateLocationsId = window.setTimeout(updateLocation, refreshRate);
    } catch (error) {
      toast.error((error as ServiceError).message)
      updateLocationsId = window.setTimeout(updateLocation, refreshRate);
    }
  };

  updateLocation();
};

const handleToggle = (event: CustomEvent<{ open: boolean }>) => {
  const { open } = event.detail

  if (open) {
    handleOpen()
  } else {
    handleClose()
  }
}

const handleOpen = () => {
  initNavigation();

  statusStream?.on('end', () => {
    clearTimeout(updateWaypointsId);
    clearTimeout(updateLocationsId);
  });
};

const handleClose = () => {
  clearTimeout(updateWaypointsId);
  clearTimeout(updateLocationsId);
};

</script>

<Collapse
  title={name}
  on:toggle={handleToggle}
>
  <v-breadcrumbs
    slot="title"
    crumbs="navigation"
  />

  <div class="flex flex-col gap-2 border border-t-0 border-medium p-4">
    <v-radio
      label="Navigation mode"
      options="Manual, Waypoint"
      on:input={setNavigationMode}
    />

    <div
      id='navigation-map'
      class="mb-2 h-[400px] w-full"
    />
  </div>
</Collapse>

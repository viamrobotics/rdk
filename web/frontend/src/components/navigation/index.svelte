<svelte:options immutable />

<script setup lang="ts">

import 'maplibre-gl/dist/maplibre-gl.css';
import { grpc } from '@improbable-eng/grpc-web';
import { toast } from '@/lib/toast';
import { filterResources } from '@/lib/resource';
import { Client, commonApi, robotApi, navigationApi, type ServiceError, type ResponseStream } from '@viamrobotics/sdk';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import { rcLogConditionally } from '@/lib/log';
import Collapse from '../collapse.svelte';
import maplibregl from 'maplibre-gl'; 
import { setMode, setWaypoint, getWaypoints, removeWaypoint, getLocation } from '@/api/navigation';

export let resources: commonApi.ResourceName.AsObject[]
export let name: string
export let client: Client
export let statusStream: ResponseStream<robotApi.StreamStatusResponse> | null

let updateWaypointsId: number;
let updateLocationsId: number;

let location = '';

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

const setNavigationLocation = () => {
  const [latStr, lngStr] = location.split(',');
  if (latStr === undefined || lngStr === undefined) {
    return;
  }

  const lat = Number.parseFloat(latStr);
  const lng = Number.parseFloat(lngStr);

  // TODO: Not sure how this works (if it does), robotApi has no ResourceRunCommandRequest method
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const req = new (robotApi as any).ResourceRunCommandRequest();
  let gpsName = '';

  const [gps] = filterResources(resources ?? [], 'rdk', 'component', 'gps');

  if (gps) {
    gpsName = gps.name;
  } else {
    toast.error('no gps device found');
    return;
  }

  req.setName(name);
  req.setResourceName(gpsName);
  req.setCommandName('set_location');
  req.setArgs(
    Struct.fromJavaScript({
      latitude: lat,
      longitude: lng,
    })
  );

  rcLogConditionally(req);
  client.genericService.doCommand(req, new grpc.Metadata(), (error: ServiceError | null) => {
    if (error) {
      toast.error(error.message);
    }
  });
};

let map: maplibregl.Map

const marker = new maplibregl.Marker()

const handleClick = async (event: maplibregl.MapMouseEvent) => {
  marker.setLngLat(event.lngLat).addTo(map);

  const { lat, lng } = event.lngLat;

  try {
    await setWaypoint(client, lat, lng, name)
  } catch (error) {
    marker.remove()
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
  let localLabelCounter = 0;

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

      localLabelCounter += 1;

      const marker = new maplibregl.Marker({
        // label: `${localLabelCounter}`,
      });

      marker.setLngLat([position.lng, position.lat]).addTo(map)

      currentWaypoints[posStr] = marker;
      knownWaypoints[posStr] = marker;

      marker.on('dblclick', async () => {
        try {
          removeWaypoint(client, name, waypoint.getId());
        } catch (error) {
          toast.error((error as ServiceError).message)
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

    updateWaypointsId = window.setTimeout(refresh, 1000);
  };

  refresh();

  const locationMarker = new maplibregl.Marker({
    // label: 'robot'
  });

  const updateLocation = async () => {
    try {
      const position = await getLocation(client, name)

      if (!centered) {
        centered = true;
        map.setCenter(position);
      }

      locationMarker.setLngLat([position.lng, position.lat]).addTo(map);

      updateLocationsId = window.setTimeout(updateLocation, 1000);
    } catch (error) {
      toast.error((error as ServiceError).message)
      updateLocationsId = window.setTimeout(updateLocation, 1000);
    }
  };

  updateLocation();
};

const handleLocationInput = (event: CustomEvent) => {
  location = event.detail.value
}

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

    <v-button
      label="Try Set Location"
      on:click={setNavigationLocation}
    />

    <div
      id='navigation-map'
      class="mb-2 h-[400px] w-full"
    />

    <v-input
      label="Location"
      value={location}
      on:input={handleLocationInput}
    />
  </div>
</Collapse>

<script setup lang="ts">

import { ref, onMounted, onUnmounted } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import jspb from 'google-protobuf';
import { toast } from '../lib/toast';
import { filterResources, type Resource } from '../lib/resource';

import commonApi from '../gen/proto/api/common/v1/common_pb.esm';
import robotApi from '../gen/proto/api/robot/v1/robot_pb.esm';
import type { ServiceError } from '../gen/proto/stream/v1/stream_pb_service.esm';
import navigationApi, { 
  type GetLocationResponse, 
  type GetWaypointsResponse, 
  type Waypoint,
} from '../gen/proto/api/service/navigation/v1/navigation_pb.esm';

interface Props {
  resources: Resource[]
  name:string
}

const props = defineProps<Props>();

let googleMapsInitResolve: () => void;
const mapReady = new Promise<void>((resolve) => {
  googleMapsInitResolve = resolve;
});

let map: google.maps.Map;
let updateTimerId: number;

const location = ref('');
const res = ref();
const container = ref<HTMLElement>();

const setNavigationMode = (mode: 'manual' | 'waypoint') => {
  let pbMode: 0 | 1 | 2 = navigationApi.Mode.MODE_UNSPECIFIED;

  if (mode === 'manual') {
    pbMode = navigationApi.Mode.MODE_MANUAL;
  } else if (mode === 'waypoint') {
    pbMode = navigationApi.Mode.MODE_WAYPOINT;
  }

  const req = new navigationApi.SetModeRequest();
  req.setName(props.name);
  req.setMode(pbMode);
  window.navigationService.setMode(req, new grpc.Metadata(), grpcCallback);
};

const setNavigationLocation = () => {
  const posSplit = location.value.split(',');
  if (posSplit.length !== 2) {
    return;
  }
  const lat = Number.parseFloat(posSplit[0]);
  const lng = Number.parseFloat(posSplit[1]);

  // TODO: Not sure how this works (if it does), robotApi has no ResourceRunCommandRequest method
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const req = new (robotApi as any).ResourceRunCommandRequest();
  let gpsName = '';
  const gpses = filterResources(props.resources, 'rdk', 'component', 'gps');
  if (gpses.length > 0) {
    gpsName = gpses[0].name;
  } else {
    toast.error('no gps device found');
    return;
  }
  req.setName(props.name);
  req.setResourceName(gpsName);
  req.setCommandName('set_location');
  req.setArgs(
    Struct.fromJavaScript({
      latitude: lat,
      longitude: lng,
    })
  );
  
  window.genericService.do(req, new grpc.Metadata(), grpcCallback);
};

const grpcCallback = (error: ServiceError | null, response: jspb.Message | Struct | null, stringify = true) => {
  if (error) {
    toast.error(error);
    return;
  }
  if (stringify) {
    try {
      if (response === null) {
        return res.value = null;
      }

      res.value = (response as Struct).toJavaScript ? JSON.stringify((response as Struct).toJavaScript()) : JSON.stringify(response.toObject());
    } catch {
      toast.error(error);
    }
  }
};

const loadMaps = () => {
  if (document.querySelector('#google-maps')) {
    return initNavigation();
  }
  const script = document.createElement('script');
  script.id = 'google-maps';
  // TODO(RSDK-51): remove api key once going into production
  script.src = 'https://maps.googleapis.com/maps/api/js?key=AIzaSyBn72TEqFOVWoj06cvua0Dc0pz2uvq90nY&callback=googleMapsInit&libraries=&v=weekly';
  script.async = true;
  document.head.append(script);
};

const initNavigation = async () => {
  await mapReady;

  map = new window.google.maps.Map(container.value!, { zoom: 18 });
  map.addListener('click', (event: google.maps.MapMouseEvent) => {
    const lat = event.latLng?.lat();
    const lng = event.latLng?.lng();

    if (lat === undefined || lng === undefined) {
      return;
    }

    const req = new navigationApi.AddWaypointRequest();
    const point = new commonApi.GeoPoint();

    point.setLatitude(lat);
    point.setLongitude(lng);
    req.setName(props.name);
    req.setLocation(point);

    window.navigationService.addWaypoint(req, new grpc.Metadata(), grpcCallback);
  });

  let centered = false;
  const knownWaypoints: Record<string, google.maps.Marker> = {};
  let localLabelCounter = 0;
  
  const updateWaypoints = () => {
    const req = new navigationApi.GetWaypointsRequest();
    req.setName(props.name);

    window.navigationService.getWaypoints(req, new grpc.Metadata(), (err: ServiceError | null, resp: GetWaypointsResponse | null) => {
      grpcCallback(err, resp, false);

      if (err) {
        updateTimerId = window.setTimeout(updateWaypoints, 1000);
        return;
      }

      let waypoints: Waypoint[] = [];

      if (resp) {
        waypoints = resp.getWaypointsList();
      }

      const currentWaypoints: Record<string, google.maps.Marker> = {};

      for (const waypoint of waypoints) {
        const pos = {
          lat: waypoint.getLocation()?.getLatitude() ?? 0,
          lng: waypoint.getLocation()?.getLongitude() ?? 0,
        };

        const posStr = JSON.stringify(pos);

        if (knownWaypoints[posStr]) {
          currentWaypoints[posStr] = knownWaypoints[posStr];
          continue;
        }

        const marker = new window.google.maps.Marker({
          position: pos,
          map,
          label: `${localLabelCounter++}`,
        });

        currentWaypoints[posStr] = marker;
        knownWaypoints[posStr] = marker;

        marker.addListener('click', () => {
          console.log('clicked on marker', pos);
        });

        marker.addListener('dblclick', () => {
          const req = new navigationApi.RemoveWaypointRequest();
          req.setName(props.name);
          req.setId(waypoint.getId());
          window.navigationService.removeWaypoint(req, new grpc.Metadata(), grpcCallback);
        });
      }

      const waypointsToDelete = Object.keys(knownWaypoints).filter((elem) => !(elem in currentWaypoints));

      for (const key of waypointsToDelete) {
        const marker = knownWaypoints[key];
        marker.setMap(null);
        delete knownWaypoints[key];
      }

      updateTimerId = window.setTimeout(updateWaypoints, 1000);
    });
  };

  updateWaypoints();

  const locationMarker = new window.google.maps.Marker({ label: 'robot' });

  const updateLocation = () => {
    const req = new navigationApi.GetLocationRequest();

    req.setName(props.name);
    window.navigationService.getLocation(req, new grpc.Metadata(), (err: ServiceError | null, resp: GetLocationResponse | null) => {
      grpcCallback(err, resp, false);

      if (err) {
        setTimeout(updateLocation, 1000);
        return;
      }

      const pos = { lat: resp?.getLocation()?.getLatitude() ?? 0, lng: resp?.getLocation()?.getLongitude() ?? 0 };

      if (!centered) {
        centered = true;
        map.setCenter(pos);
      }

      locationMarker.setPosition(pos);
      locationMarker.setMap(map);

      setTimeout(updateLocation, 1000);
    });
  };
  updateLocation();
};

window.googleMapsInit = () => {
  console.log('google maps is ready');
  googleMapsInitResolve();
};

onMounted(() => {
  loadMaps();
  initNavigation();
});

onUnmounted(() => {
  clearTimeout(updateTimerId);
});

</script>

<template>
  <v-collapse
    :title="props.name"
    class="navigation"
  >
    <v-breadcrumbs
      slot="title"
      :crumbs="['navigation'].join(',')"
    />
    <div class="flex flex-col gap-2 border border-t-0 border-black p-4">
      <v-radio
        label="Navigation mode"
        options="Manual, Waypoint"
        @input="setNavigationMode($event.detail.value.toLowerCase())"
      />

      <div>
        <v-button
          label="Try Set Location"
          @click="setNavigationLocation()"
        />
      </div>

      <div
        id="map"
        ref="container"
        class="mb-2 h-[400px] w-full"
      />

      <v-input
        label="Location"
        :value="location"
        @input="location = $event.detail.value"
      />
    </div>
  </v-collapse>
</template>

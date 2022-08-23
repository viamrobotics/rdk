<script setup lang="ts">

import { ref, onMounted, onUnmounted } from 'vue';
import commonApi from '../gen/proto/api/common/v1/common_pb.esm';
import robotApi from '../gen/proto/api/robot/v1/robot_pb.esm';
import navigationApi from '../gen/proto/api/service/navigation/v1/navigation_pb.esm';
import { toast } from '../lib/toast';
import { filterResources } from '../lib/resource';

interface Props {
  resources: any
  name:string
}

const props = defineProps<Props>();

let mapReadyResolve: any;
const mapReady = new Promise((resolve) => {
  mapReadyResolve = resolve;
});

let map: any;
let updateTimerId: number;

const location = ref('');
const res = ref();
const container = ref();

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
  (window as any).navigationService.setMode(req, {}, grpcCallback);
};

const setNavigationLocation = () => {
  const posSplit = location.value.split(',');
  if (posSplit.length !== 2) {
    return;
  }
  const lat = Number.parseFloat(posSplit[0]);
  const lng = Number.parseFloat(posSplit[1]);
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
    (window as any).proto.google.protobuf.Struct.fromJavaScript({
      latitude: lat,
      longitude: lng,
    })
  );
  (window as any).robotService.resourceRunCommand(req, {}, grpcCallback);
};

const grpcCallback = (error: any, response: any, stringify = true) => {
  if (error) {
    toast.error(error);
    return;
  }
  if (stringify) {
    try {
      res.value = response.toJavaScript ? JSON.stringify(response.toJavaScript()) : JSON.stringify(response.toObject());
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
  script.src = 'https://maps.googleapis.com/maps/api/js?key=AIzaSyBn72TEqFOVWoj06cvua0Dc0pz2uvq90nY&callback=initMap&libraries=&v=weekly';
  script.async = true;
  document.head.append(script);
};

const initNavigation = async () => {
  await mapReady;
  map = new (window as any).google.maps.Map(container.value, { zoom: 18 });
  map.addListener('click', (event: any) => {
    const req = new navigationApi.AddWaypointRequest();
    const point = new commonApi.GeoPoint();
    point.setLatitude(event.latLng.lat());
    point.setLongitude(event.latLng.lng());
    req.setName(props.name);
    req.setLocation(point);
    (window as any).navigationService.addWaypoint(req, {}, grpcCallback);
  });

  let centered = false;
  const knownWaypoints: any = {};
  let localLabelCounter = 0;
  
  const updateWaypoints = () => {
    const req = new navigationApi.GetWaypointsRequest();
    req.setName(props.name);
    (window as any).navigationService.getWaypoints(req, {}, (err: any, resp: any) => {
      grpcCallback(err, resp, false);
      if (err) {
        updateTimerId = window.setTimeout(updateWaypoints, 1000);
        return;
      }
      let waypoints = [];
      if (resp) {
        waypoints = resp.getWaypointsList();
      }
      const currentWaypoints: any = {};
      for (const waypoint of waypoints) {
        const pos = {
          lat: waypoint.getLocation().getLatitude(),
          lng: waypoint.getLocation().getLongitude(),
        };
        const posStr = JSON.stringify(pos);
        if (knownWaypoints[posStr]) {
          currentWaypoints[posStr] = knownWaypoints[posStr];
          continue;
        }
        const marker = new (window as any).google.maps.Marker({
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
          (window as any).navigationService.removeWaypoint(req, {}, grpcCallback);
        });
      }
      const waypointsToDelete = Object.keys(knownWaypoints).filter((elem) => {
        return !(elem in currentWaypoints);
      });
      for (const key of waypointsToDelete) {
        const marker = knownWaypoints[key];
        marker.setMap(null);
        delete knownWaypoints[key];
      }
      updateTimerId = window.setTimeout(updateWaypoints, 1000);
    });
  };
  updateWaypoints();

  const locationMarker = new (window as any).google.maps.Marker({ label: 'robot' });
  const updateLocation = () => {
    const req = new navigationApi.GetLocationRequest();
    req.setName(props.name);
    (window as any).navigationService.getLocation(req, {}, (err: any, resp: any) => {
      grpcCallback(err, resp, false);
      if (err) {
        setTimeout(updateLocation, 1000);
        return;
      }
      const pos = { lat: resp.getLocation().getLatitude(), lng: resp.getLocation().getLongitude() };
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

(window as any).initMap = () => {
  console.log('map is ready');
  mapReadyResolve();
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
  <v-collapse :title=props.name>
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

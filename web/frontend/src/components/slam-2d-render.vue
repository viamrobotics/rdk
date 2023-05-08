<script setup lang="ts">

import { $ref } from 'vue/macros';
import { threeInstance, MouseRaycaster, MeshDiscardMaterial } from 'trzy';
import { onMounted, onUnmounted, watch } from 'vue';
import * as THREE from 'three';
import { MapControls } from 'three/examples/jsm/controls/OrbitControls';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import type { commonApi } from '@viamrobotics/sdk';
import DestMarker from '../lib/destination-marker.png?raw';
import BaseMarker from '../lib/base-marker.png?raw';

interface SvgOffset {
  x: number,
  y: number,
  z: number
}

const backgroundGridColor = 0xCA_CA_CA;

const gridSubparts = ['AxesPos', 'AxesNeg', 'Grid'];

const gridHelperRenderOrder = 997;
const axesHelperRenderOrder = 998;
const svgMarkerRenderOrder = 999;

const gridHelperScalar = 4;
const axesHelperSize = 8;

// Note: updating the scale of the destination or base marker requires an offset update
const baseMarkerScalar = 0.002;
const destinationMarkerScalar = 0.1;

const textureLoader = new THREE.TextureLoader();

const makeMarker = (png: string, name: string, scalar: number) => {
  const geometry = new THREE.PlaneGeometry();
  const material = new THREE.MeshBasicMaterial({ map: textureLoader.load(png), color: 'red' });
  const marker = new THREE.Mesh(geometry, material);
  marker.name = name;
  marker.renderOrder = svgMarkerRenderOrder;
  return marker;
};

const baseMarker = makeMarker(BaseMarker, 'BaseMarker', baseMarkerScalar);
const destMarker = makeMarker(DestMarker, 'DestinationMarker', destinationMarkerScalar);

const baseMarkerOffset: SvgOffset = {
  x: -0.05,
  y: -0.3,
  z: 0,
};

const destinationMarkerOffset: SvgOffset = {
  x: 1.2,
  y: 2.5,
  z: 0,
};

/*
 * this color map is greyscale. The color map is being used map probability values of a PCD
 * into different color buckets provided by the color map.
 * generated with: https://grayscale.design/app
 */
const colorMapGrey = [
  [240, 240, 240],
  [220, 220, 220],
  [200, 200, 200],
  [190, 190, 190],
  [170, 170, 170],
  [150, 150, 150],
  [40, 40, 40],
  [20, 20, 20],
  [10, 10, 10],
  [0, 0, 0],
].map(([red, green, blue]) =>
  new THREE.Vector3(red, green, blue).multiplyScalar(1 / 255));

const props = defineProps<{
  name: string

  /*
   * NOTE: This is needed as vue doesn't support watchers for Uint8Array
   * so we use the pointCloudUpdateCount as a signal that the pointcloud
   * has changed & needs to be re rendered.
   */
  pointCloudUpdateCount: number
  resources: commonApi.ResourceName.AsObject[]
  pointcloud?: Uint8Array
  pose?: commonApi.Pose
  destExists: boolean
  destVector: THREE.Vector3
  axesVisible: boolean
}
>();

const emit = defineEmits<{(event: 'click', point: THREE.Vector3): void}>();

const loader = new PCDLoader();

const container = $ref<HTMLElement>();

const { scene, renderer, canvas, run, pause, setCamera } = threeInstance();

const color = new THREE.Color(0xFF_FF_FF);
renderer.setClearColor(color, 1);

canvas.style.cssText = 'width:100%;height:100%;';

const camera = new THREE.OrthographicCamera(-1, 1, 0.5, -0.5, -1, 1000);
camera.userData.size = 2;
setCamera(camera);
scene.add(camera);

const controls = new MapControls(camera, canvas);
controls.enableRotate = false;

const raycaster = new MouseRaycaster({ camera, renderer, recursive: false });

raycaster.on('click', (event: THREE.Event) => {
  const [intersection] = event.intersections as THREE.Intersection[];
  if (intersection && intersection.point) {
    emit('click', intersection.point);
  }
});

const disposeScene = () => {
  scene.traverse((object) => {
    if (object.name === 'BaseMarker' || object.name === 'DestinationMarker') {
      return;
    }

    if (object instanceof THREE.Points || object instanceof THREE.Mesh) {
      object.geometry.dispose();

      if (object.material instanceof THREE.Material) {
        object.material.dispose();
      }
    }
  });

  scene.clear();
};

const updatePose = (newPose: commonApi.Pose) => {
  const x = newPose.getX();
  const y = newPose.getY();
  const z = newPose.getZ();
  baseMarker.position.set(x + baseMarkerOffset.x, y + baseMarkerOffset.y, z + baseMarkerOffset.z);
};

/*
 * Find the desired color bucket for a given probability. This assumes the probability will be a value from 0 to 100
 * ticket to add testing: https://viam.atlassian.net/browse/RSDK-2606
 */
const probToColorMapBucket = (probability: number, numBuckets: number): number => {
  const prob = Math.max(Math.min(100, probability * 255), 0);
  return Math.floor((numBuckets - 1) * prob / 100);
};

/*
 * Map the color of a pixel to a color bucket value.
 * probability represents the probability value normalized by the size of a byte(255) to be between 0 to 1.
 * ticket to add testing: https://viam.atlassian.net/browse/RSDK-2606
 */
const colorBuckets = (probability: number): THREE.Vector3 => {
  return colorMapGrey[probToColorMapBucket(probability, colorMapGrey.length)]!;
};

// create the x and z axes
const createAxisHelper = (name: string, rotation: number): THREE.AxesHelper => {
  const axesHelper = new THREE.AxesHelper(axesHelperSize);
  axesHelper.rotateX(rotation);
  axesHelper.scale.set(1e5, 1, 1e5);
  axesHelper.renderOrder = axesHelperRenderOrder;
  axesHelper.name = name;
  axesHelper.visible = props.axesVisible;
  return axesHelper;
};

// create the background gray grid
const createGridHelper = (boundingBox: THREE.Box3): THREE.GridHelper => {
  const deltaX = Math.abs(boundingBox.max.x - boundingBox.min.x);
  const deltaZ = Math.abs(boundingBox.max.z - boundingBox.min.z);
  let maxDelta = Math.round(Math.max(deltaX, deltaZ) * gridHelperScalar);

  /*
   * if maxDelta is an odd number the x y axes will not be layered neatly over the grey grid
   * because we round maxDelta and then potentially decrease the value, the 1 meter grid spacing
   * is bound to have a margin of error
   */
  if (maxDelta % 2 !== 0) {
    maxDelta -= 1;
  }

  const gridHelper = new THREE.GridHelper(maxDelta, maxDelta, backgroundGridColor, backgroundGridColor);
  gridHelper.renderOrder = gridHelperRenderOrder;
  gridHelper.name = 'Grid';
  gridHelper.visible = props.axesVisible;
  gridHelper.rotateX(Math.PI / 2);
  return gridHelper;
};

const updateOrRemoveDestinationMarker = () => {
  if (props.destVector && props.destExists) {

    destMarker.position.set(
      props.destVector.x + destinationMarkerOffset.x,
      props.destVector.y + destinationMarkerOffset.y,
      props.destVector.z + destinationMarkerOffset.z
    );
  }

  if (!props.destExists) {
    scene.remove(destMarker);
  }
};

const updatePointCloud = (pointcloud: Uint8Array) => {
  disposeScene();

  const viewHeight = 1;
  const viewWidth = viewHeight * 2;

  const points = loader.parse(pointcloud.buffer);
  const material = points.material as THREE.PointsMaterial;
  material.sizeAttenuation = false;
  material.size = 4;
  points.geometry.computeBoundingSphere();

  const { radius = 1, center = { x: 0, y: 0 } } = points.geometry.boundingSphere ?? {};
  camera.position.set(center.x, center.y, 100);
  camera.lookAt(center.x, center.y, 0);

  const aspect = canvas.clientHeight / canvas.clientWidth;
  camera.zoom = aspect > 1
    ? viewHeight / (radius * 2)
    : camera.zoom = viewWidth / (radius * 2);

  camera.updateProjectionMatrix();

  controls.target.set(center.x, center.y, 0);
  controls.maxZoom = radius * 2;

  const intersectionPlane = new THREE.Mesh(
    new THREE.PlaneGeometry(radius * 2, radius * 2, 1, 1),
    new MeshDiscardMaterial()
  );
  intersectionPlane.name = 'Intersection Plane';
  intersectionPlane.position.z = -1;
  intersectionPlane.position.set(center.x, center.y, 0);
  raycaster.objects = [intersectionPlane];

  const colors = points.geometry.attributes.color;
  // if the PCD has a color attribute defined, convert those colors using the colorMap
  if (colors instanceof THREE.BufferAttribute) {
    for (let i = 0; i < colors.count; i += 1) {

      /*
       * Probability is currently assumed to be held in the rgb field of the PCD map, on a scale of 0 to 100.
       * ticket to look into this further https://viam.atlassian.net/browse/RSDK-2605
       */
      const colorMapPoint = colorBuckets(colors.getZ(i));
      colors.setXYZ(i, colorMapPoint.x, colorMapPoint.y, colorMapPoint.z);
    }
  }

  points.geometry.computeBoundingBox();

  // construct grid spaced at 1 meter
  const gridHelper = createGridHelper(points.geometry.boundingBox!);

  // construct axes
  const axesPos = createAxisHelper('AxesPos', Math.PI / 2);
  axesPos.rotateY(Math.PI / 2);
  const axesNeg = createAxisHelper('AxesNeg', -Math.PI / 2);
  axesNeg.rotateY(Math.PI / 2);
  axesNeg.rotateX(Math.PI);

  // add objects to scene
  scene.add(
    gridHelper,
    points,
    intersectionPlane,
    axesPos,
    axesNeg
  );

  if (props.pose !== undefined) {
    updatePose(props.pose!);
  }

  updateOrRemoveDestinationMarker();
};

onMounted(() => {
  container?.append(canvas);

  run();

  if (props.pointcloud !== undefined) {
    updatePointCloud(props.pointcloud);
  }

  if (props.pose !== undefined) {
    updatePose(props.pose);
  }

});

onUnmounted(() => {
  pause();
  disposeScene();
});

watch(() => [props.destVector!.x, props.destVector!.y, props.destExists], updateOrRemoveDestinationMarker);

watch(() => props.axesVisible, () => {
  for (const gridPart of gridSubparts) {
    const part = scene.getObjectByName(gridPart);
    if (part !== undefined) {
      part.visible = props.axesVisible;
    }
  }
});

watch(() => props.pose, (newPose) => {
  if (newPose !== undefined) {
    try {
      updatePose(newPose);
    } catch (error) {
      console.error('failed to update pose', error);
    }
  }
});

watch(() => props.pointCloudUpdateCount, () => {
  if (props.pointcloud !== undefined) {
    try {
      updatePointCloud(props.pointcloud);
    } catch (error) {
      console.error('failed to update pointcloud', error);
    }
  }
});

</script>

<template>
  <div
    ref="container"
    class="relative w-full"
  >
    <p class="absolute left-3 top-3 bg-white text-xs">
      Grid set to 1 meter
    </p>
    <div class="absolute right-3 top-3">
      <svg
        class="Axes-Legend"
        width="30"
        height="30"
        viewBox="0 0 30 30"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <rect
          width="10"
          height="10"
          rx="5"
          fill="#BE3536"
        />
        <path
          d="M4.66278 6.032H4.51878L2.76678 2.4H3.51878L4.95078 5.456H5.04678L6.47878
             2.4H7.23078L5.47878 6.032H5.33478V8H4.66278V6.032Z"
          fill="#FCECEA"
        />
        <rect
          x="20"
          y="20"
          width="10"
          height="10"
          rx="5"
          fill="#0066CC"
        />
        <path
          d="M23.6708 22.4L24.9268 24.88H25.0708L26.3268 22.4H27.0628L25.6628 25.144V25.24L27.0628
             28H26.3268L25.0708 25.504H24.9268L23.6708 28H22.9348L24.3348 25.24V25.144L22.9348 22.4H23.6708Z"
          fill="#E1F3FF"
        />
        <rect
          x="4"
          y="9"
          width="2"
          height="17"
          fill="#BE3536"
        />
        <rect
          x="21"
          y="24"
          width="2"
          height="17"
          transform="rotate(90 21 24)"
          fill="#0066CC"
        />
        <rect
          x="0.5"
          y="20.5"
          width="9"
          height="9"
          rx="4.5"
          fill="#E0FAE3"
        />
        <rect
          x="0.5"
          y="20.5"
          width="9"
          height="9"
          rx="4.5"
          stroke="#3D7D3F"
        />
      </svg>
    </div>
  </div>
</template>

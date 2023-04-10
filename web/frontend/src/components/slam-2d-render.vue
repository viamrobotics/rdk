<script setup lang="ts">

import { $ref } from 'vue/macros';
import { threeInstance, MouseRaycaster, MeshDiscardMaterial } from 'trzy';
import { onMounted, onUnmounted, watch } from 'vue';
import * as THREE from 'three';
import { MapControls } from 'three/examples/jsm/controls/OrbitControls';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import type { commonApi } from '@viamrobotics/sdk';

/*
 * this color map is greyscale. The color map is being used map probability values of a PCD
 * into different color buckets provided by the color map.
 * generated with: https://grayscale.design/app
 */
const colorMapGrey = [
  [247, 247, 247],
  [239, 239, 239],
  [223, 223, 223],
  [202, 202, 202],
  [168, 168, 168],
  [135, 135, 135],
  [109, 109, 109],
  [95, 95, 95],
  [74, 74, 74],
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
}
>();

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

raycaster.on('click', (event) => {
  const [intersection] = event.intersections as THREE.Intersection[];
  if (intersection && intersection.point) {
    console.log(intersection.point);
  }
});

const markerSize = 0.5;
const marker = new THREE.Mesh(
  new THREE.PlaneGeometry(markerSize, markerSize).rotateX(-Math.PI / 2),
  new THREE.MeshBasicMaterial({ color: 'red' })
);
marker.name = 'Marker';
// This ensures the robot marker renders on top of the pointcloud data
marker.renderOrder = 999;

const disposeScene = () => {
  scene.traverse((object) => {
    if (object.name === 'Marker') {
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

const updateCloud = (pointcloud: Uint8Array) => {
  disposeScene();

  const viewHeight = 1;
  const viewWidth = viewHeight * 2;

  const points = loader.parse(pointcloud.buffer);
  points.geometry.computeBoundingSphere();

  const { radius = 1, center = { x: 0, z: 0 } } = points.geometry.boundingSphere ?? {};
  camera.position.set(center.x, 100, center.z);
  camera.lookAt(center.x, 0, center.z);

  const aspect = canvas.clientHeight / canvas.clientWidth;
  camera.zoom = aspect > 1
    ? viewHeight / (radius * 2)
    : camera.zoom = viewWidth / (radius * 2);

  camera.updateProjectionMatrix();

  controls.target.set(center.x, 0, center.z);
  controls.maxZoom = radius * 2;

  const intersectionPlane = new THREE.Mesh(
    new THREE.PlaneGeometry(radius * 2, radius * 2, 1, 1).rotateX(-Math.PI / 2),
    new MeshDiscardMaterial()
  );
  intersectionPlane.name = 'Intersection Plane';
  intersectionPlane.position.y = -1;
  intersectionPlane.position.set(center.x, 0, center.z);
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

  scene.add(points);
  scene.add(marker);
  scene.add(intersectionPlane);
};

const updatePose = (newPose: commonApi.Pose) => {
  const x = newPose.getX();
  const z = newPose.getZ();
  marker.position.setX(x);
  marker.position.setZ(z);
};

onMounted(() => {
  container?.append(canvas);

  run();

  if (props.pointcloud !== undefined) {
    updateCloud(props.pointcloud);
  }

  if (props.pose !== undefined) {
    updatePose(props.pose);
  }
});

onUnmounted(() => {
  pause();
  disposeScene();
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
      updateCloud(props.pointcloud);
    } catch (error) {
      console.error('failed to update pointcloud', error);
    }
  }
});

</script>

<template>
  <div class="flex flex-col gap-4">
    <div
      ref="container"
      class="pcd-container relative w-full border border-black"
    />
  </div>
</template>

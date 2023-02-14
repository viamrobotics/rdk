<script setup lang="ts">

import { threeInstance, resizeRendererToDisplaySize } from 'trzy';
// import { grpc } from '@improbable-eng/grpc-web';
import { onMounted, onUnmounted, watch } from 'vue';
import * as THREE from 'three';
import { MapControls } from 'three/examples/jsm/controls/OrbitControls';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import type { Client, commonApi } from '@viamrobotics/sdk';
import { ConnectionClosedError } from '@viamrobotics/rpc';
// import { Client, commonApi, slamApi } from '@viamrobotics/sdk';
// import { rcLogConditionally } from '../lib/log';
// import { displayError } from '../lib/error';
// import { mdiProjectorScreen } from '@mdi/js';

interface Props {
  name: string
  resources: commonApi.ResourceName.AsObject[]
  pointcloud?: Uint8Array
  pose?: commonApi.Pose
  client: Client
}

const props = defineProps<Props>();

const loader = new PCDLoader();

const container = $ref<HTMLElement>();

const { scene, renderer } = threeInstance();

const color = new THREE.Color(0x69_5E_5E);
renderer.setClearColor(color, 1);

renderer.domElement.style.cssText = 'width:100%;height:100%;';

const camera = new THREE.OrthographicCamera(-1, 1, 0.5, -0.5, -1, 1000);
camera.userData.size = 1.5;

const markerSize = 0.5;
const marker = new THREE.Mesh(
  new THREE.PlaneGeometry(markerSize, markerSize).rotateX(-Math.PI / 2),
  new THREE.MeshBasicMaterial({ color: 'red' })
);

const controls = new MapControls(camera, renderer.domElement);

const disposeScene = () => {
  scene.traverse((object) => {
    if (object instanceof THREE.Points) {
      object.geometry.dispose();

      if (object.material instanceof THREE.Material) {
        object.material.dispose();
      }
    }
  });

  scene.clear();
};

const update = (pointcloud: Uint8Array, pose: commonApi.Pose) => {
  console.log("update" + pointcloud.length)
  const points = loader.parse(pointcloud.buffer, '');

  points.rotateX(Math.PI / -2);
  points.rotateX(Math.PI / 2);
  points.rotateY(Math.PI / 2);

  const x = pose.getX!();
  const z = pose.getZ!();
  console.log(x);
  console.log(z);

  marker.translateX(x);
  marker.translateZ(z);
  marker.rotateX(Math.PI / -2);
  marker.rotateX(Math.PI / 2);
  marker.rotateY(Math.PI / 2);

  disposeScene();
  scene.add(marker);
  scene.add(points);
};

const init = (pointcloud: Uint8Array, pose: commonApi.Pose) => {
  console.log("init")
  update(pointcloud, pose);
};

onMounted(() => {
  container.append(renderer.domElement);

  camera.position.set(0, 100, 0);
  camera.lookAt(0, 0, 0);

  renderer.setAnimationLoop(() => {
    resizeRendererToDisplaySize(camera, renderer);

    renderer.render(scene, camera);
    controls.update();
  });

  console.log("mount")
  if (props.pointcloud && props.pose) {
    console.log("mount, not null")
    init(props.pointcloud, props.pose);
  }
});

onUnmounted(() => {
  renderer.setAnimationLoop(null);
  disposeScene();
});

watch(
  () => props.pointcloud,
  () => props.pose,
  (pointcloud?: Uint8Array, pose?: commonApi.Pose) => {
    console.lot("watch called");
    if (pointcloud && pose) {
      init(pointcloud, pose);
    }
  }
);

</script>

<template>
  <div
    ref="container"
    class="relative w-[400px] h-[400px]"
  />
</template>

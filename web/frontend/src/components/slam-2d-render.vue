<!-- eslint-disable require-atomic-updates -->
<script setup lang="ts">

import { threeInstance, resizeRendererToDisplaySize } from 'trzy';
import { onMounted, onUnmounted } from 'vue';
import * as THREE from 'three';
import { MapControls } from 'three/examples/jsm/controls/OrbitControls';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';

const loader = new PCDLoader();

const container = $ref<HTMLElement>();

const { scene, renderer } = threeInstance();

renderer.domElement.style.cssText = 'width:100%;height:100%';

const camera = new THREE.OrthographicCamera(-1, 1, 0.5, -0.5, -1, 1000);
camera.userData.size = 1.5;

const markerSize = 0.1;
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

const loadDummyPCD = async () => {
  // loader.parse() would likely be used instead with buffers returned by GRPC endpoints.
  const points = await loader.loadAsync('https://threejs.org/examples/models/pcd/binary/Zaghetto.pcd');
  // This dummy pcd example is in a different coordinate system
  points.rotateX(Math.PI / 2);

  disposeScene();

  scene.add(marker);
  scene.add(points);
};

onMounted(() => {
  container.append(renderer.domElement);

  camera.position.set(0, 1, 0);
  camera.lookAt(0, 0, 0);

  renderer.setAnimationLoop(() => {
    resizeRendererToDisplaySize(camera, renderer);

    renderer.render(scene, camera);
    controls.update();
  });

  // We're doing this every second to simulate refreshes.
  setInterval(() => loadDummyPCD(), 1000);
});

onUnmounted(() => {
  renderer.setAnimationLoop(null);
  disposeScene();
});

</script>

<template>
  <div
    ref="container"
    class="relative w-[400px] h-[400px]"
  />
</template>

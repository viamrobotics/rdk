<script lang='ts'>

import * as THREE from 'three'
import { T, useThrelte, useRender, type CurrentWritable, useFrame } from '@threlte/core';
import { MercatorCoordinate, type Map } from 'maplibre-gl';
import { onMount } from 'svelte';

export let map: Map;
export let matrix4: CurrentWritable<THREE.Matrix4>

const { renderer, scene, advance } = useThrelte();

renderer!.autoClear = false;

const camera = new THREE.OrthographicCamera();

const vec3 = new THREE.Vector3();
const rotationX = new THREE.Matrix4();
const rotationY = new THREE.Matrix4();
const rotationZ = new THREE.Matrix4();
const l = new THREE.Matrix4();

// parameters to ensure the model is georeferenced correctly on the map
const modelRotate = [Math.PI / 2, 0, 0] as const;
const modelAsMercatorCoordinate = MercatorCoordinate.fromLngLat([-74.5, 40], 0);

const modelTransform = {
  x: modelAsMercatorCoordinate.x,
  y: modelAsMercatorCoordinate.y,
  z: modelAsMercatorCoordinate.z,
  rx: modelRotate[0],
  ry: modelRotate[1],
  rz: modelRotate[2],

  /*
   * Since our 3D model is in real world meters, a scale transform needs to be
   * applied since the CustomLayerInterface expects units in MercatorCoordinates.
   */
  scale: modelAsMercatorCoordinate.meterInMercatorCoordinateUnits(),
} as const;

useRender(() => {
  console.log('render')
  rotationX.makeRotationAxis(vec3.set(1, 0, 0), modelTransform.rx);
  rotationY.makeRotationAxis(vec3.set(0, 1, 0), modelTransform.ry);
  rotationZ.makeRotationAxis(vec3.set(0, 0, 1), modelTransform.rz);

  l
    .makeTranslation(modelTransform.x, modelTransform.y, modelTransform.z)
    .scale(vec3.set(modelTransform.scale, -modelTransform.scale, modelTransform.scale))
    .multiply(rotationX)
    .multiply(rotationY)
    .multiply(rotationZ);

  camera.projectionMatrix = matrix4.current.multiply(l);
  renderer!.resetState();
  renderer!.render(scene, camera);
  map.triggerRepaint();
})

// map.on('drag', () => advance())
// map.on('zoom', () => advance())

const size = 10_000;

useFrame(() => {
  advance()
})

onMount(() => {
  advance();
})

</script>

<T.AmbientLight />
<T.DirectionalLight position={[1, 1, 1]} />

<T.Mesh>
  <T.BoxGeometry args={[size, size * 10, size]} />
  <T.MeshStandardMaterial color='red' />
</T.Mesh>

<!--
  @component
  Renders THREE.Points from a .pcd file.
  Creates an invisible plane mesh with dimensions matching the diameter of
  the points' bounding sphere.
  Emits click events that intersect this plane.
-->
<script lang='ts'>

import * as THREE from 'three';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import { T, useThrelte, createRawEventDispatcher, extend } from '@threlte/core';
import { MeshDiscardMaterial, MouseRaycaster } from 'trzy';
import { renderOrder } from './constants';
import { onMount } from 'svelte';

extend({ MeshDiscardMaterial });

/** A buffer representing a .pcd file */
export let pointcloud: Uint8Array | undefined;

/** The size of each individual point */
export let size: number;

type $$Events = {

  /** Dispatched when a user clicks within the bounding box of the pointcloud */
  click: THREE.Vector3

  /** Dispatched whenever a new .pcd file is parsed. Emits the radius and center of the cloud's bounding sphere. */
  update: {
    radius: number
    center: { x: number; y: number }
  }
}

const dispatch = createRawEventDispatcher<$$Events>();
const { camera, renderer } = useThrelte();
const loader = new PCDLoader();

let points: THREE.Points;
let material: THREE.PointsMaterial | undefined;
let radius = 1;
let center = { x: 0, y: 0 };

const raycaster = new MouseRaycaster({
  camera: camera.current as THREE.OrthographicCamera,
  target: renderer.domElement,
  recursive: false,
});

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

const update = (cloud: Uint8Array) => {
  points = loader.parse(cloud.buffer);
  material = points.material as THREE.PointsMaterial;
  material.sizeAttenuation = false;
  material.size = size;

  points.geometry.computeBoundingSphere();
  const { boundingSphere } = points.geometry;

  if (boundingSphere !== null) {
    radius = boundingSphere.radius;
    center = boundingSphere.center;
  }

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

  dispatch('update', { center, radius });
};

$: if (material) {
  material.size = size;
}
$: if (pointcloud) {
  update(pointcloud);
}
$: raycaster.camera = camera.current as THREE.OrthographicCamera;

onMount(() => dispatch('update', { center, radius }));

raycaster.addEventListener('click', (event: THREE.Event) => {
  const [intersection] = event.intersections as THREE.Intersection[];
  if (intersection && intersection.point) {
    dispatch('click', intersection.point);
  }
});

</script>

<T
  is={points}
  renderOrder={renderOrder.points}
  frustumCulled={false}
/>

<T.Mesh
  name='Intersection plane'
  position.x={center.x}
  position.y={center.y}
  on:create={({ ref }) => (raycaster.objects = [ref])}
>
  <T.PlaneGeometry args={[radius * 2, radius * 2, 1, 1]} />
  <T.MeshDiscardMaterial />
</T.Mesh>

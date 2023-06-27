<script lang="ts">

import { threeInstance, MouseRaycaster, MeshDiscardMaterial, GridHelper } from 'trzy';
import { onMount, onDestroy, createEventDispatcher } from 'svelte';
import * as THREE from 'three';
import { MapControls } from 'three/examples/jsm/controls/MapControls';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import type { commonApi } from '@viamrobotics/sdk';
import DestMarker from '@/lib/images/destination-marker.txt?raw';
import BaseMarker from '@/lib/images/base-marker.txt?raw';
import Legend from './2d-legend.svelte';

export let pointcloud: Uint8Array | undefined;
export let pose: commonApi.Pose | undefined;
export let destination: THREE.Vector2 | undefined;
export let axesVisible: boolean;

const dispatch = createEventDispatcher();

let points: THREE.Points | undefined;
let pointsMaterial: THREE.PointsMaterial | undefined;
let intersectionPlane: THREE.Mesh | undefined;

const markerColor = '#FF0047';
const backgroundGridColor = '#cacaca';

const svgMarkerRenderOrder = 4;
const pointsRenderOrder = 3;
const axesHelperRenderOrder = 2;
const gridHelperRenderOrder = 1;

const defaultPointScale = 13.2;
const aspectInverse = 4;
const initialPointSize = 4;
const baseSpriteSize = 0.05;
const axesHelperSize = 8;

const textureLoader = new THREE.TextureLoader();

const makeMarker = (png: string, name: string) => {
  const material = new THREE.SpriteMaterial({
    map: textureLoader.load(png),
    sizeAttenuation: false,
    color: markerColor,
  });
  const marker = new THREE.Sprite(material);
  marker.name = name;
  marker.renderOrder = svgMarkerRenderOrder;
  return marker;
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

const loader = new PCDLoader();

let container: HTMLElement;

const { scene, renderer, canvas, start, stop, setCamera, update } = threeInstance({
  parameters: {
    antialias: true,
  },
  autostart: false,
});

renderer.setClearColor('white', 1);

canvas.style.cssText = 'width:100%;height:100%;';

const camera = new THREE.OrthographicCamera();
camera.near = -1000;
camera.far = 1000;
camera.userData.size = 1;
setCamera(camera);
scene.add(camera);

const baseMarker = makeMarker(BaseMarker, 'BaseMarker');
const destMarker = makeMarker(DestMarker, 'DestinationMarker');
destMarker.visible = false;
destMarker.center.set(0.5, 0.05);

let userControlling = false;

const controls = new MapControls(camera, canvas);
controls.enableRotate = false;
controls.screenSpacePanning = true;

const raycaster = new MouseRaycaster({ camera, renderer, recursive: false });

raycaster.on('click', (event: THREE.Event) => {
  const [intersection] = event.intersections as THREE.Intersection[];
  if (intersection && intersection.point) {
    dispatch('click', intersection.point);
  }
});

const dispose = (object?: THREE.Object3D) => {
  if (!object) {
    return;
  }

  scene.remove(object);

  if (object instanceof THREE.Points || object instanceof THREE.Mesh) {
    object.geometry.dispose();

    if (object.material instanceof THREE.Material) {
      object.material.dispose();
    }
  }
};

const updatePose = (newPose: commonApi.Pose) => {
  const x = newPose.getX();
  const y = newPose.getY();
  const z = newPose.getZ();

  baseMarker.position.set(x, y, z);

  const theta = THREE.MathUtils.degToRad(newPose.getTheta() - 90);
  baseMarker.material.rotation = theta;
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
const createAxisHelper = (name: string, rotateX = 0, rotateY = 0): THREE.AxesHelper => {
  const axesHelper = new THREE.AxesHelper(axesHelperSize);
  axesHelper.rotateX(rotateX);
  axesHelper.rotateY(rotateY);
  axesHelper.scale.set(1e5, 1, 1e5);
  axesHelper.renderOrder = axesHelperRenderOrder;
  axesHelper.name = name;
  axesHelper.visible = axesVisible;
  return axesHelper;
};

// create the background gray grid
const createGridHelper = (): GridHelper => {
  const gridHelper = new GridHelper(1, 10, backgroundGridColor);
  gridHelper.renderOrder = gridHelperRenderOrder;
  gridHelper.name = 'Grid';
  gridHelper.visible = axesVisible;
  gridHelper.rotateX(Math.PI / 2);
  return gridHelper;
};

const handleUserControl = () => {
  userControlling = true;
  controls.removeEventListener('start', handleUserControl);
};

// construct grid spaced at 1 meter
const gridHelper = createGridHelper();

// construct axes
const axesPos = createAxisHelper('AxesPos', Math.PI / 2, Math.PI / 2);
const axesNeg = createAxisHelper('AxesNeg', -Math.PI / 2, Math.PI / 2);
axesNeg.rotateX(Math.PI);

const updatePointCloud = (cloud: Uint8Array) => {
  dispose(points);
  dispose(intersectionPlane);

  points = loader.parse(cloud.buffer);
  pointsMaterial = points.material as THREE.PointsMaterial;
  pointsMaterial.sizeAttenuation = false;
  pointsMaterial.size = initialPointSize;
  points.geometry.computeBoundingSphere();
  points.renderOrder = pointsRenderOrder;

  const { radius = 1, center = { x: 0, y: 0 } } = points.geometry.boundingSphere ?? {};

  if (!userControlling) {
    camera.position.set(center.x, center.y, 100);
    controls.target.set(center.x, center.y, 0);
    camera.lookAt(center.x, center.y, 0);

    const viewHeight = 1;
    const viewWidth = viewHeight * 2;
    const aspect = canvas.clientHeight / canvas.clientWidth;

    camera.zoom = aspect > 1
      ? viewHeight / (radius * aspectInverse)
      : viewWidth / (radius * aspectInverse);
  }

  controls.maxZoom = radius * 2;

  intersectionPlane = new THREE.Mesh(
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

  // add objects to scene
  scene.add(
    points,
    intersectionPlane
  );
};

let removeUpdate: (() => void) | undefined;

const scaleObjects = () => {
  const { zoom } = camera;

  if (pointsMaterial) {
    pointsMaterial.size = zoom * defaultPointScale * window.devicePixelRatio;
  }

  const spriteSize = baseSpriteSize / zoom;
  baseMarker.scale.set(spriteSize, spriteSize, 1);
  destMarker.scale.set(spriteSize, spriteSize, 1);
};

onMount(() => {
  removeUpdate = update(scaleObjects);
  container?.append(canvas);

  scene.add(
    gridHelper,
    axesPos,
    axesNeg,
    baseMarker,
    destMarker
  );

  controls.addEventListener('start', handleUserControl);

  start();

  if (pointcloud !== undefined) {
    updatePointCloud(pointcloud);
  }

  if (pose !== undefined) {
    updatePose(pose);
  }
});

onDestroy(() => {
  stop();
  dispose(points);
  dispose(intersectionPlane);
  removeUpdate?.();

  controls.removeEventListener('start', handleUserControl);
  userControlling = false;
});

$: {
  if (destination) {
    destMarker.visible = true;
    destMarker.position.set(destination.x, destination.y, 0);
  } else {
    destMarker.visible = false;
  }
}

$: {
  axesPos.visible = axesVisible;
  axesNeg.visible = axesVisible;
  gridHelper.visible = axesVisible;
}

$: {
  if (pose !== undefined) {
    try {
      updatePose(pose);
    } catch (error) {
      console.error('failed to update pose', error);
    }
  }
}

$: {
  if (pointcloud !== undefined) {
    try {
      updatePointCloud(pointcloud);
    } catch (error) {
      console.error('failed to update pointcloud', error);
    }
  }
}

</script>

<div
  bind:this={container}
  class="relative w-full"
>
  {#if axesVisible}
    <p class="absolute left-3 top-3 bg-white text-xs">
      Grid set to 1 meter
    </p>
  {/if}

  <div class="absolute right-3 top-3">
    <Legend />
  </div>
</div>

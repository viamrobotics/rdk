<script setup lang="ts">

import { $ref } from 'vue/macros';
import { threeInstance, MouseRaycaster, MeshDiscardMaterial } from 'trzy';
import { onMounted, onUnmounted, watch } from 'vue';
import * as THREE from 'three';
import { MapControls } from 'three/examples/jsm/controls/OrbitControls';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import type { commonApi } from '@viamrobotics/sdk';
import { SVGLoader } from 'three/examples/jsm/loaders/SVGLoader';
import { baseUrl } from '../lib/base-url'
import { destUrl } from '../lib/destination-url'

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
  destExists?: boolean
  destVector?: THREE.Vector3
  axes?: boolean // axesVisible rename todo
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

const guiData = {
    drawFillShapes: true,
    drawStrokes: true,
    fillShapesWireframe: false,
    strokesWireframe: false,
  };
/*
 * svgLoader example for webgl:
 * https://github.com/mrdoob/three.js/blob/master/examples/webgl_loader_svg.html
 */
// 
const makeMarker = async (url : string, name: string, scalar: number) => {
  const svgLoader = new SVGLoader();
  const data = await svgLoader.loadAsync(url);

  const { paths } = data!;

  const group = new THREE.Group();
  group.scale.multiplyScalar(scalar);
  // group.position.set(-70, 0, 70) // why do we do this? - do we need it?
  // group.scale.y *= -1; // why do we do this? - do we need it?

  for (const path of paths) {

    const fillColor = path!.userData!.style.fill;

    if (guiData.drawFillShapes && fillColor !== undefined && fillColor !== 'none') {
      const material = new THREE.MeshBasicMaterial({
        color: new THREE.Color().setStyle(fillColor)
          .convertSRGBToLinear(),
        opacity: path!.userData!.style.fillOpacity,
        transparent: true,
        side: THREE.DoubleSide,
        depthWrite: false,
        wireframe: guiData.fillShapesWireframe,
      });

      const shapes = SVGLoader.createShapes(path!);

      for (const shape of shapes) {

        const geometry = new THREE.ShapeGeometry(shape);
        const mesh = new THREE.Mesh(geometry.rotateX(-Math.PI / 2).rotateY(Math.PI), material);
        group.add(mesh);

      }

    }

    const strokeColor = path!.userData!.style.stroke;

    if (guiData.drawStrokes && strokeColor !== undefined && strokeColor !== 'none') {

      const material = new THREE.MeshBasicMaterial({
        color: new THREE.Color().setStyle(strokeColor)
          .convertSRGBToLinear(),
        opacity: path!.userData!.style.strokeOpacity,
        transparent: true,
        side: THREE.DoubleSide,
        depthWrite: false,
        wireframe: guiData.strokesWireframe,
      });

      for (let j = 0, jl = path!.subPaths.length; j < jl; j += 1) {
        const subPath = path!.subPaths[j];
        const geometry = SVGLoader.pointsToStroke(subPath.getPoints(), path!.userData!.style);

        if (geometry) {
          const mesh = new THREE.Mesh(geometry.rotateX(-Math.PI / 2).rotateY(Math.PI), material);
          group.add(mesh);
        }
      }
    }
  }

  group.name = name;
  scene.add(group);
  return group;
};

const disposeScene = () => {
  scene.traverse((object) => {
    if (object.name === 'Base' || object.name === 'Marker') {
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

const updatePose = async (newPose: commonApi.Pose) => {
  const x = newPose.getX();
  const z = newPose.getZ();
  const baseMarker = scene.getObjectByName('Base') ?? await makeMarker(baseUrl, 'Base', 0.04);
  baseMarker.position.set(x + 0.35, 0, z - 0.55);
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


// rename to updatePC
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

  // construct grids
  const axesHelper1 = new THREE.AxesHelper(5);
  axesHelper1.position.set(center.x, 0, center.z);
  axesHelper1.rotateY(Math.PI / 2);
  axesHelper1.scale.x = 1e5;
  axesHelper1.scale.z = 1e5;
  axesHelper1.renderOrder = 998;
  axesHelper1.name = 'Axes1';
  axesHelper1.visible = props.axes;

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
  scene.add(intersectionPlane);


  // have these axes be drawn at 0,0
  const axesHelper2 = new THREE.AxesHelper(5);
  axesHelper2.position.set(center.x, 0, center.z);
  axesHelper2.rotateY(-Math.PI / 2);
  axesHelper2.scale.x = 1e5;
  axesHelper2.scale.z = 1e5;
  axesHelper2.renderOrder = 997;
  axesHelper2.name = 'Axes2';
  axesHelper1.visible = props.axes;

  // this needs to be updated so it is set to 1m
  // have these be constants at the top
  const gridHelper = new THREE.GridHelper(1000, 100, 0xCA_CA_CA, 0xCA_CA_CA);
  gridHelper.position.set(center.x, 0, center.z);
  gridHelper.renderOrder = 996;
  gridHelper.name = 'Grid';
  gridHelper.visible = props.axes;

  // add objects to scene
  scene.add(axesHelper1);
  scene.add(axesHelper2);
  scene.add(gridHelper);

  scene.add(points);
  scene.add(intersectionPlane);
  updatePose(props.pose!); // only do this if the pose exists
  // what is pc didnt change but the pose did
  // then we need this conditional
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

// see if we can just do props.destVector and not the subcomponents
watch(() => [props.destVector?.x, props.destVector?.y, props.destExists], async () => {
  if (props.destVector && props.destExists) {
    // update name from marker to destinationMarker
    const marker = scene.getObjectByName('Marker') ?? await makeMarker(destUrl, 'Marker', 0.1);
    // udpate from base to baseMarker
    const base = scene.getObjectByName('Base');
    // get rid of this magic number formulation
    const convertedCoord = (base!.position.z + 0.55) - props.destVector.y;
    marker.position.set(props.destVector.x + 1.2, 0, convertedCoord - 2.54);
  }
  if (!props.destExists) {
    const marker = scene.getObjectByName('Marker');
    if (marker !== undefined) {
      scene.remove(marker);
    }
  }
});

watch(() => props.axes, () => {
  //rename to xy pos
  const ax1 = scene.getObjectByName('Axes1');
  if (ax1 !== undefined) {
    ax1.visible = props.axes;
  }

  // rename to x,y negative
  const ax2 = scene.getObjectByName('Axes2');
  if (ax2 !== undefined) {
    ax2.visible = props.axes;
  }

  const grid = scene.getObjectByName('Grid');
  if (grid !== undefined) {
    grid.visible = props.axes;
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
      updateCloud(props.pointcloud);
    } catch (error) {
      console.error('failed to update pointcloud', error);
    }
  }
});

</script>

<template>
  <div
    ref="container"
    class="pcd-container relative w-full"
  >
    <p class="absolute left-3 top-3 bg-white text-xs">
      Grid set to 1 meter
    </p>
    <div class="absolute right-3 top-3">
      <svg
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

<script setup lang="ts">

import { $ref } from 'vue/macros';
import { threeInstance, MouseRaycaster, MeshDiscardMaterial } from 'trzy';
import { onMounted, onUnmounted, watch } from 'vue';
import * as THREE from 'three';
import { MapControls } from 'three/examples/jsm/controls/OrbitControls';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import type { commonApi } from '@viamrobotics/sdk';
import { SVGLoader } from 'three/examples/jsm/loaders/SVGLoader';

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
  axes?: boolean
}
>();

const baseUrl = `data:image/svg+xml,%3Csvg width='31' height='39' viewBox='0 0 31 39' fill='none' 
                xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M3.55907 4.21112L1.70584 2.78033L1.95357
                5.10847L4.71496 31.0594L5.08708 34.5565L6.61063 31.3869L11.7355 20.7249L23.5021 21.9458L27.0001
                22.3088L24.2164 20.1596L3.55907 4.21112Z' fill='%23BE3536' stroke='%23FCECEA'
                stroke-width='2'/%3E%3C/svg%3E`;
const destUrl = `data:image/svg+xml,%3Csvg width='24' height='24' viewBox='0 0 24 24' fill='none' 
                xmlns='http://www.w3.org/2000/svg'%3E%3Cg clip-path='url(%23clip0_2995_26851)'%3E%3Cpath
                d='M12 22L11.2579 22.6703L12 23.4919L12.7421 22.6703L12 22ZM12 22C12.7421 22.6703 12.7422 22.6701 
                12.7424 22.67L12.7429 22.6694L12.7443 22.6679L12.749 22.6627L12.7658 22.6439C12.7803 22.6277 
                12.8012 22.6042 12.8281 22.5737C12.8819 22.5127 12.9599 22.4238 13.0584 22.3094C13.2554 22.0808 
                13.5352 21.75 13.8702 21.3372C14.5393 20.5127 15.4332 19.3558 16.329 18.0281C17.2228 16.7032 
                18.1311 15.1902 18.8189 13.6549C19.5008 12.1328 20 10.5148 20 9C20 4.57772 16.4223 1 12 1C7.57772 
                1 4 4.57772 4 9C4 10.5148 4.49923 12.1328 5.18115 13.6549C5.86894 15.1902 6.77715 16.7032 7.67104 
                18.0281C8.56684 19.3558 9.46066 20.5127 10.1298 21.3372C10.4648 21.75 10.7446 22.0808 10.9416 
                22.3094C11.0401 22.4238 11.1181 22.5127 11.1719 22.5737C11.1988 22.6042 11.2197 22.6277 11.2342 
                22.6439L11.251 22.6627L11.2557 22.6679L11.2571 22.6694L11.2576 22.67C11.2578 22.6701 11.2579 
                22.6703 12 22ZM8 9C8 6.79228 9.79228 5 12 5C14.2077 5 16 6.79228 16 9C16 10.169 15.3927 11.7756 
                14.4208 13.5258C13.7032 14.818 12.8343 16.106 12.004 17.2276C11.1827 16.1085 10.3144 14.8168 
                9.59393 13.5214C8.61458 11.7606 8 10.1512 8 9Z' fill='%23BE3536' stroke='%23FCECEA' 
                stroke-width='2'/%3E%3Cpath d='M12 12.5C13.933 12.5 15.5 10.933 15.5 9C15.5 7.067 13.933 5.5 12 
                5.5C10.067 5.5 8.5 7.067 8.5 9C8.5 10.933 10.067 12.5 12 12.5Z' fill='%23BE3536' 
                stroke='%23FCECEA' stroke-width='2'/%3E%3C/g%3E%3Cdefs%3E%3CclipPath 
                id='clip0_2995_26851'%3E%3Crect width='24' height='24' 
                fill='white'/%3E%3C/clipPath%3E%3C/defs%3E%3C/svg%3E`;

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

raycaster.on('click', (event) => {
  const [intersection] = event.intersections as THREE.Intersection[];
  if (intersection && intersection.point) {
    emit('click', intersection.point);
  }
});

const makeMarker = async (url : string, name: string, scalar: number) => {
  const guiData = {
    currentURL: url,
    drawFillShapes: true,
    drawStrokes: true,
    fillShapesWireframe: false,
    strokesWireframe: false,
  };

  const svgLoader = new SVGLoader();
  const data = await svgLoader.loadAsync(guiData.currentURL);

  const { paths } = data!;

  const group = new THREE.Group();
  group.scale.multiplyScalar(scalar);
  group.position.x = -70;
  group.position.y = 70;
  group.scale.y *= -1;

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

  if (props.destExists) {
    const destX = props.destVector!.x;
    const destZ = props.destVector!.y;
    const destMarker = scene.getObjectByName('Marker') ?? await makeMarker(destUrl, 'Marker', 0.1);
    destMarker.position.set(destX + 0.6, 0, destZ - 1.24);
  }
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

  // construct grids
  const axesHelper1 = new THREE.AxesHelper(5);
  axesHelper1.position.set(center.x, 0, center.z);
  axesHelper1.rotateY(Math.PI / 2);
  axesHelper1.scale.x = 1e5;
  axesHelper1.scale.z = 1e5;
  axesHelper1.renderOrder = 998;
  axesHelper1.name = 'Axes1';
  axesHelper1.visible = props.axes;

  const axesHelper2 = new THREE.AxesHelper(5);
  axesHelper2.position.set(center.x, 0, center.z);
  axesHelper2.rotateY(-Math.PI / 2);
  axesHelper2.scale.x = 1e5;
  axesHelper2.scale.z = 1e5;
  axesHelper2.renderOrder = 997;
  axesHelper2.name = 'Axes2';
  axesHelper1.visible = props.axes;

  // this needs to be updated so it is set to 1m
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
  updatePose(props.pose!);
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

watch(() => [props.destVector?.x, props.destVector?.z, props.destExists], async () => {
  if (props.destVector && props.destExists) {
    const marker = scene.getObjectByName('Marker') ?? await makeMarker(destUrl, 'Marker', 0.1);
    marker.position.set(props.destVector.x + 1.2, 0, props.destVector.z - 2.44);
  }
  if (!props.destExists) {
    const marker = scene.getObjectByName('Marker');
    if (marker !== undefined) {
      scene.remove(marker);
    }
  }
});

watch(() => props.axes, () => {
  const ax1 = scene.getObjectByName('Axes1');
  if (ax1 !== undefined) {
    ax1.visible = props.axes;
  }

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

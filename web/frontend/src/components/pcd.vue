<script setup lang="ts">

/**
 * @TODO: No disposing of THREE resources currently.
 * This is causing memory leaks.
 */

import { grpc } from '@improbable-eng/grpc-web';
import { onMounted, onUnmounted, watch } from 'vue';
import * as THREE from 'three';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import { filterResources, type Resource } from '../lib/resource';
import { toast } from '../lib/toast';
import type { PointCloudObject, RectangularPrism } from '../gen/proto/api/common/v1/common_pb';
import cameraApi from '../gen/proto/api/component/camera/v1/camera_pb.esm';
import motionApi from '../gen/proto/api/service/motion/v1/motion_pb.esm';
import commonApi from '../gen/proto/api/common/v1/common_pb.esm';
import visionApi, { type TypedParameter } from '../gen/proto/api/service/vision/v1/vision_pb.esm';
import InfoButton from './info-button.vue';

interface Props {
  resources: Resource[]
  pointcloud: string
  cameraName: string
}

const props = defineProps<Props>();

const container = $ref<HTMLDivElement>();

let cube: THREE.LineSegments;
let pointsMaterial: THREE.PointsMaterial;

let segmenterParameterNames = $ref<TypedParameter[]>();
let objects = $ref<PointCloudObject[]>([]);
let segmenterNames = $ref<string[]>([]);
let segmenterParameters = $ref<Record<string, number>>({});
let calculatingSegments = $ref(false);
let url = $ref('');
let pointsSize = $ref(1);
const click = $ref(new THREE.Vector3());

const selectedObject = $ref('');
const selectedSegmenter = $ref('');

const distanceFromCamera = $computed(() => Math.round(
  Math.sqrt(
    Math.pow(click.x, 2) + Math.pow(click.y, 2) + Math.pow(click.z, 2)
  )
) || 0);

const loader = new PCDLoader();
const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(
  75, window.innerWidth / window.innerHeight, 0.01, 2000
);
const renderer = new THREE.WebGLRenderer({
  powerPreference: 'high-performance',
  antialias: true,
});
renderer.setClearColor('white');
const raycaster = new THREE.Raycaster();
raycaster.params.Points.threshold = 0.1;
const controls = new OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;

const sphereGeometry = new THREE.SphereGeometry(0.009, 32, 32);
const sphereMaterial = new THREE.MeshBasicMaterial({ color: 0xFF_00_00 });
const sphere = new THREE.Mesh(sphereGeometry, sphereMaterial);

const size = 10;
const divisions = 10;
const gridHelper = new THREE.GridHelper(size, divisions);
gridHelper.material.color.set('black');

const renderPCD = () => {
  const request = new cameraApi.GetPointCloudRequest();
  request.setName(props.cameraName);
  request.setMimeType('pointcloud/pcd');
  window.cameraService.getPointCloud(request, new grpc.Metadata(), (error, response) => {
    if (error) {
      toast.error(`Error getting point cloud: ${error}`);
      return;
    }

    update(response!.getPointCloud_asB64());
  });

  getSegmenterNames();
};

const getSegmenterNames = () => {
  const request = new visionApi.GetSegmenterNamesRequest();
  // We are deliberately just getting the first vision service to ensure this will not break.
  // May want to allow for more services in the future
  const [visionName] = filterResources(props.resources, 'rdk', 'service', 'vision');

  request.setName(visionName.name);

  window.visionService.getSegmenterNames(request, new grpc.Metadata(), (error, response) => {
    if (error) {
      toast.error(`Error getting segmenter names: ${error}`);
      return;
    }

    segmenterNames = response!.getSegmenterNamesList();
  });
};

const getSegmenterParameters = () => {
  const req = new visionApi.GetSegmenterParametersRequest();
  // We are deliberately just getting the first vision service to ensure this will not break.
  // May want to allow for more services in the future
  const visionName = filterResources(props.resources, 'rdk', 'service', 'vision')[0];

  req.setName(visionName.name);
  req.setSegmenterName(selectedSegmenter);
  
  window.visionService.getSegmenterParameters(req, new grpc.Metadata(), (error, response) => {
    if (error) {
      toast.error(`Error getting segmenter parameters: ${error}`);
      return;
    }

    segmenterParameterNames = response!.getSegmenterParametersList();
    segmenterParameters = {};
  });
};

const animate = () => {
  resizeRendererToDisplaySize();
  renderer.render(scene, camera);
  controls.update();
};

const resizeRendererToDisplaySize = () => {
  const canvas = renderer.domElement;
  const pixelRatio = window.devicePixelRatio;
  const width = Math.trunc(canvas.clientWidth * pixelRatio);
  const height = Math.trunc(canvas.clientHeight * pixelRatio);
  const needResize = canvas.width !== width || canvas.height !== height;

  if (needResize) {
    renderer.setSize(width, height, false);
    camera.aspect = canvas.clientWidth / canvas.clientHeight;
    camera.updateProjectionMatrix();
  }
};

const setPoint = (point: THREE.Vector3) => {
  click.x = Math.round(point.x * 1000);
  click.y = Math.round(point.y * 1000);
  click.z = Math.round(point.z * 1000);
  sphere.position.copy(point);
};

const parameterType = (type: string) => {
  if (type === 'int' || type === 'float64') {
    return 'number';
  } else if (type === 'string' || type === 'char') {
    return 'text';
  }
  return '';
};

const findSegments = () => {
  calculatingSegments = true;
  const req = new visionApi.GetObjectPointCloudsRequest();

  // We are deliberately just getting the first vision service to ensure this will not break.
  // May want to allow for more services in the future
  const visionName = filterResources(props.resources, 'rdk', 'service', 'vision')[0];
  
  req.setName(visionName.name);
  req.setCameraName(props.cameraName);
  req.setSegmenterName(selectedSegmenter);
  req.setParameters(Struct.fromJavaScript(segmenterParameters));
  req.setMimeType('pointcloud/pcd');

  window.visionService.getObjectPointClouds(req, new grpc.Metadata(), (error, response) => {
    if (error) {
      toast.error(`Error getting segments: ${error}`);
      calculatingSegments = false;
      return;
    }

    objects = response!.getObjectsList();
    calculatingSegments = false;
  });
};

const loadSegment = (index: string) => {
  console.log(objects, index)
  const segment = objects[index]!;
  const pointcloud = segment.getPointCloud_asB64();
  const center = segment.getGeometries()!.getGeometriesList()[0].getCenter()!;
  const box = segment.getGeometries()!.getGeometriesList()[0].getBox()!;

  const point = new THREE.Vector3(
    center.getX(),
    center.getY(),
    center.getZ()
  ).multiplyScalar(1 / 1000);

  setPoint(point);

  setBoundingBox(box, point);
  update(pointcloud);
};

const loadBoundingBox = (index: string) => {
  const segment = objects[index];
  const center = segment.getGeometries().getGeometriesList()[0].getCenter()!;
  const box = segment.getGeometries().getGeometriesList()[0].getBox();

  const point = new THREE.Vector3(
    center.getX(),
    center.getY(),
    center.getZ()
  ).multiplyScalar(1 / 1000);

  setBoundingBox(box, point);
};

const loadPoint = (index: string) => {
  const segment = objects[index];
  const center = segment.getGeometries().getGeometriesList()[0].getCenter()!;
  
  const point = new THREE.Vector3(
    center.getX(),
    center.getY(),
    center.getZ()
  ).multiplyScalar(1 / 1000);

  setPoint(point);
};

const setBoundingBox = (box: RectangularPrism, centerPoint: THREE.Vector3) => {
  const dimensions = box.getDimsMm()!;
  const geometry = new THREE.BoxGeometry(
    dimensions.getX() / 1000,
    dimensions.getY() / 1000,
    dimensions.getZ() / 1000
  );
  const edges = new THREE.EdgesGeometry(geometry);
  const material = new THREE.LineBasicMaterial({ color: 0xFF_00_00 });
  const lineSegments = new THREE.LineSegments(edges, material);
  lineSegments.position.copy(centerPoint);
  lineSegments.name = 'bounding-box';

  const previousBox = scene.getObjectByName('bounding-box')!;
  if (previousBox) {
    scene.remove(previousBox);
  }

  cube = lineSegments;
  scene.add(cube);
};

const getCanvasRelativePosition = (event: MouseEvent) => {
  const canvas = renderer.domElement;
  const rect = canvas.getBoundingClientRect();
  return {
    x: (event.clientX - rect.left) * canvas.width / rect.width,
    y: (event.clientY - rect.top) * canvas.height / rect.height,
  };
};

const handleClick = (event: MouseEvent) => {
  const { x, y } = getCanvasRelativePosition(event);
  const mouse = new THREE.Vector2(x, y);

  console.log(x, y)

  raycaster.setFromCamera(mouse, camera);

  const [intersect] = raycaster.intersectObjects([scene.getObjectByName('points')!]);

  if (intersect) {
    console.log(intersect);
    setPoint(intersect.point);
  } else {
    toast.info('No point intersected.');
  }
};

const handleMove = () => {
  const gripperName = filterResources(props.resources, 'rdk', 'component', 'gripper')[0];
  const cameraName = props.cameraName;
  const cameraPointX = click.x;
  const cameraPointY = click.y;
  const cameraPointZ = click.z;

  const req = new motionApi.MoveRequest();
  const cameraPoint = new commonApi.Pose();
  // We are deliberately just getting the first motion service to ensure this will not break.
  // May want to allow for more services in the future
  const motionName = filterResources(props.resources, 'rdk', 'service', 'motion')[0];
  cameraPoint.setX(cameraPointX);
  cameraPoint.setY(cameraPointY);
  cameraPoint.setZ(cameraPointZ);

  const pose = new commonApi.PoseInFrame();
  pose.setReferenceFrame(cameraName);
  pose.setPose(cameraPoint);
  req.setDestination(pose);
  req.setName(motionName.name);
  const componentName = new commonApi.ResourceName();
  componentName.setNamespace(gripperName.namespace);
  componentName.setType(gripperName.type);
  componentName.setSubtype(gripperName.subtype);
  componentName.setName(gripperName.name);
  req.setComponentName(componentName);

  window.motionService.move(req, new grpc.Metadata(), (error, response) => {
    if (error) {
      toast.error(`Error moving: ${error}`);
      return;
    }

    toast.success(`Move success: ${response!.getSuccess()}`);
  });
};

const handleCenter = () => {
  setPoint(new THREE.Vector3());
};

const handleSelectObject = (selection: string) => {
  switch (selection) {
    case 'Center Point':
      return loadSegment(selectedObject);
    case 'Bounding Box':
      return loadBoundingBox(selectedObject);
    case 'Cropped':
      return loadPoint(selectedObject);
  }
};

const handleDownload = () => {
  window.open(url);
};

const handlePointsResize = (event: CustomEvent) => {
  console.log(event.detail.value)
  pointsSize = event.detail.value;
  pointsMaterial.size = pointsSize;
  pointsMaterial.needsUpdate = true;
};

const update = async (cloud: string) => {
  console.log(atob(cloud));
  if (!cloud) {
    return;
  }

  url = `data:pointcloud/pcd;base64,${cloud}`;
  const points = await loader.loadAsync(url);
  points.name = 'points';
  
  pointsMaterial = points.material as THREE.PointsMaterial;
  pointsMaterial.size = pointsSize;
  pointsMaterial.vertexColors = true;

  scene.clear();
  scene.add(points);
  camera.position.set(0.5, 0.5, 1);
  camera.lookAt(0, 0, 0);

  scene.add(sphere);
  scene.add(gridHelper);

  if (cube) {
    scene.add(cube);
  }

  animate();
};

onMounted(() => {
  container.append(renderer.domElement);
  renderer.setAnimationLoop(animate);
  renderPCD();
});

onUnmounted(() => {
  renderer.setAnimationLoop(null);
});

watch(() => props.pointcloud, (updated: string) => {
  update(updated);
});

</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex justify-end gap-2">
      <v-button
        icon="center"
        label="Center"
        @click="handleCenter"
      />
      <v-button
        icon="download"
        label="Download Raw Data"
        @click="handleDownload"
      />
    </div>

    <div
      ref="container"
      class="pcd-container relative w-full border border-black"
      @click="handleClick"
    />

    <div class="flex items-center gap-1 whitespace-nowrap">
      <span class="text-xs">Controls</span>
      <InfoButton
        :info-rows="[
          'Rotate - Left/Click + Drag',
          'Pan - Right/Two Finger Click + Drag',
          'Zoom - Wheel/Two Finger Scroll',
        ]"
      />
    </div>

    <div class="flex items-center gap-1 whitespace-nowrap w-36 relative">
      <v-slider
        label="Points Size"
        min="1"
        value="1"
        max="10"
        step="0.1"
        @input="handlePointsResize"
      />
    </div>
  
    <div class="clear-both grid grid-cols-1 divide-y">
      <div>
        <div class="container mx-auto pt-4">
          <div>
            <h2>Segmentation Settings</h2>
            <div class="relative">
              <select
                v-model="selectedSegmenter"
                class="m-0 w-full appearance-none border border-solid border-black bg-white bg-clip-padding px-3 py-1.5 text-xs font-normal text-gray-700 focus:outline-none"
                aria-label="Select segmenter"
                @change="getSegmenterParameters"
              >
                <option
                  value=""
                  selected
                  disabled
                >
                  Choose
                </option>
                <option
                  v-for="segmenter in segmenterNames"
                  :key="segmenter"
                  :value="segmenter"
                >
                  {{ segmenter }}
                </option>
              </select>
              <div
                class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2"
              >
                <svg
                  class="h-4 w-4 stroke-2 text-gray-700"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-linejoin="round"
                  stroke-linecap="round"
                  fill="none"
                >
                  <path d="M18 16L12 22L6 16" />
                </svg>
              </div>
            </div>
            <div class="row flex">
              <div
                v-for="param in segmenterParameterNames"
                :key="param.getName()"
                class="column w-1/3 flex-auto pr-2"
              >
                <v-input
                  :id="param.getName()"
                  :type="parameterType(param.getType())"
                  :label="param.getName()"
                  :value="segmenterParameters[param.getName()]"
                  @input="(event: CustomEvent) => {
                    segmenterParameters[param.getName()] = Number(event.detail.value)
                  }"
                />
              </div>
            </div>
          </div>
          <div class="float-right p-4">
            <v-button
              :loading="calculatingSegments"
              :disabled="selectedSegmenter === ''"
              label="FIND SEGMENTS"
              @click="findSegments"
            />
          </div>
        </div>
        <div class="pt-4">
          <div>
            <div class="pb-1 text-xs">
              Selected Point Position
            </div>
            <div class="flex gap-3">
              <v-input
                readonly
                label="X"
                labelposition="left"
                :value="click.x"
              />
              <v-input
                readonly
                label="Y"
                labelposition="left"
                :value="click.y"
              />
              <v-input
                readonly
                labelposition="left"
                label="Z"
                :value="click.z"
              />
              <v-button
                label="Move"
                @click="handleMove"
              />
            </div>
          </div>
          <div class="pt-4 text-xs">
            Distance From Camera: {{ distanceFromCamera }}mm
          </div>
          <div class="flex pt-4 pb-8">
            <div class="column">
              <v-radio
                label="Selection Type"
                options="Center Point, Bounding Box, Cropped"
                @input="handleSelectObject($event.detail.selected)"
              />
            </div>
            <div class="pl-8">
              <p class="text-xs">
                Segmented Objects
              </p>
              <select
                v-model="selectedObject"
                class="m-0 w-full appearance-none border border-solid border-black bg-white bg-clip-padding px-3 py-1.5 text-xs font-normal text-gray-700 focus:outline-none"
                :class="['py-2 pl-2']"
                @change="handleSelectObject(($event.currentTarget as HTMLSelectElement).value)"
              >
                <option
                  disabled
                  selected
                  value=""
                >
                  Select Object
                </option>
                <option
                  v-for="(seg, index) in objects"
                  :key="index"
                  :value="index"
                >
                  Object {{ index }}
                </option>
              </select>
            </div>
            <div class="pl-8">
              <div class="grid grid-cols-1">
                <span class="text-xs">Object Points</span>
                <span class="pt-2">
                  {{ objects ? objects.length : "null" }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style>
  .pcd-container canvas {
    width: 100%;
  }
</style>

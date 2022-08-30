<script setup lang="ts">

/**
 * @TODO: No disposing of THREE resources currently.
 * This is causing memory leaks.
 */

import { grpc } from '@improbable-eng/grpc-web';
import { nextTick, onMounted, onUnmounted, watch } from 'vue';
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
let segmenterParameterNames = $ref<TypedParameter[]>();
let objects = $ref<PointCloudObject[]>([]);
let segmenterNames = $ref<string[]>([]);
let segmenterParameters = $ref<Record<string, number>>({});
let calculatingSegments = $ref(false);
let url = $ref('');

const selectedObject = $ref('');
const selectedSegmenter = $ref('');

const distanceFromCamera = $computed(() => Math.round(
  Math.sqrt(
    Math.pow(click.x, 2) + Math.pow(click.y, 2) + Math.pow(click.z, 2)
  )
) || 0);

const loader = new PCDLoader();
const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 2000);
const renderer = new THREE.WebGLRenderer();
const raycaster = new THREE.Raycaster();
const sphereGeometry = new THREE.SphereGeometry(0.009, 32, 32);
const sphereMaterial = new THREE.MeshBasicMaterial({ color: 0xFF_00_00 });
const sphere = new THREE.Mesh(sphereGeometry, sphereMaterial);
const controls = new OrbitControls(camera, renderer.domElement);
const click = new THREE.Vector3();
camera.position.set(0, 0, 0);
controls.target.set(0, 0, -1);
controls.update();
camera.updateMatrix();

let frameId = -1;

const renderPCD = () => {
  nextTick(() => {
    const request = new cameraApi.GetPointCloudRequest();
    request.setName(props.cameraName);
    request.setMimeType('pointcloud/pcd');
    window.cameraService.getPointCloud(request, new grpc.Metadata(), (error, response) => {
      if (error) {
        toast.error(`Error getting point cloud: ${error}`);
        return;
      }
      const pointcloud = response!.getPointCloud_asB64();
      url = `data:pointcloud/pcd;base64,${pointcloud}`;
    });
  });

  getSegmenterNames();
};

const getSegmenterNames = () => {
  const request = new visionApi.GetSegmenterNamesRequest();
  // We are deliberately just getting the first vision service to ensure this will not break.
  // May want to allow for more services in the future
  const visionName = filterResources(props.resources, 'rdk', 'services', 'vision')[0];
  
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
  const visionName = filterResources(props.resources, 'rdk', 'services', 'vision')[0];

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
  frameId = requestAnimationFrame(animate);

  if (resizeRendererToDisplaySize()) {
    const canvas = renderer.domElement;
    camera.aspect = canvas.clientWidth / canvas.clientHeight;
    camera.updateProjectionMatrix();
  }

  renderer.render(scene, camera);
  controls.update();
};

const resizeRendererToDisplaySize = () => {
  const canvas = renderer.domElement;
  const width = canvas.clientWidth;
  const height = canvas.clientHeight;
  const needResize = canvas.width !== width || canvas.height !== height;
  if (needResize) {
    renderer.setSize(width, height, false);
  }
  return needResize;
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
  const visionName = filterResources(props.resources, 'rdk', 'services', 'vision')[0];
  
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
  url = `data:pointcloud/pcd;base64,${pointcloud}`;
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

const handleClick = (event: MouseEvent) => {
  const mouse = new THREE.Vector2();
  const target = event.currentTarget as HTMLDivElement;
  mouse.x = (event.offsetX / target.offsetWidth) * 2 - 1;
  mouse.y = (event.offsetY / target.offsetHeight) * -2 + 1;

  raycaster.setFromCamera(mouse, camera);

  const [intersect] = raycaster.intersectObjects(scene.children);

  if (intersect !== null) {
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
  const motionName = filterResources(props.resources, 'rdk', 'services', 'motion')[0];
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

onMounted(() => {
  container.append(renderer.domElement);
  renderer.setSize(window.innerWidth / 2, window.innerHeight / 2);
  requestAnimationFrame(animate);
  renderPCD();
});

onUnmounted(() => {
  cancelAnimationFrame(frameId);
});

watch(() => props.pointcloud, async (pointcloud: string) => {
  if (!pointcloud) {
    return;
  }

  url = `data:pointcloud/pcd;base64,${pointcloud}`;
  const points = await loader.loadAsync(url);
  scene.clear();
  scene.add(points);
  scene.add(sphere);

  if (cube) {
    scene.add(cube);
  }

  animate();
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
      class="relative"
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
                  class="h-4 w-4 stroke-2 text-gray-700 dark:text-gray-300"
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

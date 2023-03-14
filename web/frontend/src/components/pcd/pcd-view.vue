<!-- eslint-disable multiline-comment-style -->
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
import { TransformControls } from 'three/examples/jsm/controls/TransformControls';
import { filterResources } from '../../lib/resource';
import { toast } from '../../lib/toast';
import { Client, commonApi, motionApi } from '@viamrobotics/sdk';
import InfoButton from '../info-button.vue';

const props = defineProps<{
  resources: commonApi.ResourceName.AsObject[]
  pointcloud?: Uint8Array
  cameraName?: string
  client: Client
}>();

const container = $ref<HTMLDivElement>();

let cube: THREE.LineSegments;
let displayGrid = true;

let transformEnabled = $ref(false);
const download = $ref<HTMLLinkElement>();
// let segmenterParameterNames = $ref<TypedParameter[]>();
const objects = $ref<commonApi.PointCloudObject[]>([]);
const segmenterNames = $ref<string[]>([]);
// let segmenterParameters = $ref<Record<string, number>>({});

const click = $ref(new THREE.Vector3());

const selectedObject = $ref('');
const selectedSegmenter = $ref('');

const distanceFromCamera = $computed(() => Math.round(
  Math.sqrt((click.x ** 2) + (click.y ** 2) + (click.z ** 2))
) || 0);

const loader = new PCDLoader();
const scene = new THREE.Scene();
const ambientLight = new THREE.AmbientLight(0xFF_FF_FF, 3);
scene.add(ambientLight);

const camera = new THREE.PerspectiveCamera(
  75, window.innerWidth / window.innerHeight, 0.01, 2000
);
camera.position.set(0.5, 0.5, 1);
camera.lookAt(0, 0, 0);

const renderer = new THREE.WebGLRenderer({
  powerPreference: 'high-performance',
  antialias: true,
});
renderer.domElement.style.width = '100%';
renderer.setClearColor('white');
const raycaster = new THREE.Raycaster();

const controls = new OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;

const transformControls = new TransformControls(camera, renderer.domElement);
transformControls.setSize(1);
transformControls.setMode('translate');
transformControls.enabled = false;

transformControls.addEventListener('dragging-changed', (event) => {
  controls.enabled = !event.value;
});

const color = new THREE.Color();
let mesh: THREE.InstancedMesh;

const matrix = new THREE.Matrix4();
const vec3 = new THREE.Vector3();
const sphereGeometry = new THREE.SphereGeometry(0.01, 16, 16);
const sphereWireframe = new THREE.WireframeGeometry(sphereGeometry);
const sphere = new THREE.LineSegments(sphereWireframe);
scene.add(sphere);

const sphereMaterial = sphere.material as THREE.MeshBasicMaterial;
sphereMaterial.color.set('black');
sphereMaterial.transparent = true;
sphereMaterial.opacity = 0.4;

const size = 10;
const divisions = 10;
const gridHelper = new THREE.GridHelper(size, divisions);
scene.add(gridHelper);

const gridMaterial = gridHelper.material as THREE.MeshBasicMaterial;
gridMaterial.color.set('black');

const getSegmenterParameters = () => {
  // const req = new visionApi.GetSegmenterParametersRequest();

  // /*
  //  * We are deliberately just getting the first vision service to ensure this will not break.
  //  * May want to allow for more services in the future
  //  */
  // const [vision] = filterResources(props.resources, 'rdk', 'service', 'vision');

  // req.setName(vision.name);
  // req.setSegmenterName(selectedSegmenter);

  // window.visionService.getSegmenterParameters(req, new grpc.Metadata(), (error, response) => {
  //   if (error) {
  //     toast.error(`Error getting segmenter parameters: ${error}`);
  //     return;
  //   }

  //   segmenterParameterNames = response!.getSegmenterParametersList();
  //   segmenterParameters = {};
  // });
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

const animate = () => {
  resizeRendererToDisplaySize();
  renderer.render(scene, camera);
  controls.update();
};

const setPoint = (point: THREE.Vector3) => {
  click.x = Math.round(point.x * 1000);
  click.y = Math.round(point.y * 1000);
  click.z = Math.round(point.z * 1000);
  sphere.position.copy(point);
};

// const parameterType = (type: string) => {
//   if (type === 'int' || type === 'float64') {
//     return 'number';
//   } else if (type === 'string' || type === 'char') {
//     return 'text';
//   }
//   return '';
// };

const findSegments = () => {
  // const req = new visionApi.GetObjectPointCloudsRequest();

  // /*
  //  * We are deliberately just getting the first vision service to ensure this will not break.
  //  * May want to allow for more services in the future
  //  */
  // const [vision] = filterResources(props.resources, 'rdk', 'service', 'vision');

  // req.setName(vision.name);
  // req.setCameraName(props.cameraName!);
  // req.setSegmenterName(selectedSegmenter);
  // // req.setParameters(Struct.fromJavaScript(segmenterParameters));
  // req.setMimeType('pointcloud/pcd');

  // window.visionService.getObjectPointClouds(req, new grpc.Metadata(), (error, response) => {
  //   if (error) {
  //     toast.error(`Error getting segments: ${error}`);
  //     return;
  //   }

  //   objects = response!.getObjectsList();

  //   if (objects.length === 0) {
  //     toast.info('Found no segments.');
  //   }
  // });
};

const getSegmenterNames = () => {

  /*
   * radius_clustering_segmenter
   * detector_segmenter
   * window.visionService.getModelParameterSchema()
   * -> returns json.schema
   */

  /*
   * take input values and send them to:
   * window.visionService.addSegmenter()
   * now you have a segmenter
   */

  /*
   * now you can inspect what segmenters are available
   * window.visionService.getSegmenterNames()
   */

  // getObjectPointClouds

  // const request = new visionApi.GetSegmenterNamesRequest();

  // /*
  //  * We are deliberately just getting the first vision service to ensure this will not break.
  //  * May want to allow for more services in the future
  //  */
  // const [vision] = filterResources(props.resources, 'rdk', 'service', 'vision');

  // request.setName(vision.name);

  // window.visionService.getSegmenterNames(request, new grpc.Metadata(), (error, response) => {
  //   if (error) {
  //     toast.error(`Error getting segmenter names: ${error}`);
  //     return;
  //   }

  //   segmenterNames = response!.getSegmenterNamesList();
  // });
};

const setBoundingBox = (box: commonApi.RectangularPrism, centerPoint: THREE.Vector3) => {
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

const update = (cloud: Uint8Array) => {
  // dispose old resources
  if (mesh) {
    scene.remove(mesh);
    mesh.geometry.dispose();
    (mesh.material as THREE.MeshBasicMaterial).dispose();
  }

  const points = loader.parse(cloud.buffer, '');
  points.name = 'points';
  const positions = points.geometry.attributes.position!.array;

  // TODO (hackday): colors is not consistently returned, if not just render all points as blue
  // eslint-disable-next-line unicorn/prefer-spread
  const colors = points.geometry.attributes.color?.array ?? Array.from(positions).flatMap(() => [0.3, 0.5, 0.7]);

  const count = positions.length / 3;
  const material = new THREE.MeshBasicMaterial();
  const geometry = new THREE.BoxGeometry(0.005, 0.005, 0.005);
  mesh = new THREE.InstancedMesh(geometry, material, count);
  mesh.name = 'points';

  for (let i = 0, j = 0; i < count; i += 1, j += 3) {
    matrix.setPosition(positions[j + 0]!, positions[j + 1]!, positions[j + 2]!);
    mesh.setMatrixAt(i, matrix);

    if (colors) {
      color.setRGB(colors[j + 0]!, colors[j + 1]!, colors[j + 2]!);
      mesh.setColorAt(i, color);
    }
  }

  if (mesh.instanceColor) {
    mesh.instanceColor.needsUpdate = true;
  }

  mesh.instanceMatrix.needsUpdate = true;

  scene.add(mesh);
  transformControls.attach(mesh);

  if (cube) {
    scene.add(cube);
  }
};

const loadSegment = (index: number) => {
  const segment = objects[index]!;

  if (!segment) {
    toast.error('Segment cannot be found.');
  }

  const pointcloud = segment.getPointCloud_asU8();
  const center = segment.getGeometries()!.getGeometriesList()[0]!.getCenter()!;
  const box = segment.getGeometries()!.getGeometriesList()[0]!.getBox()!;

  const point = new THREE.Vector3(
    center.getX(),
    center.getY(),
    center.getZ()
  ).multiplyScalar(1 / 1000);

  setPoint(point);

  setBoundingBox(box, point);
  update(pointcloud);
};

const loadBoundingBox = (index: number) => {
  const segment = objects[index];

  if (!segment) {
    toast.error('Segment cannot be found.');
    return;
  }

  const center = segment.getGeometries()!.getGeometriesList()[0]!.getCenter()!;
  const box = segment.getGeometries()!.getGeometriesList()[0]!.getBox();

  const point = new THREE.Vector3(
    center.getX(),
    center.getY(),
    center.getZ()
  ).multiplyScalar(1 / 1000);

  setBoundingBox(box!, point);
};

const loadPoint = (index: number) => {
  const segment = objects[index];

  if (!segment) {
    toast.error('Segment cannot be found.');
    return;
  }

  const center = segment.getGeometries()!.getGeometriesList()[0]!.getCenter()!;

  const point = new THREE.Vector3(
    center.getX(),
    center.getY(),
    center.getZ()
  ).multiplyScalar(1 / 1000);

  setPoint(point);
};

const getMouseNormalizedDeviceCoordinates = (event: MouseEvent) => {
  const canvas = renderer.domElement;
  const rect = canvas.getBoundingClientRect();

  return {
    x: (((event.clientX - rect.left) / canvas.width * devicePixelRatio) * 2) - 1,
    y: (-((event.clientY - rect.top) / canvas.height * devicePixelRatio) * 2) + 1,
  };
};

const epsilon = 10;
const mousedown = new THREE.Vector2();
const mouseup = new THREE.Vector2();

const handleCanvasMouseDown = (event: MouseEvent) => {
  if (controls.enabled === false) {
    return;
  }

  mousedown.set(event.clientX, event.clientY);
};

const handleCanvasMouseUp = (event: MouseEvent) => {
  if (controls.enabled === false) {
    return;
  }

  mouseup.set(event.clientX, event.clientY);

  // Don't fire on drag events
  if (mousedown.sub(mouseup).length() > epsilon) {
    return;
  }

  const { x, y } = getMouseNormalizedDeviceCoordinates(event);
  const mouse = new THREE.Vector2(x, y);

  raycaster.setFromCamera(mouse, camera);

  const [intersect] = raycaster.intersectObjects([scene.getObjectByName('points')!]);
  const points = scene.getObjectByName('points') as THREE.InstancedMesh;

  if (intersect?.instanceId === undefined) {
    toast.info('No point intersected.');
    return;
  }

  points.getMatrixAt(intersect.instanceId, matrix);
  vec3.setFromMatrixPosition(matrix);
  setPoint(vec3);
};

const handleMove = () => {
  const [gripper] = filterResources(props.resources, 'rdk', 'component', 'gripper');

  if (gripper === undefined) {
    toast.error('No gripper component detected.');
    return;
  }

  /*
   * We are deliberately just getting the first motion service to ensure this will not break.
   * May want to allow for more services in the future
   */
  const [motion] = filterResources(props.resources, 'rdk', 'service', 'motion');

  if (motion === undefined) {
    toast.error('No motion service detected.');
    return;
  }

  const req = new motionApi.MoveRequest();
  const cameraPoint = new commonApi.Pose();

  cameraPoint.setX(click.x);
  cameraPoint.setY(click.y);
  cameraPoint.setZ(click.z);

  const pose = new commonApi.PoseInFrame();
  pose.setReferenceFrame(props.cameraName!);
  pose.setPose(cameraPoint);
  req.setDestination(pose);
  req.setName(motion.name);
  const componentName = new commonApi.ResourceName();
  componentName.setNamespace(gripper.namespace);
  componentName.setType(gripper.type);
  componentName.setSubtype(gripper.subtype);
  componentName.setName(gripper.name);
  req.setComponentName(componentName);

  props.client.motionService.move(req, new grpc.Metadata(), (error, response) => {
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
    case 'Center Point': {
      return loadSegment(0);
    }
    case 'Bounding Box': {
      return loadBoundingBox(1);
    }
    case 'Cropped': {
      return loadPoint(2);
    }
  }
};

const handleToggleGrid = () => {
  if (displayGrid) {
    scene.remove(gridHelper);
  } else {
    scene.add(gridHelper);
  }

  displayGrid = !displayGrid;
};

const handleToggleTransformControls = () => {
  transformControls.enabled = !transformControls.enabled;
  transformEnabled = transformControls.enabled;

  if (transformControls.enabled) {
    scene.add(transformControls);
  } else {
    scene.remove(transformControls);
  }
};

const handleTransformModeChange = (event: CustomEvent) => {
  const { value } = event.detail;

  transformControls.setMode(value.toLowerCase());
};

const handlePointsResize = (event: CustomEvent) => {
  const points = scene.getObjectByName('points') as THREE.InstancedMesh;
  const scale = event.detail.value;

  for (let i = 0; i < points.count; i += 1) {
    points.getMatrixAt(i, matrix);
    vec3.setFromMatrixPosition(matrix);
    matrix.makeScale(scale, scale, scale);
    matrix.setPosition(vec3);
    points.setMatrixAt(i, matrix);
  }

  sphere.scale.set(scale, scale, scale);

  mesh.instanceMatrix.needsUpdate = true;
};

const init = (pointcloud: Uint8Array) => {
  update(pointcloud);

  // eslint-disable-next-line unicorn/text-encoding-identifier-case
  const decoder = new TextDecoder('utf-8');
  const file = new File([decoder.decode(pointcloud)], 'pointcloud.txt');
  download.href = URL.createObjectURL(file);

  if (props.cameraName) {
    getSegmenterNames();
  }
};

onMounted(() => {
  container.append(renderer.domElement);
  renderer.setAnimationLoop(animate);

  if (props.pointcloud) {
    init(props.pointcloud);
  }
});

onUnmounted(() => {
  renderer.setAnimationLoop(null);
});

watch(() => props.pointcloud, (updated?: Uint8Array) => {
  if (updated) {
    init(updated);
  }
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
      <a
        ref="download"
        download="pointcloud.txt"
      >
        <v-button
          icon="download"
          label="Download Raw Data"
          @click="() => {}"
        />
      </a>
    </div>

    <div
      ref="container"
      class="pcd-container relative w-full border border-black"
      @mousedown="handleCanvasMouseDown"
      @mouseup="handleCanvasMouseUp"
    />

    <div class="relative flex flex-wrap w-full items-center justify-between gap-12">
      <div class="w-full pl-4 pt-2 max-w-xs">
        <v-slider
          label="Points Scaling"
          min="0.1"
          value="1"
          max="3"
          step="0.05"
          @input="handlePointsResize"
        />
      </div>

      <div class="flex items-center gap-1">
        <span class="text-xs">Controls</span>
        <InfoButton
          :info-rows="[
            'Rotate - Left/Click + Drag',
            'Pan - Right/Two Finger Click + Drag',
            'Zoom - Wheel/Two Finger Scroll',
          ]"
        />
      </div>

      <label class="flex flex-col gap-1 text-xs">
        <div class="flex items-center gap-1.5">
          Transform controls
          <v-switch
            value="off"
            @input="handleToggleTransformControls"
          />
        </div>

        <v-radio
          v-if="transformEnabled"
          class="w-full"
          options="Translate, Rotate, Scale"
          selected="Translate"
          @input="handleTransformModeChange"
        />
      </label>

      <label class="flex items-center gap-1.5 text-xs">
        Grid
        <v-switch
          value="on"
          @input="handleToggleGrid"
        />
      </label>
    </div>

    <div
      v-if="false"
      class="container mx-auto pt-4"
    >
      <h2>Segmentation Settings</h2>
      <div class="relative">
        <select
          v-model="selectedSegmenter"
          placeholder="Choose"
          class="
            m-0 w-full appearance-none border border-solid border-black bg-white
            bg-clip-padding px-3 py-1.5 text-xs font-normal text-gray-700 focus:outline-none
          "
          aria-label="Select segmenter"
          @change="getSegmenterParameters"
        >
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

      <!-- <div class="flex items-end gap-4">
        <v-input
          v-for="param in segmenterParameterNames"
          :key="param.getName()"
          :type="parameterType(param.getType())"
          :label="param.getName()"
          :value="segmenterParameters[param.getName()]"
          @input="(event: CustomEvent) => {
            segmenterParameters[param.getName()] = Number(event.detail.value)
          }"
        />
      </div> -->

      <v-button
        :disabled="selectedSegmenter === ''"
        class="mt-2 block"
        label="Find Segments"
        @click="findSegments"
      />
    </div>
    <div class="pt-4">
      <div class="pb-1 text-xs">
        Selected Point Position
      </div>
      <div class="flex flex-wrap gap-3">
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

      <div class="pt-4 text-xs">
        Distance From Camera: {{ distanceFromCamera }}mm
      </div>
      <div
        v-if="false"
        class="flex pt-4 pb-8"
      >
        <div class="column">
          <v-radio
            label="Selection Type"
            options="Center Point, Bounding Box, Cropped"
            @input="handleSelectObject($event.detail.value)"
          />
        </div>
        <div class="pl-8">
          <p class="text-xs">
            Segmented Objects
          </p>
          <select
            v-model="selectedObject"
            class="
              m-0 w-full appearance-none border border-solid border-black bg-white
              bg-clip-padding px-3 py-1.5 text-xs font-normal text-gray-700 focus:outline-none
            "
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
              v-for="(_seg, index) in objects"
              :key="index"
              :value="index"
            >
              Object {{ index }}
            </option>
          </select>
        </div>
        <div class="pl-8">
          <span class="text-xs">Object Points</span>
          <span>
            {{ objects ? objects.length : "null" }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<!-- eslint-disable multiline-comment-style -->
<script setup lang="ts">

/**
 * @TODO: No disposing of THREE resources currently.
 * This is causing memory leaks.
 */

import { $ref, $computed } from '@vue-macros/reactivity-transform/macros';
import { onMounted, onUnmounted, watch } from 'vue';
import * as THREE from 'three';
import { OrbitControlsGizmo, GridHelper } from 'trzy';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls';
import { TransformControls } from 'three/examples/jsm/controls/TransformControls';
import { toast } from '../../lib/toast';
import { Client, commonApi } from '@viamrobotics/sdk';

const props = defineProps<{
  resources: commonApi.ResourceName.AsObject[]
  pointcloud?: Uint8Array
  cameraName?: string
  client: Client
}>();

const container = $ref<HTMLDivElement>();
const gizmoContainer = $ref<HTMLDivElement>();

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

const cellSize = 1;
const largeCellSize = 10;
const gridColor = '#cacaca';
const gridHelper = new GridHelper(cellSize, largeCellSize, gridColor);
scene.add(gridHelper);

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

  const points = loader.parse(cloud.buffer);
  points.name = 'points';
  const positions = (points.geometry.attributes.position as THREE.BufferAttribute).array as Float32Array;

  // TODO (hackday): colors is not consistently returned, if not just render all points as blue
  // eslint-disable-next-line unicorn/prefer-spread
  const colorAttrib = points.geometry.attributes.color as THREE.BufferAttribute | undefined;
  const colors = colorAttrib?.array ?? [...positions].flatMap(() => [0.3, 0.5, 0.7]);

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
  if (download) {
    download.href = URL.createObjectURL(file);
  }

  if (props.cameraName) {
    getSegmenterNames();
  }
};

let gizmo: OrbitControlsGizmo | undefined;

onMounted(() => {
  container?.append(renderer.domElement);
  renderer.setAnimationLoop(animate);

  gizmo = new OrbitControlsGizmo({ camera, el: gizmoContainer as HTMLElement, controls });

  if (props.pointcloud) {
    init(props.pointcloud);
  }
});

onUnmounted(() => {
  renderer.setAnimationLoop(null);
  gizmo?.dispose();
});

watch(() => props.pointcloud, (updated?: Uint8Array) => {
  if (updated) {
    init(updated);
  }
});

</script>

<template>
  <div class="flex gap-4">
    <div class="relative flex w-fit min-w-[370px] flex-col gap-4 pt-4">
      <v-input
        class="w-20"
        label="Points size"
        type="number"
        min="0.1"
        step="0.1"
        value="1"
        incrementor="slider"
        @input="handlePointsResize"
      />

      <div class="flex items-center gap-1">
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

      <div class="flex gap-2">
        <v-switch
          label="Grid"
          value="on"
          @input="handleToggleGrid"
        />

        <div>
          <v-switch
            label="Transform controls"
            value="off"
            @input="handleToggleTransformControls"
          />

          <v-radio
            v-if="transformEnabled"
            options="Translate, Rotate, Scale"
            selected="Translate"
            @input="handleTransformModeChange"
          />
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <div class="w-full text-xs">
          Selected Point Position
        </div>
        <v-input
          class="w-20"
          readonly
          label="X"
          labelposition="left"
          :value="click.x"
        />
        <v-input
          class="w-20"
          readonly
          label="Y"
          labelposition="left"
          :value="click.y"
        />
        <v-input
          class="w-20"
          readonly
          labelposition="left"
          label="Z"
          :value="click.z"
        />
      </div>

      <div class="text-xs">
        Distance From Camera: {{ distanceFromCamera }}mm
      </div>

      <small class="flex w-20 items-center gap-1">
        Controls
        <v-tooltip location="top">
          <v-icon name="info-outline" />
          <span slot="text">
            <p>Rotate - Left/Click + Drag</p>
            <p>Pan - Right/Two Finger Click + Drag</p>
            <p>Zoom - Wheel/Two Finger Scroll</p>
          </span>
        </v-tooltip>
      </small>
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
            m-0 w-full appearance-none border border-solid border-medium bg-white
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

    <div>
      <div
        v-if="false"
        class="flex pb-8 pt-4"
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
              m-0 w-full appearance-none border border-solid border-medium bg-white
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

    <div
      ref="container"
      class="pcd-container relative w-full border border-medium"
      @mousedown="handleCanvasMouseDown"
      @mouseup="handleCanvasMouseUp"
    >
      <div
        ref="gizmoContainer"
        class="absolute right-2 top-2"
      />
    </div>
  </div>
</template>

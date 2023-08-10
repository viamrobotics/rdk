<!-- eslint-disable multiline-comment-style -->
<script lang="ts">

import { onMount, onDestroy } from 'svelte';
import * as THREE from 'three';
import { ViewHelper, GridHelper } from 'trzy';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls';
import { TransformControls } from 'three/examples/jsm/controls/TransformControls';
import { notify } from '@viamrobotics/prime';

export let pointcloud: Uint8Array | undefined;

let container: HTMLDivElement;
let downloadHref: string;

let cube: THREE.LineSegments;
let displayGrid = true;
let transformEnabled = false;

const click = new THREE.Vector3();

$: distanceFromCamera = Math.round(Math.sqrt((click.x ** 2) + (click.y ** 2) + (click.z ** 2)));

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
    notify.info('No point intersected.');
    return;
  }

  points.getMatrixAt(intersect.instanceId, matrix);
  vec3.setFromMatrixPosition(matrix);
  setPoint(vec3);
};

const handleCenter = () => {
  setPoint(new THREE.Vector3());
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

const init = (cloud: Uint8Array) => {
  update(cloud);

  // eslint-disable-next-line unicorn/text-encoding-identifier-case
  const decoder = new TextDecoder('utf-8');
  const file = new File([decoder.decode(cloud)], 'pointcloud.txt');
  downloadHref = URL.createObjectURL(file);
};

let gizmo: ViewHelper | undefined;

onMount(() => {
  container.append(renderer.domElement);
  renderer.setAnimationLoop(animate);

  gizmo = new ViewHelper(camera, renderer);

  if (pointcloud) {
    init(pointcloud);
  }
});

onDestroy(() => {
  renderer.setAnimationLoop(null);
  gizmo?.dispose();
});

$: if (pointcloud) {
  init(pointcloud);
}

</script>

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
      on:input={handlePointsResize}
    />

    <div class="flex items-center gap-1">
      <v-button
        icon="image-filter-center-focus"
        label="Center"
        on:click={handleCenter}
      />

      <a
        href={downloadHref}
        download="pointcloud.txt"
      >
        <v-button
          icon="download"
          label="Download raw data"
        />
      </a>
    </div>

    <div class="flex gap-2">
      <v-switch
        label="Grid"
        value="on"
        on:input={handleToggleGrid}
      />

      <div>
        <v-switch
          label="Transform controls"
          value="off"
          on:input={handleToggleTransformControls}
        />

        {#if transformEnabled}
          <v-radio
            options="Translate, Rotate, Scale"
            selected="Translate"
            on:input={handleTransformModeChange}
          />
        {/if}
      </div>
    </div>

    <div class="flex flex-wrap gap-2">
      <div class="w-full text-xs">
        Selected point position
      </div>
      <v-input
        class="w-20"
        readonly
        label="X"
        labelposition="left"
        value={click.x}
      />
      <v-input
        class="w-20"
        readonly
        label="Y"
        labelposition="left"
        value={click.y}
      />
      <v-input
        class="w-20"
        readonly
        labelposition="left"
        label="Z"
        value={click.z}
      />
    </div>

    <div class="text-xs">
      Distance from camera: {distanceFromCamera}mm
    </div>

    <small class="flex w-20 items-center gap-1">
      Controls
      <v-tooltip location="top">
        <v-icon name="information-outline" />
        <span slot="text">
          <p>Rotate - Left/Click + Drag</p>
          <p>Pan - Right/Two Finger Click + Drag</p>
          <p>Zoom - Wheel/Two Finger Scroll</p>
        </span>
      </v-tooltip>
    </small>
  </div>

  <div
    bind:this={container}
    class="pcd-container relative w-full border border-medium"
    on:mousedown={handleCanvasMouseDown}
    on:mouseup={handleCanvasMouseUp}
  />
</div>

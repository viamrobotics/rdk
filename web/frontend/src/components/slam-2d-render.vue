<script setup lang="ts">

import { $ref } from 'vue/macros';
import { threeInstance, MouseRaycaster, MeshDiscardMaterial } from 'trzy';
import { onMounted, onUnmounted, watch } from 'vue';
import * as THREE from 'three';
import { MapControls } from 'three/examples/jsm/controls/OrbitControls';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import type { commonApi } from '@viamrobotics/sdk';
import { SVGLoader } from "three/examples/jsm/loaders/SVGLoader";

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
}
>();

const emit = defineEmits<{(event: 'click', point: THREE.Vector3): void}>()

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

const raycaster = new MouseRaycaster({ camera, canvas, recursive: false });

raycaster.addEventListener('click', (event) => {
  const [intersection] = event.intersections as THREE.Intersection[];
  if (intersection && intersection.point) {
    // add location marker here

    emit('click', intersection.point)
  }
});

const makeMarker = (url : string): THREE.Group => {
  let guiData = {
    currentURL: url,
    drawFillShapes: true,
    drawStrokes: true,
    fillShapesWireframe: false,
    strokesWireframe: false
  };

  let marker = new THREE.Group

  const loader = new SVGLoader();
  loader.load( guiData.currentURL, function ( data ) {
    console.log('loader.load()')

    const paths = data.paths;

    const group = new THREE.Group();
    group.scale.multiplyScalar( 0.25 );
    group.position.x = - 70;
    group.position.y = 70;
    group.scale.y *= - 1;

    console.log('u get me?')
    console.log('paths.length: ', paths.length)
    for ( let i = 0; i < paths.length; i ++ ) {

      const path = paths[ i ];
      console.log('path:', path)

      const fillColor = path.userData.style.fill;
      if ( guiData.drawFillShapes && fillColor !== undefined && fillColor !== 'none' ) {

        const material = new THREE.MeshBasicMaterial( {
          color: new THREE.Color().setStyle( fillColor ).convertSRGBToLinear(),
          opacity: path.userData.style.fillOpacity,
          transparent: true,
          side: THREE.DoubleSide,
          depthWrite: false,
          wireframe: guiData.fillShapesWireframe
        } );

        const shapes = SVGLoader.createShapes( path );

        for ( let j = 0; j < shapes.length; j ++ ) {

          const shape = shapes[ j ];

          const geometry = new THREE.ShapeGeometry( shape );
          const mesh = new THREE.Mesh( geometry, material );

          group.add( mesh );

        }

      }

      const strokeColor = path.userData.style.stroke;

      if ( guiData.drawStrokes && strokeColor !== undefined && strokeColor !== 'none' ) {

        const material = new THREE.MeshBasicMaterial( {
          color: new THREE.Color().setStyle( strokeColor ).convertSRGBToLinear(),
          opacity: path.userData.style.strokeOpacity,
          transparent: true,
          side: THREE.DoubleSide,
          depthWrite: false,
          wireframe: guiData.strokesWireframe
        } );

        for ( let j = 0, jl = path.subPaths.length; j < jl; j ++ ) {

          const subPath = path.subPaths[ j ];

          const geometry = SVGLoader.pointsToStroke( subPath.getPoints(), path.userData.style );

          if ( geometry ) {
            const mesh = new THREE.Mesh( geometry.rotateX(-Math.PI / 2), material );
            group.add( mesh );
          }

        }

      }

    }

    marker = group

  })

  return marker

}

const marker = makeMarker("data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMzEiIGhlaWdodD0iMzkiIHZpZXdCb3g9IjAgMCAzMSAzOSIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTMuNTU5MDcgNC4yMTExMkwxLjcwNTg0IDIuNzgwMzNMMS45NTM1NyA1LjEwODQ3TDQuNzE0OTYgMzEuMDU5NEw1LjA4NzA4IDM0LjU1NjVMNi42MTA2MyAzMS4zODY5TDExLjczNTUgMjAuNzI0OUwyMy41MDIxIDIxLjk0NThMMjcuMDAwMSAyMi4zMDg4TDI0LjIxNjQgMjAuMTU5NkwzLjU1OTA3IDQuMjExMTJaIiBmaWxsPSIjQkUzNTM2IiBzdHJva2U9IiNGQ0VDRUEiIHN0cm9rZS13aWR0aD0iMiIvPgo8L3N2Zz4K")
marker.name = 'Marker';
marker.renderOrder = 999;

const markerSize = 1.;
const oldMarker = new THREE.Mesh(
  new THREE.PlaneGeometry(markerSize, markerSize).rotateX(-Math.PI / 2),
  new THREE.MeshBasicMaterial({ color: 'red' })
);
oldMarker.name = 'OLDMarker';

const disposeScene = () => {
  scene.traverse((object) => {
    if (object.name === 'Marker' || object.name === 'OLDMarker') {
      console.log('I exists')
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
  const axesHelper1 = new THREE.AxesHelper( 5 );
  axesHelper1.position.set(center.x, 0, center.z);
  axesHelper1.rotateY(Math.PI/2)
  axesHelper1.scale.x = 1e5
  axesHelper1.scale.z = 1e5
  axesHelper1.renderOrder = 998
  
  const axesHelper2 = new THREE.AxesHelper( 5 );
  axesHelper2.position.set(center.x, 0, center.z);
  axesHelper2.rotateY(-Math.PI/2)
  axesHelper2.scale.x = 1e5
  axesHelper2.scale.z = 1e5
  axesHelper2.renderOrder = 997

  const gridHelper = new THREE.GridHelper( 1000, 400, 0xCACACA, 0xCACACA ); // this needs to be updated so it is set to 1m
  gridHelper.position.set(center.x, 0, center.z);
  gridHelper.renderOrder = 996;

  // add objects to scene  
  // scene.add(axesHelper1);
  // scene.add(axesHelper2);
  // scene.add(gridHelper);
  // scene.add(points);
  scene.add(oldMarker);
  scene.add(marker);
  
  // scene.add(intersectionPlane);
  
};

const updatePose = (newPose: commonApi.Pose) => {
  const x = newPose.getX();
  const z = newPose.getZ();
  marker.position.setX(x);
  marker.position.setZ(z);
  oldMarker.position.setX(x);
  oldMarker.position.setZ(z);
  console.log("marker.position: ", marker.position)
  console.log("oldMarker.position: ", oldMarker.position)
  console.log(marker)
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
        <svg width="30" height="30" viewBox="0 0 30 30" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect width="10" height="10" rx="5" fill="#BE3536"/>
        <path d="M4.66278 6.032H4.51878L2.76678 2.4H3.51878L4.95078 5.456H5.04678L6.47878 2.4H7.23078L5.47878 6.032H5.33478V8H4.66278V6.032Z" fill="#FCECEA"/>
        <rect x="20" y="20" width="10" height="10" rx="5" fill="#0066CC"/>
        <path d="M23.6708 22.4L24.9268 24.88H25.0708L26.3268 22.4H27.0628L25.6628 25.144V25.24L27.0628 28H26.3268L25.0708 25.504H24.9268L23.6708 28H22.9348L24.3348 25.24V25.144L22.9348 22.4H23.6708Z" fill="#E1F3FF"/>
        <rect x="4" y="9" width="2" height="17" fill="#BE3536"/>
        <rect x="21" y="24" width="2" height="17" transform="rotate(90 21 24)" fill="#0066CC"/>
        <rect x="0.5" y="20.5" width="9" height="9" rx="4.5" fill="#E0FAE3"/>
        <rect x="0.5" y="20.5" width="9" height="9" rx="4.5" stroke="#3D7D3F"/>
        </svg>
      </div>
    </div>
</template>

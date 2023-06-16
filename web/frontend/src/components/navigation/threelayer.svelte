<script lang='ts'>

import * as THREE from 'three';
import { type Map, MercatorCoordinate } from 'maplibre-gl';

export let map: Map

// parameters to ensure the model is georeferenced correctly on the map
const modelRotate = [Math.PI / 2, 0, 0] as const;
const modelAsMercatorCoordinate = MercatorCoordinate.fromLngLat(
  [-74.5, 40],
  0
);

const modelTransform = {
  x: modelAsMercatorCoordinate.x,
  y: modelAsMercatorCoordinate.y,
  z: modelAsMercatorCoordinate.z,
  rx: modelRotate[0],
  ry: modelRotate[1],
  rz: modelRotate[2],
  /* Since our 3D model is in real world meters, a scale transform needs to be
  * applied since the CustomLayerInterface expects units in MercatorCoordinates.
  */
  scale: modelAsMercatorCoordinate.meterInMercatorCoordinateUnits()
} as const;

const scene = new THREE.Scene()
const camera = new THREE.OrthographicCamera()

const vec3 = new THREE.Vector3()
const rotationX = new THREE.Matrix4()
const rotationY = new THREE.Matrix4()
const rotationZ = new THREE.Matrix4()
const m = new THREE.Matrix4()
const l = new THREE.Matrix4()

let renderer: THREE.WebGLRenderer

const render = (matrix: number[]) => {
  rotationX.makeRotationAxis(vec3.set(1, 0, 0), modelTransform.rx);
  rotationY.makeRotationAxis(vec3.set(0, 1, 0), modelTransform.ry);
  rotationZ.makeRotationAxis(vec3.set(0, 0, 1), modelTransform.rz);

  m.fromArray(matrix);
  l
    .makeTranslation(modelTransform.x, modelTransform.y, modelTransform.z)
    .scale(vec3.set(modelTransform.scale, -modelTransform.scale, modelTransform.scale))
    .multiply(rotationX)
    .multiply(rotationY)
    .multiply(rotationZ);

  camera.projectionMatrix = m.multiply(l);
  renderer.resetState();
  renderer.render(scene, camera);
  map.triggerRepaint();
}
 
// configuration of the custom layer for a 3D model per the CustomLayerInterface
const customLayer = {
  id: '3d-model',
  type: 'custom',
  renderingMode: '3d',
  onAdd (map: Map, gl: WebGLRenderingContext) {
    // create two three.js lights to illuminate the model
    const directionalLight = new THREE.DirectionalLight(0xffffff);
    directionalLight.position.set(0, -70, 100).normalize();
    scene.add(directionalLight);
    
    const directionalLight2 = new THREE.DirectionalLight(0xffffff);
    directionalLight2.position.set(0, 70, 100).normalize();
    scene.add(directionalLight2);

    const size = 10_000
    const model = new THREE.Mesh(
      new THREE.BoxGeometry(size, size * 10, size),
      new THREE.MeshStandardMaterial({ color: 'red' }),
    )
    scene.add(model)

    // use the MapLibre GL JS map canvas for three.js
    renderer = new THREE.WebGLRenderer({
      canvas: map.getCanvas(),
      context: gl,
      antialias: true,
    });
 
    renderer.autoClear = false;
  },
  render (_gl: WebGLRenderingContext, matrix: number[]) {
    render(matrix)
  }
};

map.on('style.load', () => map.addLayer(customLayer))

map.on('resize', () => {
  const { width, height } = map.getCanvas();
  console.log(width, height)
});

</script>
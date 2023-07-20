import * as THREE from 'three';
import { injectPlugin, useFrame, useRender, useThrelte } from '@threlte/core';
import { MercatorCoordinate, type LngLat, LngLatBounds } from 'maplibre-gl';
import { map, cameraMatrix, mapSize, view } from '../stores';

const renderTarget = new THREE.WebGLRenderTarget(0, 0, { format: THREE.RGBAFormat });
const renderTexture = renderTarget.texture;

const scene = new THREE.Scene();
const ambient = new THREE.AmbientLight();
const dir = new THREE.DirectionalLight();
dir.intensity = 1.5
scene.add(ambient);

view.subscribe((value) => {
  if (value === '2D') {
    ambient.intensity = 3.5;
    scene.remove(dir);
  } else {
    ambient.intensity = 2.5;
    scene.add(dir);
  }
});

const material = new THREE.ShaderMaterial({
  transparent: true,
  uniforms: { tex: { value: renderTexture } },
  vertexShader: `
varying vec2 vUv;
void main(){ vUv = uv; gl_Position = vec4(position,1.0);}
  `,
  fragmentShader: `
uniform sampler2D tex; 
varying vec2 vUv;
void main(){ gl_FragColor = texture2D(tex, vUv); }`,
});

const quad = new THREE.Mesh(
  new THREE.PlaneGeometry(2, 2),
  material
);

const vecPositiveX = new THREE.Vector3(1, 0, 0);
const vecPositiveY = new THREE.Vector3(0, 1, 0);
const vecPositiveZ = new THREE.Vector3(0, 0, 1);
const rotation = new THREE.Euler();

const rotationX = new THREE.Matrix4();
const rotationY = new THREE.Matrix4();
const rotationZ = new THREE.Matrix4();
const scale = new THREE.Vector3();

const scenes: {
  ref: THREE.Object3D
  camera: THREE.PerspectiveCamera
}[] = [];

let initialized = false;
let cursor = 0;

const objects: { id: number, start: () => void; stop: () => void; lngLat: LngLat }[] = [];

const setFrameloops = () => {
  const bounds = map.current!.getBounds();
  const sw = bounds.getSouthWest();
  const ne = bounds.getNorthEast();

  // Add a margin
  sw.lng -= 5;
  sw.lat -= 5;
  ne.lng += 5;
  ne.lat += 5;

  const viewportBounds = new LngLatBounds(sw, ne);

  for (const { lngLat, start, stop } of objects) {
    if (viewportBounds.contains(lngLat)) {
      start();
    } else {
      stop();
    }
  }
};

const initialize = () => {
  map.current?.on('move', setFrameloops);
  setFrameloops();

  mapSize.subscribe(({ width, height }) => {
    const dpi = window.devicePixelRatio;
    renderTarget.setSize(width * dpi, height * dpi);
  });

  const { renderer } = useThrelte();

  useRender((ctx) => {
    renderer!.resetState();
    renderer!.setRenderTarget(renderTarget);
    renderer!.clear();

    scenes.forEach(({ ref, camera }) => {
      scene.add(ref);
      renderer!.render(scene, camera);
      scene.remove(ref);
    });

    renderer!.setRenderTarget(null);
    renderer!.render(quad, ctx.camera.current);
  });

  initialized = true;
};

const deregister = (id: number) => {
  objects.splice(objects.findIndex((object) => object.id === id), 1);
};

const register = (lngLat: LngLat, start: () => void, stop: () => void) => {
  cursor += 1;
  objects.push({ id: cursor, lngLat, start, stop });
  return cursor;
};

export interface Props {
  lnglat?: LngLat;
  altitude?: number;
}

export const renderPlugin = () => injectPlugin<Props>('lnglat', ({ ref, props }) => {
  let currentRef = ref;
  let currentProps = props;

  if (!(currentRef instanceof THREE.Object3D) || currentProps.lnglat === undefined) {
    return;
  }

  if (!initialized) {
    initialize();
  }

  const camera = new THREE.PerspectiveCamera();
  const sceneObj = { ref, camera };
  scenes.push(sceneObj);

  const modelMatrix = new THREE.Matrix4();

  const updateModelMatrix = (lngLat: LngLat, altitude?: number) => {
    const mercator = MercatorCoordinate.fromLngLat(lngLat, altitude);
    const scaleScalar = mercator.meterInMercatorCoordinateUnits();
    scale.set(scaleScalar, -scaleScalar, scaleScalar);

    rotation.copy(currentRef.rotation);
    rotation.x += Math.PI / 2;
    rotationX.makeRotationAxis(vecPositiveX, rotation.x);
    rotationY.makeRotationAxis(vecPositiveY, rotation.y);
    rotationZ.makeRotationAxis(vecPositiveZ, rotation.z);

    modelMatrix
      .makeTranslation(mercator.x, mercator.y, mercator.z)
      .scale(scale)
      .multiply(rotationX)
      .multiply(rotationY)
      .multiply(rotationZ);
  };

  updateModelMatrix(currentProps.lnglat);

  const { start, stop } = useFrame(() => {
    camera.projectionMatrix.copy(cameraMatrix).multiply(modelMatrix);
  }, { order: 1 });

  const id = register(currentProps.lnglat, start, stop);

  return {
    onRefChange (nextRef) {
      currentRef = nextRef;
      sceneObj.ref = nextRef;

      return () => {
        deregister(id);
        stop();
      };
    },
    onPropsChange (nextProps) {
      currentProps = nextProps;

      if (currentProps.lnglat === undefined) {
        return;
      }

      const { lngLat } = objects.find((object) => object.id === id)!;
      lngLat.lng = currentProps.lnglat.lng;
      lngLat.lat = currentProps.lnglat.lat;

      updateModelMatrix(currentProps.lnglat, currentProps.altitude);
    },
    pluginProps: ['lnglat', 'altitude'],
  };
});

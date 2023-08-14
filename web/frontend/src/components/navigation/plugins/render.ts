import * as THREE from 'three';
import { injectPlugin, useFrame, useRender, useThrelte } from '@threlte/core';
import { MercatorCoordinate, type LngLat, LngLatBounds } from 'maplibre-gl';
import { AxesHelper } from 'trzy';
import { map, cameraMatrix, mapSize } from '../stores';
import { onDestroy } from 'svelte';

const { clamp } = THREE.MathUtils;

const world = new THREE.Group();
const axes = new AxesHelper(1, 0.1);
const rotation = new THREE.Euler();
const rotationMatrix = new THREE.Matrix4();
const scale = new THREE.Vector3();

// Viam's coordinate system.
world.rotateY(-Math.PI / 2);
world.rotateX(-Math.PI / 2);
world.rotateZ(Math.PI / 2);
world.add(axes);

let initialized = false;
let counter = 0;
let cursor = 0;

const scenes: { ref: THREE.Mesh; matrix: THREE.Matrix4 }[] = [];
const objects: { id: number, start: () => void; stop: () => void; lngLat: LngLat }[] = [];

const setFrameloops = () => {
  const bounds = map.current!.getBounds();
  const sw = bounds.getSouthWest();
  const ne = bounds.getNorthEast();

  // Add margins, clamp to min and max lng,lat.
  sw.lng = clamp(sw.lng - 5, -90, 90);
  sw.lat = clamp(sw.lat - 5, -90, 90);

  ne.lng = clamp(ne.lng + 5, -90, 90);
  ne.lat = clamp(ne.lat + 5, -90, 90);

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
  initialized = true;

  map.current!.on('move', setFrameloops);
  setFrameloops();

  const { renderer, scene, camera } = useThrelte();

  renderer.autoClear = false;

  scene.add(world);

  useRender(() => {
    renderer.clear();

    scenes.forEach(({ ref, matrix }) => {
      camera.current.projectionMatrix = matrix;
      axes.length = (ref.geometry.boundingSphere?.radius ?? 0) * 2;

      world.add(ref);
      renderer.render(scene, camera.current);
      world.remove(ref);
    });
  });

  const unsub = mapSize.subscribe((value) => {
    renderer.setSize(value.width, value.height);
  });

  onDestroy(() => unsub());
};

const deinitialize = () => {
  initialized = false;

  map.current!.off('move', setFrameloops);
  scenes.splice(0, scenes.length);
  objects.splice(0, objects.length);
};

export interface Props {
  lnglat?: LngLat;
  altitude?: number;
}

export const renderPlugin = () => injectPlugin<Props>('lnglat', ({ ref, props }) => {
  let currentRef: THREE.Mesh = ref;
  let currentProps = props;

  if (!(currentRef instanceof THREE.Mesh) || currentProps.lnglat === undefined) {
    return;
  }

  if (!initialized) {
    initialize();
  }

  const matrix = new THREE.Matrix4();
  const modelMatrix = new THREE.Matrix4();

  const sceneObj = { ref, matrix };
  scenes.push(sceneObj);

  const updateModelMatrix = (lngLat: LngLat, altitude?: number) => {
    const mercator = MercatorCoordinate.fromLngLat(lngLat, altitude);
    const scaleScalar = mercator.meterInMercatorCoordinateUnits();
    scale.set(scaleScalar, -scaleScalar, scaleScalar);

    rotation.copy(currentRef.rotation);
    rotation.x += Math.PI / 2;

    rotationMatrix.makeRotationFromEuler(rotation);

    modelMatrix
      .makeTranslation(mercator.x, mercator.y, mercator.z)
      .scale(scale)
      .multiply(rotationMatrix);
  };

  updateModelMatrix(currentProps.lnglat);

  const { start, stop } = useFrame(() => {
    matrix.copy(cameraMatrix).multiply(modelMatrix);
  }, { order: 1 });

  const { scene } = useThrelte();

  currentRef.matrixAutoUpdate = false;
  currentRef.matrixWorldAutoUpdate = false;

  scene.remove(currentRef);

  cursor += 1;
  counter += 1;

  objects.push({ id: cursor, lngLat: currentProps.lnglat, start, stop });

  const id = cursor;

  onDestroy(() => {
    stop();
    objects.splice(objects.findIndex((object) => object.id === id), 1);
    scenes.splice(scenes.indexOf(sceneObj), 1);
    counter -= 1;
    if (counter === 0) {
      deinitialize();
    }
  });

  return {
    onRefChange (nextRef: THREE.Mesh) {
      currentRef = nextRef;
      sceneObj.ref = nextRef;

      if (currentProps.lnglat) {
        updateModelMatrix(currentProps.lnglat, currentProps.altitude);
      }
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

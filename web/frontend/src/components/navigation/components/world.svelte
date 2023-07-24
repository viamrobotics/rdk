<script lang='ts'>
import * as THREE from 'three';
import { T, injectPlugin, useFrame, useThrelte } from '@threlte/core';
import { MercatorCoordinate } from 'maplibre-gl';
import { map, mapCenter, cameraMatrix } from '../stores';

const EARTH_RADIUS_METERS = 6_371_010;
const vec3 = new THREE.Vector3();

const scale = new THREE.Matrix4();
const rotation = new THREE.Matrix4();
const rotationX = new THREE.Matrix4();
const rotationY = new THREE.Matrix4();

let world: THREE.Group;

/**
 * Converts WGS84 latitude and longitude to (uncorrected) WebMercator meters.
 * (WGS84 --> WebMercator (EPSG:3857))
 */
const latLngToXY = (
  position: { lng: number, lat: number, alt?: number }
): [lng: number, lat: number, alt?: number] => {
  return [
    EARTH_RADIUS_METERS * THREE.MathUtils.degToRad(position.lng),
    EARTH_RADIUS_METERS * Math.log(Math.tan((0.25 * Math.PI) + (0.5 * THREE.MathUtils.degToRad(position.lat)))),
  ];
};

const latLngToVector3Relative = (
  point: { lng: number, lat: number, alt?: number },
  reference?: { lng: number, lat: number, alt?: number },
  target = new THREE.Vector3()
) => {
  const [px, py] = latLngToXY(point);

  let rx = 0;
  let ry = 0;

  if (reference) {
    [rx, ry] = latLngToXY(reference);
  }

  target.set(px - rx, py - ry, 0);

  // apply the spherical mercator scale-factor for the reference latitude
  target.multiplyScalar(Math.cos(THREE.MathUtils.degToRad(reference?.lat ?? 0)));

  target.z = (point.alt ?? 0) - (reference?.alt ?? 0);

  return target;
};

export const createCameraTransform = () => {
  const centerLngLat = map.current!.getCenter();
  const center = MercatorCoordinate.fromLngLat(centerLngLat, 0);
  const distance = center.meterInMercatorCoordinateUnits();
  const scale = new THREE.Matrix4().makeScale(distance, distance, -distance);
  const rotation = new THREE.Matrix4().multiplyMatrices(
    new THREE.Matrix4().makeRotationX(-0.5 * Math.PI),
    new THREE.Matrix4().makeRotationY(Math.PI)
  );
  return new THREE.Matrix4()
    .multiplyMatrices(scale, rotation)
    .setPosition(center.x, center.y, center.z);
};

const { camera } = useThrelte();

const transform = createCameraTransform()

useFrame(() => {
  camera.current.projectionMatrix
    .copy(cameraMatrix)
    .multiply(transform);

  latLngToVector3Relative(mapCenter.current, undefined, world.position);
});

injectPlugin<{
  lnglat?: undefined | { lng: number, lat: number, alt?: number }
}>('lnglat', ({ ref, props }) => {
  // skip injection if ref is not an Object3D
  if (!(ref instanceof THREE.Object3D) || !('lnglat' in props)) {
    return;
  }

  //const { invalidate } = useThrelte();

  let currentRef = ref;
  let currentProps = props;

  useFrame(() => {
    if (!currentProps.lnglat) return
    latLngToVector3Relative(currentProps.lnglat, mapCenter.current, vec3);
    currentRef.position.set(vec3.y, -vec3.x, 0);
  })

  const applyProps = () => {
    if (currentProps.lnglat === undefined) {
      return;
    }

    

    // invalidate();
  };

  applyProps();

  return {
    onRefChange (nextRef) {
      currentRef = nextRef;
      applyProps();
    },
    onPropsChange (nextProps) {
      currentProps = nextProps;
      applyProps();
    },
    pluginProps: ['lnglat'],
  };
});

// on:create={({ ref }) => {
//   // Rotate into Viam's coordinate system
//   ref.rotateY(-Math.PI / 2);
//   ref.rotateX(-Math.PI / 2);
// }}

</script>

<T.Group bind:ref={world} name='world'>
  <T.PerspectiveCamera
    makeDefault
  />

  <slot />

  <T.Mesh>
    <T.BoxGeometry args={[100,100,100]} />
    <T.MeshStandardMaterial color='red' />
  </T.Mesh>
</T.Group>

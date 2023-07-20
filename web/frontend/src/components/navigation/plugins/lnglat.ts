import * as THREE from 'three';
import { injectPlugin, useThrelte } from '@threlte/core';
import { MercatorCoordinate } from 'maplibre-gl';
import { map, mapCenter } from '../stores';

const EARTH_RADIUS_METERS = 6_371_010;
const vec3 = new THREE.Vector3();

const center = { lng: 0, lat: 0 };

mapCenter.subscribe((value) => {
  center.lng = value.lng;
  center.lat = value.lat;
});

/**
 * Converts WGS84 latitude and longitude to (uncorrected) WebMercator meters.
 * (WGS84 --> WebMercator (EPSG:3857))
 */
export const latLngToXY = (
  position: { lng: number, lat: number, alt?: number }
): [lng: number, lat: number, alt?: number] => {
  return [
    EARTH_RADIUS_METERS * THREE.MathUtils.degToRad(position.lng),
    EARTH_RADIUS_METERS * Math.log(Math.tan((0.25 * Math.PI) + (0.5 * THREE.MathUtils.degToRad(position.lat)))),
  ];
};

export const latLngToVector3Relative = (
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

const scale = new THREE.Matrix4();
const rotation = new THREE.Matrix4();
const rotationX = new THREE.Matrix4();
const rotationY = new THREE.Matrix4();

export const createCameraTransform = () => {
  const centerLngLat = map.current!.getCenter();
  const mercator = MercatorCoordinate.fromLngLat(centerLngLat, 0);
  const distance = mercator.meterInMercatorCoordinateUnits();
  scale.makeScale(distance, distance, -distance);
  rotation.multiplyMatrices(
    rotationX.makeRotationX(-0.5 * Math.PI),
    rotationY.makeRotationY(Math.PI)
  );
  return new THREE.Matrix4()
    .multiplyMatrices(scale, rotation)
    .setPosition(mercator.x, mercator.y, mercator.z);
};

export const injectLngLatPlugin = () => injectPlugin<{
  lnglat?: undefined | { lng: number, lat: number, alt?: number }
}>('lnglat', ({ ref, props }) => {
  // skip injection if ref is not an Object3D
  if (!(ref instanceof THREE.Object3D) || !('lnglat' in props)) {
    return;
  }

  const { invalidate } = useThrelte();

  let currentRef = ref;
  let currentProps = props;

  const applyProps = () => {
    if (currentProps.lnglat === undefined) {
      return;
    }

    latLngToVector3Relative(currentProps.lnglat, center, vec3);

    currentRef.position.set(vec3.y, -vec3.x, 0);

    invalidate();
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

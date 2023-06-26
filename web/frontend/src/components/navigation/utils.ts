import { type Map, MercatorCoordinate } from 'maplibre-gl';
import * as THREE from 'three';

export const EARTH_RADIUS_METERS = 6_371_010;

/**
 * Converts WGS84 latitude and longitude to (uncorrected) WebMercator meters.
 * (WGS84 --> WebMercator (EPSG:3857))
 */
export const latLngToXY = (
  position: { lng: number, lat: number, alt?: number }
): [lng: number, lat: number, alt?: number] => {
  return [
    EARTH_RADIUS_METERS * THREE.MathUtils.degToRad(position.lng),
    EARTH_RADIUS_METERS * Math.log(Math.tan(0.25 * Math.PI + 0.5 * THREE.MathUtils.degToRad(position.lat))),
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

export const createCameraTransform = (map: Map) => {
  const centerLngLat = map.getCenter();
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

export const cameraPerspectiveToOrtho = (
  perspective: THREE.PerspectiveCamera,
  orthographic: THREE.OrthographicCamera
) => {
  const perspectiveMatrix = perspective.projectionMatrix;

  // Create a new orthographic projection matrix
  const orthographicMatrix = new THREE.Matrix4();

  // Get the inverse of the perspective matrix
  const inversePerspectiveMatrix = new THREE.Matrix4()
    .copy(perspectiveMatrix)
    .invert();

  // Extract the orthographic parameters
  const left = -1;
  const right = 1;
  const top = 1;
  const bottom = -1;
  const near = 1;
  const far = 1000;

  // Transform the orthographic parameters using the inverse perspective matrix
  const transformedLeft = left * inversePerspectiveMatrix.elements[0]!;
  const transformedRight = right * inversePerspectiveMatrix.elements[0]!;
  const transformedTop = top * inversePerspectiveMatrix.elements[5]!;
  const transformedBottom = bottom * inversePerspectiveMatrix.elements[5]!;
  const transformedNear = near - inversePerspectiveMatrix.elements[14]! / inversePerspectiveMatrix.elements[10]!;
  const transformedFar = far - inversePerspectiveMatrix.elements[14]! / inversePerspectiveMatrix.elements[10]!;

  // Set the orthographic projection matrix parameters
  orthographicMatrix.makeOrthographic(
    transformedLeft,
    transformedRight,
    transformedTop,
    transformedBottom,
    transformedNear,
    transformedFar
  );

  /*
   * Apply the transformation to your objects or camera
   * For example, if you have a camera, you can set its projection matrix
   */
  orthographic.projectionMatrix.copy(orthographicMatrix);
};

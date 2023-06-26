import * as THREE from 'three';
import { injectPlugin, useThrelte } from '@threlte/core';
import { latLngToVector3Relative } from './utils';

const vec3 = new THREE.Vector3();

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

    latLngToVector3Relative(currentProps.lnglat, undefined, vec3);

    currentRef.position.set(-vec3.x, 0, vec3.y);

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

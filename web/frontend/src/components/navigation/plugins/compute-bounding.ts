import * as THREE from 'three';
import { injectPlugin } from '@threlte/core';
import { zooms } from '../stores';

export const computeBoundingPlugin = () => injectPlugin('computeBounding', ({ ref }) => {
  let currentRef: THREE.BufferGeometry = ref;

  if (!(currentRef instanceof THREE.BufferGeometry)) {
    return;
  }

  return {
    onRefChange (nextRef: THREE.BufferGeometry) {
      currentRef = nextRef;

      currentRef.computeBoundingSphere();
      const radius = currentRef.boundingSphere?.radius;
      console.log(radius);

      if (radius) {
        zooms[currentRef.name] = radius;
      }

      return () => {
        delete zooms[currentRef.name];
      };
    },
  };
});

import * as THREE from 'three';
import { injectPlugin } from '@threlte/core';
import { zooms } from '../stores';

export const zoomPlugin = () => injectPlugin<{ obstacle: string }>('plugin-name', ({ ref, props }) => {
  let currentProps = props;
  let currentRef = ref;

  if (!(ref instanceof THREE.Mesh)) {
    return;
  }

  const setZoom = () => {
    currentRef.geometry.computeBoundingSphere();
    zooms[currentProps.obstacle] = 1 / currentRef.geometry.boundingSphere!.radius;
  };

  return {
    onRefChange (nextRef: THREE.Mesh) {
      currentRef = nextRef;

      if (currentProps.obstacle === undefined) {
        return;
      }

      setZoom();
    },
    onPropsChange (nextProps) {
      currentProps = nextProps;

      if (currentProps.obstacle !== undefined) {
        setZoom();
      }
    },
  };
});

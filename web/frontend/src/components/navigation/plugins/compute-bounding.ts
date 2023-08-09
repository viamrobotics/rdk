import * as THREE from 'three';
import { injectPlugin } from '@threlte/core';
import { boundingRadius } from '../stores';

type Props = { computeBounding: string }

export const computeBoundingPlugin = () => injectPlugin<Props>('computeBounding', ({ ref, props }) => {
  let currentRef: THREE.BufferGeometry = ref;
  let currentProps = props;

  if (!(currentRef instanceof THREE.BufferGeometry) || !currentProps.computeBounding) {
    return;
  }

  const handleChange = () => {
    currentRef.computeBoundingSphere();
    const radius = currentRef.boundingSphere?.radius;

    if (radius) {
      boundingRadius[currentProps.computeBounding] = radius;
    }
  };

  return {
    onRefChange (nextRef: THREE.BufferGeometry) {
      currentRef = nextRef;
      handleChange();

      return () => {
        delete boundingRadius[currentProps.computeBounding];
      };
    },
    onPropsChange (nextProps) {
      currentProps = nextProps;
      handleChange();
    },
  };
});

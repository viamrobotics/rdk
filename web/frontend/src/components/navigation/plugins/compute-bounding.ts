import * as THREE from 'three';
import { injectPlugin } from '@threlte/core';
import { zooms } from '../stores';

type Props = { computeBounding: string }

const scaleAndInvert = (input: number): number => {
  const upperBoundMeters = 40_075_000;
  // Ensure input is within the range of 1 to upperBoundMeters
  const clampedInput = Math.min(Math.max(input, 1), upperBoundMeters);

  // Scale the clamped input to the output range of 2 to 22
  const scaledOutput = (clampedInput - 1) / (upperBoundMeters - 1) * (22 - 2) + 2;


  // Invert the scaled output
  const invertedOutput = 22 - scaledOutput;

  return invertedOutput;
};

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
      zooms[currentProps.computeBounding] = scaleAndInvert(radius);
    }
  };

  return {
    onRefChange (nextRef: THREE.BufferGeometry) {
      currentRef = nextRef;
      handleChange();

      return () => {
        delete zooms[currentProps.computeBounding];
      };
    },
    onPropsChange (nextProps) {
      currentProps = nextProps;
      handleChange();
    },
  };
});

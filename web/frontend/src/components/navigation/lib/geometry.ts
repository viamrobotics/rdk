import type { Geometry, BoxGeometry, CapsuleGeometry, SphereGeometry } from '@viamrobotics/prime-blocks';
import type { Shapes } from './types';
import { ViamObject3D } from '@viamrobotics/three';

export const defaultSize = 5;

export const createGeometry = (type: Shapes): Geometry => {
  switch (type) {
    case 'box': {
      return {
        type,
        length: defaultSize * 2,
        width: defaultSize * 2,
        height: defaultSize * 2,
        pose: new ViamObject3D(),
      } satisfies BoxGeometry;
    }
    case 'sphere': {
      return {
        type,
        radius: defaultSize,
        pose: new ViamObject3D(),
      } satisfies SphereGeometry;
    }
    case 'capsule': {
      return {
        type,
        radius: defaultSize,
        length: defaultSize * 2,
        pose: new ViamObject3D(),
      } satisfies CapsuleGeometry;
    }
  }
};

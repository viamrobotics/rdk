import type { Geometry, BoxGeometry, CapsuleGeometry, SphereGeometry } from '@/api/navigation';
import type { Shapes } from './types';

export const defaultSize = 5;

export const createGeometry = (type: Shapes): Geometry => {
  switch (type) {
    case 'box': {
      return {
        type,
        length: defaultSize * 2,
        width: defaultSize * 2,
        height: defaultSize * 2,
        quaternion: { x: 0, y: 0, z: 0, w: 0 },
      } satisfies BoxGeometry;
    }
    case 'sphere': {
      return {
        type,
        radius: defaultSize,
        quaternion: { x: 0, y: 0, z: 0, w: 0 },
      } satisfies SphereGeometry;
    }
    case 'capsule': {
      return {
        type,
        radius: defaultSize,
        length: defaultSize * 2,
        quaternion: { x: 0, y: 0, z: 0, w: 0 },
      } satisfies CapsuleGeometry;
    }
  }
};

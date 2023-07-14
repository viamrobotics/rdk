export const defaultSize = 5000;

export const createGeometry = (type: 'box' | 'sphere' | 'capsule') => {
  switch (type) {
    case 'box': {
      return {
        type,
        x: defaultSize * 2,
        y: defaultSize * 2,
        z: defaultSize * 2,
        translation: { x: 0, y: 0, z: 0 },
      };
    }
    case 'sphere': {
      return {
        type,
        r: defaultSize,
        translation: { x: 0, y: 0, z: 0 },
      };
    }
    case 'capsule': {
      return {
        type,
        r: defaultSize,
        l: defaultSize * 2,
        translation: { x: 0, y: 0, z: 0 },
      };
    }
  }
};

export const fixArmStatus = (old: {
  is_moving: boolean
  end_position: Record<string, unknown>
  joint_positions: {
    values: unknown[]
  }
}) => {
  const newStatus: {
    pos_pieces: {
      endPosition: unknown
      endPositionValue: unknown
    }[]
    joint_pieces: unknown[]
    is_moving: boolean
  } = {
    pos_pieces: [],
    joint_pieces: [],
    is_moving: old.is_moving || false,
  };

  const fieldSetters = [
    ['x', 'X'],
    ['y', 'Y'],
    ['z', 'Z'],
    ['theta', 'Theta'],
    ['o_x', 'OX'],
    ['o_y', 'OY'],
    ['o_z', 'OZ'],
  ];

  for (const fieldSetter of fieldSetters) {
    const [endPositionField] = fieldSetter;
    newStatus.pos_pieces.push(
      {
        endPosition: fieldSetter,
        endPositionValue: old.end_position[endPositionField!] || 0,
      }
    );
  }

  /*
   * this conditional is in place so the RC card renders when
   * the fake arm is not using any kinematics file
   */
  if (old.joint_positions.values === undefined) {
    newStatus.joint_pieces.push(
      {
        joint: 0,
        jointValue: 100,
      }
    );
  } else {
    for (let j = 0; j < old.joint_positions.values.length; j += 1) {
      newStatus.joint_pieces.push(
        {
          joint: j,
          jointValue: old.joint_positions.values[j] || 0,
        }
      );
    }
  }

  return newStatus;
};

export const fixBoardStatus = (old: { analogs: unknown[]; digital_interrupts: unknown[] }) => {
  return {
    analogsMap: old.analogs || [],
    digitalInterruptsMap: old.digital_interrupts || [],
  };
};

export const fixGantryStatus = (old: {
  is_moving: boolean
  lengths_mm: number[]
  positions_mm: number[]
}) => {
  const newStatus: {
    parts: {
      axis: number,
      pos: number,
      length: number
    }[]
    is_moving: boolean
  } = {
    parts: [],
    is_moving: old.is_moving || false,
  };

  if (old.lengths_mm.length !== old.positions_mm.length) {
    throw new Error('gantry lists different lengths');
  }

  for (let i = 0; i < old.lengths_mm.length; i += 1) {
    newStatus.parts.push({
      axis: i,
      pos: old.positions_mm[i]!,
      length: old.lengths_mm[i]!,
    });
  }

  return newStatus;
};

export const fixInputStatus = (old: {
  events: {
    time: unknown
    event: string
    control: string
    value: number
  }[]
}) => {
  const events = old.events || [];
  const eventsList = events.map((event) => {
    return {
      time: event.time || {},
      event: event.event || '',
      control: event.control || '',
      value: event.value || 0,
    };
  });
  return { eventsList };
};

export const fixMotorStatus = (old: {
  is_powered: boolean
  position: number
  is_moving: boolean
}) => {
  return {
    isPowered: old.is_powered || false,
    position: old.position || 0,
    isMoving: old.is_moving || false,
  };
};

export const fixServoStatus = (old: {
  position_deg: number
  is_moving: boolean
}) => {
  return {
    positionDeg: old.position_deg || 0,
    is_moving: old.is_moving || false,
  };
};

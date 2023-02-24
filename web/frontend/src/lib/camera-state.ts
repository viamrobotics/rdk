interface StreamState {
  'on' : boolean,
  'live' : boolean,
  'name' : string
}

export const cameraStreamStates = new Map<string, StreamState>();

export const selectedMap = {
  Live: -1,
  'Manual Refresh': 0,
  'Every 30 Seconds': 30,
  'Every 10 Seconds': 10,
  'Every Second': 1,
} as const;

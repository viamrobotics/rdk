export interface StreamState {
  'on' : boolean,
  'live' : boolean,
  'name' : string
}

export const cameraStreamStates = new Map<string, StreamState>();

export const selectedMap = {
  Live: -1,
  'Manual refresh': 0,
  'Every 30 seconds': 30,
  'Every 10 seconds': 10,
  'Every second': 1,
} as const;

interface StreamState {
  'on' : boolean,
  'live' : boolean
}

export const cameraStreamStates = new Map<string, StreamState>();
export const baseStreamStates = new Map<string, boolean>();

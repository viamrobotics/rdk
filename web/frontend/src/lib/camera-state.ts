interface StreamState {
  'on' : boolean,
  'live' : boolean,
  'parent' : string,
  'name' : string
}

export const cameraStreamStates = new Map<string, StreamState>();

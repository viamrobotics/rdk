interface StreamState {
  'on' : boolean,
  'live' : boolean,
  'name' : string
}

export const cameraStreamStates = new Map<string, StreamState>();

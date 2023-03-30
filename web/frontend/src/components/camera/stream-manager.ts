import {
  type Client,
  StreamClient,
} from '@viamrobotics/sdk';
import { CameraManager } from './camera-manager';

export class StreamManager {
  streamClient: StreamClient;

  constructor (
    private client:Client,
    public cameraManagers: Map<string, CameraManager> = new Map<string, CameraManager>()

  ) {
    this.streamClient = new StreamClient(client);
  }

  setCameraManager (cameraName:string) {
    const manager = this.cameraManagers.get(cameraName);
    if (manager) {
      return manager;
    }
    const tempManager = new CameraManager(this.client, cameraName, this.streamClient);
    this.cameraManagers.set(cameraName, tempManager);
    return tempManager;
  }

  refreshStreams () {
    for (const camera of this.cameraManagers.values()) {
      // Clean up previous camera managers
      if (camera.streamCount > 0) {
        camera.open();
      }
    }
  }
}

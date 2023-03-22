import type {
  Client,
} from '@viamrobotics/sdk';
import { CameraManager } from './camera-manager';

export class StreamManager {
  isConnected: boolean;

  cameraManagers: Map<string, CameraManager>;

  client: Client;

  toggleRefresh: boolean;

  constructor (client:Client) {
    this.isConnected = true;
    this.cameraManagers = new Map<string, CameraManager>();
    this.client = client;
  }

  setCameraManager (cameraName:string) {
    const tempManager = new CameraManager(cameraName, this.client);
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

import type {
  Client,
  commonApi,
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
    this.toggleRefresh = false;
  }

  setCameraManager (cameraName:string) {
    const tempManager = new CameraManager(cameraName, this.client);
    this.cameraManagers.set(cameraName, tempManager);
    return tempManager;
  }

  refreshStreams () {
    for (const camera of this.cameraManagers.values()) {
      // Clean up previous camera managers
      const tempCam = this.cameraManagers.get(camera.cameraName);
      this.cameraManagers.set(camera.cameraName, new CameraManager(camera.cameraName, this.client));
      const currCam = this.cameraManagers.get(camera.cameraName);
      if (tempCam && tempCam.streamCount > 0) {
        tempCam.close();
        if (currCam) {
          currCam.streamCount = tempCam.streamCount - 1;
          currCam.addStream();
        }
      }
    }
    this.toggleRefresh = !this.toggleRefresh;
  }
}

import {
  type Client,
  StreamClient,
} from '@viamrobotics/sdk';
import { CameraManager } from './camera-manager';

export class StreamManager {

  cameraManagers: Map<string, CameraManager>;

  client: Client;

  streamClient:StreamClient;

  constructor (client:Client) {
    this.cameraManagers = new Map<string, CameraManager>();
    this.client = client;
    this.streamClient = new StreamClient(client);
  }

  setCameraManager (cameraName:string) {
    const tempManager = new CameraManager(cameraName, this.client, this.streamClient);
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

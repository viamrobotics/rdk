import type {
  Client,
  commonApi,
} from '@viamrobotics/sdk';
import { CameraManager } from './camera-manager';

export class StreamManager {
  isConnected: boolean;

  cameraList: commonApi.ResourceName.AsObject[];

  cameraManagers: Map<string, CameraManager>;

  client: Client;

  constructor (cameras:commonApi.ResourceName.AsObject[], client:Client) {
    this.isConnected = true;
    this.cameraList = [];
    this.cameraManagers = new Map<string, CameraManager>();
    this.client = client;
    // this.refreshStreams();
  }

  setCameraManager (cameraName:string) {
    const tempManager = new CameraManager(cameraName,this.client);
    this.cameraManagers.set(cameraName, tempManager)
    return tempManager
  }

  refreshStreams () {
    console.log(this.cameraList);
    for (const camera of this.cameraList) {
      // Clean up previous
      const tempCam = this.cameraManagers.get(camera.name);
      this.cameraManagers.set(camera.name, new CameraManager(camera.name, this.client));
      const currCam = this.cameraManagers.get(camera.name);
      if (tempCam) {
        tempCam.close();
        const tempManager = this.cameraManagers.get(camera.name);
        if (tempManager && currCam) {
          currCam.streamCount = tempManager.streamCount;
          currCam.addStream();
        }
      }
    }
  }

  updateResources (cameraList:commonApi.ResourceName.AsObject[]) {
    this.cameraList = cameraList;
    this.refreshStreams();
  }
}

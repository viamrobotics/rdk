import { CameraClient, ServiceError, StreamClient, type Client } from '@viamrobotics/sdk';
import { displayError } from '../../lib/error';

export class CameraManager {
  cameraName:string;

  cameraClient:CameraClient;

  client:Client;

  streamCount:number;

  stream:StreamClient;

  public videoStream:MediaStream;

  constructor (cameraName:string, client:Client, streamClient:StreamClient) {
    this.cameraName = cameraName;
    this.cameraClient = new CameraClient(client, cameraName);
    this.client = client;
    this.streamCount = 0;
    this.stream = streamClient;
    this.videoStream = new MediaStream();
  }

  addStream () {
    if (this.streamCount === 0) {
      this.open();
    }
    this.streamCount += 1;
  }

  removeStream () {
    this.streamCount -= 1;
    if (this.streamCount === 0) {
      this.close();
    }
  }

  open () {
    this.stream.add(this.cameraName);
    this.stream.on('track', (event) => {
      const [eventStream] = event.streams;
      if (!eventStream) {
        throw new Error('expected event stream to exist');
      }
      // Ignore event if received for the wrong stream, in the case of multiple cameras
      if (eventStream.id !== this.cameraName) {
        return;
      }
      this.videoStream = eventStream;
    });
  }

  close () {
    this.stream.remove(this.cameraName);
  }

  async setImageSrc (imgEl:HTMLImageElement|undefined) {
    let blob;
    try {
      blob = await this.cameraClient.renderFrame('image/jpeg');
    } catch (error) {
      displayError(error as ServiceError);
      return;
    }

    imgEl?.setAttribute('src', URL.createObjectURL(blob));
  }

}

import { CameraClient, ServiceError, StreamClient, type Client } from '@viamrobotics/sdk';
import { displayError } from '../../lib/error';

export class CameraManager {
  cameraName:string;

  client:Client;

  streamCount:number;

  stream:StreamClient;

  public VideoElement:MediaStream;

  constructor (cameraName:string, client:Client) {
    this.cameraName = cameraName;
    this.client = client;
    this.streamCount = 0;
    this.stream = new StreamClient(client);
    this.VideoElement = new MediaStream();
  }

  addStream () {
    console.log("Stream Count Add:" + this.streamCount)
    if (this.streamCount === 0) {
      console.log('opening stream')
      this.open();
    }
    this.streamCount += 1;
  }

  removeStream () {
    console.log("Stream Count Remove:" + this.streamCount)
    this.streamCount -= 1;
    if (this.streamCount === 0) {
      console.log('Closing Stream')
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
      this.VideoElement = eventStream;
    });
  }

  close () {
    this.stream.remove(this.cameraName);
  }

  async setImageElement (imgEl:HTMLImageElement|undefined) {
    let blob;
    try {
      blob = await new CameraClient(this.client, this.cameraName).renderFrame('image/jpeg');
    } catch (error) {
      displayError(error as ServiceError);
      return;
    }

    imgEl?.setAttribute('src', URL.createObjectURL(blob));
  }

}

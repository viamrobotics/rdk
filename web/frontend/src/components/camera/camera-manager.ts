import {
  CameraClient,
  type ServiceError,
  StreamClient,
  type Client,
} from '@viamrobotics/sdk';
import { displayError } from '../../lib/error';

export class CameraManager {
  cameraClient: CameraClient;

  onOpen: (() => void) | undefined;

  constructor(
    client: Client,
    private cameraName: string,
    private streamClient: StreamClient,
    public streamCount = 0,
    public videoStream: MediaStream = new MediaStream()
  ) {
    this.cameraClient = new CameraClient(client, cameraName);
  }

  addStream() {
    console.log("addStream()")
    if (this.streamCount === 0) {
      console.log("addStream(): this.streamCount === 0")
      this.open();
    }
    this.streamCount += 1;
    console.log("addStream(): this.streamCount: ", this.streamCount)
  }

  removeStream() {
    console.log("removeStream()")
    this.streamCount -= 1;
    if (this.streamCount === 0) {
      console.log("removeStream(): this.streamCount === 0")
      this.close();
    }
  }

  open() {
    console.log("open(): calling streamClient.add() with parameter: ", this.cameraName)
    this.streamClient.add(this.cameraName);
    this.streamClient.on('track', (event) => {
      console.log("open(): cameraClient got track: ", event)
      const [eventStream] = (event as { streams: MediaStream[] }).streams;
      if (!eventStream) {
        throw new Error('expected event stream to exist');
      }
      // Ignore event if received for the wrong stream, in the case of multiple cameras
      if (eventStream.id !== this.cameraName) {
        return;
      }
      this.videoStream = eventStream;
      this.onOpen?.();
    });
  }

  close() {
    console.log("close()")
    this.streamClient.remove(this.cameraName);
  }

  async setImageSrc(imgEl: HTMLImageElement | undefined) {
    console.log("setImageSrc()", imgEl)
    let blob;
    try {
      blob = await this.cameraClient.renderFrame('image/jpeg');
      console.log("setImageSrc(): got blob")
    } catch (error) {
      displayError(error as ServiceError);
      return;
    }

    imgEl?.setAttribute('src', URL.createObjectURL(blob));
  }
}

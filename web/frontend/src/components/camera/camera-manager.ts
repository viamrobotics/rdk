import { CameraClient, type ServiceError, StreamClient, type Client } from '@viamrobotics/sdk';
import { displayError } from '../../lib/error';
import { rcLogConditionally } from '@/lib/log';

export class CameraManager {
  cameraClient: CameraClient;

  onOpen: (() => void) | undefined;

  constructor (
    client: Client,
    private cameraName: string,
    private streamClient: StreamClient,
    public streamCount = 0,
    public videoStream: MediaStream = new MediaStream()
  ) {
    this.cameraClient = new CameraClient(client, cameraName, { requestLogger: rcLogConditionally });
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
    this.streamClient.add(this.cameraName);
    this.streamClient.on('track', (event) => {
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

  close () {
    this.streamClient.remove(this.cameraName);
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

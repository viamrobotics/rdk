import CameraServiceClient from 'proto/api/component/v1/camera_pb_service';
import {
    CameraServiceRenderFrameRequest,
    CameraServiceGetFrameResponse,
    CameraServiceGetObjectPointCloudsResponse
} from "proto/api/component/v1/camera_pb";

class CameraClientWrapper {
    client: any;
  
    constructor() {
    //   this.client = new CameraServiceClient(window.webrtcHost, { transport: transportFactory });
    }

    renderFrame(name: string, mimeType: string): Promise<CameraServiceGetFrameResponse | undefined> {
        const renderFrameRequest = new CameraServiceRenderFrameRequest();
        renderFrameRequest.setName(name);
        renderFrameRequest.setMimeType(mimeType)
        return new Promise((resolve, reject) => {
            // this.client.renderFrame(renderFrameRequest, {}, function(
            //     err,
            //     response
            // ) {
            //     if (err) {
            //     reject(err);
            //     return;
            //     }
            //     resolve(response.getData_asU8());
            // });
            reject();
        });
    }

}

export default CameraClientWrapper;
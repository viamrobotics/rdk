// royaleserver.cpp

#include <chrono>
#include <iostream>
#include <royale.hpp>
#include <thread>

#include "cameraserver.h"

//#define DEBUG(x) std::cout << x << std::endl
#define DEBUG(x)

using namespace royale;
using namespace std;

class MyListener : public IDepthDataListener {
   public:
    void onNewData(const DepthData* data) {
        cout << "got data: " << data->width << " x " << data->height << endl;
        {
            int x = data->width / 2;
            int y = data->height / 2;

            x++;

            int k = x + (y * data->width);

            auto val = data->points.at(k);

            cout << "\t" << x << "," << y << " " << k << " z:" << val.z
                 << " confidence:" << int(val.depthConfidence) << "|" << endl;
        }

        std::shared_ptr<CameraOutput> output(new CameraOutput());
        output->width = data->width;
        output->height = data->height;

        float min = 100000;
        float max = 0;

        {
            std::stringbuf buffer;
            std::ostream os(&buffer);

            os << "VERSIONX\n";
            os << "2\n";
            os << ".001\n";
            os << output->width << "\n";
            os << output->height << "\n";

            for (int y = 0; y < output->height; y++) {
                for (int x = 0; x < output->width; x++) {
                    int k = x + (y * data->width);
                    auto val = data->points.at(k);

                    if (val.z > 0) {
                        if (val.z < min) min = val.z;
                        if (val.z > max) max = val.z;
                    }

                    short s = short(1000 * val.z);

                    buffer.sputn((const char*)&s, 2);
                }
            }
            output->depth = buffer.str();
        }

        {
            std::stringbuf buffer;
            std::ostream os(&buffer);

            os << "P6\n" << output->width << " " << output->height << "\n255\n";

            float span = max - min;

            for (int y = 0; y < output->height; y++) {
                for (int x = 0; x < output->width; x++) {
                    int k = x + (y * data->width);
                    auto val = data->points.at(k);

                    char clr = 0;

                    if (val.z > 0) {
                        auto ratio = (val.z - min) / span;
                        clr = (char)(60 + (int)(ratio * 192));
                    }

                    os << (char)clr;
                    os << (char)clr;
                    os << (char)clr;
                }
            }
            output->ppmdata = buffer.str();
        }

        CameraState::get()->cameras[0] = output;

        CameraState::get()->ready = 1;
        /*
        std::lock_guard<std::mutex> lock (flagMutex);

        // create two images which will be filled afterwards
        // each image containing one 32Bit channel
        zImage.create (Size (data->width, data->height), CV_32FC1);
        grayImage.create (Size (data->width, data->height), CV_32FC1);

        // set the image to zero
        zImage = Scalar::all (0);
        grayImage = Scalar::all (0);

        int k = 0;
        for (int y = 0; y < zImage.rows; y++)
        {
            float *zRowPtr = zImage.ptr<float> (y);
            float *grayRowPtr = grayImage.ptr<float> (y);
            for (int x = 0; x < zImage.cols; x++, k++)
            {
                auto curPoint = data->points.at (k);
                if (curPoint.depthConfidence > 0)
                {

                    //cout << "\t " << x << "," << y << " " << k << " : " <<
    curPoint.z << endl;
                    // if the point is valid, map the pixel from 3D world
                    // coordinates to a 2D plane (this will distort the image)
                    zRowPtr[x] = adjustZValue (curPoint.z);
                    grayRowPtr[x] = adjustGrayValue (curPoint.grayValue);
                }
            }
        }

        // create images to store the 8Bit version (some OpenCV
        // functions may only work on 8Bit images)
        zImage8.create (Size (data->width, data->height), CV_8UC1);
        grayImage8.create (Size (data->width, data->height), CV_8UC1);

        // convert images to the 8Bit version
        // This sample uses a fixed scaling of the values to (0, 255) to avoid
    flickering.
        // You can also replace this with an automatic scaling by using
        // normalize(zImage, zImage8, 0, 255, NORM_MINMAX, CV_8UC1)
        // normalize(grayImage, grayImage8, 0, 255, NORM_MINMAX, CV_8UC1)
        zImage.convertTo (zImage8, CV_8UC1);
        grayImage.convertTo (grayImage8, CV_8UC1);

        if (undistortImage)
        {
            // call the undistortion function on the z image
            Mat temp = zImage8.clone();
            undistort (temp, zImage8, cameraMatrix, distortionCoefficients);
        }

        // scale and display the depth image
        scaledZImage.create (Size (data->width * 4, data->height * 4), CV_8UC1);
        resize (zImage8, scaledZImage, scaledZImage.size());

        imshow ("Depth", scaledZImage);

        if (undistortImage)
        {
            // call the undistortion function on the gray image
            Mat temp = grayImage8.clone();
            undistort (temp, grayImage8, cameraMatrix, distortionCoefficients);
        }

        // scale and display the gray image
        scaledGrayImage.create (Size (data->width * 4, data->height * 4),
    CV_8UC1); resize (grayImage8, scaledGrayImage, scaledGrayImage.size());

        imshow ("Gray", scaledGrayImage);
    }
        */
    }

    void setLensParameters(const LensParameters& lensParameters) {
        /*
        // Construct the camera matrix
        // (fx   0    cx)
        // (0    fy   cy)
        // (0    0    1 )
        cameraMatrix = (Mat1d (3, 3) << lensParameters.focalLength.first, 0,
        lensParameters.principalPoint.first, 0,
        lensParameters.focalLength.second, lensParameters.principalPoint.second,
                        0, 0, 1);

        // Construct the distortion coefficients
        // k1 k2 p1 p2 k3
        distortionCoefficients = (Mat1d (1, 5) <<
        lensParameters.distortionRadial[0], lensParameters.distortionRadial[1],
                                  lensParameters.distortionTangential.first,
                                  lensParameters.distortionTangential.second,
                                  lensParameters.distortionRadial[2]);
        */
    }
};

/*
void cameraThread() {
    rs2::context ctx;

    std::vector<rs2::pipeline> pipelines;

    for (auto&& dev : ctx.query_devices()) {
        auto serial = dev.get_info(RS2_CAMERA_INFO_SERIAL_NUMBER);
        std::cout << "got serial: " << serial << std::endl;

        rs2::config cfg;
        cfg.enable_device(serial);
        cfg.enable_stream(RS2_STREAM_DEPTH);
        cfg.enable_stream(RS2_STREAM_COLOR);

        rs2::pipeline pipe(ctx);
        pipe.start(cfg);
        pipelines.push_back(pipe);

        CameraState::get()->cameras.push_back(0);
    }

    if (pipelines.size() == 0) {
        std::cerr << "no cameras found" << std::endl;
        exit(-1);
    }

    rs2::align alignment(RS2_STREAM_COLOR);
    // rs2::align alignment(RS2_STREAM_DEPTH);

    while (true) {
        auto start = std::chrono::high_resolution_clock::now();

        int num = 0;
        for (auto& p : pipelines) {
            std::shared_ptr<CameraOutput> output(new CameraOutput());
            rs2::frameset frames = p.wait_for_frames();

            // this handles the geometry so that the
            // x/y of the depth and color are the same
            frames = alignment.process(frames);

            // do color frame
            auto vf = frames.get_color_frame();

            assert(vf.get_bytes_per_pixel() == 3);
            assert(vf.get_stride_in_bytes() ==
                   (vf.get_width() * vf.get_bytes_per_pixel()));

            output->width = vf.get_width();
            output->height = vf.get_height();
            output->ppmdata =
                my_write_ppm((const char*)vf.get_data(), vf.get_width(),
                             vf.get_height(), vf.get_bytes_per_pixel());

            // create depth map

            auto depth = frames.get_depth_frame();
            output->add_depth(depth.get_bytes_per_pixel(),
                              depth.get_units(),
                              depth.get_width(),
                              depth.get_height(),
                              (const char*)depth.get_data());


            CameraState::get()->cameras[num++] = output;
        }

        auto finish = std::chrono::high_resolution_clock::now();
        DEBUG(std::chrono::duration_cast<std::chrono::milliseconds>(finish -
                                                                    start)
                  .count()
              << "ms");

        CameraState::get()->ready = 1;

        if (time(0) - CameraState::get()->lastRequest > 30) {
            DEBUG("sleeping");
            sleep(1);
        }
    }
}
*/

int main(int argc, char** argv) {
    int port = 8181;

    httpserver::webserver ws = httpserver::create_webserver(port);
    installWebHandlers(&ws);

    MyListener listener;

    std::unique_ptr<ICameraDevice> cameraDevice;

    {
        CameraManager manager;

        // if no argument was given try to open the first connected camera
        royale::Vector<royale::String> camlist(
            manager.getConnectedCameraList());
        cout << "Detected " << camlist.size() << " camera(s)." << endl;
        if (camlist.empty()) {
            std::cerr << "No suitable camera device detected." << std::endl
                      << "Please make sure that a supported camera is plugged "
                         "in, all drivers are "
                      << "installed, and you have proper USB permission"
                      << std::endl;
            return 1;
        }

        cameraDevice = manager.createCamera(camlist[0]);
        camlist.clear();
    }

    if (!cameraDevice) {
        std::cerr << "Cannot create the camera device" << std::endl;
        return 1;
    }

    auto status = cameraDevice->initialize();
    if (status != CameraStatus::SUCCESS) {
        cerr << "Cannot initialize the camera device, error string : "
             << getErrorString(status) << endl;
        return 1;
    }

    status = cameraDevice->setUseCase("Low_Noise_Extended");
    if (status != CameraStatus::SUCCESS) {
        cerr << "Cannot initialize the camera device, error string : "
             << getErrorString(status) << endl;
        return 1;
    }

    // retrieve the lens parameters from Royale
    LensParameters lensParameters;
    status = cameraDevice->getLensParameters(lensParameters);
    if (status != CameraStatus::SUCCESS) {
        cerr << "Can't read out the lens parameters" << endl;
        return 1;
    }

    listener.setLensParameters(lensParameters);

    // register a data listener
    if (cameraDevice->registerDataListener(&listener) !=
        CameraStatus::SUCCESS) {
        cerr << "Error registering data listener" << endl;
        return 1;
    }

    if (cameraDevice->startCapture() != CameraStatus::SUCCESS) {
        cerr << "Error starting the capturing" << endl;
        return 1;
    }

    CameraState::get()->cameras.push_back(
        0);  // signal we have one and only 1 camera

    std::cout << "Starting to listen on port: " << port << std::endl;

    ws.start(true);

    return 0;
}

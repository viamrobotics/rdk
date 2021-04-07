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
    }
};

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

    // signal we have one and only 1 camera
    CameraState::get()->cameras.push_back(0);

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

    if (cameraDevice->registerDataListener(&listener) !=
        CameraStatus::SUCCESS) {
        cerr << "Error registering data listener" << endl;
        return 1;
    }

    if (cameraDevice->startCapture() != CameraStatus::SUCCESS) {
        cerr << "Error starting the capturing" << endl;
        return 1;
    }

    std::cout << "Starting to listen on port: " << port << std::endl;

    ws.start(true);

    return 0;
}

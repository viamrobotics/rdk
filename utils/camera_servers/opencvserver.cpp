// opencvserver.cpp

#include <chrono>
#include <iostream>
#include <opencv2/videoio.hpp>
#include <thread>

#include "cameraserver.h"

//#define DEBUG(x) std::cout << x << std::endl
#define DEBUG(x)

std::shared_ptr<cv::VideoCapture> TheCamera;

std::string type2str(int type) {
    std::string r;

    uchar depth = type & CV_MAT_DEPTH_MASK;
    uchar chans = 1 + (type >> CV_CN_SHIFT);

    switch (depth) {
        case CV_8U:
            r = "8U";
            break;
        case CV_8S:
            r = "8S";
            break;
        case CV_16U:
            r = "16U";
            break;
        case CV_16S:
            r = "16S";
            break;
        case CV_32S:
            r = "32S";
            break;
        case CV_32F:
            r = "32F";
            break;
        case CV_64F:
            r = "64F";
            break;
        default:
            r = "User";
            break;
    }

    r += "C";
    r += (chans + '0');

    return r;
}

std::shared_ptr<cv::VideoCapture> findCamera() {
    for (auto i = 0; i <= 32; i++) {
        std::shared_ptr<cv::VideoCapture> camera(new cv::VideoCapture(i));
        if (camera->isOpened()) {
            return camera;
        }
    }

    std::cerr << "Could not open any camera" << std::endl;
    exit(-1);
}

void cameraThread() {
    while (true) {
        cv::Mat frame;
        *TheCamera >> frame;

        std::shared_ptr<CameraOutput> output(new CameraOutput());

        output->width = frame.cols;
        output->height = frame.rows;

        if (frame.type() != 16) {
            std::cerr << "don't know this type (" << frame.type() << ")"
                      << type2str(frame.type()) << " please finish"
                      << std::endl;
            exit(-1);
        }

        DEBUG("type: " << frame.type() << " " << type2str(frame.type())
                       << " w: " << output->width << " h: " << output->height
                       << " tota: " << frame.total());

        output->ppmdata = my_write_ppm((const char*)frame.data, output->width,
                                       output->height, 3);

        CameraState::get()->cameras[0] = output;

        CameraState::get()->ready = 1;

        if (time(0) - CameraState::get()->lastRequest > 30) {
            DEBUG("sleeping");
            sleep(1);
        }
    }
}

int main(int argc, char** argv) {
    TheCamera = findCamera();
    CameraState::get()->cameras.push_back(0);

    int port = 8182;

    httpserver::webserver ws = httpserver::create_webserver(port);
    installWebHandlers(&ws);

    std::thread t(cameraThread);

    std::cout << "Starting to listen on port: " << port << std::endl;
    ws.start(true);

    return 0;
}

// intelrealserver.cpp

#include <chrono>
#include <iostream>
#include <librealsense2/rs.hpp>
#include <thread>

#include "cameraserver.h"

//#define DEBUG(x) std::cout << x << std::endl
#define DEBUG(x)

std::string my_write_ppm(const char* pixels, int x, int y,
                         int bytes_per_pixel) {
    std::stringbuf buffer;
    std::ostream os(&buffer);

    os << "P6\n" << x << " " << y << "\n255\n";
    buffer.sputn((const char*)pixels, x * y * bytes_per_pixel);

    return buffer.str();
}

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
            output->add_depth(depth.get_bytes_per_pixel(), depth.get_units(),
                              depth.get_width(), depth.get_height(),
                              (const char*)depth.get_data());

            DEBUG("middle distance: " << depth.get_distance(
                      depth.get_width() / 2, depth.get_height() / 2));

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

int main(int argc, char** argv) {
    int port = 8181;

    httpserver::webserver ws = httpserver::create_webserver(port);
    installWebHandlers(&ws);

    std::thread t(cameraThread);

    std::cout << "Starting to listen on port: " << port << std::endl;

    ws.start(true);

    return 0;
}

// g++ -std=c++17 intelreal.cpp -lrealsense2 -lhttpserver

#include <iostream>
#include <thread>
#include <vector>

#include <httpserver.hpp>

#include <librealsense2/rs.hpp>

#define STB_IMAGE_WRITE_IMPLEMENTATION
#include "stb_image_write.h"

class CameraOutput {
public:
    std::string pngData;
    std::vector<uint64_t> depth;
};

std::shared_ptr<CameraOutput> CameraOutputInstance;
long long numRequests = 0; // so we can throttle and not kill cpu

void cameraThread() {
    // Create a Pipeline - this serves as a top-level API for streaming and processing frames
    rs2::pipeline p;
    
    rs2::config cfg;
    cfg.enable_stream(RS2_STREAM_DEPTH);
    cfg.enable_stream(RS2_STREAM_COLOR);
    auto profile = p.start(cfg);

    auto sensor = profile.get_device().first<rs2::depth_sensor>();

    rs2::align align_to_color(RS2_STREAM_COLOR);
    
    long long prevNumRequests = 0; 
    
    while (true) {
        std::shared_ptr<CameraOutput> output(new CameraOutput());
        // get next set of frames
        rs2::frameset frames = p.wait_for_frames();

        // make sure depth frame is aligned to color frame
        frames = align_to_color.process(frames);

        // do color frame
        auto vf = frames.get_color_frame();
        int len;
        auto out = stbi_write_png_to_mem((const unsigned char *)vf.get_data(),
                                         vf.get_stride_in_bytes(),
                                         vf.get_width(),
                                         vf.get_height(),
                                         vf.get_bytes_per_pixel(),
                                         &len);
        output->pngData = std::string((char*)out, len);
        STBIW_FREE(out);

        // create depth map
        auto depth = frames.get_depth_frame();
        output->depth.push_back(depth.get_width());
        output->depth.push_back(depth.get_height());
        for ( auto x = 0; x < depth.get_width(); x++ ) {
            for ( auto y = 0; y < depth.get_height(); y++ ) {
                uint64_t d = 1000 * depth.get_distance(x, y);
                output->depth.push_back(d);
            }
        }
        
        // replace output data with this
        CameraOutputInstance = output;

        if (numRequests == prevNumRequests) {
            sleep(1);
        } else {
            numRequests = prevNumRequests;
        }
    }
}

using namespace httpserver;

class hello_world_resource : public http_resource {
public:
    const std::shared_ptr<http_response> render(const http_request&) {
        return std::shared_ptr<http_response>(new string_response("Hello, World!\n"));
    }
};

class picture_resource : public http_resource {
public:
    const std::shared_ptr<http_response> render(const http_request&) {
        numRequests++;
        return std::shared_ptr<http_response>(new string_response(CameraOutputInstance->pngData, 200, "image/png"));
    }
};

class depth_resource : public http_resource {
public:
    const std::shared_ptr<http_response> render(const http_request&) {
        numRequests++;
        std::string s((char*)CameraOutputInstance->depth.data(), CameraOutputInstance->depth.size() * 8);
        return std::shared_ptr<http_response>(new string_response(s, 200, "binary"));
    }
};


int main(int argc, char** argv) {

    std::thread t(cameraThread);
    
    webserver ws = create_webserver(8181);

    hello_world_resource hwr;
    ws.register_resource("/", &hwr);

    picture_resource pr;
    ws.register_resource("/pic.png", &pr);

    depth_resource dr;
    ws.register_resource("/depth.dat", &dr);

    std::cout << "Starting to listen" << std::endl;
    
    ws.start(true);
    
    return 0;
}

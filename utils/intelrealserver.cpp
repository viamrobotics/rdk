// g++ -std=c++17 intelreal.cpp -lrealsense2 -lhttpserver

#include <iostream>
#include <thread>
#include <vector>
#include <chrono>
#include <string>

#include <httpserver.hpp>

#include <librealsense2/rs.hpp>

class CameraOutput {
public:
    int width;
    int height;
    std::string ppmdata;
    std::vector<uint64_t> depth;
};

std::vector<std::shared_ptr<CameraOutput>> CameraOutputInstance;
long long numRequests = 0; // so we can throttle and not kill cpu
bool ready = 0;

std::string my_write_ppm(const char *pixels, int x, int y, int bytes_per_pixel) {
    std::stringbuf buffer;
    std::ostream os (&buffer);

    os << "P6\n" << x << " " << y << "\n255\n";
    buffer.sputn((const char*)pixels, x * y * bytes_per_pixel);

    return buffer.str();
}

#define STB_IMAGE_WRITE_IMPLEMENTATION
#include "stb_image_write.h"

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

        CameraOutputInstance.push_back(0);
    }

    if (pipelines.size() == 0) {
        std::cerr << "no cameras found" << std::endl;
        exit(-1);
    }
    
    //rs2::align alignment(RS2_STREAM_COLOR);
    rs2::align alignment(RS2_STREAM_DEPTH);
    
    long long prevNumRequests = 0; 
    
    while (true) {

        auto start = std::chrono::high_resolution_clock::now();

        int num = 0;
        for (auto& p : pipelines) {
            
            std::shared_ptr<CameraOutput> output(new CameraOutput());
            rs2::frameset frames = p.wait_for_frames();
            
            frames = alignment.process(frames); // this handles the geometry so that the x/y of the depth and color are the same
            
            // do color frame
            auto vf = frames.get_color_frame();
            
            assert( vf.get_bytes_per_pixel() == 3);
            assert( vf.get_stride_in_bytes() == ( vf.get_width() * vf.get_bytes_per_pixel() ) );
            
            output->width = vf.get_width();
            output->height = vf.get_height();
            output->ppmdata = my_write_ppm((const char *)vf.get_data(),
                                           vf.get_width(),
                                           vf.get_height(),
                                           vf.get_bytes_per_pixel());
            
            
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

            CameraOutputInstance[num++] = output;
        }

        auto finish = std::chrono::high_resolution_clock::now();
        std::cout << std::chrono::duration_cast<std::chrono::milliseconds>(finish-start).count() << "ms\n";

        ready = 1;
        
        if (numRequests == prevNumRequests) {
            sleep(1);
        } else {
            numRequests = prevNumRequests;
        }
    }
}

using namespace httpserver;

int getCameraNumber(const http_request& r) {
    auto s = r.get_arg("num");
    if (s == "") {
        return 0;
    }
    return std::stoi(s);
}

class hello_world_resource : public http_resource {
public:
    const std::shared_ptr<http_response> render(const http_request&) {
        return std::shared_ptr<http_response>(new string_response("Hello, World!\n"));
    }
};

class picture_resource : public http_resource {
public:
    const std::shared_ptr<http_response> render(const http_request& r) {
        if (!ready) {
            return std::shared_ptr<http_response>(new string_response("not ready\n"));
        }
        numRequests++;
        int camNumera = getCameraNumber(r);
        return std::shared_ptr<http_response>(new string_response(CameraOutputInstance[camNumera]->ppmdata, 200, "image/ppm"));
    }
};

class picture_resource_png : public http_resource {
public:
    const std::shared_ptr<http_response> render(const http_request& r) {
        if (!ready) {
            return std::shared_ptr<http_response>(new string_response("not ready\n"));
        }
        numRequests++;

        int camNumera = getCameraNumber(r);
        
        std::shared_ptr<CameraOutput> mine(CameraOutputInstance[camNumera]);
        const char * raw_data = mine->ppmdata.c_str();
        int len = mine->width * mine->height * 3;
        raw_data = raw_data + (mine->ppmdata.size() - len);

        int pngLen;
        auto out = stbi_write_png_to_mem((const unsigned char*)raw_data, mine->width * 3, mine->width, mine->height, 3, &pngLen);
        std::string s((char*)out, pngLen);
        STBIW_FREE(out);

        return std::shared_ptr<http_response>(new string_response(s, 200, "image/png"));
    }
};

class depth_resource : public http_resource {
public:
    const std::shared_ptr<http_response> render(const http_request& r) {
        if (!ready) {
            return std::shared_ptr<http_response>(new string_response("not ready\n"));
        }
        int camNumera = getCameraNumber(r);
                
        numRequests++;
        std::string s((char*)CameraOutputInstance[camNumera]->depth.data(), CameraOutputInstance[camNumera]->depth.size() * 8);
        return std::shared_ptr<http_response>(new string_response(s, 200, "binary"));
    }
};


int main(int argc, char** argv) {

    std::thread t(cameraThread);
    
    webserver ws = create_webserver(8181);

    hello_world_resource hwr;
    ws.register_resource("/", &hwr);

    picture_resource pr;
    ws.register_resource("/pic.ppm", &pr);

    picture_resource_png pr2;
    ws.register_resource("/pic.png", &pr2);

    depth_resource dr;
    ws.register_resource("/depth.dat", &dr);

    std::cout << "Starting to listen" << std::endl;
    
    ws.start(true);
    
    return 0;
}

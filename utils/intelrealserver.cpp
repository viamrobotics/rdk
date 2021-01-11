// g++ -std=c++17 intelreal.cpp -lrealsense2 -lhttpserver

#include <chrono>
#include <httpserver.hpp>
#include <iostream>
#include <librealsense2/rs.hpp>
#include <string>
#include <thread>
#include <vector>

//#define DEBUG(x) std::cout << x << std::endl
#define DEBUG(x)

class CameraOutput {
   public:
    void add_depth(rs2::depth_frame& frame) {
        std::stringbuf buffer;
        std::ostream os(&buffer);

        os << "VERSIONX\n";
        os << frame.get_bytes_per_pixel() << "\n";
        os << frame.get_units() << "\n";
        os << frame.get_width() << "\n";
        os << frame.get_height() << "\n";

        buffer.sputn((const char*)frame.get_data(),
                     frame.get_width() * frame.get_height() *
                         frame.get_bytes_per_pixel());

        depth = buffer.str();
    }

    int width;
    int height;
    std::string ppmdata;
    std::string depth;
};

std::vector<std::shared_ptr<CameraOutput>> CameraOutputInstance;
bool ready = 0;
volatile time_t lastRequest = 0;

std::string my_write_ppm(const char* pixels, int x, int y,
                         int bytes_per_pixel) {
    std::stringbuf buffer;
    std::ostream os(&buffer);

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
            output->add_depth(depth);

            CameraOutputInstance[num++] = output;
        }

        auto finish = std::chrono::high_resolution_clock::now();
        DEBUG(std::chrono::duration_cast<std::chrono::milliseconds>(finish -
                                                                    start)
                  .count()
              << "ms");

        ready = 1;

        if (time(0) - lastRequest > 30) {
            DEBUG("sleeping");
            sleep(1);
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
        std::stringbuf buffer;
        std::ostream os(&buffer);

        os << "<html>";
        os << "<meta http-equiv=\"refresh\" content=\"1\" />";
        os << "<body>";
        for ( int i=0; i<CameraOutputInstance.size(); i++) {
            os << "<img width=600 src='/pic.png?num=" << i << "'/>";
        }
        os << "</body></html>";

        return std::shared_ptr<http_response>(
            new string_response(buffer.str(), 200, "text/html"));
    }
};

class picture_resource : public http_resource {
   public:
    const std::shared_ptr<http_response> render(const http_request& r) {
        lastRequest = time(0);

        int camNumera = getCameraNumber(r);
        if (!ready || camNumera >= CameraOutputInstance.size()) {
            return std::shared_ptr<http_response>(
                new string_response("not ready\n"));
        }
        std::shared_ptr<CameraOutput> mine(CameraOutputInstance[camNumera]);
        return std::shared_ptr<http_response>(
            new string_response(mine->ppmdata, 200, "image/ppm"));
    }
};

class picture_resource_png : public http_resource {
   public:
    const std::shared_ptr<http_response> render(const http_request& r) {
        lastRequest = time(0);

        int camNumera = getCameraNumber(r);

        if (!ready || camNumera >= CameraOutputInstance.size()) {
            return std::shared_ptr<http_response>(
                new string_response("not ready\n"));
        }

        std::shared_ptr<CameraOutput> mine(CameraOutputInstance[camNumera]);
        const char* raw_data = mine->ppmdata.c_str();
        int len = mine->width * mine->height * 3;
        raw_data = raw_data + (mine->ppmdata.size() - len);

        int pngLen;
        auto out = stbi_write_png_to_mem((const unsigned char*)raw_data,
                                         mine->width * 3, mine->width,
                                         mine->height, 3, &pngLen);
        std::string s((char*)out, pngLen);
        STBIW_FREE(out);

        return std::shared_ptr<http_response>(
            new string_response(s, 200, "image/png"));
    }
};

const int maxJpegSize = 512 * 1024;
struct jpeg_out {
    char buf[maxJpegSize];
    int size;
};

void my_jpg_write(void* context, void* data, int size) {
    jpeg_out* out = (jpeg_out*)context;
    if (size + out->size > maxJpegSize) {
        std::cerr << "size too big" << std::endl;
        return;
    }
    memcpy(out->buf + out->size, data, size);
    out->size += size;
}

class picture_resource_jpg : public http_resource {
   public:
    const std::shared_ptr<http_response> render(const http_request& r) {
        lastRequest = time(0);

        int camNumera = getCameraNumber(r);
        if (!ready || camNumera >= CameraOutputInstance.size()) {
            return std::shared_ptr<http_response>(
                new string_response("not ready\n"));
        }
        std::shared_ptr<CameraOutput> mine(CameraOutputInstance[camNumera]);
        const char* raw_data = mine->ppmdata.c_str();
        int len = mine->width * mine->height * 3;
        raw_data = raw_data + (mine->ppmdata.size() - len);

        jpeg_out out;
        out.size = 0;
        auto rv = stbi_write_jpg_to_func(my_jpg_write, &out, mine->width,
                                         mine->height, 3, raw_data, 20);
        std::string s(out.buf, out.size);
        return std::shared_ptr<http_response>(
            new string_response(s, 200, "image/jpg"));
    }
};

class depth_resource : public http_resource {
   public:
    const std::shared_ptr<http_response> render(const http_request& r) {
        lastRequest = time(0);

        if (!ready) {
            return std::shared_ptr<http_response>(
                new string_response("not ready\n"));
        }
        int camNumera = getCameraNumber(r);

        std::shared_ptr<CameraOutput> mine(CameraOutputInstance[camNumera]);

        return std::shared_ptr<http_response>(
            new string_response(mine->depth, 200, "binary"));
    }
};

class combined_resource : public http_resource {
   public:
    const std::shared_ptr<http_response> render(const http_request& r) {
        lastRequest = time(0);

        if (!ready) {
            return std::shared_ptr<http_response>(
                new string_response("not ready\n"));
        }
        int camNumera = getCameraNumber(r);

        std::shared_ptr<CameraOutput> mine(CameraOutputInstance[camNumera]);

        std::string both = mine->depth + mine->ppmdata;

        return std::shared_ptr<http_response>(
            new string_response(both, 200, "binary"));
    }
};

int main(int argc, char** argv) {
    int port = 8181;

    std::thread t(cameraThread);

    webserver ws = create_webserver(port);

    hello_world_resource hwr;
    ws.register_resource("/", &hwr);

    picture_resource pr;
    ws.register_resource("/pic.ppm", &pr);

    picture_resource_png pr2;
    ws.register_resource("/pic.png", &pr2);

    picture_resource_jpg pr3;
    ws.register_resource("/pic.jpg", &pr3);

    depth_resource dr;
    ws.register_resource("/depth.dat", &dr);

    combined_resource cr;
    ws.register_resource("/both", &cr);

    std::cout << "Starting to listen on port: " << port << std::endl;

    ws.start(true);

    return 0;
}

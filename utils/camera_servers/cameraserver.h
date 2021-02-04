// cameraserver.h

#pragma once

#include <httpserver.hpp>
#include <string>
#include <vector>

class CameraOutput {
   public:
    CameraOutput() {}

    void add_depth(int bytesPerPixel, float units, int width, int height,
                   const char* data);

    int width;
    int height;
    std::string ppmdata;
    std::string depth;
};

class CameraState {
   private:
    CameraState() : ready(0), lastRequest(0) {}

   public:
    static CameraState* get();

    std::vector<std::shared_ptr<CameraOutput>> cameras;
    bool ready;
    volatile time_t lastRequest;
};

class camera_resource : public httpserver::http_resource {
   public:
    camera_resource(CameraState* cam) : _cams(cam) {}

    int getCameraNumber(const httpserver::http_request& r) {
        auto s = r.get_arg("num");
        if (s == "") {
            return 0;
        }
        return std::stoi(s);
    }

    const std::shared_ptr<httpserver::http_response> render(
        const httpserver::http_request& r) {
        _cams->lastRequest = time(0);

        if (!_cams->ready) {
            return std::shared_ptr<httpserver::http_response>(
                new httpserver::string_response("not ready\n"));
        }

        int camNumera = getCameraNumber(r);
        if (camNumera >= _cams->cameras.size()) {
            return std::shared_ptr<httpserver::http_response>(
                new httpserver::string_response("invalid camera\n"));
        }

        std::shared_ptr<CameraOutput> mine(_cams->cameras[camNumera]);
        return myRender(mine.get());
    }

    virtual const std::shared_ptr<httpserver::http_response> myRender(
        CameraOutput* co) = 0;

   private:
    CameraState* _cams;
};

void installWebHandlers(httpserver::webserver* ws);

std::string my_write_ppm(const char* pixels, int x, int y, int bytes_per_pixel);

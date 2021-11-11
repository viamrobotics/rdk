#include <tuple>
#include <mutex>
#include <thread>
#include <queue>
#include <array>
#include <vector>
#include <atomic>
#include <iostream>
#include <sstream>
#include <functional>
#include <condition_variable>

#include <assert.h>
#include <string.h>
#include <unistd.h>
#include <signal.h>

#include <CubeEye/CubeEyeSink.h>
#include <CubeEye/CubeEyeCamera.h>
#include <CubeEye/CubeEyeBasicFrame.h>

#include "cameraserver.h"
#define DEBUG(x)
using namespace std;
using namespace meere;



std::atomic<bool> TOFdone{false};
bool TOFerror = false;

 class MyListener : public meere::sensor::sink
 , public meere::sensor::prepared_listener
{
    
public:

    virtual std::string name() const {
        return std::string("CubeEyeServer");
    }

    virtual void onCubeEyeCameraState(const meere::sensor::ptr_source source, meere::sensor::State state) {
        std::cout << "Camera State = " << state << std::endl;
        if (meere::sensor::State::Running == state) {
            std::cout << " Running" << std::endl;
            mReadFrameThreadStart = true;
            mReadFrameThread = std::thread(MyListener::ReadFrameProc, this);
        }
        else if (meere::sensor::State::Released == state) {
            std::cout << " Released" << std::endl;
        }
        else if (meere::sensor::State::Prepared == state) {
            std::cout << " Prepared" << std::endl;
        }
        else if (meere::sensor::State::Stopped == state) {
            std::cout << " Stopped" << std::endl;
            mReadFrameThreadStart = false;
            if (mReadFrameThread.joinable()) {
                mReadFrameThread.join();
            }
        }
    }

    virtual void onCubeEyeCameraError(const meere::sensor::ptr_source source, meere::sensor::Error error) {
        // CubeEye.h has list of errors
        if(!TOFerror){
            std::cerr << "Error with the camera device, error string : " << error << endl;
            TOFerror = true; //flag to try turning it off and on again
        }
    }

    virtual void onCubeEyeFrameList(const meere::sensor::ptr_source source , const meere::sensor::sptr_frame_list& frames) {
        if (mReadFrameThreadStart) {
            static constexpr size_t _MAX_FRAMELIST_SIZE = 4;
            if (_MAX_FRAMELIST_SIZE > mFrameListQueue.size()) {
                auto _copied_frame_list = meere::sensor::copy_frame_list(frames);
                if (_copied_frame_list) {
                    mFrameListQueue.push(std::move(_copied_frame_list));
                }
            }
        }
    }

public:
    virtual void onCubeEyeCameraPrepared(const meere::sensor::ptr_camera camera) {
        std::cout << __FUNCTION__ << ":" << __LINE__ << " source(" << camera->source()->uri().c_str() << ")" << std::endl; 
    }
/////////////// this guy is where we get camera data /////////////////////////

public:
    static void ReadFrameProc(MyListener* thiz) {
        
        while ((thiz->mReadFrameThreadStart)) {
            if (thiz->mFrameListQueue.empty()) {
                std::this_thread::sleep_for(std::chrono::milliseconds(100));
                continue;
            }
            //initialize output and grab frames
            std::shared_ptr<CameraOutput> output(new CameraOutput());
            auto _frames = std::move(thiz->mFrameListQueue.front());
            thiz->mFrameListQueue.pop();

            if (_frames) {
                static int _frame_cnt = 0;
                if (30 > ++_frame_cnt) {
                    continue;
                }
                _frame_cnt = 0;

                for (auto it : (*_frames)) {

                    int _frame_index = 0;
                    auto _center_x = it->frameWidth() / 2;
                    auto _center_y = it->frameHeight() / 2;
                    
                    if (it->frameType() == meere::sensor::CubeEyeFrame::FrameType_Depth) {
                        // setting min/max based off exp values. didnt like the idea of constantly changing upper and lower bound
                        float max = 2200;// 0;//
                        float min = 120;// 100000;//
                        // 16bits data type
                        if (it->frameDataType() == meere::sensor::CubeEyeData::DataType_16U) {
                            // casting 16bits basic frame
                            auto _sptr_basic_frame = meere::sensor::frame_cast_basic16u(it);
                            auto _sptr_frame_data = _sptr_basic_frame->frameData();    // depth data array
                            
                            //frame data
                            output->width = _sptr_basic_frame->frameWidth();
                            output->height = _sptr_basic_frame->frameHeight();

                            //depth data
                            {
                            std::stringbuf buffer;
                            std::ostream os(&buffer);
                            os << "VERSIONX\n";
                            os << "2\n";
                            os << "1\n";
                            os << output->width << "\n";
                            os << output->height << "\n";
                            for (int y = 0 ; y < _sptr_basic_frame->frameHeight(); y++) {
                                for (int x = 0 ; x < _sptr_basic_frame->frameWidth(); x++) {
                                    _frame_index = y * _sptr_basic_frame->frameWidth() + x;
                                    // depth values appear to be in mm(based off hand measurements)
                                    short s =  (*_sptr_frame_data)[_frame_index];
                                    // // set an upper bound for useful data

                                    if (max < s) max = s;
                                    if (min > s) min = s;
                                    buffer.sputn((const char*)&s, 2);

                                }
                            }
                            output->depth = buffer.str();
                            }
                            
                            
                        }

                        //ppm data
                        {
                        std::stringbuf buffer;
                        std::ostream os(&buffer);
                        auto _sptr_basic_frame = meere::sensor::frame_cast_basic16u(it);
                        auto _sptr_frame_data = _sptr_basic_frame->frameData();    // depth data array
                        os << "P6\n" << output->width << " " << output->height << "\n255\n";
                        
                        float span = max - min;
                        for (int y = 0; y < output->height; y++) {
                            for (int x = 0; x < output->width; x++) {
                                _frame_index = y * _sptr_basic_frame->frameWidth() + x;

                                auto val =  (*_sptr_frame_data)[_frame_index];

                                char clr = 0;
                                //scale depth measurement between ~0-255(ish)(assuming)
                                if (val > 0) {
                                    auto ratio = (val - min) / span;
                                    clr = (char)(60 + (int)(ratio * 192));//need to calibrate?
                                    // force bounds(just in case)
                                    if (clr > 255)
                                    clr = 255;
                                    if (clr < 0)
                                    clr = 0;
                                }
                                os << (char)clr;
                                os << (char)clr;
                                os << (char)clr;
                                
                                if (_center_x == x && _center_y == y) {
                                    DEBUG("depth("_center_x","_center_y") data : "(*_sptr_frame_data)[_frame_index]" \n");
                                }
                            }
                        }
                        output->ppmdata = buffer.str();

                        }
                        
                    }

                }
                CameraState::get()->cameras[0] = output;
                CameraState::get()->ready = 1;
            }
        }
        
    }

void getLensInfo(meere::sensor::sptr_camera camera, meere::sensor::result _rt){
    // get lens parameters
        {
            auto _lenses = camera->lenses();
            std::cout << "count of Lenses : " << _lenses << std::endl;
            for (size_t i = 0; i < _lenses; i++) {
                std::cout << "Lens index : " << i << std::endl;

                auto _fov = camera->fov(i);
                std::cout << "    FoV : " << std::get<0>(_fov) << "(H) x " << std::get<1>(_fov) << "(V)" << std::endl;

                meere::sensor::IntrinsicParameters parameters;
                if (meere::sensor::success == (_rt = camera->intrinsicParameters(parameters, i))) {
                    std::cout << "    IntrinsicParameters :" << std::endl;
                    std::cout << "        ForcalLength(fx) = " << parameters.forcal.fx << std::endl;
                    std::cout << "        ForcalLength(fy) = " << parameters.forcal.fy << std::endl;
                    std::cout << "        PrincipalPoint(cx) = " << parameters.principal.cx << std::endl;
                    std::cout << "        PrincipalPoint(cy) = " << parameters.principal.cy << std::endl;
                }

                meere::sensor::DistortionCoefficients coefficients;
                if (meere::sensor::success == (_rt = camera->distortionCoefficients(coefficients, i))) {
                    std::cout << "    DistortionCoefficients :" << std::endl;
                    std::cout << "        RadialCoefficient(K1) = " << coefficients.radial.k1 << std::endl;
                    std::cout << "        RadialCoefficient(K2) = " << coefficients.radial.k2 << std::endl;
                    std::cout << "        RadialCoefficient(K3) = " << coefficients.radial.k3 << std::endl;
                    std::cout << "        TangentialCoefficient(P1) = " << coefficients.tangential.p1 << std::endl;
                    std::cout << "        TangentialCoefficient(P2) = " << coefficients.tangential.p2 << std::endl;
                    std::cout << "        skewCoefficient = " << coefficients.skewCoefficient << std::endl;
                }
            }
        }


}


public:
    MyListener() = default;
    virtual ~MyListener() = default;

protected:
    bool mReadFrameThreadStart;
    std::thread mReadFrameThread;
    std::queue<meere::sensor::sptr_frame_list> mFrameListQueue;
};


      

void signal_callback_handler(int signum) {
    // On kill signal turn flag to allow main to finish executing
    // Which closes the webserver and camera
    TOFdone = true;
}


int main(int argc, char* argv[])
{
    int port = 8181;
    httpserver::webserver webServerTOF = httpserver::create_webserver(port);
    installWebHandlers(&webServerTOF);

    // setup listener thread
    MyListener listener;
    meere::sensor::add_prepared_listener(&listener);

    // search ToF camera source
    int selected_source = -1;
    meere::sensor::sptr_source_list _source_list = meere::sensor::search_camera_source();

    if (nullptr != _source_list) {
        int i = 0;
        for (auto it : (*_source_list)) {

            std::cout << i++ << ") source name : " << it->name() << \
                    ", serialNumber : " << it->serialNumber() << \
                    ", uri : " << it->uri() << std::endl;
        }
    }
    if (nullptr != _source_list && 0 < _source_list->size()) {
        if (1 < _source_list->size()) {
            std::cout << "Please enter the desired source number." << std::endl;
            scanf("%d", &selected_source);
            getchar();
        }
        else {
            selected_source = 0;
        }
    }
    else {
        std::cerr << "no search device!" << std::endl;
        return -1;
    }

    if (0 > selected_source) {
        std::cerr << "invalid selected source number!" << std::endl;
        return -1;
    }

    
    CameraState::get()->cameras.push_back(0);

    // create ToF camera
    meere::sensor::result _rt;
    meere::sensor::sptr_camera _camera = meere::sensor::create_camera(_source_list->at(selected_source));
    if (nullptr != _camera) {
        _camera->addSink(&listener);

        _rt = _camera->prepare();
        assert(meere::sensor::success == _rt);
        if (meere::sensor::success != _rt) {
            std::cerr << "_camera->prepare() failed." << std::endl;
            meere::sensor::destroy_camera(_camera);
            return -1;
        }

        // set wanted frame type : depth
        int wantedFrame = meere::sensor::CubeEyeFrame::FrameType_Depth;

        _rt = _camera->run(wantedFrame);
        assert(meere::sensor::success == _rt);
        if (meere::sensor::success != _rt) {
            std::cerr << "_camera->run() failed." << std::endl;
            meere::sensor::destroy_camera(_camera);
            return -1;
        }
        // change camera framerate to 30 fps
        std::string key("");
		meere::sensor::sptr_property _prop;
		meere::sensor::result_property _rt_prop;
        key = "framerate";
        _prop = meere::sensor::make_property_8u(key, 30);
        _rt = _camera->setProperty(_prop);

        if (!TOFdone.is_lock_free()) return 10; // make sure we can actually use signal
        signal(SIGTERM, signal_callback_handler);// if we kill, dont break the camera by safely turning stuff off
        signal(SIGINT, signal_callback_handler);// in this case exiting the while loop below
        signal(SIGQUIT, signal_callback_handler);

        webServerTOF.start(false);//start the webserver without using blocking
        while(!TOFdone){//keep going until we stop
            std::this_thread::sleep_for(std::chrono::milliseconds(200));
            // if an error in the camera occurs(sometimes timeout error on startup)
            // restart the camera
            if(TOFerror){
                std::cerr << "Restarting Camera to Fix " << endl;
                _camera->stop();

                _rt = _camera->run(wantedFrame);
                TOFerror = false;
            } 
        }
        // turn stuff off
        webServerTOF.stop();
        meere::sensor::destroy_camera(_camera);
        // _camera.reset();
    }
    return 0;
}

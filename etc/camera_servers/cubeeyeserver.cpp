/*
 * ReadDepthFrame.cpp
 *
 *  Created on: 2018. 10. 15.
 *      Author: erato
 */

#include <tuple>
#include <mutex>
#include <thread>
#include <queue>
#include <array>
#include <vector>
#include <atomic>
#include <iostream>
#include <functional>
#include <condition_variable>

#include <assert.h>
#include <string.h>
#include <unistd.h>

#include "CubeEyeSink.h"
#include "CubeEyeCamera.h"
#include "CubeEyeBasicFrame.h"
#include "cameraserver.h"
using namespace std;
using namespace meere;

 class MyListener : public meere::sensor::sink
 , public meere::sensor::prepared_listener
{
	
public:

	virtual std::string name() const {
		return std::string("MyListener");
	}

	virtual void onCubeEyeCameraState(const meere::sensor::ptr_source source, meere::sensor::State state) {
		printf("%s:%d source(%s) state = %d\n", __FUNCTION__, __LINE__, source->uri().c_str(), state);
		
		if (meere::sensor::State::Running == state) {
			mReadFrameThreadStart = true;
			mReadFrameThread = std::thread(MyListener::ReadFrameProc, this);
		}
		else if (meere::sensor::State::Stopped == state) {
			mReadFrameThreadStart = false;
			if (mReadFrameThread.joinable()) {
				mReadFrameThread.join();
			}
		}
	}

	virtual void onCubeEyeCameraError(const meere::sensor::ptr_source source, meere::sensor::Error error) {
		printf("%s:%d source(%s) error = %d\n", __FUNCTION__, __LINE__, source->uri().c_str(), error);
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
		printf("%s:%d source(%s)\n", __FUNCTION__, __LINE__, camera->source()->uri().c_str());
	}

public:
	static void ReadFrameProc(MyListener* thiz) {
		while (thiz->mReadFrameThreadStart) {
			if (thiz->mFrameListQueue.empty()) {
				std::this_thread::sleep_for(std::chrono::milliseconds(100));
				continue;
			}

			auto _frames = std::move(thiz->mFrameListQueue.front());
			thiz->mFrameListQueue.pop();

			if (_frames) {
				static int _frame_cnt = 0;
				if (30 > ++_frame_cnt) {
					continue;
				}
				_frame_cnt = 0;

				for (auto it : (*_frames)) {
					printf("frame : %d, "
							"frameWidth = %d "
							"frameHeight = %d "
							"frameDataType = %d "
							"timestamp = %lu \n",
							it->frameType(),
							it->frameWidth(),
							it->frameHeight(),
							it->frameDataType(),
							it->timestamp());

					int _frame_index = 0;
					auto _center_x = it->frameWidth() / 2;
					auto _center_y = it->frameHeight() / 2;

					// Depth frame
					if (it->frameType() == meere::sensor::CubeEyeFrame::FrameType_Depth) {
						// 16bits data type
						if (it->frameDataType() == meere::sensor::CubeEyeData::DataType_16U) {
							// casting 16bits basic frame 
							auto _sptr_basic_frame = meere::sensor::frame_cast_basic16u(it);
							auto _sptr_frame_data = _sptr_basic_frame->frameData();	// depth data array

							for (int y = 0 ; y < _sptr_basic_frame->frameHeight(); y++) {
								for (int x = 0 ; x < _sptr_basic_frame->frameWidth(); x++) {
									_frame_index = y * _sptr_basic_frame->frameWidth() + x;
									if (_center_x == x && _center_y == y) {
										printf("depth(%d,%d) data : %d\n", _center_x, _center_y, (*_sptr_frame_data)[_frame_index]);
										
									}
								}
							}
						}
					}
					// Amplitude frame
					else if (it->frameType() == meere::sensor::CubeEyeFrame::FrameType_Amplitude) {
						// 16bits data type
						if (it->frameDataType() == meere::sensor::CubeEyeData::DataType_16U) {
							// casting 16bits basic frame 
							auto _sptr_basic_frame = meere::sensor::frame_cast_basic16u(it);
							auto _sptr_frame_data = _sptr_basic_frame->frameData();	// amplitude data array

							for (int y = 0 ; y < _sptr_basic_frame->frameHeight(); y++) {
								for (int x = 0 ; x < _sptr_basic_frame->frameWidth(); x++) {
									_frame_index = y * _sptr_basic_frame->frameWidth() + x;
									if (_center_x == x && _center_y == y) {
										printf("amplitude(%d,%d) data : %d\n", _center_x, _center_y, (*_sptr_frame_data)[_frame_index]);
									}
								}
							}
						}
					}
				}

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
				std::cout << "	FoV : " << std::get<0>(_fov) << "(H) x " << std::get<1>(_fov) << "(V)" << std::endl;

				meere::sensor::IntrinsicParameters parameters;
				if (meere::sensor::success == (_rt = camera->intrinsicParameters(parameters, i))) {
					std::cout << "	IntrinsicParameters :" << std::endl;
					std::cout << "		ForcalLength(fx) = " << parameters.forcal.fx << std::endl;
					std::cout << "		ForcalLength(fy) = " << parameters.forcal.fy << std::endl;
					std::cout << "		PrincipalPoint(cx) = " << parameters.principal.cx << std::endl;
					std::cout << "		PrincipalPoint(cy) = " << parameters.principal.cy << std::endl;
				}

				meere::sensor::DistortionCoefficients coefficients;
				if (meere::sensor::success == (_rt = camera->distortionCoefficients(coefficients, i))) {
					std::cout << "	DistortionCoefficients :" << std::endl;
					std::cout << "		RadialCoefficient(K1) = " << coefficients.radial.k1 << std::endl;
					std::cout << "		RadialCoefficient(K2) = " << coefficients.radial.k2 << std::endl;
					std::cout << "		RadialCoefficient(K3) = " << coefficients.radial.k3 << std::endl;
					std::cout << "		TangentialCoefficient(P1) = " << coefficients.tangential.p1 << std::endl;
					std::cout << "		TangentialCoefficient(P2) = " << coefficients.tangential.p2 << std::endl;
					std::cout << "		skewCoefficient = " << coefficients.skewCoefficient << std::endl;
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



int main(int argc, char* argv[])
{
	// not sure how webserver works/could not get the install to work?
	 int port = 8181;

      httpserver::webserver ws = httpserver::create_webserver(port);
      installWebHandlers(&ws);

	//setup listener threadl
	MyListener listener;
	meere::sensor::add_prepared_listener(&listener);

	// select camera to use
	// search ToF camera source
	int _selected_source = -1;
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
			scanf("%d", &_selected_source);
			getchar();
		}
		else {
			_selected_source = 0;
		}
	}
	else {
		std::cout << "no search device!" << std::endl;
		return -1;
	}

	if (0 > _selected_source) {
		std::cout << "invalid selected source number!" << std::endl;
		return -1;
	}
	

	// create ToF camera
	meere::sensor::result _rt;
	meere::sensor::sptr_camera _camera = meere::sensor::create_camera(_source_list->at(_selected_source));
	if (nullptr != _camera) {
		_camera->addSink(&listener);

		//listener.getLensInfo( _camera ,_rt);

		_rt = _camera->prepare();
		assert(meere::sensor::success == _rt);
		if (meere::sensor::success != _rt) {
			std::cout << "_camera->prepare() failed." << std::endl;
			meere::sensor::destroy_camera(_camera);
			return -1;
		}

		// set wanted frame type : depth & amplitude
		int _wantedFrame = meere::sensor::CubeEyeFrame::FrameType_Depth;// | meere::sensor::CubeEyeFrame::FrameType_Amplitude;

		_rt = _camera->run(_wantedFrame);
		assert(meere::sensor::success == _rt);
		if (meere::sensor::success != _rt) {
			std::cout << "_camera->run() failed." << std::endl;
			meere::sensor::destroy_camera(_camera);
			return -1;
		}


		// std::this_thread::sleep_for(std::chrono::milliseconds(2000));
		// char exitKey = 0; //press any key to stop
		// while(exitKey == 0){
		// 	exitKey = getchar();
		// }

		// _camera->stop();
		// _camera->release();
		// meere::sensor::destroy_camera(_camera);
		// _camera.reset();
		//}
	}
	return 0;
}

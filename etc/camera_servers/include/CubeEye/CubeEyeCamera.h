/*
 * CubeEyeCamera.h
 *
 *  Created on: 2020. 1. 7.
 *      Author: erato
 */

#ifndef CUBEEYECAMERA_H_
#define CUBEEYECAMERA_H_

#include "CubeEyeSource.h"
#include "CubeEyeFrame.h"
#include "CubeEyeProperties.h"


BEGIN_NAMESPACE

class CubeEyeSink;
class CubeEyeCamera;
using sink = CubeEyeSink;
using ptr_sink = CubeEyeSink*;
using ptr_camera = CubeEyeCamera*;
using sptr_camera = std::shared_ptr<CubeEyeCamera>;
using FoV = std::tuple<flt64, flt64>;

class _decl_dll CubeEyeCamera
{
public:
	enum State {
		Released,
		Prepared,
		Stopped,
		Running
	};

	enum Error {
		Unknown,
		IO,	
		AccessDenied,
		NoSuchDevice,
		Busy,		
		Timeout,	
		Overflow,	
		Interrupted,
		Internal,
		FrameDropped,
		IlluminationLock
	};

public:
	struct IntrinsicParameters {
		struct ForcalLength {
			flt32 fx;
			flt32 fy;
		} forcal;

		struct PrincipalPoint {
			flt32 cx;
			flt32 cy;
		} principal;
	};

	struct DistortionCoefficients {
		struct RadialCoefficient {
			flt64 k1;
			flt64 k2;
			flt64 k3;
		} radial;

		struct TangentialCoefficient {
			flt64 p1;
			flt64 p2;
		} tangential;

		flt64 skewCoefficient;
	};

	struct ExtrinsicParameters {
		struct RotationParameters {
			flt32 r1[3];
			flt32 r2[3];
			flt32 r3[3];
		} rotation;

		struct TranslationParameters {
			flt32 tx;
			flt32 ty;
			flt32 tz;
		} translation;
	};

public:
	class PreparedListener
	{
	public:
		virtual void _decl_call onCubeEyeCameraPrepared(const ptr_camera camera) = 0;

	protected:
		PreparedListener() = default;
		virtual ~PreparedListener() = default;
	};

public:
	virtual State _decl_call state() const = 0;
	virtual ptr_source _decl_call source() const = 0;

public:
	virtual size_t _decl_call lenses() const = 0;
	virtual FoV _decl_call fov(int8u idx = 0) = 0;
	virtual result _decl_call intrinsicParameters(IntrinsicParameters& intrinsic, int8u idx = 0) = 0;
	virtual result _decl_call distortionCoefficients(DistortionCoefficients& distortion, int8u idx = 0) = 0;
	virtual result _decl_call extrinsicParameters(ExtrinsicParameters& extrinsic, int8u idx0 = 0, int8u idx1 = 1) = 0;

public:
	virtual result _decl_call prepare() = 0;
	virtual result _decl_call prepareAsync() = 0;
	virtual result _decl_call run(int32s wantedFrame) = 0;
	virtual result _decl_call stop() = 0;
	virtual result _decl_call release() = 0;

public:
	virtual result _decl_call setProperty(const sptr_property& property) = 0;
	virtual result _decl_call setProperties(const sptr_properties& properties) = 0;
	virtual result_property _decl_call getProperty(const std::string& key) = 0;
	virtual result_properties _decl_call getProperties(const std::string& name) = 0;

public:
	virtual result _decl_call addSink(ptr_sink sink) = 0;
	virtual result _decl_call removeSink(ptr_sink sink) = 0;
	virtual result _decl_call removeSink(const std::string& sinkName) = 0;
	virtual result _decl_call removeAllSinks() = 0;
	virtual bool _decl_call containsSink(const std::string& sinkName) = 0;

protected:
	CubeEyeCamera() = default;
	virtual ~CubeEyeCamera() = default;
};


using State = CubeEyeCamera::State;
using Error = CubeEyeCamera::Error;
using IntrinsicParameters = CubeEyeCamera::IntrinsicParameters;
using DistortionCoefficients = CubeEyeCamera::DistortionCoefficients;
using ExtrinsicParameters = CubeEyeCamera::ExtrinsicParameters;
using prepared_listener = CubeEyeCamera::PreparedListener;
using ptr_prepared_listener = CubeEyeCamera::PreparedListener*;

_decl_dll std::string _decl_call last_released_date();
_decl_dll std::string _decl_call last_released_version();
_decl_dll sptr_camera _decl_call create_camera(const sptr_source& source);
_decl_dll sptr_camera _decl_call find_camera(const sptr_source source);
_decl_dll result _decl_call destroy_camera(const sptr_camera& camera);
_decl_dll result _decl_call set_property(const sptr_property& property);
_decl_dll result_property _decl_call get_property(const std::string& key);
_decl_dll result _decl_call add_prepared_listener(ptr_prepared_listener listener);
_decl_dll result _decl_call remove_prepared_listener(ptr_prepared_listener listener);

END_NAMESPACE

#endif /* CUBEEYECAMERA_H_ */

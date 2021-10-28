/*
 * CubeEyeSink.h
 *
 *  Created on: 2020. 1. 6.
 *      Author: erato
 */

#ifndef CUBEEYESINK_H_
#define CUBEEYESINK_H_

#include "CubeEyeCamera.h"
#include "CubeEyeFrameList.h"

BEGIN_NAMESPACE

class _decl_dll CubeEyeSink
{
public:
	virtual std::string _decl_call name() const = 0;
	virtual void _decl_call onCubeEyeCameraState(const ptr_source source, State state) = 0;
	virtual void _decl_call onCubeEyeCameraError(const ptr_source source, Error error) = 0;
	virtual void _decl_call onCubeEyeFrameList(const ptr_source source , const sptr_frame_list& frames) = 0;

protected:
	CubeEyeSink() = default;
	virtual ~CubeEyeSink() = default;
};

END_NAMESPACE

#endif /* CUBEEYESINK_H_ */

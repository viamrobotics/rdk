/*
 * CubeEyeBasicFrame.h
 *
 *  Created on: 2020. 1. 6.
 *      Author: erato
 */

#ifndef CUBEEYEBASICFRAME_H_
#define CUBEEYEBASICFRAME_H_

#include "CubeEyeFrame.h"

BEGIN_NAMESPACE

template <typename T>
class _decl_dll CubeEyeBasicFrame : public CubeEyeFrame
{
public:
	virtual CubeEyeList<T>* _decl_call frameData() const = 0;

protected:
	CubeEyeBasicFrame() = default;
	virtual ~CubeEyeBasicFrame() = default;
};


using sptr_frame_basic8u = std::shared_ptr<CubeEyeBasicFrame<int8u>>;
using sptr_frame_basic16u = std::shared_ptr<CubeEyeBasicFrame<int16u>>;
using sptr_frame_basic32f = std::shared_ptr<CubeEyeBasicFrame<flt32>>;
using sptr_frame_basic64f = std::shared_ptr<CubeEyeBasicFrame<flt64>>;

_decl_dll sptr_frame_basic8u _decl_call frame_cast_basic8u(const sptr_frame& frame);
_decl_dll sptr_frame_basic16u _decl_call frame_cast_basic16u(const sptr_frame& frame);
_decl_dll sptr_frame_basic32f _decl_call frame_cast_basic32f(const sptr_frame& frame);
_decl_dll sptr_frame_basic64f _decl_call frame_cast_basic64f(const sptr_frame& frame);

END_NAMESPACE

#endif /* CUBEEYEBASICFRAME_H_ */

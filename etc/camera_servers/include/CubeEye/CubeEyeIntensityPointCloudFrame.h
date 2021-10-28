/*
 * CubeEyeIntensityPointCloudFrame.h
 *
 *  Created on: 2020. 1. 6.
 *      Author: erato
 */

#ifndef CUBEEYEINTENSITYPOINTCLOUDFRAME_H_
#define CUBEEYEINTENSITYPOINTCLOUDFRAME_H_

#include "CubeEyePointCloudFrame.h"

BEGIN_NAMESPACE

template <typename T>
class _decl_dll CubeEyeIntensityPointCloudFrame : public CubeEyePointCloudFrame<T>
{
public:
	virtual CubeEyeList<T>* _decl_call frameDataI() const = 0;

protected:
	CubeEyeIntensityPointCloudFrame() = default;
	virtual ~CubeEyeIntensityPointCloudFrame() = default;
};


using sptr_frame_ipcl16u = std::shared_ptr<CubeEyeIntensityPointCloudFrame<int16u>>;
using sptr_frame_ipcl32f = std::shared_ptr<CubeEyeIntensityPointCloudFrame<flt32>>;
using sptr_frame_ipcl64f = std::shared_ptr<CubeEyeIntensityPointCloudFrame<flt64>>;

_decl_dll sptr_frame_ipcl16u _decl_call frame_cast_ipcl16u(const sptr_frame& frame);
_decl_dll sptr_frame_ipcl32f _decl_call frame_cast_ipcl32f(const sptr_frame& frame);
_decl_dll sptr_frame_ipcl64f _decl_call frame_cast_ipcl64f(const sptr_frame& frame);

END_NAMESPACE

#endif /* CUBEEYEINTENSITYPOINTCLOUDFRAME_H_ */

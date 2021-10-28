/*
 * CubeEyeFrameList.h
 *
 *  Created on: 2020. 1. 8.
 *      Author: erato
 */

#ifndef CUBEEYEFRAMELIST_H_
#define CUBEEYEFRAMELIST_H_

#include "CubeEyeFrame.h"

BEGIN_NAMESPACE

class _decl_dll CubeEyeFrameList : public CubeEyeList<sptr_frame>
{
protected:
	CubeEyeFrameList() = default;
	virtual ~CubeEyeFrameList() = default;
};


using sptr_frame_list = std::shared_ptr<CubeEyeFrameList>;

_decl_dll sptr_frame _decl_call find_frame(const sptr_frame_list& frameList, FrameType frameType);
_decl_dll sptr_frame _decl_call copy_frame(const sptr_frame& frame);
_decl_dll sptr_frame_list _decl_call copy_frame_list(const sptr_frame_list& frames);

END_NAMESPACE

#endif /* CUBEEYEFRAMELIST_H_ */

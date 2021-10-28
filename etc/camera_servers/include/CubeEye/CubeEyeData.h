/*
 * CubeEyeData.h
 *
 *  Created on: 2019. 12. 26.
 *      Author: erato
 */

#ifndef CUBEEYEDATA_H_
#define CUBEEYEDATA_H_

#include "CubeEye.h"

BEGIN_NAMESPACE

class _decl_dll CubeEyeData
{
public:
	enum DataType {
		DataType_Boolean,
		DataType_8S,
		DataType_8U,
		DataType_16S,
		DataType_16U,
		DataType_32S,
		DataType_32U,
		DataType_32F,
		DataType_64S,
		DataType_64U,
		DataType_64F,
		DataType_String
	};

public:
	virtual bool _decl_call isArray() const = 0;
	virtual bool _decl_call isNumeric() const = 0;
	virtual bool _decl_call isIntegral() const = 0;
	virtual DataType _decl_call dataType() const = 0;

protected:
	CubeEyeData() = default;
	virtual ~CubeEyeData() = default;
};


using DataType = CubeEyeData::DataType;

END_NAMESPACE

#endif /* CUBEEYEDATA_H_ */

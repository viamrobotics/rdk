/*
 * CubeEyeProperty.h
 *
 *  Created on: 2019. 12. 30.
 *      Author: erato
 */

#ifndef CUBEEYEPROPERTY_H_
#define CUBEEYEPROPERTY_H_

#include "CubeEyeData.h"

BEGIN_NAMESPACE

class _decl_dll CubeEyeProperty : public CubeEyeData
{
public:
	virtual std::string _decl_call key() const = 0;

public:
	virtual auto _decl_call asBoolean(const bool& defaultValue = false) const -> bool = 0;
	virtual auto _decl_call asInt8s(const int8s& defaultValue = 0) const -> int8s = 0;
	virtual auto _decl_call asInt8u(const int8u& defaultValue = 0) const -> int8u = 0;
	virtual auto _decl_call asInt16s(const int16s& defaultValue = 0) const -> int16s = 0;
	virtual auto _decl_call asInt16u(const int16u& defaultValue = 0) const -> int16u = 0;
	virtual auto _decl_call asInt32s(const int32s& defaultValue = 0) const -> int32s = 0;
	virtual auto _decl_call asInt32u(const int32u& defaultValue = 0) const -> int32u = 0;
	virtual auto _decl_call asInt64s(const int64s& defaultValue = 0) const -> int64s = 0;
	virtual auto _decl_call asInt64u(const int64u& defaultValue = 0) const -> int64u = 0;
	virtual auto _decl_call asFlt32(const flt32& defaultValue = 0) const -> flt32 = 0;
	virtual auto _decl_call asFlt64(const flt64& defaultValue = 0) const -> flt64 = 0;
	virtual auto _decl_call asString(const std::string& defaultValue = "") const -> std::string = 0;

public:
	virtual CubeEyeProperty& _decl_call operator=(const CubeEyeProperty& other) = 0;

protected:
	CubeEyeProperty() = default;
	virtual ~CubeEyeProperty() = default;
};


using sptr_property = std::shared_ptr<CubeEyeProperty>;
using result_property = std::tuple<result, sptr_property>;

_decl_dll sptr_property _decl_call make_property_bool(const std::string& key, const bool& data);
_decl_dll sptr_property _decl_call make_property_8s(const std::string& key, const int8s& data);
_decl_dll sptr_property _decl_call make_property_8u(const std::string& key, const int8u& data);
_decl_dll sptr_property _decl_call make_property_16s(const std::string& key, const int16s& data);
_decl_dll sptr_property _decl_call make_property_16u(const std::string& key, const int16u& data);
_decl_dll sptr_property _decl_call make_property_32s(const std::string& key, const int32s& data);
_decl_dll sptr_property _decl_call make_property_32u(const std::string& key, const int32u& data);
_decl_dll sptr_property _decl_call make_property_32f(const std::string& key, const flt32& data);
_decl_dll sptr_property _decl_call make_property_64s(const std::string& key, const int64s& data);
_decl_dll sptr_property _decl_call make_property_64u(const std::string& key, const int64u& data);
_decl_dll sptr_property _decl_call make_property_64f(const std::string& key, const flt64& data);
_decl_dll sptr_property _decl_call make_property_string(const std::string& key, const std::string& data);

END_NAMESPACE

#endif /* CUBEEYEPROPERTY_H_ */

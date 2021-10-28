/*
 * CubeEyeProperties.h
 *
 *  Created on: 2021. 1. 12.
 *      Author: erato
 */

#ifndef CUBEEYEPROPERTIES_H_
#define CUBEEYEPROPERTIES_H_

#include "CubeEyeList.h"
#include "CubeEyeProperty.h"

BEGIN_NAMESPACE

class _decl_dll CubeEyeProperties
{
public:
	virtual std::string _decl_call name() const = 0;
	virtual bool _decl_call contains(const std::string& key) const = 0;
	virtual sptr_property _decl_call get(const std::string& key) const = 0;

public:
	virtual result _decl_call add(const sptr_property& property) = 0;
	virtual result _decl_call remove(const std::string& key) = 0;
	virtual result _decl_call remove(const sptr_property& property) = 0;
	virtual CubeEyeList<sptr_property>* _decl_call list() const = 0;

public:
	virtual CubeEyeProperties& _decl_call operator=(const CubeEyeProperties& other) = 0;

protected:
	CubeEyeProperties() = default;
	virtual ~CubeEyeProperties() = default;
};


using sptr_properties = std::shared_ptr<CubeEyeProperties>;
using result_properties = std::tuple<result, sptr_properties>;

_decl_dll sptr_properties _decl_call make_properties(const std::string& name);

END_NAMESPACE

#endif /* CUBEEYEPROPERTIES_H_ */

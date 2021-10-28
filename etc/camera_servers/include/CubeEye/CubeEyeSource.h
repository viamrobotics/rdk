/*
 * CubeEyeSource.h
 *
 *  Created on: 2019. 12. 26.
 *      Author: erato
 */

#ifndef CUBEEYESOURCE_H_
#define CUBEEYESOURCE_H_

#include "CubeEyeList.h"

BEGIN_NAMESPACE

class CubeEyeSource;
using ptr_source = CubeEyeSource*;
using sptr_source = std::shared_ptr<CubeEyeSource>;
using sptr_source_list = std::shared_ptr<CubeEyeList<sptr_source>>;

class _decl_dll CubeEyeSource
{
public:
	virtual std::string _decl_call name() const = 0;
	virtual std::string _decl_call serialNumber() const = 0;
	virtual std::string _decl_call uri() const = 0;

public:
	class Listener
	{
	public:
		virtual void _decl_call onAttachedCubeEyeSource(const ptr_source source) = 0;
		virtual void _decl_call onDetachedCubeEyeSource(const ptr_source source) = 0;

	protected:
		Listener() = default;
		virtual ~Listener() = default;
	};

public:
	CubeEyeSource() = default;
	virtual ~CubeEyeSource() = default;
};


using source_listener = CubeEyeSource::Listener;
using ptr_source_listener = CubeEyeSource::Listener*;

_decl_dll sptr_source_list _decl_call search_camera_source();
_decl_dll result _decl_call add_external_source(const std::string& uri);
_decl_dll result _decl_call remove_external_source(const std::string& uri);
_decl_dll result _decl_call add_source_listener(ptr_source_listener listener);
_decl_dll result _decl_call remove_source_listener(ptr_source_listener listener);

END_NAMESPACE

#endif /* CUBEEYESOURCE_H_ */

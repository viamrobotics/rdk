/*
 * CubeEye.h
 *
 *  Created on: 2019. 12. 26.
 *      Author: erato
 */

#ifndef CUBEEYE_H_
#define CUBEEYE_H_

#include <tuple>
#include <string>
#include <memory>
#include <iterator>

#define BEGIN_NAMESPACE		namespace meere { namespace sensor {
#define END_NAMESPACE		} /*namespace sensor*/ 	} /*namespace meere*/

#if defined(_WIN32) || defined(_WIN64)
#ifdef _BUILD_FOR_DLL_EXPORT
#define _decl_dll	__declspec(dllexport)
#else //_BUILD_FOR_DLL_EXPORT
#define _decl_dll	__declspec(dllimport)
#endif //!_BUILD_FOR_DLL_EXPORT
#define _decl_call	__stdcall
#else //!defined(_WIN32) && !defined(_WIN64)
#define _decl_dll
#define _decl_call
#endif //defined(_WIN32) || defined(_WIN64)

BEGIN_NAMESPACE

typedef int8_t		int8s;
typedef uint8_t		int8u;
typedef int16_t		int16s;
typedef uint16_t	int16u;
typedef int32_t		int32s;
typedef uint32_t	int32u;
typedef int64_t		int64s;
typedef uint64_t	int64u;
typedef float		flt32;
typedef double		flt64;

typedef enum result
{
	success = 0,
	fail,
	empty,
	overflow,
	not_found,
	not_exist,
	not_ready,
	not_supported,
	not_implemented,
	not_initialized,
	no_such_device,
	invalid_parameter,
	invalid_operation,
	invalid_data_type,
	out_of_memory,
	out_of_resource,
	out_of_range,
	already_exists,
	already_opened,
	already_running,
	already_initialized,
	using_resources,
	timeout,
	unknown
} result;

END_NAMESPACE

#endif /* CUBEEYE_H_ */

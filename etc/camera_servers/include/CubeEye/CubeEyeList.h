/*
 * CubeEyeList.h
 *
 *  Created on: 2019. 12. 26.
 *      Author: erato
 */

#ifndef CUBEEYELIST_H_
#define CUBEEYELIST_H_

#include "CubeEye.h"

BEGIN_NAMESPACE

template <typename T>
class _decl_dll CubeEyeList
{
public:
	class const_iterator
	{
	public:
		typedef typename std::iterator_traits<T*>::value_type value_type;
		typedef typename std::iterator_traits<T*>::difference_type difference_type;
		typedef const typename std::iterator_traits<T*>::value_type& reference;
		typedef const typename std::iterator_traits<T*>::value_type* pointer;
		typedef std::forward_iterator_tag iterator_category;

	public:
		explicit const_iterator(pointer ptr) : _ptr(ptr) {}
		virtual ~const_iterator() = default;

	public:
		virtual const_iterator _decl_call operator++() { ++_ptr; return (*this); }
		virtual const_iterator _decl_call operator++(int) { const_iterator _temp(*this); operator++(); return _temp; }
		virtual reference _decl_call operator*() const { return (*_ptr); }
		virtual pointer _decl_call operator->() const { return _ptr; }
		virtual bool _decl_call operator==(const const_iterator& other) const { return _ptr == other._ptr; }
		virtual bool _decl_call operator!=(const const_iterator& other) const { return _ptr != other._ptr; }

	private:
		pointer _ptr;
	};

public:
	virtual bool _decl_call empty() const = 0;
	virtual size_t _decl_call size() const = 0;
	virtual const T* _decl_call data() const = 0;

public:
	virtual const T& _decl_call back() const = 0;
	virtual const T& _decl_call front() const = 0;
	virtual const T& _decl_call at(size_t index) const = 0;
	virtual const T& _decl_call operator[](size_t index) const = 0;

public:
	virtual typename CubeEyeList<T>::const_iterator _decl_call begin() const = 0;
	virtual typename CubeEyeList<T>::const_iterator _decl_call end() const = 0;

protected:
	CubeEyeList() = default;
	virtual ~CubeEyeList() = default;
};

END_NAMESPACE

#endif /* CUBEEYELIST_H_ */

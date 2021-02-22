
//  Copyright (c) 2003-2020 Xsens Technologies B.V. or subsidiaries worldwide.
//  All rights reserved.
//  
//  Redistribution and use in source and binary forms, with or without modification,
//  are permitted provided that the following conditions are met:
//  
//  1.	Redistributions of source code must retain the above copyright notice,
//  	this list of conditions, and the following disclaimer.
//  
//  2.	Redistributions in binary form must reproduce the above copyright notice,
//  	this list of conditions, and the following disclaimer in the documentation
//  	and/or other materials provided with the distribution.
//  
//  3.	Neither the names of the copyright holders nor the names of their contributors
//  	may be used to endorse or promote products derived from this software without
//  	specific prior written permission.
//  
//  THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
//  EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
//  MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
//  THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
//  SPECIAL, EXEMPLARY OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT 
//  OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
//  HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY OR
//  TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
//  SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.THE LAWS OF THE NETHERLANDS 
//  SHALL BE EXCLUSIVELY APPLICABLE AND ANY DISPUTES SHALL BE FINALLY SETTLED UNDER THE RULES 
//  OF ARBITRATION OF THE INTERNATIONAL CHAMBER OF COMMERCE IN THE HAGUE BY ONE OR MORE 
//  ARBITRATORS APPOINTED IN ACCORDANCE WITH SAID RULES.
//  


//  Copyright (c) 2003-2020 Xsens Technologies B.V. or subsidiaries worldwide.
//  All rights reserved.
//  
//  Redistribution and use in source and binary forms, with or without modification,
//  are permitted provided that the following conditions are met:
//  
//  1.	Redistributions of source code must retain the above copyright notice,
//  	this list of conditions, and the following disclaimer.
//  
//  2.	Redistributions in binary form must reproduce the above copyright notice,
//  	this list of conditions, and the following disclaimer in the documentation
//  	and/or other materials provided with the distribution.
//  
//  3.	Neither the names of the copyright holders nor the names of their contributors
//  	may be used to endorse or promote products derived from this software without
//  	specific prior written permission.
//  
//  THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
//  EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
//  MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
//  THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
//  SPECIAL, EXEMPLARY OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT 
//  OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
//  HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY OR
//  TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
//  SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.THE LAWS OF THE NETHERLANDS 
//  SHALL BE EXCLUSIVELY APPLICABLE AND ANY DISPUTES SHALL BE FINALLY SETTLED UNDER THE RULES 
//  OF ARBITRATION OF THE INTERNATIONAL CHAMBER OF COMMERCE IN THE HAGUE BY ONE OR MORE 
//  ARBITRATORS APPOINTED IN ACCORDANCE WITH SAID RULES.
//  

#ifndef XSGLOVEDATA_H
#define XSGLOVEDATA_H

#include "xstypesconfig.h"
#include "xsvector3.h"
#include "xsquaternion.h"

struct XsGloveData;
struct XsFingerData;

#ifdef __cplusplus
extern "C" {
#endif
#ifndef __cplusplus
#define XSFINGERDATA_INITIALIZER {	XSQUATERNION_INITIALIZER, XSVECTOR3_INITIALIZER, XSVECTOR3_INITIALIZER, 0, 0, 0}
#define XSGLOVEDATA_INITIALIZER {	XSFINGERDATA_INITIALIZER, XSFINGERDATA_INITIALIZER, XSFINGERDATA_INITIALIZER, XSFINGERDATA_INITIALIZER, XSFINGERDATA_INITIALIZER, XSFINGERDATA_INITIALIZER, \
									XSFINGERDATA_INITIALIZER, XSFINGERDATA_INITIALIZER, XSFINGERDATA_INITIALIZER, XSFINGERDATA_INITIALIZER, XSFINGERDATA_INITIALIZER, XSFINGERDATA_INITIALIZER, \
									0 ,0 ,0 ,0 }
#endif

XSTYPES_DLL_API void XsFingerData_construct(struct XsFingerData* thisPtr);
XSTYPES_DLL_API void XsFingerData_destruct(struct XsFingerData* thisPtr);
XSTYPES_DLL_API void XsFingerData_copy(struct XsFingerData* copy, struct XsFingerData const* src);
XSTYPES_DLL_API void XsFingerData_swap(struct XsFingerData* lhs, struct XsFingerData* rhs);

XSTYPES_DLL_API void XsGloveData_construct(struct XsGloveData* thisPtr);
XSTYPES_DLL_API void XsGloveData_destruct(struct XsGloveData* thisPtr);
XSTYPES_DLL_API void XsGloveData_copy(struct XsGloveData* copy, struct XsGloveData const* src);
XSTYPES_DLL_API void XsGloveData_swap(struct XsGloveData* lhs, struct XsGloveData* rhs);

#ifdef __cplusplus
} // extern "C"
#endif

#define XSFINGERSEGMENTCOUNT	12

/*! \brief A container for Finger data
*/
struct XsFingerData
{
#ifdef __cplusplus
	//! \brief Construct an empty object
	inline XsFingerData()
		: m_flags(0)
	{
	}

	//! \brief Construct an initialized object
	inline XsFingerData(const XsQuaternion& dq, const XsVector& dv, const XsVector& mag, const uint16_t flags)
		: m_orientationIncrement(dq)
		, m_velocityIncrement(dv)
		, m_mag(mag)
		, m_flags(flags)
	{
	}

	//! \brief Copy constructor
	inline XsFingerData(const XsFingerData& other)
		: m_orientationIncrement(other.m_orientationIncrement)
		, m_velocityIncrement(other.m_velocityIncrement)
		, m_mag(other.m_mag)
		, m_flags(other.m_flags)
	{
	}

	//! \brief Assignment operator
	inline const XsFingerData& operator=(const XsFingerData& other)
	{
		if (this != &other)
		{
			m_orientationIncrement = other.m_orientationIncrement;
			m_velocityIncrement = other.m_velocityIncrement;
			m_mag = other.m_mag;
			m_flags = other.m_flags;
		}
		return *this;
	}

	//! \brief Clear the object so it contains unity data
	inline void clear()
	{
		m_orientationIncrement = XsQuaternion::identity();
		m_velocityIncrement.setZero();
		m_mag.setZero();
	}

	//! \brief Returns the contained orientation increment
	inline const XsQuaternion& orientationIncrement() const
	{
		return m_orientationIncrement;
	}

	//! \brief Returns the contained velocity increment
	inline const XsVector3& velocityIncrement() const
	{
		return m_velocityIncrement;
	}

	//! \brief Returns the mag
	inline const XsVector3& mag() const
	{
		return m_mag;
	}

	//! \brief Returns the flags
	inline uint16_t flags() const
	{
		return m_flags;
	}

	/*! \brief Returns true if all fields of this and \a other are exactly identical */
	inline bool operator == (const XsFingerData& other) const
	{
		return	m_orientationIncrement == other.m_orientationIncrement &&
			m_velocityIncrement == other.m_velocityIncrement &&
			m_mag == other.m_mag &&
			m_flags == other.m_flags;
	}

	/*! \brief Returns true if not all fields of this and \a other are exactly identical */
	inline bool operator != (const XsFingerData& other) const
	{
		return !operator==(other);
	}

	/*! \brief Swap the contents of this with \a other */
	inline void swap(XsFingerData& other)
	{
		XsFingerData_swap(this, &other);
	}

	/*! \brief Swap the contents of \a first with \a second */
	friend void swap(XsFingerData& first, XsFingerData& second)
	{
		first.swap(second);
	}

protected:
#endif

	XsQuaternion	m_orientationIncrement;	//!< The orientation increment for this segment
	XsVector3		m_velocityIncrement;	//!< The velocity increment for this segment
	XsVector3		m_mag;					//!< The magnetic field data for this segment
	uint16_t		m_flags;				//!< Data quality flags (LSB --> MSB: acc x,y,z gyr x,y,z mag x,y,z)
};
typedef struct XsFingerData XsFingerData;

/*! \brief A container for glove data
*/
struct XsGloveData
{
#ifdef __cplusplus
	//! \brief Construct an empty object
	inline XsGloveData()
		: m_frameNumber(0)
		, m_validSampleFlags(0)
	{
	}

	//! \brief Construct an initialized object
	inline XsGloveData(const uint16_t frameNumber, const uint16_t validSampleFlags, const XsFingerData *fingerData)
		: m_frameNumber(frameNumber)
		, m_validSampleFlags(validSampleFlags)
	{
		for (int i = 0; i < XSFINGERSEGMENTCOUNT; ++i)
			m_fingerData[i] = fingerData[i];
	}

	//! \brief Copy constructor
	inline XsGloveData(const XsGloveData& other)
		: m_frameNumber(other.m_frameNumber)
		, m_validSampleFlags(other.m_validSampleFlags)
	{
		for (int i = 0; i < XSFINGERSEGMENTCOUNT; ++i)
			m_fingerData[i] = other.m_fingerData[i];
	}

	//! \brief Returns the snapshot counter
	inline uint32_t frameNumber() const
	{
		return m_frameNumber;
	}

	//! \brief Returns the valid sample flags
	inline uint16_t validSampleFlags() const
	{
		return m_validSampleFlags;
	}


	//! \brief Returns the number of items in fingerData
	inline static int fingerSegmentCount()
	{
		return XSFINGERSEGMENTCOUNT;
	}

	//! \brief Returns the finger data
	inline XsFingerData const& fingerData(int i) const
	{
		assert(i >= 0 && i < XSFINGERSEGMENTCOUNT);
		return m_fingerData[i];
	}

	/*! \brief Returns true if all fields of this and \a other are exactly identical */
	inline bool operator == (const XsGloveData& other) const
	{
		if (m_frameNumber != other.m_frameNumber ||
			m_validSampleFlags != other.m_validSampleFlags )
			return false;

		for (int i = 0; i < XSFINGERSEGMENTCOUNT; ++i)
		{
			if (!(m_fingerData[i] == other.m_fingerData[i]))
				return false;
		}
		return true;
	}

	/*! \brief Returns true if not all fields of this and \a other are exactly identical */
	inline bool operator != (const XsGloveData& other) const
	{
		return !operator==(other);
	}

	//! \brief Assignment operator, copies contents of \a other into this
	inline XsGloveData const& operator = (XsGloveData const& other)
	{
		XsGloveData_copy(this, &other);
		return *this;
	}

	/*! \brief Swap the contents of this with \a other */
	inline void swap(XsGloveData& other)
	{
		XsGloveData_swap(this, &other);
	}
	
	/*! \brief Swap the contents of \a first with \a second */
	friend void swap(XsGloveData& first, XsGloveData& second)
	{
		first.swap(second);
	}

protected:
#endif
	XsFingerData m_fingerData[XSFINGERSEGMENTCOUNT];	//!< Data for each tracked finger segment
	uint32_t m_frameNumber;		//!< The sequential frame number for the data
	uint16_t m_validSampleFlags;	//!< Flags describing the general validity of the data
};

typedef struct XsGloveData XsGloveData;

#endif

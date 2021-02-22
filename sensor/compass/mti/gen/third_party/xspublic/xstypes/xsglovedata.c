
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

#include "xsglovedata.h"
#include "xsquaternion.h"

/*! \class XsGloveData
	\brief Container for Glove data.
*/

#define SWAP(T, a, b)	{ T tmp = a; a = b; b = tmp; }

/*! \addtogroup cinterface C Interface
	@{
*/

/*! \relates XsGloveData
	\brief Initialize an %XsGloveData object
*/
void XsGloveData_construct(struct XsGloveData* thisPtr)
{
	int i;
	memset(thisPtr, 0, sizeof(XsGloveData));
	for (i = 0; i < XSFINGERSEGMENTCOUNT; i++)
		XsFingerData_construct(&thisPtr->m_fingerData[i]);
}

/*! \relates XsGloveData
	\brief Destruct an %XsGloveData object
*/
void XsGloveData_destruct(struct XsGloveData* thisPtr)
{
	// no destruction necessary, no dynamic memory allocated
	(void)thisPtr;
}

/*! \relates XsGloveData
	\brief Copy the contents of an %XsGloveData object
	\param copy The destination of the copy operation
	\param src The object to copy from
*/
void XsGloveData_copy(struct XsGloveData* copy, struct XsGloveData const* src)
{
	if (copy == src)
		return;
	int i;
	copy->m_frameNumber = src->m_frameNumber;
	copy->m_validSampleFlags = src->m_validSampleFlags;
	for (i = 0; i < XSFINGERSEGMENTCOUNT; i++)
		XsFingerData_copy(&copy->m_fingerData[i], &src->m_fingerData[i]);
}

/*! \relates XsGloveData
	\brief Swap the contents of two %XsGloveData objects
	\param lhs The left hand side object of the swap
	\param rhs The right hand side object of the swap
*/
void XsGloveData_swap(struct XsGloveData* lhs, struct XsGloveData* rhs)
{
	if (lhs == rhs)
		return;
	int i;
	SWAP(uint32_t, lhs->m_frameNumber, rhs->m_frameNumber);
	SWAP(uint16_t, lhs->m_validSampleFlags, rhs->m_validSampleFlags);
	for (i = 0; i < XSFINGERSEGMENTCOUNT; i++)
		XsFingerData_swap(&lhs->m_fingerData[i], &rhs->m_fingerData[i]);
}

/*! \relates XsFingerData
	\brief Initialize an %XsFingerData object
*/
void XsFingerData_construct(struct XsFingerData* thisPtr)
{
	// XsQuaternion_construct(&thisPtr->m_orientationIncrement);
	XsVector3_construct(&thisPtr->m_velocityIncrement, 0);
	XsVector3_construct(&thisPtr->m_mag, 0);
	thisPtr->m_flags = 0;
}

/*! \relates XsFingerData
\brief Destruct an %XsFingerData object
*/
void XsFingerData_destruct(struct XsFingerData* thisPtr)
{
	// no destruction necessary, no dynamic memory allocated
	(void)thisPtr;
}

/*! \relates XsFingerData
	\brief Copy the contents of an %XsFingerData object
	\param copy The destination of the copy operation
	\param src The object to copy from
*/
void XsFingerData_copy(struct XsFingerData* copy, struct XsFingerData const* src)
{
	if (copy == src)
		return;
	XsQuaternion_copy(&copy->m_orientationIncrement, &src->m_orientationIncrement);
	XsVector3_copy(&copy->m_velocityIncrement.m_vector, &src->m_velocityIncrement);
	XsVector3_copy(&copy->m_mag.m_vector, &src->m_mag);
	copy->m_flags = src->m_flags;
}

/*! \relates XsFingerData
	\brief Swap the contents of two %XsGloveData objects
	\param lhs The left hand side object of the swap
	\param rhs The right hand side object of the swap
*/
void XsFingerData_swap(struct XsFingerData* lhs, struct XsFingerData* rhs)
{
	if (lhs == rhs)
		return;
	XsQuaternion_swap(&lhs->m_orientationIncrement, &rhs->m_orientationIncrement);
	XsVector_swap(&lhs->m_velocityIncrement.m_vector, &rhs->m_velocityIncrement.m_vector);
	XsVector_swap(&lhs->m_mag.m_vector, &rhs->m_mag.m_vector);

	SWAP(uint16_t, lhs->m_flags, rhs->m_flags);
}

/*! @} */

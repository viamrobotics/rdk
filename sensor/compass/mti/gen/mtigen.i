%module mtigen
%feature("flatnested", "1");

%include <stdint.i>
%include <std_string.i>

%{
	#include "third_party/include/xstypes.h"
	#include "third_party/include/xscontroller.h"

	Journaller* gJournal = 0;
%}

// swig -v -go -cgo -c++ -intgosize 64 mtigen.i

%rename(opIndex) operator[];
%rename(opLeftShift) operator<<;
%rename(opNEq) operator!=;
%rename(opEq) operator==;
%rename(opAssign) operator=;
%rename(opLt) operator<;
%rename(opGt) operator>;
%rename(opLtEq) operator<=;
%rename(opGtEq) operator>=;
%rename(opPlus) operator+;
%rename(XSDeviceId) XsDeviceId;
%rename(XSString) XsString;
%rename(XSBaudRate) XsBaudRate;
%rename(XSPortInfo) XsPortInfo;
%rename(XSScanner) XsScanner;
%rename(XSDataPacket) XsDataPacket;
%rename(XSDevice) XsDevice;
%rename(XSQuaternion) XsQuaternion;
%rename(XSEuler) XsEuler;

#define XSNOLINUXEXPORT
#define XSNOEXPORT
#define XSNOCOMEXPORT
#define XDA_DLL_API
#define XSTYPES_DLL_API
#define XSENS_NO_PORT_NUMBERS

%include "third_party/xspublic/xstypes/xstypesconfig.h"
%include "third_party/xspublic/xstypes/xstypedefs.h"
%include "third_party/xspublic/xstypes/xsarray.h"

%template(XsArrayXsPortInfo) XsArrayImpl<XsPortInfo, g_xsPortInfoArrayDescriptor, XsPortInfoArray>;
%include "third_party/xspublic/xstypes/xsportinfo.h"

%template(XsArrayImplXsDevice) XsArrayImpl<XsDevicePtr,g_xsDevicePtrArrayDescriptor,XsDevicePtrArray>;
%include "third_party/xspublic/xscontroller/xsdeviceptrarray.h"

%include "third_party/xspublic/xscontroller/xscallbackplainc.h"
%include "third_party/xspublic/xscontroller/xscallback.h"
%include "third_party/xspublic/xscontroller/callbackmanagerxda.h"
%include "third_party/xspublic/xscontroller/xsscanner.h"
%include "third_party/xspublic/xscontroller/xsdevice_def.h"
%include "third_party/xspublic/xstypes/xsdeviceid.h"

%template(XsArrayXsString) XsArrayImpl<char, g_xsStringDescriptor, XsString>;
%include "third_party/xspublic/xstypes/xsstring.h"

%include "third_party/xspublic/xstypes/xsbaudrate.h"
%include "third_party/xspublic/xstypes/xsdatapacket.h"
%include "third_party/xspublic/xstypes/xsquaternion.h"
%include "third_party/xspublic/xstypes/xseuler.h"
%include "third_party/xspublic/xscontroller/xscontrol_def.h"

%template(XsArrayImplXsOutput) XsArrayImpl<XsOutputConfiguration,g_xsOutputConfigurationArrayDescriptor,XsOutputConfigurationArray>;
%include "third_party/xspublic/xstypes/xsoutputconfigurationarray.h"

%include "third_party/xspublic/xscontroller/xscallback.h"
%include "third_party/xspublic/xscontroller/xscallbackplainc.h"

%{
#include <chrono>
#include <thread>
#include <iostream>
#include <iomanip>
#include <list>
#include <string>
#include <mutex>

class CallbackHandler : public XsCallback
{
public:
	CallbackHandler(size_t maxBufferSize = 5)
		: m_maxNumberOfPacketsInBuffer(maxBufferSize)
		, m_numberOfPacketsInBuffer(0)
	{
	}

	virtual ~CallbackHandler() throw()
	{
	}

	bool packetAvailable() const
	{
		std::lock_guard<std::mutex> guard(m_mutex);
		return m_numberOfPacketsInBuffer > 0;
	}

	XsDataPacket getNextPacket()
	{
		assert(packetAvailable());
		std::lock_guard<std::mutex> guard(m_mutex);
		XsDataPacket oldestPacket(m_packetBuffer.front());
		m_packetBuffer.pop_front();
		--m_numberOfPacketsInBuffer;
		return oldestPacket;
	}

protected:
	void onLiveDataAvailable(XsDevice*, const XsDataPacket* packet) override
	{
		std::lock_guard<std::mutex> guard(m_mutex);
		assert(packet != 0);
		while (m_numberOfPacketsInBuffer >= m_maxNumberOfPacketsInBuffer)
			(void)getNextPacket();

		m_packetBuffer.push_back(*packet);
		++m_numberOfPacketsInBuffer;
		assert(m_numberOfPacketsInBuffer <= m_maxNumberOfPacketsInBuffer);
	}
private:
	mutable std::mutex m_mutex;

	size_t m_maxNumberOfPacketsInBuffer;
	size_t m_numberOfPacketsInBuffer;
	std::list<XsDataPacket> m_packetBuffer;
};

using namespace std;

void addCallbackHandler(CallbackHandler* cb, XsDevice* dev) {
	dev->addCallbackHandler(cb);
}

%}

class CallbackHandler : public XsCallback
{
public:
	CallbackHandler(size_t maxBufferSize = 5);
	virtual ~CallbackHandler() throw();
	bool packetAvailable() const;
	XsDataPacket getNextPacket();
protected:
	void onLiveDataAvailable(XsDevice*, const XsDataPacket* packet) override;
};
void addCallbackHandler(CallbackHandler* cb, XsDevice* dev);

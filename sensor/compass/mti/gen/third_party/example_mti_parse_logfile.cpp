
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

//--------------------------------------------------------------------------------
// Public Xsens device API C++ example MTi parse log file.
//--------------------------------------------------------------------------------
#include <xscontroller/xscontrol_def.h>
#include <xscontroller/xsdevice_def.h>
#include <xscontroller/xsscanner.h>
#include <xstypes/xsoutputconfigurationarray.h>
#include <xstypes/xsdatapacket.h>
#include <xstypes/xstime.h>
#include <xstypes/xsthread.h>

#include <iostream>
#include <iomanip>
#include <fstream>
#include <string>
#include <atomic>

Journaller* gJournal = 0;

using namespace std;

class CallbackHandler : public XsCallback
{
public:
	CallbackHandler()
		: m_progress(0)
	{
	}

	virtual ~CallbackHandler() throw()
	{
	}

	int progress() const
	{
		return m_progress;
	}

protected:
	void onProgressUpdated(XsDevice* dev, int current, int total, const XsString* identifier) override
	{
		(void)dev;
		(void)total;
		(void)identifier;
		m_progress = current;
	}
private:
	volatile std::atomic_int m_progress;
};

//--------------------------------------------------------------------------------
int main(void)
{
	cout << "Creating XsControl object..." << endl;
	XsControl* control = XsControl::construct();
	assert(control != 0);

	// Lambda function for error handling
	auto handleError = [=](string errorString)
	{
		control->destruct();
		cout << errorString << endl;
		cout << "Press [ENTER] to continue." << endl;
		cin.get();
		return -1;
	};

	cout << "Opening log file..." << endl;
	string logFileName = "logfile.mtb";
	if (!control->openLogFile(logFileName))
		return handleError("Failed to open log file. Aborting.");
	else
		cout << "Opened log file: " << logFileName.c_str() << endl;

	// Get number of devices in the file
	vector<XsDeviceId> deviceIdArray = control->mainDeviceIds();
	XsDeviceId mtDevice;
	// Find an MTi device
	for (auto const &deviceId : deviceIdArray)
	{
		if (deviceId.isMti() || deviceId.isMtig())
		{
			mtDevice = deviceId;
			break;
		}
	}

	if (!mtDevice.isValid())
		return handleError("No MTi device found. Aborting.");

	// Get the device object
	XsDevice* device = control->device(mtDevice);
	assert(device != nullptr);

	cout << "Device with ID: " << device->deviceId().toString() << " found in file." << endl;

	// By default XDA does not retain data for reading it back.
	// By enabling this option XDA keeps the buffered data in a cache so it can be accessed 
	// through XsDevice::getDataPacketByIndex or XsDevice::takeFirstDataPacketInQueue
	device->setOptions(XSO_RetainBufferedData, XSO_None);

	// Load the log file and wait until it is loaded
	// Wait for logfile to be fully loaded, there are three ways to do this:
	// - callback: Demonstrated here, which has loading progress information
	// - waitForLoadLogFileDone: Blocking function, returning when file is loaded
	// - isLoadLogFileInProgress: Query function, used to query the device if the loading is done
	//
	// The callback option is used here.

	// Create and attach callback handler to device
	CallbackHandler callback;
	device->addCallbackHandler(&callback);

	cout << "Loading the file..." << endl;
	device->loadLogFile();
	while(callback.progress() != 100)
		xsYield();
	cout << "File is fully loaded" << endl;

	cout << "Exporting the data..." << endl;
	string exportFileName = "exportfile.txt";
	ofstream outfile;
	outfile.open(exportFileName);

	// Get total number of samples
	XsSize packetCount = device->getDataPacketCount();
	for (XsSize i = 0; i < packetCount; i++)
	{
		outfile << setw(5) << fixed << setprecision(2);

		// Retrieve a packet
		XsDataPacket packet = device->getDataPacketByIndex(i);

		if (packet.containsCalibratedData())
		{
			XsVector acc = packet.calibratedAcceleration();
			outfile << "Acc X:" << acc[0]
					<< ", Acc Y:" << acc[1]
					<< ", Acc Z:" << acc[2];

			XsVector gyr = packet.calibratedGyroscopeData();
			outfile << " |Gyr X:" << gyr[0]
					<< ", Gyr Y:" << gyr[1]
					<< ", Gyr Z:" << gyr[2];

			XsVector mag = packet.calibratedMagneticField();
			outfile << " |Mag X:" << mag[0]
					<< ", Mag Y:" << mag[1]
					<< ", Mag Z:" << mag[2];
		}

		if (packet.containsOrientation())
		{
			XsQuaternion quaternion = packet.orientationQuaternion();
			outfile << "q0:" << quaternion.w()
					<< ", q1:" << quaternion.x()
					<< ", q2:" << quaternion.y()
					<< ", q3:" << quaternion.z();

			XsEuler euler = packet.orientationEuler();
			outfile << " |Roll:" << euler.roll()
					<< ", Pitch:" << euler.pitch()
					<< ", Yaw:" << euler.yaw();
		}

		if (packet.containsLatitudeLongitude())
		{
			XsVector latLon = packet.latitudeLongitude();
			outfile << " |Lat:" << latLon[0]
					<< ", Lon:" << latLon[1];
		}

		if (packet.containsAltitude())
			outfile << " |Alt:" << packet.altitude();

		if (packet.containsVelocity())
		{
			XsVector vel = packet.velocity(XDI_CoordSysEnu);
			outfile << "|E:" << vel[0]
					<< ", N:" << vel[1]
					<< ", U:" << vel[2];
		}

		outfile << endl;
	}
	outfile.close();
	cout << "File is exported to: " << exportFileName << endl;

	cout << "Freeing XsControl object..." << endl;
	control->destruct();

	cout << "Successful exit." << endl;

	cout << "Press [ENTER] to continue." << endl;
	cin.get();

	return 0;
}

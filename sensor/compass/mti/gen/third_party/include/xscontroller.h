
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

#include "xscontroller/xsfloatformat.h"
#include "xscontroller/usbcommunicator.h"
#include "xscontroller/xsdeviceparameter.h"
#include "xscontroller/xsdeviceptrarray.h"
#include "xscontroller/xsicccommand.h"
#include "xscontroller/mtdevice.h"
#include "xscontroller/xscoordinatesystem.h"
#include "xscontroller/communicatorfactory.h"
#include "xscontroller/xscontrollerconfig.h"
#include "xscontroller/mtix0device.h"
#include "xscontroller/messageserializer.h"
#include "xscontroller/mtixdevice.h"
#include "xscontroller/nmea_protocolhandler.h"
#include "xscontroller/synclinemk4.h"
#include "xscontroller/simpleprotocolmanager.h"
#include "xscontroller/replymonitor.h"
#include "xscontroller/serialcommunicator.h"
#include "xscontroller/broadcastdevice.h"
#include "xscontroller/enumexpanders.h"
#include "xscontroller/protocolmanager.h"
#include "xscontroller/mtix00device.h"
#include "xscontroller/xsdeviceconfiguration.h"
#include "xscontroller/xsgnssplatform.h"
#include "xscontroller/serialportcommunicator.h"
#include "xscontroller/packeterrorrateestimator.h"
#include "xscontroller/rx_tx_log.h"
#include "xscontroller/deviceredetector.h"
#include "xscontroller/xswinusb.h"
#include "xscontroller/xsdevice_def.h"
#include "xscontroller/xsrejectreason.h"
#include "xscontroller/messageextractor.h"
#include "xscontroller/streaminterface.h"
#include "xscontroller/idfetchhelpers.h"
#include "xscontroller/openportstage.h"
#include "xscontroller/xsaccesscontrolmode.h"
#include "xscontroller/xsdevice_public.h"
#include "xscontroller/xsscanner.h"
#include "xscontroller/iointerfacefile.h"
#include "xscontroller/xsdeviceparameteridentifier.h"
#include "xscontroller/serialinterface.h"
#include "xscontroller/mti3x0device.h"
#include "xscontroller/iprotocolmanager.h"
#include "xscontroller/iprotocolhandler.h"
#include "xscontroller/xserrormode.h"
#include "xscontroller/usbinterface.h"
#include "xscontroller/xsselftestresult.h"
#include "xscontroller/callbackmanagerxda.h"
#include "xscontroller/synclinegmt.h"
#include "xscontroller/mti6x0device.h"
#include "xscontroller/xsdevicestate.h"
#include "xscontroller/xsdeviceptr.h"
#include "xscontroller/xsprocessingflag.h"
#include "xscontroller/mtbfilecommunicator.h"
#include "xscontroller/messagelocation.h"
#include "xscontroller/datalogger.h"
#include "xscontroller/xsorientationmode.h"
#include "xscontroller/xslibusb.h"
#include "xscontroller/communicator.h"
#include "xscontroller/replyobject.h"
#include "xscontroller/xscallbackplainc.h"
#include "xscontroller/devicefactory.h"
#include "xscontroller/xsprotocoltype.h"
#include "xscontroller/xscontrol_public.h"
#include "xscontroller/protocolhandler.h"
#include "xscontroller/compat.h"
#include "xscontroller/xscallback.h"
#include "xscontroller/packetstamper.h"
#include "xscontroller/xscontrol_def.h"
#include "xscontroller/enumerateusbdevices.h"
#include "xscontroller/devicetypes.h"
#include "xscontroller/datapacketcache.h"
#include "xscontroller/mtbdatalogger.h"
#include "xscontroller/mtibasedevice.h"
#include "xscontroller/datapoller.h"
#include "xscontroller/clocksynccommand.h"
#include "xscontroller/xsdef.h"
#include "xscontroller/proxycommunicator.h"
#include "xscontroller/mtthread.h"
#include "xscontroller/iointerface.h"
#include "xscontroller/xsalignmentframe.h"
#include "xscontroller/restorecommunication.h"
#include "xscontroller/mti8x0device.h"
#include "xscontroller/lastresultmanager.h"
#include "xscontroller/rangequeue.h"
#include "xscontroller/gpsstatus.h"
#include "xscontroller/scenariomatchpred.h"
#include "xscontroller/supportedsyncsettings.h"
#include "xscontroller/mtigdevice.h"
#include "xscontroller/mtsyncsettings.h"
#include "xscontroller/udev.h"
#include "xscontroller/xscalibrateddatamode.h"
#include "xscontroller/fileloader.h"
#include "xscontroller/devicecommunicator.h"
#include "xscontroller/dataparser.h"
#include "xscontroller/xdacommunicatorfactory.h"
#include "xscontroller/scanner.h"
#include "xscontroller/xsusbhubinfo.h"
#include "xscontroller/xsconnectivitystate.h"
#include "xscontroller/xsiccrepmotionresult.h"
#include "xscontroller/xsoperationalmode.h"
#include "xscontroller/mti7device.h"

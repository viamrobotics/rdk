
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

#include "xstypes/pstdint.h"
#include "xstypes/xsintarray.h"
#include "xstypes/xsrssi.h"
#include "xstypes/xsbusid.h"
#include "xstypes/xsarray.h"
#include "xstypes/xsresetmethod.h"
#include "xstypes/xsmessagearray.h"
#include "xstypes/xsoutputconfiguration.h"
#include "xstypes/xsfilterprofilekind.h"
#include "xstypes/xscontrolline.h"
#include "xstypes/xsglovedata.h"
#include "xstypes/xsdid.h"
#include "xstypes/xsplatform.h"
#include "xstypes/xssyncrole.h"
#include "xstypes/xsdeviceidarray.h"
#include "xstypes/xscandataidentifier.h"
#include "xstypes/xsmalloc.h"
#include "xstypes/xstypedefs.h"
#include "xstypes/xshandid.h"
#include "xstypes/xssyncpolarity.h"
#include "xstypes/xstimestamp.h"
#include "xstypes/xsfilepos.h"
#include "xstypes/xsstringstreaming.h"
#include "xstypes/xsmessage.h"
#include "xstypes/xsfloatmath.h"
#include "xstypes/xsmfmresultvalue.h"
#include "xstypes/xsstatusflag.h"
#include "xstypes/xssimpleversion.h"
#include "xstypes/xsstringoutputtype.h"
#include "xstypes/xspressure.h"
#include "xstypes/xsfilterprofilearray.h"
#include "xstypes/xsprotocol.h"
#include "xstypes/xsversion.h"
#include "xstypes/xsxbusmessageid.h"
#include "xstypes/xsushortvector.h"
#include "xstypes/xsscrdata.h"
#include "xstypes/xsportinfoarray.h"
#include "xstypes/xsbaud.h"
#include "xstypes/xscalibrateddata.h"
#include "xstypes/xscanoutputconfigurationarray.h"
#include "xstypes/xsbytearray.h"
#include "xstypes/xsdevicecapabilities.h"
#include "xstypes/xsens_compat.h"
#include "xstypes/xsdeviceid.h"
#include "xstypes/xsmatrix3x3.h"
#include "xstypes/xsdebugcounters.h"
#include "xstypes/xsglovesnapshot.h"
#include "xstypes/xscopy.h"
#include "xstypes/xsoutputconfigurationarray.h"
#include "xstypes/xsoption.h"
#include "xstypes/xsdatapacketptr.h"
#include "xstypes/xsmath2.h"
#include "xstypes/xscanoutputconfiguration.h"
#include "xstypes/xsfilterprofile.h"
#include "xstypes/xsstring.h"
#include "xstypes/xssyncfunction.h"
#include "xstypes/xstypesinfo.h"
#include "xstypes/xsint64array.h"
#include "xstypes/xssyncsetting.h"
#include "xstypes/xssyncline.h"
#include "xstypes/xsdeviceoptionflag.h"
#include "xstypes/xsthread.h"
#include "xstypes/xsmatrix.h"
#include "xstypes/xsfile.h"
#include "xstypes/xsexception.h"
#include "xstypes/xsbaudrate.h"
#include "xstypes/xslibraryloader.h"
#include "xstypes/xstriggerindicationdata.h"
#include "xstypes/xsvector3.h"
#include "xstypes/xsresultvalue.h"
#include "xstypes/xsrawgnsssatinfo.h"
#include "xstypes/xssensorranges.h"
#include "xstypes/xsstringarray.h"
#include "xstypes/xsquaternionarray.h"
#include "xstypes/xsanalogindata.h"
#include "xstypes/xstypesdynlib.h"
#include "xstypes/xssocket.h"
#include "xstypes/xsdataidentifier.h"
#include "xstypes/xsvector.h"
#include "xstypes/xstime.h"
#include "xstypes/xscanframeformat.h"
#include "xstypes/xsquaternion.h"
#include "xstypes/xsbaudcode.h"
#include "xstypes/xsutctime.h"
#include "xstypes/xsportinfo.h"
#include "xstypes/xssnapshot.h"
#include "xstypes/xssdidata.h"
#include "xstypes/xssyncsettingarray.h"
#include "xstypes/xseuler.h"
#include "xstypes/xsrange.h"
#include "xstypes/xsdatapacketptrarray.h"
#include "xstypes/xsinforequest.h"
#include "xstypes/xsmath.h"
#include "xstypes/xstypesconfig.h"
#include "xstypes/xsstringoutputtypearray.h"
#include "xstypes/xsdatapacket.h"
#include "xstypes/xscanbaudcode.h"
#include "xstypes/xsdataidentifiervalue.h"
#include "xstypes/xsrawgnsspvtdata.h"
#include "xstypes/xstimeinfo.h"

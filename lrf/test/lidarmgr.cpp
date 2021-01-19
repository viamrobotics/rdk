/*
 * Copyright (C) 2014  RoboPeak
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */
/*
 *  RoboPeak Lidar System
 *  Win32 Demo Application
 *
 *  Copyright 2009 - 2014 RoboPeak Team
 *  http://www.robopeak.com
 *
 */

#include "stdafx.h"
#include "lidarmgr.h"

LidarMgr * LidarMgr::g_instance = NULL;
rp::hal::Locker LidarMgr::g_oplocker;
RPlidarDriver * LidarMgr::lidar_drv = NULL;

LidarMgr & LidarMgr::GetInstance()
{
    if (g_instance) return *g_instance;
    rp::hal::AutoLocker l(g_oplocker);

    if (g_instance) return *g_instance;
    g_instance = new LidarMgr();
	lidar_drv = RPlidarDriver::CreateDriver(RPlidarDriver::DRIVER_TYPE_SERIALPORT);
    return *g_instance;
}

LidarMgr::LidarMgr()
    : _isConnected(false)
{

}

LidarMgr::~LidarMgr()
{
    rp::hal::AutoLocker l(g_oplocker);
    onDisconnect();
	delete g_instance;
	g_instance = NULL;
	lidar_drv->DisposeDriver(lidar_drv);
}

void LidarMgr::onDisconnect()
{
    if (_isConnected) {
        lidar_drv->stop();
    }
}

bool  LidarMgr::checkDeviceHealth(int * errorCode)
{
  
    int errcode = 0;
    bool ans = false;

    do {
        if (!_isConnected) {
            errcode = RESULT_OPERATION_FAIL;
            break;
        }

        rplidar_response_device_health_t healthinfo;
        if (IS_FAIL(lidar_drv->getHealth(healthinfo))) {
            errcode = RESULT_OPERATION_FAIL;
            break;
        }

        if (healthinfo.status != RPLIDAR_STATUS_OK) {
            errcode = healthinfo.error_code;
            break;
        }

        ans = true;
    } while(0);

    if (errorCode) *errorCode = errcode;
    return ans;
}

bool LidarMgr::onConnect(const char * port)
{
    if (_isConnected) return true;

    if (IS_FAIL(lidar_drv->connect(port, 115200))) return false;
    // retrieve the devinfo
    u_result ans = lidar_drv->getDeviceInfo(devinfo);

    if (IS_FAIL(ans)) {
        return false;
    }

    _isConnected = true;
    return true;
}
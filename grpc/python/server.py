import asyncio
import sys
from typing import List

from grpclib.const import Status as GRPCStatus
from grpclib.exceptions import GRPCError
from grpclib.server import Stream, Server
from grpclib.utils import graceful_exit

import gen
from proto.api.v1.robot_grpc import RobotServiceBase
from proto.api.v1.robot_pb2 import ConfigRequest, ConfigResponse, Status, StatusRequest, StatusResponse
from proto.api.service.v1.metadata_grpc import MetadataServiceBase
from proto.api.service.v1.metadata_pb2 import ResourceName, ResourcesRequest, ResourcesResponse

class RobotService(RobotServiceBase):
    
    async def Status(self, stream: Stream[StatusRequest, StatusResponse]) -> None:
        status = Status()
        status.bases['base1'] = True
        await stream.send_message(StatusResponse(status=status))

    async def BaseMoveArc(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BaseMoveStraight(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BaseSpin(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BaseStop(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BaseWidthMillis(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BoardReadAnalogReader(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BoardGetDigitalInterruptValue(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BoardGetGPIO(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BoardSetGPIO(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BoardSetPWM(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BoardSetPWMFrequency(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def BoardStatus(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def CompassHeading(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def CompassMark(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def CompassStartCalibration(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def CompassStopCalibration(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def Config(self, stream: Stream[ConfigRequest, ConfigResponse]) -> None:
        await stream.send_message(ConfigResponse())
    async def FrameServiceConfig(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def GPSAccuracy(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def GPSAltitude(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def GPSLocation(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def GPSSpeed(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def InputControllerControls(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def InputControllerEventStream(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def InputControllerInjectEvent(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def InputControllerLastEvents(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorGetPIDConfig(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorGo(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorGoFor(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorGoTillStop(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorGoTo(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorIsOn(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorOff(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorPIDStep(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorPosition(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorPositionSupported(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorPower(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorSetPIDConfig(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def MotorZero(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def NavigationServiceAddWaypoint(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def NavigationServiceLocation(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def NavigationServiceMode(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def NavigationServiceRemoveWaypoint(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def NavigationServiceSetMode(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def NavigationServiceWaypoints(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def Move(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def ResourceRunCommand(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def SensorReadings(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)
    async def StatusStream(self, stream) -> None:
        raise GRPCError(GRPCStatus.UNIMPLEMENTED)

class MetadataService(MetadataServiceBase):
    async def Resources(self, stream: Stream[ResourcesRequest, ResourcesResponse]) -> None:

        await stream.send_message(ResourcesResponse())

async def main(args: List[str]) -> int:
    host = None
    if len(args) > 2:
        host = args[1]
    else:
        host='127.0.0.1'
    port = None
    if len(args) > 2:
        port = int(args[2])
    else:
        port=50051
    server = Server([RobotService(), MetadataService()])
    with graceful_exit([server]):
        await server.start(host, port)
        print(f'Serving on {host}:{port}')
        await server.wait_closed()
    return 0

if __name__ == "__main__":
    sys.exit(asyncio.run(main(sys.argv)))

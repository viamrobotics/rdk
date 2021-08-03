import asyncio
import sys
from typing import List

from grpclib.const import Status
from grpclib.exceptions import GRPCError
from grpclib.server import Stream, Server
from grpclib.utils import graceful_exit

import gen
from proto.api.v1.robot_grpc import RobotServiceBase
from proto.api.v1.robot_pb2 import Status, StatusRequest, StatusResponse

class RobotService(RobotServiceBase):
    
    async def Status(self, stream: Stream[StatusRequest, StatusResponse]) -> None:
        status = Status()
        status.bases['base1'] = True
        await stream.send_message(StatusResponse(status=status))

    async def StatusStream(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def Config(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def DoAction(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def ArmCurrentPosition(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def ArmMoveToPosition(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def ArmCurrentJointPositions(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def ArmMoveToJointPositions(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def BaseMoveStraight(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def BaseSpin(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def BaseStop(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def GripperOpen(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def GripperGrab(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def CameraFrame(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def CameraRenderFrame(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def PointCloud(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def ObjectPointClouds(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def LidarInfo(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def LidarStart(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def LidarStop(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def LidarScan(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def LidarRange(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def LidarBounds(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def LidarAngularResolution(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def BoardStatus(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def BoardMotorGo(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def BoardMotorGoFor(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def BoardServoMove(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def SensorReadings(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def CompassHeading(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def CompassStartCalibration(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def CompassStopCalibration(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)

    async def CompassMark(self, stream) -> None:
        raise GRPCError(Status.UNIMPLEMENTED)


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
    server = Server([RobotService()])
    with graceful_exit([server]):
        await server.start(host, port)
        print(f'Serving on {host}:{port}')
        await server.wait_closed()
    return 0

if __name__ == "__main__":
    sys.exit(asyncio.run(main(sys.argv)))

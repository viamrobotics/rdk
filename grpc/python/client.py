import asyncio
import sys
from typing import List

from grpclib.client import Channel

import gen
from proto.api.v1.robot_pb2 import StatusRequest, StatusResponse
from proto.api.v1.robot_grpc import RobotServiceStub

async def main(args: List[str]) -> int:
    if len(args) < 2:
        print("must supply grpc address", file=sys.stderr)
        return 1
    grpc_addr = args[1]
    grpc_port = None
    if len(args) > 2:
        grpc_port = args[2]
    try:
        async with Channel(grpc_addr, grpc_port) as channel:
            client = RobotServiceStub(channel)
            status = await client.Status(StatusRequest())
            print(status)
    except Exception as e:
        print(e, file=sys.stderr)
        return 1
    return 0

if __name__ == "__main__":
    sys.exit(asyncio.run(main(sys.argv)))

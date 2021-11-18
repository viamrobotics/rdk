import asyncio
import sys
import time
from typing import List

from grpclib.client import Channel

import gen
from proto.api.v1.robot_pb2 import ForceMatrixMatrixRequest, ForceMatrixMatrixResponse
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

            print('Time\t\t\tTop\t\t\t\t\t\t\tBottom')
            while True:
                async def getMatrix(name: str):
                    req = ForceMatrixMatrixRequest()
                    req.name = name
                    resp = await client.ForceMatrixMatrix(req)
                    print('\t[', end='')
                    for rowNum in range(resp.matrix.rows):
                        print('[', end='')
                        for colNum in range(resp.matrix.cols):
                            print(resp.matrix.data[(rowNum*resp.matrix.cols)+colNum], end='')
                            if colNum == resp.matrix.cols-1:
                                continue
                            print(',', end='')
                        print(']', end='')
                        if rowNum == resp.matrix.rows-1:
                            continue
                        print(',', end='')
                    print(']', end=' ')
                now = time.time()
                print(now, end='')
                await getMatrix('matrix-top')
                await getMatrix('matrix-bottom')
                print()
    except Exception as e:
        print(e, file=sys.stderr)
        return 1
    return 0

if __name__ == "__main__":
    loop = asyncio.get_event_loop()
    sys.exit(loop.run_until_complete((main(sys.argv))))

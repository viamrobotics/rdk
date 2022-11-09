import asyncio

from viam.robot.client import RobotClient
from viam.rpc.dial import Credentials, DialOptions
from viam.components.arm import Arm, JointPositions
from viam.proto.common import Pose, PoseInFrame, WorldState, GeometriesInFrame, Geometry, RectangularPrism, ResourceName, Vector3, Transform
from viam.services.motion import MotionServiceClient
import time

async def connect():
    creds = Credentials(
        type='robot-location-secret',
        payload='ewvmwn3qs6wqcrbnewwe1g231nvzlx5k5r5g34c31n6f7hs8')
    opts = RobotClient.Options(
        refresh_interval=0,
        dial_options=DialOptions(credentials=creds)
    )
    return await RobotClient.at_address('ray-pi-main.tcz8zh8cf6.viam.cloud', opts)
async def main():
    robot = await connect()
    
    arm_name = "sasha"
    motion = MotionServiceClient.from_robot(robot)
    arm = Arm.from_robot(robot, arm_name)

     #####    #####   #######  #     #  #######       #####  
    #     #  #     #  #        ##    #  #            #     # 
    #        #        #        # #   #  #                  # 
     #####   #        #####    #  #  #  #####         #####  
          #  #        #        #   # #  #            #       
    #     #  #     #  #        #    ##  #            #       
     #####    #####   #######  #     #  #######      ####### 

    # create a geometry representing the table to which the arm is attached
    table = Geometry(center=Pose(x=0, y=0, z=-20), box=RectangularPrism(dims_mm=Vector3(x=2000, y=2000, z=40)))
    tableFrame = GeometriesInFrame(reference_frame=arm_name, geometries=[table])

    # create a geometry 200mm behind the arm to block it from hitting the xarm
    xARM = Geometry(center=Pose(x=600, y=0, z=0), box=RectangularPrism(dims_mm=Vector3(x=200, y=200, z=600)))
    xARMFrame = GeometriesInFrame(reference_frame=arm_name, geometries=[xARM])

    worldstate = WorldState(obstacles=[tableFrame, xARMFrame] )
    startInput = JointPositions(values=[-8,-28,57,-28,-8,0]) 
    
    await arm.move_to_joint_positions(startInput)
    
    pose = await arm.get_end_position()
    pose.x += 600
    pose.y += 600

    await arm.move_to_position(pose, world_state=worldstate)

    await robot.close()

    # for resname in robot.resource_names:
    #     if resname.name == arm_name:
    #         print (resname)
    #         await motion.move(component_name=resname, destination = PoseInFrame(reference_frame="world", pose=pose), world_state=worldstate)
    
    # await arm.move_to_joint_positions(startInput)

if __name__ == '__main__':
  asyncio.run(main())
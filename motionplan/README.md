
# Kinematics

This provides various models and software to determine the forward/inverse kinematics positions of a variety of robot arms.
## Contents
* `models` contains the WRL and XML files that specify various robot arms
* `go-kin` contains a kinematics lobrary capable of parsing the above model files and performing forward and inverse kinematics on them
* Maybe other things that didn't exist when I last updated this file

### Models
This tutorial can mostly be followed to create a new robot model, assuming you already have some sort of 3d model: https://www.roboticslibrary.org/tutorials/create-a-robot-model/
### go-kin
This is a 100% Go port of [Robotics Library](https://github.com/roboticslibrary/rl). Currently all that has been implemented is part of `mdl` and `math`, enough to load a model and calculate forward and inverse kinematics. Dynamics are currently out of scope.
## Installation
`go get github.com/viamrobotics/kinematics/`
## Operation

```
// Load the model file
m, _ := ParseFile("/home/peter/Documents/echo/kinematics/models/mdl/wx250s.xml")

// Initialize the IK solver with the model
ik := CreateJacobianIKSolver(m)

// ForwardPosition will calculate the forward kinematic position based on current joint positions
// By default joint positions will all be zero
m.ForwardPosition()

// Print out the x,y,z,Rx,Ry,Rz position of the End-Effector (EE) in question (in this case 0)
fmt.Println(m.Get6dPosition(0))

// Get the current position of EE0 as a Transform object, and add that to be a goal for EE0
ik.AddGoal(m.GetOperationalPosition(0), 0)

// Create a new list of joint positions
newPos := []float64{1.1, 0.1, 1.3, 0, 0, -1}
// Set joints to the new positions
m.SetPosition(newPos)
// Calculate the forward kinematic position with the new positions
m.ForwardPosition()
// Print the new 6d position with the new joint angles
fmt.Println(m.Get6dPosition(0))

// Try to solve the joint angles to get us back to the original position that was added to the Goals
// Will set joint angles if successful
yay := ik.Solve()
// Print whether the Solve was successful
fmt.Println("yay? ", yay)
// Calculate forward position
m.ForwardPosition()
// Print the new 6d position- should be very close to the one above
fmt.Println(m.Get6dPosition(0))

// Print the []float64 list of current joint positions in radians
fmt.Println(m.Position())
```
Currently this has only been tested with 6dof robots.

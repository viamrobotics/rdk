# Resetbox Description
This sample is a remote client which coordinates the resetting action of a robot play area (aka resetbox.) The resetbox places two small cubes and a rubber duck on a tippable table, and an arm+gripper in the center is able to "play" with them. When needing to be reset, the table tips up, dumping all objects into a system below the table which sorts and orients them. They are then brought back up via elevator, and the arm places them in their preset starting locations.

## Setup
- gripper.json is the configuration for a Viam v1 gripper in local-only mode (no cloud.)
- resetbox.json is the configuration for the resetbox itself (the "master" robot.)
  - Be sure to edit the file to point to the correct local adddresses for the gripper and the xArm.
Both of these should be run with the stock viam-server (RDK) binary.

After those are running. Execute the client sample in this directory with the address of the resetbox server.
```
go run . resetbox.local:8080
```

If you port these configs to the cloud (app.viam.com) then you can still run this client by obtaining the address and secret from the "Connect" tab for your resetbox, in the RDK Remote Config section, and using the "-secret" arg to set the secret.

Example:
```
go run . -secret lkjadsfoui309ufnanvoiupo2u resetbox.5j56kj29.viam.cloud
```
Be sure to replace the secret and address with the ones for your actual robot.

## Commands
When the client is running on the command prompt, type a letter (and hit enter) to run a few fixed commands.
* h = Home all components. This must be done at least once after startup, before a reset can be run.
* r = Run reset cycle. This is the main operation that clears the table and replaces objects.
* s = Immediate stop of all components.
* q = Quit and exit.
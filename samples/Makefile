PORT = `arduino-cli board list  | grep mega | grep -o "/dev/cu.[a-z0-9]*"`

compile-minirover1: minirover1/*.ino
	arduino-cli compile --fqbn arduino:avr:mega minirover1

deploy-minirover1: compile-minirover1
	arduino-cli upload --port ${PORT} --fqbn arduino:avr:mega minirover1

serial:
	minicom -b 9600 -D ${PORT}

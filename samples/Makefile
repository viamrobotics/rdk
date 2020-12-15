PORT = `arduino-cli board list  | grep avr | grep -o "/dev/cu.[a-z0-9]*"`
ARCH = `arduino-cli board list  | grep avr | grep -o "arduino:avr:[a-z]*"`

foo:
	echo ${PORT} ${ARCH}

compile-minirover1: minirover1/*.ino
	arduino-cli compile --fqbn ${ARCH} minirover1

deploy-minirover1: compile-minirover1
	arduino-cli upload --port ${PORT} --fqbn ${ARCH} minirover1

compile-minirover2: minirover2/*.ino
	arduino-cli compile --fqbn ${ARCH} minirover2

deploy-minirover2: compile-minirover2
	arduino-cli upload --port ${PORT} --fqbn ${ARCH} minirover2


serial:
	minicom -b 9600 -D ${PORT}

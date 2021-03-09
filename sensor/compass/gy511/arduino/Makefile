ifeq ($(PORT),)
PORT = `arduino-cli board list  | grep "arduino:" | grep -o "/dev/[cu|ty]*.[a-zA-Z0-9]*"`
endif
ifeq ($(ARCH),)
ARCH = `arduino-cli board list  | grep "arduino:" | grep -o "arduino:[a-z]*:[a-z_]*"`
endif

COMPILE_FLAGS = compile --libraries ../../../../arduino/ --fqbn ${ARCH} --warnings all

all: upload
.PHONY: all

compile: ./control/control.ino
	arduino-cli ${COMPILE_FLAGS} ./control

upload: compile
	arduino-cli upload --port ${PORT} --fqbn ${ARCH} ./control

serial:
	minicom -b 9600 -D ${PORT}

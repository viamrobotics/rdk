ifeq ($(PORT),)
PORT = `arduino-cli board list  | grep "arduino:" | grep -o "/dev/[cu|ty]*.[a-zA-Z0-9]*"`
endif
ifeq ($(ARCH),)
ARCH = `arduino-cli board list  | grep "arduino:" | grep -o "arduino:[a-z]*:[a-z_]*"`
endif

COMPILE_FLAGS = compile --libraries ../arduino/ --fqbn ${ARCH} --warnings all

foo:
	echo ${PORT} ${ARCH}

compile: vafw/*.ino vafw/*.cpp vafw/*.h
	arduino-cli ${COMPILE_FLAGS} vafw

deploy: compile
	arduino-cli upload --port ${PORT} --fqbn ${ARCH} vafw

serial:
	minicom -b 9600 -D ${PORT}

test-cpp:
	g++ -o ./vafw_cpptest/encoder ./vafw_cpptest/encoder.cpp ./vafw/encoder.cpp
	./vafw_cpptest/encoder

build-go:
	go build

test: test-cpp build-go deploy
	sleep 1 && go test -v


format:
	clang-format -i --style="{BasedOnStyle: Google, IndentWidth: 4}" `find * -iname "*.cpp" -or -iname "*.h" -or -iname "*.ino"`


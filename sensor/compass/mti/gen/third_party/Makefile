TARGETS:= example_mti_receive_data example_mti_parse_logfile
OBJLIBS	= xspublic
INCLUDE=-I. -Ixspublic
CFLAGS=-g $(INCLUDE)
CXXFLAGS=-std=c++11 $(CFLAGS)
LFLAGS=-Lxspublic/xscontroller -Lxspublic/xscommon -Lxspublic/xstypes -lxscontroller -lxscommon -lxstypes -lpthread -ldl

UNAME := $(shell uname)
ifeq ($(UNAME), Darwin)
else
	LFLAGS += -lrt
endif


.PHONY: $(OBJLIBS)
all : $(OBJLIBS) $(TARGETS)

xspublic :
	$(MAKE) -C xspublic $(MFLAGS)

example_mti_receive_data: example_mti_receive_data.cpp.o
example_mti_parse_logfile: example_mti_parse_logfile.cpp.o

$(TARGETS): | $(OBJLIBS)
	$(CXX) $(CFLAGS) $(INCLUDE) $^ -o $@ $(LFLAGS)

%.cpp.o: %.cpp
	$(CXX) -c $(CXXFLAGS) $< -o $@

clean :
	-$(RM) $(OBJECTS) $(TARGETS)
	-$(RM) *.o
	-$(MAKE) -C xspublic/xscontroller $(MFLAGS) clean
	-$(MAKE) -C xspublic/xscommon $(MFLAGS) clean
	-$(MAKE) -C xspublic/xstypes $(MFLAGS) clean

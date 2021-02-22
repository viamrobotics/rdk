TARGET:=libxstypes.a
EXCLUDE:=xstypesdynlib.c
C_FILES := $(filter-out $(EXCLUDE), $(wildcard *.c))
CPP_FILES := $(wildcard *.cpp)
OBJECTS := $(C_FILES:.c=.c.o) $(CPP_FILES:.cpp=.cpp.o)
CFLAGS+= -fPIC -I$(shell dirname $$(pwd))/xspublic
CXXFLAGS+= -std=c++11 -I$(shell dirname $$(pwd))/xspublic

.PHONY: all xstypes
all: xstypes
xstypes: $(TARGET)

-include $(FILES:.c=.d)

%.c.o: %.c
	$(CC) -c $(CFLAGS) $< -o $@
	@$(CC) -MM $(CFLAGS) $< > $*.d
	@mv -f $*.d $*.d.tmp
	@sed -e 's|.*:|$*.c.o:|' < $*.d.tmp > $*.d
	@sed -e 's/.*://' -e 's/\\$$//' < $*.d.tmp | fmt -1 | \
	  sed -e 's/^ *//' -e 's/$$/:/' >> $*.d
	@rm -f $*.d.tmp

-include $(FILES:.cpp=.dpp)

%.cpp.o: %.cpp
	$(CXX) -c $(CXXFLAGS) $< -o $@
	@$(CXX) -MM $(CXXFLAGS) $< > $*.dpp
	@mv -f $*.dpp $*.dpp.tmp
	@sed -e 's|.*:|$*.c.o:|' < $*.dpp.tmp > $*.dpp
	@sed -e 's/.*://' -e 's/\\$$//' < $*.dpp.tmp | fmt -1 | \
	  sed -e 's/^ *//' -e 's/$$/:/' >> $*.dpp
	@rm -f $*.dpp.tmp

$(TARGET): $(OBJECTS)
	ar rcs $@ $^

clean:
	-$(RM) buildversion.h
	-$(RM) *.o *.d *.dpp
	-$(RM) $(TARGET)

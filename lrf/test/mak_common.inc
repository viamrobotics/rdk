#/*
# * Copyright (C) 2014  RoboPeak
# *
# * This program is free software: you can redistribute it and/or modify
# * it under the terms of the GNU General Public License as published by
# * the Free Software Foundation, either version 3 of the License, or
# * (at your option) any later version.
# *
# * This program is distributed in the hope that it will be useful,
# * but WITHOUT ANY WARRANTY; without even the implied warranty of
# * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# * GNU General Public License for more details.
# *
# * You should have received a copy of the GNU General Public License
# * along with this program.  If not, see <http://www.gnu.org/licenses/>.
# *
# */
#
#  Common Generation Law for rplidar sdk Building
#
#
#

.PHONY: make_subs build_sdk build_app clean_sdk clean_app

make_subs:
	@for subdir in $(MAKE_TARGETS) ; do  $(MAKE) -C $$subdir $(MAKECMDGOALS) || exit 1;  done

# Define all object files.

_CSRC   = $(filter-out %*.c,$(CSRC))
_CXXSRC = $(filter-out %*.cpp,$(CXXSRC))
_ASRC   = $(filter-out %*.S,$(ASRC))

OBJ += $(patsubst %.c, $(TARGET_OBJ_ROOT)/%.o, $(_CSRC))
OBJ += $(patsubst %.cpp, $(TARGET_OBJ_ROOT)/%.o, $(_CXXSRC))
OBJ += $(patsubst %.S, $(TARGET_OBJ_ROOT)/%.o, $(_ASRC))

build_sdk:  $(SDK_TARGET)

clean_sdk:
	$(RMDIR) $(TARGET_OBJ_ROOT)
	$(RM) $(SDK_TARGET)

build_app: $(APP_TARGET)

clean_app:
	$(RMDIR) $(TARGET_OBJ_ROOT)
	$(RM) $(APP_TARGET)

$(SDK_TARGET): $(OBJ) $(EXTRA_OBJ)
	$(MKDIR) `dirname $@`
	@for i in $^; do echo " pack `basename $$i`->`basename $@`"; $(AR) rcs $@ $$i; done
	
$(APP_TARGET): $(OBJ) $(EXTRA_OBJ) $(SDK_TARGET)
	@$(MKDIR) `dirname $@`
	@echo " LD   $@"
	$(CMD_HIDE)$(CC) $(ALL_CXXFLAGS) -o $@ $^ $(LDFLAGS)  


#
# Compile: create object files from C++ source files.
$(TARGET_OBJ_ROOT)/%.o: %.cpp
	@$(MKDIR) `dirname $@`
	@echo " CXX  $<"
	$(CMD_HIDE)$(CXX) -c $(ALL_CXXFLAGS) $< -o $@ 

# Compile: create object files from C source files.
$(TARGET_OBJ_ROOT)/%.o: %.c
	@$(MKDIR) `dirname $@`
	@echo " CC   $<"
	$(CMD_HIDE)$(CC) -c $(ALL_CFLAGS) $< -o $@ 

# Assemble: create object files from assembler source files.
$(TARGET_OBJ_ROOT)/%.o: %.S
	@$(MKDIR) `dirname $@`
	@echo " AS   $<"
	$(CMD_HIDE)$(CC) -c $(ALL_ASFLAGS) $< -o $@


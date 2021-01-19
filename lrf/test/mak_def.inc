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
#  Common Definition for RoboPeak Building
#
#
#

HOME_TREE:=$(realpath $(HOME_TREE))

BUILD_ROOT  = $(HOME_TREE)

#valid platforms are:
#Linux Darwin

#valid architectures are:
#x86

BUILD_HOST_PLATFORM := $(shell uname -s)
BUILD_HOST_ARCH := $(shell uname -m)

ifeq ($(BUILD_HOST_ARCH),x86_64)
BUILD_HOST_ARCH:=x64
CFLAGS +=
CXXFLAGS +=
endif

BUILD_TARGET_ARCH?=$(BUILD_HOST_ARCH)
BUILD_TARGET_PLATFORM?=$(BUILD_HOST_PLATFORM)

RM = rm -f
MV = mv -f
MKDIR = mkdir -p
RMDIR = rm -rf

ifeq ($(V),1)
CMD_HIDE:=
else
CMD_HIDE:=@
endif

ifeq ($(CMD_HIDE),@)
CMD_HIDE_CC="@echo   CC   $^; @"
CMD_HIDE_CXX="@echo  CXX  $^; @"
CMD_HIDE_LD="@echo   LD   $@; @"
endif

ifdef DEBUG
OUTPUT_BUILD_PREFIX =Debug
else
OUTPUT_BUILD_PREFIX =Release
endif

BUILD_OUTPUT_ROOT = $(BUILD_ROOT)/output/$(BUILD_TARGET_PLATFORM)/$(OUTPUT_BUILD_PREFIX)
BUILD_OBJ_ROOT    = $(BUILD_ROOT)/obj/$(BUILD_TARGET_PLATFORM)/$(OUTPUT_BUILD_PREFIX)

SDK_LIB_CORENAME := rplidar_sdk
SDK_STATIC_MODULE:=lib$(SDK_LIB_CORENAME).a
TARGET_OBJ_ROOT = $(BUILD_OBJ_ROOT)/$(MODULE_NAME)
SDK_TARGET = $(BUILD_OUTPUT_ROOT)/$(SDK_STATIC_MODULE)
APP_TARGET=$(BUILD_OUTPUT_ROOT)/$(MODULE_NAME)

# Debugging format.
DEBUG_TYPE = gdb

#Optimize Level
OPT = 2 

# Place -D or -U options here
ifeq ($(BUILD_TARGET_ARCH),x86)
CDEFS += -D_CPU_ENDIAN_SMALL
endif

ifeq ($(BUILD_TARGET_PLATFORM),Darwin)
CDEFS += -D_MACOS
else
LD_LIBS += -lrt
endif


CDEFS += $(EXTRA_DEFS)

CXXDEFS +=
CXXDEFS += $(CDEFS) $(EXTRA_DEFS)

C_INCLUDES += -I.
CXX_INCLUDES += $(C_INCLUDES)

# Compiler flag to set the C Standard level.
# c89   - "ANSI" C
# gnu89 - c89 plus GCC extensions
# c99   - ISO C99 standard (not yet fully implemented)
# gnu99 - c99 plus GCC extensions
CSTANDARD = -std=gnu99
CDEBUG = -g$(DEBUG_TYPE)
CWARN = -Wall 
CTUNING = -funsigned-char 

# force everything to become PIC, unless explicitly claimed
ifndef BUILD_APP_NO_PIC_CODE
CTUNING += -fPIC
endif

ifdef DEBUG
OPT_FLAG = $(CDEBUG)
CDEFS += -D_DEBUG -DDEBUG
else
OPT_FLAG = -O$(OPT)
CDEFS += -DNDEBUG -D_NDEBUG
endif

CFLAGS += $(OPT_FLAG) $(CDEFS) $(C_INCLUDES) $(CWARN) $(CSTANDARD) $(CEXTRA) $(CTUNING) -Wstrict-prototypes

CXXFLAGS += $(OPT_FLAG) $(CXXDEFS) $(CXX_INCLUDES) $(CWARN) $(CEXTRA) $(CXXEXTRA) $(CTUNING)

ASFLAGS += -Wa,-adhlns=$(<:.S=.lst),-gstabs $(CDEFS) $(C_INCLUDES)
LDFLAGS += $(LD_LIBS)

ALL_CFLAGS = $(CFLAGS)
ALL_CXXFLAGS = $(CXXFLAGS)
ALL_ASFLAGS = $(ASFLAGS)


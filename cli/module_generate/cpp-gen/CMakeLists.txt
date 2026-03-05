cmake_minimum_required(VERSION 3.25 FATAL_ERROR)

project(viam-cpp-module-gen
    VERSION 0.0.1
    LANGUAGES C CXX
)

set(CMAKE_MACOSX_RPATH 1)
set(CMAKE_INSTALL_RPATH_USE_LINK_PATH TRUE)

# Set RPATH for installed binaries to find libraries in the same install directory
if(APPLE)
    # On macOS, use @executable_path to find libraries relative to the executable
    set(CMAKE_INSTALL_RPATH "@executable_path/../lib;@executable_path")
elseif(UNIX)
    # On Linux, use $ORIGIN
    set(CMAKE_INSTALL_RPATH "$ORIGIN/../lib:$ORIGIN")
endif()

if (LLVM_ROOT)
    # LLVM_ROOT is absolute
    get_filename_component(LLVM_ROOT "${LLVM_ROOT}" ABSOLUTE)
    set(LLVM_ROOT "${LLVM_ROOT}" CACHE PATH "Root of LLVM install." FORCE)
    if (NOT EXISTS "${LLVM_ROOT}")
        message(FATAL_ERROR
               "LLVM_ROOT (${LLVM_ROOT}) provided does not exist.\n"
                "Please set LLVM_ROOT to the root of LLVM install.\n")
    endif()
    if (NOT EXISTS "${LLVM_ROOT}/lib/cmake/llvm")
        message(FATAL_ERROR
                "LLVM_ROOT (${LLVM_ROOT}) provided is invalid.\n"
                "No <LLVM_ROOT>/lib/cmake/llvm found.\n"
                "Please set LLVM_ROOT to the root of LLVM install.\n")
    endif()
    message(STATUS "LLVM_ROOT: ${LLVM_ROOT}")
endif()

if (Clang_ROOT)
    # Clang_ROOT is absolute
    get_filename_component(Clang_ROOT "${Clang_ROOT}" ABSOLUTE)
    set(LLVM_ROOT "${LLVM_ROOT}" CACHE PATH "Root of Clang install." FORCE)
elseif (LLVM_ROOT)
    # Clang_ROOT matches LLVM_ROOT by default
    set(Clang_ROOT "${LLVM_ROOT}" CACHE PATH "Root of Clang install." FORCE)
endif()

find_package(LLVM REQUIRED CONFIG)
find_package(Clang REQUIRED CONFIG)

list(APPEND CMAKE_MODULE_PATH "${LLVM_CMAKE_DIR}")
include(HandleLLVMOptions)
add_definitions(${LLVM_DEFINITIONS})

# ClangConfig.cmake does not specify a version so we manually check below for the only supported version
set(_clang_version_header_path "${CLANG_INCLUDE_DIRS}/clang/Basic/Version.inc")

if(NOT EXISTS ${_clang_version_header_path})
    message(FATAL_ERROR "Could not find ${_clang_version_header_path} at ${CLANG_INCLUDE_DIRS}")
endif()

file(READ ${_clang_version_header_path}  _clang_version_header_contents)
string(FIND "${_clang_version_header_contents}" "#define CLANG_VERSION_MAJOR 15" _version_match)

if (${_version_match} EQUAL -1)
    message(FATAL_ERROR "Incorrect version or no version found in clang version header ${_clang_version_header_path}
    ${_clang_version_header_contents}
    required version is Clang 15")
endif()

message(STATUS "Clang include ${CLANG_INCLUDE_DIRS}")

add_library(generator SHARED
    src/viam/generator/compilation_db.cpp
    src/viam/generator/compiler_info.cpp
    src/viam/generator/generator.cpp
)

target_include_directories(generator
    SYSTEM PUBLIC ${LLVM_INCLUDE_DIRS}
    SYSTEM PUBLIC ${CLANG_INCLUDE_DIRS}
    PUBLIC src/
)

target_link_libraries(generator PUBLIC
    clangAST
    clangASTMatchers
    clangBasic
    clangFrontend
    clangIndex
    clangSerialization
    clangTooling
    clangToolingCore
    clangToolingInclusions
    LLVMSupport
)

target_compile_features(generator PUBLIC cxx_std_17)
target_compile_options(generator PUBLIC "-fno-rtti")

add_executable(cl_gen
    src/viam/generator/main.cpp
)

target_link_libraries(cl_gen PRIVATE generator)

install(
    TARGETS generator cl_gen
)

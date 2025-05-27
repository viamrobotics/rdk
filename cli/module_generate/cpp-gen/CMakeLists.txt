cmake_minimum_required(VERSION 3.25 FATAL_ERROR)

project(viam-cpp-module-gen
    VERSION 0.0.1
    LANGUAGES C CXX
)

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

add_library(generator
    compilation_db.cpp
    compiler_info.cpp
    generator.cpp
)

target_include_directories(generator
    SYSTEM PUBLIC ${LLVM_INCLUDE_DIRS}
    SYSTEM PUBLIC ${CLANG_INCLUDE_DIRS}
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

add_library(generator_api
    gen_api.cpp
)

target_link_libraries(generator_api PRIVATE generator)

add_executable(cl_gen
    main.cpp
)

target_link_libraries(cl_gen PRIVATE generator)

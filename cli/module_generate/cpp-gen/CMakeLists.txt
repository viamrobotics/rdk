cmake_minimum_required(VERSION 3.25 FATAL_ERROR)

project(viam-cpp-module-gen
    VERSION 0.0.1
    LANGUAGES C CXX
)

find_package(LLVM REQUIRED CONFIG)
find_package(Clang REQUIRED CONFIG)

list(APPEND CMAKE_MODULE_PATH "${LLVM_CMAKE_DIR}")
include(HandleLLVMOptions)
add_definitions(${LLVM_DEFINITIONS})

add_executable(generator
    main.cpp
    compilation_db.cpp
    compiler_info.cpp
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

target_compile_features(generator PRIVATE cxx_std_17)
target_compile_options(generator PRIVATE "-fno-rtti")

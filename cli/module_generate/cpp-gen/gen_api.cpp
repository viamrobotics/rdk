#include "gen_api.h"

#include <exception>
#include <iostream>

#include <llvm/Support/FileSystem.h>

#include "generator.hpp"

extern "C" {

int viam_cli_generate_cpp_module(const char* className,
                                 const char* componentName,
                                 const char* buildDir,
                                 const char* sourceDir,
                                 const char* outPath) try {
    auto tmpFile = llvm::sys::fs::TempFile::create("viam-cli-cpp-tmp-%%%%%%");

    if (!tmpFile) {
        std::cerr << "failed to create temp file: "
                  << llvm::errorToErrorCode(tmpFile.takeError()).message() << "\n";
        return 1;
    }

    std::cerr << "Created temp file " << tmpFile->TmpName << "\n";

    std::error_code ec;

    llvm::raw_fd_ostream outStream(tmpFile->TmpName, ec);

    if (ec != std::error_code{}) {
        std::cerr << "ostream failed with " << ec.message() << "\n";
        return 1;
    }

    auto gen =
        viam::gen::Generator::create(className, componentName, buildDir, sourceDir, outStream);

    if (gen.run() != 0) {
        std::cerr << "Generator::run failed\n";
        return 1;
    }

    std::cerr << "Ran generator\n";

    llvm::Error err = tmpFile->keep(outPath);

    if (err) {
        std::cerr << "failed to keep module output file "
                  << llvm::errorToErrorCode(std::move(err)).message() << "\n";
        return 1;
    }

    std::cerr << "kept temp file\n";

    return 0;
} catch (const std::exception& e) {
    std::cerr << "module generate failed with exception " << e.what() << "\n";
    return 1;
}
}

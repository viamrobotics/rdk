#include <viam/generator/generator.hpp>

#include <clang/Tooling/CommonOptionsParser.h>
#include <llvm/Support/CommandLine.h>

#include <iostream>

using namespace viam::gen;

namespace cl = llvm::cl;

static cl::OptionCategory opts("module-gen options");

static cl::OptionCategory quickExit("module-gen quick exit options");

static cl::opt<std::string> BuildPath("p", cl::desc("Build path"), cl::Optional, cl::cat(opts));

static cl::opt<std::string> SourcePath(cl::Positional,
                                       cl::desc("<source>"),
                                       cl::Optional,
                                       cl::cat(opts));

static llvm::cl::opt<std::string> outfile("o",
                                          llvm::cl::init("-"),
                                          llvm::cl::desc("Output file, default stdout"),
                                          llvm::cl::cat(opts));

static llvm::cl::opt<bool> justMain("main",
                                    llvm::cl::desc("If true, output the stub main file and exit"),
                                    llvm::cl::cat(quickExit));

static llvm::cl::opt<bool> justCMake(
    "cmake",
    llvm::cl::desc("If true, output the template CMakeLists.txt and exit"),
    llvm::cl::cat(quickExit));

int main(int argc, const char** argv) try {
    cl::ParseCommandLineOptions(argc, argv);

    std::error_code ec;
    llvm::raw_fd_ostream out(outfile, ec, llvm::sys::fs::CD_CreateAlways);

    if (ec != std::error_code{}) {
        throw std::system_error(ec);
    }

    if (justMain) {
        Generator::main_fn(out);
        return 0;
    }

    if (justCMake) {
        Generator::cmakelists(out);
        return 0;
    }

    if (BuildPath.empty()) {
        llvm::errs() << "A build path argument is mandatory when not using a quick-exit option\n";
        cl::PrintHelpMessage();
        return 1;
    }

    if (SourcePath.empty()) {
        llvm::errs() << "A source path is mandatory when not using a quick-exit option\n";
        cl::PrintHelpMessage();
        return 1;
    }

    std::string err;
    auto compilations =
        clang::tooling::CompilationDatabase::autoDetectFromDirectory(BuildPath, err);

    if (!compilations) {
        llvm::errs() << "Error while trying to load compilation database:\n" << err << "\n";
        return 1;
    }

    auto gen = Generator::createFromCommandLine(*compilations, SourcePath, out);

    return gen.run();
} catch (const std::exception& e) {
    std::cerr << "Generator failed with exception: " << e.what() << "\n";
    return 1;
}

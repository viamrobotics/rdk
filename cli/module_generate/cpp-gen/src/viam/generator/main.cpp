#include <viam/generator/generator.hpp>

#include <clang/Tooling/CommonOptionsParser.h>
#include <llvm/Support/CommandLine.h>

#include <iostream>

using namespace viam::gen;

static llvm::cl::OptionCategory opts("module-gen options");

static llvm::cl::opt<bool> justMain("main",
                                    llvm::cl::desc("If true, output the stub main file and exit"),
                                    llvm::cl::cat(opts));

static llvm::cl::opt<bool> justCMake(
    "cmake",
    llvm::cl::desc("If true, output the template CMakeLists.txt and exit"),
    llvm::cl::cat(opts));

int main(int argc, const char** argv) try {
    // CommonOptionsParser::create will set up a compilation DB, so first let's check for the
    // quick exit options
    llvm::cl::ParseCommandLineOptions(argc, argv);

    if (justMain) {
        Generator::main_fn(llvm::outs());
        return 0;
    }

    if (justCMake) {
        Generator::cmakelists(llvm::outs());
        return 0;
    }

    auto ExpectedParser = clang::tooling::CommonOptionsParser::create(argc, argv, opts);

    if (!ExpectedParser) {
        // Fail gracefully for unsupported options.
        llvm::errs() << ExpectedParser.takeError();
        return 1;
    }
    clang::tooling::CommonOptionsParser& OptionsParser = ExpectedParser.get();

    const auto& sources = OptionsParser.getSourcePathList();
    if (sources.size() != 1) {
        llvm::errs() << "Specified more than one source\n";
        return 1;
    }

    auto gen = Generator::createFromCommandLine(
        OptionsParser.getCompilations(), sources.front(), llvm::outs());

    return gen.run();
} catch (const std::exception& e) {
    std::cerr << e.what() << "\n";
    return 1;
}

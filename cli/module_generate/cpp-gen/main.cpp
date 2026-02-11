#include "generator.hpp"

#include <clang/Tooling/CommonOptionsParser.h>
#include <llvm/Support/CommandLine.h>

#include <iostream>

using namespace viam::gen;

static llvm::cl::OptionCategory opts("module-gen options");

static llvm::cl::extrahelp moreHelp("Viam C++ SDK module generator");

int main(int argc, const char** argv) try {
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

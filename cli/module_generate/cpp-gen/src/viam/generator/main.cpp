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

static llvm::cl::opt<std::string> outdir(
    "d",
    llvm::cl::init("-"),
    llvm::cl::desc("Output directory; use '-' to print to stdout"),
    llvm::cl::cat(opts));

static llvm::cl::opt<bool> justMain(
    "main",
    llvm::cl::desc("If true, output the template main.cpp.in and exit"),
    llvm::cl::cat(quickExit));

static llvm::cl::opt<bool> justCMake(
    "cmake",
    llvm::cl::desc("If true, output the template CMakeLists.txt.in and exit"),
    llvm::cl::cat(quickExit));

static llvm::cl::opt<bool> justConan(
    "conan",
    llvm::cl::desc("If true, output the template conanfile.py.in and exit"),
    llvm::cl::cat(quickExit));

static llvm::cl::extrahelp extra(R"--(
OUTPUT DIRECTORY HELP:

The directory option (-d) is used to automatically deduced output file names to be created
in a directory prefix. Given -d generator/output/directory, the output file will be
deduced from either the quick exit options, or the appropriate directory and filenames
from the source file input. For example, /path/to/components/arm.cpp will generate an
arm.hpp.in and arm.cpp.in in the output directory. This implies in particular that the
output directory must already contain components/ and services/ subdirectories.

If -d is not provided, or set to "-", stdout is used for all files.

)--");

int main(int argc, const char** argv) try {
    cl::ParseCommandLineOptions(argc, argv);

    auto make_out = [&](llvm::StringRef filename) -> std::unique_ptr<llvm::raw_fd_ostream> {
        std::string path = outdir;

        if (path != "-") {
            path += "/" + filename.str();
        }

        std::error_code ec;

        auto os = std::make_unique<llvm::raw_fd_ostream>(path, ec, llvm::sys::fs::CD_CreateAlways);
        if (ec != std::error_code{}) {
            throw std::runtime_error("Error " + ec.message() + " opening file " + path);
        }
        return os;
    };

    if (justMain) {
        auto os = make_out("main.cpp.in");
        Generator::main_fn(*os);
        return 0;
    }

    if (justCMake) {
        auto os = make_out("CMakeLists.txt.in");
        Generator::cmakelists(*os);
        return 0;
    }

    if (justConan) {
        auto os = make_out("conanfile.py.in");
        Generator::conanfile(*os);
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

    // Validate source path has a terminal .cpp file and a parent directory component
    // (e.g. components/arm.cpp or services/motion.cpp)
    {
        const std::string& src = SourcePath.getValue();
        auto it = llvm::sys::path::rbegin(src);
        const auto rend = llvm::sys::path::rend(src);
        if (it == rend || !llvm::StringRef(*it).endswith(".cpp")) {
            llvm::errs() << "Source path must end in a .cpp file "
                            "(e.g. components/arm.cpp)\n";
            return 1;
        }
        ++it;
        if (it == rend) {
            llvm::errs() << "Source path must include a parent directory component "
                            "(e.g. components/arm.cpp)\n";
            return 1;
        }
    }

    std::string err;
    auto compilations =
        clang::tooling::CompilationDatabase::autoDetectFromDirectory(BuildPath, err);

    if (!compilations) {
        llvm::errs() << "Error while trying to load compilation database:\n" << err << "\n";
        return 1;
    }

    // Build an output stream whose filename is derived from the trailing two
    // components of SourcePath (e.g. components/arm) with the given extension
    // and a .in suffix (e.g. outdir/components/arm.hpp.in).
    auto make_gen_out = [&](llvm::StringRef ext) -> std::unique_ptr<llvm::raw_fd_ostream> {
        const std::string& src = SourcePath.getValue();
        auto it = llvm::sys::path::rbegin(src);
        llvm::StringRef stem = llvm::sys::path::stem(*it);
        ++it;
        llvm::StringRef parentDir = *it;
        return make_out((llvm::Twine(parentDir) + "/" + stem + "." + ext + ".in").str());
    };

    auto headerOut = make_gen_out("hpp");
    auto srcOut = make_gen_out("cpp");
    auto gen = Generator::createFromCommandLine(
        *compilations, SourcePath, std::move(headerOut), std::move(srcOut));

    gen.run();

    return 0;
} catch (const std::exception& e) {
    std::cerr << "Generator failed with exception: " << e.what() << "\n";
    return 1;
}

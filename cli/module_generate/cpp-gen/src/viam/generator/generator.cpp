#include <viam/generator/generator.hpp>

#include <viam/generator/compilation_db.hpp>
#include <viam/generator/compiler_info.hpp>
#include <viam/generator/template_constants.hpp>

#include <clang/AST/PrettyPrinter.h>
#include <clang/AST/QualTypeNames.h>
#include <clang/ASTMatchers/ASTMatchFinder.h>
#include <clang/ASTMatchers/ASTMatchers.h>
#include <clang/Frontend/FrontendActions.h>
#include <clang/Tooling/JSONCompilationDatabase.h>
#include <llvm/ADT/SmallVector.h>
#include <llvm/ADT/StringExtras.h>
#include <llvm/ADT/StringMap.h>
#include <llvm/Support/FormatVariadic.h>
#include <llvm/Support/Path.h>
#include <llvm/Support/raw_ostream.h>

#include <string_view>
#include <unordered_map>

namespace viam::gen {

Generator Generator::create(Generator::ModuleInfo moduleInfo,
                            Generator::CppTreeInfo cppInfo,
                            std::unique_ptr<llvm::raw_fd_ostream> headerOut,
                            std::unique_ptr<llvm::raw_fd_ostream> srcOut) {
    std::string error;
    auto jsonDb =
        clang::tooling::JSONCompilationDatabase::autoDetectFromDirectory(cppInfo.buildDir, error);
    if (!jsonDb) {
        throw std::runtime_error(error);
    }

    return Generator(
        GeneratorCompDB(*jsonDb, getCompilersDefaultIncludeDir(*jsonDb, true)),
        moduleInfo.resourceType,
        moduleInfo.resourceSubtypeSnake.str(),
        (cppInfo.sourceDir +
         resourceToSource(moduleInfo.resourceSubtypeSnake, moduleInfo.resourceType, SrcType::cpp))
            .str(),
        std::move(headerOut),
        std::move(srcOut));
}

Generator Generator::createFromCommandLine(const clang::tooling::CompilationDatabase& db,
                                           llvm::StringRef sourceFile,
                                           std::unique_ptr<llvm::raw_fd_ostream> headerOut,
                                           std::unique_ptr<llvm::raw_fd_ostream> srcOut) {
    return Generator(GeneratorCompDB(db, getCompilersDefaultIncludeDir(db, true)),
                     to_resource_type((*++llvm::sys::path::rbegin(sourceFile)).drop_back()),
                     llvm::sys::path::stem(sourceFile).str(),
                     sourceFile.str(),
                     std::move(headerOut),
                     std::move(srcOut));
}

Generator::ResourceType Generator::to_resource_type(llvm::StringRef resourceType) {
    if (resourceType == "component") {
        return ResourceType::component;
    }

    if (resourceType == "service") {
        return ResourceType::service;
    }

    throw std::runtime_error(("Invalid resource type" + resourceType).str());
}

Generator::Generator(GeneratorCompDB db,
                     ResourceType resourceType,
                     std::string resourceSubtypeSnake,
                     std::string resourcePath,
                     std::unique_ptr<llvm::raw_fd_ostream> headerOut,
                     std::unique_ptr<llvm::raw_fd_ostream> srcOut)
    : db_(std::move(db)),
      resourceType_(resourceType),
      resourceSubtypeSnake_(std::move(resourceSubtypeSnake)),
      resourceSubtypePascal_(llvm::convertToCamelFromSnakeCase(resourceSubtypeSnake_, true)),
      resourcePath_(std::move(resourcePath)),
      headerOut_(std::move(headerOut)),
      srcOut_(std::move(srcOut)) {
    if (llvm::StringRef(resourceSubtypeSnake_).startswith("generic_")) {
        resourceSubtypeSnake_ = "generic";
    }
}

void Generator::header_prefix() {
    include_stmts();

    *headerOut_ << llvm::formatv("namespace {0} {\n\n", fmt_str::moduleName);

    const char* fmt =
        R"--(
class {0} : public viam::sdk::{1}, public viam::sdk::Reconfigurable {{
public:
    {0}(const viam::sdk::Dependencies& deps, const viam::sdk::ResourceConfig& cfg);

    static std::vector<std::string> validate(const viam::sdk::ResourceConfig& cfg);

    void reconfigure(const viam::sdk::Dependencies& deps, const viam::sdk::ResourceConfig& cfg) override;

)--";

    *headerOut_ << llvm::formatv(fmt, fmt_str::modelPascal, resourceSubtypePascal_);
}

void Generator::src_prefix() {
    *srcOut_ << llvm::formatv(R"--(
#include "{0}.hpp"

#include <stdexcept>

namespace {1} {

)--",
                              fmt_str::modelSnake,
                              fmt_str::moduleName);

    const char* ctorFmt = R"--(
{0}::{0}(const viam::sdk::Dependencies& deps, const viam::sdk::ResourceConfig& cfg)
    : {1}(cfg.name()) {{
    this->reconfigure(deps, cfg);
}

)--";
    *srcOut_ << llvm::formatv(ctorFmt, fmt_str::modelPascal, resourceSubtypePascal_)
             << llvm::formatv(R"--(
std::vector<std::string> {0}::validate(const viam::sdk::ResourceConfig& cfg) {{
    throw std::runtime_error("\"validate\" not implemented");
}

void {0}::reconfigure(const viam::sdk::Dependencies& deps, const viam::sdk::ResourceConfig& cfg) {{
    throw std::runtime_error("\"reconfigure\" not implemented");
}

)--",
                              fmt_str::modelPascal);
}

template <>
const char* Generator::include_fmt<Generator::ResourceType::component>() {
    constexpr const char* fmt = R"--(
#include <viam/sdk/common/proto_value.hpp>
#include <viam/sdk/{0}>
#include <viam/sdk/config/resource.hpp>
#include <viam/sdk/module/service.hpp>
#include <viam/sdk/resource/reconfigurable.hpp>

)--";

    return fmt;
}

template <>
const char* Generator::include_fmt<Generator::ResourceType::service>() {
    constexpr const char* fmt = R"--(
#include <viam/sdk/common/proto_value.hpp>
#include <viam/sdk/config/resource.hpp>
#include <viam/sdk/module/service.hpp>
#include <viam/sdk/resource/reconfigurable.hpp>
#include <viam/sdk/{0}>

)--";

    return fmt;
}

void Generator::include_stmts() {
    *headerOut_ << "#pragma once\n\n";

    const char* fmt = (resourceType_ == ResourceType::component)
                          ? include_fmt<ResourceType::component>()
                          : include_fmt<ResourceType::service>();

    *headerOut_ << llvm::formatv(
        fmt, resourceToSource(resourceSubtypeSnake_, resourceType_, SrcType::hpp));
}

void Generator::do_stubs() {
    clang::tooling::ClangTool tool(db_, resourcePath_);

    using namespace clang::ast_matchers;

    std::string qualName = ("viam::sdk::" + resourceSubtypePascal_);

    DeclarationMatcher methodMatcher =
        cxxMethodDecl(isPure(), hasParent(cxxRecordDecl(hasName(qualName)))).bind("method");

    DeclarationMatcher stoppableMatcher =
        cxxRecordDecl(hasName(qualName), isDerivedFrom("viam::sdk::Stoppable")).bind("stoppable");

    struct MethodPrinter : MatchFinder::MatchCallback {
        MethodPrinter(llvm::raw_ostream& headerOut_, llvm::raw_ostream& srcOut_)
            : headerOut(headerOut_), srcOut(srcOut_) {}

        llvm::raw_ostream& headerOut;
        llvm::raw_ostream& srcOut;

        void printParm(llvm::raw_ostream& os, const clang::ParmVarDecl& parm) {
            os << clang::TypeName::getFullyQualifiedName(
                      parm.getType(), parm.getASTContext(), {parm.getASTContext().getLangOpts()})
               << " " << parm.getName();
        }

        void printParams(llvm::raw_ostream& os, const clang::CXXMethodDecl* method) {
            const auto paramCount = method->getNumParams();

            auto printParamBreak = [paramCount, &os] {
                if (paramCount > 1) {
                    os << "\n        ";
                }
            };

            if (paramCount > 0) {
                auto param_begin = method->param_begin();

                printParamBreak();
                printParm(os, **param_begin);

                if (paramCount > 1) {
                    for (const clang::ParmVarDecl* parm :
                         llvm::makeArrayRef(++param_begin, method->param_end())) {
                        os << ",";
                        printParamBreak();
                        printParm(os, *parm);
                    }
                }
            }
        }

        void do_header_declaration(const clang::CXXMethodDecl* method,
                                   const clang::PrintingPolicy& printPolicy,
                                   const std::string& retType) {
            headerOut << "    " << retType << (retType.size() < 70 ? " " : "\n    ")
                      << method->getName() << "(";
            printParams(headerOut, method);
            headerOut << ")";
            method->getMethodQualifiers().print(headerOut, printPolicy, false);
            headerOut << " override;\n\n";
        }

        void do_src_definition(const clang::CXXMethodDecl* method,
                               const clang::PrintingPolicy& printPolicy,
                               const std::string& retType) {
            srcOut << retType << (retType.size() < 70 ? " " : "\n") << fmt_str::modelPascal
                   << "::" << method->getName() << "(";
            printParams(srcOut, method);
            srcOut << ")";
            method->getMethodQualifiers().print(srcOut, printPolicy, false);
            srcOut << llvm::formatv(R"--(
{
    throw std::logic_error("\"{0}\" not implemented");
}

)--",
                                    method->getName());
        }

        void run(const MatchFinder::MatchResult& result) override {
            if (const auto* method = result.Nodes.getNodeAs<clang::CXXMethodDecl>("method")) {
                clang::PrintingPolicy printPolicy(method->getASTContext().getLangOpts());
                printPolicy.FullyQualifiedName = 1;

                const std::string& retType = clang::TypeName::getFullyQualifiedName(
                    method->getReturnType(), method->getASTContext(), printPolicy);

                do_header_declaration(method, printPolicy, retType);
                do_src_definition(method, printPolicy, retType);

                return;
            }

            if (result.Nodes.getNodeAs<clang::CXXRecordDecl>("stoppable")) {
                // it's friday afternoon and i am writing this out manually rather than with my
                // method printing helpers

                headerOut << "    void stop(const viam::sdk::ProtoStruct & extra) override;\n\n";

                srcOut << "void " << fmt_str::modelPascal
                       << "::stop(const viam::sdk::ProtoStruct & extra)\n{\n"
                       << R"--(    throw std::logic_error("\"stop\" not implemented");)--"
                       << "\n}\n\n";
            }
        }
    };

    MethodPrinter printer(*headerOut_, *srcOut_);
    MatchFinder finder;

    finder.addMatcher(stoppableMatcher, &printer);
    finder.addMatcher(methodMatcher, &printer);

    if (int result = tool.run(clang::tooling::newFrontendActionFactory(&finder).get())) {
        throw std::runtime_error("Nonzero return from stub generation: " + std::to_string(result));
    }
}

void Generator::main_fn(llvm::raw_ostream& moduleFile) {
    moduleFile <<

        llvm::formatv(R"--(
#include "{0}.hpp"

#include <iostream>
#include <memory>
#include <vector>

#include <viam/sdk/common/exception.hpp>
#include <viam/sdk/common/instance.hpp>
#include <viam/sdk/log/logging.hpp>
#include <viam/sdk/registry/registry.hpp>


)--",
                      fmt_str::modelSnake)
               << "int main(int argc, char** argv) try {\n"
               << llvm::formatv(R"--(
    // Every Viam C++ SDK program must have one and only one Instance object which is created before
    // any other SDK objects and stays alive until all of them are destroyed.
    viam::sdk::Instance inst;

    // Write general log statements using the VIAM_SDK_LOG macro.
    VIAM_SDK_LOG(info) << "Starting up {1} module";

    viam::sdk::Model model("{0}", "{1}", "{2}");)--",
                                fmt_str::orgID,
                                fmt_str::moduleName,
                                fmt_str::modelSnake)
               << "\n\n"
               << llvm::formatv(
                      R"--(
    auto mr = std::make_shared<viam::sdk::ModelRegistration>(
        viam::sdk::API::get<viam::sdk::{0}>(),
        model,
        [](viam::sdk::Dependencies deps, viam::sdk::ResourceConfig cfg) {
            return std::make_unique<{1}>(deps, cfg);
        },
        &{1}::validate);
)--",
                      fmt_str::resourceSubtypePascal,
                      fmt_str::moduleName + llvm::Twine("::") + fmt_str::modelPascal)
               << "\n\n"
               <<
        R"--(
    std::vector<std::shared_ptr<viam::sdk::ModelRegistration>> mrs = {mr};
    auto my_mod = std::make_shared<viam::sdk::ModuleService>(argc, argv, mrs);
    my_mod->serve();

    return EXIT_SUCCESS;
} catch (const viam::sdk::Exception& ex) {
    std::cerr << "main failed with exception: " << ex.what() << "\n";
    return EXIT_FAILURE;
}
)--";
}

void Generator::cmakelists(llvm::raw_ostream& outFile) {
    outFile << llvm::formatv(R"--(
cmake_minimum_required(VERSION 3.25 FATAL_ERROR)

project({0}
    DESCRIPTION "Viam C++ {0} Module"
    LANGUAGES CXX
)

# Everything needs threads, and prefer -pthread if available
set(THREADS_PREFER_PTHREAD_FLAG ON)
find_package(Threads REQUIRED)

find_package(viam-cpp-sdk CONFIG REQUIRED COMPONENTS viamsdk)

add_executable({0}
    main.cpp
    src/{1}.cpp
)

target_include_directories({0} PUBLIC src)

target_link_libraries({0}
    viam-cpp-sdk::viamsdk
)

install(
    FILES meta.json
    DESTINATION .
)

install(TARGETS {0})

set(CPACK_PACKAGE_NAME "{0}")
set(CPACK_PACKAGE_FILE_NAME "module")
set(CPACK_GENERATOR "TGZ")
set(CPACK_INCLUDE_TOPLEVEL_DIRECTORY 0)
include(CPack)

)--",
                             fmt_str::moduleName,
                             fmt_str::modelSnake);
}

void Generator::conanfile(llvm::raw_ostream& outFile) {
    outFile << llvm::formatv(R"--(
from conan import ConanFile
from conan.tools.cmake import CMakeToolchain, CMake, cmake_layout, CMakeDeps

class {0}Recipe(ConanFile):
    name = "{1}"
    version = "0.1"
    package_type = "application"

    # Optional metadata
    license = "<Put the package license here>"
    author = "<Put your name here> <And your email here>"
    url = "<Package recipe repository url here, for issues about the package>"
    description = "<Description of mysensor package here>"
    topics = ("<Put some tag here>", "<here>", "<and here>")

    # Binary configuration
    settings = "os", "compiler", "build_type", "arch"

    # Sources are located in the same place as this recipe, copy them to the recipe
    exports_sources = "CMakeLists.txt", "src/*", "main.cpp", "meta.json"

    def layout(self):
        cmake_layout(self)

    def configure(self):
        self.options["viam-cpp-sdk"].shared = False

    def generate(self):
        deps = CMakeDeps(self)
        deps.generate()
        tc = CMakeToolchain(self)
        tc.generate()

    def build(self):
        cmake = CMake(self)
        cmake.configure()
        cmake.build(target="package")

    def package(self):
        cmake = CMake(self)
        cmake.install()

    def requirements(self):
        self.requires("viam-cpp-sdk/{2}")
)--",
                             fmt_str::modulePascal,
                             fmt_str::moduleName,
                             fmt_str::sdkVersion);
}

void Generator::run() {
    header_prefix();
    src_prefix();

    do_stubs();

    *headerOut_ << "};\n\n";

    const auto close_ns = llvm::formatv("} // namespace {0}\n\n", fmt_str::moduleName);

    *headerOut_ << close_ns;
    *srcOut_ << close_ns;
}

std::string Generator::resourceToSource(llvm::StringRef resourceSubtype,
                                        Generator::ResourceType resourceType,
                                        Generator::SrcType srcType) {
    if (resourceSubtype.startswith("generic_")) {
        resourceSubtype = "generic";
    }

    return llvm::formatv("{0}/{1}.{2}",
                         (resourceType == ResourceType::component) ? "components" : "services",
                         resourceSubtype,
                         (srcType == SrcType::hpp) ? "hpp" : "cpp");
}

}  // namespace viam::gen

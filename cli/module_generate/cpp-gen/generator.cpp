#include "generator.hpp"

#include "compilation_db.hpp"
#include "compiler_info.hpp"
#include "template_constants.hpp"
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
                            llvm::raw_ostream& moduleFile) {
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
        moduleFile);
}

Generator Generator::createFromCommandLine(const clang::tooling::CompilationDatabase& db,
                                           llvm::StringRef sourceFile,
                                           llvm::raw_ostream& outFile) {
    return Generator(GeneratorCompDB(db, getCompilersDefaultIncludeDir(db, true)),
                     to_resource_type((*++llvm::sys::path::rbegin(sourceFile)).drop_back()),
                     llvm::sys::path::stem(sourceFile).str(),
                     sourceFile.str(),
                     outFile);
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
                     llvm::raw_ostream& moduleFile)
    : db_(std::move(db)),
      resourceType_(resourceType),
      resourceSubtypeSnake_(std::move(resourceSubtypeSnake)),
      resourceSubtypePascal_(llvm::convertToCamelFromSnakeCase(resourceSubtypeSnake_, true)),
      resourcePath_(std::move(resourcePath)),
      moduleFile_(moduleFile) {
    if (llvm::StringRef(resourceSubtypeSnake_).startswith("generic_")) {
        resourceSubtypeSnake_ = "generic";
    }
}

int Generator::run() {
    include_stmts();

    const char* fmt =
        R"--(
class {0} : public viam::sdk::{1}, public viam::sdk::Reconfigurable {{
public:
    {0}(const viam::sdk::Dependencies& deps, const viam::sdk::ResourceConfig& cfg) : {1}(cfg.name()) {{
        this->reconfigure(deps, cfg);
    }

)--";

    moduleFile_ << llvm::formatv(fmt, fmt_str::modelPascal, resourceSubtypePascal_);

    moduleFile_ << R"--(
    static std::vector<std::string> validate(const viam::sdk::ResourceConfig&)
    {
        throw std::runtime_error("\"validate\" not implemented");
    }

    void reconfigure(const viam::sdk::Dependencies&, const viam::sdk::ResourceConfig&) override
    {
        throw std::runtime_error("\"reconfigure\" not implemented");
    }

)--";

    int result = do_stubs();

    if (result != 0) {
        throw std::runtime_error("Nonzero return from stub generation");
    }

    moduleFile_ << "};\n\n";

    main_fn();

    return 0;
}

template <>
const char* Generator::include_fmt<Generator::ResourceType::component>() {
    constexpr const char* fmt = R"--(
#include <iostream>
#include <memory>
#include <vector>

#include <viam/sdk/common/exception.hpp>
#include <viam/sdk/common/instance.hpp>
#include <viam/sdk/common/proto_value.hpp>
#include <viam/sdk/{0}>
#include <viam/sdk/config/resource.hpp>
#include <viam/sdk/log/logging.hpp>
#include <viam/sdk/module/service.hpp>
#include <viam/sdk/registry/registry.hpp>
#include <viam/sdk/resource/reconfigurable.hpp>

    )--";

    return fmt;
}

template <>
const char* Generator::include_fmt<Generator::ResourceType::service>() {
    constexpr const char* fmt = R"--(
#include <iostream>
#include <memory>
#include <vector>

#include <viam/sdk/common/exception.hpp>
#include <viam/sdk/common/instance.hpp>
#include <viam/sdk/common/proto_value.hpp>
#include <viam/sdk/config/resource.hpp>
#include <viam/sdk/log/logging.hpp>
#include <viam/sdk/module/service.hpp>
#include <viam/sdk/registry/registry.hpp>
#include <viam/sdk/resource/reconfigurable.hpp>
#include <viam/sdk/{0}>
    )--";

    return fmt;
}

void Generator::include_stmts() {
    const char* fmt = (resourceType_ == ResourceType::component)
                          ? include_fmt<ResourceType::component>()
                          : include_fmt<ResourceType::service>();

    moduleFile_ << llvm::formatv(
        fmt, resourceToSource(resourceSubtypeSnake_, resourceType_, SrcType::hpp));
}

int Generator::do_stubs() {
    clang::tooling::ClangTool tool(db_, resourcePath_);

    using namespace clang::ast_matchers;

    std::string qualName = ("viam::sdk::" + resourceSubtypePascal_);

    DeclarationMatcher methodMatcher =
        cxxMethodDecl(isPure(), hasParent(cxxRecordDecl(hasName(qualName)))).bind("method");

    struct MethodPrinter : MatchFinder::MatchCallback {
        MethodPrinter(llvm::raw_ostream& os_) : os(os_) {}

        llvm::raw_ostream& os;

        void printParm(const clang::ParmVarDecl& parm) {
            os << clang::TypeName::getFullyQualifiedName(
                      parm.getType(), parm.getASTContext(), {parm.getASTContext().getLangOpts()})
               << " " << parm.getName();
        }

        void run(const MatchFinder::MatchResult& result) override {
            if (const auto* method = result.Nodes.getNodeAs<clang::CXXMethodDecl>("method")) {
                clang::PrintingPolicy printPolicy(method->getASTContext().getLangOpts());
                printPolicy.FullyQualifiedName = 1;

                os << "    "
                   << clang::TypeName::getFullyQualifiedName(
                          method->getReturnType(), method->getASTContext(), printPolicy)
                   << " " << method->getName() << "(";

                if (method->getNumParams() > 0) {
                    auto param_begin = method->param_begin();
                    printParm(**param_begin);

                    if (method->getNumParams() > 1) {
                        for (const clang::ParmVarDecl* parm :
                             llvm::makeArrayRef(++param_begin, method->param_end())) {
                            os << ", ";
                            printParm(*parm);
                        }
                    }
                }

                os << ")";

                method->getMethodQualifiers().print(os, printPolicy, false);

                os << " override";

                os << llvm::formatv(R"--(
    {
        throw std::logic_error("\"{0}\" not implemented");
    }

)--",
                                    method->getName());
            }
        }
    };

    MethodPrinter printer(moduleFile_);
    MatchFinder finder;

    finder.addMatcher(methodMatcher, &printer);

    return tool.run(clang::tooling::newFrontendActionFactory(&finder).get());
}

void Generator::main_fn() {
    moduleFile_ << "int main(int argc, char** argv) try {\n"
                << llvm::formatv(R"--(
    // Every Viam C++ SDK program must have one and only one Instance object which is created before
    // any other SDK objects and stays alive until all of them are destroyed.
    viam::sdk::Instance inst;

    // Write general log statements using the VIAM_SDK_LOG macro.
    VIAM_SDK_LOG(info) << "Starting up {1} module";

    Model model("viam", "{0}", "{1}");)--",
                                 resourceSubtypeSnake_,
                                 fmt_str::modelSnake)
                << "\n\n"
                << llvm::formatv(
                       R"--(
    std::shared_ptr<ModelRegistration> mr = std::make_shared<ModelRegistration>(
        API::get<{viam::sdk::0}>,
        model,
        [](viam::sdk::Dependencies deps, viam::sdk::ResourceConfig cfg) {
            return std::make_unique<{1}>(deps, cfg);
        },
        &{1}::validate);
)--",
                       resourceSubtypePascal_,
                       fmt_str::modelPascal)
                << "\n\n"
                <<
        R"--(
    std::vector<std::shared_ptr<ModelRegistration>> mrs = {mr};
    auto my_mod = std::make_shared<ModuleService>(argc, argv, mrs);
    my_mod->serve();

    return EXIT_SUCCESS;
} catch (const viam::sdk::Exception& ex) {
    std::cerr << "main failed with exception: " << ex.what() << "\n";
    return EXIT_FAILURE;
}
)--";
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

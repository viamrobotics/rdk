#include "generator.hpp"

#include "compilation_db.hpp"
#include "compiler_info.hpp"
#include <clang/AST/PrettyPrinter.h>
#include <clang/ASTMatchers/ASTMatchFinder.h>
#include <clang/ASTMatchers/ASTMatchers.h>
#include <clang/Frontend/FrontendActions.h>
#include <clang/Tooling/JSONCompilationDatabase.h>
#include <llvm/ADT/SmallVector.h>
#include <llvm/ADT/StringMap.h>
#include <llvm/Support/FormatVariadic.h>
#include <llvm/Support/Path.h>
#include <llvm/Support/raw_ostream.h>

#include <string_view>
#include <unordered_map>

namespace viam::gen {

Generator Generator::create(llvm::StringRef className,
                            llvm::StringRef componentName,
                            llvm::StringRef buildDir,
                            llvm::StringRef sourceDir,
                            llvm::raw_ostream& moduleFile) {
    std::string error;
    auto jsonDb = clang::tooling::JSONCompilationDatabase::autoDetectFromDirectory(buildDir, error);
    if (!jsonDb) {
        throw std::runtime_error(error);
    }

    llvm::outs() << llvm::formatv("class {0} component {1} build {2} source {3}\n",
                                  className,
                                  componentName,
                                  buildDir,
                                  sourceDir);

    return Generator(GeneratorCompDB(*jsonDb, getCompilersDefaultIncludeDir(*jsonDb, true)),
                     className.str(),
                     componentName.str(),
                     (sourceDir + componentNameToSource(componentName)).str(),
                     moduleFile);
}

Generator Generator::createFromCommandLine(const clang::tooling::CompilationDatabase& compDb,
                                           llvm::StringRef sourceFile,
                                           llvm::raw_ostream& moduleFile) {
    llvm::SmallVector<llvm::StringRef, 3> splits;
    auto componentFileName = llvm::sys::path::filename(sourceFile);

    componentFileName.substr(0, componentFileName.find('.')).split(splits, '_');

    std::string componentName;

    llvm::raw_string_ostream os(componentName);

    for (llvm::StringRef component : splits) {
        os << static_cast<char>(std::toupper(*component.bytes_begin())) << component.drop_front();
    }

    return Generator(GeneratorCompDB(compDb, getCompilersDefaultIncludeDir(compDb, true)),
                     "My" + componentName,
                     componentName,
                     sourceFile.str(),
                     moduleFile);
}

int Generator::run() {
    // TODO: this should store the result of do_stubs in an intermediate and only write to the
    // output stream if it succeeded
    // also should take a config option for the class name and module/ns name
    include_stmts();

    const char* fmt =
        R"--(
class {0} : public viam::sdk::{1}, public viam::sdk::Reconfigurable {{
public:
    {0}(const viam::sdk::Dependencies& deps, const viam::sdk::ResourceConfig& cfg) : {1}(cfg.name()) {{
        this->reconfigure(deps, cfg);
    }

)--";

    moduleFile_ << llvm::formatv(fmt, className_, componentName_);

    moduleFile_ << R"--(
    static std::vector<std::string> validate(const viam::sdk::ResourceConfig&)
    {
        throw std::runtime_error("\"validate\" not implemented");
    }

    void reconfigure(const viam::sdk::Dependencies&, const ResourceConfig&) override
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

void Generator::include_stmts() {
    const char* fmt = R"--(
#include <iostream>
#include <memory>
#include <vector>

#include <viam/sdk/common/exception.hpp>
#include <viam/sdk/common/instance.hpp>
#include <viam/sdk/common/proto_value.hpp>
#include <viam/sdk/components/{0}.hpp
#include <viam/sdk/config/resource.hpp>
#include <viam/sdk/log/logging.hpp>
#include <viam/sdk/module/service.hpp>
#include <viam/sdk/registry/registry.hpp>
#include <viam/sdk/resource/reconfigurable.hpp>

    )--";

    llvm::StringRef cppFilename = componentNameToSource(componentName_);

    moduleFile_ << llvm::formatv(fmt,
                                 llvm::StringRef(cppFilename).substr(0, cppFilename.find('.')));
}

int Generator::do_stubs() {
    clang::tooling::ClangTool tool(db_, componentPath_);

    using namespace clang::ast_matchers;

    std::string qualName = ("viam::sdk::" + componentName_);

    DeclarationMatcher methodMatcher =
        cxxMethodDecl(isPure(), hasParent(cxxRecordDecl(hasName(qualName)))).bind("method");

    struct MethodPrinter : MatchFinder::MatchCallback {
        MethodPrinter(llvm::raw_ostream& os_) : os(os_) {}

        llvm::raw_ostream& os;

        void printParm(const clang::ParmVarDecl& parm) {
            os << parm.getType().getAsString({parm.getASTContext().getLangOpts()}) << " "
               << parm.getName();
        }

        void run(const MatchFinder::MatchResult& result) override {
            if (const auto* method = result.Nodes.getNodeAs<clang::CXXMethodDecl>("method")) {
                clang::PrintingPolicy printPolicy(method->getASTContext().getLangOpts());

                os << "    " << method->getReturnType().getAsString(printPolicy) << " "
                   << method->getName() << "(";

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
    llvm::StringRef cppFilename = componentNameToSource(componentName_);

    llvm::StringRef c1 = cppFilename.substr(0, cppFilename.find('.'));

    std::string c2 = ("my_" + c1).str();

    moduleFile_ << "int main(int argc, char** argv) try {\n"
                << llvm::formatv(R"--(
    // Every Viam C++ SDK program must have one and only one Instance object which is created before
    // any other SDK objects and stays alive until all of them are destroyed.
    viam::sdk::Instance inst;

    // Write general log statements using the VIAM_SDK_LOG macro.
    VIAM_SDK_LOG(info) << "Starting up {1} module";

    Model model("viam", "{0}", "{1}");)--",
                                 c1,
                                 c2)
                << "\n\n"
                << llvm::formatv(
                       R"--(
    std::shared_ptr<ModelRegistration> mr = std::make_shared<ModelRegistration>(
        API::get<{0}>,
        model,
        [](viam::sdk::Dependencies deps, viam::sdk::ResourceConfig cfg) {
            return std::make_unique<{1}>(deps, cfg);
        },
        &{1}::validate);
)--",
                       componentName_,
                       className_)
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

Generator::Generator(GeneratorCompDB db,
                     std::string className,
                     std::string componentName,
                     std::string componentPath,
                     llvm::raw_ostream& moduleFile)
    : db_(std::move(db)),
      className_(std::move(className)),
      componentName_(std::move(componentName)),
      componentPath_(std::move(componentPath)),
      moduleFile_(moduleFile) {
    llvm::outs() << llvm::formatv(
        "class {0} component {1} path {2}\n", className_, componentName_, componentPath_);
}

llvm::StringRef Generator::componentNameToSource(llvm::StringRef componentName) {
    static std::unordered_map<std::string_view, std::string_view> correspondence{
        {"Arm", "arm.cpp"},
        {"Base", "base.cpp"},
        {"Board", "board.cpp"},
        {"Camera", "camera.cpp"},
        {"Component", "component.cpp"},
        {"Encoder", "encoder.cpp"},
        {"Gantry", "gantry.cpp"},
        {"Generic", "generic.cpp"},
        {"Gripper", "gripper.cpp"},
        {"Motor", "motor.cpp"},
        {"MovementSensor", "movement_sensor.cpp"},
        {"PoseTracker", "pose_tracker.cpp"},
        {"PowerSensor", "power_sensor.cpp"},
        {"Sensor", "sensor.cpp"},
        {"Servo", "servo.cpp"}};

    return correspondence.at(componentName);
}

}  // namespace viam::gen

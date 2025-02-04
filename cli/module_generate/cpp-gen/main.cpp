#include "compiler_info.hpp"

#include <clang/AST/PrettyPrinter.h>
#include <clang/ASTMatchers/ASTMatchFinder.h>
#include <clang/ASTMatchers/ASTMatchers.h>
#include <clang/Frontend/FrontendActions.h>
#include <clang/Tooling/CommonOptionsParser.h>
#include <clang/Tooling/CompilationDatabase.h>
#include <clang/Tooling/Tooling.h>
#include <llvm/Support/CommandLine.h>
#include <llvm/Support/Path.h>

#include <string>
#include <unordered_map>
#include <vector>

using namespace viam::gen;

static llvm::cl::OptionCategory opts("module-gen options");

struct GeneratorCompDB : clang::tooling::CompilationDatabase {
    GeneratorCompDB(
        const clang::tooling::CompilationDatabase& orig,
        const std::unordered_map<std::string, std::vector<std::string>>& implicitIncludes);

    std::vector<clang::tooling::CompileCommand> getCompileCommands(
        llvm::StringRef file) const override;

    std::vector<std::string> getAllFiles() const override;

    std::vector<clang::tooling::CompileCommand> getAllCompileCommands() const override {
        return commands_;
    }

    std::vector<clang::tooling::CompileCommand> commands_;
};

GeneratorCompDB::GeneratorCompDB(
    const clang::tooling::CompilationDatabase& orig,
    const std::unordered_map<std::string, std::vector<std::string>>& implicitIncludes) {
    commands_ = orig.getAllCompileCommands();

    for (clang::tooling::CompileCommand& cmd : commands_) {
        auto& cmdLine = cmd.CommandLine;
        if (auto it = implicitIncludes.find(cmdLine.front()); it != implicitIncludes.end()) {
            for (const auto& inc : it->second) {
                cmdLine.emplace_back("-isystem" + inc);
            }
        }
    }
}

std::vector<std::string> GeneratorCompDB::getAllFiles() const {
    std::vector<std::string> result;
    result.reserve(commands_.size());

    for (const auto& cmd : commands_) {
        result.push_back(cmd.Filename);
    }

    return result;
}

std::vector<clang::tooling::CompileCommand> GeneratorCompDB::getCompileCommands(
    llvm::StringRef file) const {
    auto it = std::find_if(commands_.begin(), commands_.end(), [file](const auto& cmd) {
        return file == cmd.Filename;
    });
    if (it == commands_.end()) {
        return {};
    }

    return {*it};
}

std::string className(llvm::StringRef fileName) {
    std::string result = fileName.str();
    result[0] = std::toupper(result[0]);

    std::size_t pos = result.find_first_of('_');

    while ((pos != std::string::npos) && (pos + 1 != result.size())) {
        result[pos + 1] = std::toupper(result[pos + 1]);

        pos = result.find_first_of('_', pos + 1);
    }

    result.erase(std::remove(result.begin(), result.end(), '_'), result.end());

    return result;
}

int main(int argc, const char** argv) {
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

    auto filename = llvm::sys::path::filename(sources.front());
    std::string class_ = className(filename.substr(0, filename.find('.')));

    const auto& db = OptionsParser.getCompilations();
    const GeneratorCompDB genDB{db, getCompilersDefaultIncludeDir(db, true)};

    clang::tooling::ClangTool Tool(genDB, OptionsParser.getSourcePathList());

    using namespace clang::ast_matchers;

    DeclarationMatcher methodMatcher =
        cxxMethodDecl(isPure(), hasParent(cxxRecordDecl(hasName("viam::sdk::" + class_))))
            .bind("method");

    struct MethodPrinter : MatchFinder::MatchCallback {
        static void printParm(const clang::ParmVarDecl& parm,
                              llvm::raw_ostream& out = llvm::outs()) {
            out << parm.getType().getAsString({parm.getASTContext().getLangOpts()}) << " "
                << parm.getName();
        }

        void run(const MatchFinder::MatchResult& result) override {
            if (const auto* method = result.Nodes.getNodeAs<clang::CXXMethodDecl>("method")) {
                clang::PrintingPolicy printPolicy(method->getASTContext().getLangOpts());

                llvm::outs() << method->getReturnType().getAsString(printPolicy) << " "
                             << method->getName() << "(";

                if (method->getNumParams() > 0) {
                    auto param_begin = method->param_begin();
                    printParm(**param_begin);

                    if (method->getNumParams() > 1) {
                        for (const clang::ParmVarDecl* parm :
                             llvm::makeArrayRef(++param_begin, method->param_end())) {
                            llvm::outs() << ", ";
                            printParm(*parm);
                        }
                    }
                }

                llvm::outs() << ")";

                method->getMethodQualifiers().print(llvm::outs(), printPolicy, true);

                llvm::outs() << "\n{\n\tthrow std::logic_error(\"" << method->getName()
                             << "\" not implemented\");\n}\n\n";
            }
        }
    };

    MethodPrinter printer;
    MatchFinder finder;

    finder.addMatcher(methodMatcher, &printer);

    return Tool.run(clang::tooling::newFrontendActionFactory(&finder).get());
}

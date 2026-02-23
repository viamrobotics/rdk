#pragma once

#include <clang/Tooling/CompilationDatabase.h>

#include <string>
#include <unordered_map>
#include <vector>

namespace viam::gen {

// An implementation of a clang CompilationDatabase to be used to instantiate a ClangTool
// for module generation.
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

}  // namespace viam::gen

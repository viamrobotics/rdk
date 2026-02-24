#pragma once

#include <clang/Tooling/CompilationDatabase.h>

#include <llvm/ADT/StringRef.h>

#include <optional>
#include <string>
#include <unordered_map>
#include <vector>

// The functions here and their implementations are largely copied from MrDocs:
// https://github.com/cppalliance/mrdocs/blob/develop/src/tool/CompilerInfo.hpp

namespace viam::gen {

// Get the verbose output of a compiler, including the implicit include paths
std::optional<std::string> getCompilerVerboseOutput(llvm::StringRef compilerPath);

// Parse the include paths from getCompilerVerboseOutput to retrieve the implicit include paths.
std::vector<std::string> parseIncludePaths(std::string const& compilerOutput);

std::unordered_map<std::string, std::vector<std::string>> getCompilersDefaultIncludeDir(
    clang::tooling::CompilationDatabase const& compDb, bool useSystemStdlib);

}  // namespace viam::gen

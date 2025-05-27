#pragma once

#include "compilation_db.hpp"

#include <clang/Tooling/Tooling.h>
#include <llvm/ADT/StringRef.h>
#include <llvm/Support/raw_ostream.h>

namespace viam::gen {

class Generator {
   public:
    static Generator create(llvm::StringRef className,
                            llvm::StringRef componentName,
                            llvm::StringRef buildDir,
                            llvm::StringRef sourceDir,
                            llvm::raw_ostream& moduleFile);

    static Generator createFromCommandLine(const clang::tooling::CompilationDatabase& compDb,
                                           llvm::StringRef sourceFile,
                                           llvm::raw_ostream& moduleFile);

    int run();

   private:
    void include_stmts();
    int do_stubs();
    void main_fn();

    Generator(GeneratorCompDB db,
              std::string className,
              std::string componentName,
              std::string componentPath,
              llvm::raw_ostream& moduleFile);

    static llvm::StringRef componentNameToSource(llvm::StringRef className);

    GeneratorCompDB db_;

    std::string className_;
    std::string componentName_;

    std::string componentPath_;

    llvm::raw_ostream& moduleFile_;
};

}  // namespace viam::gen

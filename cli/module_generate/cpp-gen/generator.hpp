#pragma once

#include "compilation_db.hpp"

#include <clang/Tooling/Tooling.h>
#include <llvm/ADT/StringRef.h>
#include <llvm/Support/raw_ostream.h>

namespace viam::gen {

class Generator {
   public:
    enum class ResourceType : bool { component, service };

    struct ModuleInfo {
        ResourceType resourceType;
        llvm::StringRef resourceSubtype;
        llvm::StringRef modelName;
    };

    struct CppTreeInfo {
        llvm::StringRef buildDir;
        llvm::StringRef sourceDir;
    };

    static Generator create(ModuleInfo moduleInfo,
                            CppTreeInfo cppInfo,
                            llvm::raw_ostream& moduleFile);

    int run();

   private:
    template <ResourceType>
    const char* include_fmt();

    void include_stmts();
    int do_stubs();
    void main_fn();

    Generator(GeneratorCompDB db,
              ResourceType resourceType,
              std::string resourceSubtype,
              std::string modelName,
              std::string resourcePath,
              llvm::raw_ostream& moduleFile);

    enum SrcType { cpp, hpp };

    static std::string resourceToSource(llvm::StringRef resourceSubtype,
                                        ResourceType resourceType,
                                        SrcType);

    GeneratorCompDB db_;

    ResourceType resourceType_;
    std::string resourceSubtype_;
    std::string modelName_;

    std::string className_;

    std::string resourcePath_;

    llvm::raw_ostream& moduleFile_;
};

}  // namespace viam::gen

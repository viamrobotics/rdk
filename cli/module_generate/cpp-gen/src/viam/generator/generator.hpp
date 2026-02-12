#pragma once

#include <viam/generator/compilation_db.hpp>

#include <clang/Tooling/Tooling.h>
#include <llvm/ADT/StringRef.h>
#include <llvm/Support/raw_ostream.h>

namespace viam::gen {

class Generator {
   public:
    enum class ResourceType : bool { component, service };

    struct ModuleInfo {
        ResourceType resourceType;
        llvm::StringRef resourceSubtypeSnake;
    };

    struct CppTreeInfo {
        llvm::StringRef buildDir;
        llvm::StringRef sourceDir;
    };

    static Generator create(ModuleInfo moduleInfo,
                            CppTreeInfo cppInfo,
                            llvm::raw_ostream& moduleFile);

    static Generator createFromCommandLine(const clang::tooling::CompilationDatabase& db,
                                           llvm::StringRef sourceFile,
                                           llvm::raw_ostream& outFile);

    static ResourceType to_resource_type(llvm::StringRef resourceType);

    static void main_fn(llvm::raw_ostream& moduleFile);

    static void cmakelists(llvm::raw_ostream& outFile);

    int run();

   private:
    template <ResourceType>
    const char* include_fmt();

    void include_stmts();
    int do_stubs();

    Generator(GeneratorCompDB db,
              ResourceType resourceType,
              std::string resourceSubtypeSnake,
              std::string resourcePath,
              llvm::raw_ostream& moduleFile);

    enum SrcType { cpp, hpp };

    static std::string resourceToSource(llvm::StringRef resourceSubtype,
                                        ResourceType resourceType,
                                        SrcType);

    GeneratorCompDB db_;

    ResourceType resourceType_;

    std::string resourceSubtypeSnake_;
    std::string resourceSubtypePascal_;

    std::string resourcePath_;

    llvm::raw_ostream& moduleFile_;
};

}  // namespace viam::gen

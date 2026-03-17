#pragma once

#include <viam/generator/compilation_db.hpp>

#include <clang/Tooling/Tooling.h>
#include <llvm/ADT/StringRef.h>
#include <llvm/Support/raw_ostream.h>

#include <memory>

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
                            std::unique_ptr<llvm::raw_fd_ostream> headerOut,
                            std::unique_ptr<llvm::raw_fd_ostream> srcOut);

    static Generator createFromCommandLine(const clang::tooling::CompilationDatabase& db,
                                           llvm::StringRef sourceFile,
                                           std::unique_ptr<llvm::raw_fd_ostream> headerOut,
                                           std::unique_ptr<llvm::raw_fd_ostream> srcOut);

    static ResourceType to_resource_type(llvm::StringRef resourceType);

    static void main_fn(llvm::raw_ostream& moduleFile);

    static void cmakelists(llvm::raw_ostream& outFile);

    static void conanfile(llvm::raw_ostream& outFile);

    void run();

   private:
    template <ResourceType>
    const char* include_fmt();

    void header_prefix();

    void src_prefix();

    void include_stmts();
    void do_stubs();

    Generator(GeneratorCompDB db,
              ResourceType resourceType,
              std::string resourceSubtypeSnake,
              std::string resourcePath,
              std::unique_ptr<llvm::raw_fd_ostream> headerOut,
              std::unique_ptr<llvm::raw_fd_ostream> srcOut);

    enum SrcType { cpp, hpp };

    static std::string resourceToSource(llvm::StringRef resourceSubtype,
                                        ResourceType resourceType,
                                        SrcType);

    GeneratorCompDB db_;

    ResourceType resourceType_;

    std::string resourceSubtypeSnake_;
    std::string resourceSubtypePascal_;

    std::string resourcePath_;

    std::unique_ptr<llvm::raw_fd_ostream> headerOut_;
    std::unique_ptr<llvm::raw_fd_ostream> srcOut_;
};

}  // namespace viam::gen

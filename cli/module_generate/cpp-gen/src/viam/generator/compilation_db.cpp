#include <viam/generator/compilation_db.hpp>

namespace viam::gen {

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

}  // namespace viam::gen

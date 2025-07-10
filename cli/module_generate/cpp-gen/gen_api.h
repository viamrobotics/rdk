#ifdef __cplusplus
extern "C" {
#endif

int viam_cli_generate_cpp_module(const char* modelName,
                                 const char* resourceSubtype,
                                 const char* buildDir,
                                 const char* sourceDir,
                                 const char* outPath);
#ifdef __cplusplus
}
#endif

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix github.com/go-nlopt/nlopt
gazelle(name = "gazelle")

go_library(
    name = "nlopt",
    srcs = [
        "cfunc_reg.go",
        "nlopt.go",
        "nlopt.h",
        "nlopt_cfunc.go",
    ],
    cgo = True,
    clinkopts = ["-lnlopt", "-lm"],
    copts = ["-Os", "-fno-common", "-mtune=native", "-march=native"],
    importpath = "github.com/go-nlopt/nlopt",
    visibility = ["//visibility:public"],
)

go_test(
    name = "nlopt_test",
    srcs = ["nlopt_test.go"],
    embed = [":nlopt"],
)

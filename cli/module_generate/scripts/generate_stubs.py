import ast
import atexit
import json
import os
import shutil
import site
import subprocess
import sys
import tempfile
import time
from importlib import import_module
from typing import List, Set, Union


def sdk_requirement(extra: str = "") -> str:
    """Pip requirement for viam-sdk.

    VIAM_PYTHON_SDK_VERSION pins the install to the same version written into
    requirements.txt. Unset, the laptop CLI keeps installing the unpinned SDK.
    """
    version = os.environ.get("VIAM_PYTHON_SDK_VERSION", "").strip()
    name = "viam-sdk" if extra == "" else f"viam-sdk[{extra}]"
    if version:
        return f"{name}=={version}"
    return name


def return_attribute(value: str, attr: str) -> ast.Attribute:
    return ast.Attribute(
        value=ast.Name(id=value, ctx=ast.Load()),
        attr=attr,
        ctx=ast.Load(),
    )


def update_annotation(
    resource_name: str,
    annotation: Union[ast.Name, ast.Subscript],
    nodes: Set[str],
    parent: str,
) -> Union[ast.Attribute, ast.Subscript]:
    if isinstance(annotation, ast.Name) and annotation.id in nodes:
        value = parent if parent else resource_name
        return return_attribute(value, annotation.id)
    elif isinstance(annotation, ast.Subscript):
        annotation.slice = update_annotation(
            resource_name,
            annotation.slice,
            nodes,
            parent,
        )
    return annotation


def replace_async_func(
    resource_name: str, func: ast.AsyncFunctionDef, nodes: Set[str], parent: str = ""
) -> None:
    for arg in func.args.args:
        arg.annotation = update_annotation(resource_name, arg.annotation, nodes, parent)
    func.body = [
        ast.Expr(
            ast.Call(
                func=ast.Attribute(
                    value=ast.Attribute(ast.Name(id="self"), attr="logger"),
                    attr="error",
                ),
                args=[ast.Constant(value=f"`{func.name}` is not implemented")],
                keywords=[],
            )
        ),
        ast.Raise(
            exc=ast.Call(
                func=ast.Name(id="NotImplementedError", ctx=ast.Load()),
                args=[],
                keywords=[],
            ),
            cause=None,
        ),
    ]
    func.decorator_list = []
    if isinstance(func.returns, (ast.Name, ast.Subscript)):
        func.returns = update_annotation(resource_name, func.returns, nodes, parent)


def resolve_import_module(base_module: str, level: int, module: str) -> str:
    parts = base_module.split(".")
    base_parts = parts[:-level]
    return ".".join(base_parts) + ("." + module if module else "")


def is_abstract_class(cls: ast.ClassDef) -> bool:
    for node in ast.walk(cls):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            for decorator in node.decorator_list:
                if isinstance(decorator, ast.Name) and decorator.id == "abstractmethod":
                    return True
                if (
                    isinstance(decorator, ast.Attribute)
                    and decorator.attr == "abstractmethod"
                ):
                    return True
    return False


def return_subclass(
    resource_name: str, stmt: ast.ClassDef, parent: str = ""
) -> List[str]:
    def parse_subclass(resource_name: str, stmt: ast.ClassDef, parent: str):
        nodes = set()
        nodes_to_remove = []
        parent = parent if parent else resource_name
        stmt.bases = [ast.Name(id=f"{parent}.{stmt.name}", ctx=ast.Load())]
        for cstmt in stmt.body:
            if isinstance(cstmt, ast.Expr) or (
                isinstance(cstmt, ast.FunctionDef) and cstmt.name == "__init__"
            ):
                nodes_to_remove.append(cstmt)
            elif isinstance(cstmt, ast.AnnAssign):
                nodes.add(cstmt.target.id)
                nodes_to_remove.append(cstmt)
            elif isinstance(cstmt, ast.ClassDef):
                parse_subclass(resource_name, cstmt, stmt.bases[0].id)
            elif isinstance(cstmt, ast.AsyncFunctionDef):
                replace_async_func(resource_name, cstmt, nodes, stmt.bases[0].id)
        for node in nodes_to_remove:
            stmt.body.remove(node)
        if stmt.body == []:
            stmt.body = [ast.Pass()]

    parse_subclass(resource_name, stmt, parent)
    return "\n".join(["    " + line for line in ast.unparse(stmt).splitlines()])


def main(
    resource_type: str,
    resource_subtype: str,
    namespace: str,
    mod_name: str,
    model_name: str,
    slots: bool = False,
) -> str:
    from slugify import slugify

    module_name = f"viam.{resource_type}s.{resource_subtype}.{resource_subtype}"
    module = import_module(module_name)
    resource_name = {"input": "Controller", "slam": "SLAM", "mlmodel": "MLModel"}.get(
        resource_subtype,
        "".join(word.capitalize() for word in resource_subtype.split("_")),
    )

    imports, subclasses, abstract_methods = [], [], []
    nodes = set()
    modules_to_ignore = [
        "abc",
        "component_base",
        "service_base",
        "viam.resource.types",
    ]
    with open(module.__file__, "r") as f:
        tree = ast.parse(f.read())
        for stmt in tree.body:
            if isinstance(stmt, ast.Import):
                for imp in stmt.names:
                    if imp.name in modules_to_ignore:
                        continue
                    imports.append(
                        f"import {imp.name} as {imp.asname}"
                        if imp.asname
                        else f"import {imp.name}"
                    )
            elif isinstance(stmt, ast.ImportFrom):
                abs_module = (
                    resolve_import_module(module_name, stmt.level, stmt.module)
                    if stmt.level > 0
                    else stmt.module
                )
                if abs_module and abs_module not in modules_to_ignore:
                    i_strings = ", ".join(
                        [
                            (
                                f"{imp.name} as {imp.asname}"
                                if imp.asname is not None
                                else imp.name
                            )
                            for imp in stmt.names
                        ]
                    )
                    imports.append(f"from {abs_module} import {i_strings}")
            elif isinstance(stmt, ast.ClassDef) and stmt.name == resource_name:
                for cstmt in stmt.body:
                    if isinstance(cstmt, ast.ClassDef):
                        nodes.add(cstmt.name)
                    elif isinstance(cstmt, ast.AnnAssign):
                        nodes.add(cstmt.target.id)
                for cstmt in stmt.body:
                    if isinstance(cstmt, ast.ClassDef) and is_abstract_class(cstmt):
                        subclasses.append(return_subclass(resource_name, cstmt))
                    elif isinstance(cstmt, ast.AsyncFunctionDef):
                        replace_async_func(resource_name, cstmt, nodes)
                        indented_code = "\n".join(
                            ["    " + line for line in ast.unparse(cstmt).splitlines()]
                        )
                        abstract_methods.append(indented_code)

    type_module = import_module(f"viam.{resource_type}s.{resource_type}_base")
    with open(type_module.__file__, "r") as f:
        tree = ast.parse(f.read())
        for stmt in tree.body:
            if isinstance(stmt, ast.ClassDef):
                for cstmt in stmt.body:
                    if isinstance(cstmt, ast.AsyncFunctionDef):
                        replace_async_func("", cstmt, [])
                        indented_code = "\n".join(
                            ["    " + line for line in ast.unparse(cstmt).splitlines()]
                        )
                        abstract_methods.append(indented_code)
                        if cstmt.name == "do_command":
                            imports.append("from typing import Optional")
                            imports.append("from viam.utils import ValueTypes")
                        elif cstmt.name == "get_geometries":
                            imports.append(
                                "from typing import Any, Dict, List, Optional"
                            )
                            imports.append("from viam.proto.common import Geometry")

    model_name_pascal = "".join(
        [word.capitalize() for word in slugify(model_name).split("-")]
    )
    imports_text = "\n".join(list(set(imports)))
    subclasses_text = "\n\n".join([subclass for subclass in subclasses])
    methods_text = "\n\n".join([f"{method}" for method in abstract_methods])
    if slots:
        imports_text = f"/*inline-slot:imports*/\n{imports_text}\n/*inline-slot:imports-end*/"
        subclasses_text = f"/*inline-slot:methods*/\n{subclasses_text}"
        methods_text = f"{methods_text}\n/*inline-slot:methods-end*/"
    resource_file = '''
from typing import ClassVar, Mapping, Sequence, Tuple
from typing_extensions import Self
from viam.proto.app.robot import ComponentConfig
from viam.proto.common import ResourceName
from viam.resource.base import ResourceBase
from viam.resource.easy_resource import EasyResource
from viam.resource.types import Model, ModelFamily
{0}
from viam.{1}s.{2} import *


class {3}({4}, EasyResource):
    # To enable debug-level logging, either run viam-server with the --debug option,
    # or configure your resource/machine to display debug logs.
    MODEL: ClassVar[Model] = Model(ModelFamily("{5}", "{6}"), "{7}")

    @classmethod
    def new(cls, config: ComponentConfig, dependencies: Mapping[ResourceName, ResourceBase]) -> Self:
        """This method creates a new instance of this {4} {1}.
        The default implementation sets the name from the `config` parameter.

        Args:
            config (ComponentConfig): The configuration for this resource
            dependencies (Mapping[ResourceName, ResourceBase]): The dependencies (both required and optional)

        Returns:
            Self: The resource
        """
        return super().new(config, dependencies)

    @classmethod
    def validate_config(cls, config: ComponentConfig) -> Tuple[Sequence[str], Sequence[str]]:
        """This method allows you to validate the configuration object received from the machine,
        as well as to return any required dependencies or optional dependencies based on that `config`.

        Args:
            config (ComponentConfig): The configuration for this resource

        Returns:
            Tuple[Sequence[str], Sequence[str]]: A tuple where the
                first element is a list of required dependencies and the
                second element is a list of optional dependencies
        """
        return [], []

{8}
{9}
'''.format(
        imports_text,
        resource_type,
        resource_subtype,
        model_name_pascal,
        resource_name,
        namespace,
        mod_name,
        model_name,
        subclasses_text,
        methods_text,
    )
    if slots:
        old_class = f"class {model_name_pascal}({resource_name}, EasyResource):"
        new_class = (
            f"class {model_name_pascal}(/*inline-slot:bases*/{resource_name}, "
            "EasyResource/*inline-slot:bases-end*/):"
        )
        resource_file = resource_file.replace(old_class, new_class, 1)
        return resource_file
    with tempfile.NamedTemporaryFile(mode="w+", suffix=".py", delete=False) as f:
        f_name = f.name
        f.write(resource_file)
    try:
        try:
            subprocess.check_call(
                [sys.executable, "-m", "ruff", "format", "--quiet", f_name]
            )
            subprocess.check_call(
                [
                    sys.executable,
                    "-m",
                    "ruff",
                    "check",
                    "--select",
                    "F401,F811,I",
                    "--fix",
                    "--fix-only",
                    "--quiet",
                    f_name,
                ]
            )
            with open(f_name, "r") as f:
                resource_file = f.read()
        except subprocess.CalledProcessError:
            pass
    finally:
        os.remove(f_name)
    return resource_file


def pip_install_command(packages: List[str], target: str = "") -> List[str]:
    """Pip command for packages. A target dir keeps the bundle install off the active env."""
    command = [sys.executable, "-m", "pip", "install"]
    if target:
        # --isolated ignores a pip.conf `user = true`, which cannot combine with --target.
        command.extend(
            ["--isolated", "--disable-pip-version-check", "--target", target]
        )
    return command + packages


def install_requirements(
    packages: List[str], attempts: int = 3, target: str = ""
) -> None:
    """Install the packages this script imports.

    PyPI drops requests often enough that a single failed install regularly
    fails module generation, so retry with a backoff and, when the last attempt
    fails, raise with pip's own output instead of a bare exit status.
    """
    for attempt in range(1, attempts + 1):
        res = subprocess.run(
            pip_install_command(packages, target),
            capture_output=True,
            text=True,
        )
        if res.returncode == 0:
            return
        if attempt == attempts:
            output = res.stderr.strip() or res.stdout.strip()
            raise Exception(
                "Could not install requirements to generate python stubs:\n" + output
            )
        time.sleep(attempt * 2)


def run_bundle(version: str) -> None:
    """Install one SDK version, then print a JSON list of slotted model files.

    stdin is a list of {resource_type, resource_subtype, namespace, module_name, model_name}.
    """
    os.environ["VIAM_PYTHON_SDK_VERSION"] = version
    items = json.load(sys.stdin)
    # Install into a temp dir. The active interpreter may be a conda env whose
    # site-packages is not writable, and an older viam already installed there
    # must not shadow the pinned SDK.
    target = tempfile.mkdtemp(prefix="inline-python-deps-")
    atexit.register(shutil.rmtree, target, True)
    install_requirements(
        [
            sdk_requirement("mlmodel"),
            "typing-extensions",
            "ruff",
            "python-slugify",
            "numpy",
        ],
        target=target,
    )
    sys.path.insert(0, target)
    site.addsitedir(target)
    rendered = []
    for item in items:
        try:
            generated = main(
                item["resource_type"],
                item["resource_subtype"],
                item["namespace"],
                item["module_name"],
                item["model_name"],
                slots=True,
            )
        except ModuleNotFoundError as exc:
            # The installed SDK can lag a resource the CLI already lists. Skip
            # that API instead of failing the rest of the catalog.
            print(
                f"skipping {item['resource_subtype']} {item['resource_type']}: {exc}",
                file=sys.stderr,
            )
            continue
        rendered.append(
            {
                "resource_type": item["resource_type"],
                "resource_subtype": item["resource_subtype"],
                "file": generated,
            }
        )
    json.dump(rendered, sys.stdout)


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--bundle":
        run_bundle(sys.argv[2])
        sys.exit(0)
    packages = [sdk_requirement(), "typing-extensions", "ruff", "python-slugify"]
    if sys.argv[2] == "mlmodel":
        packages.append("numpy")
    install_requirements(packages)
    result = main(sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5])
    print(result)

import ast
import os
import subprocess
import sys
from importlib import import_module
from typing import List, Set


def return_attribute(value: str, attr: str) -> ast.Attribute:
    return ast.Attribute(
        value=ast.Name(id=value, ctx=ast.Load()),
        attr=attr,
        ctx=ast.Load())


def update_annotation(
    resource_name: str,
    annotation: ast.Name | ast.Subscript,
    nodes: Set[str],
    parent: str
) -> ast.Attribute | ast.Subscript:
    if isinstance(annotation, ast.Name) and annotation.id in nodes:
        value = parent if parent else resource_name
        return return_attribute(value, annotation.id)
    elif isinstance(annotation, ast.Subscript):
        annotation.slice = update_annotation(
            resource_name,
            annotation.slice,
            nodes,
            parent)
    return annotation


def replace_async_func(
    resource_name: str,
    func: ast.AsyncFunctionDef,
    nodes: Set[str],
    parent: str = ""
) -> None:
    for arg in func.args.args:
        arg.annotation = update_annotation(
            resource_name,
            arg.annotation,
            nodes,
            parent)
    func.body = [
        ast.Raise(
            exc=ast.Call(func=ast.Name(id='NotImplementedError',
                                       ctx=ast.Load()),
                         args=[],
                         keywords=[]),
            cause=None)
    ]
    func.decorator_list = []
    if isinstance(func.returns, (ast.Name, ast.Subscript)):
        func.returns = update_annotation(
            resource_name, func.returns, nodes, parent
        )


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
    return '\n'.join(
        ['    ' + line for line in ast.unparse(stmt).splitlines()]
    )


def main(
    resource_type: str,
    resource_subtype: str,
    namespace: str,
    mod_name: str,
    model_name: str,
) -> str:
    import isort
    from slugify import slugify

    module_name = (
        f"viam.{resource_type}s.{resource_subtype}.{resource_subtype}"
    )
    module = import_module(module_name)
    resource_name = {
        "input": "Controller", "slam": "SLAM", "mlmodel": "MLModel"
    }.get(resource_subtype, "".join(word.capitalize()
                                    for word in resource_subtype.split("_")))

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
                    imports.append(f"import {imp.name} as {imp.asname}"
                                   if imp.asname else f"import {imp.name}")
            elif (
                isinstance(stmt, ast.ImportFrom)
                and stmt.module
                and stmt.module not in modules_to_ignore
            ):
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
                i = f"from {stmt.module} import {i_strings}"
                imports.append(i)
            elif isinstance(stmt, ast.ClassDef) and stmt.name == resource_name:
                for cstmt in stmt.body:
                    if isinstance(cstmt, ast.ClassDef):
                        subclasses.append(return_subclass(resource_name, cstmt))
                    elif isinstance(cstmt, ast.AnnAssign):
                        nodes.add(cstmt.target.id)
                    elif isinstance(cstmt, ast.AsyncFunctionDef):
                        replace_async_func(resource_name, cstmt, nodes)
                        indented_code = '\n'.join(
                            ['    ' + line for line in ast.unparse(cstmt).splitlines()]
                        )
                        abstract_methods.append(indented_code)

    model_name_pascal = "".join(
        [word.capitalize() for word in slugify(model_name).split("-")]
    )
    main_file = '''
import asyncio
from typing import ClassVar, Mapping, Sequence
from typing_extensions import Self
from viam.module.module import Module
from viam.proto.app.robot import ComponentConfig
from viam.proto.common import ResourceName
from viam.resource.base import ResourceBase
from viam.resource.easy_resource import EasyResource
from viam.resource.types import Model, ModelFamily
{0}
from viam.{1}s.{2} import *


class {3}({4}, EasyResource):
    MODEL: ClassVar[Model] = Model(ModelFamily("{5}", "{6}"), "{7}")

    @classmethod
    def new(cls, config: ComponentConfig, dependencies: Mapping[ResourceName, ResourceBase]) -> Self:
        """This method creates a new instance of this {4} {1}.
        The default implementation sets the name from the `config` parameter and then calls `reconfigure`.

        Args:
            config (ComponentConfig): The configuration for this resource
            dependencies (Mapping[ResourceName, ResourceBase]): The dependencies (both implicit and explicit)

        Returns:
            Self: The resource
        """
        return super().new(config, dependencies)

    @classmethod
    def validate_config(cls, config: ComponentConfig) -> Sequence[str]:
        """This method allows you to validate the configuration object received from the machine,
        as well as to return any implicit dependencies based on that `config`.

        Args:
            config (ComponentConfig): The configuration for this resource

        Returns:
            Sequence[str]: A list of implicit dependencies
        """
        return []

    def reconfigure(self, config: ComponentConfig, dependencies: Mapping[ResourceName, ResourceBase]):
        """This method allows you to dynamically update your service when it receives a new `config` object.

        Args:
            config (ComponentConfig): The new configuration
            dependencies (Mapping[ResourceName, ResourceBase]): Any dependencies (both implicit and explicit)
        """
        return super().reconfigure(config, dependencies)

{8}
{9}


if __name__ == '__main__':
    asyncio.run(Module.run_from_registry())

'''.format(
        "\n".join(list(set(imports))),
        resource_type,
        resource_subtype,
        model_name_pascal,
        resource_name,
        namespace,
        mod_name,
        model_name,
        '\n\n'.join([subclass for subclass in subclasses]),
        '\n\n'.join([f'{method}' for method in abstract_methods]),
    )
    f_name = os.path.join(mod_name, "src", "main.py")
    with open(f_name, "w+") as f:
        f.write(main_file)
        try:
            f.seek(0)
            subprocess.check_call([sys.executable, "-m", "black", f_name, "-q"])
            f.seek(0)
            main_file = f.read()
        except subprocess.CalledProcessError:
            pass
    os.remove(f_name)
    sorted_main = isort.code(main_file)
    return sorted_main


if __name__ == "__main__":
    packages = ["viam-sdk", "typing-extensions", "black", "isort", "python-slugify"]
    if sys.argv[2] == "mlmodel":
        packages.append("numpy")
    install_res = subprocess.run(
        [
            sys.executable,
            "-m",
            "pip",
            "install"
        ] + packages,
        capture_output=True,
    )
    if install_res.returncode != 0:
        raise Exception("Could not install requirements to generate python stubs")
    result = main(sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5])
    print(result)

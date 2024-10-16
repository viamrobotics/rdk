import ast
import os
import subprocess
import sys
from importlib import import_module
from typing import List


def return_attribute(resource_name: str, attr: str) -> ast.Attribute:
    return ast.Attribute(
        value=ast.Name(id=resource_name, ctx=ast.Load()),
        attr=attr,
        ctx=ast.Load())


def replace_async_func(stmt: ast.AsyncFunctionDef) -> None:
    stmt.body = [
        ast.Raise(
            exc=ast.Call(func=ast.Name(id='NotImplementedError', ctx=ast.Load()),
                         args=[], 
                         keywords=[]),
            cause=None)
    ]
    stmt.decorator_list = []


def get_final_imports(imports: List[str]) -> List[str]:
    final_imports = []
    for i in imports:
        if i not in final_imports:
            final_imports.append(i)
    return final_imports


def main(
    resource_type: str,
    resource_subtype: str,
    namespace: str,
    mod_name: str,
    model_name: str,
) -> str:
    import isort
    from slugify import slugify

    module_name = f"viam.{resource_type}s.{resource_subtype}.{resource_subtype}"
    module = import_module(module_name)
    if resource_subtype == "input":
        resource_name = "Controller"
    elif resource_subtype == "slam":
        resource_name = "SLAM"
    elif resource_subtype == "mlmodel":
        resource_name = "MLModel"
    else:
        resource_name = "".join(word.capitalize() for word in resource_subtype.split("_"))

    imports = []
    modules_to_ignore = [
        "abc",
        "component_base",
        "service_base",
        "viam.resource.types",
    ]
    abstract_methods = []
    subclasses = []
    with open(module.__file__, "r") as f:
        def update_annotation(annotation):
            if isinstance(annotation, ast.Name) and annotation.id in nodes:
                return return_attribute(resource_name, annotation.id)
            elif isinstance(annotation, ast.Subscript):
                annotation.slice = update_annotation(annotation.slice)
                return annotation
            return annotation

        tree = ast.parse(f.read())
        nodes = []
        for stmt in tree.body:
            if isinstance(stmt, ast.Import):
                for imp in stmt.names:
                    if imp.name in modules_to_ignore:
                        continue
                    if imp.asname:
                        imports.append(f"import {imp.name} as {imp.asname}")
                    else:
                        imports.append(f"import {imp.name}")
            elif isinstance(stmt, ast.ImportFrom):
                if stmt.module in modules_to_ignore or stmt.module is None:
                    continue
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
                        cstmt.bases = [ast.Name(id=f"{resource_name}.{cstmt.name}", ctx=ast.Load())]
                        for scstmt in cstmt.body:
                            if isinstance(scstmt, ast.Expr):
                                cstmt.body.remove(scstmt)
                            elif isinstance(scstmt, ast.AsyncFunctionDef):
                                replace_async_func(scstmt)
                        indented_code = '\n'.join(['    ' + line for line in ast.unparse(cstmt).splitlines()])
                        subclasses.append(indented_code)
                    elif isinstance(cstmt, ast.AnnAssign):
                        nodes.append(cstmt.target.id)
                    elif isinstance(cstmt, ast.AsyncFunctionDef):
                        for arg in cstmt.args.args:
                            arg.annotation = update_annotation(arg.annotation)
                        replace_async_func(cstmt)
                        if isinstance(cstmt.returns, ast.Name) and cstmt.returns.id in nodes:
                            cstmt.returns = return_attribute(resource_name, cstmt.returns.id)
                        indented_code = '\n'.join(['    ' + line for line in ast.unparse(cstmt).splitlines()])
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

{9}
{8}


if __name__ == '__main__':
    asyncio.run(Module.run_from_registry())

'''.format(
        "\n".join(get_final_imports(imports)),
        resource_type,
        resource_subtype,
        model_name_pascal,
        resource_name,
        namespace,
        mod_name,
        model_name,
        '\n\n'.join([method for method in abstract_methods]),
        '\n\n'.join([subclass for subclass in subclasses])
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

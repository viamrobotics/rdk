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
                if isinstance(cstmt.target, ast.Name):
                    nodes.add(cstmt.target.id)
                nodes_to_remove.append(cstmt)
            elif isinstance(cstmt, ast.Assign):
                for target in cstmt.targets:
                    if isinstance(target, ast.Name):
                        nodes.add(target.id)
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


def wrap_do_command_region(methods_text: str) -> str:
    """Wrap each do_command body in inline-example comments. Other methods stay plain."""
    lines = methods_text.splitlines(keepends=True)
    out: List[str] = []
    index = 0
    while index < len(lines):
        line = lines[index]
        stripped = line.lstrip()
        if not (
            stripped.startswith("async def do_command(")
            or stripped.startswith("def do_command(")
        ):
            out.append(line)
            index += 1
            continue
        indent = line[: len(line) - len(stripped)]
        out.append(line)
        index += 1
        while index < len(lines) and not out[-1].rstrip().endswith(":"):
            out.append(lines[index])
            index += 1
        body: List[str] = []
        while index < len(lines):
            nxt = lines[index]
            if nxt.strip() == "":
                break
            if len(nxt) - len(nxt.lstrip()) <= len(indent):
                break
            body.append(nxt)
            index += 1
        body_indent = indent + "    "
        out.append(f"{body_indent}# inline-example:do-command\n")
        out.extend(body)
        out.append(f"{body_indent}# inline-example:do-command-end\n")
    return "".join(out)


def resource_import_line(
    resource_type: str, resource_subtype: str, resource_name: str
) -> str:
    """Named import of the base class. A star import leaves ``Generic`` unbound once removed."""
    return (
        f"from viam.{resource_type}s.{resource_subtype} import {resource_name}"
    )


def with_resource_import(
    imports_text: str, resource_type: str, resource_subtype: str, resource_name: str
) -> str:
    """Put the base-class import in the imports slot when there is one."""
    line = resource_import_line(resource_type, resource_subtype, resource_name)
    if line in imports_text.splitlines():
        return imports_text
    marker = "/*inline-slot:imports*/"
    if marker in imports_text:
        return imports_text.replace(marker, f"{marker}\n{line}", 1)
    if imports_text.strip() == "":
        return line
    return f"{line}\n{imports_text}"


def render_resource_file(
    imports_text: str,
    resource_type: str,
    resource_subtype: str,
    model_name_pascal: str,
    resource_name: str,
    namespace: str,
    mod_name: str,
    model_name: str,
    subclasses_text: str,
    methods_text: str,
    slots: bool,
) -> str:
    """Render one Python model file. slots adds the inline-bundle example regions."""
    imports_text = with_resource_import(
        imports_text, resource_type, resource_subtype, resource_name
    )
    example_imports = ""
    example_fields = ""
    init_body = "        return super().new(config, dependencies)"
    validate_body = "        return [], []"
    class_warning = ""
    if slots:
        # The inline editor shows this above the class. viam module generate does not.
        class_warning = (
            "# IMPORTANT: Do not change the class name or the MODEL triplet below.\n"
            "# The platform uses these auto-generated values to identify your module.\n"
            "# Changing them will break your inline module.\n"
        )
        example_imports = "# inline-example:imports\n# inline-example:imports-end\n"
        # The blank line after the fields region stays outside the markers, so a
        # filled field list is still separated from ``new``.
        example_fields = (
            "    # inline-example:fields\n    # inline-example:fields-end\n\n"
        )
        init_body = (
            "        # inline-example:init\n"
            "        return super().new(config, dependencies)\n"
            "        # inline-example:init-end"
        )
        validate_body = (
            "        # inline-example:validate\n"
            "        return [], []\n"
            "        # inline-example:validate-end"
        )
        methods_text = wrap_do_command_region(methods_text)
    resource_file = """from typing import ClassVar, Mapping, Sequence, Tuple
from typing_extensions import Self
from viam.proto.app.robot import ComponentConfig
from viam.proto.common import ResourceName
from viam.resource.base import ResourceBase
from viam.resource.easy_resource import EasyResource
from viam.resource.types import Model, ModelFamily
{0}
{10}
{14}class {3}({4}, EasyResource):
    # To enable debug-level logging, either run viam-server with the --debug option,
    # or configure your resource/machine to display debug logs.
    MODEL: ClassVar[Model] = Model(ModelFamily("{5}", "{6}"), "{7}")

{13}    @classmethod
    def new(cls, config: ComponentConfig, dependencies: Mapping[ResourceName, ResourceBase]) -> Self:
        \"\"\"This method creates a new instance of this {4} {1}.
        The default implementation sets the name from the `config` parameter.

        Args:
            config (ComponentConfig): The configuration for this resource
            dependencies (Mapping[ResourceName, ResourceBase]): The dependencies (both required and optional)

        Returns:
            Self: The resource
        \"\"\"
{11}

    @classmethod
    def validate_config(cls, config: ComponentConfig) -> Tuple[Sequence[str], Sequence[str]]:
        \"\"\"This method allows you to validate the configuration object received from the machine,
        as well as to return any required dependencies or optional dependencies based on that `config`.

        Args:
            config (ComponentConfig): The configuration for this resource

        Returns:
            Tuple[Sequence[str], Sequence[str]]: A tuple where the
                first element is a list of required dependencies and the
                second element is a list of optional dependencies
        \"\"\"
{12}

{8}
{9}
""".format(
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
        example_imports,
        init_body,
        validate_body,
        example_fields,
        class_warning,
    )
    if slots:
        old_class = f"class {model_name_pascal}({resource_name}, EasyResource):"
        new_class = (
            f"class {model_name_pascal}(/*inline-slot:bases*/{resource_name}, "
            "EasyResource/*inline-slot:bases-end*/):"
        )
        resource_file = resource_file.replace(old_class, new_class, 1)
    return resource_file


def annotation_names(node: ast.AST) -> Set[str]:
    """Names a type annotation mentions. Attribute access counts the root only."""
    if node is None:
        return set()
    if isinstance(node, ast.Name):
        return {node.id}
    if isinstance(node, ast.Attribute):
        return annotation_names(node.value)
    if isinstance(node, ast.Subscript):
        return annotation_names(node.value) | annotation_names(node.slice)
    if isinstance(node, (ast.Tuple, ast.List)):
        found: Set[str] = set()
        for elt in node.elts:
            found |= annotation_names(elt)
        return found
    if isinstance(node, ast.BinOp):
        return annotation_names(node.left) | annotation_names(node.right)
    return set()


def signature_names(func: ast.AST) -> Set[str]:
    if not isinstance(func, (ast.FunctionDef, ast.AsyncFunctionDef)):
        return set()
    found: Set[str] = set()
    args = [*func.args.posonlyargs, *func.args.args, *func.args.kwonlyargs]
    for arg in args:
        found |= annotation_names(arg.annotation)
    found |= annotation_names(func.returns)
    return found


def module_level_type_names(tree: ast.AST, resource_name: str) -> Set[str]:
    """Classes and aliases defined beside the resource, not on it.

    ``TickStream`` and ``CaptureAllResult`` live in the SDK file. The stub
    copies methods out of that file, so those names have to be imported or
    pyflakes reports them as undefined.
    """
    names: Set[str] = set()
    for stmt in tree.body:
        if isinstance(stmt, ast.ClassDef):
            if stmt.name != resource_name:
                names.add(stmt.name)
            continue
        if isinstance(stmt, ast.Assign):
            for target in stmt.targets:
                if isinstance(target, ast.Name):
                    names.add(target.id)
        elif isinstance(stmt, ast.AnnAssign) and isinstance(stmt.target, ast.Name):
            names.add(stmt.target.id)
    return names


def imports_without(lines: List[str], reserved: Set[str]) -> List[str]:
    """Drop names the scaffold header already binds, keeping the rest of the line."""
    kept: List[str] = []
    for line in lines:
        text = line.strip()
        if text.startswith("import "):
            module = ""
            body = text[len("import ") :]
            kind = "import"
        elif text.startswith("from ") and " import " in text:
            module, body = text.split(" import ", 1)
            kind = "from"
        else:
            kept.append(line)
            continue
        parts = []
        for part in body.split(","):
            part = part.strip()
            if part == "" or names_bound_by_import(f"from m import {part}") & reserved:
                continue
            parts.append(part)
        if not parts:
            continue
        if kind == "import":
            kept.append("import " + ", ".join(parts))
        else:
            kept.append(f"{module} import {', '.join(parts)}")
    return kept


def names_bound_by_import(line: str) -> Set[str]:
    text = line.strip()
    if text.startswith("import "):
        body = text[len("import ") :]
    elif text.startswith("from ") and " import " in text:
        body = text.split(" import ", 1)[1]
    else:
        return set()
    bound: Set[str] = set()
    for part in body.split(","):
        pieces = part.split()
        if not pieces or pieces[0] in {"*", "("}:
            continue
        bound.add(pieces[-1] if "as" in pieces else pieces[0].split(".")[0])
    return bound


def same_file_stub(
    source: str, resource_name: str, module_name: str
) -> tuple:
    """Imports, nested classes, and methods copied from one SDK module file.

    A class attribute such as ``AudioIn.AudioStream`` is qualified. A name
    defined next to the class, such as ``TickStream`` or ``Control``, is
    imported from this module.
    """
    tree = ast.parse(source)
    imports: List[str] = []
    subclasses: List[str] = []
    abstract_methods: List[str] = []
    nodes: Set[str] = set()
    modules_to_ignore = [
        "abc",
        "component_base",
        "service_base",
        "viam.resource.types",
    ]
    module_types = module_level_type_names(tree, resource_name)
    referenced: Set[str] = set()
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
                elif isinstance(cstmt, ast.AnnAssign) and isinstance(
                    cstmt.target, ast.Name
                ):
                    nodes.add(cstmt.target.id)
                elif isinstance(cstmt, ast.Assign):
                    for target in cstmt.targets:
                        if isinstance(target, ast.Name):
                            nodes.add(target.id)
            for cstmt in stmt.body:
                if isinstance(cstmt, ast.ClassDef) and is_abstract_class(cstmt):
                    subclasses.append(return_subclass(resource_name, cstmt))
                    for node in ast.walk(cstmt):
                        referenced |= signature_names(node)
                elif isinstance(cstmt, ast.AsyncFunctionDef):
                    replace_async_func(resource_name, cstmt, nodes)
                    referenced |= signature_names(cstmt)
                    indented_code = "\n".join(
                        ["    " + line for line in ast.unparse(cstmt).splitlines()]
                    )
                    abstract_methods.append(indented_code)
    bound: Set[str] = set()
    for line in imports:
        bound |= names_bound_by_import(line)
    missing = sorted(
        name
        for name in referenced
        if name in module_types and name not in bound
    )
    if missing:
        imports.append(f"from {module_name} import {', '.join(missing)}")
    return imports, subclasses, abstract_methods


def sdk_module_candidates(resource_subtype: str) -> List[str]:
    """Python package names to try for a resource subtype.

    movement_sensor stays movement_sensor. world_state_store is published as
    worldstatestore, so the underscored name is tried first and the name with
    underscores removed is the fallback.
    """
    names = [resource_subtype]
    stripped = resource_subtype.replace("_", "")
    if stripped != resource_subtype:
        names.append(stripped)
    return names


def load_sdk_module(resource_type: str, resource_subtype: str):
    """Import the SDK module that defines this resource's class."""
    last_error = None
    for name in sdk_module_candidates(resource_subtype):
        module_name = f"viam.{resource_type}s.{name}.{name}"
        try:
            return import_module(module_name)
        except ModuleNotFoundError as exc:
            missing = exc.name or ""
            # A missing parent of this candidate is the name mismatch. A missing
            # dependency of a module that was found is a real failure.
            if missing != module_name and not module_name.startswith(missing + "."):
                raise
            last_error = exc
    if last_error is None:
        raise ModuleNotFoundError(resource_subtype)
    raise last_error


def main(
    resource_type: str,
    resource_subtype: str,
    namespace: str,
    mod_name: str,
    model_name: str,
    slots: bool = False,
) -> str:
    from slugify import slugify

    module = load_sdk_module(resource_type, resource_subtype)
    module_name = module.__name__
    # The published package, not the resource subtype: world_state_store loads
    # from worldstatestore.
    package = module_name.rsplit(".", 1)[-1]
    resource_name = {"input": "Controller", "slam": "SLAM", "mlmodel": "MLModel"}.get(
        resource_subtype,
        "".join(word.capitalize() for word in resource_subtype.split("_")),
    )

    # Names the scaffold header imports itself. Copying one again from the SDK
    # file (arm imports collections.abc.Mapping) is a redefinition. Slot mode does
    # not run ruff, so the filter has to happen here.
    scaffold_names = {
        "ClassVar",
        "Mapping",
        "Sequence",
        "Tuple",
        "Self",
        "ComponentConfig",
        "ResourceName",
        "ResourceBase",
        "EasyResource",
        "Model",
        "ModelFamily",
    }
    with open(module.__file__, "r") as f:
        imports, subclasses, abstract_methods = same_file_stub(
            f.read(), resource_name, module_name
        )
    imports = imports_without(imports, scaffold_names)

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
    # package, not the resource subtype: world_state_store lives in worldstatestore.
    resource_file = render_resource_file(
        imports_text,
        resource_type,
        package,
        model_name_pascal,
        resource_name,
        namespace,
        mod_name,
        model_name,
        subclasses_text,
        methods_text,
        slots,
    )
    if slots:
        return resource_file
    with tempfile.NamedTemporaryFile(mode="w+", suffix=".py", delete=False) as f:
        f_name = f.name
        f.write(resource_file)
    try:
        try:
            # --isolated so the output matches the module's own ruff config rather than
            # whatever ruff config surrounds the directory the CLI runs in.
            subprocess.check_call(
                [
                    sys.executable,
                    "-m",
                    "ruff",
                    "check",
                    "--isolated",
                    "--select",
                    "F401,F811,I",
                    "--fix",
                    "--fix-only",
                    "--quiet",
                    f_name,
                ]
            )
            subprocess.check_call(
                [sys.executable, "-m", "ruff", "format", "--isolated", "--quiet", f_name]
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


def _assert_same_file_types() -> None:
    """Method signatures keep class attributes and import sibling types."""
    source = """
import abc
from typing import Dict, List

class Control:
    pass

class Event:
    pass

TickStream = dict

class CaptureAllResult:
    pass

class AudioIn:
    AudioStream = dict

    async def get_audio(self) -> AudioStream:
        raise NotImplementedError()

class Board:
    async def stream_ticks(self) -> TickStream:
        raise NotImplementedError()

class Controller:
    async def get_controls(self) -> List[Control]:
        raise NotImplementedError()

    async def get_events(self) -> Dict[Control, Event]:
        raise NotImplementedError()

class Vision:
    async def capture_all_from_camera(self) -> CaptureAllResult:
        raise NotImplementedError()
"""
    _, _, audio_methods = same_file_stub(
        source, "AudioIn", "viam.components.audio_in.audio_in"
    )
    audio = "\n".join(audio_methods)
    if "AudioIn.AudioStream" not in audio or re_bare("AudioStream", audio):
        raise SystemExit("AudioStream should be qualified as AudioIn.AudioStream")

    board_imports, _, board_methods = same_file_stub(
        source, "Board", "viam.components.board.board"
    )
    if "from viam.components.board.board import TickStream" not in board_imports:
        raise SystemExit(f"board imports: {board_imports}")
    if "TickStream" not in "\n".join(board_methods):
        raise SystemExit("board method dropped TickStream")

    input_imports, _, _ = same_file_stub(
        source, "Controller", "viam.components.input.input"
    )
    input_import = "from viam.components.input.input import Control, Event"
    if input_import not in input_imports:
        raise SystemExit(f"input imports: {input_imports}")

    vision_imports, _, _ = same_file_stub(
        source, "Vision", "viam.services.vision.vision"
    )
    vision_import = "from viam.services.vision.vision import CaptureAllResult"
    if vision_import not in vision_imports:
        raise SystemExit(f"vision imports: {vision_imports}")


def re_bare(name: str, text: str) -> bool:
    import re

    return re.search(rf"(?<!\.)\b{name}\b", text) is not None


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--self-test":
        _assert_same_file_types()
        sys.exit(0)
    if len(sys.argv) > 1 and sys.argv[1] == "--bundle":
        run_bundle(sys.argv[2])
        sys.exit(0)
    packages = [
        sdk_requirement("mlmodel" if sys.argv[2] == "mlmodel" else ""),
        "typing-extensions",
        "ruff",
        "python-slugify",
    ]
    if sys.argv[2] == "mlmodel":
        packages.append("numpy")
    install_requirements(packages)
    result = main(sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5])
    print(result, end="")

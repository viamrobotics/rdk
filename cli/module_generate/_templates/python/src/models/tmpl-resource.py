from typing import ClassVar, Mapping, Sequence, Self, Tuple
from viam.proto.app.robot import ComponentConfig
from viam.proto.common import ResourceName
from viam.resource.base import ResourceBase
from viam.resource.easy_resource import EasyResource
from viam.resource.types import Model, ModelFamily
from viam.{{ .ResourceType }}s.{{ .ResourceSubtype }} import {{ .ResourceSubtypePascal }}


class {{ .ModelPascal  }}({{ .ResourceSubtypePascal }}, EasyResource):
    MODEL: ClassVar[Model] = "{{ .ModelTriple }}"

    @classmethod
    def new(cls, config: ComponentConfig, dependencies: Mapping[ResourceName, ResourceBase]) -> Self:
        """This method creates a new instance of this {{ .ResourceSubtype }}.
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

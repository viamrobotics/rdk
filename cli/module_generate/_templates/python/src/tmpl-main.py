import asyncio
from viam.module.module import Module
try:
    from models.{{ .ModelSnake }} import {{ .ModelPascal }}
except ModuleNotFoundError:
    # when running as local module
    from .models.{{ .ModelSnake }} import {{ .ModelPascal }}


if __name__ == '__main__':
    asyncio.run(Module.run_from_registry())

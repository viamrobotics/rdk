import asyncio
from viam.module.module import Module
from .{{ .ModelSnake }} import {{ .ModelPascal }}


if __name__ == '__main__':
    asyncio.run(Module.run_from_registry())

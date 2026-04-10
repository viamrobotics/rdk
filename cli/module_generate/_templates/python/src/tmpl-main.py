import asyncio
from viam.module.module import Module
from models.{{ .ModelSnake }} import {{ .ModelPascal }} as {{ .ModelPascal }}Model


if __name__ == '__main__':
    asyncio.run(Module.run_from_registry())

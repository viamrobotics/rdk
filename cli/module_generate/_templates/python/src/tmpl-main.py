import asyncio
import os
import sys
from viam.module.module import Module
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from models.{{ .ModelSnake }} import {{ .ModelPascal }}


if __name__ == '__main__':
    asyncio.run(Module.run_from_registry())

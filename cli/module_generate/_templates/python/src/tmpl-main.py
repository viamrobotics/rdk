import asyncio
import os
import sys
from viam.module.module import Module
try:
    from models.hello_sensor import HelloSensor
except ModuleNotFoundError:
    from .models.hello_sensor import HelloSensor


if __name__ == '__main__':
    asyncio.run(Module.run_from_registry())

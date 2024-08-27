#!/bin/bash

cd cli/modulegen/ && \
python3 -m venv .venv && \
source .venv/bin/activate && \
pip install -r requirements.txt && \
poetry run pyinstaller --onefile --collect-data cookiecutter --add-data module_generator/module:module --hidden-import cookiecutter.extensions module_generator/__main__.py

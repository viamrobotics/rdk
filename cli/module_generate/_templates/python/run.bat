@echo off
cd /d "%~dp0"
set VENV_NAME=venv
set PYTHON=%VENV_NAME%\Scripts\python.exe
%PYTHON% src\main.py %*

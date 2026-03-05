@echo off
cd /d "%~dp0"

REM Create a virtual environment to run our code
set VENV_NAME=venv
set PYTHON=%VENV_NAME%\Scripts\python.exe

%PYTHON% -m pip install pyinstaller -Uqq
if errorlevel 1 exit /b 1

%PYTHON% -m PyInstaller --onefile --hidden-import="googleapiclient" src/main.py
tar -czvf dist/archive.tar.gz meta.json ./dist/main.exe
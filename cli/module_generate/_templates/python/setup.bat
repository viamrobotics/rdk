@echo off
cd /d "%~dp0"

REM Create a virtual environment to run our code
set VENV_NAME=venv
set PYTHON=%VENV_NAME%\Scripts\python.exe
set ENV_ERROR=This module requires Python >=3.8, pip, and virtualenv to be installed.

python -m venv %VENV_NAME% >nul 2>&1
if errorlevel 1 (
    echo Failed to create virtualenv.
    echo %ENV_ERROR%
    exit /b 1
)

REM remove -U if viam-sdk should not be upgraded whenever possible
REM -qq suppresses extraneous output from pip
echo Virtualenv found/created. Installing/upgrading Python packages...
if not exist .installed (
    "%PYTHON%" -m pip install -r requirements.txt -Uqq
    if errorlevel 1 (
        exit /b 1
    ) else (
        type nul > .installed
    )
)
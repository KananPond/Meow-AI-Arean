@echo off
rem ---------------------------------------------------------------------------
rem Desert Arena referee desk launcher.
rem KEEP THIS FILE PURE ASCII AND CRLF.
rem If a .bat carries non-ASCII bytes, cmd.exe loses its byte offset after
rem "chcp 65001" and starts executing fragments of lines - the window shows
rem errors like 'eferee_desk.py" ...' and the desk never opens.
rem ---------------------------------------------------------------------------
setlocal
chcp 65001 >nul
cd /d "%~dp0"

set "PY="
call :pick "py -3"
if not defined PY call :pick "python"
if not defined PY goto :nopython

%PY% -X utf8 "referee_desk.py" %*
if errorlevel 1 goto :failed
goto :end

:pick
rem Accept the candidate only if it can import tkinter.
%~1 -X utf8 -c "import tkinter" >nul 2>nul
if errorlevel 1 goto :eof
set "PY=%~1"
goto :eof

:nopython
echo.
echo [referee_desk] No Python with tkinter found.
echo   * Install Python 3 for Windows from python.org, then double-click again.
echo   * Or run manually: py -3 -X utf8 referee_desk.py
echo.
if "%~1"=="" pause
goto :end

:failed
echo.
echo [referee_desk] The referee desk exited with an error, see the messages above.
echo.
if "%~1"=="" pause

:end
endlocal

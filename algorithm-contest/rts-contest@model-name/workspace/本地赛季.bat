@echo off
rem ASCII only - see the note in the local-match script.
chcp 65001 >nul
setlocal
cd /d "%~dp0"

echo ============================================================
echo   Desert Arena - local season
echo   plan  ->  run  ->  rate (BT rating + weighted score)
echo ============================================================
echo.

go build -o bot.exe .\submission
if errorlevel 1 (
  echo [x] Build failed. Fix the errors above.
  pause
  exit /b 1
)

pushd arena

echo.
echo [1/3] Freezing schedule ...
go run ./cmd/arena plan -config ..\tournament\local.json
if errorlevel 1 goto :done

echo.
echo [2/3] Running all matches ...
go run ./cmd/arena run -config ..\tournament\local.json -skip-seal
if errorlevel 1 goto :done

echo.
echo [3/3] Rating ...
go run ./cmd/arena rate -config ..\tournament\local.json

:done
popd
echo.
echo Results: matches\season\  (standings + replays)
pause

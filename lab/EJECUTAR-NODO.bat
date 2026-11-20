@echo off
rem Nodo del cluster de aire-distribuido: doble clic en cada equipo del lab.
rem Copia nodo.exe a %USERPROFILE%\aire-nodo, lo lanza en su propia ventana y
rem termina, asi el USB se puede sacar y llevar al siguiente equipo.
rem El nodo llama al coordinador de coordinador.txt y vuelve a llamar solo
rem entre corridas. Se detiene con Ctrl+C o cerrando su ventana.
rem No necesita administrador: el nodo solo abre conexiones de salida.
setlocal
set "DESTINO=%USERPROFILE%\aire-nodo"
set "COORD="
if exist "%~dp0coordinador.txt" set /p COORD=<"%~dp0coordinador.txt"
if "%COORD%"=="" goto :sincoord
if not exist "%DESTINO%" mkdir "%DESTINO%"
copy /Y "%~dp0nodo.exe" "%DESTINO%\" >nul || goto :error
start "nodo aire-distribuido" /D "%DESTINO%" cmd /k nodo.exe -coordinador %COORD%
echo.
echo   Listo: el nodo llama a %COORD% en su propia ventana.
echo   Ya puedes sacar el USB.
echo.
timeout /t 5 >nul
exit /b 0
:sincoord
echo   ERROR: falta coordinador.txt (una linea con IP:puerto del coordinador)
pause
exit /b 1
:error
echo   ERROR: no pude copiar nodo.exe a %DESTINO%
pause
exit /b 1

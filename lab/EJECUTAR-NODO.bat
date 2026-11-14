@echo off
rem Nodo del cluster de aire-distribuido: doble clic en cada equipo del lab.
rem Copia nodo.exe y el gold (oefa_features.csv) a %USERPROFILE%\aire-nodo, lanza
rem el nodo en su propia ventana y termina, asi el USB se puede sacar y llevar
rem al siguiente equipo. El nodo busca el gold junto a nodo.exe.
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
if exist "%~dp0oefa_features.csv" goto :congold
echo   AVISO: falta oefa_features.csv en el USB: el nodo solo sirve para el smoke test.
goto :lanzar
:congold
copy /Y "%~dp0oefa_features.csv" "%DESTINO%\" >nul || goto :error
:lanzar
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
echo   ERROR: no pude copiar nodo.exe o el gold a %DESTINO%
echo   Si ya hay un nodo abierto en este equipo, cierra su ventana y vuelve a intentar.
pause
exit /b 1

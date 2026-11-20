@echo off
rem K-Means del gold de aire-distribuido en la GPU de este PC (experimento de la PC4).
rem La primera vez baja de la laptop el Python portable con PyTorch CUDA (unos GB por
rem la red del lab) y lo deja en %LOCALAPPDATA%\aire-gpu; despues solo baja el script
rem y los datos, corre, abre nvidia-smi en otra ventana y sube el resultado.
rem No necesita administrador ni instala nada en el sistema.
rem Para borrarlo todo: rmdir /s /q "%LOCALAPPDATA%\aire-gpu"
setlocal
set "SERVIDOR=http://10.11.37.73:8090"
set "DESTINO=%LOCALAPPDATA%\aire-gpu"
if not exist "%DESTINO%" mkdir "%DESTINO%"
cd /d "%DESTINO%" || goto :error

rem python\listo.txt se escribe solo si la descompresion termino: si se corto a
rem medias, la siguiente vez se vuelve a bajar y descomprimir.
if exist "python\listo.txt" goto :datos
echo.
echo   Bajando Python + PyTorch desde %SERVIDOR% (unos GB por la red del lab)...
curl.exe -fS --progress-bar -o paquete.zip "%SERVIDOR%/python-torch.zip" || goto :error
echo   Descomprimiendo...
if exist python rmdir /s /q python
tar -xf paquete.zip || goto :error
del paquete.zip
echo ok> python\listo.txt

:datos
for %%f in (kmeans_gpu.py oefa_features.csv kmeans.json centroides_iniciales.json) do (
  curl.exe -fsS -o %%f "%SERVIDOR%/%%f" || goto :error
)
start "GPU - nvidia-smi" cmd /k nvidia-smi -l 1
set PYTHONUTF8=1
python\python.exe kmeans_gpu.py --gold oefa_features.csv --kmeans kmeans.json --inicios centroides_iniciales.json --salida resultado.json %*
if errorlevel 1 goto :error
curl.exe -fsS -T resultado.json "%SERVIDOR%/resultados/kmeans_gpu_%COMPUTERNAME%.json" && echo   Resultado enviado a la laptop.
echo.
pause
exit /b 0

:error
echo.
echo   ERROR: algo fallo; copia lo de arriba y mandaselo a quien corre la demo.
pause
exit /b 1

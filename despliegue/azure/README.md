# Despliegue en una máquina virtual de Azure

El sistema completo (API, nodos, MongoDB, Redis y seed) en una sola máquina virtual chica, con
HTTPS. Es el mismo Docker Compose del repo con un override encima; no cambia código.

Para la entrega del curso no hace falta: el enunciado pide Docker Compose y todo corre en una
laptop con `docker compose up`. Esto sirve para tener un enlace público en la sustentación.

## Qué hay en esta carpeta

| Archivo | Para qué |
|---|---|
| `compose.azure.yml` | Override de Compose: el servicio `web` (el Caddy de `web/Caddyfile`) recibe el tráfico de internet con HTTPS, sirve la UI y pasa lo demás a la API; MongoDB con la caché acotada, límites de memoria, reinicio solo y ningún puerto interno publicado |
| `cloud-init.yaml` | Deja la máquina lista al crearla: Docker, 2 GiB de swap y el repo |

## Cuánto ocupa

Medido con 2 nodos, después de un entrenamiento y de predicciones:

| Servicio | Memoria |
|---|---|
| MongoDB (caché de 0,25 GiB) | 160 a 215 MB |
| API | 17 a 31 MB |
| Caddy (servicio `web`, con la UI) | unos 57 MB |
| Cada nodo | 14 MB |
| Redis | 11 a 13 MB |

Entra en 1 GiB de RAM con swap. La imagen pesa 59 MB y el gold 5 MB.

## Crear la máquina

Con Azure for Students (correo de la universidad, sin tarjeta), el tamaño gratuito es
`Standard_B2ats_v2` (2 vCPU, 1 GiB): 750 horas al mes durante 12 meses. Tres cosas que no son
obvias:

- **La región.** La suscripción de estudiante solo deja crear recursos en algunas regiones, y el
  tamaño gratuito no está en todas. Se comprueba con
  `az vm list-skus -l <región> --size Standard_B2ats --all -o table` (tarda un minuto por región).
  En noviembre de 2026 estaba en `canadacentral` y no en `eastus2`.
- **El disco.** El gratuito es el P6: 64 GiB, `Premium_LRS`. Uno más grande se cobra.
- **La IP pública** estándar y estática puede consumir crédito.

```bash
SUB="<id de la suscripción Azure for Students>"
RG=aire-distribuido-rg; VM=aire-vm; LOC=canadacentral
DNS=aire-distribuido-$(openssl rand -hex 3)      # queda $DNS.$LOC.cloudapp.azure.com

ssh-keygen -t ed25519 -N "" -f ~/.ssh/aire-azure_ed25519

az group create --subscription $SUB -n $RG -l $LOC
az vm create --subscription $SUB -g $RG -n $VM \
  --image Ubuntu2404 --size Standard_B2ats_v2 \
  --admin-username azureuser --ssh-key-values ~/.ssh/aire-azure_ed25519.pub \
  --os-disk-size-gb 64 --storage-sku Premium_LRS \
  --public-ip-sku Standard --public-ip-address-dns-name $DNS \
  --nsg-rule NONE --custom-data despliegue/azure/cloud-init.yaml

# Solo SSH desde la IP propia, y la web para todos. Nada más.
az network nsg rule create --subscription $SUB -g $RG --nsg-name ${VM}NSG -n ssh \
  --priority 1000 --access Allow --protocol Tcp --direction Inbound \
  --source-address-prefixes "$(curl -s https://api.ipify.org)/32" --destination-port-ranges 22
az network nsg rule create --subscription $SUB -g $RG --nsg-name ${VM}NSG -n web \
  --priority 1010 --access Allow --protocol Tcp --direction Inbound \
  --source-address-prefixes Internet --destination-port-ranges 80 443
```

Con más de una suscripción en la CLI, pasar siempre `--subscription`: así no se crea nada en la
equivocada.

`cloud-init.yaml` tiene que quedar **solo en ASCII**. Azure altera los caracteres con tilde del
`--custom-data` y cloud-init descarta el archivo entero: `cloud-init status --long` dice
«empty cloud config» y la máquina queda sin Docker. Si pasa, los mismos pasos se corren a mano:

```bash
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
echo "/swapfile none swap sw 0 0" | sudo tee -a /etc/fstab
sudo apt-get update && sudo apt-get install -y docker.io docker-compose-v2 git
sudo usermod -aG docker azureuser
git clone --branch develop https://github.com/R0SEWT/aire-distribuido.git ~/aire-distribuido
```

## Levantar el sistema

La imagen se construye en la laptop y se manda: compilar Go en 1 GiB de RAM es lento.

```bash
HOST=azureuser@$DNS.$LOC.cloudapp.azure.com; LLAVE=~/.ssh/aire-azure_ed25519

docker build -t aire-distribuido .
docker save aire-distribuido | gzip | ssh -i $LLAVE $HOST 'gunzip | sudo docker load'
scp -i $LLAVE data/gold/oefa_features.csv $HOST:aire-distribuido/data/gold/     # el gold no se versiona
```

En la máquina, el `.env` con secretos propios (no los del ejemplo) y el nombre público:

```bash
cd ~/aire-distribuido && umask 077
{ echo "JWT_SECRETO=$(openssl rand -hex 32)"
  echo "API_USUARIOS=admin:$(openssl rand -hex 9):admin,demo:$(openssl rand -hex 9):usuario"
  echo "DOMINIO=<el nombre público de la máquina>"
  echo "CORS_ORIGENES="; } > .env

sudo docker compose -f docker-compose.yml -f despliegue/azure/compose.azure.yml up -d --wait --scale nodo=2
```

Caddy saca el certificado de Let's Encrypt solo, en el primer pedido. La UI y la API quedan en el
mismo origen (`https://$DOMINIO`), así que `CORS_ORIGENES` va vacío.

La imagen `aire-distribuido-web` no se construye en la máquina (compilar la UI necesita Node y más
memoria de la que tiene): se construye en la laptop y se manda, igual que la de la API.

```bash
# en la laptop, desde la raíz del repo
docker build -t aire-distribuido-web web
docker save aire-distribuido-web | gzip | ssh azureuser@$DOMINIO 'gunzip | sudo docker load'
```

## Comprobar

```bash
curl -s https://$DOMINIO/api/salud        # api, mongo y redis en ok, y los nodos en el pool
```

La UI queda en `https://$DOMINIO/` y Swagger UI en `https://$DOMINIO/docs`. Desde afuera solo responden el 80 y el 443 (y el 22
desde la IP autorizada): el puerto de los nodos (7070), la API directa (8080), MongoDB y Redis no
se publican. El puerto de los nodos no tiene autenticación, así que no se abre nunca.

## La latencia desde lejos

El requisito de menos de 100 ms se mide donde corre el sistema. Contra esta máquina, desde Lima,
una predicción tarda unos 150 ms con la conexión ya abierta: es el viaje de ida y vuelta hasta
Canadá, no el servidor, que la resuelve en menos de 1 ms (el campo `ms`). Las capturas de Postman
y `cmd/carga` del informe se hacen contra el Compose local.

## Actualizar, apagar y borrar

```bash
# Actualizar: mandar la imagen nueva (como arriba) y, en la máquina:
git pull && sudo docker compose -f docker-compose.yml -f despliegue/azure/compose.azure.yml up -d --scale nodo=2

# Apagar sin borrar (deja de contar horas de cómputo; el disco y la IP siguen):
az vm deallocate --subscription $SUB -g $RG -n $VM

# Borrar todo:
az group delete --subscription $SUB -n $RG
```

## Límites conocidos

- La ventana de 24 h de cada estación vive en memoria: si la API se reinicia, las alertas vuelven
  a empezar hasta juntar 18 lecturas.
- Con 1 GiB conviene no pasar de 2 nodos ni lanzar entrenamientos con muchas réplicas.
- MongoDB y Redis no tienen contraseña: quedan protegidos por no estar publicados.

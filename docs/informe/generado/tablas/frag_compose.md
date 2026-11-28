<!-- Generado por tablas.py desde docker-compose.yml. No editar a mano. -->

::: {#lst-compose}
```yaml
services:
  api:
    build: .
    image: aire-distribuido
    command: ["api", "-addr", ":8080", "-cola", "32768", "-nodos-addr", ":7070", "-gold", "/datos/gold/oefa_features.csv",
              "-jwt-duracion", "${JWT_DURACION:-1h}"]
    expose:
      - "7070"
    volumes:
      - ${GOLD_DIR:-./data/gold}:/datos/gold:ro
    stop_grace_period: 20s
    environment:
      JWT_SECRETO: ${JWT_SECRETO:-}
      API_USUARIOS: ${API_USUARIOS:-}
      API_INVITADO: ${API_INVITADO:-}
      MONGO_URI: mongodb://mongo:27017
      MONGO_DB: aire_distribuido
      REDIS_ADDR: redis:6379
    ports:
      - "127.0.0.1:${API_PORT:-8080}:8080"
    depends_on:
      mongo:
        condition: service_healthy
      redis:
        condition: service_healthy
      seed:
        condition: service_completed_successfully
    healthcheck:
      test: ["CMD", "api", "-salud"]
      interval: 5s
      timeout: 4s
      retries: 6
      start_period: 5s

  web:
    build: ./web
    image: aire-distribuido-web
    ports:
      - "127.0.0.1:${WEB_PORT:-8081}:80"
    depends_on:
      api:
        condition: service_healthy

  nodo:
    build: .
    image: aire-distribuido
    command: ["nodo", "-coordinador", "api:7070", "-gold", "/datos/gold/oefa_features.csv"]
    volumes:
      - ${GOLD_DIR:-./data/gold}:/datos/gold:ro
    depends_on:
      api:
        condition: service_healthy
    stop_signal: SIGINT

  mongo:
    image: mongo:8.0.38
    ports:
      - "127.0.0.1:${MONGO_PORT:-27017}:27017"
    volumes:
      - mongo_data:/data/db
    healthcheck:
      test: ["CMD", "mongosh", "--quiet", "--eval", "quit(db.adminCommand('ping').ok ? 0 : 1)"]
      interval: 5s
      timeout: 3s
      retries: 12
      start_period: 10s

  seed:
    image: mongo:8.0.38
    entrypoint: ["mongosh"]
    command: ["--quiet", "mongodb://mongo:27017/aire_distribuido", "--file", "/seed/seed.js"]
    environment:
      MONGO_INITDB_DATABASE: aire_distribuido
      ESTACIONES_CSV: /seed/estaciones_ficha.csv
    volumes:
      - ./docs/datos/seed.js:/seed/seed.js:ro
      - ./reports/estaciones_ficha.csv:/seed/estaciones_ficha.csv:ro
    depends_on:
      mongo:
        condition: service_healthy
    restart: "no"

  redis:
    image: redis:7.4.12-alpine
    command: ["redis-server", "--appendonly", "yes"]
    ports:
      - "127.0.0.1:${REDIS_PORT:-6379}:6379"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "-e", "ping"]
      interval: 5s
      timeout: 3s
      retries: 12
      start_period: 5s

volumes:
  mongo_data:
  redis_data:
```

El despliegue completo con Docker Compose (`docker-compose.yml`, completo, sin sus comentarios)
:::

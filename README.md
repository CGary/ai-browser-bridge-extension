# AI Browser Bridge Extension (aibbe)

## Propósito

Puente de comunicación entre un daemon Go y una extensión Chromium (Manifest V3). Permite a agentes externos automatizar interacciones con el navegador mediante Native Messaging.

## Arquitectura

```
┌─────────┐    Unix Socket    ┌────────────┐    Native Messaging    ┌──────────────────────┐
│  CLI    │ ──────────────► │  Daemon    │ ────────────────────► │  Chrome Extension    │
│ cmd/cli │    JSON/IPC      │  daemon/   │    4-byte LE + JSON   │  extension/          │
│         │ ◄────────────── │            │ ◄──────────────────── │                      │
└─────────┘   ipc.Response   └────────────┘                       │  background.js       │
                                                                   │  (Service Worker)    │
                                                                   └──────────────────────┘
```

## Componentes

| Componente | Descripción |
|------------|-------------|
| `cmd/cli/main.go` | Cliente CLI que envía comandos al daemon |
| `daemon/main.go` | Demonio que escucha en Unix socket y reenvía via Native Messaging |
| `extension/manifest.json` | Manifest V3 con ID estático |
| `extension/background.js` | Service Worker que conecta al native host |

## Protocolo de Comunicación

1. **CLI → Daemon**: JSON via Unix socket (`ipc.Request{Cmd, Payload}`)
2. **Daemon → Extension**: 4-byte LE length prefix + JSON (Native Messaging wire format)
3. **Daemon → CLI**: JSON response (`ipc.Response{Status}`)

## Extensión Chromium

- **Version**: 0.1.0
- **ID estático**: `bedlojjaiogmaefoadfpdecgajipcpgj`
- **Permisos**: `nativeMessaging`
- **Native host**: `aibbe`

## Uso

### Requisitos Previos

- Go 1.21+
- Chromium/Chrome o navegador basado en Chromium
- Sistema operativo Unix (Linux/macOS)

### Compilación

```bash
# Compilar daemon
go build -o daemon/aibbe ./daemon/

# Compilar CLI
go build -o aibbe-cli ./cmd/cli/
```

### Configuración del Native Messaging Host

1. Copiar el manifiest a la ubicación de Chromium:

```bash
mkdir -p ~/.config/chromium/NativeMessagingHosts/
cp configs/aibbe.nm-host.json ~/.config/chromium/NativeMessagingHosts/aibbe.json
```

2. Actualizar la ruta del binario en el manifest si es diferente:

```bash
# Editar ~/.config/chromium/NativeMessagingHosts/aibbe.json
# Cambiar "path" a la ruta absoluta del binario compilado
```

### Cargar la Extensión (Sideload)

1. Abrir `chrome://extensions`
2. Activar "Modo de desarrollador" (esquina superior derecha)
3. Clic en "Cargar descomprimida"
4. Seleccionar el directorio `extension/`

### Variables de Entorno

| Variable | Default | Descripción |
|----------|---------|-------------|
| `AIBBE_SOCKET_PATH` | `/tmp/aibbe.sock` | Ruta del socket Unix |

### Ejecución

```bash
# Iniciar daemon (en terminal separada)
go run daemon/main.go

# Enviar comando desde CLI
go run cmd/cli/main.go -cmd "query" -payload "hello"

# O con binario compilado
./aibbe-cli -cmd "query" -payload "hello"
```

### Comandos Disponibles

| Cmd | Descripción |
|-----|-------------|
| `echo` | Eco de respuesta (extensión) |
| (extensible) | Nuevos comandos según extienda la extensión |

### Resolución de Problemas

- **Error "native messaging host has not registered"**: Verificar que el manifest esté en `~/.config/chromium/NativeMessagingHosts/aibbe.json`
- **Error de conexión al socket**: Verificar que el daemon esté ejecutándose
- **Permiso denegado en socket**: El daemon crea el socket con permisos 0600 (solo propietario)
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

```bash
# Iniciar daemon
go run daemon/main.go

# Enviar comando desde CLI
go run cmd/cli/main.go -cmd "query" -payload "hello"
```
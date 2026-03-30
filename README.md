# AI Browser Bridge Extension (Manifest V3)

## 🎯 Propósito
Esta extensión actúa como una capa de abstracción (Adapter Pattern) entre el navegador y software externo (Agentes). Su función principal es interactuar con el DOM de plataformas de IA (ChatGPT, Claude, Gemini, etc.) para automatizar consultas y recuperar respuestas sin intervención humana directa.

## 🏗️ Arquitectura de Comunicación
Para garantizar la integración con software externo, la extensión implementa dos canales:
1. **Native Messaging API:** Comunicación bidireccional con aplicaciones locales (Python, Go, Node.js).
2. **Dynamic Selector Engine:** Sistema de mapeo basado en JSON para adaptar la lógica de raspado (scraping) y entrada de datos sin modificar el núcleo de la extensión ante cambios en la UI de las IAs.

## 📂 Estructura del Repositorio
```text
├── manifest.json          # Permisos de 'nativeMessaging' y 'host_permissions'.
├── background/
│   └── bridge_handler.js  # Gestión de mensajes entre el agente y el navegador.
├── content_scripts/
│   ├── adapters/          # Lógica específica por plataforma (chatgpt.js, claude.js).
│   └── injector.js        # Motor de inyección de prompts.
├── config/
│   └── selectors.json     # Mapeo de selectores CSS/XPath actualizables.
└── host/
    └── install_host.sh    # Script para registrar el host de mensajería nativa.
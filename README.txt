███╗   ███╗ ██████╗ ███╗   ██╗ ██████╗ ██╗     ██╗████████╗██╗  ██╗
████╗ ████║██╔═══██╗████╗  ██║██╔═══██╗██║     ██║╚══██╔══╝██║  ██║
██╔████╔██║██║   ██║██╔██╗ ██║██║   ██║██║     ██║   ██║   ███████║
██║╚██╔╝██║██║   ██║██║╚██╗██║██║   ██║██║     ██║   ██║   ██╔══██║
██║ ╚═╝ ██║╚██████╔╝██║ ╚████║╚██████╔╝███████╗██║   ██║   ██║  ██║
╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝ ╚══════╝╚═╝   ╚═╝   ╚═╝  ╚═╝


  ░▒▓█ _monolink_ █▓▒░
  Shared WebSocket client and wire helpers for MONOLITH services.

  ───────────────────────────────────────────────────────────────
  ▓ OVERVIEW
  **monolink** is a **Go library** (not a standalone binary) used by MONOLITH **nodes** and clients.
  ▪ Connects to **concentrator** over WebSocket (`ws://` or `wss://` with optional mTLS)
  ▪ Parses and emits the shared DSKY-style wire format (`TO:VERB:NOUN[:ARGS]:FROM`)
  ▪ Ships **governor**, **achtung**, **monoview**, and any other hub peers in the superproject

  ───────────────────────────────────────────────────────────────
  ▓ ARCHITECTURE
  ▪ **RUNTIME**: Go 1.24+ (see `go.mod`)
  ▪ **MODULE**: `github.com/mzh/monolink` — publish as its **own** GitHub repo (`mzh/monolink`), not under `monolith/`
  ▪ **API**: `Client` (dial, reconnect, handlers, optional inbox), `Message` / `Parse` / `Encode`, `LoadClientTLS`

  ───────────────────────────────────────────────────────────────
  ▓ FEATURES
  ▪ Gorilla WebSocket client with optional TLS client config (cloned per dial)
  ▪ Auto-reconnect loop; verb-keyed handlers plus catch-all `*`
  ▪ Optional buffered inbox for TUI / event-loop integrations
  ▪ `Request.Reply` for responses addressed back to the sender

  ───────────────────────────────────────────────────────────────
  ▓ REQUIREMENTS
  ▪ Go 1.24+ (see `go.mod`)

  ───────────────────────────────────────────────────────────────
  ▓ USE
  **Import**
  ```go
  import "github.com/MrZloHex/monolink"
  ```

  **Add to a module** (released version)
  ```sh
  go get github.com/MrZloHex/monolink@v0.1.0
  ```

  ───────────────────────────────────────────────────────────────
  ▓ PROTOCOL
  Wire format: `TO:VERB:NOUN[:ARGS]:FROM` (minimum four colon-separated fields). Application verbs and nouns are defined by each service; see **governor**, **achtung**, and **concentrator** README files in the superproject.

  ───────────────────────────────────────────────────────────────
  ▓ FINAL WORDS
  One link. Many nodes.

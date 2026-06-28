# Contributing to Brionic Remote

Thanks for your interest in improving Brionic Remote! This is an open-source
project and contributions of all sizes are welcome.

## Project layout

```
main.go                 entry point; embeds web/dist, starts the server
internal/vault/         encrypted vault: crypto, model, open/save/CRUD
internal/server/        HTTP/WebSocket API + static serving + SSH bridge
web/                    Vite + TypeScript frontend (xterm.js terminal)
  src/
    main.ts             UI (setup / unlock / connection list / form / terminal)
    api.ts              typed fetch wrappers
    terminal.ts         xterm.js <-> WebSocket bridge
    types.ts            shared types
Makefile                build / run / cross-compile helpers
```

## Development setup

Requirements: **Go 1.25+**, **Node 20+**.

```bash
make build      # build UI + binary
make dev        # run backend only (go run . --no-browser)
# in another terminal, for hot-reload UI:
cd web && npm install && npm run dev
```

## Before opening a PR

- `go build ./...` and `go test ./...` must pass.
- `cd web && npm run build` must pass (it type-checks via `tsc --noEmit`).
- Run `gofmt`/`go vet`; keep the frontend in the existing vanilla-TS style.
- Keep changes focused; describe the motivation in the PR.

## Security

Please **do not** open public issues for security vulnerabilities. Instead,
report them privately to security@brionicsecurity.com.

Areas where security review is especially valued: the vault crypto/envelope,
the SSH bridge, session handling, and host-key verification.

## License

By contributing you agree that your contributions are licensed under the
project's [MIT License](LICENSE).

## 1. Build

- [x] 1.1 Declare `Interop` in `bundle.go`
- [x] 1.2 Pin the declaration in `TestInteropDeclaration`

## 2. Release

- [ ] 2.1 Pin plugin-sdk v0.4.6 and move `SDKFloor`/`SDKCeiling` to `0.4.6`
- [ ] 2.2 `openspec validate --all --strict` and `script/gate.sh` pass
- [ ] 2.3 Tag the release, and the SDK matrix records it
- [ ] 2.4 `openspec archive core-2812-interop -y`

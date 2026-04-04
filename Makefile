.PHONY: build run test web wasm clean

# Build the full binary (frontend + backend)
build: web
	go build -o arbiter ./cmd/arbiter

# Run in development mode
run:
	go run ./cmd/arbiter

# Run all Go tests
test:
	go test ./... -count=1

# Build frontend
web:
	cd web && npm run build

# Build WASM evaluation engine
wasm:
	GOOS=js GOARCH=wasm go build -o wasm/arbiter.wasm ./cmd/wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" wasm/wasm_exec.js
	@echo "Built wasm/arbiter.wasm ($$(du -h wasm/arbiter.wasm | cut -f1))"

# Serve WASM example locally
wasm-serve: wasm
	@echo "Open http://localhost:9090/example.html"
	cd wasm && python3 -m http.server 9090

# Clean build artifacts
clean:
	rm -f arbiter wasm/arbiter.wasm wasm/wasm_exec.js
	rm -rf web/dist

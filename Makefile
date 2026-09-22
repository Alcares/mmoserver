MODULE := github.com/alcares/mmoserver/backend
BACKEND := backend
PROTO_DIR := $(BACKEND)/api/proto
CSHARP_OUT := unity/Assets/Scripts/Generated
UNITY_VERSION := 6000.6.2f1
UNITY := $(HOME)/Unity/Hub/Editor/$(UNITY_VERSION)/Editor/Unity
UNITY_PROJECT := $(CURDIR)/unity
SERVER_BIN := $(BACKEND)/bin/server
SERVER_PID := $(BACKEND)/server.pid
SERVER_LOG := $(BACKEND)/server.log
BENCH_CPU := $(CURDIR)/$(BACKEND)/bench-cpu.out
BENCH ?= .
BENCH_PKG ?= ./...
BENCH_PROF_PKG ?= ./internal/sim
UNITY_BATCH = LD_LIBRARY_PATH=$(HOME)/.local/lib/unity-compat:$$LD_LIBRARY_PATH $(UNITY) \
	-batchmode -quit -projectPath $(UNITY_PROJECT) -logFile - -executeMethod

.PHONY: proto clean server-start server-stop test bench bench-profile hooks client client-linux client-mac all

proto:
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(BACKEND) \
		--go_opt=module=$(MODULE) \
		$(PROTO_DIR)/game/v1/*.proto
	mkdir -p $(CSHARP_OUT)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--csharp_out=$(CSHARP_OUT) \
		$(PROTO_DIR)/game/v1/*.proto

clean:
	rm -rf $(BACKEND)/gen/
	rm -f $(CSHARP_OUT)/*.cs

# Rebuilds and (re)starts the server in the background; output goes to $(SERVER_LOG).
server-start: server-stop
	go -C $(BACKEND) build -o bin/server ./cmd/server
	nohup ./$(SERVER_BIN) > $(SERVER_LOG) 2>&1 & echo $$! > $(SERVER_PID)
	@sleep 1; kill -0 $$(cat $(SERVER_PID)) 2>/dev/null \
		&& echo "server running, pid $$(cat $(SERVER_PID))" \
		|| { echo "server exited, see $(SERVER_LOG)"; tail -n 20 $(SERVER_LOG); rm -f $(SERVER_PID); exit 1; }

server-stop:
	@if [ -f $(SERVER_PID) ]; then kill $$(cat $(SERVER_PID)) 2>/dev/null && echo "server stopped"; rm -f $(SERVER_PID); fi

test:
	go -C $(BACKEND) test ./...

# BENCH picks the benchmarks by regex, BENCH_PKG the packages:
# make bench BENCH=Step BENCH_PKG=./internal/sim
bench:
	go -C $(BACKEND) test $(BENCH_PKG) -run='^$$' -bench=$(BENCH) -benchmem

# Profiles one package's benchmarks and prints the hottest calls. BENCH_PROF_PKG is separate
# because -cpuprofile takes a single package; it also drops a $(BACKEND)/*.test binary.
bench-profile:
	go -C $(BACKEND) test $(BENCH_PROF_PKG) -run='^$$' -bench=$(BENCH) -benchtime=5s -cpuprofile=$(BENCH_CPU)
	go tool pprof -top -nodecount=25 $(BENCH_CPU)

hooks:
	git config core.hooksPath .githooks

# Unity client builds land in unity/Builds/; the Editor must be closed.
# client builds Linux and macOS in one Unity run, so the project loads only once.
client:
	$(UNITY_BATCH) Game.EditorTools.ClientBuilder.BuildAll

client-linux:
	$(UNITY_BATCH) Game.EditorTools.ClientBuilder.BuildLinux

client-mac:
	$(UNITY_BATCH) Game.EditorTools.ClientBuilder.BuildMac

# Regenerates protos, runs the Go tests, restarts the server in the background, then builds the Unity client.
all:
	$(MAKE) proto
	$(MAKE) test
	$(MAKE) client

run:
	$(MAKE) server-start
	$(UNITY_PROJECT)/Builds/Linux/MMOClient.x86_64
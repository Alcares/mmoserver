MODULE := github.com/alcares/mmoserver/backend
BACKEND := backend
PROTO_DIR := $(BACKEND)/api/proto
CSHARP_OUT := unity/Assets/Scripts/Generated
RL_DIR := rl-training
# Generated into the src root so `sim` is a top-level package next to rl_training, and the
# stubs' own `from sim.v1 import env_pb2` resolves without anyone touching sys.path.
SIM_PY_ROOT := $(RL_DIR)/src
SIM_PY_PKG := $(SIM_PY_ROOT)/sim
BOT_PY_PKG := $(SIM_PY_ROOT)/bot
UNITY_VERSION := 6000.6.2f1
UNITY := $(HOME)/Unity/Hub/Editor/$(UNITY_VERSION)/Editor/Unity
UNITY_PROJECT := $(CURDIR)/unity
SERVER_BIN := $(BACKEND)/bin/server
SERVER_PID := $(BACKEND)/server.pid
SERVER_LOG := $(BACKEND)/server.log
BENCH_CPU := $(CURDIR)/$(BACKEND)/bench-cpu.out
BENCH ?= .
ARGS ?=
BENCH_PKG ?= ./...
BENCH_PROF_PKG ?= ./internal/sim
UNITY_BATCH = LD_LIBRARY_PATH=$(HOME)/.local/lib/unity-compat:$$LD_LIBRARY_PATH $(UNITY) \
	-batchmode -quit -projectPath $(UNITY_PROJECT) -logFile - -executeMethod

.PHONY: proto proto-py clean server-start server-stop test bench bench-profile hooks client client-linux client-mac all baseline train

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
	# The sim service needs protoc-gen-go-grpc; Unity never sees it, the C# step above
	# globs game/v1 only. bot/v1 defines no service, for which the grpc plugin emits nothing.
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(BACKEND) \
		--go_opt=module=$(MODULE) \
		--go-grpc_out=$(BACKEND) \
		--go-grpc_opt=module=$(MODULE) \
		$(PROTO_DIR)/sim/v1/*.proto $(PROTO_DIR)/bot/v1/*.proto

# Python stubs for the training client. Needs grpcio-tools in the rl-training venv.
proto-py:
	# Before protoc, not after: protoc leaves the package dirs without __init__.py, which keeps
	# them out of the wheel, and uv builds the project before it runs anything - so a package
	# named in module-name that does not exist yet fails the build.
	mkdir -p $(SIM_PY_PKG)/v1 $(BOT_PY_PKG)/v1
	touch $(SIM_PY_PKG)/__init__.py $(SIM_PY_PKG)/v1/__init__.py
	touch $(BOT_PY_PKG)/__init__.py $(BOT_PY_PKG)/v1/__init__.py
	uv run --project $(RL_DIR) python -m grpc_tools.protoc \
		--proto_path=$(PROTO_DIR) \
		--python_out=$(SIM_PY_ROOT) \
		--pyi_out=$(SIM_PY_ROOT) \
		--grpc_python_out=$(SIM_PY_ROOT) \
		$(PROTO_DIR)/sim/v1/*.proto $(PROTO_DIR)/bot/v1/*.proto

# Baselines through the Python wrapper: scripted should reproduce the ~1.05 steps/optimal that
# internal/sim measures in Go. If it doesn't, the wrapper is wrong, not the policy.
# ARGS passes flags through: make baseline ARGS="--envs 32 --episodes 500"
baseline:
	uv run --project $(RL_DIR) python -m rl_training.baseline $(ARGS)

# Trains and evaluates; both spawn their own sim, so there is no stale binary to forget about.
# TensorBoard reads rl-training/runs.
train:
	uv run --project $(RL_DIR) python -m rl_training.train $(ARGS)

clean:
	rm -rf $(BACKEND)/gen/
	rm -rf $(SIM_PY_PKG) $(BOT_PY_PKG)
	rm -f $(CSHARP_OUT)/*.cs

# Rebuilds and (re)starts the server in the background; the server writes $(SERVER_LOG) itself.
server-start: server-stop
	go -C $(BACKEND) build -o bin/server ./cmd/server
	nohup ./$(SERVER_BIN) > /dev/null 2>&1 & echo $$! > $(SERVER_PID)
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
	$(MAKE) client-linux

run:
	$(MAKE) server-start
	$(UNITY_PROJECT)/Builds/Linux/MMOClient.x86_64

everything:
	$(MAKE) train
	$(MAKE) all
	$(MAKE) run

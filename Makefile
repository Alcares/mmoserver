MODULE := github.com/alcares/mmoserver
PROTO_DIR := api/proto
CSHARP_OUT := unity-client/Assets/Scripts/Generated
UNITY_VERSION := 6000.6.2f1
UNITY := $(HOME)/Unity/Hub/Editor/$(UNITY_VERSION)/Editor/Unity
UNITY_PROJECT := $(CURDIR)/unity-client
SERVER_BIN := bin/server
SERVER_PID := server.pid
SERVER_LOG := server.log
UNITY_BATCH = LD_LIBRARY_PATH=$(HOME)/.local/lib/unity-compat:$$LD_LIBRARY_PATH $(UNITY) \
	-batchmode -quit -projectPath $(UNITY_PROJECT) -logFile - -executeMethod

.PHONY: proto clean server-start server-stop test hooks client client-linux client-mac all

proto:
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=. \
		--go_opt=module=$(MODULE) \
		$(PROTO_DIR)/game/v1/*.proto
	mkdir -p $(CSHARP_OUT)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--csharp_out=$(CSHARP_OUT) \
		$(PROTO_DIR)/game/v1/*.proto

clean:
	rm -rf gen/
	rm -f $(CSHARP_OUT)/*.cs

# Rebuilds and (re)starts the server in the background; output goes to $(SERVER_LOG).
server-start: server-stop
	go build -o $(SERVER_BIN) ./cmd/server
	nohup ./$(SERVER_BIN) > $(SERVER_LOG) 2>&1 & echo $$! > $(SERVER_PID)
	@sleep 1; kill -0 $$(cat $(SERVER_PID)) 2>/dev/null \
		&& echo "server running, pid $$(cat $(SERVER_PID))" \
		|| { echo "server exited, see $(SERVER_LOG)"; tail -n 20 $(SERVER_LOG); rm -f $(SERVER_PID); exit 1; }

server-stop:
	@if [ -f $(SERVER_PID) ]; then kill $$(cat $(SERVER_PID)) 2>/dev/null && echo "server stopped"; rm -f $(SERVER_PID); fi

test:
	go test ./...

hooks:
	git config core.hooksPath .githooks

# Unity client builds land in unity-client/Builds/; the Editor must be closed.
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
	~/Projects/MMOServer/unity-client/Builds/Linux/MMOClient.x86_64
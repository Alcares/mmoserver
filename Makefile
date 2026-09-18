MODULE := github.com/alcares/mmoserver
PROTO_DIR := api/proto
CSHARP_OUT := unity-client/Assets/Scripts/Generated

.PHONY: proto clean run

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

run:
	go run ./cmd/server

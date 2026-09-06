MODULE := github.com/alcares/mmoserver
PROTO_DIR := api/proto

.PHONY: proto clean

proto:
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=. \
		--go_opt=module=$(MODULE) \
		$(PROTO_DIR)/game/v1/*.proto

clean:
	rm -rf gen/

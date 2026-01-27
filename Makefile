.all: build

build:
	go mod tidy
	(cd apps/garbled && rm ./garbled || true && go build)

# Build C shared library for Rust FFI
lib:
	@echo "Building C shared library from Go code..."
	@(cd apps/garbled && go build -buildmode=c-shared -o libgarbled.so c_export.go result.go)
	@echo "Successfully built apps/garbled/libgarbled.so and apps/garbled/libgarbled.h"

clean-lib:
	@rm -f apps/garbled/libgarbled.so apps/garbled/libgarbled.h

kill_tmux:
	@tmux kill-session -t mpchd || true

run: kill_tmux build
	@(cd apps/garbled && \
		tmux new-session -s mpchd \
		-n m -d ";" new-window \
		-n e -d ";" new-window \
		-n g -d ";")
	@tmux send-keys -t mpchd:m "(cd ../messenger && go run main.go)" C-m
	@tmux send-keys -t mpchd:e "sleep 2 && ./garbled -e -i 0x1919810" C-m
	@tmux send-keys -t mpchd:g "sleep 2 && ./garbled -i 0x114514,0x4de216d2fdc9301e5b9c78486f7109a05670d200d9e2f275ec0aad08ec42afe7,893" C-m

.PHONY: build lib clean-lib kill_tmux run
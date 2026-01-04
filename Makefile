.all: build

build:
	go mod tidy
	(cd apps/garbled && rm ./garbled && go build)

kill_tmux:
	@tmux kill-session -t mpchd || true

run: kill_tmux build
	@(cd apps/garbled && \
		tmux new-session -s mpchd \
		-n m -d ";" new-window \
		-n e -d ";" new-window \
		-n g -d ";")
	@tmux send-keys -t mpchd:m "(cd ../messenger && go run main.go)" C-m
	@sleep 3
	@tmux send-keys -t mpchd:e "./garbled -e -v -i 0xf87a00ef89c2396de32f6ac0748f6fa1b641013d46f74ce25cc625904215a67501c0c7196a2602f6516527958a82271847933c35d170d98bfdb04d2ddf3bb197 examples/hmac-sha256.mpcl" C-m
	@tmux send-keys -t mpchd:g "./garbled -v -i 0x48656c6c6f2c20776f726c64212e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e,0x4de216d2fdc9301e5b9c78486f7109a05670d200d9e2f275ec0aad08ec42af47fcb59bf460d50b01333a748f3a9efb13e08036d49a26c21ba2e33a5f8a2cf0e7 examples/hmac-sha256.mpcl" C-m
BIN := kairo
PREFIX := $(HOME)/.local/bin
MODELS := $(HOME)/.kairo/models
PIPER_VOICE := https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx

build:
	go build -o $(BIN) .

install: build
	mkdir -p $(PREFIX)
	cp $(BIN) $(PREFIX)/$(BIN).new
	mv -f $(PREFIX)/$(BIN).new $(PREFIX)/$(BIN)

run: build
	./$(BIN) start

vet:
	go vet ./...

models:
	mkdir -p $(MODELS)
	curl -L -o $(MODELS)/ggml-base.en.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin
	curl -L -o $(MODELS)/en_US-lessac-medium.onnx $(PIPER_VOICE)
	curl -L -o $(MODELS)/en_US-lessac-medium.onnx.json $(PIPER_VOICE).json

hotkey:
	sudo install -Dm644 scripts/keyd-kairo.conf /etc/keyd/kairo.conf
	sudo systemctl enable keyd
	sudo systemctl restart keyd

clean:
	rm -f $(BIN)

.PHONY: build install run vet models hotkey clean

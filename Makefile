GO_CMD   := go
GO_BUILD := go build

BINARIES := bin/cri-loadgen

all: $(BINARIES)

clean:
	@rm -f $(BINARIES)

bin/cri-loadgen: $(wildcard cmd/cri-loadgen/*.go)
	@mkdir -p bin && $(GO_BUILD) -o $@ ./$(dir $<)

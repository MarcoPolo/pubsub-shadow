# Default target
all: run

# Clean all generated shadow simulation files
clean:
	rm -rf shadow*.data || true
	rm -rf synctest-*.data || true
	rm -rf gossipsub-v0.13.1-stock.data || true
	rm plots/* || true

binaries:
	cd gossipsub-v0.13.1 && go build -linkshared -o gossipsub-bin

# Run the shadow simulation
run:
	uv run run.py

.PHONY: binaries all run clean

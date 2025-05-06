# Default target
all: run

gossipsub-v0.13.1/gossipsub-bin: gossipsub-v0.13.1/*
	cd gossipsub-v0.13.1 && go build -linkshared -o gossipsub-bin

# Clean all generated shadow simulation files
clean:
	rm -rf shadow*.data || true
	rm -rf synctest-*.data || true
	rm plots/* || true

extract_data:
	bash -c "for file in shadow-*.tar.gz; do tar xvf \$$file; done"

# Run the shadow simulation
run: gossipsub-v0.13.1/gossipsub-bin graph.gml shadow.yaml params.json
	$(eval filename=shadow.data)
	rm -rf $(filename) || true

	shadow --progress true -d $(filename) shadow.yaml
	cp shadow.yaml $(filename)/shadow.yaml
	cp graph.gml $(filename)/graph.gml

params.json: generate_params.py
	python3 generate_params.py --node_count $(node_count) --output params.json

# node_count is required. i.e. make network_graph node_count=1000
graph.gml shadow.yaml: network_graph.py shadow.template.yaml
	uv run network_graph.py --node_count $(node_count) --output shadow.yaml --binary-and-percentage $(binary_and_percentage)

.PHONY: all run clean extract_data

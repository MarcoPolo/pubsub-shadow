# Default target
all: run

pubsub-shadow: *.go go.mod go.sum
	go build -linkshared

# Clean all generated shadow simulation files
clean:
	rm -rf shadow-*.data || true
	rm -rf synctest-*.data || true
	rm plots/* || true

extract_data:
	bash -c "for file in shadow-*.tar.gz; do tar xvf \$$file; done"

# Run the shadow simulation
run: pubsub-shadow
	$(eval filename=shadow-$(blob_count)-blobs-$(node_count)-$(publish_strategy).data)
	@echo Writing network graph to $(filename)/shadow.yaml

	python3 network_graph.py --nodeCount $(node_count) --targetConns $(target_conns) --publishStrategy $(publish_strategy) --blobCount $(blob_count) --output shadow.yaml
	shadow --progress true -d $(filename) shadow.yaml
	mv shadow.yaml $(filename)/shadow.yaml
	mv graph.gml $(filename)/graph.gml


shadow-72-blobs-1000-rarestFirst.data:
	$(MAKE) run publish_strategy=rarestFirst blob_count=72 node_count=1000 target_conns=64

shadow-72-blobs-1000-inOrder.data:
	$(MAKE) run publish_strategy=inOrder blob_count=72 node_count=1000 target_conns=64

.PHONY: all run clean extract_data

# Provenance

Everything in this repository was written for it, on personal time, by Tran Trung Dung.

- Four wire formats behind one interface, and a gate that keeps their claims true. No employer code, no client code, no proprietary data. Nothing here is
  extracted from a product I worked on; it talks to nothing external; the schemas and the payloads are in this repository.
- Third-party code is not vendored: the dependencies are github.com/bufbuild/protocompile, github.com/google/flatbuffers, google.golang.org/protobuf and gopkg.in/yaml.v3, and each keeps its own licence.
- The diagrams are drawn here. The commit history is the history of this repository.
- `scripts/chart-check.sh` renders the chart in CI and checks that the manifest still makes the claims the README does; it is part of the evidence, not decoration.

If you believe something here belongs to you, open an issue and it will be dealt with.

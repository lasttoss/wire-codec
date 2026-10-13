.PHONY: test mutate layout image compose gate

test: ## every gate
	@go vet ./...
	@go test ./... -count=1

# A gate nobody has seen fail is a gate nobody can trust. Each mutation is a plausible mistake,
# and each one has to make the suite red; the file is restored from a copy, and the copy is
# compared afterwards, because "the tests pass again" is not the same as "the file is back".
mutate: ## break the codec on purpose; every mutation must be caught
	@set -e; for m in \
	  's/binary.BigEndian.AppendUint16(out, uint16(int16(canon\[f.Name\].(int64))))/binary.LittleEndian.AppendUint16(out, uint16(int16(canon[f.Name].(int64))))/' \
	  's/for _, axis := range \[\]string{"x", "y", "z"}/for _, axis := range []string{"z", "y", "x"}/' \
	  's/binary.BigEndian.AppendUint16(out, uint16(len(s)))/binary.LittleEndian.AppendUint16(out, uint16(len(s)))/' ; do \
	  cp codec/bytecodec/byte.go /tmp/wire-codec-byte.orig; \
	  sed -i "$$m" codec/bytecodec/byte.go; \
	  if diff -q /tmp/wire-codec-byte.orig codec/bytecodec/byte.go >/dev/null; then \
	    echo "mutation did not apply (pattern drifted): $$m"; exit 1; fi; \
	  n=$$(go test ./codec/ -count=1 2>&1 | grep -cE '^--- FAIL|disagree' || true); \
	  cp /tmp/wire-codec-byte.orig codec/bytecodec/byte.go; \
	  diff -q /tmp/wire-codec-byte.orig codec/bytecodec/byte.go >/dev/null || { echo "restore failed"; exit 1; }; \
	  if [ "$$n" -eq 0 ]; then echo "GATE IS BLIND: $$m"; exit 1; fi; \
	  echo "caught ($$n failures): $$(echo "$$m" | cut -c1-60)..."; \
	done

layout: ## regenerate the byte layout document
	@go run ./cmd/layout > docs/byte-layout.md
	@echo "docs/byte-layout.md regenerated"

image: ## build the lab image
	@docker build -q -t wire-codec:dev .

compose: image ## the interchange lab: two containers, one format (CODEC=byte|json)
	@CODEC=$${CODEC:-byte} docker compose up --exit-code-from reader --abort-on-container-exit

gate: ## the interchange gate, inside the container
	@docker compose run --rm --no-deps gate

chart:  ## lint, render every codec branch, and validate
	@./scripts/chart-check.sh

diagram: ## regenerate the figure and check it against its sources
	@python3 docs/figures/wire-codec-flow.generator.py
	@bash ~/.pi/agent/scripts/diagram-freshness.sh 2>/dev/null | grep -E "FRESH|STALE|TOUCHED|NO-META|UNCOMMITTED" | sed 's/^/  /'

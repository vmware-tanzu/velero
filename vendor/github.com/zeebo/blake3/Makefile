asm: internal/alg/hash/hash_avx2/impl_amd64.s internal/alg/compress/compress_sse41/impl_amd64.s

internal/alg/hash/hash_avx2/impl_amd64.s: avo/avx2/*.go
	( cd avo; go run ./avx2 ) > internal/alg/hash/hash_avx2/impl_amd64.s

internal/alg/compress/compress_sse41/impl_amd64.s: avo/sse41/*.go
	( cd avo; go run ./sse41 ) > internal/alg/compress/compress_sse41/impl_amd64.s

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: clean
clean:
	rm -f internal/alg/hash/hash_avx2/impl_amd64.s
	rm -f internal/alg/compress/compress_sse41/impl_amd64.s

.PHONY: test
test:
	go test -race -bench=. -benchtime=1x

.PHONY: vet
vet:
	go tool dist list         \
	| sed -e 's#/# #g'        \
	| while read goos goarch; \
	do                        \
		echo $$goos $$goarch; \
		GOOS=$$goos GOARCH=$$goarch CGO_ENABLED=1 GO386=softfloat go vet              ./...; \
		GOOS=$$goos GOARCH=$$goarch CGO_ENABLED=1 GO386=softfloat go vet -tags=purego ./...; \
	done

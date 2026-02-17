CC = gcc
CXX = g++
CFLAGS = -O3 -flto -march=native
CXXFLAGS = -O3 -flto -march=native -std=c++17

.PHONY: all go rust c cpp haskell clean benchmark

all: go rust c cpp

go:
	go build -o bin/go-reasoner ./cmd/classify

rust:
	cd rust-impl && cargo build --release
	cp rust-impl/target/release/el-reasoner bin/

c:
	$(CC) $(CFLAGS) -o bin/c-reasoner c-impl/main.c

cpp:
	$(CXX) $(CXXFLAGS) -o bin/cpp-reasoner cpp-impl/main.cpp

haskell:
	cd haskell-impl && cabal build
	cp haskell-impl/dist-newstyle/build/*/ghc-*/el-reasoner-*/x/el-reasoner/opt/build/el-reasoner/el-reasoner bin/

clean:
	rm -rf bin/

bin:
	mkdir -p bin

benchmark: bin
	./run_benchmark.sh

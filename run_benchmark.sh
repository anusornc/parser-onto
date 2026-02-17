#!/bin/bash

OBO_FILE="testdata/chebi.obo"
RUNS=5

echo "=========================================="
echo "    EL REASONER MULTI-LANGUAGE BENCHMARK"
echo "=========================================="
echo "Input: $OBO_FILE"
echo "Runs: $RUNS"
echo ""

if [ ! -f "$OBO_FILE" ]; then
    echo "Error: $OBO_FILE not found"
    exit 1
fi

mkdir -p bin

# Build all implementations
echo "Building implementations..."
echo ""

echo "--- Go ---"
go build -o bin/go-reasoner ./cmd/classify 2>&1 | grep -v "^$"
[ -f bin/go-reasoner ] && echo "OK: bin/go-reasoner" || echo "FAILED"

echo ""
echo "--- Rust ---"
cd rust-impl && cargo build --release 2>&1 | grep -E "Compiling|Finished"
cp target/release/el-reasoner ../bin/ 2>/dev/null
cd ..
[ -f bin/el-reasoner ] && echo "OK: bin/el-reasoner" || echo "FAILED"

echo ""
echo "--- C ---"
gcc -O3 -flto -march=native -o bin/c-reasoner c-impl/main.c 2>&1 | grep -v "^$"
[ -f bin/c-reasoner ] && echo "OK: bin/c-reasoner" || echo "FAILED"

echo ""
echo "--- C++ ---"
g++ -O3 -flto -march=native -std=c++17 -o bin/cpp-reasoner cpp-impl/main.cpp 2>&1 | grep -v "^$"
[ -f bin/cpp-reasoner ] && echo "OK: bin/cpp-reasoner" || echo "FAILED"

# Haskell needs cabal - skip if not available
if command -v cabal &> /dev/null; then
    echo ""
    echo "--- Haskell ---"
    cd haskell-impl && cabal build 2>&1 | tail -3
    cd ..
fi

echo ""
echo "=========================================="
echo "               BENCHMARKS"
echo "=========================================="

# Go
if [ -f bin/go-reasoner ]; then
    echo ""
    echo "--- Go ---"
    for i in $(seq 1 $RUNS); do
        echo -n "Run $i: "
        time bin/go-reasoner -input $OBO_FILE -output /tmp/go-$i.json 2>&1 | grep -E "Saturation time|Total time"
    done
fi

# Rust
if [ -f bin/el-reasoner ]; then
    echo ""
    echo "--- Rust ---"
    for i in $(seq 1 $RUNS); do
        echo -n "Run $i: "
        time bin/el-reasoner $OBO_FILE 2>&1 | grep -E "Saturation time|Total time"
    done
fi

# C
if [ -f bin/c-reasoner ]; then
    echo ""
    echo "--- C ---"
    for i in $(seq 1 $RUNS); do
        echo -n "Run $i: "
        time bin/c-reasoner $OBO_FILE 2>&1 | grep -E "Saturation|Total"
    done
fi

# C++
if [ -f bin/cpp-reasoner ]; then
    echo ""
    echo "--- C++ ---"
    for i in $(seq 1 $RUNS); do
        echo -n "Run $i: "
        time bin/cpp-reasoner $OBO_FILE 2>&1 | grep -E "Saturation|Total"
    done
fi

# ELK (if available)
if [ -f /tmp/elk/elk-distribution-cli-0.6.0/elk.jar ]; then
    echo ""
    echo "--- ELK (Java) ---"
    for i in $(seq 1 3); do
        echo "Run $i:"
        time /tmp/jdk-17.0.2/bin/java -Xmx8G -jar /tmp/elk/elk-distribution-cli-0.6.0/elk.jar -i /tmp/chebi-functional.ofn -c 2>&1 | grep -v WARN | tail -1
    done
fi

echo ""
echo "=========================================="
echo "               SUMMARY"
echo "=========================================="
echo ""
echo "Results saved to /tmp/*.json"

#!/bin/bash
# Benchmark script for comparing Go and Rust EL reasoners

OBO_FILE="../testdata/chebi.obo"
RUNS=5

echo "=== EL Reasoner Benchmark ==="
echo "Input: $OBO_FILE"
echo "Runs: $RUNS"
echo ""

# Check if files exist
if [ ! -f "$OBO_FILE" ]; then
    echo "Error: $OBO_FILE not found"
    exit 1
fi

# Build binaries
echo "Building Go reasoner..."
cd ..
go build -o chebi-classify ./cmd/classify
cd rust-impl

echo "Building Rust reasoner..."
cargo build --release 2>/dev/null
if [ $? -ne 0 ]; then
    echo "Warning: Rust build failed. Make sure Rust is installed."
fi

echo ""
echo "=== Running Benchmarks ==="

# Go benchmark
echo ""
echo "--- Go Implementation ---"
for i in $(seq 1 $RUNS); do
    echo "Run $i:"
    time ../chebi-classify -input $OBO_FILE -output /tmp/go-out-$i.json 2>&1 | grep -E "Saturation|Total"
done

# Rust benchmark (if available)
if [ -f target/release/el-reasoner ]; then
    echo ""
    echo "--- Rust Implementation ---"
    for i in $(seq 1 $RUNS); do
        echo "Run $i:"
        time target/release/el-reasoner $OBO_FILE 2>&1 | grep -E "Saturation|Total"
    done
fi

# ELK benchmark (if available)
if [ -f /tmp/elk/elk-distribution-cli-0.6.0/elk.jar ]; then
    echo ""
    echo "--- ELK (Java) ---"
    for i in $(seq 1 $RUNS); do
        echo "Run $i:"
        time /tmp/jdk-17.0.2/bin/java -Xmx8G -jar /tmp/elk/elk-distribution-cli-0.6.0/elk.jar -i /tmp/chebi-functional.ofn -c 2>&1 | grep -v WARN | tail -1
    done
fi

echo ""
echo "=== Done ==="

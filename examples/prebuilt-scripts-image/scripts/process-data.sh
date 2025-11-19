#!/bin/bash
set -e

# Pre-built script for data processing
# This script demonstrates using a pre-built script with ScriptRunner

echo "=================================="
echo "Data Processing Script"
echo "=================================="
echo ""

# Show script info
echo "Script: process-data.sh"
echo "ScriptRunner: $SCRIPTRUNNER_NAME"
echo "Namespace: $SCRIPTRUNNER_NAMESPACE"
echo ""

# Process arguments
if [ $# -eq 0 ]; then
    echo "No arguments provided. Using defaults."
    OPERATION="${INPUT_operation:-process}"
    COUNT="${INPUT_count:-10}"
else
    echo "Arguments provided: $@"
    OPERATION="${1:-process}"
    COUNT="${2:-10}"
fi

echo "Operation: $OPERATION"
echo "Count: $COUNT"
echo ""

# Show all INPUT_ environment variables
echo "Input Variables:"
env | grep "^INPUT_" | sort | while IFS='=' read -r key value; do
    echo "  $key = $value"
done
echo ""

# Perform the operation
echo "Starting $OPERATION operation..."
for i in $(seq 1 "$COUNT"); do
    if [ $((i % 5)) -eq 0 ]; then
        echo "  Processed $i/$COUNT items..."
    fi
    sleep 0.1
done

echo ""
echo "=================================="
echo "Processing Complete!"
echo "=================================="
echo "Processed $COUNT items successfully."

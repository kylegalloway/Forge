#!/usr/bin/env python3
"""
Pre-built Python script for status reporting
This demonstrates using Python scripts with ScriptRunner
"""

import os
import json
import sys
from datetime import datetime

def main():
    print("=" * 50)
    print("Status Report Script (Python)")
    print("=" * 50)
    print()

    # Get ScriptRunner metadata
    sr_name = os.environ.get('SCRIPTRUNNER_NAME', 'unknown')
    sr_namespace = os.environ.get('SCRIPTRUNNER_NAMESPACE', 'default')

    print(f"ScriptRunner: {sr_name}")
    print(f"Namespace: {sr_namespace}")
    print(f"Timestamp: {datetime.now().isoformat()}")
    print()

    # Collect all INPUT_ variables
    inputs = {}
    for key, value in os.environ.items():
        if key.startswith('INPUT_'):
            input_name = key[6:]  # Remove 'INPUT_' prefix
            inputs[input_name] = value

    # Parse arguments if provided
    if len(sys.argv) > 1:
        report_format = sys.argv[1]
    else:
        report_format = inputs.get('format', 'text')

    print(f"Report Format: {report_format}")
    print()

    # Generate report
    report = {
        "scriptrunner": sr_name,
        "namespace": sr_namespace,
        "timestamp": datetime.now().isoformat(),
        "inputs": inputs,
        "status": "success"
    }

    if report_format == 'json':
        print("JSON Report:")
        print(json.dumps(report, indent=2))
    else:
        print("Text Report:")
        print(f"  Status: {report['status']}")
        print(f"  Inputs: {len(inputs)}")
        if inputs:
            print("  Input Details:")
            for key, value in sorted(inputs.items()):
                print(f"    - {key}: {value}")

    print()
    print("=" * 50)
    print("✓ Report Generated Successfully")
    print("=" * 50)

if __name__ == '__main__':
    main()

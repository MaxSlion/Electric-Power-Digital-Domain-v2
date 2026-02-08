#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""
Algorithm Service Test Runner
统一测试脚本 - 运行功能测试、接口测试、集成测试
"""

import subprocess
import sys
import os

def run_tests(test_type=None, verbose=True):
    """
    Run pytest tests with specified markers.
    
    Args:
        test_type: "unit", "integration", "api", or None for all
        verbose: Whether to show verbose output
    """
    os.chdir(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    
    cmd = ["python", "-m", "pytest"]
    
    if verbose:
        cmd.append("-v")
    
    if test_type:
        cmd.extend(["-m", test_type])
    
    cmd.extend(["--tb=short", "tests/"])
    
    print(f"\n{'='*60}")
    print(f"Running {test_type or 'all'} tests...")
    print(f"Command: {' '.join(cmd)}")
    print(f"{'='*60}\n")
    
    result = subprocess.run(cmd, cwd=os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    return result.returncode

def main():
    """Main entry point for test runner."""
    import argparse
    
    parser = argparse.ArgumentParser(description="Algorithm Service Test Runner")
    parser.add_argument(
        "--type", "-t",
        choices=["unit", "integration", "api", "all"],
        default="all",
        help="Type of tests to run (default: all)"
    )
    parser.add_argument(
        "--quiet", "-q",
        action="store_true",
        help="Quiet mode (less verbose output)"
    )
    
    args = parser.parse_args()
    
    test_type = None if args.type == "all" else args.type
    
    exit_code = run_tests(test_type, verbose=not args.quiet)
    
    print(f"\n{'='*60}")
    if exit_code == 0:
        print("✅ All tests passed!")
    else:
        print(f"❌ Tests failed with exit code: {exit_code}")
    print(f"{'='*60}\n")
    
    sys.exit(exit_code)

if __name__ == "__main__":
    main()

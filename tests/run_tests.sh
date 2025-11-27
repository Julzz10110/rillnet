#!/bin/bash

set -e

echo "🧪 RillNet Test Runner"
echo "======================"

case "${1:-help}" in
    unit)
        echo "Running unit tests..."
        make unit
        ;;
    integration)
        echo "Running integration tests..."
        make integration
        ;;
    load)
        echo "Running load tests..."
        make load
        ;;
    stress)
        echo "Running stress tests..."
        make stress
        ;;
    benchmark)
        echo "Running benchmark tests..."
        make benchmark
        ;;
    coverage)
        echo "Generating coverage report..."
        make coverage
        ;;
    all)
        echo "Running all tests..."
        make all
        ;;
    ci)
        echo "Running CI test suite..."
        make ci
        ;;
    clean)
        echo "Cleaning test artifacts..."
        make clean
        ;;
    help|*)
        echo "Usage: $0 {unit|integration|load|stress|benchmark|coverage|all|ci|clean|help}"
        echo ""
        echo "Examples:"
        echo "  $0 unit          # Run unit tests"
        echo "  $0 integration   # Run integration tests" 
        echo "  $0 load          # Run load tests"
        echo "  $0 all           # Run all tests"
        echo "  $0 ci            # Run CI test suite"
        exit 1
        ;;
esac
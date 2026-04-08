#!/usr/bin/env bash
# Trek Control Plane - Pre-PR Validation Script
# Run this before opening a PR to catch issues early
# Usage: ./scripts/validate.sh [local|ci]

set -euo pipefail

# 🎨 Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# 📊 Global counters
TOTAL_STEPS=0
PASSED_STEPS=0
FAILED_STEPS=0
SKIPPED_STEPS=0
WARNING_COUNT=0
START_TIME=$(date +%s)

# 🔧 Configuration
MODE=${1:-"local"}
COVERAGE_THRESHOLD=${COVERAGE_THRESHOLD:-70}
TEST_TIMEOUT=${TEST_TIMEOUT:-5m}
SKIP_E2E=${SKIP_E2E:-true}

# 🎯 Helper functions
print_header() {
    echo -e "\n${PURPLE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${PURPLE}🚀 TREK CONTROL PLANE - VALIDATION PIPELINE${NC}"
    echo -e "${PURPLE}Mode: ${CYAN}$MODE${PURPLE} | Coverage: ${CYAN}${COVERAGE_THRESHOLD}%${PURPLE} | Timeout: ${CYAN}$TEST_TIMEOUT${NC}"
    echo -e "${PURPLE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

print_step() {
    local step_name="$1"
    local icon="$2"
    echo -e "${BLUE}$icon Running: ${CYAN}$step_name${NC}"
}

print_success() {
    local step_name="$1"
    echo -e "${GREEN}✅ $step_name: PASSED${NC}"
    ((PASSED_STEPS++))
}

print_failure() {
    local step_name="$1"
    local error_msg="$2"
    echo -e "${RED}❌ $step_name: FAILED${NC}"
    echo -e "${RED}   Error: $error_msg${NC}"
    ((FAILED_STEPS++))
}

print_skipped() {
    local step_name="$1"
    local reason="$2"
    echo -e "${YELLOW}⏭️  $step_name: SKIPPED${NC}"
    echo -e "${YELLOW}   Reason: $reason${NC}"
    ((SKIPPED_STEPS++))
}

print_warning() {
    local message="$1"
    echo -e "${YELLOW}⚠️  Warning: $message${NC}"
    ((WARNING_COUNT++))
}

print_info() {
    local message="$1"
    echo -e "${CYAN}ℹ️  $message${NC}"
}

run_step() {
    local step_name="$1"
    local step_function="$2"
    local icon="$3"
    local skip_reason="${4:-}"

    ((TOTAL_STEPS++))

    if [[ -n "$skip_reason" ]]; then
        print_skipped "$step_name" "$skip_reason"
        return 0
    fi

    print_step "$step_name" "$icon"

    if $step_function; then
        print_success "$step_name"
        return 0
    else
        print_failure "$step_name" "Check output above for details"
        return 1
    fi
}

print_summary() {
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))

    echo -e "\n${PURPLE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${PURPLE}📊 VALIDATION SUMMARY${NC}"
    echo -e "${PURPLE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}✅ Passed:  $PASSED_STEPS${NC}"
    echo -e "${RED}❌ Failed:  $FAILED_STEPS${NC}"
    echo -e "${YELLOW}⏭️  Skipped: $SKIPPED_STEPS${NC}"
    echo -e "${YELLOW}⚠️  Warnings: $WARNING_COUNT${NC}"
    echo -e "${CYAN}⏱️  Duration: ${duration}s${NC}"

    if [[ $FAILED_STEPS -eq 0 ]]; then
        echo -e "\n${GREEN}🎉 All validations passed! Ready for PR.${NC}\n"
    else
        echo -e "\n${RED}💥 Validation failed. Please fix issues before opening PR.${NC}\n"
    fi
}

# 🔍 Environment validation
check_environment() {
    if ! command -v go &> /dev/null; then
        echo "Go is not installed or not in PATH"
        return 1
    fi

    local go_version=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+' || go version | sed -n 's/.*go\([0-9]*\.[0-9]*\).*/\1/p')
    print_info "Go version: $go_version"

    if ! command -v git &> /dev/null; then
        echo "Git is not installed"
        return 1
    fi

    if ! git rev-parse --git-dir &> /dev/null; then
        echo "Not in a git repository"
        return 1
    fi

    print_info "Environment checks passed!"
    return 0
}

# 🔍 Linting
run_linting() {
    local gopath_bin="$(go env GOPATH)/bin"
    if [[ ":$PATH:" != *":$gopath_bin:"* ]]; then
        export PATH="$gopath_bin:$PATH"
    fi

    if ! command -v golangci-lint >/dev/null 2>&1; then
        print_warning "golangci-lint not found, installing..."
        if ! go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; then
            echo "Failed to install golangci-lint"
            return 1
        fi
    fi

    if ! golangci-lint run --timeout=$TEST_TIMEOUT ./...; then
        return 1
    fi

    print_info "Code passes all lint checks!"
    return 0
}

# 🏗️ Build validation
validate_build() {
    print_info "Building all packages..."
    if ! go build ./...; then
        return 1
    fi

    print_info "Checking module dependencies..."
    local mod_before mod_sum_before
    mod_before=$(cat go.mod 2>/dev/null || echo "")
    mod_sum_before=$(cat go.sum 2>/dev/null || echo "")

    go mod tidy

    local mod_after mod_sum_after
    mod_after=$(cat go.mod 2>/dev/null || echo "")
    mod_sum_after=$(cat go.sum 2>/dev/null || echo "")

    if [[ "$mod_before" != "$mod_after" ]] || [[ "$mod_sum_before" != "$mod_sum_after" ]]; then
        if [[ "$MODE" == "ci" ]]; then
            echo "go.mod or go.sum changed after 'go mod tidy'"
            echo "Run 'go mod tidy' and commit changes"
            return 1
        else
            print_info "go mod tidy updated dependencies"
        fi
    fi

    print_info "Build successful!"
    return 0
}

# 🧪 Unit tests
run_unit_tests() {
    print_info "Running unit tests with race detection..."

    if ! go test -race -timeout=$TEST_TIMEOUT -coverprofile=coverage.out -covermode=atomic ./internal/...; then
        return 1
    fi

    print_info "All unit tests passed!"
    return 0
}

# 🔗 E2E tests (requires Docker)
run_e2e_tests() {
    print_info "Running e2e tests..."

    if ! command -v docker &> /dev/null; then
        print_warning "Docker not available, skipping e2e tests"
        return 0
    fi

    if ! go test -timeout=$TEST_TIMEOUT ./tests/e2e/...; then
        return 1
    fi

    print_info "E2E tests passed!"
    return 0
}

# 📊 Coverage validation
validate_coverage() {
    if [[ ! -f coverage.out ]]; then
        print_warning "No coverage file found"
        return 0
    fi

    local coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    local coverage_int=${coverage%.*}

    print_info "Coverage: ${coverage}%"

    if [[ $coverage_int -lt $COVERAGE_THRESHOLD ]]; then
        echo "Coverage ${coverage}% is below threshold ${COVERAGE_THRESHOLD}%"
        return 1
    fi

    print_info "Coverage meets threshold!"
    return 0
}

# 📚 Documentation check
validate_documentation() {
    local missing_docs=0

    if [[ ! -f README.md ]]; then
        echo "Missing README.md"
        missing_docs=1
    fi

    if [[ ! -d docs ]]; then
        print_warning "No docs/ directory found"
    fi

    if [[ $missing_docs -eq 1 ]]; then
        return 1
    fi

    print_info "Documentation check passed!"
    return 0
}

# 🧹 Final cleanup
final_validation() {
    print_info "Running final checks..."

    # Check for uncommitted changes in CI
    if [[ "$MODE" == "ci" ]]; then
        if ! git diff --quiet; then
            echo "Uncommitted changes detected"
            git diff --stat
            return 1
        fi
    fi

    # Clean up coverage file
    if [[ -f coverage.out ]]; then
        rm -f coverage.out
    fi

    print_info "Final validation complete!"
    return 0
}

# 🚀 Main pipeline
main() {
    print_header

    run_step "Environment Check" "check_environment" "🔍" || exit 1
    run_step "Linting" "run_linting" "🔍" || exit 1
    run_step "Build Validation" "validate_build" "🏗️" || exit 1
    run_step "Unit Tests" "run_unit_tests" "🧪" || exit 1

    local e2e_skip_reason=""
    if [[ "$SKIP_E2E" == "true" ]]; then
        e2e_skip_reason="SKIP_E2E flag set (requires Docker)"
    fi
    run_step "E2E Tests" "run_e2e_tests" "🔗" "$e2e_skip_reason" || exit 1

    run_step "Coverage Check" "validate_coverage" "📊" || exit 1
    run_step "Documentation" "validate_documentation" "📚" || exit 1
    run_step "Final Validation" "final_validation" "🧹" || exit 1

    print_summary

    if [[ $FAILED_STEPS -eq 0 ]]; then
        exit 0
    else
        exit 1
    fi
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi

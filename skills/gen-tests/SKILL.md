---
name: ratchet:gen-tests
description: Generate tests from adversarial findings in debates
---

# /ratchet:gen-tests — Generate Tests from Debate Findings

Turn adversarial critique into permanent test coverage. Scans debate findings and generates tests that encode the quality standards discovered during debates.

## Usage
```
/ratchet:gen-tests              # Generate tests from all recent debates
/ratchet:gen-tests [debate-id]  # Generate tests from a specific debate
/ratchet:gen-tests [pair-name]  # Generate tests from all debates for a pair
```

## Execution Steps

### Step 1: Gather Findings

Read debate round files and extract adversarial findings with severity `critical` or `major`.

For each finding, extract:
- The issue description
- The evidence (test output, reproduction steps)
- The file and location
- Whether it was resolved (check verdict)

### Step 2: Identify Test Gaps

For each finding, check if a test already covers this case:
- Search existing test files for related test names/descriptions
- If a test exists, skip
- If no test exists, queue for generation

### Step 3: Load Testing Spec

Read `.ratchet/project.yaml` testing section to understand:
- Test framework and conventions
- Test directory patterns
- How to run specific test types

### Step 4: Generate Tests

For each unresolved finding, spawn a generative agent to write a test:

```
Based on this adversarial finding from a Ratchet debate:

Finding: [description]
Evidence: [what the adversarial agent demonstrated]
File: [source file]
Test framework: [from project.yaml]
Test directory: [from project.yaml]

Write a test that would catch this issue if it regresses.
Follow the project's existing test conventions.
Place the test in the appropriate directory.
```

### Step 5: Verify Tests

Run the generated tests to verify they:
1. Pass against the current (fixed) code
2. Are syntactically valid
3. Follow project conventions

### Step 6: Report

```
Generated tests from debate findings:

  [debate-id]: [pair-name]
    ✓ test_empty_input_validation (src/__tests__/unit/validation.test.ts)
    ✓ test_n_plus_one_query_prevention (src/__tests__/integration/users.test.ts)
    ⊘ skipped: pagination_edge_case (test already exists)

  [N] tests generated, [N] skipped (already covered)

Run your test suite to verify: [test command from project.yaml]
```

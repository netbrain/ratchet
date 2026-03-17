---
name: ratchet:gen-tests
description: Generate tests from adversarial findings in debates
---

# /ratchet:gen-tests — Generate Tests from Debate Findings

Turn adversarial critique into permanent test coverage — both example-based and property-based. Scans debate findings and generates tests that encode the quality standards and invariants discovered during debates.

## Usage
```
/ratchet:gen-tests              # Generate tests from all recent debates
/ratchet:gen-tests [debate-id]  # Generate tests from a specific debate
/ratchet:gen-tests [pair-name]  # Generate tests from all debates for a pair
```

## Execution Steps

### Step 1: Gather Findings

Read debate round files and extract adversarial findings with severity `critical` or `major`.

If no debates exist, or the specified debate/pair has no adversarial rounds, inform the user:
> "No debate findings to generate tests from. Run /ratchet:run first to produce adversarial findings."

Then use `AskUserQuestion` with options: `"Start a debate (/ratchet:run) (Recommended)"`, `"Done for now"`.

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

### Step 3b: Identify Invariants for Property-Based Tests

Review the adversarial findings for patterns that suggest **invariants** — properties that should always hold, not just specific examples:

- Input validation findings → property: "for all inputs matching X, the function should never Y"
- Data transformation findings → property: "roundtrip encode/decode always preserves data"
- Boundary/edge case findings → property: "for all values in range [min, max], output is within [a, b]"
- State transition findings → property: "no sequence of valid operations produces an invalid state"

For each invariant identified, generate a property-based test using the project's stack:
- **Go**: use `testing.F` / `go test -fuzz` or `gopter`
- **Python**: use `hypothesis`
- **JavaScript/TypeScript**: use `fast-check`
- **Rust**: use `proptest` or `quickcheck`

Property-based tests are higher value than example-based tests — they catch regressions the original finding didn't anticipate. Prioritize generating these when the finding maps to a clear invariant.

### Step 4: Generate Tests

For each unresolved finding, spawn a generative agent to write a test. Use the generative model from `workflow.yaml` (`models.generative`). The agent operates outside the debate loop — it generates tests only, not production code.

Agent spawn configuration:
- `subagent_type`: generative
- `model`: value of `workflow.yaml` → `models.generative` (or `opus` if unset)
- `tools`: Read, Grep, Glob, Bash, Write, Edit
- `disallowedTools`: none

```
Based on this adversarial finding from a Ratchet debate:

Finding: [description]
Evidence: [what the adversarial agent demonstrated]
File: [source file]
Test framework: [from project.yaml]
Test directory: [from project.yaml]

Write a test that would catch this issue if it regresses.
Follow the project's existing test conventions.

If this finding maps to an invariant (identified in Step 3b), write a PROPERTY-BASED test
using the appropriate library for this stack. Property-based tests should:
- Define the property clearly in a comment
- Generate random valid inputs
- Assert the invariant holds for all generated inputs
- Use shrinking to find minimal counterexamples

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
    ✓ test_empty_input_validation (src/__tests__/unit/validation.test.ts) [example]
    ✓ prop_roundtrip_encoding (src/__tests__/property/encoding.test.ts) [property]
    ✓ fuzz_parse_input (src/__tests__/fuzz/parser.test.ts) [fuzz]
    ⊘ skipped: pagination_edge_case (test already exists)

  [N] tests generated ([N] example, [N] property, [N] fuzz), [N] skipped

Run your test suite to verify: [test command from project.yaml]
```

After reporting, use `AskUserQuestion` to guide the user:
- Options:
  - "Run test suite now (Recommended)" — execute the test command from project.yaml
  - "Continue to next milestone (/ratchet:run)"
  - "View quality metrics (/ratchet:score)"
  - "Done for now"

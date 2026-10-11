# Prompts

This file lists the prompts given to the AI assistant (Claude Code) while
building this project, in chronological order. Each prompt is followed by a
short summary of the changes it led to.

- Prompts that shaped the design or content are quoted verbatim.
- Short procedural follow-ups are summarized.

## Before coding: design discussion

Before opening Claude Code, I worked through the design in a separate
conversation with Claude. That's where the decisions in
prompt 1 come from. The main questions I explored:

- Structured operations vs. a single expression endpoint
- float64 vs. decimal, and apd vs. shopspring/decimal (I ran both on the
  same inputs to compare)
- Why not use an existing Go expression library
- Spreadsheet-style vs. phone-calculator-style percent
- One function per precedence level vs. precedence climbing (I compared
  both on the same inputs; they produce identical trees)

## 1. Design and planning request

```text
Hey, I'm working on a take-home assignment: a calculator with a React +
TypeScript frontend and a Go backend. I've already thought through the
main design decisions, so I'd like your help turning them into a plan
before we write any code.

The idea is that the backend has a single endpoint, POST /api/v1/calculate,
which takes an expression string like "2 * (3 + 4) ^ 2", parses it, and
returns the result. It should support addition, subtraction,
multiplication, division, exponentiation (^), square root (sqrt), percent
(%), parentheses, and negative numbers.

I want to write the parser myself rather than use a library, using
precedence climbing for the binary operators so their precedence lives in
one table. I'm also using decimals instead of floats (thinking
cockroachdb/apd), so 0.1 + 0.2 should come out as exactly 0.3, and
numbers go over the API as strings so the frontend doesn't lose
precision.

A few behaviors I care about:
- Percent should work like a phone calculator: 200 + 10% = 220
- Functions like sqrt should be easy to add without touching the parser
- Errors should be clear: invalid input vs. undefined math (like dividing
  by zero), and syntax errors should say where the problem is

On the frontend: a keypad that also works with the keyboard, an Ans key,
and long results shown rounded on screen while the exact value is kept.

I'd like to keep it simple: standard library where possible, a clean
separation between the math, the parsing, and the HTTP layer, and good
tests for each part.

Could you suggest a project structure and break the work into small
steps I can build and test one at a time? No code yet. I'd like to
review the plan first.
```

**Changes:** A plan in chat, with no code:

- Project structure and API contract.
- Design notes on percent semantics, `Ans`, the function registry, the
  precedence table, decimal exactness and input limits.
- 16 implementation steps.

## 2. Plan revisions

```text
this is a good plan. i agree with all decisions but a few changes or adding in plan:
1- parantheses should block the percent role 100 + (10%) should be 100.1 100+ 10% should be 110. S parser should keep parantheses.
2- Keep 400 vs 422 as you described but use codes like syntax_error, unknown_function,division_by_zero...
3- Coverage reports are a required deliverable so add it to plan
4- add a multi-stage Dockerfile at the end
```

**Changes:** The plan was revised:

- Parentheses become a `Group` AST node, which blocks the percent rule.
- An error catalog with specific codes, each mapped to 400 or 422.
- A coverage section.
- A multi-stage Dockerfile section.
- Steps 16–18: static file serving, the Dockerfile, and delivery.

## 3. Final decisions and PLAN.md

```text
aggreed on both. puth the both part in PLAN.md all repo should be english.
```

"Both" refers to two decisions that were still open: `100 + -10%` = `90`, and
coverage reports are committed and reproducible with one command.

**Changes:** Created `PLAN.md`, which merges both planning rounds. All
repository content is in English.

## 4. Step 0 (summarized)

- Approved the plan, including a few decisions the assistant made on its
  own in PLAN.md (input limits, repeated unary minus, graceful shutdown,
  loading state), and asked to start with step 0.
- Asked to keep this prompt log, with the changes listed after each prompt.

**Changes:**

- Git setup.
- Go module with apd and package stubs.
- Vite + React + TypeScript frontend with Vitest and coverage.
- A smoke test, `README.md` and this file.

## 5. Step 1 (summarized)

- Asked to continue with step 1.

**Changes:**

- `arith` package with:
  - a shared apd context (34 digits, decimal128 exponent range, half-up rounding)
  - `Parse`
  - `Add`/`Sub`/`Mul`/`Div`
  - `MathError` with error codes
  - `Format` (plain or scientific notation)
- Table-driven tests with 100% coverage.

## 6. Rounding corrections

```text
1- use half even for rounding. decimal's default rounding is not half up.2- parse literal exactly and only round results. if inputs have bigger than 34 digits, results will be wrong
```

**Changes:**

- Rounding switched to half-even.
- `Parse` keeps every digit of its input; only operation results are rounded.
- Tests for ties-to-even and long inputs. They fail under the old behavior.
- PLAN.md updated to match.

## 7. Final-result rounding and step 2

```text
in sixth step, round the final result also. i committed so you can start step 2
```

**Changes:**

- PLAN.md step 6: the evaluator also rounds the final value to 34 digits.
- `arith` gains:
  - `Pow` (special cases `0^0 = 1` and `0^negative` → `division_by_zero`)
  - `Sqrt`
  - `Percent` / `PercentOf`
  - a function registry with `sqrt` (`Lookup`)
- Tests for edge cases: domain errors, overflow and underflow with huge
  exponents. Coverage stays at 100%.

## 8. Overflow vs. underflow for huge exponents

```text
pow returns wrong errors for huge exponents they returns underflow but it should returm overflow check 0.5^-1E+100
```

**Changes:**

- `Pow` now works out the overflow/underflow direction from its operands.
- Root cause: for negative exponents apd only turns overflow into underflow,
  never the reverse.
- Tests for small and large bases with huge negative exponents. Three of them
  fail without the fix.

## 9. Step 3 (summarized)

- Asked to start step 3 after committing step 2.

**Changes:**

- `parser` package gains:
  - a lexer that produces tokens on demand, each with its position
  - `SyntaxError` with `Code()` and `Pos()`
- Malformed numbers (`5.`, `1.2.3`) and non-ASCII characters are rejected at
  the offending character.
- Table-driven tests with 100% coverage. Each check was removed once to confirm
  a test fails without it.
- PLAN.md step 3 records the identifier rules, the whitespace set and the error
  positions.

## 10. Step 4 (summarized)

- Approved step 3 and asked to continue with step 4.
- Agreed with the step 4 decisions and with fixing the step 7 round trip.

**Changes:**

- `parser` package gains:
  - AST nodes `Number`, `Unary`, `Binary` and `Group`, with an S-expression
    `String()`
  - `Parse`: precedence climbing over a single binding-power table, with
    unary minus between `*` and `^`
  - `NestingError` (`nesting_too_deep`) once more than 100 parentheses are open
- Tests for precedence, associativity, node positions and every error position
  listed in PLAN.md. Coverage stays at 100%.
- Each rule (associativity of `^`, the power of unary minus, the depth limit,
  `Group` nodes) was broken once to confirm a test fails without it.
- PLAN.md step 4 records the error messages, the depth rule and that unary plus
  is not supported.
- PLAN.md step 7: the fuzz round trip goes through a test-only infix printer.
  `String()` prints an S-expression, which the parser cannot read back.

## 11. Step 5 (summarized)

- Asked to continue with step 5 after committing step 4.

**Changes:**

- `parser` package gains:
  - `Variable`, `Call` and `Percent` nodes
  - postfix `%` as the tightest entry of the precedence table
  - function calls with any name and any number of arguments; their
    parentheses count toward the nesting limit
- Tests for the percent AST shapes from PLAN.md, calls, variables, error
  positions inside argument lists and nesting through calls. Coverage stays at
  100%.
- Each new rule (the power of `%`, call parsing, call depth, the `call` tag in
  `String()`) was broken once to confirm a test fails without it.
- PLAN.md step 5 records these rules.

## 12. Step 6 (summarized)

- Asked to continue with step 6.

**Changes:**

- `eval` package:
  - `Evaluate(expression, vars)`, `ParseValue` and `Value`, so the HTTP layer
    depends on `eval` alone
  - the percent rule, with unary-minus unwrapping and `Group` blocking
  - a first pass that checks names and argument counts before anything is
    computed, so an invalid expression always gets a 400
  - math errors at the operator or function that failed
- `arith` gains exact negation (`Neg`) and `Round`, and `Parse` now tells
  overflow from underflow for numbers outside the exponent range.
- `parser.Node` gains `Position()`.
- End-to-end tests with the full percent table from PLAN.md, and every error
  code with its position. Coverage stays at 100%.
- Each rule was broken once to confirm a test fails without it.
- PLAN.md step 6 records the decisions made while implementing.

## 13. `ans` errors are always 400

```text
for step 8: any error from ParseValue for the ans field must be invalid_ans (400), even when it's an overflow or underflow, and add a test for ans "1E+7000"
```

**Changes:** PLAN.md only; the handler is written in step 8.

- Step 8: every `ParseValue` error is `invalid_ans`. The handler maps it
  itself, because the error behind `ans = "1E+7000"` has the code `overflow`,
  which the code → status table would turn into a 422.
- Step 8 tests: `ans` = `"1E+7000"` and `"1E-7000"` → 400 `invalid_ans`.
- The error catalog says that an out-of-range `ans` is `invalid_ans`.

## 14. Step 7 (summarized)

- Asked to continue with step 7.

**Changes:**

- `FuzzParse` (parser): no panics, error codes and positions in range, node
  positions point at their tokens, the tree follows the precedence table, and
  `Parse(infix(ast))` rebuilds the same tree.
- `FuzzEvaluate` (eval, with a fuzzed `ans`): no panics, every error has a
  catalog code and a position in range, and results have at most 34 digits and
  parse back as `ans`.
- Six injected parser bugs (associativity, dropped groups, positions, the power
  of unary minus) each fail the fuzz seeds.
- Bug found while fuzzing: a non-integer power of a base with thousands of
  digits (a crafted `ans`) failed with an error that had no code, and was slow
  well before that. `arith.Pow` now splits such a power into integer and
  fractional parts. Results for bases of up to 44 digits are unchanged, and a
  test checks that the split path matches apd where apd still works.
- PLAN.md step 7 records the invariants and the fix. The README lists the fuzz
  commands.

## 15. `ans` is capped at 34 digits

```text
for step 8: also cap ans at 34 significant digits, more is invalid_ans. check it on the parsed number's digit count.change tests according to that
```

**Changes:**

- `eval.ParseValue` returns `ErrTooManyDigits` for a number with more than 34
  digits, counted on the parsed number: leading zeros do not count, trailing
  zeros do. Results never have more, so an `ans` from the frontend always
  passes. The step 8 handler maps it to `invalid_ans`, like every other
  `ParseValue` error.
- Tests that used a longer `ans` now use a 34-digit `ans` or a long literal.
  New tests cover 34 digits accepted and 35 rejected, including trailing zeros.
- PLAN.md: the error catalog, the limits table, and steps 6, 7 and 8.

## 16. Step 8 (summarized)

- Asked to continue with step 8.

**Changes:**

- `api` package: `NewHandler` serves `POST /api/v1/calculate`.
  - It decodes the body (4 KiB limit, exactly one JSON object, `expression`
    required), checks the expression length (1000) and parses `ans`.
  - Every `ans` error is `invalid_ans`, and errors from evaluation map to 400
    or 422 through one code → status table.
  - Unexpected errors are logged and answered with 500.
- `eval.Codes()` lists every code `Evaluate` can return. The API test checks
  that each one has a status, so a new code breaks the test instead of
  becoming a 500.
- `httptest` tests cover every error code with its position, malformed bodies,
  the limits at and past their bounds, and 405. Coverage stays at 100%.
- Eight handler bugs were injected one at a time; the tests catch each.
- PLAN.md step 8 records the decisions made while implementing.

## 17. Reject unknown JSON fields

```text
i dont agree with unknown json fields. answer key would be silently ignored result would be wrong. i agreed with other decisions.
```

**Changes:**

- The handler decodes the body with `encoding/json/v2` and
  `RejectUnknownMembers`. An unknown name (`answer`), a name with different
  case (`ANS`) or a repeated name is now `invalid_request`, for example
  `unknown field "answer"`.
- Responses are written with v2 as well, and the tests decode them strictly.
- New tests cover these names, invalid UTF-8 and an unreadable body. The new
  name tests fail when unknown names are allowed again.
- PLAN.md step 8 records the rule.

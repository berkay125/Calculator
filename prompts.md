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

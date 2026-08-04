# Component System Contract

Status: ready

## Source-bound choices

Component selection should prefer the stack already in the repo. Do not introduce a second component library without change/refreeze.

## Required components

Primary/secondary/destructive buttons, inputs, cards, badges, tables/lists, empty, error, loading, toast/inline alert, modal/drawer (if used).

## States per interactive component

default · hover · focus-visible · active · disabled · loading · error

## Accessibility

Every interactive control has a name, focus style, and keyboard path.

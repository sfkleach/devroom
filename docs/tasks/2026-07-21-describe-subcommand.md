# Task: Implement the 'describe' subcommand

## Step 1: Basic implementation

`devroom describe` is used to generate descriptions regarding the state of
rooms. 

- `devroom describe NAME` uses the configured AI to generate descriptions
  at three levels: (1) a single line description regarding the purpose and
  progress, (2) a paragraph describing the purpose of the room, the
  working branch, the changes made so far, an indication of progress,
  and a readable summary of the git status, and (3) a one page detailed
  description of the work, including tool stack, state of testing, approach,
  plan etc.

- These different levels are set by the repeating option `-v,--verbose`. Each
  repetition allows for longer replies.


## Step 2: Extend the `list` subcommand

The same function should be available with the `--describe` option on the 
`list subcommand`.

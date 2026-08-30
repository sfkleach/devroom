# Task: Implement the destroy subcommand

The destroy subcommand removes the base image. If a base image does not exist
then it will issue a warning and return a non-zero status. If there are 
still rooms in existence then it will interactively ask if the rooms should
be deleted as well.

- Option '-f,--force', ignore the base image missing condition, ask no 
  questions, assume "yes" throughout.

- Option '-k,--keep-children', do not remove children.

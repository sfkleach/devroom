# Task: Implement the `list` subcommand

The `list` subcommand should show a plain list of the rooms. 

- Option `-b,--branch` should additionally list the branches associated with the room.

 Option `-s,--statistics` includes the container statistics. I think the
  most important are as follows - but please make suggestions for other important
  aspects:
  - When was it built?
  - How big is it?
  - Is it running or stopped?

- Option - `-i,--images` shows the image stack that the container 
  is based on.

- Option `-f,--format=(json|md)` arranges for the output to be in JSON or
  Markdown formats.
  

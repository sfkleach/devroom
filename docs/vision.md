# Using Devroom

Steve has decided he wants to modify his current project `widgetzilla` using his
favourite AI agent. As usual, he has a shell open with the working directory set
to the root folder of `widgetzilla`. So he opens up a new tab (Ctrl-Shift-T),
which automatically opens in the same working directory, and types `devroom`. 

Steve knows how to use `devroom` and has written a small configuration file 
in .config/devroom.toml already with his preferred container runtime. But
this is the first time that Steve has used devroom in this project and a little
interaction follows:

```txt
% devroom
What is this devroom's nickname? (room001): taskbar-rampage
What branch should it use? (taskbar-rampage): add/!!
  No branch `add/taskbar-rampage` exists, create now? (Y/n): y
  Creating branch `add/taskbar-rampage`

Found project folder: /home/steve/projects/widgetzilla
Found origin: https://github.com/sfkleach/widgetzilla
  Organisation: sfkleach
  Project: widgetzilla
Found configuration:
  Base image: ubuntu:22.04
  Container runtime: podman
  Installation script: scripts/jumpstart.sh

There is no ready-made container image, so building the image now. This may 
take a little while ...
```

Steve grabs a coffee while `devroom` creates a temporary Containerfile that 
is then executed to pull the ubuntu:22.04 image, run the installation script
and then launches a persistent container that pulls the repo and switches to
the `add/taskbar-rampage` branch before dropping into a bash.

Steve comes back from the kitchen to see the shell prompt `taskbar-rampage% `
waiting for him. He types `claude` and gets to work.





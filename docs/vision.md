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
No devrooms have been created yet. Press 'n' to create a new room.
Command? (Press '?' for help): n

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
waiting for him. He types `claude` and gets to work ...

... after a while, Steve needs to restart his system. He tidily exits claude 
and then the devroom, then restarts his system. Once the system is back again
he reopens hs terminal, navigates to his repo and runs `devroom`.

```txt
% devroom
Rooms List
1. taskbar-rampage

Command ('?' for help): ?

Summary of commands
  c - configure devroom
  D - destroy the base container image (it will be rebuilt on demand)
  e - enter a room, will prompt for name
  1-9 enter a listed room
  l - list rooms
  n - open a new room
  q - quits devroom (or use ctrl-d)
  s - summarise room activity
  X - exits and closes a room  (container is deleted)
  
Command? (Press '?' for help): s

Rooms summary
1. taskbar-rampage
This room is implementing GitHub issue #123 "Widgetzilla goes on taskbar
rampage". The taskbar is snapshotted and an animation of Widgetzilla, marching
up and down the taskbar, tearing down tall icons and roaring defiantly. The
animation is then played over the taskbar area. Working branch is 
`add/taskbar-rampage`. 

Command? (Press '?' for help): 1
Welcome back to `taskbar-rampage`.
% 
```

A couple of months go by and Steve realises that he hasn't worked on 
Widgetzilla for a few weeks and has stopped work. There's not much point in
keeping the base image. He knows the subcommand for this:

```txt
% devroom destroy -y
Delete container for taskbar-rampage ... done
Delete base-image for widgetzilla ... done
```




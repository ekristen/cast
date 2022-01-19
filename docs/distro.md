# Distro

A Cast Distro is simply a self contained set of Saltstack states that get bundled up by the Cast binary such that they are signed and downloadable by the tool to be installed against a linux system.

If you are familiar with Saltstack this should be a fairly straightfoward process, if you are however unfamiliar with Saltstack some additional learning will be required to get up to speed, to help with that there's a saltstack primer located in this documentation.

## Organization

The most important aspect of a Cast distro is how the salt states are organized. Typically speaking you'd have a directly, let's call it `example` and within it you'd define a state file `server.sls` and this file would do something like `pkg.install` of `htop`. However to allow Cast distributions to be required by other Cast distributions, we leave the creation of the folder `example` up to Cast, instead a manifest is defined to set the name of the project (aka base) of where all the files starting at the root of the repository will be extracted to. The primary purpose for this is to allow another distro to use `git submodules` to essentially include distro A into distro B.

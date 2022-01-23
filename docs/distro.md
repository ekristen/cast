# Distro

A Cast Distro is simply a self contained set of Saltstack states that get bundled up by the Cast binary such that they are signed and downloadable by the tool to be installed against a linux system.

If you are familiar with Saltstack this should be a fairly straightfoward process, if you are however unfamiliar with Saltstack some additional learning will be required to get up to speed, to help with that there's a saltstack primer located in this documentation.

## Organization

The most important aspect of a Cast distro is how the salt states are organized. Typically speaking you'd have a directly, let's call it `example` and within it you'd define a state file `server.sls` and this file would do something like `pkg.install` of `htop`. However to allow Cast distributions to be required by other Cast distributions, we leave the creation of the folder `example` up to Cast, instead a manifest is defined to set the name of the project (aka base) of where all the files starting at the root of the repository will be extracted to. The primary purpose for this is to allow another distro to use `git submodules` to essentially include distro A into distro B.

## Manifest

The manifest is a file that is included with the releases assets on a GitHub Release that provides context to the installer for the distro.

### Specification

```yaml
project_name: sift
base_dir: sift
modes:
  - name: desktop
    state: sift.desktop
  - name: server
    state: sift.server
    default: true
  - name: packages-only
    state: sift.server
    deprecated: true
    replacement: server
  - name: complete
    state: sift.desktop
    deprecated: true
    replacement: desktop
supported_os:
  - id: ubuntu
    release: 20.04
    focal: focal
saltstack:
  pillars:
    distro_user_template: '{{ .User }}'
```

### Modes

Modes are essentially aliases and born from the original install tools from SIFT and REMnux. These `modes` allow for defining a single name alias like `desktop` to point to `sift.desktop`

### Supported Operating Systems (OS)

`SupportedOS` allows for defining what operating systems are officially supported by the distro.

```yaml
supported_os:
  - id: ubuntu
    release: 20.04
    focal: focal
```

### SaltStack

The `saltstack` sections allows for configuring aspects of SaltStack. At this time it only allows for passing custom pillar data.

#### Pillars

Pillars are essentially data that's made available to the SaltStack run, currently it only supports a `key: value` format, it does not support nested data.

!!! important
    The `_template` suffix has special meaning. It indicates a template variable, the value before the `_template` is the actual end result variable name. (eg `sift_user_template` becomes `sift_user`)

```yaml
saltstack:
  pillars:
    sift_user_template: "{{ .User }}"
```

##### Template Data

This is the data available to the template process.

```yaml
user: SUDO_SUER
```

- user, this is the CLI option `--user` but defaults to `SUDO_USER` environment variable, as the tool is only suppose to be run with sudo.
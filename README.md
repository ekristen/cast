# Cast (Status: Beta)

Cast is an installer tool for any Cast distro like [SIFT](https://github.com/teamdfir/sift) or [REMNUX](https://github.com/remnux/remnux).

[![asciicast](https://asciinema.org/a/463178.svg)](https://asciinema.org/a/463178)

## Usage

```bash
cast install <owner/repo|alias>
```

**Note:** there are only two aliases currently supported, `sift` and `remnux` which are resolved to `teamdfir/sift-saltstack` and `remnux/salt-states` respectively.

## Dependencies

* [cosign](https://github.com/sigstore/cosign) - required for release and verification

## What is a Cast Distro?

Simply put a cast distro is one published by the cast binary, however it's a bit more complicated than that.

The underlying technology used for installing the distro is Saltstack.

There are only two (2) version 1 cast distros out there and they are SIFT and REMnux and what makes them a v1 is the way the release files are generated and signed and how the saltstack files are organized in the repository.

A version 2 cast distro is one like [cast-example-distro](https://github.com/ekristen/cast-example-distro) where the salt states start in the root of the project and the release is generated by the `cast release`. The reason for moving the states to the root is for `git submodule` usage for distros that extend another distro.

## Configuration

Cast is configured via the `.cast.yml` file in the root of the cast distro project. This configuration is required and contains important information like what GitHub Repository should releases be published to, what the manifest file contents should be.

Part of the configuration is the `manifest` definition that ultimately gets uploaded as a release asset. The manifest dictates things like `base`, `modes`, and `supported operating systems`.

* `name` - the name of the distro
* `base` - (optional) this is the name of the base directory where the saltstack files exist in the project root
* `modes` - this is a way to define modes that the user can specify along with the default mode, if none is specified
* `supported_os` - this is a way to define what operating systems are supported

### For Developers

If your distro is called `alpha` then your `name` should be `alpha`, all base saltstack states should be based out of the root of the project, however if all the states have to be in a subdirectory, then the `base_dir` must be set to the name of that subdirectory.

### Aliases

Aliases are only for SIFT and REMnux and not intended for long term use. The aliases serve a more specific purpose for supporing backwards compatibility for older versions of SIFT and REMnux that were not released as a **cast** distro originally.

## Developers

Developing a distro for cast is very simple, this documentation will walk you through cast initialization.

### Saltstack

To make embedding a distro easier in another distro both for the community and for SANS builds purposes, the salt states have now been moved to the root of the project. This allows for a repo to be submoduled into another repo and the `manifest.yml` dictates to the installer how things should be extracted and installed.

### Release

Requirements

* Tag must be created outside of the tool and pushed to the remote
* Cosign private key and public key must be present in the repo

Creating a release has never been more simply. However one thing is required, that you tag and push the tag to GitHub prior to running the command.

```bash
git tag v1.0.0 && git push origin --tags
```

Then you simply run the release command from your local branch.

```bash
cast release
```

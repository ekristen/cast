# Migrating to Cast

This is a migration guide for existing distros that are using some variation of the `sift-cli` like SIFT or REMnux.

## Overview

Cast is a singlular tool designed to install, test and release cast compatible distributions that are built with SaltStack.

It uses a single file in the project root called `.cast.yml` to define how releases and installations take place.

Cast uses [cosign]() to sign all releases but supports the legacy PGP signing that the `sift-cli` and `remnux-cli` uses.

## Design

A Cast distro is a bunch of SaltStack states and there's and how they are organized is very flexible but there is a recommended
base organization. To support embedding states from one project into another project using git submodules, it is recommended
that all base states start in the root of the project. A base state is generally a target for building like `desktop` or `server` or
`dedicated` or `standalone`. All other states are encourage to be in subdirectories that make sense from a human readability and organization
standpoint.

This design allows for a distro to be submoduled into another distro.

### Development

During development of a Cast distro, Cast provides testing tools but if you'd like to use SaltStack directly you simply need to honor a few
guidelines for how to setup your development environment. For example if you are building a distro called `example` you'd want your file root
for saltstack to be `/tmp/salt` with your Cast distro cloned into `/tmp/salt/example`. 

## Migrating

To migrate to Cast and to use Cast to manage all your releases there are a few steps that must be taken.

1. Create a Cosign Private and Public Key
2. Ensure you have your PGP Private and Public Key
3. Create a `.cast.yml` file and populate it properly.
4. Modify .gitignore
5. Commit changes
6. Create a tag
7. Create a release

### Migration by Example

The current version of SIFT is not in the recommended format. All base states exist in a subfolder within the root of the project, but Cast is designed
to handle this scenario if it's required.

To migrate SIFT as it is currently, the Cast file would look like the following. 

```yaml
version: 2
name: sift
base_dir: sift
modes:
  - name: server
    state: sift.server
    default: true
supported_os:
  - name: ubuntu
```

Since all the states exist within the `sift` folder we define the `base_dir` as sift, this is how we ensure files get packed and unpacked properly for execution.
The modes are a carry over from the sift-cli and the easy modes that make represent an installation mode for the distro, modes allows for a default to be specified
and what the modes are and what state should be called as a result, this allows for a great deal of flexibility by a distro author.

Finally the supported OS provides a way to indicate if the target OS is supported or not.

Once the `.cast.yml` is in place and committed to the repository, we can create our first tag and publish using Cast. Since we want to maintain backwards
compatibility with the `sift-cli` we'll enable legacy signing using PGP.

First, update your `.gitignore` and ignore `*.key`

Second, create your cosign keys.

```bash
cosign generate-keypair
```

Third, copy your PGP keys as `pgp.key` and `pgp.pub`.

Fourth, create a new tag and push it to the remote.

Finally, run Cast to release. 

```bash
cast release --legacy-pgp-sign
```

This will do some basic validation and package the distro for release and upload all the assets to GitHub Releases.

Congrats!


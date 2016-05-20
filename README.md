deviced
=======

`deviced` is an embedded Docker device manager. It controls a local Docker Daemon, and handles versioning, uptime management and logging, and other general management of local containers.

`deviced` has its own API which accepts a configuration document describing the target state of the device. This state is a container list. Each container in the list has information about the docker image and tag/version the container should originate from. Rather than specify a single target version for a container, a list of acceptable versions can be specified in order of priority.

For all containers that are not running at the desired (best priority) version, `deviced` will ping known registries for available versions of the container. It will then select the best available version of the image, from the best (closest) registry, and then pulls this version and swaps the container to the latest.

This allows a distributed model for registries. Registries can be distributed over many different servers and updates can be distributed to the registries over time.

`deviced` is designed to run within a Docker container, and can update itself. `deviced-bootstrap` is a minimal bootstrapper that can load a compiled version of `deviced` into the Docker daemon to bootstrap the daemon.

Startup Process
===============

This is the process:

 - Load config file
 - Setup API endpoint
 - Start Docker sync loop.
  - Loop checks available list of images, current running list of containers, attempts to reconcile.
 - Start image sync loop.
  - Loop checks list of images, target list of containers, identifies images that are needed, queries registries...
  - If this loop identifies that no changes need to be made it will exit.
  - The loop can be started again on a configuration reload event.

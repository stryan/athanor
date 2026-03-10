# Athanor

A backup tool for [Materia](https://primamateria.systems) to safely backup volumes on a regular basis.

By default it will perform a `podman volume export` for every volume in every component, stopping and starting containers as needed per volume.

Configuration is done with `[X-Athanor]` groups in the Quadlet files, or as part of the component manifest.

## Building

Requires Go 1.26. Build using `mise build` to produce amd64 and arm64 binaries in `/bin`.


## Running

Athanor follows the same conventions as Materia:

- For root users it will look in `/etc/containers/systemd/` and `/var/lib/materia/components/` for components.
- For non-root users it will look in `$XDG_CONFIG_HOME/containers/systemd` and `$XDG_DATA_HOME/materia/components/` for components.

Use `athanor plan` to see what athanor will do if run. By default plan will backup all components, but you can specify specific targets on the command line:

~~~default
hostname:~ # ./athanor plan
Plan:
1. (act) Stop Container act_runner.container
2. (act) Dump Volume act_runner-data.volume
3. (act) Start Container act_runner.container
4. (beszel-server) Stop Container beszel-server.container
5. (beszel-server) Dump Volume beszel-data.volume
6. (beszel-server) Start Container beszel-server.container
7. (mumble) Stop Container mumble-server.container
8. (mumble) Dump Volume mumble-data.volume
9. (mumble) Start Container mumble-server.container
10. (uptimekuma) Stop Container uptimekuma.container
11. (uptimekuma) Dump Volume uptimekuma-data.volume
12. (uptimekuma) Start Container uptimekuma.container
~~~

To backup a single component, use `-n`
~~~default
hostname:~ # ./athanor plan -n uptimekuma
Plan:
1. (uptimekuma) Stop Container uptimekuma.container
2. (uptimekuma) Dump Volume uptimekuma-data.volume
3. (uptimekuma) Start Container uptimekuma.container
~~~


Use `athanor backup` to actually run the backup:

~~~default
hostname:~ # ./athanor backup -n uptimekuma
Plan:
1. (uptimekuma) Stop Container uptimekuma.container
2. (uptimekuma) Dump Volume uptimekuma-data.volume
3. (uptimekuma) Start Container uptimekuma.container

hostname:~ #
~~~

## Configuration

Configuration can be provided as either a `TOML` file or with `ATHANOR_` environmental variables

Main command config options:
- `quadlet_dir`: Where to look for quadlet files e.g. `/etc/containers/systemd`
- `data_dir`: Where to look for Materia component data files e.g. `/var/lib/materia/components`
- `output_dir`: Where to output the backup files. Defaults to `/var/backups` or `$XDG_DATA_HOME/backups`
- `compression_command`: What command to use to compress the backups, if any
- `compression_suffix`: What file suffix to add to compressed backups


The actual backup configuration is done on the `.volume` or `.container` Quadlet files. The following options are valid:

- `Skip`: Do not do anything with this resource
- `InPlace`: For containers, do not stop/start this container during the backup. For volumes, do not start/stop the containers mounting the volume
- `Group`: Optional group name
- `DumpCommand`: For containers, a command to perform in the container with `podman exec`

Backup configurations can also be provided in a components `MANIFEST.toml` like so:

~~~toml
[Volumes.cache-data]
Skip=true
~~~

This is equivalent to:

`cache-data.volume`
~~~ini
[Volume]

[X-Athanor]
Skip=true
~~~

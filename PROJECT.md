# Project 1 — Build Your Own Container Runtime (mini-Docker)

Operating Systems · Course Project 1

---

## Overview

In this project you will build `minicontainer`, a command-line tool that runs a program inside an isolated environment, the way `docker run` does. You will build it directly on top of Linux kernel features, without using any container libraries. Your second and third workshop of the semester will be dedicated to Docker and container concepts.

The isolation comes from three kernel mechanisms:

- **Namespaces** give the process its own view of process IDs, hostname, mounts, IPC and network.
- **cgroups v2** cap how much CPU and memory the process can use, and how many processes it can create.
- **overlayfs + `pivot_root`** give the process its own root filesystem, built from an image, that it cannot escape.

By the end you should be able to back this claim with evidence from your own code: *a container is not a virtual machine; it is an ordinary Linux process that the kernel shows a restricted view of the system.*

This project is about the isolation mechanism itself. Image registries, orchestration and user experience are out of scope.

This document describes **what** you must build and how it will be evaluated. **How** to build it is up to you: researching the kernel interfaces involved is part of the project.

---

## Learning objectives

After completing this project you should be able to:

1. Explain what each Linux namespace type isolates, and create processes in new namespaces.
2. Explain why PID 1 is special inside a PID namespace, and handle its responsibilities correctly.
3. Build an isolated root filesystem from an image, and explain why `chroot` alone is not enough.
4. Explain mount propagation, and why the container's mounts must not leak back to the host.
5. Use cgroups v2 to enforce CPU, memory and process-count limits, and explain what the kernel does when a limit is hit.
6. Reason about what a container does **not** isolate, and why containers are a weaker security boundary than virtual machines.

---

## Logistics

| Item | Details |
| --- | --- |
| Language | Go or Rust is recommended. You may use another language if you can justify it in your report. |
| Environment | A Linux virtual machine (see below) |
| Kernel | Linux 5.10 or newer, with cgroups v2 |

**Use a virtual machine.** Your program runs as root and changes mounts, cgroups and network configuration. A bug can damage the host system. Develop and test in a disposable VM and take a snapshot before you start. WSL2 and Docker-in-Docker are not supported environments.

---

## The big picture

Your container is built by combining the following kernel features. You are expected to research each one: what it does, which system calls control it, and how the pieces must be ordered.

| Mechanism | Role in your container |
| --- | --- |
| PID namespace | The container sees only its own processes; its first process is PID 1 |
| UTS namespace | The container has its own hostname |
| Mount namespace | The container has its own mount table, isolated from the host's |
| IPC namespace | The container has its own System V IPC objects and POSIX message queues |
| Network namespace | The container has its own network stack, separate from the host's |
| cgroups v2 | The container's CPU, memory and process count are limited |
| overlayfs | The image stays read-only; the container's changes go to a separate writable layer that is thrown away on exit |
| `pivot_root` | The container's root is switched to the overlay, and the host's filesystem is removed from view |

---

## CLI specification

Your tool must support this interface:

```
sudo ./minicontainer run [options] <rootfs-dir> <command> [args...]
```

| Option | Meaning | Default |
| --- | --- | --- |
| `--hostname <name>` | Hostname inside the container | `container` |
| `--mem <size>` | Memory limit, e.g. `64M`, `1G` | No limit |
| `--cpus <float>` | CPU limit in cores, e.g. `0.5`, `2` | No limit |
| `--pids <n>` | Maximum number of processes | No limit |
| `--name <id>` | Container name | Random ID |

`<rootfs-dir>` is an unpacked root filesystem that serves as the container's image. We suggest the [Alpine mini root filesystem](https://alpinelinux.org/downloads/). Your tool must never modify this directory.

Required behavior:

- **Interactive use.** The command is attached to the user's terminal, so `run ... /bin/sh` gives a working interactive shell.
- **Exit status.** `minicontainer` exits with the container command's exit code, or `128 + signal number` if the command was killed by a signal.
- **Cleanup.** When the container exits, for any reason, everything your tool created for it is removed.
- **Concurrency.** Several containers can run at the same time without interfering with each other.
- **Errors.** Problems such as a missing rootfs, a bad option, not running as root, or an unsupported system produce a clear error message and a non-zero exit code, never a crash.

Example session:

```
$ sudo ./minicontainer run --hostname box --mem 64M --cpus 0.5 ./images/alpine /bin/sh
/ # hostname
box
/ # ps
PID   USER     TIME  COMMAND
    1 root      0:00 /bin/sh
    2 root      0:00 ps
/ # exit 3
$ echo $?
3
```

---

## Milestones

We recommend building the project in this order. Each milestone produces something you can run and check, and each builds on the previous one.

### Milestone 1 — Process isolation

The command runs in its own PID, UTS and IPC namespaces, and its hostname is set from `--hostname`.

**Done when:**

- The container shows the chosen hostname, and the host's hostname is unchanged.
- The container's first process is PID 1.
- The tool reports the command's exit code.

### Milestone 2 — Filesystem isolation

The container gets its own mount namespace and a root filesystem built from the image with overlayfs. The tool switches into that filesystem with `pivot_root`. The container has working `/proc`, `/dev` and `/tmp`.

**Done when:**

- Process-listing tools inside the container show only the container's processes.
- The container sees the image's filesystem, and no host files are reachable from inside.
- Container mounts are not visible on the host.
- Changes made inside the container disappear when it exits, and the image is left unmodified.

### Milestone 3 — Resource limits

Each container gets its own cgroup, configured from `--mem`, `--cpus` and `--pids`. The limits apply from the moment the user's command starts.

**Done when:**

- A process that exceeds the memory limit is killed, and the tool tells the user why.
- A CPU-bound process uses no more than the configured CPU share.
- A fork bomb cannot exceed the process limit, and the host is unaffected.

### Milestone 4 — Network isolation

The container has its own network namespace.

**Done when:**

- The container has only a loopback interface, and that interface works.
- The container cannot reach the host network or the outside world.

### Milestone 5 — Robustness

**Done when:**

- Signals sent to `minicontainer`, such as Ctrl+C, reach the container and end it cleanly.
- PID 1 responsibilities, including reaping zombie processes, are handled, or your report explains why your design does not need to handle them.
- Every exit path leaves nothing behind: normal exit, error, signal, and OOM kill. After many consecutive runs there must be no leftover cgroups, mounts or directories.

---

## Bonus challenges

These are harder, open-ended parts for students who want to go deeper. Each is worth up to **+5 points**, for a maximum of **+15**.

- Bonuses are graded only once Milestones 1–5 work.
- A single challenge done well is worth more than three half-finished ones.
- Each bonus needs its own section in the report, covering what you built, the main difficulty you hit, and how you tested it.

### B1 — Pull and run real images

Add a `pull` command that downloads an image from Docker Hub, and let `run` accept an image name instead of a directory:

```
sudo ./minicontainer pull alpine:3.20
sudo ./minicontainer run alpine:3.20 /bin/sh
```

Requirements:

- Implement the registry protocol yourself, including authentication, manifest resolution for your CPU architecture, and integrity verification of downloaded data.
- Handle multi-layer images correctly, including files that a later layer deletes.
- Store a layer shared by two images only once, and use the layers as separate overlay layers rather than merging them into one directory.
- Honor the image's default command, environment variables and working directory.

You must demonstrate at least two different images.

### B2 — Container networking

Give containers network access similar to Docker's default bridge network:

- Containers can reach each other and the internet.
- Each container gets its own IP address.
- A `-p <hostport>:<containerport>` option makes a service running inside a container reachable from the host.
- All network configuration created for a container is removed when it exits.

You must demonstrate two containers talking to each other, internet access from inside a container, and a web server inside a container reached from the host through `-p`.

### B3 — Security hardening

Make your container resist a hostile process running inside it:

- **Rootless mode.** A `--rootless` flag lets an unprivileged user run containers. The user is root inside the container but unprivileged on the host. Document which features still work in this mode and which do not.
- **Least privilege.** The container process runs with only a minimal set of privileges.
- **System call filtering.** Dangerous system calls are blocked, and calling them returns an error rather than killing the process.

You must demonstrate privileged operations failing inside the container. You must also explain one real container-escape technique that your hardening blocks.

---

## Rules

**You make the system calls yourself.** Anything that implements the isolation for you is not allowed. This includes:

- container runtimes and container libraries, such as Docker, Podman, runc, crun, containerd, LXC and libcontainer;
- shelling out to utilities such as `unshare`, `nsenter`, `chroot`, `mount` or `cgcreate` for the core milestones.

**Exceptions:**

- Thin system-call wrappers are fine, for example Go's `x/sys/unix` or Rust's `nix`.
- General-purpose libraries are fine, for example for argument parsing, HTTP, JSON or archives.
- For B2 only, you may use host networking utilities.

**AI tools are allowed.** However, you must be able to explain every part of your code, and you must understand every concept the project covers. Expect detailed questions during the demo. If you cannot explain your code, it is treated as code you did not write.

---

## Verification

During your demo, graders will check the properties below, along with some unannounced tests. You should test all of them yourself before submitting.

| # | Property | Expected result |
| --- | --- | --- |
| 1 | PID isolation | Only the container's processes are visible; the command is PID 1 |
| 2 | Hostname | Container uses the `--hostname` value; host is unchanged |
| 3 | Root filesystem | The image's filesystem is visible, not the host's |
| 4 | No escape | No host files are reachable from inside the container |
| 5 | Copy-on-write | Image unchanged after the container modifies files; changes are gone after exit |
| 6 | Mount leakage | No container mounts appear on the host |
| 7 | Memory limit | Exceeding `--mem` kills the process, and the tool reports it |
| 8 | CPU limit | CPU usage stays within `--cpus` |
| 9 | PID limit | A fork bomb cannot exceed `--pids`; host unaffected |
| 10 | Network | Only a working loopback interface; no external access |
| 11 | Exit code | Matches the container command's exit code |
| 12 | Signals | Ctrl+C ends the container promptly and cleanly |
| 13 | Cleanup | Nothing left behind after many runs, including failed ones |
| 14 | Concurrency | Two containers run at once without interfering |
| 15 | Errors | Clear message and non-zero exit; no crash |

---

## Deliverables

1. **Source code**, with build and run instructions.
2. **Report** (mandatory). This is a substantial part of your grade, so treat it as real technical documentation. It must include:
    - **Design.** The architecture of your tool: what the parent process does, what the container process does, and in what order. Include a diagram.
    - **Justification.** Why the setup steps must happen in the order you chose.
    - **Language choice.** Why you chose your language. This is especially important if you did not use Go or Rust.
    - **Evidence.** Proof of each verification property, using terminal output or screenshots.
    - **Status.** What works, what does not, and any known limitations.
    - **Bonus sections.** One section for each bonus challenge you attempted.
    - **Answers.** Your answers to the questions below.
3. **Live demo** with the TA. You will run your tool and answer questions about your code and the underlying concepts.

### Report questions

Answer each question in a short paragraph. Refer to your own code where you can.

1. Your container's shell is PID 1. What is different about being PID 1 inside a namespace, with respect to signals and orphaned children? What did you do about it?
2. Why is `chroot` alone not enough for filesystem isolation? Describe a concrete way a root process could escape a `chroot`, and explain why `pivot_root` prevents it.
3. What would go wrong if the container's mount changes propagated to the host? How did you prevent this?
4. What exactly happened when your container exceeded its memory limit? Which process was killed, and how did your tool find out?
5. With `--cpus 0.5` on a 4-core machine, can the container's threads ever run on more than one core at the same instant? Explain how the CPU limit is enforced.
6. Name three things that are **still shared** between your container and the host. For each one, describe a risk it creates.
7. Why do security-sensitive environments, such as cloud providers and CI services, often run containers inside virtual machines? Answer in terms of kernel attack surface.

---

## Grading

| Component | Points |
| --- | --- |
| Milestone 1: process isolation | 10 |
| Milestone 2: filesystem isolation | 20 |
| Milestone 3: resource limits | 15 |
| Milestone 4: network isolation | 5 |
| Milestone 5: robustness | 10 |
| Code quality and error handling | 10 |
| Report | 15 |
| Demo and oral questions | 15 |
| **Total** | **100** |
| B1: pull real images (bonus) | +5 |
| B2: container networking (bonus) | +5 |
| B3: security hardening (bonus) | +5 |

Credit depends on working features and on the report:

- A feature earns credit only if it works in the demo.
- A feature that only partly works can earn partial credit if your report documents its limitations honestly.
- A failure that your report does not mention earns no credit.

---

## Starting points

These are the primary references for your own research:

- Linux man pages for `namespaces(7)`, `cgroups(7)`, `pivot_root(2)`, `clone(2)` and related pages ([man7.org](https://man7.org/linux/man-pages/))
- Kernel documentation: [Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html) and [Overlay Filesystem](https://docs.kernel.org/filesystems/overlayfs.html)
- For B1: the [OCI Distribution](https://github.com/opencontainers/distribution-spec) and [OCI Image](https://github.com/opencontainers/image-spec) specifications

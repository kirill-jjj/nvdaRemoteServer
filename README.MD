# Introduction

Welcome to the server for the NVDARemote addon, written in the [Go programming language.](https://golang.org/) The idea for this program was enspired by [the one released here.](https://github.com/jmdaweb/NVDARemoteServer)

The original server for the addon is written in [Python,](https://www.python.org/) which works well enough under most circumstances. However, there were a few reasons why I wanted to write the server in Go.

- Performance. Go can be much faster than Python if you write your program properly.
- Automatic generation of the self-signed certificate, if desired. Both the addon and the server utalize self-signed SSL certificates, and the addon originally didn't verify a certificate's authenticity before connecting. This has since been updated. I wanted a program that would automatically generate the needed certificate, store it in memory, and use it until the program is terminated.
- Less memory should be used by the Go program.
- Easy to compile on other operating systems without changing the code base much, if at all.
- Can be compiled into a static, position-independent binary if desired, for use across systems using different libraries.
- Should be able to easily upgrade a program to utalize the latest version of Go without changing API calls within the program.


## Goals

My goals for this project are to create a stable server for the NVDARemote addon, which will be fast and have an efficient memory footprint. Configuration of the program will be done through the command line. Any log output can easily be redirected by the various facilities across operating systems, so a log file wouldn't need to be implemented by the program.


# Downloading / installation

## Binary releases

On the [releases](https://github.com/tech10/nvdaRemoteServer/releases) page, there are compiled binaries for various operating systems and architectures that Go supports. I have made an effort to ensure each binary is a completely static build, and thus, not dependent on any libraries that may or may not be used in different operating systems upon which the program can be run. Since there are numerous systems and configurations available for use, the download and installation of the server's binary builds won't be completely covered in this documentation.

Due to some recent deprecation notices in Goreleaser, the following changes have been made to the archive file format.

- The architecture format has changed. Most notibly, x86_64 is now amd64, i386 is 386, aarch64 is arm64.
- Operating system formats have been changed. For the most part, the capital letters have been removed from the operating system formats. Windows is windows, OpenBSD is openbsd, etc. The most significant of these changes is that macOS will be replaced with darwin. Any scripts that update the server from the GitHub releases will need to be updated accordingly, so as to reflect these changes.

Some basic instructions to download and use the binary on Linux using a x86_64 architecture would be the following, for example.

```console
# Assume wget is installed.
# First, download the latest archive.
$ wget https://github.com/tech10/nvdaRemoteServer/releases/latest/download/nvdaRemoteServer_linux_amd64.tar.gz
# Use tar to extract the archive and change to the directory.
$ tar -axf ./nvdaRemoteServer_linux_amd64.tar.gz
$ cd ./nvdaRemoteServer_linux_amd64
# Copy the binary to the users bin directory, for example.
$ cp -a ./nvdaRemoteServer ~/bin/
# Clean up after yourself by removing what you don't need.
$ cd ../
$ rm -r ./nvdaRemoteServer_linux_amd64*
```

A sample systemd service file is also available within the archive, which is for use on systems using systemd. Configuration of systemd with this server is outside the scope of this documentation, as there are numerous configurations available for use with systemd.


### Note on creating packaged releases

There are currently no packaged releases of this program, requiring manual installation. If this server is packaged into a release that can be easily installed on an operating system, please include the license along with the binary.


## Go

With the latest version of Go installed, use the following command.

```console
$ go install github.com/tech10/nvdaRemoteServer@latest
```

This should download and compile the latest code, placing the binary within your GOBIN environment variable. Presuming you have the GOBIN in your path, you will be able to execute it fairly easily. If not, you will need to update your path or execute the binary with its full path.


### Static build

If you require a completely static build of the program, you can clone the GitHub repository and execute the bash script. So long as you have [musl libc](https://www.musl-libc.org/) installed, and musl-gcc in your path, you can do the following:

```console
$ git clone https://github.com/tech10/nvdaRemoteServer
$ cd ./nvdaRemoteServer
$ ./build-static.sh
```

Installing musl libc is beyond the scope of this document, and is specific to your Linux distribution.


## Docker

Thanks to a feature request in [issue 1,](https://github.com/tech10/nvdaRemoteServer/issues/1) a Dockerfile has been added. Automatically built and published images with GitHub actions are currently placed on the GitHub Container Registry, a service that is in public beta. As such, this service is subject to change until it's announced as stable. I don't anticipate many changes to the service, so everything ought to continue working as is.

All example commands expose the host networking to the docker image, which is the quickest means of hosting the service without any difficulty. There are other ways, but they won't work as well with IPV6 unless you set it up, and are beyond the scope of this document.

By default, the docker image will contain the certificate within the GitHub repository, and utalize it. This will prevent entropy difficulties on systems that may not be able to generate their own certificates quickly. This can be altered by the user at the run time of the Docker image. Presuming your image is tagged nvdaremoteserver, here is a sample command.

```console
$ docker run --network host nvdaremoteserver /nvdaRemoteServer
```

This will run the Docker image with no parameters, allowing the server to choose its own defaults. This will prevent it from automatically using the included certificate within the git repository. To use the included certificate, you can use the minimal examples below.


### Downloading the Docker image

#### GitHub Container Registry

The automatically built images are [located here.](https://github.com/users/tech10/packages/container/package/nvdaremoteserver-docker) Provided on that page is a command you can copy to the clipboard which should pull the latest image. Here is an example use of downloading and running the image.

```console
$ docker pull ghcr.io/tech10/nvdaremoteserver-docker:latest
$ docker run --network host ghcr.io/tech10/nvdaremoteserver-docker:latest
```


#### Docker Hub

Images are available on [Docker Hub.](https://hub.docker.com) They are not automatically built as they once were, due to that feature leaving free accounts on June 18, 2021. The images may not be as up to date as those on GitHub. Here is an example to pull the latest image and run it.

```console
$ docker pull tech10/nvdaremoteserver
$ docker run --network host tech10/nvdaremoteserver
```


### Manually build docker image

Clone the repository, and from within the directory, build a Docker image, then run it. Some sample commands are below, one of which will update the certificate from the repository, rewriting it to a freshly generated certificate. To update the certificate, you need go installed, as documented within the shell script. If you simply want to build the docker image, remove the command updating the certificate. You only need Docker installed in order to build the image.

```console
$ git clone https://github.com/tech10/nvdaRemoteServer
$ cd ./nvdaRemoteServer
$ ./update-cert.sh
$ docker build -t nvdaremoteserver-docker .
$ docker run --network host nvdaremoteserver-docker
```


# About the included certificate

The included certificate file is a single file that contains an automatically generated self-signed certificate from this program, along with its private key. They are both encoded in the same pem format that the official addon uses. Before every release, a new certificate file is generated and uploaded to the GitHub repository. For now, this is how it is placed in the Docker images, and how it is made available to other users. This is subject to change in the future.


# Usage

```console
$ nvdaRemoteServer [-configfile /etc/NVDARemoteServer.conf] [-interface 0.0.0.0] [-interface6 ::] [-port 6837] [-port6 6837] [-certfile /path/to/ssl/certificate] [-keyfile /path/to/ssl/key] [-pidfile /path/to/pid/file] [-logfile /path/to/log/file] [-loglevel 2] [-motd "Example message of the day."] [-motd_force_display=false] [-includeTracebacks=false] [-timeout 5.0] [-ping_time 300] [-allowedMessageLength 0]
```

Please note that the brackets around a parameter indicate that it is optional.

The command line parameters are a near-exact mirror of the original
[Python NVDARemoteServer](https://github.com/jmdaweb/NVDARemoteServer), both
in name and in behavior. Parameters with an underscore, such as
`-ping_time`, can also be written with two dashes, e.g. `--ping_time=60`,
which the Go flag package accepts.

## Configuration file

Like the Python server, this server reads a configuration file in the
`option=value` format (UTF-8). The default location on Linux is
`/etc/NVDARemoteServer.conf`; on macOS it is `/etc/NVDARemoteServer.conf`
as well. The `-configfile` parameter overrides the location.

The format is identical to the Python server's:

```console
# Comments start with a hash sign.
port=6837
port6=6837
interface=0.0.0.0
interface6=::
logfile=/var/log/NVDARemoteServer/NVDARemoteServer.log
pidfile=/run/NVDARemoteServer/NVDARemoteServer.pid
loglevel=2
keyfile=/usr/share/NVDARemoteServer/server.pem
certfile=/usr/share/NVDARemoteServer/server.pem
motd=Welcome to my server!
motd_force_display=0
includeTracebacks=0
allowedMessageLength=0
timeout=5.0
ping_time=300
```

- Unknown options are ignored, exactly like the Python server (so a
  configuration file written for the Python server works as is).
- Boolean options (`motd_force_display`, `includeTracebacks`) take `0` or `1`.
- Command line parameters always take priority over the configuration file.
- If the configuration file can't be opened, the program prints
  `Can't open the configuration file, using default or commandline values`
  and continues, just like the Python server.
- `includeTracebacks` is accepted for compatibility, but has no effect:
  this Go server has no tracebacks.

If the default certificate file (`/usr/share/NVDARemoteServer/server.pem`)
does not exist, the server generates its own self-signed certificate in
memory, so it works out of the box. To use a real certificate, point
`certfile`/`keyfile` at your files (see Let's Encrypt below).

## Parameters

### Optional

#### `-configfile`

Path to a configuration file in the Python NVDARemoteServer format.
Defaults to `/etc/NVDARemoteServer.conf` on Linux and macOS.

#### `-interface`

IPv4 interface the server will listen on. This does not affect IPv6
interfaces. An empty value (the default) means all IPv4 interfaces.

#### `-interface6`

IPv6 interface the server will listen on. This does not affect IPv4
interfaces. An empty value (the default) means all IPv6 interfaces.

#### `-port`

TCP port the server will listen on for IPv4 connections. The port must
be between 1 and 65535. Defaults to 6837.

#### `-port6`

TCP port the server will listen on for IPv6 connections. The port must
be between 1 and 65535. By default, uses the value specified in `-port`.

#### `-certfile`

This is the path to the SSL certificate the program will use to
communicate securely, as the NVDA addon uses TLS for secure
communication. If the file does not exist, the server generates a
self-signed certificate in memory instead.

#### `-keyfile`

This is the path to the SSL key. Both the certificate and key file need
to exist and be accessible by the program, or the program will fall back
to generating its own certificate.

If the certificate and key files both exist and fail to load a valid SSL
key pair, the program will terminate rather than falling back on
automatic self-signed SSL key generation.

#### `-pidfile`

Path to a file where the process ID is stored. Defaults to
`/run/NVDARemoteServer/NVDARemoteServer.pid` on Linux.

This file will only be created once the server has successfully started,
and will be removed upon shutdown.

#### `-logfile`

Choose a file for the program to log its data. Any logged information
will always be sent to the console, but in addition to this, a log file
can also be used. Defaults to `/var/log/NVDARemoteServer/NVDARemoteServer.log`
on Linux. If the file can't be created, the program falls back to
console logging only.

#### `-loglevel`

This will choose what you want logged. The default level is 2.

##### Logging Levels

Each level above the previous will also log what the prior level is
logging. For instance, 1 will log both levels 0 and 1.

- -1 will disable logging, with the exception of error messages, which are always logged. This includes panicks, which crash the program.
- 0 will log when the server has started, stopped, or if an error has occurred that isn't severe enough to be logged at all times.
- 1 will log information about which clients connect, logging both their ID and IP address.
- 2 will log what channels each client joins and leaves, which will contain channel passwords. Don't use this log level in production.
- 3 will log what the program is doing at each stage of its operation. Use this for debugging purposes only.
- 4 will log the protocol that the server and client are exchanging. Don't use this unless you're a developer or you want to annalize the protocol being used.

#### `-motd`

Enter a message of the day for your server. You probably want to quote
this string in the shell, ensuring spaces will be escaped properly.

#### `-motd_force_display`

Force the client to display the message of the day from the server, even
if it hasn't changed since you last connected. This value is a boolean,
so it can be true, false, 1, or 0.

#### `-includeTracebacks`

Accepted for compatibility with the Python server. This Go server has no
tracebacks, so the option has no effect.

#### `-timeout`

Maximum time, in seconds, that a client can be connected without
negotiating a TLS connection before the connection is dropped. Values
below 1.0 are reset to the default of 5.0 seconds.

#### `-ping_time`

Interval, in seconds, at which the server pings all connected clients.
Values below 30 are reset to the default of 300 seconds.

#### `-allowedMessageLength`

Maximum allowed length, in characters, of incoming client messages.
Clients sending longer messages are disconnected. 0 means no limit,
which is the default.

## Other parameters

### `version`

Print the program version, then shut down immediately. Example:

```console
$ nvdaRemoteServer version
development
$
```

### `buildinfo`

Print the build information, then shut down immediately. Example:

```console
$ nvdaRemoteServer buildinfo
This application was compiled with go1.20.1. It was compiled for the amd64 architecture and the linux operating system.
```

### Daemon commands: `start`, `stop`, `restart`, `kill`, `status`

On Linux and other Unix-like systems, the server can be run as a
background daemon, mirroring the commands of the original Python server.
The daemon uses a PID file to track the running instance; by default
this is the server's `-pidfile` value (e.g.
`/run/NVDARemoteServer/NVDARemoteServer.pid`), and can be changed with
the `-pidfile` parameter, which must be passed after the command.

```console
$ nvdaRemoteServer start -port 6837
Server started with pid 12345.

$ nvdaRemoteServer status
Server is running with pid 12345.

$ nvdaRemoteServer restart
Stopping server with pid 12345...
Server started with pid 12346.

$ nvdaRemoteServer stop
Stopping server with pid 12346...
Server stopped.

$ nvdaRemoteServer kill
Killing server with pid 12346...
Server killed.
```

`stop` sends a SIGTERM and waits for the server to shut down gracefully.
`kill` sends a SIGKILL and should only be used if the server is frozen.
All arguments after the command are passed to the server itself. These
commands are not supported on Windows; run the server directly there.

## Let's Encrypt

Like the Python server, this server works with Let's Encrypt certificates
obtained through [certbot](https://certbot.eff.org/). The repository
contains a post-renewal hook script,
[`NVDARemoteCertificate-letsencrypt`](NVDARemoteCertificate-letsencrypt),
which copies the renewed certificate into the file used by the
`certfile`/`keyfile` options (by default
`/usr/share/NVDARemoteServer/server.pem`).

To set it up:

1. Edit the script and set `DOMAIN` to your server's domain.
2. Install it, for example:

```console
$ sudo install -m 755 NVDARemoteCertificate-letsencrypt /etc/letsencrypt/renewal-hooks/post/
```

3. Configure certbot to issue a certificate for your domain:

```console
$ sudo certbot certonly --standalone -d your-domain.com
```

After each renewal, certbot runs the hook automatically, and the server
picks up the new certificate on its next restart. If the certificate
file exists, the server uses it instead of generating a self-signed one.

# Extra Features

This server has a few extra features that the official addon and server do not support at the moment. This list is subject to change as the addon changes.

- The official server sends all data that one connected client sends, to all other connected clients. It relies upon the addon to determine whether or not a computer needs to be controlled. This server will determine if a computer is a controller (master, or a computer being controlled (slave). If a computer is a master, any sent data is only sent to all other connected slaves. If the computer is a slave, any sent data is only sent to all other masters.
- This server can optionally send no origin field, which may result in unexpected behavior, depending on whether or not the origin field is used for anything in the future.
- This server uses the nvda_not_connected type in the protocol, which is included in the addon, but not used in any way by the official server. If you are a master and no slaves are connected to control, the server will send this type to you, which will then instruct NVDA to tell you that NVDA Remote is not connected. This can be useful feedback if you have no idea whether or not a client is actually connected to control.
- If you prefix a key with "lock_", for example, using a key called "lock_nocontrol", no master will have the ability to control a slave. The server will intercept all data sent to a slave from a master and disguard it immediately. In addition, a message of the day will be displayed upon connection to the server, notifying you that you can't control a computer if you're a master, and that no one will be able to control your computer if you're a slave. Anyone could join this channel by using the key "nocontrol" or "lock_nocontrol"
- If you use a key called "nocontrol__password__controlme" or "lock_nocontrol__password__controlme", a locked channel will be created which can be controlled by any client using the password "controlme". Any master joining with "nocontrol" will be unable to control a slave, but if a master joins with the key "nocontrol__password__controlme", they can control a slave. Note: the first client connecting with the key will be the one to set the password, regardless of whether or not this client is a master or a slave.


# Statistics

These by no means should be taken as representative of any definitive stats, but the results given by systemd's tracking of memory and CPU use can speak for themselves.

When running the servers, both the Go and Python versions, I was performing similar tasks over different periods of time, typing, reading, and doing virtually everything on the remote system, including writing this section of the document and collecting the statistics.

Here is what they gathered for both the Python and Go versions of the NVDARemote server. The Python version I was running was 3.9.2, and Go was 1.16.2.


## Python

The run time was approximately one hour.

```console
$ sudo systemctl status NVDARemoteServer
 NVDARemoteServer.service - NVDARemote relay server
 Loaded: loaded (/usr/lib/systemd/system/NVDARemoteServer.service; disabled; vendor preset: disabled)
 Active: active (running) since Tue 2021-03-23 12:37:24 MDT; 1h 0min ago
 Process: 11551 ExecStart=/usr/bin/python /usr/share/NVDARemoteServer/server.py start (code=exited, status=0/SUCCESS)
 Main PID: 11553 (python)
 IP: 4.5M in, 5.2M out
 Tasks: 5 (limit: 1151)
 Memory: 12.0M
 CPU: 1min 13.385s
```


## Go

The runtime was approximately five hours, fourteen minutes.

```console
$ sudo systemctl status nvdaRemoteServer
 nvdaRemoteServer.service - NVDARemote relay server
 Loaded: loaded (/etc/systemd/system/nvdaRemoteServer.service; enabled; vendor preset: disabled)
 Active: active (running) since Tue 2021-03-23 07:21:02 MDT; 5h 14min ago
 Main PID: 7512 (nvdaRemoteServe)
 IP: 10.6M in, 11.6M out
 Tasks: 8 (limit: 1151)
 Memory: 4.8M
 CPU: 13.324s
```


## Notes about the results

There is one difference between the Python and Go versions of the server that is significant, other than the programming language being used. The Python server forks itself into the background, something that isn't strictly necessary to do with systemd processes. The Go version of the server does no forking, so the systemd service is capable of monitoring its process directly. This may cause results to be different than they should be, but the use of memory and CPU time should be fairly accurate.


## Observations using the servers

My personal observations are the following, running both servers on a server approximately 50MS ping time away from both locations, which would make the round trip approximately 100MS:

- When running the Python version of the NVDARemote server, the delay is noticeable between the controlling computer, and the computer being controlled.
- When running the Python version of the server using [PyPy,](https://www.pypy.org/) which is a faster version of Python for longer running programs, the delay is less, better than Python and a bit more stable. I can still tell that I'm controlling a remote computer.
- When running the Go version of the NVDARemote server, the delay can still be noticed, but is less than the Python version of the server. General stability and delay between keystrokes is also improved on the Go version of the server, and sometimes, I forget that I'm actually controlling a remote computer.

I took no benchmarks of response times between sending and receiving data, but I would estimate that the Go program is at least four or five times faster than the Python program, perhaps more so. It definitely seems to use less CPU.

In comparing the two servers, keep the following in mind. Python is an interpreted language. Therefore, the program is compiled into machine code as it is executed. This compiling and reading of the program will increase CPU use and slow down the responsiveness of a program. PyPy will compile the entire Python program into machine code before it begins to execute, which makes it faster than Python for long running processes, though slightly slower in starting. I believe it was stated that PyPy is at least four times faster than Python. Go will compile the entire program into machine code before you execute it, leaving you with a binary that you will run on your computer, similar to programming languages such as C. In [one particular use case,](https://getstream.io/blog/switched-python-go/#:~:text=Go%20is%20extremely%20fast.,40%20times%20faster%20than%20Python.) it was stated that Go was forty times faster than Python.


# Bugs

Open an issue explaining what the bug is and how you encountered it. Try and be as detailed as you can, to allow the bug to be reproduced. Be detailed, or your issue will be closed if it can't be resolved properly.


# Contributing

Fork this project and submit a pull request. Please use another branch on your fork of this project if you are submitting a pull request for something. Also, keep the following guidelines in mind.

- Test your contributions before submitting them, making sure the program compiles properly.
- Remember to make use of gofmt. This will keep the formatting of the code standard for everyone.
- Try and keep your code as clean and efficient as possible.


# Final thoughts

Primarily, I am writing this program for my use, but am releasing it for anyone to utalize, should they wish.

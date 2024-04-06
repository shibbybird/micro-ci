# eazy-ci

eazy-ci is a free to use language agnostic tool for developing, building, and testing complex systems.


## Why another CI tool?

Because the tools today are not complex enough and/or free for developing, building, and testing against complex microservice systems. Historically people have mocked or faked interactions with other systems, but with the advent of containers we can build better tested systems. Are integration tests slower? Of course they are, but are they more reliable, absolutely. With how powerful and cheap compute is these days we should all embrace integration tests... thus the advent of eazy-ci.

## Install

### Prerequisites
- Install ssh agent if you don't have it and set up an ssh key with your github account
- Install latest version of docker we are currently tracking against docker client version 1.40

Pull the appropriate binary from the releases page rename it "eazy" and put it on your path

### Linux
```
wget https://github.com/shibbybird/eazy-ci/releases/download/v0.0.2/eazy_linux -O /home/$USER/bin/eazy && chmod +x /home/$USER/bin/eazy
```

### MacOS
```
wget https://github.com/shibbybird/eazy-ci/releases/download/v0.0.2/eazy_macos -O /usr/local/bin/eazy && chmod +x /usr/local/bin/eazy
```

### Windows
```
# Not sure if this works?
# Someone please confirm?
wget https://github.com/shibbybird/eazy-ci/releases/download/v0.0.2/eazy_windows.exe -O /home/$USER/bin/eazy.exe
```

## Usage

### Basic Usage

```sh
$ cd /home/user/my-project
$ eazy
```
This command will in order:
- recursively pull eazy.yml files for all dependencies, and peer dependencies
- Start all dependency containers in order
- Build an artifact if configured in eazy.yml
- Build and start the production dockerfile
- Build and run the integration test dockerfile
- Report back results of the operations

### Development Mode
```sh
$ eazy -d
```
Using the -d flag will:
- recursively pull eazy.yml files for all dependencies, and peer dependencies
- Start all dependency containers in order
- Start build image or integration dockerfile if build image not supplied
- volume mount source code directory and attach to container which allows users to develop without manually configuring a dev environment on their local machine

### Integration Mode
```sh
$ eazy -i
```
Using the -d flag will:
- recursively pull eazy.yml files for all dependencies, and peer dependencies
- Start all dependency containers in order
- Build and run the production dockerfile image
- Start build image or integration dockerfile if build image not supplied
- volume mount source code directory and attach to container which allows users to integration test their code

### Other options

```
  -f	Specify the Eazy-CI file to run (default "./eazy.yml")
  -k	File path for ssh private key github access
  -p  Open ports to depedencies and project containers locally. DISCLAIMER: If there are port conflicts starting eazy will fail.
```

## Setting up a Eazy-CI Project

### Directory Structure

```
----> eazy.yml # eazy-ci configuration file
----> Dockerfile # The production image
----> Integration.Dockerfile #The file that can run healthchecks, bootstrapping, and integration tests for your project
```

### eazy.yml
#### Example:
```yml
eazyVersion: '1.0'
name: 'eazy-kotlin-test-service'
releases:
  - 'latest'
  - '0.0.1'
image: 'shibbybird/eazy-ci-kotlin-test-service'
build:
  buildEnvironment: 'gradle'
  image: 'gradle:5.2.1-jdk8'
  command:
    - '/bin/sh'
    - '-c'
    - 'gradle build'
deployment:
  env:
    - 'APP_ENV=integration'
  ports:
    - 7070
  health:
    - '/bin/sh'
    - '-c'
    - 'while ! curl http://eazy-kotlin-test-service:7070/health; do sleep 1; done;'
integration:
  env:
    - 'FOO=bar'
  bootstrap:
    - '/bin/sh'
    - '-c'
    - 'cqlsh eazy-ci-cassandra --cqlversion=3.4.4 -f /root/build/cassandra/init.cql'
  runTest:
    - '/bin/sh'
    - '-c'
    - './gradlew integration -i'
  dependencies:
    - 'github.com/shibbybird/eazy-ci-cassandra'
  peerDependencies:
    - 'github.com/shibbybird/eazy-ci-kafka'
```

## Build Environment
This is an agnostic tool, but special build environment caching is supported for some build environments. This works through volume mounting to your home directory in the ~/.eazy directory. Feel free to contribute more build environment caches so that development and building can be faster. On the docket is maven, and go.

### Build Environment Cache Support List:
- gradle
- sbt

### Example Projects:

#### [eazy - Kotlin Example Service](https://github.com/shibbybird/eazy-kotlin-test-service)
#### [eazy - Node.js Example API](https://github.com/shibbybird/eazy-node-test-registry)
#### [eazy - Scala Example API](https://github.com/shibbybird/eazy-scala-test-api)

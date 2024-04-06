package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/docker/docker/pkg/term"
	"github.com/shibbybird/eazy-ci/lib/builders"
	"github.com/shibbybird/eazy-ci/lib/runtimes"
	"github.com/shibbybird/eazy-ci/lib/utils"

	"github.com/shibbybird/eazy-ci/lib/config"
)

var version = "v0.0.2"

var liveContainerIDs = []string{}
var routableLinks = []string{}

var oldStateOut *term.State = nil

// end of code for environment variables

func main() {
	var runtime runtimes.ContainerRuntime
	ctx := context.Background()

	doCleanup := cleanUp(ctx, &runtime)

	// Create a .eazy directory in user home
	homeDir, err := utils.GetEazyHomeDir()
	if err != nil {
		doCleanup(err)
	}
	if _, err := os.Stat(homeDir); os.IsNotExist(err) {
		os.Mkdir(homeDir, 0775)
	}

	oldStateOut, _ = term.SetRawTerminalOutput(os.Stdout.Fd())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			doCleanup(errors.New("Interrupted by user"))
		}
	}()

	getVersion := flag.Bool("v", false, "Get version info")
	filePath := flag.String("f", "./eazy.yml", "The Eazy CI file")
	isDev := flag.Bool("d", false, "Run dependencies and peer depedencies")
	openPortsLocally := flag.Bool("p", false, "Open ports to depedencies and project containers locally. DISCLAIMER: If there are port conflicts starting eazy-ci will fail.")
	isIntegration := flag.Bool("i", false, "Run dependencies, peer dependencies, and build/start Dockerfile")
	pemKeyPath := flag.String("k", "", "File path for ssh private key for github access")

	flag.Parse()

	if *getVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	fileData, err := ioutil.ReadFile(*filePath)
	if err != nil {
		doCleanup(err)
	}

	yml, err := config.EazyYmlUnmarshal(fileData)
	if err != nil {
		doCleanup(err)
	}

	if runtime, err = runtimes.NewRuntime(yml); err != nil {
		doCleanup(err)
	}

	dependencies := []config.EazyYml{}

	err = utils.GetDependencies(yml, &dependencies, *pemKeyPath)

	// try to set up ssh agent if ssh isn't working
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "ssh") {
			err = utils.SetUpSSHKeys()
			if err != nil {
				doCleanup(err)
			}
			err = utils.GetDependencies(yml, &dependencies, *pemKeyPath)
			if err != nil {
				doCleanup(err)
			}
		} else {
			doCleanup(err)
		}
	}

	peerDependencies := []config.EazyYml{}
	peerDependenciesSet := map[string]bool{}

	// Collect Peer Dependencies
	for _, d := range dependencies {
		err = utils.GetPeerDependencies(d, &peerDependencies, peerDependenciesSet, *pemKeyPath)
		if err != nil {
			doCleanup(errors.New("can not find all peer dependencies"))
		}
	}
	err = utils.GetPeerDependencies(yml, &peerDependencies, peerDependenciesSet, *pemKeyPath)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "ssh") {
			err = utils.SetUpSSHKeys()
			if err != nil {
				doCleanup(err)
			}
			err = utils.GetPeerDependencies(yml, &peerDependencies, peerDependenciesSet, *pemKeyPath)
			if err != nil {
				doCleanup(errors.New("can not find peer dependencies on eazy.yml"))
			}
		} else {
			doCleanup(err)
		}
	}

	// Get integration env variables
	integrationEnv := yml.Integration.Env

	for _, d := range peerDependencies {
		startUnit(ctx, d, *openPortsLocally, runtime, integrationEnv)
	}

	for _, d := range dependencies {
		startUnit(ctx, d, *openPortsLocally, runtime, integrationEnv)
	}

	envBuilder := builders.GetBuildEnvironment(yml.Build.BuildEnvironment)
	localCacheMounts, err := envBuilder.GetLocalCacheMounts()

	if err != nil {
		doCleanup(err)
	}

	buildImageDocker, err := envBuilder.GetBuildContainerOptions()

	if err != nil {
		doCleanup(err)
	}

	var integrationImageID string

	if len(yml.Integration.Bootstrap) > 0 {
		integrationImageID, err = runtime.BuildAndRunContainer(ctx, yml, config.RuntimeConfig{
			Dockerfile:  "Integration.Dockerfile",
			Command:     yml.Integration.Bootstrap,
			Wait:        true,
			ExposePorts: false,
			Attach:      false,
			Mounts:      localCacheMounts,
		}, &routableLinks, &liveContainerIDs)

		if err != nil {
			doCleanup(err)
		}
	}

	if !*isDev {

		if len(yml.Build.Image) > 0 {
			buildImageDocker.Command = yml.Build.Command
			_, err := runtime.StartContainerByEazyYml(ctx, yml, yml.Build.Image,
				buildImageDocker, &routableLinks, &liveContainerIDs)

			if err != nil {
				doCleanup(err)
			}
		}

		_, err = runtime.BuildAndRunContainer(ctx, yml, config.RuntimeConfig{
			Env:         yml.Deployment.Env,
			Dockerfile:  "Dockerfile",
			Command:     []string{},
			Wait:        false,
			ExposePorts: *openPortsLocally,
			Attach:      false,
			IsRootImage: true,
		}, &routableLinks, &liveContainerIDs)

		if err != nil {
			doCleanup(err)
		}

		if len(yml.Deployment.Health) > 0 {
			healthDockerConfig := config.RuntimeConfig{
				Dockerfile:    "Integration.Dockerfile",
				Command:       yml.Deployment.Health,
				Wait:          true,
				ExposePorts:   false,
				Attach:        false,
				SkipImagePull: true,
				Mounts:        localCacheMounts,
			}
			if len(integrationImageID) > 0 {
				log.Println(integrationImageID)
				_, err = runtime.StartContainerByEazyYml(ctx, yml, integrationImageID, healthDockerConfig, &routableLinks, &liveContainerIDs)
			} else {
				integrationImageID, err = runtime.BuildAndRunContainer(ctx, yml, healthDockerConfig, &routableLinks, &liveContainerIDs)
			}
			if err != nil {
				doCleanup(err)
			}
		}
	}

	if *isDev || *isIntegration {
		buildImageDocker.Command = []string{"/bin/bash"}

		// If you have a build image then use this for dev
		// if not then use the integration docker image
		// Why do you not need a build image?
		if len(yml.Build.Image) > 0 && *isDev {
			_, err = runtime.StartContainerByEazyYml(ctx, yml, yml.Build.Image, buildImageDocker, &routableLinks, &liveContainerIDs)
		} else {
			buildImageDocker.Dockerfile = "Integration.Dockerfile"
			if len(integrationImageID) > 0 {
				buildImageDocker.SkipImagePull = true
				_, err = runtime.StartContainerByEazyYml(ctx, yml, integrationImageID, buildImageDocker, &routableLinks, &liveContainerIDs)
			} else {
				integrationImageID, err = runtime.BuildAndRunContainer(ctx, yml, buildImageDocker, &routableLinks, &liveContainerIDs)
			}
		}

		if err != nil {
			doCleanup(err)
		}

	} else {
		var runtimeConfig = config.RuntimeConfig{
			Dockerfile:  "Integration.Dockerfile",
			Command:     yml.Integration.RunTest,
			Wait:        true,
			ExposePorts: false,
			Attach:      false,
			Mounts:      localCacheMounts,
		}
		if len(integrationImageID) > 0 {
			runtimeConfig.SkipImagePull = true
			_, err = runtime.StartContainerByEazyYml(ctx, yml, integrationImageID, runtimeConfig, &routableLinks, &liveContainerIDs)
			if err != nil {
				doCleanup(err)
			}
		} else {
			integrationImageID, err = runtime.BuildAndRunContainer(ctx, yml, runtimeConfig, &routableLinks, &liveContainerIDs)
			if err != nil {
				doCleanup(err)
			}
		}
		doCleanup(nil)
	}

	doCleanup(nil)
}

func startUnit(ctx context.Context, yml config.EazyYml, openPortsLocally bool, runtime runtimes.ContainerRuntime, additionalEnv []string) {
	doCleanup := cleanUp(ctx, &runtime)

	if len(yml.Integration.Bootstrap) > 0 {
		_, err := runtime.StartContainerByEazyYml(ctx, yml, config.GetLatestIntegrationImageName(yml), config.RuntimeConfig{
			Command:     yml.Integration.Bootstrap,
			Wait:        true,
			ExposePorts: false,
		}, &routableLinks, &liveContainerIDs)

		if err != nil {
			doCleanup(err)
		}
	}
	_, err := runtime.StartContainerByEazyYml(ctx, yml, "", config.RuntimeConfig{
		Env:         append(yml.Deployment.Env, additionalEnv...),
		Wait:        false,
		ExposePorts: openPortsLocally,
		IsRootImage: true,
	}, &routableLinks, &liveContainerIDs)
	if err != nil {
		doCleanup(err)
	}
	if len(yml.Deployment.Health) > 0 {
		_, err := runtime.StartContainerByEazyYml(ctx, yml, config.GetLatestIntegrationImageName(yml), config.RuntimeConfig{
			Command:     yml.Deployment.Health,
			Wait:        true,
			ExposePorts: false,
		}, &routableLinks, &liveContainerIDs)
		if err != nil {
			doCleanup(err)
		}
	}
}

func cleanUp(ctx context.Context, runtime *runtimes.ContainerRuntime) func(err error) {
	return func(err error) {
		log.Println("cleaning up running containers...")
		term.RestoreTerminal(os.Stdout.Fd(), oldStateOut)
		for _, id := range liveContainerIDs {
			if runtime != nil {
				err := (*runtime).KillContainer(ctx, id)
				if err == nil {
					log.Println("container successfully shutdown: " + id)
				}
			}
		}
		if err == nil {
			log.Println("Succeeded!")
			os.Exit(0)
		} else {
			log.Println(err)
			log.Println("CI Failed!")
			os.Exit(1)
		}
	}
}

package config

import (
	"io/ioutil"
	"testing"
)

func TestEazyYml(t *testing.T) {
	data, err := ioutil.ReadFile("./testdata/test-eazy.yml")

	if err != nil {
		t.Error(err)
	}

	eazy, err := EazyYmlUnmarshal(data)

	if err != nil {
		t.Error(err)
	}

	if eazy.EazyVersion != "1.0" ||
		eazy.Releases[0] != "v1.0.0" ||
		eazy.Releases[1] != "v1.1.0" ||
		eazy.Image != "test/image" ||
		eazy.Runtime != "docker" ||
		eazy.Deployment.Health[0] != "/bin/sh" ||
		eazy.Deployment.Health[1] != "-c" ||
		eazy.Deployment.Health[2] != "while ! curl http://host.docker.internal:8080/health; do sleep 1; done;" ||
		eazy.Deployment.Ports[0] != "9000" ||
		eazy.Integration.Bootstrap[0] != "/bin/sh" ||
		eazy.Integration.Bootstrap[1] != "-c" ||
		eazy.Integration.Bootstrap[2] != "liquibase" ||
		eazy.Integration.RunTest[0] != "/bin/sh" ||
		eazy.Integration.RunTest[1] != "-c" ||
		eazy.Integration.RunTest[2] != "npm test" ||
		eazy.Integration.Dependencies[0] != "github.com/shibbybird/test-api" ||
		eazy.Integration.Dependencies[1] != "github.com/shibbybird/test-service" ||
		eazy.Integration.PeerDependencies[0] != "github.com/shibbybird/cassandra-db" ||
		eazy.Name != "test-service" ||
		eazy.Build.Image != "gradle:5.6.2-jdk8" ||
		eazy.Build.Command[2] != "gradle build" ||
		eazy.Deployment.Env[0] != "APP_ENV=integration" ||
		eazy.Build.BuildEnvironment != "gradle" ||
		eazy.Integration.Env[0] != "FOO=bar" {
		t.Error(eazy)
	}

}
